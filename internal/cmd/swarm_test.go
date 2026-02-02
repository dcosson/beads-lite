package cmd

import (
	"bytes"
	"context"
	"encoding/json"
	"path/filepath"
	"testing"

	"beads-lite/internal/issuestorage"
	"beads-lite/internal/issuestorage/filesystem"
)

func newSwarmTestApp(t *testing.T) (*App, issuestorage.IssueStore) {
	t.Helper()
	dir := filepath.Join(t.TempDir(), ".beads")
	store := filesystem.New(dir, "bl-")
	if err := store.Init(context.Background()); err != nil {
		t.Fatalf("Init: %v", err)
	}
	app := &App{
		Storage: store,
		Out:     &bytes.Buffer{},
		Err:     &bytes.Buffer{},
	}
	return app, store
}

// buildSwarmEpic creates an epic with children and block edges for swarm testing.
func buildSwarmEpic(t *testing.T, store issuestorage.IssueStore, childTitles []string, blockEdges map[string][]string) (string, map[string]string) {
	t.Helper()
	ctx := context.Background()

	epic := &issuestorage.Issue{
		Title:    "Test Epic",
		Type:     issuestorage.TypeEpic,
		Status:   issuestorage.StatusOpen,
		Priority: issuestorage.PriorityMedium,
	}
	epicID, err := store.Create(ctx, epic)
	if err != nil {
		t.Fatalf("create epic: %v", err)
	}

	byTitle := make(map[string]string)
	for _, title := range childTitles {
		child := &issuestorage.Issue{
			Title:    title,
			Type:     issuestorage.TypeTask,
			Status:   issuestorage.StatusOpen,
			Priority: issuestorage.PriorityMedium,
		}
		id, err := store.Create(ctx, child)
		if err != nil {
			t.Fatalf("create child %s: %v", title, err)
		}
		byTitle[title] = id
		if err := store.AddDependency(ctx, id, epicID, issuestorage.DepTypeParentChild); err != nil {
			t.Fatalf("add parent-child %s: %v", title, err)
		}
	}

	for childTitle, blockerTitles := range blockEdges {
		for _, blockerTitle := range blockerTitles {
			if err := store.AddDependency(ctx, byTitle[childTitle], byTitle[blockerTitle], issuestorage.DepTypeBlocks); err != nil {
				t.Fatalf("add blocks %s->%s: %v", childTitle, blockerTitle, err)
			}
		}
	}

	return epicID, byTitle
}

func TestSwarmValidate(t *testing.T) {
	app, store := newSwarmTestApp(t)
	provider := NewTestProvider(app)

	epicID, _ := buildSwarmEpic(t, store, []string{"A", "B", "C"}, map[string][]string{
		"B": {"A"},
		"C": {"B"},
	})

	cmd := newSwarmValidateCmd(provider)
	cmd.SetArgs([]string{epicID})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("swarm validate: %v", err)
	}

	output := app.Out.(*bytes.Buffer).String()
	if output == "" {
		t.Error("expected non-empty output")
	}
}

func TestSwarmValidate_JSON(t *testing.T) {
	app, store := newSwarmTestApp(t)
	app.JSON = true
	provider := NewTestProvider(app)

	epicID, _ := buildSwarmEpic(t, store, []string{"A", "B", "C"}, map[string][]string{
		"B": {"A"},
		"C": {"A"},
	})

	cmd := newSwarmValidateCmd(provider)
	cmd.SetArgs([]string{epicID})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("swarm validate: %v", err)
	}

	var result SwarmValidateJSON
	if err := json.Unmarshal(app.Out.(*bytes.Buffer).Bytes(), &result); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	if !result.Swarmable {
		t.Error("expected swarmable=true")
	}
	if result.TotalChildren != 3 {
		t.Errorf("expected 3 children, got %d", result.TotalChildren)
	}
	if len(result.Waves) != 2 {
		t.Errorf("expected 2 waves, got %d", len(result.Waves))
	}
	if result.MaxParallelism != 2 {
		t.Errorf("expected max parallelism 2, got %d", result.MaxParallelism)
	}
}

func TestSwarmValidate_NoChildren(t *testing.T) {
	app, store := newSwarmTestApp(t)
	provider := NewTestProvider(app)
	ctx := context.Background()

	epic := &issuestorage.Issue{
		Title:    "Empty Epic",
		Type:     issuestorage.TypeEpic,
		Status:   issuestorage.StatusOpen,
		Priority: issuestorage.PriorityMedium,
	}
	epicID, err := store.Create(ctx, epic)
	if err != nil {
		t.Fatalf("create: %v", err)
	}

	cmd := newSwarmValidateCmd(provider)
	cmd.SetArgs([]string{epicID})

	if err := cmd.Execute(); err == nil {
		t.Error("expected error for empty epic")
	}
}

