package cmd

import (
	"bytes"
	"context"
	"encoding/json"
	"strings"
	"testing"

	"beads-lite/internal/storage"
)

func TestDepAddBasic(t *testing.T) {
	app, store := setupTestApp(t)
	out := app.Out.(*bytes.Buffer)

	// Create two issues
	issueA := &storage.Issue{
		Title:    "Issue A",
		Status:   storage.StatusOpen,
		Priority: storage.PriorityMedium,
		Type:     storage.TypeTask,
	}
	idA, err := store.Create(context.Background(), issueA)
	if err != nil {
		t.Fatalf("failed to create issue A: %v", err)
	}

	issueB := &storage.Issue{
		Title:    "Issue B",
		Status:   storage.StatusOpen,
		Priority: storage.PriorityMedium,
		Type:     storage.TypeTask,
	}
	idB, err := store.Create(context.Background(), issueB)
	if err != nil {
		t.Fatalf("failed to create issue B: %v", err)
	}

	// Add dependency: A depends on B
	cmd := newDepAddCmd(NewTestProvider(app))
	cmd.SetArgs([]string{idA, idB})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("dep add failed: %v", err)
	}

	output := out.String()
	if !strings.Contains(output, "Added dependency") {
		t.Errorf("expected output to contain 'Added dependency', got %q", output)
	}

	// Verify the dependency was created
	gotA, err := store.Get(context.Background(), idA)
	if err != nil {
		t.Fatalf("failed to get issue A: %v", err)
	}
	if !gotA.HasDependency(idB) {
		t.Errorf("expected A.Dependencies to contain B; got %v", gotA.Dependencies)
	}

	gotB, err := store.Get(context.Background(), idB)
	if err != nil {
		t.Fatalf("failed to get issue B: %v", err)
	}
	if !gotB.HasDependent(idA) {
		t.Errorf("expected B.Dependents to contain A; got %v", gotB.Dependents)
	}
}

func TestDepAddCycle(t *testing.T) {
	app, store := setupTestApp(t)

	// Create two issues
	issueA := &storage.Issue{
		Title:    "Issue A",
		Status:   storage.StatusOpen,
		Priority: storage.PriorityMedium,
		Type:     storage.TypeTask,
	}
	idA, _ := store.Create(context.Background(), issueA)

	issueB := &storage.Issue{
		Title:    "Issue B",
		Status:   storage.StatusOpen,
		Priority: storage.PriorityMedium,
		Type:     storage.TypeTask,
	}
	idB, _ := store.Create(context.Background(), issueB)

	// Add A depends on B
	if err := store.AddDependency(context.Background(), idA, idB, storage.DepTypeBlocks); err != nil {
		t.Fatalf("failed to add initial dependency: %v", err)
	}

	// Try to add B depends on A (should fail - cycle)
	cmd := newDepAddCmd(NewTestProvider(app))
	cmd.SetArgs([]string{idB, idA})
	err := cmd.Execute()
	if err == nil {
		t.Error("expected error for cycle, got nil")
	}
	if !strings.Contains(err.Error(), "cycle") {
		t.Errorf("expected error to mention 'cycle', got %v", err)
	}
}

func TestDepAddJSON(t *testing.T) {
	app, store := setupTestApp(t)
	app.JSON = true
	out := app.Out.(*bytes.Buffer)

	issueA := &storage.Issue{
		Title:    "Issue A",
		Status:   storage.StatusOpen,
		Priority: storage.PriorityMedium,
		Type:     storage.TypeTask,
	}
	idA, _ := store.Create(context.Background(), issueA)

	issueB := &storage.Issue{
		Title:    "Issue B",
		Status:   storage.StatusOpen,
		Priority: storage.PriorityMedium,
		Type:     storage.TypeTask,
	}
	idB, _ := store.Create(context.Background(), issueB)

	cmd := newDepAddCmd(NewTestProvider(app))
	cmd.SetArgs([]string{idA, idB})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("dep add failed: %v", err)
	}

	var result map[string]string
	if err := json.Unmarshal(out.Bytes(), &result); err != nil {
		t.Fatalf("failed to parse JSON: %v", err)
	}

	if result["issue_id"] != idA {
		t.Errorf("expected issue_id %q, got %q", idA, result["issue_id"])
	}
	if result["depends_on_id"] != idB {
		t.Errorf("expected depends_on_id %q, got %q", idB, result["depends_on_id"])
	}
	if result["status"] != "added" {
		t.Errorf("expected status 'added', got %q", result["status"])
	}
}

