// Package filesystem implements the storage.Storage interface using the local filesystem.
// Issues are stored as JSON files in open/ and closed/ directories.
package filesystem

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"syscall"
	"time"

	"beads2/storage"
)

// FilesystemStorage implements storage.Storage using the local filesystem.
// Issues are stored as JSON files in .beads/open/ and .beads/closed/ directories.
type FilesystemStorage struct {
	root string // path to .beads directory
	fs   FS     // filesystem interface (for testing)
}

// New creates a new FilesystemStorage with the given root directory.
func New(root string) *FilesystemStorage {
	return &FilesystemStorage{
		root: root,
		fs:   osFS{},
	}
}

// NewWithFS creates a new FilesystemStorage with a custom filesystem implementation.
// This is primarily used for testing with mock filesystems.
func NewWithFS(root string, fs FS) *FilesystemStorage {
	return &FilesystemStorage{
		root: root,
		fs:   fs,
	}
}

// Init creates the required directory structure.
func (s *FilesystemStorage) Init(ctx context.Context) error {
	openDir := filepath.Join(s.root, "open")
	closedDir := filepath.Join(s.root, "closed")

	if err := s.fs.MkdirAll(openDir, 0755); err != nil {
		return fmt.Errorf("create open directory: %w", err)
	}
	if err := s.fs.MkdirAll(closedDir, 0755); err != nil {
		return fmt.Errorf("create closed directory: %w", err)
	}
	return nil
}

// generateID generates a random issue ID in the format "bd-XXXX".
func generateID() (string, error) {
	bytes := make([]byte, 2) // 16 bits = 4 hex chars
	if _, err := rand.Read(bytes); err != nil {
		return "", err
	}
	return "bd-" + hex.EncodeToString(bytes), nil
}

// issuePath returns the path to an issue file.
func (s *FilesystemStorage) issuePath(id string, closed bool) string {
	dir := "open"
	if closed {
		dir = "closed"
	}
	return filepath.Join(s.root, dir, id+".json")
}

// lockPath returns the path to an issue's lock file.
func (s *FilesystemStorage) lockPath(id string) string {
	return filepath.Join(s.root, "open", id+".lock")
}

// acquireLock gets an exclusive flock on the issue.
func (s *FilesystemStorage) acquireLock(id string) (*os.File, error) {
	lockPath := s.lockPath(id)
	f, err := os.OpenFile(lockPath, os.O_CREATE|os.O_RDWR, 0644)
	if err != nil {
		return nil, err
	}

	if err := syscall.Flock(int(f.Fd()), syscall.LOCK_EX); err != nil {
		f.Close()
		return nil, err
	}

	return f, nil
}

// acquireLocksOrdered acquires locks on multiple issues in sorted order to prevent deadlock.
func (s *FilesystemStorage) acquireLocksOrdered(ids []string) ([]*os.File, error) {
	sorted := make([]string, len(ids))
	copy(sorted, ids)
	sort.Strings(sorted)

	locks := make([]*os.File, 0, len(sorted))
	for _, id := range sorted {
		lock, err := s.acquireLock(id)
		if err != nil {
			// Release already-acquired locks
			for _, l := range locks {
				l.Close()
			}
			return nil, err
		}
		locks = append(locks, lock)
	}

	return locks, nil
}

// releaseLocks releases a slice of lock files.
func releaseLocks(locks []*os.File) {
	for i := len(locks) - 1; i >= 0; i-- {
		locks[i].Close()
	}
}

// atomicWriteJSON writes data to a file atomically using write-to-temp-then-rename.
func (s *FilesystemStorage) atomicWriteJSON(path string, data interface{}) error {
	// Generate temp file path
	dir := filepath.Dir(path)
	base := filepath.Base(path)

	randBytes := make([]byte, 4)
	rand.Read(randBytes)
	tmpName := fmt.Sprintf(".%s.tmp.%s", base, hex.EncodeToString(randBytes))
	tmpPath := filepath.Join(dir, tmpName)

	// Marshal to JSON
	jsonData, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal JSON: %w", err)
	}
	jsonData = append(jsonData, '\n')

	// Write to temp file
	if err := s.fs.WriteFile(tmpPath, jsonData, 0644); err != nil {
		return fmt.Errorf("write temp file: %w", err)
	}

	// Atomic rename
	if err := s.fs.Rename(tmpPath, path); err != nil {
		s.fs.Remove(tmpPath) // Clean up temp file on failure
		return fmt.Errorf("rename temp file: %w", err)
	}

	return nil
}

