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

	"beads2/internal/storage"
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
	return nil
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

// AddDependency creates a dependency relationship (A depends on B).
func (fs *FilesystemStorage) AddDependency(ctx context.Context, dependentID, dependencyID string) error {
	// Check for cycle before acquiring locks
	hasCycle, err := fs.hasDependencyCycle(ctx, dependentID, dependencyID)
	if err != nil {
		return err
	}
	if hasCycle {
		return storage.ErrCycle
	}

	locks, err := fs.acquireLocksOrdered([]string{dependentID, dependencyID})
	if err != nil {
		return err
	}
	defer releaseLocks(locks)

	dependent, err := fs.Get(ctx, dependentID)
	if err != nil {
		return err
	}
	dependency, err := fs.Get(ctx, dependencyID)
	if err != nil {
		return err
	}

	// Re-check for cycle after acquiring locks (data may have changed)
	hasCycle, err = fs.hasDependencyCycle(ctx, dependentID, dependencyID)
	if err != nil {
		return err
	}
	if hasCycle {
		return storage.ErrCycle
	}

	// Add to dependent's depends_on
	if !contains(dependent.DependsOn, dependencyID) {
		dependent.DependsOn = append(dependent.DependsOn, dependencyID)
	}
	// Add to dependency's dependents
	if !contains(dependency.Dependents, dependentID) {
		dependency.Dependents = append(dependency.Dependents, dependentID)
	}

	dependent.UpdatedAt = time.Now()
	dependency.UpdatedAt = time.Now()

	if err := atomicWriteJSON(fs.issuePath(dependentID, false), dependent); err != nil {
		return err
	}
	return atomicWriteJSON(fs.issuePath(dependencyID, false), dependency)
}

// RemoveDependency removes a dependency relationship.
func (fs *FilesystemStorage) RemoveDependency(ctx context.Context, dependentID, dependencyID string) error {
	locks, err := fs.acquireLocksOrdered([]string{dependentID, dependencyID})
	if err != nil {
		return err
	}
	defer releaseLocks(locks)

	dependent, err := fs.Get(ctx, dependentID)
	if err != nil {
		return err
	}
	dependency, err := fs.Get(ctx, dependencyID)
	if err != nil {
		return err
	}

	dependent.DependsOn = remove(dependent.DependsOn, dependencyID)
	dependency.Dependents = remove(dependency.Dependents, dependentID)

	dependent.UpdatedAt = time.Now()
	dependency.UpdatedAt = time.Now()

	if err := atomicWriteJSON(fs.issuePath(dependentID, false), dependent); err != nil {
		return err
	}
	return atomicWriteJSON(fs.issuePath(dependencyID, false), dependency)
}

// AddBlock creates a blocking relationship (A blocks B).
func (fs *FilesystemStorage) AddBlock(ctx context.Context, blockerID, blockedID string) error {
	// Check for cycle before acquiring locks
	hasCycle, err := fs.hasBlockCycle(ctx, blockerID, blockedID)
	if err != nil {
		return err
	}
	if hasCycle {
		return storage.ErrCycle
	}

	locks, err := fs.acquireLocksOrdered([]string{blockerID, blockedID})
	if err != nil {
		return err
	}
	defer releaseLocks(locks)

	blocker, err := fs.Get(ctx, blockerID)
	if err != nil {
		return err
	}
	blocked, err := fs.Get(ctx, blockedID)
	if err != nil {
		return err
	}

	// Re-check for cycle after acquiring locks (data may have changed)
	hasCycle, err = fs.hasBlockCycle(ctx, blockerID, blockedID)
	if err != nil {
		return err
	}
	if hasCycle {
		return storage.ErrCycle
	}

	if !contains(blocker.Blocks, blockedID) {
		blocker.Blocks = append(blocker.Blocks, blockedID)
	}
	if !contains(blocked.BlockedBy, blockerID) {
		blocked.BlockedBy = append(blocked.BlockedBy, blockerID)
	}

	blocker.UpdatedAt = time.Now()
	blocked.UpdatedAt = time.Now()

	if err := atomicWriteJSON(fs.issuePath(blockerID, false), blocker); err != nil {
		return err
	}
	return atomicWriteJSON(fs.issuePath(blockedID, false), blocked)
}

