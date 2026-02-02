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
	if err := os.MkdirAll(filepath.Join(fs.root, issuestorage.DirOpen), 0755); err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Join(fs.root, issuestorage.DirClosed), 0755); err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Join(fs.root, issuestorage.DirDeleted), 0755); err != nil {
		return err
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
	for _, dir := range []string{issuestorage.DirOpen, issuestorage.DirClosed, issuestorage.DirDeleted} {
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

// acquireLocksOrdered acquires locks on multiple issues in sorted order to prevent deadlocks.
func (fs *FilesystemStorage) acquireLocksOrdered(ids []string) ([]*issueLock, error) {
	sorted := make([]string, len(ids))
	copy(sorted, ids)
	sort.Strings(sorted)

	locks := make([]*issueLock, 0, len(sorted))
	for _, id := range sorted {
		lock, err := fs.acquireLock(id)
		if err != nil {
			// Release already-acquired locks
			for _, l := range locks {
				l.release()
			}
			return nil, err
		}
		locks = append(locks, lock)
	}

	return locks, nil
}

func releaseLocks(locks []*issueLock) {
	for i := len(locks) - 1; i >= 0; i-- {
		locks[i].release()
	}
}

// countAllIssues returns the total number of JSON issue files across all directories.
func (fs *FilesystemStorage) countAllIssues() (int, error) {
	count := 0
	for _, dir := range []string{issuestorage.DirOpen, issuestorage.DirClosed, issuestorage.DirDeleted} {
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
func (fs *FilesystemStorage) Create(ctx context.Context, issue *issuestorage.Issue) (string, error) {
	if issue.ID != "" {
		// Use the pre-set ID (e.g. from GetNextChildID or explicit --id)

		// Enforce hierarchy depth limit for explicit hierarchical IDs.
		if parentID, _, ok := issuestorage.ParseHierarchicalID(issue.ID); ok {
			if err := issuestorage.CheckHierarchyDepth(parentID, fs.maxHierarchyDepth); err != nil {
				return "", err
			}
		}

		path := fs.issuePath(issue.ID, false)

		f, err := os.OpenFile(path, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0644)
		if os.IsExist(err) {
			return "", fmt.Errorf("issue %s already exists", issue.ID)
		}
		if err != nil {
			return "", err
		}
		f.Close()

		// Update child counter when creating with an explicit hierarchical ID
		// so future GetNextChildID calls won't generate colliding IDs.
		if parentID, childNum, ok := issuestorage.ParseHierarchicalID(issue.ID); ok {
			if err := fs.ensureChildCounterUpdated(parentID, childNum); err != nil {
				os.Remove(path)
				return "", fmt.Errorf("updating child counter for %s: %w", parentID, err)
			}
		}

		issue.CreatedAt = time.Now()
		issue.UpdatedAt = issue.CreatedAt
		if issue.Status == "" {
			issue.Status = issuestorage.StatusOpen
		}

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

	// Retry with fresh random IDs on collision.
	// AdaptiveLength ensures ≤25% collision probability,
	// so P(MaxIDRetries consecutive collisions) ≈ 0.25^20 ≈ 10^-12.
	for attempt := 0; attempt < MaxIDRetries; attempt++ {
		id, err := idgen.RandomID(fs.prefix, length)
		if err != nil {
			return "", fmt.Errorf("generating random ID: %w", err)
		}
		path := fs.issuePath(id, false)

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
		if issue.Status == "" {
			issue.Status = issuestorage.StatusOpen
		}

		if err := atomicWriteJSON(path, issue); err != nil {
			os.Remove(path)
			return "", err
		}

		return id, nil
	}
	return "", fmt.Errorf("failed to generate unique ID: %d retries exhausted at length %d", MaxIDRetries, length)
}

// Get retrieves an issue by ID.
func (fs *FilesystemStorage) Get(ctx context.Context, id string) (*issuestorage.Issue, error) {
	// Check open first (more common case)
	path := fs.issuePath(id, false)
	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		// Check closed
		path = fs.issuePath(id, true)
		data, err = os.ReadFile(path)
	}
	if os.IsNotExist(err) {
		// Check deleted (tombstones)
		path = fs.issuePathInDir(id, issuestorage.DirDeleted)
		data, err = os.ReadFile(path)
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

// Update replaces an issue's data.
func (fs *FilesystemStorage) Update(ctx context.Context, issue *issuestorage.Issue) error {
	lock, err := fs.acquireLock(issue.ID)
	if err != nil {
		return err
	}
	defer lock.release()

	// Check if issue exists (open -> closed -> deleted)
	path := fs.issuePath(issue.ID, false)
	if _, err := os.Stat(path); os.IsNotExist(err) {
		path = fs.issuePath(issue.ID, true)
		if _, err := os.Stat(path); os.IsNotExist(err) {
			path = fs.issuePathInDir(issue.ID, issuestorage.DirDeleted)
			if _, err := os.Stat(path); os.IsNotExist(err) {
				return issuestorage.ErrNotFound
			}
		}
	}

	issue.UpdatedAt = time.Now()
	return atomicWriteJSON(path, issue)
}

// Modify atomically reads an issue, applies fn, and writes it back.
// It flocks the issue JSON file itself (not a separate lock file),
// so there is no flock+unlink race. A .backup copy is created before
// the in-place write for crash recovery.
func (fs *FilesystemStorage) Modify(ctx context.Context, id string, fn func(*issuestorage.Issue) error) error {
	// Find the issue file.
	path := fs.issuePath(id, false)
	if _, err := os.Stat(path); os.IsNotExist(err) {
		path = fs.issuePath(id, true)
		if _, err := os.Stat(path); os.IsNotExist(err) {
			path = fs.issuePathInDir(id, issuestorage.DirDeleted)
			if _, err := os.Stat(path); os.IsNotExist(err) {
				return issuestorage.ErrNotFound
			}
		}
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

	if err := fn(&issue); err != nil {
		return err
	}

	issue.UpdatedAt = time.Now()

	newData, err := json.MarshalIndent(&issue, "", "  ")
	if err != nil {
		return fmt.Errorf("encoding issue: %w", err)
	}
	newData = append(newData, '\n')

	// Create backup before in-place write; recovered by Init if we crash.
	backupPath := path + ".backup"
	if err := os.WriteFile(backupPath, data, 0644); err != nil {
		return fmt.Errorf("writing backup: %w", err)
	}

	// Write in-place to the same inode (preserving the flock).
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

	// Write complete — remove the backup.
	os.Remove(backupPath)

	return nil
}

// Delete permanently removes an issue.
func (fs *FilesystemStorage) Delete(ctx context.Context, id string) error {
	lock, err := fs.acquireLock(id)
	if err != nil {
		return err
	}
	defer lock.release()

	path := fs.issuePath(id, false)
	err = os.Remove(path)
	if os.IsNotExist(err) {
		path = fs.issuePath(id, true)
		err = os.Remove(path)
	}
	if os.IsNotExist(err) {
		path = fs.issuePathInDir(id, issuestorage.DirDeleted)
		err = os.Remove(path)
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

// Close moves an issue to closed status.
func (fs *FilesystemStorage) Close(ctx context.Context, id string) error {
	lock, err := fs.acquireLock(id)
	if err != nil {
		return err
	}
	defer lock.release()

	issue, err := fs.Get(ctx, id)
	if err != nil {
		return err
	}

	issue.Status = issuestorage.StatusClosed
	now := time.Now()
	issue.ClosedAt = &now
	issue.CloseReason = "Closed"
	issue.UpdatedAt = now

	// Write to closed/ first
	closedPath := fs.issuePath(id, true)
	if err := atomicWriteJSON(closedPath, issue); err != nil {
		return err
	}

	// Remove from open/
	openPath := fs.issuePath(id, false)
	if err := os.Remove(openPath); err != nil {
		// Rollback: remove from closed/
		os.Remove(closedPath)
		return err
	}

	// Clean up the lock file (it's no longer needed for closed issues)
	os.Remove(fs.lockPath(id))

	return nil
}

// CreateTombstone converts an issue to a tombstone (soft-delete).
func (fs *FilesystemStorage) CreateTombstone(ctx context.Context, id string, actor string, reason string) error {
	lock, err := fs.acquireLock(id)
	if err != nil {
		return err
	}
	defer lock.release()

	issue, err := fs.Get(ctx, id)
	if err != nil {
		return err
	}

	if issue.Status == issuestorage.StatusTombstone {
		return issuestorage.ErrAlreadyTombstoned
	}

	// Record original type before overwriting
	issue.OriginalType = issue.Type

	// Set tombstone metadata
	issue.Status = issuestorage.StatusTombstone
	now := time.Now()
	issue.DeletedAt = &now
	issue.DeletedBy = actor
	issue.DeleteReason = reason
	issue.ClosedAt = nil
	issue.UpdatedAt = now

	// Write to deleted/ first
	deletedPath := fs.issuePathInDir(id, issuestorage.DirDeleted)
	if err := atomicWriteJSON(deletedPath, issue); err != nil {
		return err
	}

	// Remove from original location (try open/ then closed/)
	openPath := fs.issuePath(id, false)
	closedPath := fs.issuePath(id, true)
	errOpen := os.Remove(openPath)
	if os.IsNotExist(errOpen) {
		errClosed := os.Remove(closedPath)
		if os.IsNotExist(errClosed) {
			// Issue wasn't in open/ or closed/ — might have been created directly
			// in deleted/. The write above succeeded, so this is fine.
		} else if errClosed != nil {
			// Rollback: remove from deleted/
			os.Remove(deletedPath)
			return errClosed
		}
	} else if errOpen != nil {
		// Rollback: remove from deleted/
		os.Remove(deletedPath)
		return errOpen
	}

	// Clean up lock file
	os.Remove(fs.lockPath(id))

	return nil
}

// Reopen moves a closed issue back to open status.
func (fs *FilesystemStorage) Reopen(ctx context.Context, id string) error {
	lock, err := fs.acquireLock(id)
	if err != nil {
		return err
	}
	defer lock.release()

	issue, err := fs.Get(ctx, id)
	if err != nil {
		return err
	}

	issue.Status = issuestorage.StatusOpen
	issue.ClosedAt = nil
	issue.CloseReason = ""
	issue.UpdatedAt = time.Now()

	// Write to open/ first
	openPath := fs.issuePath(id, false)
	if err := atomicWriteJSON(openPath, issue); err != nil {
		return err
	}

	// Remove from closed/
	closedPath := fs.issuePath(id, true)
	if err := os.Remove(closedPath); err != nil && !os.IsNotExist(err) {
		// Rollback: remove from open/
		os.Remove(openPath)
		return err
	}

	return nil
}

// AddDependency creates a typed dependency relationship (issueID depends on dependsOnID).
// When depType is parent-child, also sets issueID.Parent and handles reparenting.
func (fs *FilesystemStorage) AddDependency(ctx context.Context, issueID, dependsOnID string, depType issuestorage.DependencyType) error {
	if depType == issuestorage.DepTypeParentChild {
		return fs.addParentChildDep(ctx, issueID, dependsOnID)
	}

	// Check for cycle before acquiring locks
	hasCycle, err := fs.hasDependencyCycle(ctx, issueID, dependsOnID)
	if err != nil {
		return err
	}
	if hasCycle {
		return issuestorage.ErrCycle
	}

	locks, err := fs.acquireLocksOrdered([]string{issueID, dependsOnID})
	if err != nil {
		return err
	}
	defer releaseLocks(locks)

	issue, err := fs.Get(ctx, issueID)
	if err != nil {
		return err
	}
	dependsOn, err := fs.Get(ctx, dependsOnID)
	if err != nil {
		return err
	}

	// Re-check for cycle after acquiring locks (data may have changed)
	hasCycle, err = fs.hasDependencyCycle(ctx, issueID, dependsOnID)
	if err != nil {
		return err
	}
	if hasCycle {
		return issuestorage.ErrCycle
	}

	// Add to issue's dependencies
	if !issue.HasDependency(dependsOnID) {
		issue.Dependencies = append(issue.Dependencies, issuestorage.Dependency{ID: dependsOnID, Type: depType})
	}
	// Add to dependsOn's dependents
	if !dependsOn.HasDependent(issueID) {
		dependsOn.Dependents = append(dependsOn.Dependents, issuestorage.Dependency{ID: issueID, Type: depType})
	}

	issue.UpdatedAt = time.Now()
	dependsOn.UpdatedAt = time.Now()

	if err := atomicWriteJSON(fs.issuePath(issueID, issue.Status == issuestorage.StatusClosed), issue); err != nil {
		return err
	}
	return atomicWriteJSON(fs.issuePath(dependsOnID, dependsOn.Status == issuestorage.StatusClosed), dependsOn)
}

// addParentChildDep handles AddDependency with parent-child type.
// Sets the Parent field and handles reparenting (removing old parent).
func (fs *FilesystemStorage) addParentChildDep(ctx context.Context, childID, parentID string) error {
	// Check for hierarchy cycle before acquiring locks
	hasCycle, err := fs.hasHierarchyCycle(ctx, childID, parentID)
	if err != nil {
		return err
	}
	if hasCycle {
		return issuestorage.ErrCycle
	}

	// Get the current child to check for old parent
	child, err := fs.Get(ctx, childID)
	if err != nil {
		return err
	}

	ids := []string{childID, parentID}
	if child.Parent != "" && child.Parent != parentID {
		ids = append(ids, child.Parent)
	}

	locks, err := fs.acquireLocksOrdered(ids)
	if err != nil {
		return err
	}
	defer releaseLocks(locks)

	// Re-read after locking
	child, err = fs.Get(ctx, childID)
	if err != nil {
		return err
	}
	parent, err := fs.Get(ctx, parentID)
	if err != nil {
		return err
	}

	// Re-check for cycle after acquiring locks
	hasCycle, err = fs.hasHierarchyCycle(ctx, childID, parentID)
	if err != nil {
		return err
	}
	if hasCycle {
		return issuestorage.ErrCycle
	}

	// Remove from old parent if exists
	if child.Parent != "" && child.Parent != parentID {
		oldParent, err := fs.Get(ctx, child.Parent)
		if err == nil {
			oldParent.Dependents = removeDep(oldParent.Dependents, childID)
			oldParent.UpdatedAt = time.Now()
			atomicWriteJSON(fs.issuePath(child.Parent, oldParent.Status == issuestorage.StatusClosed), oldParent)
		}
		child.Dependencies = removeDep(child.Dependencies, child.Parent)
	}

	child.Parent = parentID

	// Add parent-child typed dependency
	if !child.HasDependency(parentID) {
		child.Dependencies = append(child.Dependencies, issuestorage.Dependency{ID: parentID, Type: issuestorage.DepTypeParentChild})
	}
	if !parent.HasDependent(childID) {
		parent.Dependents = append(parent.Dependents, issuestorage.Dependency{ID: childID, Type: issuestorage.DepTypeParentChild})
	}

	child.UpdatedAt = time.Now()
	parent.UpdatedAt = time.Now()

	if err := atomicWriteJSON(fs.issuePath(childID, child.Status == issuestorage.StatusClosed), child); err != nil {
		return err
	}
	return atomicWriteJSON(fs.issuePath(parentID, parent.Status == issuestorage.StatusClosed), parent)
}

// RemoveDependency removes a dependency relationship by ID from both sides.
// If the removed dep was parent-child, also clears issueID.Parent.
func (fs *FilesystemStorage) RemoveDependency(ctx context.Context, issueID, dependsOnID string) error {
	locks, err := fs.acquireLocksOrdered([]string{issueID, dependsOnID})
	if err != nil {
		return err
	}
	defer releaseLocks(locks)

	issue, err := fs.Get(ctx, issueID)
	if err != nil {
		return err
	}
	dependsOn, err := fs.Get(ctx, dependsOnID)
	if err != nil {
		return err
	}

	// Check if this is a parent-child dep — clear Parent field
	for _, dep := range issue.Dependencies {
		if dep.ID == dependsOnID && dep.Type == issuestorage.DepTypeParentChild {
			issue.Parent = ""
			break
		}
	}

	issue.Dependencies = removeDep(issue.Dependencies, dependsOnID)
	dependsOn.Dependents = removeDep(dependsOn.Dependents, issueID)

	issue.UpdatedAt = time.Now()
	dependsOn.UpdatedAt = time.Now()

	if err := atomicWriteJSON(fs.issuePath(issueID, issue.Status == issuestorage.StatusClosed), issue); err != nil {
		return err
	}
	return atomicWriteJSON(fs.issuePath(dependsOnID, dependsOn.Status == issuestorage.StatusClosed), dependsOn)
}

// AddComment adds a comment to an issue.
func (fs *FilesystemStorage) AddComment(ctx context.Context, issueID string, comment *issuestorage.Comment) error {
	lock, err := fs.acquireLock(issueID)
	if err != nil {
		return err
	}
	defer lock.release()

	issue, err := fs.Get(ctx, issueID)
	if err != nil {
		return err
	}

	if comment.ID == 0 {
		maxID := 0
		for _, c := range issue.Comments {
			if c.ID > maxID {
				maxID = c.ID
			}
		}
		comment.ID = maxID + 1
	}
	if comment.CreatedAt.IsZero() {
		comment.CreatedAt = time.Now()
	}

	issue.Comments = append(issue.Comments, *comment)
	issue.UpdatedAt = time.Now()

	isClosed := issue.Status == issuestorage.StatusClosed
	return atomicWriteJSON(fs.issuePath(issueID, isClosed), issue)
}

// Doctor checks for and optionally fixes inconsistencies.
func (fs *FilesystemStorage) Doctor(ctx context.Context, fix bool) ([]string, error) {
	var problems []string

	// Load all issues from both directories
	openIssues := make(map[string]*issuestorage.Issue)
	closedIssues := make(map[string]*issuestorage.Issue)
	allIssues := make(map[string]*issuestorage.Issue)

	// Scan open/ directory
	openEntries, err := os.ReadDir(filepath.Join(fs.root, issuestorage.DirOpen))
	if err != nil && !os.IsNotExist(err) {
		return nil, err
	}

	for _, entry := range openEntries {
		name := entry.Name()
		ext := filepath.Ext(name)

		// Check for orphaned temp files
		if strings.Contains(name, ".tmp.") {
			problems = append(problems, fmt.Sprintf("orphaned temp file: %s/%s", issuestorage.DirOpen, name))
			if fix {
				os.Remove(filepath.Join(fs.root, issuestorage.DirOpen, name))
			}
			continue
		}

		// Check for orphaned lock files
		if ext == ".lock" {
			id := name[:len(name)-5]
			jsonPath := filepath.Join(fs.root, issuestorage.DirOpen, id+".json")
			if _, err := os.Stat(jsonPath); os.IsNotExist(err) {
				problems = append(problems, fmt.Sprintf("orphaned lock file: %s/%s", issuestorage.DirOpen, name))
				if fix {
					os.Remove(filepath.Join(fs.root, issuestorage.DirOpen, name))
				}
			}
			continue
		}

		if ext != ".json" {
			continue
		}

		id := name[:len(name)-5]
		path := filepath.Join(fs.root, issuestorage.DirOpen, name)
		data, err := os.ReadFile(path)
		if err != nil {
			problems = append(problems, fmt.Sprintf("cannot read file: %s/%s: %v", issuestorage.DirOpen, name, err))
			continue
		}

		var issue issuestorage.Issue
		if err := json.Unmarshal(data, &issue); err != nil {
			problems = append(problems, fmt.Sprintf("malformed JSON: %s/%s: %v", issuestorage.DirOpen, name, err))
			continue
		}

		openIssues[id] = &issue
		allIssues[id] = &issue
	}

	// Scan closed/ directory
	closedEntries, err := os.ReadDir(filepath.Join(fs.root, issuestorage.DirClosed))
	if err != nil && !os.IsNotExist(err) {
		return nil, err
	}

	for _, entry := range closedEntries {
		name := entry.Name()
		ext := filepath.Ext(name)

		// Check for orphaned temp files
		if strings.Contains(name, ".tmp.") {
			problems = append(problems, fmt.Sprintf("orphaned temp file: %s/%s", issuestorage.DirClosed, name))
			if fix {
				os.Remove(filepath.Join(fs.root, issuestorage.DirClosed, name))
			}
			continue
		}

		if ext != ".json" {
			continue
		}

		id := name[:len(name)-5]
		path := filepath.Join(fs.root, issuestorage.DirClosed, name)
		data, err := os.ReadFile(path)
		if err != nil {
			problems = append(problems, fmt.Sprintf("cannot read file: %s/%s: %v", issuestorage.DirClosed, name, err))
			continue
		}

		var issue issuestorage.Issue
		if err := json.Unmarshal(data, &issue); err != nil {
			problems = append(problems, fmt.Sprintf("malformed JSON: %s/%s: %v", issuestorage.DirClosed, name, err))
			continue
		}

		closedIssues[id] = &issue
		// Only add to allIssues if not already in open (duplicate check below)
		if _, exists := allIssues[id]; !exists {
			allIssues[id] = &issue
		}
	}

	// Check for duplicate issues (same ID in both open/ and closed/)
	for id := range openIssues {
		if closedIssue, exists := closedIssues[id]; exists {
			openIssue := openIssues[id]
			problems = append(problems, fmt.Sprintf("duplicate issue: %s exists in both %s/ and %s/", id, issuestorage.DirOpen, issuestorage.DirClosed))
			if fix {
				// Prefer the version with Status=closed if in closed/, otherwise keep the newer one
				if closedIssue.Status == issuestorage.StatusClosed {
					// Remove from open/
					os.Remove(filepath.Join(fs.root, issuestorage.DirOpen, id+".json"))
					allIssues[id] = closedIssue
				} else if openIssue.Status == issuestorage.StatusClosed {
					// Remove from closed/
					os.Remove(filepath.Join(fs.root, issuestorage.DirClosed, id+".json"))
					allIssues[id] = openIssue
				} else {
					// Both have non-closed status, keep the one in open/ and remove from closed/
					os.Remove(filepath.Join(fs.root, issuestorage.DirClosed, id+".json"))
					allIssues[id] = openIssue
				}
			}
		}
	}

	// Check for status-location mismatches
	for id, issue := range openIssues {
		if _, isDuplicate := closedIssues[id]; isDuplicate {
			continue // Already handled above
		}
		if issue.Status == issuestorage.StatusClosed {
			problems = append(problems, fmt.Sprintf("status mismatch: %s has status=closed but is in %s/", id, issuestorage.DirOpen))
			if fix {
				// Move to closed/
				openPath := filepath.Join(fs.root, issuestorage.DirOpen, id+".json")
				closedPath := filepath.Join(fs.root, issuestorage.DirClosed, id+".json")
				if err := atomicWriteJSON(closedPath, issue); err == nil {
					os.Remove(openPath)
				}
			}
		}
	}

	for id, issue := range closedIssues {
		if _, isDuplicate := openIssues[id]; isDuplicate {
			continue // Already handled above
		}
		if issue.Status != issuestorage.StatusClosed {
			problems = append(problems, fmt.Sprintf("status mismatch: %s has status=%s but is in %s/", id, issue.Status, issuestorage.DirClosed))
			if fix {
				// Move to open/
				closedPath := filepath.Join(fs.root, issuestorage.DirClosed, id+".json")
				openPath := filepath.Join(fs.root, issuestorage.DirOpen, id+".json")
				if err := atomicWriteJSON(openPath, issue); err == nil {
					os.Remove(closedPath)
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
			isClosed := issue.Status == issuestorage.StatusClosed
			path := fs.issuePath(id, isClosed)
			atomicWriteJSON(path, issue)
		}
	}

	return problems, nil
}

func contains(slice []string, item string) bool {
	for _, s := range slice {
		if s == item {
			return true
		}
	}
	return false
}

func remove(slice []string, item string) []string {
	result := make([]string, 0, len(slice))
	for _, s := range slice {
		if s != item {
			result = append(result, s)
		}
	}
	return result
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

// childCountersPath returns the path to the child counters JSON file.
func (fs *FilesystemStorage) childCountersPath() string {
	return filepath.Join(fs.root, "child_counters.json")
}

// childCountersLockPath returns the path to the lock file for child counters.
func (fs *FilesystemStorage) childCountersLockPath() string {
	return filepath.Join(fs.root, "child_counters.lock")
}

// ensureChildCounterUpdated sets the child counter for parentID to at least
// childNum, using max(current_counter, childNum). This prevents
// GetNextChildID from generating IDs that collide with explicitly-created
// hierarchical child IDs.
func (fs *FilesystemStorage) ensureChildCounterUpdated(parentID string, childNum int) error {
	lockPath := fs.childCountersLockPath()
	f, err := os.OpenFile(lockPath, os.O_CREATE|os.O_RDWR, 0644)
	if err != nil {
		return fmt.Errorf("opening child counters lock: %w", err)
	}
	defer f.Close()

	if err := syscall.Flock(int(f.Fd()), syscall.LOCK_EX); err != nil {
		return fmt.Errorf("acquiring child counters lock: %w", err)
	}
	defer syscall.Flock(int(f.Fd()), syscall.LOCK_UN)

	counters := make(map[string]int)
	counterPath := fs.childCountersPath()
	data, err := os.ReadFile(counterPath)
	if err == nil {
		if err := json.Unmarshal(data, &counters); err != nil {
			return fmt.Errorf("parsing child counters: %w", err)
		}
	} else if !os.IsNotExist(err) {
		return fmt.Errorf("reading child counters: %w", err)
	}

	if counters[parentID] < childNum {
		counters[parentID] = childNum
		if err := atomicWriteJSON(counterPath, counters); err != nil {
			return fmt.Errorf("writing child counters: %w", err)
		}
	}

	return nil
}

// GetNextChildID validates the parent exists, checks hierarchy depth limits,
// atomically increments the child counter, and returns the full child ID.
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

	// 3. Atomically increment the child counter
	lockPath := fs.childCountersLockPath()
	f, err := os.OpenFile(lockPath, os.O_CREATE|os.O_RDWR, 0644)
	if err != nil {
		return "", fmt.Errorf("opening child counters lock: %w", err)
	}
	defer f.Close()

	if err := syscall.Flock(int(f.Fd()), syscall.LOCK_EX); err != nil {
		return "", fmt.Errorf("acquiring child counters lock: %w", err)
	}
	defer syscall.Flock(int(f.Fd()), syscall.LOCK_UN)

	counters := make(map[string]int)
	counterPath := fs.childCountersPath()
	data, err := os.ReadFile(counterPath)
	if err == nil {
		if err := json.Unmarshal(data, &counters); err != nil {
			return "", fmt.Errorf("parsing child counters: %w", err)
		}
	} else if !os.IsNotExist(err) {
		return "", fmt.Errorf("reading child counters: %w", err)
	}

	counters[parentID]++
	next := counters[parentID]

	if err := atomicWriteJSON(counterPath, counters); err != nil {
		return "", fmt.Errorf("writing child counters: %w", err)
	}

	// 4. Return the full child ID
	return issuestorage.ChildID(parentID, next), nil
}

// hasDependencyCycle checks if adding a dependency from issueID to dependsOnID would create a cycle.
// It walks the dependency graph from 'dependsOnID' to see if 'issueID' is reachable.
func (fs *FilesystemStorage) hasDependencyCycle(ctx context.Context, issueID, dependsOnID string) (bool, error) {
	// Self-reference is always a cycle
	if issueID == dependsOnID {
		return true, nil
	}

	// BFS to check if issueID is reachable from dependsOnID via Dependencies
	visited := make(map[string]bool)
	queue := []string{dependsOnID}

	for len(queue) > 0 {
		current := queue[0]
		queue = queue[1:]

		if visited[current] {
			continue
		}
		visited[current] = true

		issue, err := fs.Get(ctx, current)
		if err != nil {
			if err == issuestorage.ErrNotFound {
				continue
			}
			return false, err
		}

		for _, dep := range issue.Dependencies {
			if dep.ID == issueID {
				return true, nil
			}
			if !visited[dep.ID] {
				queue = append(queue, dep.ID)
			}
		}
	}

	return false, nil
}

// hasHierarchyCycle checks if setting parent as the parent of child would create a cycle.
// It walks up the ancestor chain from parent to see if child is an ancestor of parent.
func (fs *FilesystemStorage) hasHierarchyCycle(ctx context.Context, child, parent string) (bool, error) {
	// Self-reference is always a cycle
	if child == parent {
		return true, nil
	}

	// Walk up the ancestor chain from parent
	current := parent
	visited := make(map[string]bool)

	for current != "" {
		if visited[current] {
			break // Existing cycle in data, stop here
		}
		visited[current] = true

		issue, err := fs.Get(ctx, current)
		if err != nil {
			if err == issuestorage.ErrNotFound {
				break
			}
			return false, err
		}

		if issue.Parent == child {
			return true, nil
		}
		current = issue.Parent
	}

	return false, nil
}
