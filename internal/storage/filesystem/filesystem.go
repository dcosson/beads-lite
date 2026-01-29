// Package filesystem implements the Storage interface using the local filesystem.
// Each issue is stored as a JSON file in .beads/<project>/open/ or .beads/<project>/closed/.
package filesystem

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"syscall"
	"time"

	"beads-lite/internal/storage"
)

// FilesystemStorage implements storage.Storage using filesystem-based JSON files.
type FilesystemStorage struct {
	root string // path to .beads directory
}

// New creates a new FilesystemStorage rooted at the given directory.
func New(root string) *FilesystemStorage {
	return &FilesystemStorage{root: root}
}

// Init initializes the storage by creating the required directories.
func (fs *FilesystemStorage) Init(ctx context.Context) error {
	if err := os.MkdirAll(filepath.Join(fs.root, "open"), 0755); err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Join(fs.root, "closed"), 0755); err != nil {
		return err
	}
	// Clean up any stale lock files from previous crashed processes
	fs.CleanupStaleLocks()
	return nil
}

// CleanupStaleLocks removes lock files that don't have an active flock.
// This handles the case where a process was killed before it could clean up.
func (fs *FilesystemStorage) CleanupStaleLocks() {
	openDir := filepath.Join(fs.root, "open")
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
	dir := "open"
	if closed {
		dir = "closed"
	}
	return filepath.Join(fs.root, dir, id+".json")
}

func (fs *FilesystemStorage) lockPath(id string) string {
	return filepath.Join(fs.root, "open", id+".lock")
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

func generateID() (string, error) {
	bytes := make([]byte, 2) // 16 bits = 4 hex chars
	if _, err := rand.Read(bytes); err != nil {
		return "", err
	}
	return fmt.Sprintf("bd-%s", hex.EncodeToString(bytes)), nil
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

// Create creates a new issue and returns its generated ID.
func (fs *FilesystemStorage) Create(ctx context.Context, issue *storage.Issue) (string, error) {
	for attempts := 0; attempts < 3; attempts++ {
		id, err := generateID()
		if err != nil {
			return "", err
		}

		path := fs.issuePath(id, false)

		// O_EXCL fails if file exists - collision detection
		f, err := os.OpenFile(path, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0644)
		if os.IsExist(err) {
			continue // Collision, retry with new ID
		}
		if err != nil {
			return "", err
		}
		f.Close()

		issue.ID = id
		issue.CreatedAt = time.Now()
		issue.UpdatedAt = issue.CreatedAt
		if issue.Status == "" {
			issue.Status = storage.StatusOpen
		}

		if err := atomicWriteJSON(path, issue); err != nil {
			os.Remove(path)
			return "", err
		}

		return id, nil
	}
	return "", errors.New("failed to generate unique ID after 3 attempts")
}

// Get retrieves an issue by ID.
func (fs *FilesystemStorage) Get(ctx context.Context, id string) (*storage.Issue, error) {
	// Check open first (more common case)
	path := fs.issuePath(id, false)
	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		// Check closed
		path = fs.issuePath(id, true)
		data, err = os.ReadFile(path)
	}
	if os.IsNotExist(err) {
		return nil, storage.ErrNotFound
	}
	if err != nil {
		return nil, err
	}

	var issue storage.Issue
	if err := json.Unmarshal(data, &issue); err != nil {
		return nil, err
	}

	return &issue, nil
}

// Update replaces an issue's data.
func (fs *FilesystemStorage) Update(ctx context.Context, issue *storage.Issue) error {
	lock, err := fs.acquireLock(issue.ID)
	if err != nil {
		return err
	}
	defer lock.release()

	// Check if issue exists
	path := fs.issuePath(issue.ID, false)
	if _, err := os.Stat(path); os.IsNotExist(err) {
		path = fs.issuePath(issue.ID, true)
		if _, err := os.Stat(path); os.IsNotExist(err) {
			return storage.ErrNotFound
		}
	}

	issue.UpdatedAt = time.Now()
	return atomicWriteJSON(path, issue)
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
		return storage.ErrNotFound
	}
	if err == nil {
		// Clean up lock file for deleted issues.
		_ = os.Remove(fs.lockPath(id))
	}
	return err
}