// RemoveBlock removes a blocking relationship.
func (fs *FilesystemStorage) RemoveBlock(ctx context.Context, blockerID, blockedID string) error {
	locks, err := fs.acquireLocksOrdered([]string{blockerID, blockedID})
	if err != nil {
		return err
	}
	defer releaseLocks(locks)

	blocker, err := fs.Get(ctx, blockerID)
	if err != nil {
		return err
	}
	blocked, err := fs.Get(ctx, blockedID)
	if err != nil {
		return err
	}

	blocker.Blocks = remove(blocker.Blocks, blockedID)
	blocked.BlockedBy = remove(blocked.BlockedBy, blockerID)

	blocker.UpdatedAt = time.Now()
	blocked.UpdatedAt = time.Now()

	if err := atomicWriteJSON(fs.issuePath(blockerID, false), blocker); err != nil {
		return err
	}
	return atomicWriteJSON(fs.issuePath(blockedID, false), blocked)
}

// SetParent sets the parent of an issue.
func (fs *FilesystemStorage) SetParent(ctx context.Context, childID, parentID string) error {
	// Check for cycle before acquiring locks
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

	// Re-check for cycle after acquiring locks (data may have changed)
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
			oldParent.Children = remove(oldParent.Children, childID)
			oldParent.UpdatedAt = time.Now()
			atomicWriteJSON(fs.issuePath(child.Parent, false), oldParent)
		}
	}

	child.Parent = parentID
	if !contains(parent.Children, childID) {
		parent.Children = append(parent.Children, childID)
	}

	child.UpdatedAt = time.Now()
	parent.UpdatedAt = time.Now()

	if err := atomicWriteJSON(fs.issuePath(childID, false), child); err != nil {
		return err
	}
	return atomicWriteJSON(fs.issuePath(parentID, false), parent)
}

