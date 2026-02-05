// Package filesystem implements the IssueStore interface using the local filesystem.
// Each issue is stored as a JSON file in .beads/<project>/open/ or .beads/<project>/closed/.
package filesystem

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"syscall"
	"time"

	"beads-lite/internal/idgen"
	"beads-lite/internal/issuestorage"
)

// MaxIDRetries is the maximum number of random ID generation attempts before
// returning an error. With AdaptiveLength ensuring ≤25% collision probability,
// P(20 consecutive collisions) ≈ 0.25^20 ≈ 10^-12.
const MaxIDRetries = 20

// FilesystemStorage implements issuestorage.IssueStore using filesystem-based JSON files.
type FilesystemStorage struct {
	root              string // path to .beads directory
	maxHierarchyDepth int
	prefix            string // ID prefix (e.g., "bd-", "bl-")
}

// Option configures a FilesystemStorage instance.
type Option func(*FilesystemStorage)

// WithMaxHierarchyDepth sets the maximum hierarchy depth for child IDs.
func WithMaxHierarchyDepth(n int) Option {
	return func(fs *FilesystemStorage) {
		fs.maxHierarchyDepth = n
	}
}

// New creates a new FilesystemStorage rooted at the given directory.
// The prefix is prepended to generated IDs (e.g., "bd-", "bl-").
func New(root, prefix string, opts ...Option) *FilesystemStorage {
	fs := &FilesystemStorage{
		root:              root,
		maxHierarchyDepth: issuestorage.DefaultMaxHierarchyDepth,
		prefix:            prefix,
	}
	for _, opt := range opts {
		opt(fs)
	}
	return fs
}

// Init initializes the storage by creating the required directories.
func (fs *FilesystemStorage) Init(ctx context.Context) error {
	for _, dir := range []string{issuestorage.DirOpen, issuestorage.DirClosed, issuestorage.DirDeleted, issuestorage.DirEphemeral} {
		if err := os.MkdirAll(filepath.Join(fs.root, dir), 0755); err != nil {
			return err
		}
	}
	// Recover any .backup files left by a crashed Modify.
	fs.recoverBackups()
	// Clean up any stale lock files from previous crashed processes
	fs.CleanupStaleLocks()
	return nil
}

// recoverBackups restores .json.backup files left by a Modify that
// crashed between creating the backup and completing the write.
func (fs *FilesystemStorage) recoverBackups() {
	for _, dir := range []string{issuestorage.DirOpen, issuestorage.DirClosed, issuestorage.DirDeleted, issuestorage.DirEphemeral} {
		dirPath := filepath.Join(fs.root, dir)
		entries, err := os.ReadDir(dirPath)
		if err != nil {
			continue
		}
		for _, entry := range entries {
			name := entry.Name()
			if !strings.HasSuffix(name, ".json.backup") {
				continue
			}
			backupPath := filepath.Join(dirPath, name)
			jsonPath := filepath.Join(dirPath, strings.TrimSuffix(name, ".backup"))
			// Restore the backup over the (potentially corrupt) json file.
			os.Rename(backupPath, jsonPath)
		}
	}
}

// CleanupStaleLocks removes lock files that don't have an active flock.
// This handles the case where a process was killed before it could clean up.
func (fs *FilesystemStorage) CleanupStaleLocks() {
	openDir := filepath.Join(fs.root, issuestorage.DirOpen)
	entries, err := os.ReadDir(openDir)
	if err != nil {
		return // Best effort cleanup
	}

	for _, entry := range entries {
		if !strings.HasSuffix(entry.Name(), ".lock") {
			continue
		}

		lockPath := filepath.Join(openDir, entry.Name())
		f, err := os.OpenFile(lockPath, os.O_RDWR, 0644)
		if err != nil {
			continue // Can't open, skip
		}

		// Try non-blocking lock - if we get it, the lock is stale
		err = syscall.Flock(int(f.Fd()), syscall.LOCK_EX|syscall.LOCK_NB)
		if err == nil {
			// We got the lock, meaning no other process holds it - it's stale
			syscall.Flock(int(f.Fd()), syscall.LOCK_UN)
			f.Close()
			os.Remove(lockPath)
		} else {
			// Lock is held by another process, leave it alone
			f.Close()
		}
	}
}

