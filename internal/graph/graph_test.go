package graph

import (
	"context"
	"path/filepath"
	"testing"

	"beads-lite/internal/storage"
	"beads-lite/internal/storage/filesystem"
)

func newStore(t *testing.T) storage.Storage {
	t.Helper()
	dir := filepath.Join(t.TempDir(), ".beads")
	s := filesystem.New(dir)
	ctx := context.Background()
	if err := s.Init(ctx); err != nil {
		t.Fatalf("Init: %v", err)
	}
	return s
}

// createIssue is a test helper that creates an issue and fails the test on error.
func createIssue(t *testing.T, ctx context.Context, s storage.Storage, title string, typ storage.IssueType) *storage.Issue {
	t.Helper()
	issue := &storage.Issue{
		Title:    title,
		Status:   storage.StatusOpen,
		Priority: storage.PriorityMedium,
		Type:     typ,
	}
	id, err := s.Create(ctx, issue)
	if err != nil {
		t.Fatalf("Create %q: %v", title, err)
	}
	issue.ID = id
	return issue
}

// buildMolecule creates a root epic with children and optional DepBlocks edges.
// Returns (root, children). The children are created with parent-child deps to root.
// blockEdges maps childTitle → []blockerTitle for DepBlocks dependencies.
func buildMolecule(t *testing.T, ctx context.Context, s storage.Storage, rootTitle string, childTitles []string, blockEdges map[string][]string) (*storage.Issue, []*storage.Issue) {
	t.Helper()
	root := createIssue(t, ctx, s, rootTitle, storage.TypeEpic)

	byTitle := make(map[string]*storage.Issue)
	var children []*storage.Issue
	for _, title := range childTitles {
		child := createIssue(t, ctx, s, title, storage.TypeTask)
		if err := s.AddDependency(ctx, child.ID, root.ID, storage.DepTypeParentChild); err != nil {
			t.Fatalf("AddDependency parent-child %s->%s: %v", child.ID, root.ID, err)
		}
		byTitle[title] = child
		children = append(children, child)
	}

	for childTitle, blockerTitles := range blockEdges {
		child := byTitle[childTitle]
		for _, blockerTitle := range blockerTitles {
			blocker := byTitle[blockerTitle]
			if err := s.AddDependency(ctx, child.ID, blocker.ID, storage.DepTypeBlocks); err != nil {
				t.Fatalf("AddDependency blocks %s->%s: %v", child.ID, blocker.ID, err)
			}
		}
	}

	// Re-read all children to get updated dependency data
	for i, child := range children {
		got, err := s.Get(ctx, child.ID)
		if err != nil {
			t.Fatalf("re-read child %s: %v", child.ID, err)
		}
		children[i] = got
	}

	root, err := s.Get(ctx, root.ID)
	if err != nil {
		t.Fatalf("re-read root: %v", err)
	}
	return root, children
}

func TestFindMoleculeRoot(t *testing.T) {
	ctx := context.Background()
	s := newStore(t)

	// Build: root -> child -> grandchild
	root := createIssue(t, ctx, s, "Root Epic", storage.TypeEpic)
	child := createIssue(t, ctx, s, "Child", storage.TypeTask)
	grandchild := createIssue(t, ctx, s, "Grandchild", storage.TypeTask)

	if err := s.AddDependency(ctx, child.ID, root.ID, storage.DepTypeParentChild); err != nil {
		t.Fatal(err)
	}
	if err := s.AddDependency(ctx, grandchild.ID, child.ID, storage.DepTypeParentChild); err != nil {
		t.Fatal(err)
	}

	tests := []struct {
		name    string
		startID string
	}{
		{"from root", root.ID},
		{"from child", child.ID},
		{"from grandchild", grandchild.ID},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := FindMoleculeRoot(ctx, s, tt.startID)
			if err != nil {
				t.Fatalf("FindMoleculeRoot(%s): %v", tt.startID, err)
			}
			if got.ID != root.ID {
				t.Errorf("got root %s, want %s", got.ID, root.ID)
			}
		})
	}

	// Non-existent issue
	_, err := FindMoleculeRoot(ctx, s, "nonexistent")
	if err == nil {
		t.Error("expected error for nonexistent issue")
	}
}

func TestCollectMoleculeChildren(t *testing.T) {
	ctx := context.Background()
	s := newStore(t)

	// root -> [A, B], A -> [C]
	root := createIssue(t, ctx, s, "Root", storage.TypeEpic)
	a := createIssue(t, ctx, s, "A", storage.TypeTask)
	b := createIssue(t, ctx, s, "B", storage.TypeTask)
	c := createIssue(t, ctx, s, "C", storage.TypeTask)

	for _, pair := range [][2]string{
		{a.ID, root.ID},
		{b.ID, root.ID},
		{c.ID, a.ID},
	} {
		if err := s.AddDependency(ctx, pair[0], pair[1], storage.DepTypeParentChild); err != nil {
			t.Fatal(err)
		}
	}

	children, err := CollectMoleculeChildren(ctx, s, root.ID)
	if err != nil {
		t.Fatalf("CollectMoleculeChildren: %v", err)
	}

	if len(children) != 3 {
		t.Fatalf("got %d children, want 3", len(children))
	}

	ids := make(map[string]bool)
	for _, ch := range children {
		ids[ch.ID] = true
	}
	for _, want := range []string{a.ID, b.ID, c.ID} {
		if !ids[want] {
			t.Errorf("missing child %s", want)
		}
	}

	// Non-existent root
	_, err = CollectMoleculeChildren(ctx, s, "nonexistent")
	if err == nil {
		t.Error("expected error for nonexistent root")
	}
}