// List returns all issues matching the filter.
// Results are sorted by CreatedAt (oldest first).
func (fs *FilesystemStorage) List(ctx context.Context, filter *storage.ListFilter) ([]*storage.Issue, error) {
	var issues []*storage.Issue

	// Determine which directories to scan
	scanOpen := true
	scanClosed := false

	if filter != nil && filter.Status != nil {
		if *filter.Status == storage.StatusClosed {
			scanOpen = false
			scanClosed = true
		}
	}

	if scanOpen {
		openIssues, err := fs.listDir(filepath.Join(fs.root, "open"), filter)
		if err != nil {
			return nil, err
		}
		issues = append(issues, openIssues...)
	}

	if scanClosed {
		closedIssues, err := fs.listDir(filepath.Join(fs.root, "closed"), filter)
		if err != nil {
			return nil, err
		}
		issues = append(issues, closedIssues...)
	}

	// Sort by CreatedAt (oldest first)
	sort.Slice(issues, func(i, j int) bool {
		return issues[i].CreatedAt.Before(issues[j].CreatedAt)
	})

	return issues, nil
}

func (fs *FilesystemStorage) listDir(dir string, filter *storage.ListFilter) ([]*storage.Issue, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}

	var issues []*storage.Issue
	for _, entry := range entries {
		if entry.IsDir() || filepath.Ext(entry.Name()) != ".json" {
			continue
		}

		path := filepath.Join(dir, entry.Name())
		data, err := os.ReadFile(path)
		if err != nil {
			continue
		}

		var issue storage.Issue
		if err := json.Unmarshal(data, &issue); err != nil {
			continue
		}

		if fs.matchesFilter(&issue, filter) {
			issues = append(issues, &issue)
		}
	}

	return issues, nil
}

func (fs *FilesystemStorage) matchesFilter(issue *storage.Issue, filter *storage.ListFilter) bool {
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

	issue.Status = storage.StatusClosed
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

	issue.Status = storage.StatusOpen
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
func (fs *FilesystemStorage) AddDependency(ctx context.Context, issueID, dependsOnID string, depType storage.DependencyType) error {
	if depType == storage.DepTypeParentChild {
		return fs.addParentChildDep(ctx, issueID, dependsOnID)
	}

	// Check for cycle before acquiring locks
	hasCycle, err := fs.hasDependencyCycle(ctx, issueID, dependsOnID)
	if err != nil {
		return err
	}
	if hasCycle {
		return storage.ErrCycle
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
		return storage.ErrCycle
	}

	// Add to issue's dependencies
	if !issue.HasDependency(dependsOnID) {
		issue.Dependencies = append(issue.Dependencies, storage.Dependency{ID: dependsOnID, Type: depType})
	}
	// Add to dependsOn's dependents
	if !dependsOn.HasDependent(issueID) {
		dependsOn.Dependents = append(dependsOn.Dependents, storage.Dependency{ID: issueID, Type: depType})
	}

	issue.UpdatedAt = time.Now()
	dependsOn.UpdatedAt = time.Now()

	if err := atomicWriteJSON(fs.issuePath(issueID, issue.Status == storage.StatusClosed), issue); err != nil {
		return err
	}
	return atomicWriteJSON(fs.issuePath(dependsOnID, dependsOn.Status == storage.StatusClosed), dependsOn)
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
		return storage.ErrCycle
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
		return storage.ErrCycle
	}

	// Remove from old parent if exists
	if child.Parent != "" && child.Parent != parentID {
		oldParent, err := fs.Get(ctx, child.Parent)
		if err == nil {
			oldParent.Dependents = removeDep(oldParent.Dependents, childID)
			oldParent.UpdatedAt = time.Now()
			atomicWriteJSON(fs.issuePath(child.Parent, oldParent.Status == storage.StatusClosed), oldParent)
		}
		child.Dependencies = removeDep(child.Dependencies, child.Parent)
	}

	child.Parent = parentID

	// Add parent-child typed dependency
	if !child.HasDependency(parentID) {
		child.Dependencies = append(child.Dependencies, storage.Dependency{ID: parentID, Type: storage.DepTypeParentChild})
	}
	if !parent.HasDependent(childID) {
		parent.Dependents = append(parent.Dependents, storage.Dependency{ID: childID, Type: storage.DepTypeParentChild})
	}

	child.UpdatedAt = time.Now()
	parent.UpdatedAt = time.Now()

	if err := atomicWriteJSON(fs.issuePath(childID, child.Status == storage.StatusClosed), child); err != nil {
		return err
	}
	return atomicWriteJSON(fs.issuePath(parentID, parent.Status == storage.StatusClosed), parent)
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
		if dep.ID == dependsOnID && dep.Type == storage.DepTypeParentChild {
			issue.Parent = ""
			break
		}
	}

	issue.Dependencies = removeDep(issue.Dependencies, dependsOnID)
	dependsOn.Dependents = removeDep(dependsOn.Dependents, issueID)

	issue.UpdatedAt = time.Now()
	dependsOn.UpdatedAt = time.Now()

	if err := atomicWriteJSON(fs.issuePath(issueID, issue.Status == storage.StatusClosed), issue); err != nil {
		return err
	}
	return atomicWriteJSON(fs.issuePath(dependsOnID, dependsOn.Status == storage.StatusClosed), dependsOn)
}