func (fs *FilesystemStorage) issuePath(id string, closed bool) string {
	dir := issuestorage.DirOpen
	if closed {
		dir = issuestorage.DirClosed
	}
	return filepath.Join(fs.root, dir, id+".json")
}

func (fs *FilesystemStorage) issuePathInDir(id string, dir string) string {
	return filepath.Join(fs.root, dir, id+".json")
}

func (fs *FilesystemStorage) lockPath(id string) string {
	return filepath.Join(fs.root, issuestorage.DirOpen, id+".lock")
}

// issueLock holds a file lock and its path for cleanup.
type issueLock struct {
	file *os.File
	path string
}

// release closes the lock file and removes it from disk.
func (l *issueLock) release() {
	l.file.Close()
	os.Remove(l.path)
}

// acquireLock gets an exclusive flock on the issue.
func (fs *FilesystemStorage) acquireLock(id string) (*issueLock, error) {
	lockPath := fs.lockPath(id)
	f, err := os.OpenFile(lockPath, os.O_CREATE|os.O_RDWR, 0644)
	if err != nil {
		return nil, err
	}

	if err := syscall.Flock(int(f.Fd()), syscall.LOCK_EX); err != nil {
		f.Close()
		return nil, err
	}

	return &issueLock{file: f, path: lockPath}, nil
}


// countAllIssues returns the total number of JSON issue files across all directories.
func (fs *FilesystemStorage) countAllIssues() (int, error) {
	count := 0
	for _, dir := range []string{issuestorage.DirOpen, issuestorage.DirClosed, issuestorage.DirDeleted, issuestorage.DirEphemeral} {
		entries, err := os.ReadDir(filepath.Join(fs.root, dir))
		if os.IsNotExist(err) {
			continue
		}
		if err != nil {
			return 0, err
		}
		for _, entry := range entries {
			if !entry.IsDir() && filepath.Ext(entry.Name()) == ".json" && !strings.Contains(entry.Name(), ".tmp.") {
				count++
			}
		}
	}
	return count, nil
}

func atomicWriteJSON(path string, data interface{}) error {
	// Generate a unique temporary filename
	randBytes := make([]byte, 8)
	if _, err := rand.Read(randBytes); err != nil {
		return fmt.Errorf("generating random suffix: %w", err)
	}
	tmp := path + ".tmp." + hex.EncodeToString(randBytes)

	f, err := os.OpenFile(tmp, os.O_CREATE|os.O_WRONLY|os.O_EXCL, 0644)
	if err != nil {
		return err
	}

	enc := json.NewEncoder(f)
	enc.SetIndent("", "  ")
	if err := enc.Encode(data); err != nil {
		f.Close()
		os.Remove(tmp)
		return err
	}
	if err := f.Sync(); err != nil {
		f.Close()
		os.Remove(tmp)
		return err
	}
	if err := f.Close(); err != nil {
		os.Remove(tmp)
		return err
	}

	if err := os.Rename(tmp, path); err != nil {
		os.Remove(tmp)
		return err
	}
	return nil
}