func TestSwarmCreate(t *testing.T) {
	app, store := newSwarmTestApp(t)
	provider := NewTestProvider(app)

	epicID, _ := buildSwarmEpic(t, store, []string{"A", "B"}, nil)

	cmd := newSwarmCreateCmd(provider)
	cmd.SetArgs([]string{epicID})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("swarm create: %v", err)
	}

	output := app.Out.(*bytes.Buffer).String()
	if output == "" {
		t.Error("expected non-empty output")
	}
}

func TestSwarmCreate_JSON(t *testing.T) {
	app, store := newSwarmTestApp(t)
	app.JSON = true
	provider := NewTestProvider(app)

	epicID, _ := buildSwarmEpic(t, store, []string{"A", "B"}, nil)

	cmd := newSwarmCreateCmd(provider)
	cmd.SetArgs([]string{epicID})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("swarm create: %v", err)
	}

	var result SwarmCreateJSON
	if err := json.Unmarshal(app.Out.(*bytes.Buffer).Bytes(), &result); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	if result.MoleculeID == "" {
		t.Error("expected non-empty molecule ID")
	}
	if result.EpicID != epicID {
		t.Errorf("expected epic ID %s, got %s", epicID, result.EpicID)
	}

	// Verify molecule was created with correct type
	ctx := context.Background()
	mol, err := store.Get(ctx, result.MoleculeID)
	if err != nil {
		t.Fatalf("get molecule: %v", err)
	}
	if mol.Type != issuestorage.TypeMolecule {
		t.Errorf("expected type molecule, got %s", mol.Type)
	}
	if mol.MolType != issuestorage.MolTypeSwarm {
		t.Errorf("expected mol_type swarm, got %s", mol.MolType)
	}

	// Verify relates-to link
	relatesTo := issuestorage.DepTypeRelatesTo
	deps := mol.DependencyIDs(&relatesTo)
	if len(deps) != 1 || deps[0] != epicID {
		t.Errorf("expected relates-to %s, got %v", epicID, deps)
	}
}

func TestSwarmCreate_AutoWrapTask(t *testing.T) {
	app, store := newSwarmTestApp(t)
	app.JSON = true
	provider := NewTestProvider(app)
	ctx := context.Background()

	task := &issuestorage.Issue{
		Title:    "Solo Task",
		Type:     issuestorage.TypeTask,
		Status:   issuestorage.StatusOpen,
		Priority: issuestorage.PriorityMedium,
	}
	taskID, err := store.Create(ctx, task)
	if err != nil {
		t.Fatalf("create task: %v", err)
	}

	cmd := newSwarmCreateCmd(provider)
	cmd.SetArgs([]string{taskID})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("swarm create (auto-wrap): %v", err)
	}

	var result SwarmCreateJSON
	if err := json.Unmarshal(app.Out.(*bytes.Buffer).Bytes(), &result); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	if result.EpicID == taskID {
		t.Error("expected auto-wrapped epic ID to differ from task ID")
	}

	// Verify wrapper epic was created
	epic, err := store.Get(ctx, result.EpicID)
	if err != nil {
		t.Fatalf("get wrapper epic: %v", err)
	}
	if epic.Type != issuestorage.TypeEpic {
		t.Errorf("expected epic type, got %s", epic.Type)
	}
}

func TestSwarmCreate_WithCoordinator(t *testing.T) {
	app, store := newSwarmTestApp(t)
	app.JSON = true
	provider := NewTestProvider(app)

	epicID, _ := buildSwarmEpic(t, store, []string{"A"}, nil)

	cmd := newSwarmCreateCmd(provider)
	cmd.SetArgs([]string{epicID, "--coordinator", "witness"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("swarm create: %v", err)
	}

	var result SwarmCreateJSON
	if err := json.Unmarshal(app.Out.(*bytes.Buffer).Bytes(), &result); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	ctx := context.Background()
	mol, err := store.Get(ctx, result.MoleculeID)
	if err != nil {
		t.Fatalf("get molecule: %v", err)
	}
	if mol.Assignee != "witness" {
		t.Errorf("expected assignee 'witness', got %q", mol.Assignee)
	}
}

func TestSwarmStatus(t *testing.T) {
	app, store := newSwarmTestApp(t)
	provider := NewTestProvider(app)

	epicID, byTitle := buildSwarmEpic(t, store, []string{"A", "B", "C"}, map[string][]string{
		"B": {"A"},
		"C": {"B"},
	})

	// Close A to make B ready
	ctx := context.Background()
	if err := store.Close(ctx, byTitle["A"]); err != nil {
		t.Fatalf("close A: %v", err)
	}

	cmd := newSwarmStatusCmd(provider)
	cmd.SetArgs([]string{epicID})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("swarm status: %v", err)
	}

	output := app.Out.(*bytes.Buffer).String()
	if output == "" {
		t.Error("expected non-empty output")
	}
}

