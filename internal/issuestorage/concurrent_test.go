package issuestorage

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"time"
)

// ConcurrentTestSuite runs concurrent access tests against a IssueStore implementation.
// Pass a factory function that creates a fresh, initialized storage for each test.
type ConcurrentTestSuite struct {
	NewStorage func(t *testing.T) IssueStore
}

// Run executes all concurrent tests.
func (s *ConcurrentTestSuite) Run(t *testing.T) {
	t.Run("ConcurrentCreates", s.TestConcurrentCreates)
	t.Run("ConcurrentUpdatesToSameIssue", s.TestConcurrentUpdatesToSameIssue)
	t.Run("ConcurrentDependencyAddition", s.TestConcurrentDependencyAddition)
}

// TestConcurrentCreates verifies that 100 goroutines can create issues concurrently
// without generating duplicate IDs.
func (s *ConcurrentTestSuite) TestConcurrentCreates(t *testing.T) {
	store := s.NewStorage(t)
	ctx := context.Background()

	const numGoroutines = 100
	var wg sync.WaitGroup
	var createdIDs sync.Map

	// Track errors from goroutines
	errCh := make(chan error, numGoroutines)

	wg.Add(numGoroutines)
	for i := 0; i < numGoroutines; i++ {
		go func(idx int) {
			defer wg.Done()

			issue := &Issue{
				Title:       fmt.Sprintf("Concurrent Issue %d", idx),
				Description: fmt.Sprintf("Created by goroutine %d", idx),
				Status:      StatusOpen,
				Priority:    PriorityMedium,
				Type:        TypeTask,
				CreatedAt:   time.Now(),
				UpdatedAt:   time.Now(),
			}

			id, err := store.Create(ctx, issue)
			if err != nil {
				errCh <- fmt.Errorf("goroutine %d: create failed: %w", idx, err)
				return
			}

			// Check for duplicate ID using sync.Map
			if _, loaded := createdIDs.LoadOrStore(id, idx); loaded {
				errCh <- fmt.Errorf("duplicate ID detected: %s (goroutine %d)", id, idx)
				return
			}
		}(i)
	}

	wg.Wait()
	close(errCh)

	// Collect and report all errors
	var errs []error
	for err := range errCh {
		errs = append(errs, err)
	}

	if len(errs) > 0 {
		for _, err := range errs {
			t.Error(err)
		}
		t.Fatalf("concurrent creates failed with %d errors", len(errs))
	}

	// Verify exactly 100 unique IDs were created
	count := 0
	createdIDs.Range(func(_, _ any) bool {
		count++
		return true
	})

	if count != numGoroutines {
		t.Errorf("expected %d unique IDs, got %d", numGoroutines, count)
	}

	// Verify all issues can be retrieved
	createdIDs.Range(func(key, _ any) bool {
		id := key.(string)
		issue, err := store.Get(ctx, id)
		if err != nil {
			t.Errorf("failed to retrieve issue %s: %v", id, err)
			return true
		}
		if issue.ID != id {
			t.Errorf("retrieved issue has wrong ID: expected %s, got %s", id, issue.ID)
		}
		return true
	})
}

// TestConcurrentUpdatesToSameIssue verifies that 50 goroutines can update the
// same issue concurrently without corrupting its data.
func (s *ConcurrentTestSuite) TestConcurrentUpdatesToSameIssue(t *testing.T) {
	store := s.NewStorage(t)
	ctx := context.Background()

	// Create the issue to be updated
	issue := &Issue{
		Title:       "Concurrent Update Target",
		Description: "Initial description",
		Status:      StatusOpen,
		Priority:    PriorityMedium,
		Type:        TypeTask,
		Labels:      []string{},
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
	}

	id, err := store.Create(ctx, issue)
	if err != nil {
		t.Fatalf("failed to create target issue: %v", err)
	}

	const numGoroutines = 50
	var wg sync.WaitGroup

	// Track which goroutines successfully updated
	var successfulUpdates sync.Map
	errCh := make(chan error, numGoroutines)

	wg.Add(numGoroutines)
	for i := 0; i < numGoroutines; i++ {
		go func(idx int) {
			defer wg.Done()

			// Each goroutine tries to add a unique label
			label := fmt.Sprintf("label-%d", idx)

			// Get current state, add label, update
			// This is intentionally racy to test the storage's handling
			current, err := store.Get(ctx, id)
			if err != nil {
				errCh <- fmt.Errorf("goroutine %d: get failed: %w", idx, err)
				return
			}

			current.Labels = append(current.Labels, label)
			current.UpdatedAt = time.Now()

			if err := store.Update(ctx, current); err != nil {
				errCh <- fmt.Errorf("goroutine %d: update failed: %w", idx, err)
				return
			}

			successfulUpdates.Store(idx, true)
		}(i)
	}

	wg.Wait()
	close(errCh)

	// Collect errors - some may be expected due to concurrent modification
	var errs []error
	for err := range errCh {
		errs = append(errs, err)
	}

	// Log errors but don't fail on them - the key test is data integrity
	if len(errs) > 0 {
		t.Logf("concurrent updates had %d errors (may be expected)", len(errs))
		for _, err := range errs {
			t.Logf("  %v", err)
		}
	}

	// Verify the issue is not corrupted - should still be retrievable
	// and have valid data
	final, err := store.Get(ctx, id)
	if err != nil {
		t.Fatalf("failed to retrieve issue after concurrent updates: %v", err)
	}

	// Basic integrity checks
	if final.ID != id {
		t.Errorf("issue ID corrupted: expected %s, got %s", id, final.ID)
	}

	if final.Title != "Concurrent Update Target" {
		t.Errorf("issue title corrupted: expected 'Concurrent Update Target', got %q", final.Title)
	}

	if final.Status != StatusOpen {
		t.Errorf("issue status corrupted: expected %s, got %s", StatusOpen, final.Status)
	}

	if final.Type != TypeTask {
		t.Errorf("issue type corrupted: expected %s, got %s", TypeTask, final.Type)
	}

	// Labels should be present (exact count may vary due to races)
	t.Logf("final issue has %d labels after %d concurrent update attempts",
		len(final.Labels), numGoroutines)
}

