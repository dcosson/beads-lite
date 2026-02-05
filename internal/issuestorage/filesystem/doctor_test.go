package filesystem

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"beads-lite/internal/issuestorage"
)

func TestDoctorOrphanedLockFiles(t *testing.T) {
	dir := t.TempDir()
	fs := New(dir, "bd-")
	ctx := context.Background()

	if err := fs.Init(ctx); err != nil {
		t.Fatalf("Init failed: %v", err)
	}

	// Create an orphaned lock file (no corresponding .json)
	lockPath := filepath.Join(dir, DataDirName, "open", "orphan.lock")
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
	fs := New(dir, "bd-")
	ctx := context.Background()

	if err := fs.Init(ctx); err != nil {
		t.Fatalf("Init failed: %v", err)
	}

	// Create orphaned temp files in both directories
	openTmp := filepath.Join(dir, DataDirName, "open", "test.json.tmp.abc123")
	closedTmp := filepath.Join(dir, DataDirName, "closed", "test.json.tmp.def456")

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
	fs := New(dir, "bd-")
	ctx := context.Background()

	if err := fs.Init(ctx); err != nil {
		t.Fatalf("Init failed: %v", err)
	}

	// Create an issue normally
	issue := &issuestorage.Issue{
		Title:    "Test Issue",
		Status:   issuestorage.StatusOpen,
		Priority: issuestorage.PriorityMedium,
		Type:     issuestorage.TypeTask,
	}
	id, err := fs.Create(ctx, issue)
	if err != nil {
		t.Fatalf("Create failed: %v", err)
	}

	// Read the issue data
	openPath := filepath.Join(dir, DataDirName, "open", id+".json")
	data, err := os.ReadFile(openPath)
	if err != nil {
		t.Fatalf("Read failed: %v", err)
	}

	// Modify the issue to have closed status and write to closed/
	var closedIssue issuestorage.Issue
	json.Unmarshal(data, &closedIssue)
	closedIssue.Status = issuestorage.StatusClosed

	closedData, _ := json.Marshal(closedIssue)
	closedPath := filepath.Join(dir, DataDirName, "closed", id+".json")
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
	fs := New(dir, "bd-")
	ctx := context.Background()

	if err := fs.Init(ctx); err != nil {
		t.Fatalf("Init failed: %v", err)
	}

	// Create an issue with status=closed in open/ directory
	issue := issuestorage.Issue{
		ID:       "bd-test",
		Title:    "Misplaced Closed",
		Status:   issuestorage.StatusClosed,
		Priority: issuestorage.PriorityMedium,
		Type:     issuestorage.TypeTask,
	}
	data, _ := json.Marshal(issue)
	openPath := filepath.Join(dir, DataDirName, "open", "bd-test.json")
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
	closedPath := filepath.Join(dir, DataDirName, "closed", "bd-test.json")
	if _, err := os.Stat(closedPath); os.IsNotExist(err) {
		t.Error("File should exist in closed/")
	}
}

func TestDoctorBrokenDependencyReference(t *testing.T) {
	dir := t.TempDir()
	fs := New(dir, "bd-")
	ctx := context.Background()

	if err := fs.Init(ctx); err != nil {
		t.Fatalf("Init failed: %v", err)
	}

	// Create an issue with a dependency on a non-existent issue
	issue := issuestorage.Issue{
		ID:           "bd-test",
		Title:        "Has Broken Dep",
		Status:       issuestorage.StatusOpen,
		Priority:     issuestorage.PriorityMedium,
		Type:         issuestorage.TypeTask,
		Dependencies: []issuestorage.Dependency{{ID: "bd-nonexistent", Type: issuestorage.DepTypeBlocks}},
	}
	data, _ := json.Marshal(issue)
	issuePath := filepath.Join(dir, DataDirName, "open", "bd-test.json")
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

	// Verify: Dependencies should be empty
	got, err := fs.Get(ctx, "bd-test")
	if err != nil {
		t.Fatalf("Get after fix failed: %v", err)
	}
	if len(got.Dependencies) != 0 {
		t.Errorf("Expected empty Dependencies after fix, got: %v", got.Dependencies)
	}
}