// Create creates a new issue and returns its generated ID.
func (s *FilesystemStorage) Create(ctx context.Context, issue *storage.Issue) (string, error) {
	// Retry loop for collision handling
	for attempts := 0; attempts < 3; attempts++ {
		id, err := generateID()
		if err != nil {
			return "", fmt.Errorf("generate ID: %w", err)
		}

		path := s.issuePath(id, false)

		// Check if file already exists (collision detection)
		if _, err := s.fs.Stat(path); err == nil {
			continue // Collision, retry with new ID
		}

		// Set issue fields
		issue.ID = id
		now := time.Now()
		if issue.CreatedAt.IsZero() {
			issue.CreatedAt = now
		}
		issue.UpdatedAt = now
		if issue.Status == "" {
			issue.Status = storage.StatusOpen
		}

		// Write the issue
		if err := s.atomicWriteJSON(path, issue); err != nil {
			return "", err
		}

		return id, nil
	}
	return "", fmt.Errorf("failed to generate unique ID after 3 attempts")
}

// Get retrieves an issue by ID.
func (s *FilesystemStorage) Get(ctx context.Context, id string) (*storage.Issue, error) {
	// Check open first (more common case)
	path := s.issuePath(id, false)
	data, err := s.fs.ReadFile(path)
	if os.IsNotExist(err) {
		// Check closed
		path = s.issuePath(id, true)
		data, err = s.fs.ReadFile(path)
	}
	if os.IsNotExist(err) {
		return nil, storage.ErrNotFound
	}
	if err != nil {
		return nil, err
	}

	var issue storage.Issue
	if err := json.Unmarshal(data, &issue); err != nil {
		return nil, fmt.Errorf("parse issue JSON: %w", err)
	}

	return &issue, nil
}

// Update replaces an issue's data.
func (s *FilesystemStorage) Update(ctx context.Context, issue *storage.Issue) error {
	lock, err := s.acquireLock(issue.ID)
	if err != nil {
		return fmt.Errorf("acquire lock: %w", err)
	}
	defer lock.Close()

	// Determine which directory the issue is in
	openPath := s.issuePath(issue.ID, false)
	closedPath := s.issuePath(issue.ID, true)

	var path string
	if _, err := s.fs.Stat(openPath); err == nil {
		path = openPath
	} else if _, err := s.fs.Stat(closedPath); err == nil {
		path = closedPath
	} else {
		return storage.ErrNotFound
	}

	issue.UpdatedAt = time.Now()

	return s.atomicWriteJSON(path, issue)
}

// Delete permanently removes an issue.
func (s *FilesystemStorage) Delete(ctx context.Context, id string) error {
	lock, err := s.acquireLock(id)
	if err != nil {
		return fmt.Errorf("acquire lock: %w", err)
	}
	defer lock.Close()

	// Try to remove from open
	openPath := s.issuePath(id, false)
	if err := s.fs.Remove(openPath); err == nil {
		// Clean up lock file
		s.fs.Remove(s.lockPath(id))
		return nil
	}

	// Try to remove from closed
	closedPath := s.issuePath(id, true)
	if err := s.fs.Remove(closedPath); err == nil {
		// Clean up lock file
		s.fs.Remove(s.lockPath(id))
		return nil
	}

	return storage.ErrNotFound
}

// List returns all issues matching the filter.
func (s *FilesystemStorage) List(ctx context.Context, filter *storage.ListFilter) ([]*storage.Issue, error) {
	var issues []*storage.Issue

	// Read from open directory
	openDir := filepath.Join(s.root, "open")
	openIssues, err := s.readIssuesFromDir(openDir)
	if err != nil {
		return nil, fmt.Errorf("read open issues: %w", err)
	}
	issues = append(issues, openIssues...)

	// If filter requests closed issues, read from closed directory too
	if filter != nil && filter.Status != nil && *filter.Status == storage.StatusClosed {
		closedDir := filepath.Join(s.root, "closed")
		closedIssues, err := s.readIssuesFromDir(closedDir)
		if err != nil {
			return nil, fmt.Errorf("read closed issues: %w", err)
		}
		issues = closedIssues // Replace with just closed
	}

	// Apply filters
	if filter != nil {
		issues = applyFilter(issues, filter)
	}

	return issues, nil
}