// TestConcurrentDependencyAddition verifies that many goroutines can add
// dependencies concurrently without deadlocking. Uses a 5-second timeout.
func (s *ConcurrentTestSuite) TestConcurrentDependencyAddition(t *testing.T) {
	store := s.NewStorage(t)

	// Use a context with 5-second timeout to detect deadlocks
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Create a pool of issues to work with
	const numIssues = 20
	issueIDs := make([]string, numIssues)

	for i := 0; i < numIssues; i++ {
		issue := &Issue{
			Title:       fmt.Sprintf("Dep Test Issue %d", i),
			Description: fmt.Sprintf("Issue %d for dependency testing", i),
			Status:      StatusOpen,
			Priority:    PriorityMedium,
			Type:        TypeTask,
			CreatedAt:   time.Now(),
			UpdatedAt:   time.Now(),
		}

		id, err := store.Create(ctx, issue)
		if err != nil {
			t.Fatalf("failed to create test issue %d: %v", i, err)
		}
		issueIDs[i] = id
	}

	// Many goroutines will try to add dependencies between random pairs
	const numGoroutines = 50
	var wg sync.WaitGroup

	errCh := make(chan error, numGoroutines)
	successCh := make(chan struct{}, numGoroutines)

	wg.Add(numGoroutines)
	for i := 0; i < numGoroutines; i++ {
		go func(idx int) {
			defer wg.Done()

			// Pick two different issues based on index to create varied patterns
			from := issueIDs[idx%numIssues]
			to := issueIDs[(idx+1)%numIssues]

			// Skip if same issue
			if from == to {
				successCh <- struct{}{}
				return
			}

			err := store.AddDependency(ctx, from, to, DepTypeBlocks)
			if err != nil {
				// ErrCycle is acceptable - some dependencies may create cycles
				if err == ErrCycle {
					successCh <- struct{}{}
					return
				}
				// Context deadline exceeded means deadlock
				if ctx.Err() != nil {
					errCh <- fmt.Errorf("goroutine %d: deadlock detected (context timeout): %w", idx, err)
					return
				}
				errCh <- fmt.Errorf("goroutine %d: add dependency failed: %w", idx, err)
				return
			}

			successCh <- struct{}{}
		}(i)
	}

	// Wait for completion or timeout
	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		// All goroutines completed
	case <-ctx.Done():
		t.Fatal("test timed out - possible deadlock detected")
	}

	close(errCh)
	close(successCh)

	// Count successes and failures
	var errs []error
	for err := range errCh {
		errs = append(errs, err)
	}

	successCount := 0
	for range successCh {
		successCount++
	}

	if len(errs) > 0 {
		for _, err := range errs {
			t.Error(err)
		}
		t.Errorf("concurrent dependency addition failed with %d errors", len(errs))
	}

	t.Logf("concurrent dependency additions: %d successful, %d failed", successCount, len(errs))

	// Verify dependency relationships are consistent
	// For each issue, check that its dependencies are symmetric
	for _, id := range issueIDs {
		issue, err := store.Get(ctx, id)
		if err != nil {
			t.Errorf("failed to retrieve issue %s for consistency check: %v", id, err)
			continue
		}

		// Check that for each dependency, the target has this issue in dependents
		for _, depID := range issue.DependencyIDs(nil) {
			dep, err := store.Get(ctx, depID)
			if err != nil {
				t.Errorf("failed to retrieve dependency %s: %v", depID, err)
				continue
			}

			if !dep.HasDependent(id) {
				t.Errorf("asymmetric dependency: %s depends on %s, but %s doesn't list %s in dependents",
					id, depID, depID, id)
			}
		}
	}
}
