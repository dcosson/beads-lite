package cmd

import (
	"bytes"
	"context"
	"encoding/json"
	"sort"
	"strings"
	"testing"

	"beads-lite/internal/issueservice"
	"beads-lite/internal/issuestorage"
	"beads-lite/internal/issuestorage/filesystem"
)

func TestGraphText_BackReferenceAndDirectBlockers(t *testing.T) {
	dir := t.TempDir()
	store := filesystem.New(dir, "bd-")
	ctx := context.Background()
	if err := store.Init(ctx); err != nil {
		t.Fatalf("init storage: %v", err)
	}
	rs := issueservice.New(nil, store)

	parentID, err := rs.Create(ctx, &issuestorage.Issue{Title: "Parent", Type: issuestorage.TypeEpic})
	if err != nil {
		t.Fatalf("create parent: %v", err)
	}
	a1, _ := rs.Create(ctx, &issuestorage.Issue{Title: "A1"})
	a2, _ := rs.Create(ctx, &issuestorage.Issue{Title: "A2"})
	a3, _ := rs.Create(ctx, &issuestorage.Issue{Title: "A3"})

	for _, taskID := range []string{a1, a2, a3} {
		if err := rs.AddDependency(ctx, taskID, parentID, issuestorage.DepTypeParentChild); err != nil {
			t.Fatalf("add parent-child %s: %v", taskID, err)
		}
	}
	if err := rs.AddDependency(ctx, a3, a1, issuestorage.DepTypeBlocks); err != nil {
		t.Fatalf("add dep a3->a1: %v", err)
	}
	if err := rs.AddDependency(ctx, a3, a2, issuestorage.DepTypeBlocks); err != nil {
		t.Fatalf("add dep a3->a2: %v", err)
	}

	var out bytes.Buffer
	app := &App{Storage: rs, Out: &out}
	cmd := newGraphCmd(NewTestProvider(app))
	cmd.SetArgs([]string{parentID})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("graph command failed: %v", err)
	}

	got := out.String()
	if !strings.Contains(got, "↗ "+a3) {
		t.Fatalf("expected back-reference for %s, got:\n%s", a3, got)
	}
	blockerIDs := []string{a1, a2}
	sort.Strings(blockerIDs)
	if !strings.Contains(got, "[blocked by: "+blockerIDs[0]+", "+blockerIDs[1]+"]") {
		t.Fatalf("expected direct blocker annotation for %s, got:\n%s", a3, got)
	}
}

func TestGraphText_ParentBlockedAnnotationAndParentOrdering(t *testing.T) {
	dir := t.TempDir()
	store := filesystem.New(dir, "bd-")
	ctx := context.Background()
	if err := store.Init(ctx); err != nil {
		t.Fatalf("init storage: %v", err)
	}
	rs := issueservice.New(nil, store)

	parentB, _ := rs.Create(ctx, &issuestorage.Issue{Title: "Parent B", Type: issuestorage.TypeEpic})
	parentA, _ := rs.Create(ctx, &issuestorage.Issue{Title: "Parent A", Type: issuestorage.TypeEpic})
	if err := rs.AddDependency(ctx, parentA, parentB, issuestorage.DepTypeBlocks); err != nil {
		t.Fatalf("add parent dep: %v", err)
	}

	taskB, _ := rs.Create(ctx, &issuestorage.Issue{Title: "Task B"})
	taskA, _ := rs.Create(ctx, &issuestorage.Issue{Title: "Task A"})
	if err := rs.AddDependency(ctx, taskB, parentB, issuestorage.DepTypeParentChild); err != nil {
		t.Fatalf("add parent-child taskB: %v", err)
	}
	if err := rs.AddDependency(ctx, taskA, parentA, issuestorage.DepTypeParentChild); err != nil {
		t.Fatalf("add parent-child taskA: %v", err)
	}

	var out bytes.Buffer
	app := &App{Storage: rs, Out: &out}
	cmd := newGraphCmd(NewTestProvider(app))
	if err := cmd.Execute(); err != nil {
		t.Fatalf("graph command failed: %v", err)
	}

	got := out.String()
	if !strings.Contains(got, "[parent blocked]") {
		t.Fatalf("expected parent blocked annotation, got:\n%s", got)
	}

	idxB := strings.Index(got, parentB+" [")
	idxA := strings.Index(got, parentA+" [")
	if idxB == -1 || idxA == -1 {
		t.Fatalf("expected both parent headers in output, got:\n%s", got)
	}
	if idxB >= idxA {
		t.Fatalf("expected blocker parent %s to appear before blocked parent %s, got:\n%s", parentB, parentA, got)
	}
}