// Create creates a new issue and returns its ID.
// If issue.ID is already set, that ID is used directly (for hierarchical child IDs).
// Otherwise, a deterministic content-based ID is generated using SHA256 + base36.
func (fs *FilesystemStorage) Create(ctx context.Context, issue *issuestorage.Issue, opts ...issuestorage.CreateOpts) (string, error) {
	if issue.ID != "" {
		// Use the pre-set ID (e.g. from GetNextChildID or explicit --id)

		// Enforce hierarchy depth limit for explicit hierarchical IDs.
		if parentID, _, ok := issuestorage.ParseHierarchicalID(issue.ID); ok {
			if err := issuestorage.CheckHierarchyDepth(parentID, fs.maxHierarchyDepth); err != nil {
				return "", err
			}
		}

		if issue.Status == "" {
			issue.Status = issuestorage.StatusOpen
		}
		dir := issuestorage.DirForIssue(issue)
		path := fs.issuePathInDir(issue.ID, dir)

		f, err := os.OpenFile(path, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0644)
		if os.IsExist(err) {
			return "", fmt.Errorf("issue %s already exists", issue.ID)
		}
		if err != nil {
			return "", err
		}
		f.Close()

		issue.CreatedAt = time.Now()
		issue.UpdatedAt = issue.CreatedAt

		if err := atomicWriteJSON(path, issue); err != nil {
			os.Remove(path)
			return "", err
		}

		return issue.ID, nil
	}

	// Random ID generation with collision retry.
	// Count existing issues for adaptive length scaling.
	count, err := fs.countAllIssues()
	if err != nil {
		return "", fmt.Errorf("counting issues for adaptive length: %w", err)
	}

	length := idgen.AdaptiveLength(count)
	now := time.Now()

	// Compose the effective prefix, incorporating any PrefixAddition.
	var prefixAddition string
	if len(opts) > 0 {
		prefixAddition = opts[0].PrefixAddition
	}
	effectivePrefix := issuestorage.BuildPrefix(fs.prefix, prefixAddition)

	if issue.Status == "" {
		issue.Status = issuestorage.StatusOpen
	}
	dir := issuestorage.DirForIssue(issue)

	// Retry with fresh random IDs on collision.
	// AdaptiveLength ensures ≤25% collision probability,
	// so P(MaxIDRetries consecutive collisions) ≈ 0.25^20 ≈ 10^-12.
	for attempt := 0; attempt < MaxIDRetries; attempt++ {
		id, err := idgen.RandomID(effectivePrefix, length)
		if err != nil {
			return "", fmt.Errorf("generating random ID: %w", err)
		}
		path := fs.issuePathInDir(id, dir)

		// O_EXCL fails if file exists - collision detection
		f, err := os.OpenFile(path, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0644)
		if os.IsExist(err) {
			continue // Collision, try next random ID
		}
		if err != nil {
			return "", err
		}
		f.Close()

		issue.ID = id
		issue.CreatedAt = now
		issue.UpdatedAt = now

		if err := atomicWriteJSON(path, issue); err != nil {
			os.Remove(path)
			return "", err
		}

		return id, nil
	}
	return "", fmt.Errorf("failed to generate unique ID: %d retries exhausted at length %d", MaxIDRetries, length)
}

// readFileSharedLock reads a file while holding a shared (LOCK_SH) flock.
// This prevents reading while Modify holds an exclusive lock and is doing
// an in-place truncate+write.
func readFileSharedLock(path string) ([]byte, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	if err := syscall.Flock(int(f.Fd()), syscall.LOCK_SH); err != nil {
		return nil, err
	}
	defer syscall.Flock(int(f.Fd()), syscall.LOCK_UN)

	return io.ReadAll(f)
}

// Get retrieves an issue by ID.
func (fs *FilesystemStorage) Get(ctx context.Context, id string) (*issuestorage.Issue, error) {
	// Search order: open → ephemeral → closed → deleted
	var data []byte
	var err error
	for _, dir := range []string{issuestorage.DirOpen, issuestorage.DirEphemeral, issuestorage.DirClosed, issuestorage.DirDeleted} {
		data, err = readFileSharedLock(fs.issuePathInDir(id, dir))
		if !os.IsNotExist(err) {
			break
		}
	}
	if os.IsNotExist(err) {
		return nil, issuestorage.ErrNotFound
	}
	if err != nil {
		return nil, err
	}

	var issue issuestorage.Issue
	if err := json.Unmarshal(data, &issue); err != nil {
		return nil, err
	}

	return &issue, nil
}

