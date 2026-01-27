package filesystem

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"beads2/storage"
)

func TestDoctorOrphanedLockFiles(t *testing.T) {
	dir := t.TempDir()
	fs := New(dir)
	ctx := context.Background()

	if err := fs.Init(ctx); err != nil {
		t.Fatalf("Init failed: %v", err)
	}

	// Create an orphaned lock file (no corresponding .json)
	lockPath := filepath.Join(dir, "open", "orphan.lock")
	if err := os.WriteFile(lockPath, []byte{}, 0644); err != nil {
		t.Fatalf("Failed to create orphan lock: %v", err)
	}

	// Doctor should detect the problem
	problems, err := fs.Doctor(ctx, false)
	if err != nil {
		t.Fatalf("Doctor failed: %v", err)
	}
	if len(problems) != 1 {
		t.Fatalf("Expected 1 problem, got %d: %v", len(problems), problems)
	}
	if !strings.Contains(problems[0], "orphaned lock file") {
		t.Errorf("Expected 'orphaned lock file' in problem: %s", problems[0])
	}

	// Doctor should fix when fix=true
	problems, err = fs.Doctor(ctx, true)
	if err != nil {
		t.Fatalf("Doctor fix failed: %v", err)
	}
	if len(problems) != 1 {
		t.Fatalf("Expected 1 problem reported during fix, got %d", len(problems))
	}

	// Verify fix - the lock file should be gone
	if _, err := os.Stat(lockPath); !os.IsNotExist(err) {
		t.Error("Lock file should have been removed")
	}

	// Doctor should now report no problems
	problems, err = fs.Doctor(ctx, false)
	if err != nil {
		t.Fatalf("Doctor after fix failed: %v", err)
	}
	if len(problems) != 0 {
		t.Errorf("Expected 0 problems after fix, got %d: %v", len(problems), problems)
	}
}

func TestDoctorOrphanedTempFiles(t *testing.T) {
	dir := t.TempDir()
	fs := New(dir)
	ctx := context.Background()

	if err := fs.Init(ctx); err != nil {
		t.Fatalf("Init failed: %v", err)
	}

	// Create orphaned temp files in both directories
	openTmp := filepath.Join(dir, "open", "test.json.tmp.abc123")
	closedTmp := filepath.Join(dir, "closed", "test.json.tmp.def456")

	os.WriteFile(openTmp, []byte("{}"), 0644)
	os.WriteFile(closedTmp, []byte("{}"), 0644)

	// Doctor should detect both problems
	problems, err := fs.Doctor(ctx, false)
	if err != nil {
		t.Fatalf("Doctor failed: %v", err)
	}
	if len(problems) != 2 {
		t.Fatalf("Expected 2 problems, got %d: %v", len(problems), problems)
	}

	// Doctor should fix when fix=true
	fs.Doctor(ctx, true)

	// Verify fix
	if _, err := os.Stat(openTmp); !os.IsNotExist(err) {
		t.Error("Open temp file should have been removed")
	}
	if _, err := os.Stat(closedTmp); !os.IsNotExist(err) {
		t.Error("Closed temp file should have been removed")
	}
}