// readIssuesFromDir reads all issue JSON files from a directory.
func (s *FilesystemStorage) readIssuesFromDir(dir string) ([]*storage.Issue, error) {
	entries, err := s.fs.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}

	var issues []*storage.Issue
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".json") {
			continue
		}

		path := filepath.Join(dir, entry.Name())
		data, err := s.fs.ReadFile(path)
		if err != nil {
			continue // Skip unreadable files
		}

		var issue storage.Issue
		if err := json.Unmarshal(data, &issue); err != nil {
			continue // Skip malformed files
		}

		issues = append(issues, &issue)
	}

	return issues, nil
}

// applyFilter filters issues based on the given criteria.
func applyFilter(issues []*storage.Issue, filter *storage.ListFilter) []*storage.Issue {
	var result []*storage.Issue
	for _, issue := range issues {
		if filter.Status != nil && issue.Status != *filter.Status {
			continue
		}
		if filter.Priority != nil && issue.Priority != *filter.Priority {
			continue
		}
		if filter.Type != nil && issue.Type != *filter.Type {
			continue
		}
		if filter.Parent != nil && issue.Parent != *filter.Parent {
			continue
		}
		if filter.Assignee != nil && issue.Assignee != *filter.Assignee {
			continue
		}
		if len(filter.Labels) > 0 && !hasAllLabels(issue.Labels, filter.Labels) {
			continue
		}
		result = append(result, issue)
	}
	return result
}

// hasAllLabels checks if issueLabels contains all requiredLabels.
func hasAllLabels(issueLabels, requiredLabels []string) bool {
	labelSet := make(map[string]bool)
	for _, l := range issueLabels {
		labelSet[l] = true
	}
	for _, l := range requiredLabels {
		if !labelSet[l] {
			return false
		}
	}
	return true
}

// Close moves an issue to closed status.
func (s *FilesystemStorage) Close(ctx context.Context, id string) error {
	lock, err := s.acquireLock(id)
	if err != nil {
		return fmt.Errorf("acquire lock: %w", err)
	}
	defer lock.Close()

	issue, err := s.Get(ctx, id)
	if err != nil {
		return err
	}

	issue.Status = storage.StatusClosed
	now := time.Now()
	issue.ClosedAt = &now
	issue.UpdatedAt = now

	// Write to closed/ first
	closedPath := s.issuePath(id, true)
	if err := s.atomicWriteJSON(closedPath, issue); err != nil {
		return fmt.Errorf("write to closed: %w", err)
	}

	// Remove from open/
	openPath := s.issuePath(id, false)
	if err := s.fs.Remove(openPath); err != nil {
		// Rollback: remove from closed/
		s.fs.Remove(closedPath)
		return fmt.Errorf("remove from open: %w", err)
	}

	return nil
}

// Reopen moves a closed issue back to open status.
func (s *FilesystemStorage) Reopen(ctx context.Context, id string) error {
	lock, err := s.acquireLock(id)
	if err != nil {
		return fmt.Errorf("acquire lock: %w", err)
	}
	defer lock.Close()

	issue, err := s.Get(ctx, id)
	if err != nil {
		return err
	}

	issue.Status = storage.StatusOpen
	issue.ClosedAt = nil
	issue.UpdatedAt = time.Now()

	// Write to open/ first
	openPath := s.issuePath(id, false)
	if err := s.atomicWriteJSON(openPath, issue); err != nil {
		return fmt.Errorf("write to open: %w", err)
	}

	// Remove from closed/
	closedPath := s.issuePath(id, true)
	if err := s.fs.Remove(closedPath); err != nil {
		// Rollback: remove from open/
		s.fs.Remove(openPath)
		return fmt.Errorf("remove from closed: %w", err)
	}

	return nil
}