// AddComment adds a comment to an issue.
func (fs *FilesystemStorage) AddComment(ctx context.Context, issueID string, comment *storage.Comment) error {
	lock, err := fs.acquireLock(issueID)
	if err != nil {
		return err
	}
	defer lock.release()

	issue, err := fs.Get(ctx, issueID)
	if err != nil {
		return err
	}

	if comment.ID == "" {
		randBytes := make([]byte, 2)
		rand.Read(randBytes)
		comment.ID = fmt.Sprintf("c-%s", hex.EncodeToString(randBytes))
	}
	if comment.CreatedAt.IsZero() {
		comment.CreatedAt = time.Now()
	}

	issue.Comments = append(issue.Comments, *comment)
	issue.UpdatedAt = time.Now()

	isClosed := issue.Status == storage.StatusClosed
	return atomicWriteJSON(fs.issuePath(issueID, isClosed), issue)
}

// Doctor checks for and optionally fixes inconsistencies.
func (fs *FilesystemStorage) Doctor(ctx context.Context, fix bool) ([]string, error) {
	var problems []string

	// Load all issues from both directories
	openIssues := make(map[string]*storage.Issue)
	closedIssues := make(map[string]*storage.Issue)
	allIssues := make(map[string]*storage.Issue)

	// Scan open/ directory
	openEntries, err := os.ReadDir(filepath.Join(fs.root, "open"))
	if err != nil && !os.IsNotExist(err) {
		return nil, err
	}

	for _, entry := range openEntries {
		name := entry.Name()
		ext := filepath.Ext(name)

		// Check for orphaned temp files
		if strings.Contains(name, ".tmp.") {
			problems = append(problems, fmt.Sprintf("orphaned temp file: open/%s", name))
			if fix {
				os.Remove(filepath.Join(fs.root, "open", name))
			}
			continue
		}

		// Check for orphaned lock files
		if ext == ".lock" {
			id := name[:len(name)-5]
			jsonPath := filepath.Join(fs.root, "open", id+".json")
			if _, err := os.Stat(jsonPath); os.IsNotExist(err) {
				problems = append(problems, fmt.Sprintf("orphaned lock file: open/%s", name))
				if fix {
					os.Remove(filepath.Join(fs.root, "open", name))
				}
			}
			continue
		}

		if ext != ".json" {
			continue
		}

		id := name[:len(name)-5]
		path := filepath.Join(fs.root, "open", name)
		data, err := os.ReadFile(path)
		if err != nil {
			problems = append(problems, fmt.Sprintf("cannot read file: open/%s: %v", name, err))
			continue
		}

		var issue storage.Issue
		if err := json.Unmarshal(data, &issue); err != nil {
			problems = append(problems, fmt.Sprintf("malformed JSON: open/%s: %v", name, err))
			continue
		}

		openIssues[id] = &issue
		allIssues[id] = &issue
	}

	// Scan closed/ directory
	closedEntries, err := os.ReadDir(filepath.Join(fs.root, "closed"))
	if err != nil && !os.IsNotExist(err) {
		return nil, err
	}

	for _, entry := range closedEntries {
		name := entry.Name()
		ext := filepath.Ext(name)

		// Check for orphaned temp files
		if strings.Contains(name, ".tmp.") {
			problems = append(problems, fmt.Sprintf("orphaned temp file: closed/%s", name))
			if fix {
				os.Remove(filepath.Join(fs.root, "closed", name))
			}
			continue
		}

		if ext != ".json" {
			continue
		}

		id := name[:len(name)-5]
		path := filepath.Join(fs.root, "closed", name)
		data, err := os.ReadFile(path)
		if err != nil {
			problems = append(problems, fmt.Sprintf("cannot read file: closed/%s: %v", name, err))
			continue
		}

		var issue storage.Issue
		if err := json.Unmarshal(data, &issue); err != nil {
			problems = append(problems, fmt.Sprintf("malformed JSON: closed/%s: %v", name, err))
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
			problems = append(problems, fmt.Sprintf("duplicate issue: %s exists in both open/ and closed/", id))
			if fix {
				// Prefer the version with Status=closed if in closed/, otherwise keep the newer one
				if closedIssue.Status == storage.StatusClosed {
					// Remove from open/
					os.Remove(filepath.Join(fs.root, "open", id+".json"))
					allIssues[id] = closedIssue
				} else if openIssue.Status == storage.StatusClosed {
					// Remove from closed/
					os.Remove(filepath.Join(fs.root, "closed", id+".json"))
					allIssues[id] = openIssue
				} else {
					// Both have non-closed status, keep the one in open/ and remove from closed/
					os.Remove(filepath.Join(fs.root, "closed", id+".json"))
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
		if issue.Status == storage.StatusClosed {
			problems = append(problems, fmt.Sprintf("status mismatch: %s has status=closed but is in open/", id))
			if fix {
				// Move to closed/
				openPath := filepath.Join(fs.root, "open", id+".json")
				closedPath := filepath.Join(fs.root, "closed", id+".json")
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
		if issue.Status != storage.StatusClosed {
			problems = append(problems, fmt.Sprintf("status mismatch: %s has status=%s but is in closed/", id, issue.Status))
			if fix {
				// Move to open/
				closedPath := filepath.Join(fs.root, "closed", id+".json")
				openPath := filepath.Join(fs.root, "open", id+".json")
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
						parent.Dependents = append(parent.Dependents, storage.Dependency{ID: id, Type: storage.DepTypeParentChild})
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
						target.Dependents = append(target.Dependents, storage.Dependency{ID: id, Type: dep.Type})
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
						dependent.Dependencies = append(dependent.Dependencies, storage.Dependency{ID: id, Type: dep.Type})
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
			isClosed := issue.Status == storage.StatusClosed
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
func removeDep(deps []storage.Dependency, id string) []storage.Dependency {
	result := make([]storage.Dependency, 0, len(deps))
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

// GetNextChildID atomically increments and returns the next child number
// for the given parent ID. The first child of any parent returns 1.
func (fs *FilesystemStorage) GetNextChildID(ctx context.Context, parentID string) (int, error) {
	lockPath := fs.childCountersLockPath()
	f, err := os.OpenFile(lockPath, os.O_CREATE|os.O_RDWR, 0644)
	if err != nil {
		return 0, fmt.Errorf("opening child counters lock: %w", err)
	}
	defer f.Close()

	if err := syscall.Flock(int(f.Fd()), syscall.LOCK_EX); err != nil {
		return 0, fmt.Errorf("acquiring child counters lock: %w", err)
	}
	defer syscall.Flock(int(f.Fd()), syscall.LOCK_UN)

	// Read current counters
	counters := make(map[string]int)
	counterPath := fs.childCountersPath()
	data, err := os.ReadFile(counterPath)
	if err == nil {
		if err := json.Unmarshal(data, &counters); err != nil {
			return 0, fmt.Errorf("parsing child counters: %w", err)
		}
	} else if !os.IsNotExist(err) {
		return 0, fmt.Errorf("reading child counters: %w", err)
	}

	// Increment
	counters[parentID]++
	next := counters[parentID]

	// Write back atomically
	if err := atomicWriteJSON(counterPath, counters); err != nil {
		return 0, fmt.Errorf("writing child counters: %w", err)
	}

	return next, nil
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
			if err == storage.ErrNotFound {
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
			if err == storage.ErrNotFound {
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