// RemoveParent removes the parent relationship.
func (fs *FilesystemStorage) RemoveParent(ctx context.Context, childID string) error {
	child, err := fs.Get(ctx, childID)
	if err != nil {
		return err
	}

	if child.Parent == "" {
		return nil // No parent to remove
	}

	locks, err := fs.acquireLocksOrdered([]string{childID, child.Parent})
	if err != nil {
		return err
	}
	defer releaseLocks(locks)

	// Re-read after locking
	child, err = fs.Get(ctx, childID)
	if err != nil {
		return err
	}

	parent, err := fs.Get(ctx, child.Parent)
	if err != nil {
		return err
	}

	parent.Children = remove(parent.Children, childID)
	child.Parent = ""

	child.UpdatedAt = time.Now()
	parent.UpdatedAt = time.Now()

	if err := atomicWriteJSON(fs.issuePath(childID, false), child); err != nil {
		return err
	}
	return atomicWriteJSON(fs.issuePath(parent.ID, false), parent)
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
					issuesNeedingUpdate[id] = true
				}
			} else {
				// Check asymmetry: parent should have this issue in children
				parent := allIssues[issue.Parent]
				if !contains(parent.Children, id) {
					problems = append(problems, fmt.Sprintf("asymmetric parent/child: %s has parent %s but parent doesn't list it as child", id, issue.Parent))
					if fix {
						parent.Children = append(parent.Children, id)
						issuesNeedingUpdate[issue.Parent] = true
					}
				}
			}
		}

		// Check children references
		for _, childID := range issue.Children {
			if _, exists := allIssues[childID]; !exists {
				problems = append(problems, fmt.Sprintf("broken child reference: %s references non-existent child %s", id, childID))
				if fix {
					issue.Children = remove(issue.Children, childID)
					issuesNeedingUpdate[id] = true
				}
			} else {
				// Check asymmetry: child should have this issue as parent
				child := allIssues[childID]
				if child.Parent != id {
					problems = append(problems, fmt.Sprintf("asymmetric parent/child: %s lists %s as child but child's parent is %q", id, childID, child.Parent))
					if fix {
						if child.Parent == "" {
							child.Parent = id
							issuesNeedingUpdate[childID] = true
						} else {
							// Child has a different parent, remove from our children
							issue.Children = remove(issue.Children, childID)
							issuesNeedingUpdate[id] = true
						}
					}
				}
			}
		}

		// Check depends_on references
		for _, depID := range issue.DependsOn {
			if _, exists := allIssues[depID]; !exists {
				problems = append(problems, fmt.Sprintf("broken dependency: %s depends on non-existent %s", id, depID))
				if fix {
					issue.DependsOn = remove(issue.DependsOn, depID)
					issuesNeedingUpdate[id] = true
				}
			} else {
				// Check asymmetry: dependency should have this issue in dependents
				dep := allIssues[depID]
				if !contains(dep.Dependents, id) {
					problems = append(problems, fmt.Sprintf("asymmetric dependency: %s depends on %s but %s doesn't list it as dependent", id, depID, depID))
					if fix {
						dep.Dependents = append(dep.Dependents, id)
						issuesNeedingUpdate[depID] = true
					}
				}
			}
		}

		// Check dependents references
		for _, depID := range issue.Dependents {
			if _, exists := allIssues[depID]; !exists {
				problems = append(problems, fmt.Sprintf("broken dependent reference: %s has non-existent dependent %s", id, depID))
				if fix {
					issue.Dependents = remove(issue.Dependents, depID)
					issuesNeedingUpdate[id] = true
				}
			} else {
				// Check asymmetry: dependent should have this issue in depends_on
				dep := allIssues[depID]
				if !contains(dep.DependsOn, id) {
					problems = append(problems, fmt.Sprintf("asymmetric dependency: %s lists %s as dependent but %s doesn't depend on it", id, depID, depID))
					if fix {
						dep.DependsOn = append(dep.DependsOn, id)
						issuesNeedingUpdate[depID] = true
					}
				}
			}
		}

		// Check blocks references
		for _, blockedID := range issue.Blocks {
			if _, exists := allIssues[blockedID]; !exists {
				problems = append(problems, fmt.Sprintf("broken blocks reference: %s blocks non-existent %s", id, blockedID))
				if fix {
					issue.Blocks = remove(issue.Blocks, blockedID)
					issuesNeedingUpdate[id] = true
				}
			} else {
				// Check asymmetry: blocked issue should have this issue in blocked_by
				blocked := allIssues[blockedID]
				if !contains(blocked.BlockedBy, id) {
					problems = append(problems, fmt.Sprintf("asymmetric blocks: %s blocks %s but %s doesn't list it in blocked_by", id, blockedID, blockedID))
					if fix {
						blocked.BlockedBy = append(blocked.BlockedBy, id)
						issuesNeedingUpdate[blockedID] = true
					}
				}
			}
		}

		// Check blocked_by references
		for _, blockerID := range issue.BlockedBy {
			if _, exists := allIssues[blockerID]; !exists {
				problems = append(problems, fmt.Sprintf("broken blocked_by reference: %s blocked by non-existent %s", id, blockerID))
				if fix {
					issue.BlockedBy = remove(issue.BlockedBy, blockerID)
					issuesNeedingUpdate[id] = true
				}
			} else {
				// Check asymmetry: blocker should have this issue in blocks
				blocker := allIssues[blockerID]
				if !contains(blocker.Blocks, id) {
					problems = append(problems, fmt.Sprintf("asymmetric blocked_by: %s blocked by %s but %s doesn't list it in blocks", id, blockerID, blockerID))
					if fix {
						blocker.Blocks = append(blocker.Blocks, id)
						issuesNeedingUpdate[blockerID] = true
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

// hasDependencyCycle checks if adding a dependency from dependent to dependency would create a cycle.
// It walks the dependency graph from 'dependency' to see if 'dependent' is reachable.
func (fs *FilesystemStorage) hasDependencyCycle(ctx context.Context, dependent, dependency string) (bool, error) {
	// Self-reference is always a cycle
	if dependent == dependency {
		return true, nil
	}

	// BFS to check if dependent is reachable from dependency via DependsOn
	visited := make(map[string]bool)
	queue := []string{dependency}

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

		for _, dep := range issue.DependsOn {
			if dep == dependent {
				return true, nil
			}
			if !visited[dep] {
				queue = append(queue, dep)
			}
		}
	}

	return false, nil
}

// hasBlockCycle checks if adding a block relationship (blocker blocks blocked) would create a cycle.
// It walks the block graph from 'blocked' to see if 'blocker' is reachable via Blocks.
func (fs *FilesystemStorage) hasBlockCycle(ctx context.Context, blocker, blocked string) (bool, error) {
	// Self-reference is always a cycle
	if blocker == blocked {
		return true, nil
	}

	// BFS to check if blocker is reachable from blocked via Blocks
	visited := make(map[string]bool)
	queue := []string{blocked}

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

		for _, b := range issue.Blocks {
			if b == blocker {
				return true, nil
			}
			if !visited[b] {
				queue = append(queue, b)
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