// AddDependency creates a dependency relationship (dependentID depends on dependencyID).
func (s *FilesystemStorage) AddDependency(ctx context.Context, dependentID, dependencyID string) error {
	locks, err := s.acquireLocksOrdered([]string{dependentID, dependencyID})
	if err != nil {
		return fmt.Errorf("acquire locks: %w", err)
	}
	defer releaseLocks(locks)

	dependent, err := s.Get(ctx, dependentID)
	if err != nil {
		return fmt.Errorf("get dependent: %w", err)
	}

	dependency, err := s.Get(ctx, dependencyID)
	if err != nil {
		return fmt.Errorf("get dependency: %w", err)
	}

	// Update dependent
	if !contains(dependent.DependsOn, dependencyID) {
		dependent.DependsOn = append(dependent.DependsOn, dependencyID)
	}

	// Update dependency
	if !contains(dependency.Dependents, dependentID) {
		dependency.Dependents = append(dependency.Dependents, dependentID)
	}

	// Write both
	if err := s.Update(ctx, dependent); err != nil {
		return fmt.Errorf("update dependent: %w", err)
	}
	if err := s.Update(ctx, dependency); err != nil {
		return fmt.Errorf("update dependency: %w", err)
	}

	return nil
}

// RemoveDependency removes a dependency relationship.
func (s *FilesystemStorage) RemoveDependency(ctx context.Context, dependentID, dependencyID string) error {
	locks, err := s.acquireLocksOrdered([]string{dependentID, dependencyID})
	if err != nil {
		return fmt.Errorf("acquire locks: %w", err)
	}
	defer releaseLocks(locks)

	dependent, err := s.Get(ctx, dependentID)
	if err != nil {
		return fmt.Errorf("get dependent: %w", err)
	}

	dependency, err := s.Get(ctx, dependencyID)
	if err != nil {
		return fmt.Errorf("get dependency: %w", err)
	}

	dependent.DependsOn = removeString(dependent.DependsOn, dependencyID)
	dependency.Dependents = removeString(dependency.Dependents, dependentID)

	if err := s.Update(ctx, dependent); err != nil {
		return fmt.Errorf("update dependent: %w", err)
	}
	if err := s.Update(ctx, dependency); err != nil {
		return fmt.Errorf("update dependency: %w", err)
	}

	return nil
}

// AddBlock creates a blocking relationship (blockerID blocks blockedID).
func (s *FilesystemStorage) AddBlock(ctx context.Context, blockerID, blockedID string) error {
	locks, err := s.acquireLocksOrdered([]string{blockerID, blockedID})
	if err != nil {
		return fmt.Errorf("acquire locks: %w", err)
	}
	defer releaseLocks(locks)

	blocker, err := s.Get(ctx, blockerID)
	if err != nil {
		return fmt.Errorf("get blocker: %w", err)
	}

	blocked, err := s.Get(ctx, blockedID)
	if err != nil {
		return fmt.Errorf("get blocked: %w", err)
	}

	if !contains(blocker.Blocks, blockedID) {
		blocker.Blocks = append(blocker.Blocks, blockedID)
	}

	if !contains(blocked.BlockedBy, blockerID) {
		blocked.BlockedBy = append(blocked.BlockedBy, blockerID)
	}

	if err := s.Update(ctx, blocker); err != nil {
		return fmt.Errorf("update blocker: %w", err)
	}
	if err := s.Update(ctx, blocked); err != nil {
		return fmt.Errorf("update blocked: %w", err)
	}

	return nil
}

// RemoveBlock removes a blocking relationship.
func (s *FilesystemStorage) RemoveBlock(ctx context.Context, blockerID, blockedID string) error {
	locks, err := s.acquireLocksOrdered([]string{blockerID, blockedID})
	if err != nil {
		return fmt.Errorf("acquire locks: %w", err)
	}
	defer releaseLocks(locks)

	blocker, err := s.Get(ctx, blockerID)
	if err != nil {
		return fmt.Errorf("get blocker: %w", err)
	}

	blocked, err := s.Get(ctx, blockedID)
	if err != nil {
		return fmt.Errorf("get blocked: %w", err)
	}

	blocker.Blocks = removeString(blocker.Blocks, blockedID)
	blocked.BlockedBy = removeString(blocked.BlockedBy, blockerID)

	if err := s.Update(ctx, blocker); err != nil {
		return fmt.Errorf("update blocker: %w", err)
	}
	if err := s.Update(ctx, blocked); err != nil {
		return fmt.Errorf("update blocked: %w", err)
	}

	return nil
}