// Modify atomically reads an issue, applies fn, and writes it back.
// It flocks the issue JSON file itself (not a separate lock file),
// so there is no flock+unlink race. A .backup copy is created before
// the in-place write for crash recovery.
//
// If fn changes the issue's status such that it belongs in a different
// directory (e.g., open/ → closed/), Modify handles the file movement
// automatically. Status transition side effects (ClosedAt, CloseReason)
// are applied via ApplyStatusDefaults.
func (fs *FilesystemStorage) Modify(ctx context.Context, id string, fn func(*issuestorage.Issue) error) error {
	// Find the issue file: open → ephemeral → closed → deleted
	var path string
	for _, dir := range []string{issuestorage.DirOpen, issuestorage.DirEphemeral, issuestorage.DirClosed, issuestorage.DirDeleted} {
		candidate := fs.issuePathInDir(id, dir)
		if _, err := os.Stat(candidate); err == nil {
			path = candidate
			break
		}
	}
	if path == "" {
		return issuestorage.ErrNotFound
	}

	f, err := os.OpenFile(path, os.O_RDWR, 0644)
	if err != nil {
		return fmt.Errorf("opening issue file: %w", err)
	}
	defer f.Close()

	if err := syscall.Flock(int(f.Fd()), syscall.LOCK_EX); err != nil {
		return fmt.Errorf("locking issue file: %w", err)
	}
	defer syscall.Flock(int(f.Fd()), syscall.LOCK_UN)

	// Read the current issue from the locked fd.
	data, err := io.ReadAll(f)
	if err != nil {
		return fmt.Errorf("reading issue file: %w", err)
	}

	var issue issuestorage.Issue
	if err := json.Unmarshal(data, &issue); err != nil {
		return fmt.Errorf("parsing issue file: %w", err)
	}

	oldStatus := issue.Status
	oldEphemeral := issue.Ephemeral

	if err := fn(&issue); err != nil {
		return err
	}

	// Apply status transition side effects.
	old := issuestorage.Issue{Status: oldStatus}
	issuestorage.ApplyStatusDefaults(&old, &issue)

	issue.UpdatedAt = time.Now()

	oldDir := issuestorage.DirForIssue(&issuestorage.Issue{Status: oldStatus, Ephemeral: oldEphemeral})
	newDir := issuestorage.DirForIssue(&issue)

	if oldDir == newDir {
		// Same directory — in-place write with backup (current behavior).
		newData, err := json.MarshalIndent(&issue, "", "  ")
		if err != nil {
			return fmt.Errorf("encoding issue: %w", err)
		}
		newData = append(newData, '\n')

		backupPath := path + ".backup"
		if err := os.WriteFile(backupPath, data, 0644); err != nil {
			return fmt.Errorf("writing backup: %w", err)
		}

		if _, err := f.Seek(0, 0); err != nil {
			return fmt.Errorf("seeking issue file: %w", err)
		}
		if err := f.Truncate(0); err != nil {
			return fmt.Errorf("truncating issue file: %w", err)
		}
		if _, err := f.Write(newData); err != nil {
			return fmt.Errorf("writing issue file: %w", err)
		}
		if err := f.Sync(); err != nil {
			return fmt.Errorf("syncing issue file: %w", err)
		}

		os.Remove(backupPath)
	} else {
		// Different directory — write to new location, remove old.
		newPath := fs.issuePathInDir(id, newDir)
		if err := atomicWriteJSON(newPath, &issue); err != nil {
			return fmt.Errorf("writing issue to %s: %w", newDir, err)
		}
		os.Remove(path)
	}

	return nil
}

// Delete permanently removes an issue.
func (fs *FilesystemStorage) Delete(ctx context.Context, id string) error {
	lock, err := fs.acquireLock(id)
	if err != nil {
		return err
	}
	defer lock.release()

	// Search order: open → ephemeral → closed → deleted
	for _, dir := range []string{issuestorage.DirOpen, issuestorage.DirEphemeral, issuestorage.DirClosed, issuestorage.DirDeleted} {
		err = os.Remove(fs.issuePathInDir(id, dir))
		if !os.IsNotExist(err) {
			break
		}
	}
	if os.IsNotExist(err) {
		return issuestorage.ErrNotFound
	}
	if err == nil {
		// Clean up lock file for deleted issues.
		_ = os.Remove(fs.lockPath(id))
	}
	return err
}

// List returns all issues matching the filter.
// Results are sorted by CreatedAt (oldest first).
func (fs *FilesystemStorage) List(ctx context.Context, filter *issuestorage.ListFilter) ([]*issuestorage.Issue, error) {
	var issues []*issuestorage.Issue

	// Determine which directories to scan
	scanOpen := true
	scanClosed := false
	scanDeleted := false

	if filter != nil && filter.Status != nil {
		switch *filter.Status {
		case issuestorage.StatusClosed:
			scanOpen = false
			scanClosed = true
		case issuestorage.StatusTombstone:
			scanOpen = false
			scanDeleted = true
		}
	}

	if scanOpen {
		openIssues, err := fs.listDir(filepath.Join(fs.root, issuestorage.DirOpen), filter)
		if err != nil {
			return nil, err
		}
		issues = append(issues, openIssues...)

		ephemeralIssues, err := fs.listDir(filepath.Join(fs.root, issuestorage.DirEphemeral), filter)
		if err != nil {
			return nil, err
		}
		issues = append(issues, ephemeralIssues...)
	}

	if scanClosed {
		closedIssues, err := fs.listDir(filepath.Join(fs.root, issuestorage.DirClosed), filter)
		if err != nil {
			return nil, err
		}
		issues = append(issues, closedIssues...)
	}

	if scanDeleted {
		deletedIssues, err := fs.listDir(filepath.Join(fs.root, issuestorage.DirDeleted), filter)
		if err != nil {
			return nil, err
		}
		issues = append(issues, deletedIssues...)
	}

	// Sort by CreatedAt (oldest first)
	sort.Slice(issues, func(i, j int) bool {
		return issues[i].CreatedAt.Before(issues[j].CreatedAt)
	})

	return issues, nil
}