func TestDoctorDuplicateIssue(t *testing.T) {
	dir := t.TempDir()
	fs := New(dir)
	ctx := context.Background()

	if err := fs.Init(ctx); err != nil {
		t.Fatalf("Init failed: %v", err)
	}

	// Create an issue normally
	issue := &storage.Issue{
		Title:    "Test Issue",
		Status:   storage.StatusOpen,
		Priority: storage.PriorityMedium,
		Type:     storage.TypeTask,
	}
	id, err := fs.Create(ctx, issue)
	if err != nil {
		t.Fatalf("Create failed: %v", err)
	}

	// Read the issue data
	openPath := filepath.Join(dir, "open", id+".json")
	data, err := os.ReadFile(openPath)
	if err != nil {
		t.Fatalf("Read failed: %v", err)
	}

	// Modify the issue to have closed status and write to closed/
	var closedIssue storage.Issue
	json.Unmarshal(data, &closedIssue)
	closedIssue.Status = storage.StatusClosed

	closedData, _ := json.Marshal(closedIssue)
	closedPath := filepath.Join(dir, "closed", id+".json")
	os.WriteFile(closedPath, closedData, 0644)

	// Doctor should detect the duplicate
	problems, err := fs.Doctor(ctx, false)
	if err != nil {
		t.Fatalf("Doctor failed: %v", err)
	}
	found := false
	for _, p := range problems {
		if strings.Contains(p, "duplicate issue") {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("Expected 'duplicate issue' problem, got: %v", problems)
	}

	// Fix should remove the inconsistent copy (keep closed since status=closed)
	fs.Doctor(ctx, true)

	// Verify: open/ should be gone, closed/ should remain
	if _, err := os.Stat(openPath); !os.IsNotExist(err) {
		t.Error("Open file should have been removed for duplicate with closed status")
	}
	if _, err := os.Stat(closedPath); os.IsNotExist(err) {
		t.Error("Closed file should remain")
	}
}

func TestDoctorStatusLocationMismatch(t *testing.T) {
	dir := t.TempDir()
	fs := New(dir)
	ctx := context.Background()

	if err := fs.Init(ctx); err != nil {
		t.Fatalf("Init failed: %v", err)
	}

	// Create an issue with status=closed in open/ directory
	issue := storage.Issue{
		ID:       "bd-test",
		Title:    "Misplaced Closed",
		Status:   storage.StatusClosed,
		Priority: storage.PriorityMedium,
		Type:     storage.TypeTask,
	}
	data, _ := json.Marshal(issue)
	openPath := filepath.Join(dir, "open", "bd-test.json")
	os.WriteFile(openPath, data, 0644)

	// Doctor should detect the mismatch
	problems, err := fs.Doctor(ctx, false)
	if err != nil {
		t.Fatalf("Doctor failed: %v", err)
	}
	found := false
	for _, p := range problems {
		if strings.Contains(p, "status mismatch") {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("Expected 'status mismatch' problem, got: %v", problems)
	}

	// Fix should move to closed/
	fs.Doctor(ctx, true)

	// Verify: should be in closed/, not open/
	if _, err := os.Stat(openPath); !os.IsNotExist(err) {
		t.Error("File should have been moved from open/")
	}
	closedPath := filepath.Join(dir, "closed", "bd-test.json")
	if _, err := os.Stat(closedPath); os.IsNotExist(err) {
		t.Error("File should exist in closed/")
	}
}

func TestDoctorBrokenDependencyReference(t *testing.T) {
	dir := t.TempDir()
	fs := New(dir)
	ctx := context.Background()

	if err := fs.Init(ctx); err != nil {
		t.Fatalf("Init failed: %v", err)
	}

	// Create an issue with a dependency on a non-existent issue
	issue := storage.Issue{
		ID:        "bd-test",
		Title:     "Has Broken Dep",
		Status:    storage.StatusOpen,
		Priority:  storage.PriorityMedium,
		Type:      storage.TypeTask,
		DependsOn: []string{"bd-nonexistent"},
	}
	data, _ := json.Marshal(issue)
	issuePath := filepath.Join(dir, "open", "bd-test.json")
	os.WriteFile(issuePath, data, 0644)

	// Doctor should detect the broken reference
	problems, err := fs.Doctor(ctx, false)
	if err != nil {
		t.Fatalf("Doctor failed: %v", err)
	}
	found := false
	for _, p := range problems {
		if strings.Contains(p, "broken dependency") {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("Expected 'broken dependency' problem, got: %v", problems)
	}

	// Fix should remove the broken reference
	fs.Doctor(ctx, true)

	// Verify: DependsOn should be empty
	got, err := fs.Get(ctx, "bd-test")
	if err != nil {
		t.Fatalf("Get after fix failed: %v", err)
	}
	if len(got.DependsOn) != 0 {
		t.Errorf("Expected empty DependsOn after fix, got: %v", got.DependsOn)
	}
}

func TestDoctorAsymmetricDependency(t *testing.T) {
	dir := t.TempDir()
	fs := New(dir)
	ctx := context.Background()

	if err := fs.Init(ctx); err != nil {
		t.Fatalf("Init failed: %v", err)
	}

	// Create two issues where A depends on B but B doesn't have A as dependent
	issueA := storage.Issue{
		ID:        "bd-aaaa",
		Title:     "Issue A",
		Status:    storage.StatusOpen,
		Priority:  storage.PriorityMedium,
		Type:      storage.TypeTask,
		DependsOn: []string{"bd-bbbb"},
	}
	issueB := storage.Issue{
		ID:         "bd-bbbb",
		Title:      "Issue B",
		Status:     storage.StatusOpen,
		Priority:   storage.PriorityMedium,
		Type:       storage.TypeTask,
		Dependents: []string{}, // Missing A!
	}

	dataA, _ := json.Marshal(issueA)
	dataB, _ := json.Marshal(issueB)
	os.WriteFile(filepath.Join(dir, "open", "bd-aaaa.json"), dataA, 0644)
	os.WriteFile(filepath.Join(dir, "open", "bd-bbbb.json"), dataB, 0644)

	// Doctor should detect the asymmetry
	problems, err := fs.Doctor(ctx, false)
	if err != nil {
		t.Fatalf("Doctor failed: %v", err)
	}
	found := false
	for _, p := range problems {
		if strings.Contains(p, "asymmetric") {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("Expected 'asymmetric' problem, got: %v", problems)
	}

	// Fix should restore the symmetric relationship
	fs.Doctor(ctx, true)

	// Verify: B should now have A as dependent
	gotB, err := fs.Get(ctx, "bd-bbbb")
	if err != nil {
		t.Fatalf("Get B after fix failed: %v", err)
	}
	found = false
	for _, dep := range gotB.Dependents {
		if dep == "bd-aaaa" {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("Expected B.Dependents to contain A after fix, got: %v", gotB.Dependents)
	}
}

func TestDoctorBrokenParentReference(t *testing.T) {
	dir := t.TempDir()
	fs := New(dir)
	ctx := context.Background()

	if err := fs.Init(ctx); err != nil {
		t.Fatalf("Init failed: %v", err)
	}

	// Create an issue with a parent that doesn't exist
	issue := storage.Issue{
		ID:       "bd-child",
		Title:    "Orphan Child",
		Status:   storage.StatusOpen,
		Priority: storage.PriorityMedium,
		Type:     storage.TypeTask,
		Parent:   "bd-nonexistent-parent",
	}
	data, _ := json.Marshal(issue)
	os.WriteFile(filepath.Join(dir, "open", "bd-child.json"), data, 0644)

	// Doctor should detect the broken reference
	problems, err := fs.Doctor(ctx, false)
	if err != nil {
		t.Fatalf("Doctor failed: %v", err)
	}
	found := false
	for _, p := range problems {
		if strings.Contains(p, "broken parent reference") {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("Expected 'broken parent reference' problem, got: %v", problems)
	}

	// Fix should remove the broken parent reference
	fs.Doctor(ctx, true)

	// Verify: Parent should be empty
	got, err := fs.Get(ctx, "bd-child")
	if err != nil {
		t.Fatalf("Get after fix failed: %v", err)
	}
	if got.Parent != "" {
		t.Errorf("Expected empty Parent after fix, got: %s", got.Parent)
	}
}

func TestDoctorMalformedJSON(t *testing.T) {
	dir := t.TempDir()
	fs := New(dir)
	ctx := context.Background()

	if err := fs.Init(ctx); err != nil {
		t.Fatalf("Init failed: %v", err)
	}

	// Create a malformed JSON file (simulates git merge conflict)
	malformed := `{
  "id": "bd-corrupt",
  "title": "<<<<<<< HEAD",
  "title": "Branch A",
  =======
  "title": "Branch B",
  >>>>>>> feature
}`
	os.WriteFile(filepath.Join(dir, "open", "bd-corrupt.json"), []byte(malformed), 0644)

	// Doctor should detect the malformed JSON
	problems, err := fs.Doctor(ctx, false)
	if err != nil {
		t.Fatalf("Doctor failed: %v", err)
	}
	found := false
	for _, p := range problems {
		if strings.Contains(p, "malformed JSON") {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("Expected 'malformed JSON' problem, got: %v", problems)
	}

	// Note: malformed JSON cannot be auto-fixed
}

func TestDoctorCleanStorage(t *testing.T) {
	dir := t.TempDir()
	fs := New(dir)
	ctx := context.Background()

	if err := fs.Init(ctx); err != nil {
		t.Fatalf("Init failed: %v", err)
	}

	// Create some valid issues with proper relationships
	issueA := &storage.Issue{
		Title:    "Issue A",
		Status:   storage.StatusOpen,
		Priority: storage.PriorityMedium,
		Type:     storage.TypeTask,
	}
	issueB := &storage.Issue{
		Title:    "Issue B",
		Status:   storage.StatusOpen,
		Priority: storage.PriorityMedium,
		Type:     storage.TypeTask,
	}

	idA, _ := fs.Create(ctx, issueA)
	idB, _ := fs.Create(ctx, issueB)

	// Add a proper dependency
	fs.AddDependency(ctx, idA, idB)

	// Close one issue properly
	fs.Close(ctx, idB)

	// Doctor should report no problems
	problems, err := fs.Doctor(ctx, false)
	if err != nil {
		t.Fatalf("Doctor failed: %v", err)
	}
	if len(problems) != 0 {
		t.Errorf("Expected 0 problems for clean storage, got %d: %v", len(problems), problems)
	}
}

func TestDoctorAsymmetricBlocks(t *testing.T) {
	dir := t.TempDir()
	fs := New(dir)
	ctx := context.Background()

	if err := fs.Init(ctx); err != nil {
		t.Fatalf("Init failed: %v", err)
	}

	// Create two issues where A blocks B but B doesn't have A in blocked_by
	issueA := storage.Issue{
		ID:       "bd-aaaa",
		Title:    "Blocker A",
		Status:   storage.StatusOpen,
		Priority: storage.PriorityMedium,
		Type:     storage.TypeTask,
		Blocks:   []string{"bd-bbbb"},
	}
	issueB := storage.Issue{
		ID:        "bd-bbbb",
		Title:     "Blocked B",
		Status:    storage.StatusOpen,
		Priority:  storage.PriorityMedium,
		Type:      storage.TypeTask,
		BlockedBy: []string{}, // Missing A!
	}

	dataA, _ := json.Marshal(issueA)
	dataB, _ := json.Marshal(issueB)
	os.WriteFile(filepath.Join(dir, "open", "bd-aaaa.json"), dataA, 0644)
	os.WriteFile(filepath.Join(dir, "open", "bd-bbbb.json"), dataB, 0644)

	// Doctor should detect the asymmetry
	problems, err := fs.Doctor(ctx, false)
	if err != nil {
		t.Fatalf("Doctor failed: %v", err)
	}
	found := false
	for _, p := range problems {
		if strings.Contains(p, "asymmetric blocks") {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("Expected 'asymmetric blocks' problem, got: %v", problems)
	}

	// Fix should restore the symmetric relationship
	fs.Doctor(ctx, true)

	// Verify: B should now have A in blocked_by
	gotB, err := fs.Get(ctx, "bd-bbbb")
	if err != nil {
		t.Fatalf("Get B after fix failed: %v", err)
	}
	found = false
	for _, blocker := range gotB.BlockedBy {
		if blocker == "bd-aaaa" {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("Expected B.BlockedBy to contain A after fix, got: %v", gotB.BlockedBy)
	}
}

func TestDoctorAsymmetricParentChild(t *testing.T) {
	dir := t.TempDir()
	fs := New(dir)
	ctx := context.Background()

	if err := fs.Init(ctx); err != nil {
		t.Fatalf("Init failed: %v", err)
	}

	// Create two issues where child has parent but parent doesn't list child
	parent := storage.Issue{
		ID:       "bd-parent",
		Title:    "Parent",
		Status:   storage.StatusOpen,
		Priority: storage.PriorityMedium,
		Type:     storage.TypeEpic,
		Children: []string{}, // Missing child!
	}
	child := storage.Issue{
		ID:       "bd-child",
		Title:    "Child",
		Status:   storage.StatusOpen,
		Priority: storage.PriorityMedium,
		Type:     storage.TypeTask,
		Parent:   "bd-parent",
	}

	dataP, _ := json.Marshal(parent)
	dataC, _ := json.Marshal(child)
	os.WriteFile(filepath.Join(dir, "open", "bd-parent.json"), dataP, 0644)
	os.WriteFile(filepath.Join(dir, "open", "bd-child.json"), dataC, 0644)

	// Doctor should detect the asymmetry
	problems, err := fs.Doctor(ctx, false)
	if err != nil {
		t.Fatalf("Doctor failed: %v", err)
	}
	found := false
	for _, p := range problems {
		if strings.Contains(p, "asymmetric parent/child") {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("Expected 'asymmetric parent/child' problem, got: %v", problems)
	}

	// Fix should restore the symmetric relationship
	fs.Doctor(ctx, true)

	// Verify: parent should now have child in Children
	gotParent, err := fs.Get(ctx, "bd-parent")
	if err != nil {
		t.Fatalf("Get parent after fix failed: %v", err)
	}
	found = false
	for _, c := range gotParent.Children {
		if c == "bd-child" {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("Expected Parent.Children to contain child after fix, got: %v", gotParent.Children)
	}
}