func TestTopologicalOrder(t *testing.T) {
	ctx := context.Background()
	s := newStore(t)

	// A -> B -> C (B blocked by A, C blocked by B)
	_, children := buildMolecule(t, ctx, s, "Root", []string{"A", "B", "C"},
		map[string][]string{
			"B": {"A"},
			"C": {"B"},
		})

	ordered, err := TopologicalOrder(children)
	if err != nil {
		t.Fatalf("TopologicalOrder: %v", err)
	}

	if len(ordered) != 3 {
		t.Fatalf("got %d ordered, want 3", len(ordered))
	}

	// Build position map to check ordering constraints
	pos := make(map[string]int)
	for i, issue := range ordered {
		pos[issue.Title] = i
	}

	if pos["A"] >= pos["B"] {
		t.Errorf("A (pos %d) should come before B (pos %d)", pos["A"], pos["B"])
	}
	if pos["B"] >= pos["C"] {
		t.Errorf("B (pos %d) should come before C (pos %d)", pos["B"], pos["C"])
	}
}

func TestTopologicalOrder_NoDeps(t *testing.T) {
	ctx := context.Background()
	s := newStore(t)

	_, children := buildMolecule(t, ctx, s, "Root", []string{"X", "Y", "Z"}, nil)

	ordered, err := TopologicalOrder(children)
	if err != nil {
		t.Fatalf("TopologicalOrder: %v", err)
	}
	if len(ordered) != 3 {
		t.Fatalf("got %d ordered, want 3", len(ordered))
	}
}

func TestTopologicalOrder_Empty(t *testing.T) {
	ordered, err := TopologicalOrder(nil)
	if err != nil {
		t.Fatalf("TopologicalOrder(nil): %v", err)
	}
	if ordered != nil {
		t.Errorf("expected nil, got %v", ordered)
	}
}

func TestTopologicalOrder_Cycle(t *testing.T) {
	// Manually construct issues with circular DepBlocks to test cycle detection.
	// We can't use storage.AddDependency because it rejects cycles, so we build
	// the issue structs directly.
	depType := storage.DepTypeBlocks
	a := &storage.Issue{
		ID:     "a",
		Title:  "A",
		Status: storage.StatusOpen,
		Dependencies: []storage.Dependency{
			{ID: "b", Type: depType},
		},
	}
	b := &storage.Issue{
		ID:     "b",
		Title:  "B",
		Status: storage.StatusOpen,
		Dependencies: []storage.Dependency{
			{ID: "a", Type: depType},
		},
	}

	_, err := TopologicalOrder([]*storage.Issue{a, b})
	if err == nil {
		t.Fatal("expected cycle error")
	}
}

func TestFindReadySteps(t *testing.T) {
	ctx := context.Background()
	s := newStore(t)

	// A (no deps), B blocked by A, C blocked by A
	_, children := buildMolecule(t, ctx, s, "Root", []string{"A", "B", "C"},
		map[string][]string{
			"B": {"A"},
			"C": {"A"},
		})

	byTitle := make(map[string]*storage.Issue)
	for _, c := range children {
		byTitle[c.Title] = c
	}

	// No closed set: only A is ready
	ready := FindReadySteps(children, map[string]bool{})
	if len(ready) != 1 || ready[0].Title != "A" {
		titles := titlesOf(ready)
		t.Errorf("with empty closedSet: got %v, want [A]", titles)
	}

	// Close A: B and C become ready
	closedSet := map[string]bool{byTitle["A"].ID: true}
	// Mark A as closed on the issue struct too
	byTitle["A"].Status = storage.StatusClosed
	ready = FindReadySteps(children, closedSet)
	titles := titlesOf(ready)
	if len(ready) != 2 {
		t.Errorf("after closing A: got %v, want [B, C]", titles)
	}
}

func TestFindReadySteps_AllClosed(t *testing.T) {
	ctx := context.Background()
	s := newStore(t)

	_, children := buildMolecule(t, ctx, s, "Root", []string{"A", "B"}, nil)

	closedSet := make(map[string]bool)
	for _, c := range children {
		c.Status = storage.StatusClosed
		closedSet[c.ID] = true
	}

	ready := FindReadySteps(children, closedSet)
	if len(ready) != 0 {
		t.Errorf("all closed: got %d ready, want 0", len(ready))
	}
}