func (fs *FilesystemStorage) listDir(dir string, filter *issuestorage.ListFilter) ([]*issuestorage.Issue, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}

	var issues []*issuestorage.Issue
	for _, entry := range entries {
		if entry.IsDir() || filepath.Ext(entry.Name()) != ".json" {
			continue
		}

		path := filepath.Join(dir, entry.Name())
		data, err := os.ReadFile(path)
		if err != nil {
			continue
		}

		var issue issuestorage.Issue
		if err := json.Unmarshal(data, &issue); err != nil {
			continue
		}

		if fs.matchesFilter(&issue, filter) {
			issues = append(issues, &issue)
		}
	}

	return issues, nil
}

func (fs *FilesystemStorage) matchesFilter(issue *issuestorage.Issue, filter *issuestorage.ListFilter) bool {
	if filter == nil {
		return true
	}

	if filter.Status != nil && issue.Status != *filter.Status {
		return false
	}
	if filter.Priority != nil && issue.Priority != *filter.Priority {
		return false
	}
	if filter.Type != nil && issue.Type != *filter.Type {
		return false
	}
	if filter.MolType != nil {
		filterMT := *filter.MolType
		issueMT := issue.MolType
		// Treat empty and "work" as equivalent
		if filterMT == "" || filterMT == issuestorage.MolTypeWork {
			if issueMT != "" && issueMT != issuestorage.MolTypeWork {
				return false
			}
		} else if issueMT != filterMT {
			return false
		}
	}
	if filter.Assignee != nil && issue.Assignee != *filter.Assignee {
		return false
	}
	if filter.Parent != nil {
		if *filter.Parent == "" && issue.Parent != "" {
			return false
		} else if *filter.Parent != "" && issue.Parent != *filter.Parent {
			return false
		}
	}

	// Check labels (must have all)
	for _, label := range filter.Labels {
		found := false
		for _, issueLabel := range issue.Labels {
			if issueLabel == label {
				found = true
				break
			}
		}
		if !found {
			return false
		}
	}

	return true
}