// SetParent sets the parent of an issue.
func (s *FilesystemStorage) SetParent(ctx context.Context, childID, parentID string) error {
	child, err := s.Get(ctx, childID)
	if err != nil {
		return fmt.Errorf("get child: %w", err)
	}

	// Collect all IDs we need to lock
	ids := []string{childID, parentID}
	if child.Parent != "" {
		ids = append(ids, child.Parent)
	}

	locks, err := s.acquireLocksOrdered(ids)
	if err != nil {
		return fmt.Errorf("acquire locks: %w", err)
	}
	defer releaseLocks(locks)

	// Refresh child after acquiring lock
	child, err = s.Get(ctx, childID)
	if err != nil {
		return fmt.Errorf("get child: %w", err)
	}

	parent, err := s.Get(ctx, parentID)
	if err != nil {
		return fmt.Errorf("get parent: %w", err)
	}

	// Remove from old parent if exists
	if child.Parent != "" && child.Parent != parentID {
		oldParent, err := s.Get(ctx, child.Parent)
		if err == nil {
			oldParent.Children = removeString(oldParent.Children, childID)
			if err := s.Update(ctx, oldParent); err != nil {
				return fmt.Errorf("update old parent: %w", err)
			}
		}
	}

	// Update child
	child.Parent = parentID

	// Update parent
	if !contains(parent.Children, childID) {
		parent.Children = append(parent.Children, childID)
	}

	if err := s.Update(ctx, child); err != nil {
		return fmt.Errorf("update child: %w", err)
	}
	if err := s.Update(ctx, parent); err != nil {
		return fmt.Errorf("update parent: %w", err)
	}

	return nil
}

// RemoveParent removes the parent relationship.
func (s *FilesystemStorage) RemoveParent(ctx context.Context, childID string) error {
	child, err := s.Get(ctx, childID)
	if err != nil {
		return fmt.Errorf("get child: %w", err)
	}

	if child.Parent == "" {
		return nil // Nothing to do
	}

	locks, err := s.acquireLocksOrdered([]string{childID, child.Parent})
	if err != nil {
		return fmt.Errorf("acquire locks: %w", err)
	}
	defer releaseLocks(locks)

	// Refresh child after acquiring lock
	child, err = s.Get(ctx, childID)
	if err != nil {
		return fmt.Errorf("get child: %w", err)
	}

	if child.Parent == "" {
		return nil
	}

	parent, err := s.Get(ctx, child.Parent)
	if err != nil {
		return fmt.Errorf("get parent: %w", err)
	}

	child.Parent = ""
	parent.Children = removeString(parent.Children, childID)

	if err := s.Update(ctx, child); err != nil {
		return fmt.Errorf("update child: %w", err)
	}
	if err := s.Update(ctx, parent); err != nil {
		return fmt.Errorf("update parent: %w", err)
	}

	return nil
}

// AddComment adds a comment to an issue.
func (s *FilesystemStorage) AddComment(ctx context.Context, issueID string, comment *storage.Comment) error {
	lock, err := s.acquireLock(issueID)
	if err != nil {
		return fmt.Errorf("acquire lock: %w", err)
	}
	defer lock.Close()

	issue, err := s.Get(ctx, issueID)
	if err != nil {
		return err
	}

	if comment.ID == "" {
		id, err := generateID()
		if err != nil {
			return fmt.Errorf("generate comment ID: %w", err)
		}
		comment.ID = "c-" + id[3:] // Strip "bd-" prefix
	}
	if comment.CreatedAt.IsZero() {
		comment.CreatedAt = time.Now()
	}

	issue.Comments = append(issue.Comments, *comment)
	issue.UpdatedAt = time.Now()

	return s.Update(ctx, issue)
}