func TestDoctorAsymmetricDependency(t *testing.T) {
	dir := t.TempDir()
	fs := New(dir, "bd-")
	ctx := context.Background()

	if err := fs.Init(ctx); err != nil {
		t.Fatalf("Init failed: %v", err)
	}

	// Create two issues where A depends on B but B doesn't have A as dependent
	issueA := issuestorage.Issue{
		ID:           "bd-aaaa",
		Title:        "Issue A",
		Status:       issuestorage.StatusOpen,
		Priority:     issuestorage.PriorityMedium,
		Type:         issuestorage.TypeTask,
		Dependencies: []issuestorage.Dependency{{ID: "bd-bbbb", Type: issuestorage.DepTypeBlocks}},
	}
	issueB := issuestorage.Issue{
		ID:         "bd-bbbb",
		Title:      "Issue B",
		Status:     issuestorage.StatusOpen,
		Priority:   issuestorage.PriorityMedium,
		Type:       issuestorage.TypeTask,
		Dependents: []issuestorage.Dependency{}, // Missing A!
	}

	dataA, _ := json.Marshal(issueA)
	dataB, _ := json.Marshal(issueB)
	os.WriteFile(filepath.Join(dir, DataDirName, "open", "bd-aaaa.json"), dataA, 0644)
	os.WriteFile(filepath.Join(dir, DataDirName, "open", "bd-bbbb.json"), dataB, 0644)

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
	if !gotB.HasDependent("bd-aaaa") {
		t.Errorf("Expected B.Dependents to contain A after fix, got: %v", gotB.Dependents)
	}
}