func TestFindNextStep(t *testing.T) {
	ctx := context.Background()
	s := newStore(t)

	// A -> B -> C
	_, children := buildMolecule(t, ctx, s, "Root", []string{"A", "B", "C"},
		map[string][]string{
			"B": {"A"},
			"C": {"B"},
		})

	ordered, err := TopologicalOrder(children)
	if err != nil {
		t.Fatal(err)
	}

	byTitle := make(map[string]*storage.Issue)
	for _, c := range children {
		byTitle[c.Title] = c
	}

	// Current is A, A is in closedSet → next ready should be B
	closedSet := map[string]bool{byTitle["A"].ID: true}
	byTitle["A"].Status = storage.StatusClosed

	// Re-read ordered to pick up status changes; actually the ordered slice
	// has the same pointers as children via buildMolecule re-reads, but we
	// modified byTitle. Let's just use the ordered from TopologicalOrder which
	// was built from the children slice before we modified status.
	// We need ordered from fresh children. Let's rebuild.
	// Actually, TopologicalOrder got its data from children before status change.
	// The ordered slice's status field may be stale. Let's update it.
	for _, o := range ordered {
		if o.ID == byTitle["A"].ID {
			o.Status = storage.StatusClosed
		}
	}

	next := FindNextStep(ordered, byTitle["A"].ID, closedSet)
	if next == nil {
		t.Fatal("expected next step, got nil")
	}
	if next.Title != "B" {
		t.Errorf("got next %q, want B", next.Title)
	}

	// Close B too, next from B should be C
	closedSet[byTitle["B"].ID] = true
	for _, o := range ordered {
		if o.ID == byTitle["B"].ID {
			o.Status = storage.StatusClosed
		}
	}
	next = FindNextStep(ordered, byTitle["B"].ID, closedSet)
	if next == nil {
		t.Fatal("expected next step C, got nil")
	}
	if next.Title != "C" {
		t.Errorf("got next %q, want C", next.Title)
	}

	// From C (last), no more steps
	closedSet[byTitle["C"].ID] = true
	next = FindNextStep(ordered, byTitle["C"].ID, closedSet)
	if next != nil {
		t.Errorf("expected nil after last step, got %q", next.Title)
	}
}

func TestClassifySteps(t *testing.T) {
	ctx := context.Background()
	s := newStore(t)

	// A (no deps), B blocked by A, C blocked by B
	_, children := buildMolecule(t, ctx, s, "Root", []string{"A", "B", "C"},
		map[string][]string{
			"B": {"A"},
			"C": {"B"},
		})

	byTitle := make(map[string]*storage.Issue)
	for _, c := range children {
		byTitle[c.Title] = c
	}

	// Initial state: A=ready, B=blocked, C=blocked
	classes := ClassifySteps(children, map[string]bool{})
	assertStatus(t, classes, byTitle["A"].ID, StepReady)
	assertStatus(t, classes, byTitle["B"].ID, StepBlocked)
	assertStatus(t, classes, byTitle["C"].ID, StepBlocked)

	// Set A to in_progress: A=current, B=blocked, C=blocked
	byTitle["A"].Status = storage.StatusInProgress
	classes = ClassifySteps(children, map[string]bool{})
	assertStatus(t, classes, byTitle["A"].ID, StepCurrent)
	assertStatus(t, classes, byTitle["B"].ID, StepBlocked)

	// Close A: A=done, B=ready, C=blocked
	byTitle["A"].Status = storage.StatusClosed
	closedSet := map[string]bool{byTitle["A"].ID: true}
	classes = ClassifySteps(children, closedSet)
	assertStatus(t, classes, byTitle["A"].ID, StepDone)
	assertStatus(t, classes, byTitle["B"].ID, StepReady)
	assertStatus(t, classes, byTitle["C"].ID, StepBlocked)
}

func TestBuildClosedSet(t *testing.T) {
	ctx := context.Background()
	s := newStore(t)

	a := createIssue(t, ctx, s, "Open Issue", storage.TypeTask)
	b := createIssue(t, ctx, s, "Closed Issue", storage.TypeTask)

	if err := s.Close(ctx, b.ID); err != nil {
		t.Fatalf("Close: %v", err)
	}

	closedSet, err := BuildClosedSet(ctx, s)
	if err != nil {
		t.Fatalf("BuildClosedSet: %v", err)
	}

	if closedSet[a.ID] {
		t.Errorf("open issue %s should not be in closed set", a.ID)
	}
	if !closedSet[b.ID] {
		t.Errorf("closed issue %s should be in closed set", b.ID)
	}
}

func TestBuildClosedSet_Empty(t *testing.T) {
	ctx := context.Background()
	s := newStore(t)

	closedSet, err := BuildClosedSet(ctx, s)
	if err != nil {
		t.Fatalf("BuildClosedSet: %v", err)
	}
	if len(closedSet) != 0 {
		t.Errorf("expected empty set, got %d entries", len(closedSet))
	}
}

// helpers

func titlesOf(issues []*storage.Issue) []string {
	var out []string
	for _, i := range issues {
		out = append(out, i.Title)
	}
	return out
}

func assertStatus(t *testing.T, classes map[string]StepStatus, id string, want StepStatus) {
	t.Helper()
	got, ok := classes[id]
	if !ok {
		t.Errorf("no status for %s", id)
		return
	}
	if got != want {
		t.Errorf("status of %s: got %s, want %s", id, got, want)
	}
}
