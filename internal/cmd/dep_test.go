package cmd

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"beads-lite/internal/issueservice"
	"beads-lite/internal/issuestorage"
	"beads-lite/internal/issuestorage/filesystem"
	"beads-lite/internal/routing"
)

func TestDepAddBasic(t *testing.T) {
	app, store := setupTestApp(t)
	out := app.Out.(*bytes.Buffer)

	// Create two issues
	issueA := &issuestorage.Issue{
		Title:    "Issue A",
		Status:   issuestorage.StatusOpen,
		Priority: issuestorage.PriorityMedium,
		Type:     issuestorage.TypeTask,
	}
	idA, err := store.Create(context.Background(), issueA)
	if err != nil {
		t.Fatalf("failed to create issue A: %v", err)
	}

	issueB := &issuestorage.Issue{
		Title:    "Issue B",
		Status:   issuestorage.StatusOpen,
		Priority: issuestorage.PriorityMedium,
		Type:     issuestorage.TypeTask,
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
	issueA := &issuestorage.Issue{
		Title:    "Issue A",
		Status:   issuestorage.StatusOpen,
		Priority: issuestorage.PriorityMedium,
		Type:     issuestorage.TypeTask,
	}
	idA, _ := store.Create(context.Background(), issueA)

	issueB := &issuestorage.Issue{
		Title:    "Issue B",
		Status:   issuestorage.StatusOpen,
		Priority: issuestorage.PriorityMedium,
		Type:     issuestorage.TypeTask,
	}
	idB, _ := store.Create(context.Background(), issueB)

	// Add A depends on B
	if err := store.AddDependency(context.Background(), idA, idB, issuestorage.DepTypeBlocks); err != nil {
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

	issueA := &issuestorage.Issue{
		Title:    "Issue A",
		Status:   issuestorage.StatusOpen,
		Priority: issuestorage.PriorityMedium,
		Type:     issuestorage.TypeTask,
	}
	idA, _ := store.Create(context.Background(), issueA)

	issueB := &issuestorage.Issue{
		Title:    "Issue B",
		Status:   issuestorage.StatusOpen,
		Priority: issuestorage.PriorityMedium,
		Type:     issuestorage.TypeTask,
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
	issueA := &issuestorage.Issue{
		Title:    "Issue A",
		Status:   issuestorage.StatusOpen,
		Priority: issuestorage.PriorityMedium,
		Type:     issuestorage.TypeTask,
	}
	idA, _ := store.Create(context.Background(), issueA)

	issueB := &issuestorage.Issue{
		Title:    "Issue B",
		Status:   issuestorage.StatusOpen,
		Priority: issuestorage.PriorityMedium,
		Type:     issuestorage.TypeTask,
	}
	idB, _ := store.Create(context.Background(), issueB)

	// Add dependency
	if err := store.AddDependency(context.Background(), idA, idB, issuestorage.DepTypeBlocks); err != nil {
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
	issueA := &issuestorage.Issue{
		Title:    "Issue A",
		Status:   issuestorage.StatusOpen,
		Priority: issuestorage.PriorityMedium,
		Type:     issuestorage.TypeTask,
	}
	idA, _ := store.Create(context.Background(), issueA)

	issueB := &issuestorage.Issue{
		Title:    "Issue B",
		Status:   issuestorage.StatusOpen,
		Priority: issuestorage.PriorityMedium,
		Type:     issuestorage.TypeTask,
	}
	idB, _ := store.Create(context.Background(), issueB)

	issueC := &issuestorage.Issue{
		Title:    "Issue C",
		Status:   issuestorage.StatusOpen,
		Priority: issuestorage.PriorityMedium,
		Type:     issuestorage.TypeTask,
	}
	idC, _ := store.Create(context.Background(), issueC)

	// A depends on B, C depends on A
	store.AddDependency(context.Background(), idA, idB, issuestorage.DepTypeBlocks)
	store.AddDependency(context.Background(), idC, idA, issuestorage.DepTypeBlocks)

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

	issueA := &issuestorage.Issue{
		Title:    "Issue A",
		Status:   issuestorage.StatusOpen,
		Priority: issuestorage.PriorityMedium,
		Type:     issuestorage.TypeTask,
	}
	idA, _ := store.Create(context.Background(), issueA)

	issueB := &issuestorage.Issue{
		Title:    "Issue B",
		Status:   issuestorage.StatusOpen,
		Priority: issuestorage.PriorityMedium,
		Type:     issuestorage.TypeTask,
	}
	idB, _ := store.Create(context.Background(), issueB)

	store.AddDependency(context.Background(), idA, idB, issuestorage.DepTypeBlocks)

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
	issueA := &issuestorage.Issue{
		Title:    "Issue A",
		Status:   issuestorage.StatusOpen,
		Priority: issuestorage.PriorityMedium,
		Type:     issuestorage.TypeTask,
	}
	idA, _ := store.Create(context.Background(), issueA)

	issueB := &issuestorage.Issue{
		Title:    "Issue B",
		Status:   issuestorage.StatusOpen,
		Priority: issuestorage.PriorityMedium,
		Type:     issuestorage.TypeTask,
	}
	idB, _ := store.Create(context.Background(), issueB)

	issueC := &issuestorage.Issue{
		Title:    "Issue C",
		Status:   issuestorage.StatusClosed,
		Priority: issuestorage.PriorityMedium,
		Type:     issuestorage.TypeTask,
	}
	idC, _ := store.Create(context.Background(), issueC)

	store.AddDependency(context.Background(), idA, idB, issuestorage.DepTypeBlocks)
	store.AddDependency(context.Background(), idB, idC, issuestorage.DepTypeBlocks)

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

	issueA := &issuestorage.Issue{
		Title:    "Issue A",
		Status:   issuestorage.StatusOpen,
		Priority: issuestorage.PriorityMedium,
		Type:     issuestorage.TypeTask,
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

	issueA := &issuestorage.Issue{
		Title:    "Issue A",
		Status:   issuestorage.StatusOpen,
		Priority: issuestorage.PriorityMedium,
		Type:     issuestorage.TypeTask,
	}
	idA, _ := store.Create(context.Background(), issueA)

	issueB := &issuestorage.Issue{
		Title:    "Issue B",
		Status:   issuestorage.StatusOpen,
		Priority: issuestorage.PriorityMedium,
		Type:     issuestorage.TypeTask,
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

// setupCrossStoreTestApp creates an App with two rigs (local "bl-" and remote "hq-")
// connected by a Router. Returns the app, local store, and remote store.
func setupCrossStoreTestApp(t *testing.T) (*App, *filesystem.FilesystemStorage, *filesystem.FilesystemStorage) {
	t.Helper()
	townRoot := t.TempDir()

	// Local rig at townRoot/local
	localRig := filepath.Join(townRoot, "local")
	localBeads := filepath.Join(localRig, ".beads")
	os.MkdirAll(filepath.Join(localBeads, "issues", "open"), 0755)
	os.MkdirAll(filepath.Join(localBeads, "issues", "closed"), 0755)
	os.MkdirAll(filepath.Join(localBeads, "issues", "ephemeral"), 0755)
	os.MkdirAll(filepath.Join(localBeads, "issues", "deleted"), 0755)
	os.WriteFile(filepath.Join(localBeads, "config.yaml"), []byte("project.name: issues\nissue_prefix: bl-\n"), 0644)

	// Remote rig at townRoot/remote
	remoteRig := filepath.Join(townRoot, "remote")
	remoteBeads := filepath.Join(remoteRig, ".beads")
	os.MkdirAll(filepath.Join(remoteBeads, "issues", "open"), 0755)
	os.MkdirAll(filepath.Join(remoteBeads, "issues", "closed"), 0755)
	os.MkdirAll(filepath.Join(remoteBeads, "issues", "ephemeral"), 0755)
	os.MkdirAll(filepath.Join(remoteBeads, "issues", "deleted"), 0755)
	os.WriteFile(filepath.Join(remoteBeads, "config.yaml"), []byte("project.name: issues\nissue_prefix: hq-\n"), 0644)

	// Write routes.jsonl at the town root
	townBeads := filepath.Join(townRoot, ".beads")
	os.MkdirAll(townBeads, 0755)
	routesContent := `{"prefix": "bl-", "path": "local"}` + "\n" + `{"prefix": "hq-", "path": "remote"}` + "\n"
	os.WriteFile(filepath.Join(townBeads, "routes.jsonl"), []byte(routesContent), 0644)

	// Create router from local rig's .beads
	router, err := routing.New(localBeads)
	if err != nil {
		t.Fatalf("failed to create router: %v", err)
	}

	localStore := filesystem.New(filepath.Join(localBeads, "issues"), "bl-")
	remoteStore := filesystem.New(filepath.Join(remoteBeads, "issues"), "hq-")
	rs := issueservice.New(router, localStore)

	app := &App{
		Storage: rs,
		Out:     &bytes.Buffer{},
		Err:     &bytes.Buffer{},
	}

	return app, localStore, remoteStore
}

func TestDepAddCrossStore(t *testing.T) {
	app, localStore, remoteStore := setupCrossStoreTestApp(t)
	out := app.Out.(*bytes.Buffer)
	ctx := context.Background()

	// Create local issue
	issueA := &issuestorage.Issue{
		Title:    "Local Issue A",
		Status:   issuestorage.StatusOpen,
		Priority: issuestorage.PriorityMedium,
		Type:     issuestorage.TypeTask,
	}
	idA, err := localStore.Create(ctx, issueA)
	if err != nil {
		t.Fatalf("failed to create local issue: %v", err)
	}

	// Create remote issue
	issueB := &issuestorage.Issue{
		Title:    "Remote Issue B",
		Status:   issuestorage.StatusOpen,
		Priority: issuestorage.PriorityMedium,
		Type:     issuestorage.TypeTask,
	}
	idB, err := remoteStore.Create(ctx, issueB)
	if err != nil {
		t.Fatalf("failed to create remote issue: %v", err)
	}

	// Add cross-store dependency: local A depends on remote B
	cmd := newDepAddCmd(NewTestProvider(app))
	cmd.SetArgs([]string{idA, idB})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("dep add cross-store failed: %v", err)
	}

	output := out.String()
	if !strings.Contains(output, "Added dependency") {
		t.Errorf("expected 'Added dependency' in output, got %q", output)
	}

	// Verify local side: A has B in dependencies
	gotA, err := localStore.Get(ctx, idA)
	if err != nil {
		t.Fatalf("failed to get local issue A: %v", err)
	}
	if !gotA.HasDependency(idB) {
		t.Errorf("expected A.Dependencies to contain B; got %v", gotA.Dependencies)
	}

	// Verify remote side: B has A in dependents
	gotB, err := remoteStore.Get(ctx, idB)
	if err != nil {
		t.Fatalf("failed to get remote issue B: %v", err)
	}
	if !gotB.HasDependent(idA) {
		t.Errorf("expected B.Dependents to contain A; got %v", gotB.Dependents)
	}
}

func TestDepAddCrossStoreCycleDetection(t *testing.T) {
	app, localStore, remoteStore := setupCrossStoreTestApp(t)
	ctx := context.Background()

	// Create local issue A and remote issue B
	issueA := &issuestorage.Issue{
		Title: "Local A", Status: issuestorage.StatusOpen,
		Priority: issuestorage.PriorityMedium, Type: issuestorage.TypeTask,
	}
	idA, _ := localStore.Create(ctx, issueA)

	issueB := &issuestorage.Issue{
		Title: "Remote B", Status: issuestorage.StatusOpen,
		Priority: issuestorage.PriorityMedium, Type: issuestorage.TypeTask,
	}
	idB, _ := remoteStore.Create(ctx, issueB)

	// Add A depends on B (cross-store)
	cmd := newDepAddCmd(NewTestProvider(app))
	cmd.SetArgs([]string{idA, idB})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("first dep add failed: %v", err)
	}

	// Try B depends on A (should fail — cycle)
	app.Out = &bytes.Buffer{} // reset output
	cmd2 := newDepAddCmd(NewTestProvider(app))
	cmd2.SetArgs([]string{idB, idA})
	err := cmd2.Execute()
	if err == nil {
		t.Fatal("expected error for cross-store cycle, got nil")
	}
	if !strings.Contains(err.Error(), "cycle") {
		t.Errorf("expected 'cycle' in error, got %v", err)
	}
}

func TestDepAddCrossStoreParentChildRejected(t *testing.T) {
	app, localStore, remoteStore := setupCrossStoreTestApp(t)
	ctx := context.Background()

	issueA := &issuestorage.Issue{
		Title: "Local A", Status: issuestorage.StatusOpen,
		Priority: issuestorage.PriorityMedium, Type: issuestorage.TypeTask,
	}
	idA, _ := localStore.Create(ctx, issueA)

	issueB := &issuestorage.Issue{
		Title: "Remote B", Status: issuestorage.StatusOpen,
		Priority: issuestorage.PriorityMedium, Type: issuestorage.TypeEpic,
	}
	idB, _ := remoteStore.Create(ctx, issueB)

	// Try cross-store parent-child — should be rejected
	cmd := newDepAddCmd(NewTestProvider(app))
	cmd.SetArgs([]string{idA, idB, "--type", "parent-child"})
	err := cmd.Execute()
	if err == nil {
		t.Fatal("expected error for cross-store parent-child, got nil")
	}
	if !strings.Contains(err.Error(), "parent-child") {
		t.Errorf("expected 'parent-child' in error, got %v", err)
	}
}

func TestDepRemoveCrossStore(t *testing.T) {
	app, localStore, remoteStore := setupCrossStoreTestApp(t)
	ctx := context.Background()

	// Create and add cross-store dep
	issueA := &issuestorage.Issue{
		Title: "Local A", Status: issuestorage.StatusOpen,
		Priority: issuestorage.PriorityMedium, Type: issuestorage.TypeTask,
	}
	idA, _ := localStore.Create(ctx, issueA)

	issueB := &issuestorage.Issue{
		Title: "Remote B", Status: issuestorage.StatusOpen,
		Priority: issuestorage.PriorityMedium, Type: issuestorage.TypeTask,
	}
	idB, _ := remoteStore.Create(ctx, issueB)

	// Add cross-store dep first
	addCmd := newDepAddCmd(NewTestProvider(app))
	addCmd.SetArgs([]string{idA, idB})
	if err := addCmd.Execute(); err != nil {
		t.Fatalf("dep add failed: %v", err)
	}

	// Remove cross-store dep
	app.Out = &bytes.Buffer{}
	rmCmd := newDepRemoveCmd(NewTestProvider(app))
	rmCmd.SetArgs([]string{idA, idB})
	if err := rmCmd.Execute(); err != nil {
		t.Fatalf("dep remove cross-store failed: %v", err)
	}

	// Verify local side: A no longer has B in dependencies
	gotA, _ := localStore.Get(ctx, idA)
	if gotA.HasDependency(idB) {
		t.Errorf("expected A.Dependencies to NOT contain B; got %v", gotA.Dependencies)
	}

	// Verify remote side: B no longer has A in dependents
	gotB, _ := remoteStore.Get(ctx, idB)
	if gotB.HasDependent(idA) {
		t.Errorf("expected B.Dependents to NOT contain A; got %v", gotB.Dependents)
	}
}