func TestDoctorBrokenParentReference(t *testing.T) {
	dir := t.TempDir()
	fs := New(dir, "bd-")
	ctx := context.Background()

	if err := fs.Init(ctx); err != nil {
		t.Fatalf("Init failed: %v", err)
	}

	// Create an issue with a parent that doesn't exist
	issue := issuestorage.Issue{
		ID:       "bd-child",
		Title:    "Orphan Child",
		Status:   issuestorage.StatusOpen,
		Priority: issuestorage.PriorityMedium,
		Type:     issuestorage.TypeTask,
		Parent:   "bd-nonexistent-parent",
	}
	data, _ := json.Marshal(issue)
	os.WriteFile(filepath.Join(dir, DataDirName, "open", "bd-child.json"), data, 0644)

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
	fs := New(dir, "bd-")
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
	os.WriteFile(filepath.Join(dir, DataDirName, "open", "bd-corrupt.json"), []byte(malformed), 0644)

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
	fs := New(dir, "bd-")
	ctx := context.Background()

	if err := fs.Init(ctx); err != nil {
		t.Fatalf("Init failed: %v", err)
	}

	// Create some valid issues with proper relationships
	issueA := &issuestorage.Issue{
		Title:    "Issue A",
		Status:   issuestorage.StatusOpen,
		Priority: issuestorage.PriorityMedium,
		Type:     issuestorage.TypeTask,
	}
	issueB := &issuestorage.Issue{
		Title:    "Issue B",
		Status:   issuestorage.StatusOpen,
		Priority: issuestorage.PriorityMedium,
		Type:     issuestorage.TypeTask,
	}

	idA, _ := fs.Create(ctx, issueA)
	idB, _ := fs.Create(ctx, issueB)

	// Add a proper dependency by directly modifying both sides
	fs.Modify(ctx, idA, func(i *issuestorage.Issue) error {
		i.Dependencies = append(i.Dependencies, issuestorage.Dependency{ID: idB, Type: issuestorage.DepTypeBlocks})
		return nil
	})
	fs.Modify(ctx, idB, func(i *issuestorage.Issue) error {
		i.Dependents = append(i.Dependents, issuestorage.Dependency{ID: idA, Type: issuestorage.DepTypeBlocks})
		return nil
	})

	// Close one issue properly
	fs.Modify(ctx, idB, func(i *issuestorage.Issue) error {
		i.Status = issuestorage.StatusClosed
		return nil
	})

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
	fs := New(dir, "bd-")
	ctx := context.Background()

	if err := fs.Init(ctx); err != nil {
		t.Fatalf("Init failed: %v", err)
	}

	// Create two issues where A blocks B (A has B as dependent) but B doesn't have A in dependencies
	issueA := issuestorage.Issue{
		ID:         "bd-aaaa",
		Title:      "Blocker A",
		Status:     issuestorage.StatusOpen,
		Priority:   issuestorage.PriorityMedium,
		Type:       issuestorage.TypeTask,
		Dependents: []issuestorage.Dependency{{ID: "bd-bbbb", Type: issuestorage.DepTypeBlocks}},
	}
	issueB := issuestorage.Issue{
		ID:           "bd-bbbb",
		Title:        "Blocked B",
		Status:       issuestorage.StatusOpen,
		Priority:     issuestorage.PriorityMedium,
		Type:         issuestorage.TypeTask,
		Dependencies: []issuestorage.Dependency{}, // Missing A!
	}

	dataA, _ := json.Marshal(issueA)
	dataB, _ := json.Marshal(issueB)
	os.WriteFile(filepath.Join(dir, DataDirName, "open", "bd-aaaa.json"), dataA, 0644)
	os.WriteFile(filepath.Join(dir, DataDirName, "open", "bd-bbbb.json"), dataB, 0644)

	// Doctor should detect the asymmetry
	problems, err := fs.Doctor(ctx, false)
	if err != nil {
		t.Fatalf("Doctor failed: %v", err)
	}
	found := false
	for _, p := range problems {
		if strings.Contains(p, "asymmetric dependency") {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("Expected 'asymmetric dependency' problem, got: %v", problems)
	}

	// Fix should restore the symmetric relationship
	fs.Doctor(ctx, true)

	// Verify: B should now have A in dependencies (blocked by A)
	gotB, err := fs.Get(ctx, "bd-bbbb")
	if err != nil {
		t.Fatalf("Get B after fix failed: %v", err)
	}
	if !gotB.HasDependency("bd-aaaa") {
		t.Errorf("Expected B.Dependencies to contain A after fix, got: %v", gotB.Dependencies)
	}
}

func TestDoctorAsymmetricParentChild(t *testing.T) {
	dir := t.TempDir()
	fs := New(dir, "bd-")
	ctx := context.Background()

	if err := fs.Init(ctx); err != nil {
		t.Fatalf("Init failed: %v", err)
	}

	// Create two issues where child has parent but parent doesn't list child
	parent := issuestorage.Issue{
		ID:         "bd-parent",
		Title:      "Parent",
		Status:     issuestorage.StatusOpen,
		Priority:   issuestorage.PriorityMedium,
		Type:       issuestorage.TypeEpic,
		Dependents: []issuestorage.Dependency{}, // Missing child!
	}
	child := issuestorage.Issue{
		ID:       "bd-child",
		Title:    "Child",
		Status:   issuestorage.StatusOpen,
		Priority: issuestorage.PriorityMedium,
		Type:     issuestorage.TypeTask,
		Parent:   "bd-parent",
	}

	dataP, _ := json.Marshal(parent)
	dataC, _ := json.Marshal(child)
	os.WriteFile(filepath.Join(dir, DataDirName, "open", "bd-parent.json"), dataP, 0644)
	os.WriteFile(filepath.Join(dir, DataDirName, "open", "bd-child.json"), dataC, 0644)

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

	// Verify: parent should now have child in Dependents
	gotParent, err := fs.Get(ctx, "bd-parent")
	if err != nil {
		t.Fatalf("Get parent after fix failed: %v", err)
	}
	if !gotParent.HasDependent("bd-child") {
		t.Errorf("Expected Parent.Dependents to contain child after fix, got: %v", gotParent.Dependents)
	}
}