func TestDepRemoveBasic(t *testing.T) {
	app, store := setupTestApp(t)
	out := app.Out.(*bytes.Buffer)

	// Create two issues with a dependency
	issueA := &storage.Issue{
		Title:    "Issue A",
		Status:   storage.StatusOpen,
		Priority: storage.PriorityMedium,
		Type:     storage.TypeTask,
	}
	idA, _ := store.Create(context.Background(), issueA)

	issueB := &storage.Issue{
		Title:    "Issue B",
		Status:   storage.StatusOpen,
		Priority: storage.PriorityMedium,
		Type:     storage.TypeTask,
	}
	idB, _ := store.Create(context.Background(), issueB)

	// Add dependency
	if err := store.AddDependency(context.Background(), idA, idB, storage.DepTypeBlocks); err != nil {
		t.Fatalf("failed to add dependency: %v", err)
	}

	// Remove dependency
	cmd := newDepRemoveCmd(NewTestProvider(app))
	cmd.SetArgs([]string{idA, idB})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("dep remove failed: %v", err)
	}

	output := out.String()
	if !strings.Contains(output, "Removed dependency") {
		t.Errorf("expected output to contain 'Removed dependency', got %q", output)
	}

	// Verify the dependency was removed
	gotA, _ := store.Get(context.Background(), idA)
	if gotA.HasDependency(idB) {
		t.Errorf("expected A.Dependencies to NOT contain B; got %v", gotA.Dependencies)
	}

	gotB, _ := store.Get(context.Background(), idB)
	if gotB.HasDependent(idA) {
		t.Errorf("expected B.Dependents to NOT contain A; got %v", gotB.Dependents)
	}
}

func TestDepListBasic(t *testing.T) {
	app, store := setupTestApp(t)
	out := app.Out.(*bytes.Buffer)

	// Create three issues with dependencies
	issueA := &storage.Issue{
		Title:    "Issue A",
		Status:   storage.StatusOpen,
		Priority: storage.PriorityMedium,
		Type:     storage.TypeTask,
	}
	idA, _ := store.Create(context.Background(), issueA)

	issueB := &storage.Issue{
		Title:    "Issue B",
		Status:   storage.StatusOpen,
		Priority: storage.PriorityMedium,
		Type:     storage.TypeTask,
	}
	idB, _ := store.Create(context.Background(), issueB)

	issueC := &storage.Issue{
		Title:    "Issue C",
		Status:   storage.StatusOpen,
		Priority: storage.PriorityMedium,
		Type:     storage.TypeTask,
	}
	idC, _ := store.Create(context.Background(), issueC)

	// A depends on B, C depends on A
	store.AddDependency(context.Background(), idA, idB, storage.DepTypeBlocks)
	store.AddDependency(context.Background(), idC, idA, storage.DepTypeBlocks)

	cmd := newDepListCmd(NewTestProvider(app))
	cmd.SetArgs([]string{idA})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("dep list failed: %v", err)
	}

	output := out.String()
	// Should show B as dependency
	if !strings.Contains(output, idB) {
		t.Errorf("expected output to contain dependency %s, got %q", idB, output)
	}
	// Should show C as dependent
	if !strings.Contains(output, idC) {
		t.Errorf("expected output to contain dependent %s, got %q", idC, output)
	}
}