// Doctor checks for and optionally fixes inconsistencies.
func (fs *FilesystemStorage) Doctor(ctx context.Context, fix bool) ([]string, error) {
	var problems []string

	// issuesByDir tracks which directory each issue was found in.
	type locatedIssue struct {
		issue *issuestorage.Issue
		dir   string
	}
	issuesByID := make(map[string]*locatedIssue)
	allIssues := make(map[string]*issuestorage.Issue)

	// Scan all directories
	for _, dir := range []string{issuestorage.DirOpen, issuestorage.DirEphemeral, issuestorage.DirClosed} {
		entries, err := os.ReadDir(filepath.Join(fs.root, dir))
		if err != nil && !os.IsNotExist(err) {
			return nil, err
		}

		for _, entry := range entries {
			name := entry.Name()
			ext := filepath.Ext(name)

			// Check for orphaned temp files
			if strings.Contains(name, ".tmp.") {
				problems = append(problems, fmt.Sprintf("orphaned temp file: %s/%s", dir, name))
				if fix {
					os.Remove(filepath.Join(fs.root, dir, name))
				}
				continue
			}

			// Check for orphaned lock files (only in open/)
			if ext == ".lock" && dir == issuestorage.DirOpen {
				id := name[:len(name)-5]
				jsonPath := filepath.Join(fs.root, dir, id+".json")
				if _, err := os.Stat(jsonPath); os.IsNotExist(err) {
					problems = append(problems, fmt.Sprintf("orphaned lock file: %s/%s", dir, name))
					if fix {
						os.Remove(filepath.Join(fs.root, dir, name))
					}
				}
				continue
			}

			if ext != ".json" {
				continue
			}

			id := name[:len(name)-5]
			path := filepath.Join(fs.root, dir, name)
			data, err := os.ReadFile(path)
			if err != nil {
				problems = append(problems, fmt.Sprintf("cannot read file: %s/%s: %v", dir, name, err))
				continue
			}

			var issue issuestorage.Issue
			if err := json.Unmarshal(data, &issue); err != nil {
				problems = append(problems, fmt.Sprintf("malformed JSON: %s/%s: %v", dir, name, err))
				continue
			}

			if existing, exists := issuesByID[id]; exists {
				problems = append(problems, fmt.Sprintf("duplicate issue: %s exists in both %s/ and %s/", id, existing.dir, dir))
				if fix {
					// Keep the one in the correct directory based on status/ephemeral
					correctDir := issuestorage.DirForIssue(&issue)
					if dir == correctDir {
						os.Remove(filepath.Join(fs.root, existing.dir, id+".json"))
						issuesByID[id] = &locatedIssue{issue: &issue, dir: dir}
						allIssues[id] = &issue
					} else {
						os.Remove(filepath.Join(fs.root, dir, id+".json"))
					}
				}
			} else {
				issuesByID[id] = &locatedIssue{issue: &issue, dir: dir}
				allIssues[id] = &issue
			}
		}
	}

	// Check for status-location and ephemeral-location mismatches
	for id, loc := range issuesByID {
		expectedDir := issuestorage.DirForIssue(loc.issue)
		if loc.dir != expectedDir {
			if loc.issue.Ephemeral && loc.dir != issuestorage.DirEphemeral {
				problems = append(problems, fmt.Sprintf("ephemeral mismatch: %s is ephemeral but is in %s/ (expected %s/)", id, loc.dir, issuestorage.DirEphemeral))
			} else if !loc.issue.Ephemeral {
				problems = append(problems, fmt.Sprintf("status mismatch: %s has status=%s but is in %s/", id, loc.issue.Status, loc.dir))
			}
			if fix {
				oldPath := filepath.Join(fs.root, loc.dir, id+".json")
				newPath := filepath.Join(fs.root, expectedDir, id+".json")
				if err := atomicWriteJSON(newPath, loc.issue); err == nil {
					os.Remove(oldPath)
					loc.dir = expectedDir
				}
			}
		}
	}

	// Check for broken references and asymmetric relationships
	issuesNeedingUpdate := make(map[string]bool)

	for id, issue := range allIssues {
		// Check parent reference
		if issue.Parent != "" {
			if _, exists := allIssues[issue.Parent]; !exists {
				problems = append(problems, fmt.Sprintf("broken parent reference: %s references non-existent parent %s", id, issue.Parent))
				if fix {
					issue.Parent = ""
					issue.Dependencies = removeDep(issue.Dependencies, issue.Parent)
					issuesNeedingUpdate[id] = true
				}
			} else {
				// Check asymmetry: parent should have this issue as a parent-child dependent
				parent := allIssues[issue.Parent]
				if !parent.HasDependent(id) {
					problems = append(problems, fmt.Sprintf("asymmetric parent/child: %s has parent %s but parent doesn't list it as dependent", id, issue.Parent))
					if fix {
						parent.Dependents = append(parent.Dependents, issuestorage.Dependency{ID: id, Type: issuestorage.DepTypeParentChild})
						issuesNeedingUpdate[issue.Parent] = true
					}
				}
			}
		}

		// Check dependencies references
		for _, dep := range issue.Dependencies {
			if _, exists := allIssues[dep.ID]; !exists {
				problems = append(problems, fmt.Sprintf("broken dependency: %s depends on non-existent %s", id, dep.ID))
				if fix {
					issue.Dependencies = removeDep(issue.Dependencies, dep.ID)
					issuesNeedingUpdate[id] = true
				}
			} else {
				// Check asymmetry: dependency target should have this issue in dependents
				target := allIssues[dep.ID]
				if !target.HasDependent(id) {
					problems = append(problems, fmt.Sprintf("asymmetric dependency: %s depends on %s but %s doesn't list it as dependent", id, dep.ID, dep.ID))
					if fix {
						target.Dependents = append(target.Dependents, issuestorage.Dependency{ID: id, Type: dep.Type})
						issuesNeedingUpdate[dep.ID] = true
					}
				}
			}
		}

		// Check dependents references
		for _, dep := range issue.Dependents {
			if _, exists := allIssues[dep.ID]; !exists {
				problems = append(problems, fmt.Sprintf("broken dependent reference: %s has non-existent dependent %s", id, dep.ID))
				if fix {
					issue.Dependents = removeDep(issue.Dependents, dep.ID)
					issuesNeedingUpdate[id] = true
				}
			} else {
				// Check asymmetry: dependent should have this issue in dependencies
				dependent := allIssues[dep.ID]
				if !dependent.HasDependency(id) {
					problems = append(problems, fmt.Sprintf("asymmetric dependency: %s lists %s as dependent but %s doesn't depend on it", id, dep.ID, dep.ID))
					if fix {
						dependent.Dependencies = append(dependent.Dependencies, issuestorage.Dependency{ID: id, Type: dep.Type})
						issuesNeedingUpdate[dep.ID] = true
					}
				}
			}
		}
	}

	// Write back updated issues
	if fix {
		for id := range issuesNeedingUpdate {
			issue := allIssues[id]
			dir := issuestorage.DirForIssue(issue)
			path := fs.issuePathInDir(id, dir)
			atomicWriteJSON(path, issue)
		}
	}

	return problems, nil
}