func TestGraphJSONOutput(t *testing.T) {
	dir := t.TempDir()
	store := filesystem.New(dir, "bd-")
	ctx := context.Background()
	if err := store.Init(ctx); err != nil {
		t.Fatalf("init storage: %v", err)
	}
	rs := issueservice.New(nil, store)

	parentID, _ := rs.Create(ctx, &issuestorage.Issue{Title: "Parent", Type: issuestorage.TypeEpic})
	taskID, _ := rs.Create(ctx, &issuestorage.Issue{Title: "Child"})
	standaloneID, _ := rs.Create(ctx, &issuestorage.Issue{Title: "Standalone"})
	if err := rs.AddDependency(ctx, taskID, parentID, issuestorage.DepTypeParentChild); err != nil {
		t.Fatalf("add parent-child: %v", err)
	}

	var out bytes.Buffer
	app := &App{Storage: rs, Out: &out, JSON: true}
	cmd := newGraphCmd(NewTestProvider(app))
	if err := cmd.Execute(); err != nil {
		t.Fatalf("graph command failed: %v", err)
	}

	var payload GraphOutputJSON
	if err := json.Unmarshal(out.Bytes(), &payload); err != nil {
		t.Fatalf("unmarshal json: %v\nraw: %s", err, out.String())
	}

	if !payload.CascadeParentBlocking {
		t.Fatalf("expected cascade_parent_blocking=true")
	}
	if len(payload.Groups) != 1 {
		t.Fatalf("expected 1 group, got %d", len(payload.Groups))
	}
	if payload.Groups[0].ParentID != parentID {
		t.Fatalf("expected group parent %s, got %s", parentID, payload.Groups[0].ParentID)
	}
	if len(payload.Groups[0].Tasks) != 1 || payload.Groups[0].Tasks[0].ID != taskID {
		t.Fatalf("expected child task %s in group, got %+v", taskID, payload.Groups[0].Tasks)
	}
	if len(payload.Standalone) != 1 || payload.Standalone[0].ID != standaloneID {
		t.Fatalf("expected standalone task %s, got %+v", standaloneID, payload.Standalone)
	}
	if len(payload.Waves) == 0 {
		t.Fatalf("expected waves in json output")
	}
}

func TestGraphWavesFlagText(t *testing.T) {
	dir := t.TempDir()
	store := filesystem.New(dir, "bd-")
	ctx := context.Background()
	if err := store.Init(ctx); err != nil {
		t.Fatalf("init storage: %v", err)
	}
	rs := issueservice.New(nil, store)

	parentB, _ := rs.Create(ctx, &issuestorage.Issue{Title: "Parent B", Type: issuestorage.TypeEpic})
	parentA, _ := rs.Create(ctx, &issuestorage.Issue{Title: "Parent A", Type: issuestorage.TypeEpic})
	if err := rs.AddDependency(ctx, parentA, parentB, issuestorage.DepTypeBlocks); err != nil {
		t.Fatalf("add parent dep: %v", err)
	}
	taskB, _ := rs.Create(ctx, &issuestorage.Issue{Title: "Task B"})
	taskA, _ := rs.Create(ctx, &issuestorage.Issue{Title: "Task A"})
	if err := rs.AddDependency(ctx, taskB, parentB, issuestorage.DepTypeParentChild); err != nil {
		t.Fatalf("add parent-child taskB: %v", err)
	}
	if err := rs.AddDependency(ctx, taskA, parentA, issuestorage.DepTypeParentChild); err != nil {
		t.Fatalf("add parent-child taskA: %v", err)
	}

	var out bytes.Buffer
	app := &App{Storage: rs, Out: &out}
	cmd := newGraphCmd(NewTestProvider(app))
	cmd.SetArgs([]string{"--waves"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("graph --waves command failed: %v", err)
	}

	got := out.String()
	if !strings.Contains(got, "Waves") || !strings.Contains(got, "Wave 0:") {
		t.Fatalf("expected waves section, got:\n%s", got)
	}
	if !strings.Contains(got, taskA) || !strings.Contains(got, taskB) {
		t.Fatalf("expected both task IDs in output, got:\n%s", got)
	}
}