func TestSwarmStatus_JSON(t *testing.T) {
	app, store := newSwarmTestApp(t)
	app.JSON = true
	provider := NewTestProvider(app)

	epicID, byTitle := buildSwarmEpic(t, store, []string{"A", "B", "C"}, map[string][]string{
		"B": {"A"},
		"C": {"B"},
	})

	ctx := context.Background()
	if err := store.Close(ctx, byTitle["A"]); err != nil {
		t.Fatalf("close A: %v", err)
	}

	cmd := newSwarmStatusCmd(provider)
	cmd.SetArgs([]string{epicID})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("swarm status: %v", err)
	}

	var result SwarmStatusJSON
	if err := json.Unmarshal(app.Out.(*bytes.Buffer).Bytes(), &result); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	if result.Total != 3 {
		t.Errorf("expected 3 total, got %d", result.Total)
	}
	if result.Completed != 1 {
		t.Errorf("expected 1 completed, got %d", result.Completed)
	}
	if result.Ready != 1 {
		t.Errorf("expected 1 ready, got %d", result.Ready)
	}
	if result.Blocked != 1 {
		t.Errorf("expected 1 blocked, got %d", result.Blocked)
	}
}

func TestSwarmStatus_FromMolecule(t *testing.T) {
	app, store := newSwarmTestApp(t)
	app.JSON = true
	provider := NewTestProvider(app)
	ctx := context.Background()

	epicID, _ := buildSwarmEpic(t, store, []string{"A", "B"}, nil)

	// Create swarm molecule
	createCmd := newSwarmCreateCmd(provider)
	createCmd.SetArgs([]string{epicID})
	if err := createCmd.Execute(); err != nil {
		t.Fatalf("swarm create: %v", err)
	}

	var createResult SwarmCreateJSON
	if err := json.Unmarshal(app.Out.(*bytes.Buffer).Bytes(), &createResult); err != nil {
		t.Fatalf("unmarshal create: %v", err)
	}

	// Reset output buffer and get status via molecule ID
	app.Out = &bytes.Buffer{}
	_ = ctx

	statusCmd := newSwarmStatusCmd(provider)
	statusCmd.SetArgs([]string{createResult.MoleculeID})
	if err := statusCmd.Execute(); err != nil {
		t.Fatalf("swarm status from molecule: %v", err)
	}

	var statusResult SwarmStatusJSON
	if err := json.Unmarshal(app.Out.(*bytes.Buffer).Bytes(), &statusResult); err != nil {
		t.Fatalf("unmarshal status: %v", err)
	}

	if statusResult.EpicID != epicID {
		t.Errorf("expected epic %s, got %s", epicID, statusResult.EpicID)
	}
	if statusResult.MoleculeID != createResult.MoleculeID {
		t.Errorf("expected molecule %s, got %s", createResult.MoleculeID, statusResult.MoleculeID)
	}
}

func TestSwarmList(t *testing.T) {
	app, store := newSwarmTestApp(t)
	provider := NewTestProvider(app)

	epicID, _ := buildSwarmEpic(t, store, []string{"A", "B"}, nil)

	// Create swarm molecule (with JSON to capture ID)
	app.JSON = true
	createCmd := newSwarmCreateCmd(provider)
	createCmd.SetArgs([]string{epicID})
	if err := createCmd.Execute(); err != nil {
		t.Fatalf("swarm create: %v", err)
	}

	// Reset and list
	app.Out = &bytes.Buffer{}

	listCmd := newSwarmListCmd(provider)
	listCmd.SetArgs([]string{})
	if err := listCmd.Execute(); err != nil {
		t.Fatalf("swarm list: %v", err)
	}

	var result SwarmListJSON
	if err := json.Unmarshal(app.Out.(*bytes.Buffer).Bytes(), &result); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	if len(result.Swarms) != 1 {
		t.Fatalf("expected 1 swarm, got %d", len(result.Swarms))
	}
	if result.Swarms[0].EpicID != epicID {
		t.Errorf("expected epic %s, got %s", epicID, result.Swarms[0].EpicID)
	}
	if result.Swarms[0].Total != 2 {
		t.Errorf("expected 2 total, got %d", result.Swarms[0].Total)
	}
}

func TestSwarmList_Empty(t *testing.T) {
	app, _ := newSwarmTestApp(t)
	provider := NewTestProvider(app)

	cmd := newSwarmListCmd(provider)
	cmd.SetArgs([]string{})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("swarm list: %v", err)
	}

	output := app.Out.(*bytes.Buffer).String()
	if output == "" {
		t.Error("expected non-empty output")
	}
}

func TestSwarmValidate_DisconnectedWarning(t *testing.T) {
	app, store := newSwarmTestApp(t)
	app.JSON = true
	provider := NewTestProvider(app)

	// A has a dep on B, C is disconnected
	epicID, _ := buildSwarmEpic(t, store, []string{"A", "B", "C"}, map[string][]string{
		"B": {"A"},
	})

	cmd := newSwarmValidateCmd(provider)
	cmd.SetArgs([]string{epicID})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("swarm validate: %v", err)
	}

	var result SwarmValidateJSON
	if err := json.Unmarshal(app.Out.(*bytes.Buffer).Bytes(), &result); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	if len(result.Warnings) == 0 {
		t.Error("expected warnings about disconnected issues")
	}
}