// removeDep removes a dependency entry by ID from a Dependency slice.
func removeDep(deps []issuestorage.Dependency, id string) []issuestorage.Dependency {
	result := make([]issuestorage.Dependency, 0, len(deps))
	for _, d := range deps {
		if d.ID != id {
			result = append(result, d)
		}
	}
	return result
}

// scanMaxChildNumber scans all issue directories for direct children of
// parentID and returns the highest child number found. Returns 0 if no
// children exist.
func (fs *FilesystemStorage) scanMaxChildNumber(parentID string) (int, error) {
	dirs := []string{
		issuestorage.DirOpen, issuestorage.DirEphemeral,
		issuestorage.DirClosed, issuestorage.DirDeleted,
	}
	prefix := parentID + "."
	maxChild := 0

	for _, dir := range dirs {
		dirPath := filepath.Join(fs.root, dir)
		entries, err := os.ReadDir(dirPath)
		if err != nil {
			if os.IsNotExist(err) {
				continue
			}
			return 0, fmt.Errorf("reading %s: %w", dir, err)
		}
		for _, entry := range entries {
			name := entry.Name()
			if !strings.HasPrefix(name, prefix) || !strings.HasSuffix(name, ".json") {
				continue
			}
			id := strings.TrimSuffix(name, ".json")
			parent, childNum, ok := issuestorage.ParseHierarchicalID(id)
			if ok && parent == parentID && childNum > maxChild {
				maxChild = childNum
			}
		}
	}

	return maxChild, nil
}

// GetNextChildID validates the parent exists, checks hierarchy depth limits,
// scans the filesystem for existing children, and returns the next child ID.
// The returned ID is not reserved — the caller should create the issue with
// O_EXCL to handle concurrent races, retrying GetNextChildID on collision.
func (fs *FilesystemStorage) GetNextChildID(ctx context.Context, parentID string) (string, error) {
	// 1. Validate the parent exists
	if _, err := fs.Get(ctx, parentID); err != nil {
		if err == issuestorage.ErrNotFound {
			return "", fmt.Errorf("parent %s: %w", parentID, issuestorage.ErrNotFound)
		}
		return "", fmt.Errorf("checking parent %s: %w", parentID, err)
	}

	// 2. Check hierarchy depth limit
	if err := issuestorage.CheckHierarchyDepth(parentID, fs.maxHierarchyDepth); err != nil {
		return "", err
	}

	// 3. Scan filesystem for existing children, find highest number
	maxChild, err := fs.scanMaxChildNumber(parentID)
	if err != nil {
		return "", fmt.Errorf("scanning children of %s: %w", parentID, err)
	}

	return issuestorage.ChildID(parentID, maxChild+1), nil
}