// Doctor checks for and optionally fixes inconsistencies.
func (s *FilesystemStorage) Doctor(ctx context.Context, fix bool) ([]string, error) {
	var problems []string

	openDir := filepath.Join(s.root, "open")
	closedDir := filepath.Join(s.root, "closed")

	// Track all issue IDs found
	openIssues := make(map[string]bool)
	closedIssues := make(map[string]bool)

	// Scan open directory
	openEntries, err := s.fs.ReadDir(openDir)
	if err != nil && !os.IsNotExist(err) {
		return nil, fmt.Errorf("read open directory: %w", err)
	}

	for _, entry := range openEntries {
		name := entry.Name()

		// Check for orphaned temp files
		if strings.HasPrefix(name, ".") && strings.Contains(name, ".tmp.") {
			problem := fmt.Sprintf("orphaned temp file: open/%s", name)
			problems = append(problems, problem)
			if fix {
				s.fs.Remove(filepath.Join(openDir, name))
			}
			continue
		}

		// Skip non-JSON files (like .lock files)
		if !strings.HasSuffix(name, ".json") {
			continue
		}

		id := strings.TrimSuffix(name, ".json")
		openIssues[id] = true

		// Validate JSON
		path := filepath.Join(openDir, name)
		data, err := s.fs.ReadFile(path)
		if err != nil {
			problems = append(problems, fmt.Sprintf("unreadable file: open/%s: %v", name, err))
			continue
		}

		var issue storage.Issue
		if err := json.Unmarshal(data, &issue); err != nil {
			problems = append(problems, fmt.Sprintf("invalid JSON: open/%s: %v", name, err))
			continue
		}

		// Check status matches directory
		if issue.Status == storage.StatusClosed {
			problem := fmt.Sprintf("status mismatch: %s has status 'closed' but is in open/", id)
			problems = append(problems, problem)
			if fix {
				// Move to closed directory
				closedPath := filepath.Join(closedDir, name)
				if err := s.fs.Rename(path, closedPath); err == nil {
					delete(openIssues, id)
					closedIssues[id] = true
				}
			}
		}
	}

	// Scan closed directory
	closedEntries, err := s.fs.ReadDir(closedDir)
	if err != nil && !os.IsNotExist(err) {
		return nil, fmt.Errorf("read closed directory: %w", err)
	}

	for _, entry := range closedEntries {
		name := entry.Name()

		// Check for orphaned temp files
		if strings.HasPrefix(name, ".") && strings.Contains(name, ".tmp.") {
			problem := fmt.Sprintf("orphaned temp file: closed/%s", name)
			problems = append(problems, problem)
			if fix {
				s.fs.Remove(filepath.Join(closedDir, name))
			}
			continue
		}

		if !strings.HasSuffix(name, ".json") {
			continue
		}

		id := strings.TrimSuffix(name, ".json")
		closedIssues[id] = true

		// Validate JSON
		path := filepath.Join(closedDir, name)
		data, err := s.fs.ReadFile(path)
		if err != nil {
			problems = append(problems, fmt.Sprintf("unreadable file: closed/%s: %v", name, err))
			continue
		}

		var issue storage.Issue
		if err := json.Unmarshal(data, &issue); err != nil {
			problems = append(problems, fmt.Sprintf("invalid JSON: closed/%s: %v", name, err))
			continue
		}

		// Check status matches directory
		if issue.Status != storage.StatusClosed {
			problem := fmt.Sprintf("status mismatch: %s has status '%s' but is in closed/", id, issue.Status)
			problems = append(problems, problem)
			if fix {
				// Move to open directory
				openPath := filepath.Join(openDir, name)
				if err := s.fs.Rename(path, openPath); err == nil {
					delete(closedIssues, id)
					openIssues[id] = true
				}
			}
		}
	}

	// Check for duplicates (same ID in both directories)
	for id := range openIssues {
		if closedIssues[id] {
			problem := fmt.Sprintf("duplicate issue: %s exists in both open/ and closed/", id)
			problems = append(problems, problem)
			if fix {
				// Prefer the closed version (more recent state during Close operation)
				// Remove from open/
				openPath := filepath.Join(openDir, id+".json")
				if err := s.fs.Remove(openPath); err == nil {
					delete(openIssues, id)
				}
			}
		}
	}

	// Check for orphaned lock files
	for _, entry := range openEntries {
		name := entry.Name()
		if !strings.HasSuffix(name, ".lock") {
			continue
		}

		id := strings.TrimSuffix(name, ".lock")
		if !openIssues[id] && !closedIssues[id] {
			problem := fmt.Sprintf("orphaned lock file: open/%s", name)
			problems = append(problems, problem)
			if fix {
				s.fs.Remove(filepath.Join(openDir, name))
			}
		}
	}

	return problems, nil
}

// contains checks if a slice contains a string.
func contains(slice []string, s string) bool {
	for _, item := range slice {
		if item == s {
			return true
		}
	}
	return false
}

// removeString removes a string from a slice.
func removeString(slice []string, s string) []string {
	result := make([]string, 0, len(slice))
	for _, item := range slice {
		if item != s {
			result = append(result, item)
		}
	}
	return result
}