func TestDepListJSON(t *testing.T) {
	app, store := setupTestApp(t)
	app.JSON = true
	out := app.Out.(*bytes.Buffer)

	issueA := &storage.Issue{
		Title:    "Issue A",
		Status:   storage.StatusOpen,
		Priority: storage.PriorityMedium,
		Type:     storage.TypeTask,
	}
	idA, _ := store.Create(context.Background(), issueA)

	issueB := &storage.Issue{
		Title:    "Issue B",
		Status:   storage.StatusOpen,
		Priority: storage.PriorityMedium,
		Type:     storage.TypeTask,
	}
	idB, _ := store.Create(context.Background(), issueB)

	store.AddDependency(context.Background(), idA, idB, storage.DepTypeBlocks)

	cmd := newDepListCmd(NewTestProvider(app))
	cmd.SetArgs([]string{idA})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("dep list failed: %v", err)
	}

	// dep list now returns an array of enriched dependencies
	var result []map[string]interface{}
	if err := json.Unmarshal(out.Bytes(), &result); err != nil {
		t.Fatalf("failed to parse JSON: %v", err)
	}

	if len(result) != 1 {
		t.Fatalf("expected 1 dependency, got %d", len(result))
	}
	depObj := result[0]
	if depObj["id"] != idB {
		t.Errorf("expected dependency id %q, got %q", idB, depObj["id"])
	}
	if depObj["dependency_type"] != "blocks" {
		t.Errorf("expected dependency_type %q, got %q", "blocks", depObj["dependency_type"])
	}
}

func TestDepListTree(t *testing.T) {
	app, store := setupTestApp(t)
	out := app.Out.(*bytes.Buffer)

	// Create chain: A depends on B, B depends on C
	issueA := &storage.Issue{
		Title:    "Issue A",
		Status:   storage.StatusOpen,
		Priority: storage.PriorityMedium,
		Type:     storage.TypeTask,
	}
	idA, _ := store.Create(context.Background(), issueA)

	issueB := &storage.Issue{
		Title:    "Issue B",
		Status:   storage.StatusOpen,
		Priority: storage.PriorityMedium,
		Type:     storage.TypeTask,
	}
	idB, _ := store.Create(context.Background(), issueB)

	issueC := &storage.Issue{
		Title:    "Issue C",
		Status:   storage.StatusClosed,
		Priority: storage.PriorityMedium,
		Type:     storage.TypeTask,
	}
	idC, _ := store.Create(context.Background(), issueC)

	store.AddDependency(context.Background(), idA, idB, storage.DepTypeBlocks)
	store.AddDependency(context.Background(), idB, idC, storage.DepTypeBlocks)

	cmd := newDepListCmd(NewTestProvider(app))
	cmd.SetArgs([]string{idA, "--tree"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("dep list --tree failed: %v", err)
	}

	output := out.String()
	// Should show tree structure with B and C
	if !strings.Contains(output, "Dependency Tree") {
		t.Errorf("expected output to contain 'Dependency Tree', got %q", output)
	}
	if !strings.Contains(output, idB) {
		t.Errorf("expected output to contain %s, got %q", idB, output)
	}
	if !strings.Contains(output, idC) {
		t.Errorf("expected output to contain %s, got %q", idC, output)
	}
}

func TestDepNonExistent(t *testing.T) {
	app, store := setupTestApp(t)

	issueA := &storage.Issue{
		Title:    "Issue A",
		Status:   storage.StatusOpen,
		Priority: storage.PriorityMedium,
		Type:     storage.TypeTask,
	}
	idA, _ := store.Create(context.Background(), issueA)

	// Try to add dependency with non-existent issue
	cmd := newDepAddCmd(NewTestProvider(app))
	cmd.SetArgs([]string{idA, "bd-nonexistent"})
	err := cmd.Execute()
	if err == nil {
		t.Error("expected error for non-existent issue")
	}
}

func TestDepPrefixMatching(t *testing.T) {
	app, store := setupTestApp(t)

	issueA := &storage.Issue{
		Title:    "Issue A",
		Status:   storage.StatusOpen,
		Priority: storage.PriorityMedium,
		Type:     storage.TypeTask,
	}
	idA, _ := store.Create(context.Background(), issueA)

	issueB := &storage.Issue{
		Title:    "Issue B",
		Status:   storage.StatusOpen,
		Priority: storage.PriorityMedium,
		Type:     storage.TypeTask,
	}
	idB, _ := store.Create(context.Background(), issueB)

	// Use partial IDs (first 6 chars should be unique enough in a test)
	shortA := idA[:6]
	shortB := idB[:6]

	cmd := newDepAddCmd(NewTestProvider(app))
	cmd.SetArgs([]string{shortA, shortB})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("dep add with prefix matching failed: %v", err)
	}

	// Verify the dependency was created with full IDs
	gotA, _ := store.Get(context.Background(), idA)
	if !gotA.HasDependency(idB) {
		t.Errorf("expected A.Dependencies to contain B; got %v", gotA.Dependencies)
	}
}
