package cmd

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"testing"
	"time"

	"beads-lite/internal/issuestorage"
	"beads-lite/internal/issuestorage/filesystem"
)

// mockExecutor returns a commandExecutor that returns canned responses
// based on the command arguments.
func mockExecutor(responses map[string]struct {
	output []byte
	err    error
}) commandExecutor {
	return func(name string, args ...string) ([]byte, error) {
		key := name + " " + strings.Join(args, " ")
		if resp, ok := responses[key]; ok {
			return resp.output, resp.err
		}
		return nil, fmt.Errorf("unexpected command: %s %s", name, strings.Join(args, " "))
	}
}

func setupCheckTestApp(t *testing.T) (*App, *filesystem.FilesystemStorage) {
	t.Helper()
	dir := t.TempDir()
	store := filesystem.New(dir, "bd-")
	if err := store.Init(context.Background()); err != nil {
		t.Fatalf("failed to init storage: %v", err)
	}
	return &App{
		Storage: store,
		Out:     &bytes.Buffer{},
		Err:     &bytes.Buffer{},
	}, store
}

func TestGateCheckTimerExpired(t *testing.T) {
	app, store := setupCheckTestApp(t)
	ctx := context.Background()

	// Create a timer gate that expired 1 hour ago
	id, err := store.Create(ctx, &issuestorage.Issue{
		Title:     "Timer gate",
		Type:      issuestorage.TypeGate,
		Priority:  issuestorage.PriorityMedium,
		AwaitType: "timer",
		TimeoutNS: int64(1 * time.Hour),
	})
	if err != nil {
		t.Fatalf("failed to create gate: %v", err)
	}

	// Backdate the gate's creation time so the timer is expired
	gate, _ := store.Get(ctx, id)
	gate.CreatedAt = time.Now().Add(-2 * time.Hour)
	if err := store.Update(ctx, gate); err != nil {
		t.Fatalf("failed to backdate gate: %v", err)
	}

	out := app.Out.(*bytes.Buffer)
	cmd := gateCheckCmd(NewTestProvider(app), nil, false)
	cmd.SetArgs([]string{})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("gate check failed: %v", err)
	}

	output := out.String()
	if !strings.Contains(output, "resolved") {
		t.Errorf("expected 'resolved' in output, got: %s", output)
	}
	if !strings.Contains(output, "deadline passed") {
		t.Errorf("expected 'deadline passed' in output, got: %s", output)
	}

	// Verify gate was closed
	closed, _ := store.Get(ctx, id)
	if closed.Status != issuestorage.StatusClosed {
		t.Errorf("expected gate to be closed, got status %q", closed.Status)
	}
}

func TestGateCheckTimerNotExpired(t *testing.T) {
	app, store := setupCheckTestApp(t)
	ctx := context.Background()

	id, err := store.Create(ctx, &issuestorage.Issue{
		Title:     "Future timer",
		Type:      issuestorage.TypeGate,
		Priority:  issuestorage.PriorityMedium,
		AwaitType: "timer",
		TimeoutNS: int64(24 * time.Hour),
	})
	if err != nil {
		t.Fatalf("failed to create gate: %v", err)
	}

	out := app.Out.(*bytes.Buffer)
	cmd := gateCheckCmd(NewTestProvider(app), nil, false)
	cmd.SetArgs([]string{})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("gate check failed: %v", err)
	}

	output := out.String()
	if !strings.Contains(output, "pending") {
		t.Errorf("expected 'pending' in output, got: %s", output)
	}
	if !strings.Contains(output, "deadline in") {
		t.Errorf("expected 'deadline in' in output, got: %s", output)
	}

	// Verify gate is still open
	gate, _ := store.Get(ctx, id)
	if gate.Status != issuestorage.StatusOpen {
		t.Errorf("expected gate to remain open, got status %q", gate.Status)
	}
}

func TestGateCheckTimerNoTimeout(t *testing.T) {
	app, store := setupCheckTestApp(t)
	ctx := context.Background()

	_, err := store.Create(ctx, &issuestorage.Issue{
		Title:     "No timeout gate",
		Type:      issuestorage.TypeGate,
		Priority:  issuestorage.PriorityMedium,
		AwaitType: "timer",
		TimeoutNS: 0,
	})
	if err != nil {
		t.Fatalf("failed to create gate: %v", err)
	}

	out := app.Out.(*bytes.Buffer)
	cmd := gateCheckCmd(NewTestProvider(app), nil, false)
	cmd.SetArgs([]string{})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("gate check failed: %v", err)
	}

	output := out.String()
	if !strings.Contains(output, "pending") {
		t.Errorf("expected 'pending' in output, got: %s", output)
	}
	if !strings.Contains(output, "no timeout configured") {
		t.Errorf("expected 'no timeout configured' in output, got: %s", output)
	}
}

func TestGateCheckTimerExpiredWithEscalate(t *testing.T) {
	app, store := setupCheckTestApp(t)
	ctx := context.Background()

	id, err := store.Create(ctx, &issuestorage.Issue{
		Title:     "Expired timer",
		Type:      issuestorage.TypeGate,
		Priority:  issuestorage.PriorityMedium,
		AwaitType: "timer",
		TimeoutNS: int64(1 * time.Hour),
	})
	if err != nil {
		t.Fatalf("failed to create gate: %v", err)
	}

	gate, _ := store.Get(ctx, id)
	gate.CreatedAt = time.Now().Add(-2 * time.Hour)
	if err := store.Update(ctx, gate); err != nil {
		t.Fatalf("failed to backdate gate: %v", err)
	}

	out := app.Out.(*bytes.Buffer)
	cmd := gateCheckCmd(NewTestProvider(app), nil, false)
	cmd.SetArgs([]string{"--escalate"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("gate check --escalate failed: %v", err)
	}

	output := out.String()
	if !strings.Contains(output, "escalate") {
		t.Errorf("expected 'escalate' in output, got: %s", output)
	}
	if !strings.Contains(output, "no prior resolution") {
		t.Errorf("expected 'no prior resolution' in output, got: %s", output)
	}

	// Gate should still be closed even with escalate
	closed, _ := store.Get(ctx, id)
	if closed.Status != issuestorage.StatusClosed {
		t.Errorf("expected gate to be closed even with escalate, got status %q", closed.Status)
	}
}

func TestGateCheckHumanSkipped(t *testing.T) {
	app, store := setupCheckTestApp(t)
	ctx := context.Background()

	_, err := store.Create(ctx, &issuestorage.Issue{
		Title:     "Human gate",
		Type:      issuestorage.TypeGate,
		Priority:  issuestorage.PriorityMedium,
		AwaitType: "human",
	})
	if err != nil {
		t.Fatalf("failed to create gate: %v", err)
	}

	out := app.Out.(*bytes.Buffer)
	cmd := gateCheckCmd(NewTestProvider(app), nil, false)
	cmd.SetArgs([]string{})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("gate check failed: %v", err)
	}

	output := out.String()
	if !strings.Contains(output, "skipped") {
		t.Errorf("expected 'skipped' in output, got: %s", output)
	}
	if !strings.Contains(output, "manual gate") {
		t.Errorf("expected 'manual gate' in output, got: %s", output)
	}
}

func TestGateCheckUnknownType(t *testing.T) {
	app, store := setupCheckTestApp(t)
	ctx := context.Background()

	_, err := store.Create(ctx, &issuestorage.Issue{
		Title:     "Mystery gate",
		Type:      issuestorage.TypeGate,
		Priority:  issuestorage.PriorityMedium,
		AwaitType: "alien:signal",
	})
	if err != nil {
		t.Fatalf("failed to create gate: %v", err)
	}

	out := app.Out.(*bytes.Buffer)
	cmd := gateCheckCmd(NewTestProvider(app), nil, false)
	cmd.SetArgs([]string{})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("gate check failed: %v", err)
	}

	output := out.String()
	if !strings.Contains(output, "skipped") {
		t.Errorf("expected 'skipped' in output, got: %s", output)
	}
	if !strings.Contains(output, "unknown await_type") {
		t.Errorf("expected 'unknown await_type' in output, got: %s", output)
	}
}

func TestGateCheckBeadClosed(t *testing.T) {
	app, store := setupCheckTestApp(t)
	ctx := context.Background()

	// Create a target bead and close it
	targetID, err := store.Create(ctx, &issuestorage.Issue{
		Title:    "Target bead",
		Type:     issuestorage.TypeTask,
		Priority: issuestorage.PriorityMedium,
	})
	if err != nil {
		t.Fatalf("failed to create target bead: %v", err)
	}
	if err := store.Close(ctx, targetID); err != nil {
		t.Fatalf("failed to close target bead: %v", err)
	}

	// Create a gate waiting on the target bead
	gateID, err := store.Create(ctx, &issuestorage.Issue{
		Title:     "Bead gate",
		Type:      issuestorage.TypeGate,
		Priority:  issuestorage.PriorityMedium,
		AwaitType: "bead",
		AwaitID:   targetID,
	})
	if err != nil {
		t.Fatalf("failed to create gate: %v", err)
	}

	out := app.Out.(*bytes.Buffer)
	cmd := gateCheckCmd(NewTestProvider(app), nil, false)
	cmd.SetArgs([]string{})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("gate check failed: %v", err)
	}

	output := out.String()
	if !strings.Contains(output, "resolved") {
		t.Errorf("expected 'resolved' in output, got: %s", output)
	}
	if !strings.Contains(output, "is closed") {
		t.Errorf("expected 'is closed' in output, got: %s", output)
	}

	// Verify the gate was closed
	closed, _ := store.Get(ctx, gateID)
	if closed.Status != issuestorage.StatusClosed {
		t.Errorf("expected gate to be closed, got status %q", closed.Status)
	}
}

func TestGateCheckBeadStillOpen(t *testing.T) {
	app, store := setupCheckTestApp(t)
	ctx := context.Background()

	// Create a target bead that's still open
	targetID, err := store.Create(ctx, &issuestorage.Issue{
		Title:    "Open target",
		Type:     issuestorage.TypeTask,
		Priority: issuestorage.PriorityMedium,
	})
	if err != nil {
		t.Fatalf("failed to create target bead: %v", err)
	}

	gateID, err := store.Create(ctx, &issuestorage.Issue{
		Title:     "Bead gate",
		Type:      issuestorage.TypeGate,
		Priority:  issuestorage.PriorityMedium,
		AwaitType: "bead",
		AwaitID:   targetID,
	})
	if err != nil {
		t.Fatalf("failed to create gate: %v", err)
	}

	out := app.Out.(*bytes.Buffer)
	cmd := gateCheckCmd(NewTestProvider(app), nil, false)
	cmd.SetArgs([]string{})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("gate check failed: %v", err)
	}

	output := out.String()
	if !strings.Contains(output, "pending") {
		t.Errorf("expected 'pending' in output, got: %s", output)
	}

	// Gate should remain open
	gate, _ := store.Get(ctx, gateID)
	if gate.Status != issuestorage.StatusOpen {
		t.Errorf("expected gate to remain open, got status %q", gate.Status)
	}
}

func TestGateCheckBeadNotFound(t *testing.T) {
	app, store := setupCheckTestApp(t)
	ctx := context.Background()

	_, err := store.Create(ctx, &issuestorage.Issue{
		Title:     "Orphan gate",
		Type:      issuestorage.TypeGate,
		Priority:  issuestorage.PriorityMedium,
		AwaitType: "bead",
		AwaitID:   "bd-nonexistent",
	})
	if err != nil {
		t.Fatalf("failed to create gate: %v", err)
	}

	out := app.Out.(*bytes.Buffer)
	cmd := gateCheckCmd(NewTestProvider(app), nil, false)
	cmd.SetArgs([]string{})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("gate check failed: %v", err)
	}

	output := out.String()
	if !strings.Contains(output, "pending") {
		t.Errorf("expected 'pending' in output, got: %s", output)
	}
	if !strings.Contains(output, "cannot find bead") {
		t.Errorf("expected 'cannot find bead' in output, got: %s", output)
	}
}

func TestGateCheckBeadNoAwaitID(t *testing.T) {
	app, store := setupCheckTestApp(t)
	ctx := context.Background()

	_, err := store.Create(ctx, &issuestorage.Issue{
		Title:     "No target gate",
		Type:      issuestorage.TypeGate,
		Priority:  issuestorage.PriorityMedium,
		AwaitType: "bead",
	})
	if err != nil {
		t.Fatalf("failed to create gate: %v", err)
	}

	out := app.Out.(*bytes.Buffer)
	cmd := gateCheckCmd(NewTestProvider(app), nil, false)
	cmd.SetArgs([]string{})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("gate check failed: %v", err)
	}

	output := out.String()
	if !strings.Contains(output, "no await_id configured") {
		t.Errorf("expected 'no await_id configured' in output, got: %s", output)
	}
}

func TestGateCheckGHRunSuccess(t *testing.T) {
	app, store := setupCheckTestApp(t)
	ctx := context.Background()

	gateID, err := store.Create(ctx, &issuestorage.Issue{
		Title:     "CI gate",
		Type:      issuestorage.TypeGate,
		Priority:  issuestorage.PriorityMedium,
		AwaitType: "gh:run",
		AwaitID:   "12345",
	})
	if err != nil {
		t.Fatalf("failed to create gate: %v", err)
	}

	executor := mockExecutor(map[string]struct {
		output []byte
		err    error
	}{
		"gh run view 12345 --json status,conclusion": {
			output: []byte(`{"status":"completed","conclusion":"success"}`),
		},
	})

	out := app.Out.(*bytes.Buffer)
	cmd := gateCheckCmd(NewTestProvider(app), executor, true)
	cmd.SetArgs([]string{})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("gate check failed: %v", err)
	}

	output := out.String()
	if !strings.Contains(output, "resolved") {
		t.Errorf("expected 'resolved' in output, got: %s", output)
	}
	if !strings.Contains(output, "run completed successfully") {
		t.Errorf("expected 'run completed successfully' in output, got: %s", output)
	}

	closed, _ := store.Get(ctx, gateID)
	if closed.Status != issuestorage.StatusClosed {
		t.Errorf("expected gate to be closed, got status %q", closed.Status)
	}
}

func TestGateCheckGHRunFailure(t *testing.T) {
	app, store := setupCheckTestApp(t)
	ctx := context.Background()

	gateID, err := store.Create(ctx, &issuestorage.Issue{
		Title:     "Failed CI gate",
		Type:      issuestorage.TypeGate,
		Priority:  issuestorage.PriorityMedium,
		AwaitType: "gh:run",
		AwaitID:   "99999",
	})
	if err != nil {
		t.Fatalf("failed to create gate: %v", err)
	}

	executor := mockExecutor(map[string]struct {
		output []byte
		err    error
	}{
		"gh run view 99999 --json status,conclusion": {
			output: []byte(`{"status":"completed","conclusion":"failure"}`),
		},
	})

	out := app.Out.(*bytes.Buffer)
	cmd := gateCheckCmd(NewTestProvider(app), executor, true)
	cmd.SetArgs([]string{})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("gate check failed: %v", err)
	}

	output := out.String()
	if !strings.Contains(output, "pending") {
		t.Errorf("expected 'pending' in output, got: %s", output)
	}

	// Gate should remain open
	gate, _ := store.Get(ctx, gateID)
	if gate.Status != issuestorage.StatusOpen {
		t.Errorf("expected gate to remain open, got status %q", gate.Status)
	}
}

func TestGateCheckGHRunFailureWithEscalate(t *testing.T) {
	app, store := setupCheckTestApp(t)
	ctx := context.Background()

	_, err := store.Create(ctx, &issuestorage.Issue{
		Title:     "Failed CI gate",
		Type:      issuestorage.TypeGate,
		Priority:  issuestorage.PriorityMedium,
		AwaitType: "gh:run",
		AwaitID:   "99999",
	})
	if err != nil {
		t.Fatalf("failed to create gate: %v", err)
	}

	executor := mockExecutor(map[string]struct {
		output []byte
		err    error
	}{
		"gh run view 99999 --json status,conclusion": {
			output: []byte(`{"status":"completed","conclusion":"failure"}`),
		},
	})

	out := app.Out.(*bytes.Buffer)
	cmd := gateCheckCmd(NewTestProvider(app), executor, true)
	cmd.SetArgs([]string{"--escalate"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("gate check --escalate failed: %v", err)
	}

	output := out.String()
	if !strings.Contains(output, "escalate") {
		t.Errorf("expected 'escalate' in output, got: %s", output)
	}
	if !strings.Contains(output, "conclusion: failure") {
		t.Errorf("expected 'conclusion: failure' in output, got: %s", output)
	}
}

func TestGateCheckGHRunCancelled(t *testing.T) {
	app, store := setupCheckTestApp(t)
	ctx := context.Background()

	_, err := store.Create(ctx, &issuestorage.Issue{
		Title:     "Cancelled CI",
		Type:      issuestorage.TypeGate,
		Priority:  issuestorage.PriorityMedium,
		AwaitType: "gh:run",
		AwaitID:   "55555",
	})
	if err != nil {
		t.Fatalf("failed to create gate: %v", err)
	}

	executor := mockExecutor(map[string]struct {
		output []byte
		err    error
	}{
		"gh run view 55555 --json status,conclusion": {
			output: []byte(`{"status":"completed","conclusion":"cancelled"}`),
		},
	})

	out := app.Out.(*bytes.Buffer)
	cmd := gateCheckCmd(NewTestProvider(app), executor, true)
	cmd.SetArgs([]string{"--escalate"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("gate check failed: %v", err)
	}

	output := out.String()
	if !strings.Contains(output, "escalate") {
		t.Errorf("expected 'escalate' in output, got: %s", output)
	}
	if !strings.Contains(output, "conclusion: cancelled") {
		t.Errorf("expected 'conclusion: cancelled' in output, got: %s", output)
	}
}

func TestGateCheckGHRunInProgress(t *testing.T) {
	app, store := setupCheckTestApp(t)
	ctx := context.Background()

	_, err := store.Create(ctx, &issuestorage.Issue{
		Title:     "Running CI",
		Type:      issuestorage.TypeGate,
		Priority:  issuestorage.PriorityMedium,
		AwaitType: "gh:run",
		AwaitID:   "77777",
	})
	if err != nil {
		t.Fatalf("failed to create gate: %v", err)
	}

	executor := mockExecutor(map[string]struct {
		output []byte
		err    error
	}{
		"gh run view 77777 --json status,conclusion": {
			output: []byte(`{"status":"in_progress","conclusion":""}`),
		},
	})

	out := app.Out.(*bytes.Buffer)
	cmd := gateCheckCmd(NewTestProvider(app), executor, true)
	cmd.SetArgs([]string{})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("gate check failed: %v", err)
	}

	output := out.String()
	if !strings.Contains(output, "pending") {
		t.Errorf("expected 'pending' in output, got: %s", output)
	}
	if !strings.Contains(output, "run status: in_progress") {
		t.Errorf("expected 'run status: in_progress' in output, got: %s", output)
	}
}

func TestGateCheckGHPRMerged(t *testing.T) {
	app, store := setupCheckTestApp(t)
	ctx := context.Background()

	gateID, err := store.Create(ctx, &issuestorage.Issue{
		Title:     "PR gate",
		Type:      issuestorage.TypeGate,
		Priority:  issuestorage.PriorityMedium,
		AwaitType: "gh:pr",
		AwaitID:   "42",
	})
	if err != nil {
		t.Fatalf("failed to create gate: %v", err)
	}

	executor := mockExecutor(map[string]struct {
		output []byte
		err    error
	}{
		"gh pr view 42 --json state": {
			output: []byte(`{"state":"MERGED"}`),
		},
	})

	out := app.Out.(*bytes.Buffer)
	cmd := gateCheckCmd(NewTestProvider(app), executor, true)
	cmd.SetArgs([]string{})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("gate check failed: %v", err)
	}

	output := out.String()
	if !strings.Contains(output, "resolved") {
		t.Errorf("expected 'resolved' in output, got: %s", output)
	}
	if !strings.Contains(output, "PR merged") {
		t.Errorf("expected 'PR merged' in output, got: %s", output)
	}

	closed, _ := store.Get(ctx, gateID)
	if closed.Status != issuestorage.StatusClosed {
		t.Errorf("expected gate to be closed, got status %q", closed.Status)
	}
}

func TestGateCheckGHPRClosed(t *testing.T) {
	app, store := setupCheckTestApp(t)
	ctx := context.Background()

	_, err := store.Create(ctx, &issuestorage.Issue{
		Title:     "Closed PR gate",
		Type:      issuestorage.TypeGate,
		Priority:  issuestorage.PriorityMedium,
		AwaitType: "gh:pr",
		AwaitID:   "99",
	})
	if err != nil {
		t.Fatalf("failed to create gate: %v", err)
	}

	executor := mockExecutor(map[string]struct {
		output []byte
		err    error
	}{
		"gh pr view 99 --json state": {
			output: []byte(`{"state":"CLOSED"}`),
		},
	})

	out := app.Out.(*bytes.Buffer)
	cmd := gateCheckCmd(NewTestProvider(app), executor, true)
	cmd.SetArgs([]string{"--escalate"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("gate check --escalate failed: %v", err)
	}

	output := out.String()
	if !strings.Contains(output, "escalate") {
		t.Errorf("expected 'escalate' in output, got: %s", output)
	}
	if !strings.Contains(output, "PR closed without merge") {
		t.Errorf("expected 'PR closed without merge' in output, got: %s", output)
	}
}

func TestGateCheckGHPROpen(t *testing.T) {
	app, store := setupCheckTestApp(t)
	ctx := context.Background()

	_, err := store.Create(ctx, &issuestorage.Issue{
		Title:     "Open PR gate",
		Type:      issuestorage.TypeGate,
		Priority:  issuestorage.PriorityMedium,
		AwaitType: "gh:pr",
		AwaitID:   "50",
	})
	if err != nil {
		t.Fatalf("failed to create gate: %v", err)
	}

	executor := mockExecutor(map[string]struct {
		output []byte
		err    error
	}{
		"gh pr view 50 --json state": {
			output: []byte(`{"state":"OPEN"}`),
		},
	})

	out := app.Out.(*bytes.Buffer)
	cmd := gateCheckCmd(NewTestProvider(app), executor, true)
	cmd.SetArgs([]string{})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("gate check failed: %v", err)
	}

	output := out.String()
	if !strings.Contains(output, "pending") {
		t.Errorf("expected 'pending' in output, got: %s", output)
	}
	if !strings.Contains(output, "PR state: OPEN") {
		t.Errorf("expected 'PR state: OPEN' in output, got: %s", output)
	}
}

func TestGateCheckGHNotAvailable(t *testing.T) {
	app, store := setupCheckTestApp(t)
	ctx := context.Background()

	_, err := store.Create(ctx, &issuestorage.Issue{
		Title:     "GH Run gate",
		Type:      issuestorage.TypeGate,
		Priority:  issuestorage.PriorityMedium,
		AwaitType: "gh:run",
		AwaitID:   "12345",
	})
	if err != nil {
		t.Fatalf("failed to create gh:run gate: %v", err)
	}

	_, err = store.Create(ctx, &issuestorage.Issue{
		Title:     "GH PR gate",
		Type:      issuestorage.TypeGate,
		Priority:  issuestorage.PriorityMedium,
		AwaitType: "gh:pr",
		AwaitID:   "42",
	})
	if err != nil {
		t.Fatalf("failed to create gh:pr gate: %v", err)
	}

	out := app.Out.(*bytes.Buffer)
	// ghAvailable = false
	cmd := gateCheckCmd(NewTestProvider(app), nil, false)
	cmd.SetArgs([]string{})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("gate check failed: %v", err)
	}

	output := out.String()
	if !strings.Contains(output, "skipped") {
		t.Errorf("expected 'skipped' in output, got: %s", output)
	}
	if !strings.Contains(output, "gh CLI not available") {
		t.Errorf("expected 'gh CLI not available' in output, got: %s", output)
	}
}

func TestGateCheckTypeFilter(t *testing.T) {
	app, store := setupCheckTestApp(t)
	ctx := context.Background()

	// Create a timer gate (expired)
	timerID, err := store.Create(ctx, &issuestorage.Issue{
		Title:     "Timer gate",
		Type:      issuestorage.TypeGate,
		Priority:  issuestorage.PriorityMedium,
		AwaitType: "timer",
		TimeoutNS: int64(1 * time.Hour),
	})
	if err != nil {
		t.Fatalf("failed to create timer gate: %v", err)
	}
	gate, _ := store.Get(ctx, timerID)
	gate.CreatedAt = time.Now().Add(-2 * time.Hour)
	if err := store.Update(ctx, gate); err != nil {
		t.Fatalf("failed to backdate: %v", err)
	}

	// Create a human gate
	_, err = store.Create(ctx, &issuestorage.Issue{
		Title:     "Human gate",
		Type:      issuestorage.TypeGate,
		Priority:  issuestorage.PriorityMedium,
		AwaitType: "human",
	})
	if err != nil {
		t.Fatalf("failed to create human gate: %v", err)
	}

	out := app.Out.(*bytes.Buffer)
	cmd := gateCheckCmd(NewTestProvider(app), nil, false)
	cmd.SetArgs([]string{"--type", "timer"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("gate check --type timer failed: %v", err)
	}

	output := out.String()
	// Should only see the timer gate
	if !strings.Contains(output, "resolved") {
		t.Errorf("expected 'resolved' for timer gate, got: %s", output)
	}
	// Should NOT see the human gate
	if strings.Contains(output, "human") {
		t.Errorf("expected no human gate in output, got: %s", output)
	}
	if !strings.Contains(output, "Checked 1 gate(s)") {
		t.Errorf("expected 'Checked 1 gate(s)' in summary, got: %s", output)
	}
}

func TestGateCheckDryRun(t *testing.T) {
	app, store := setupCheckTestApp(t)
	ctx := context.Background()

	id, err := store.Create(ctx, &issuestorage.Issue{
		Title:     "Dry run gate",
		Type:      issuestorage.TypeGate,
		Priority:  issuestorage.PriorityMedium,
		AwaitType: "timer",
		TimeoutNS: int64(1 * time.Hour),
	})
	if err != nil {
		t.Fatalf("failed to create gate: %v", err)
	}

	gate, _ := store.Get(ctx, id)
	gate.CreatedAt = time.Now().Add(-2 * time.Hour)
	if err := store.Update(ctx, gate); err != nil {
		t.Fatalf("failed to backdate: %v", err)
	}

	out := app.Out.(*bytes.Buffer)
	cmd := gateCheckCmd(NewTestProvider(app), nil, false)
	cmd.SetArgs([]string{"--dry-run"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("gate check --dry-run failed: %v", err)
	}

	output := out.String()
	if !strings.Contains(output, "resolved") {
		t.Errorf("expected 'resolved' in dry-run output, got: %s", output)
	}

	// Gate should NOT be closed in dry-run mode
	stillOpen, _ := store.Get(ctx, id)
	if stillOpen.Status != issuestorage.StatusOpen {
		t.Errorf("expected gate to remain open in dry-run mode, got status %q", stillOpen.Status)
	}
}

func TestGateCheckNoGates(t *testing.T) {
	app, store := setupCheckTestApp(t)
	ctx := context.Background()

	// Create a non-gate issue (should not appear)
	_, err := store.Create(ctx, &issuestorage.Issue{
		Title:    "Regular task",
		Type:     issuestorage.TypeTask,
		Priority: issuestorage.PriorityMedium,
	})
	if err != nil {
		t.Fatalf("failed to create task: %v", err)
	}

	out := app.Out.(*bytes.Buffer)
	cmd := gateCheckCmd(NewTestProvider(app), nil, false)
	cmd.SetArgs([]string{})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("gate check failed: %v", err)
	}

	output := out.String()
	if !strings.Contains(output, "No open gates found.") {
		t.Errorf("expected 'No open gates found.' message, got: %s", output)
	}
}

func TestGateCheckJSONOutput(t *testing.T) {
	app, store := setupCheckTestApp(t)
	app.JSON = true
	ctx := context.Background()

	// Create an expired timer gate
	id, err := store.Create(ctx, &issuestorage.Issue{
		Title:     "Timer gate",
		Type:      issuestorage.TypeGate,
		Priority:  issuestorage.PriorityMedium,
		AwaitType: "timer",
		TimeoutNS: int64(1 * time.Hour),
	})
	if err != nil {
		t.Fatalf("failed to create gate: %v", err)
	}
	gate, _ := store.Get(ctx, id)
	gate.CreatedAt = time.Now().Add(-2 * time.Hour)
	if err := store.Update(ctx, gate); err != nil {
		t.Fatalf("failed to backdate: %v", err)
	}

	// Create a human gate
	humanID, err := store.Create(ctx, &issuestorage.Issue{
		Title:     "Human gate",
		Type:      issuestorage.TypeGate,
		Priority:  issuestorage.PriorityMedium,
		AwaitType: "human",
	})
	if err != nil {
		t.Fatalf("failed to create human gate: %v", err)
	}

	out := app.Out.(*bytes.Buffer)
	cmd := gateCheckCmd(NewTestProvider(app), nil, false)
	cmd.SetArgs([]string{})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("gate check JSON failed: %v", err)
	}

	var results []GateCheckResultJSON
	if err := json.Unmarshal(out.Bytes(), &results); err != nil {
		t.Fatalf("failed to parse JSON: %v\noutput: %s", err, out.String())
	}

	if len(results) != 2 {
		t.Fatalf("expected 2 results, got %d", len(results))
	}

	// Find results by gate_id
	resultMap := map[string]GateCheckResultJSON{}
	for _, r := range results {
		resultMap[r.GateID] = r
	}

	timerResult := resultMap[id]
	if timerResult.Result != "resolved" {
		t.Errorf("expected timer gate result 'resolved', got %q", timerResult.Result)
	}
	if timerResult.AwaitType != "timer" {
		t.Errorf("expected await_type 'timer', got %q", timerResult.AwaitType)
	}

	humanResult := resultMap[humanID]
	if humanResult.Result != "skipped" {
		t.Errorf("expected human gate result 'skipped', got %q", humanResult.Result)
	}
}

func TestGateCheckJSONEmptyOutput(t *testing.T) {
	app, _ := setupCheckTestApp(t)
	app.JSON = true

	out := app.Out.(*bytes.Buffer)
	cmd := gateCheckCmd(NewTestProvider(app), nil, false)
	cmd.SetArgs([]string{})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("gate check JSON empty failed: %v", err)
	}

	var results []GateCheckResultJSON
	if err := json.Unmarshal(out.Bytes(), &results); err != nil {
		t.Fatalf("failed to parse JSON: %v\noutput: %s", err, out.String())
	}

	if len(results) != 0 {
		t.Errorf("expected empty array, got %d results", len(results))
	}
}

func TestGateCheckGHRunBadJSON(t *testing.T) {
	app, store := setupCheckTestApp(t)
	ctx := context.Background()

	_, err := store.Create(ctx, &issuestorage.Issue{
		Title:     "Bad JSON gate",
		Type:      issuestorage.TypeGate,
		Priority:  issuestorage.PriorityMedium,
		AwaitType: "gh:run",
		AwaitID:   "88888",
	})
	if err != nil {
		t.Fatalf("failed to create gate: %v", err)
	}

	executor := mockExecutor(map[string]struct {
		output []byte
		err    error
	}{
		"gh run view 88888 --json status,conclusion": {
			output: []byte(`not valid json`),
		},
	})

	out := app.Out.(*bytes.Buffer)
	cmd := gateCheckCmd(NewTestProvider(app), executor, true)
	cmd.SetArgs([]string{})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("gate check failed: %v", err)
	}

	output := out.String()
	if !strings.Contains(output, "pending") {
		t.Errorf("expected 'pending' for bad JSON, got: %s", output)
	}
	if !strings.Contains(output, "failed to parse gh output") {
		t.Errorf("expected parse error message, got: %s", output)
	}
}

func TestGateCheckGHRunCommandError(t *testing.T) {
	app, store := setupCheckTestApp(t)
	ctx := context.Background()

	_, err := store.Create(ctx, &issuestorage.Issue{
		Title:     "Error gate",
		Type:      issuestorage.TypeGate,
		Priority:  issuestorage.PriorityMedium,
		AwaitType: "gh:run",
		AwaitID:   "44444",
	})
	if err != nil {
		t.Fatalf("failed to create gate: %v", err)
	}

	executor := mockExecutor(map[string]struct {
		output []byte
		err    error
	}{
		"gh run view 44444 --json status,conclusion": {
			err: fmt.Errorf("exit status 1"),
		},
	})

	out := app.Out.(*bytes.Buffer)
	cmd := gateCheckCmd(NewTestProvider(app), executor, true)
	cmd.SetArgs([]string{})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("gate check failed: %v", err)
	}

	output := out.String()
	if !strings.Contains(output, "pending") {
		t.Errorf("expected 'pending' for command error, got: %s", output)
	}
	if !strings.Contains(output, "gh run view failed") {
		t.Errorf("expected 'gh run view failed' message, got: %s", output)
	}
}

func TestGateCheckMultipleGatesMixed(t *testing.T) {
	app, store := setupCheckTestApp(t)
	ctx := context.Background()

	// Expired timer → resolved
	timerID, _ := store.Create(ctx, &issuestorage.Issue{
		Title:     "Expired timer",
		Type:      issuestorage.TypeGate,
		Priority:  issuestorage.PriorityMedium,
		AwaitType: "timer",
		TimeoutNS: int64(1 * time.Hour),
	})
	gate, _ := store.Get(ctx, timerID)
	gate.CreatedAt = time.Now().Add(-2 * time.Hour)
	store.Update(ctx, gate)

	// Human → skipped
	store.Create(ctx, &issuestorage.Issue{
		Title:     "Human gate",
		Type:      issuestorage.TypeGate,
		Priority:  issuestorage.PriorityMedium,
		AwaitType: "human",
	})

	// Open bead → pending
	targetID, _ := store.Create(ctx, &issuestorage.Issue{
		Title:    "Open bead",
		Type:     issuestorage.TypeTask,
		Priority: issuestorage.PriorityMedium,
	})
	store.Create(ctx, &issuestorage.Issue{
		Title:     "Bead gate",
		Type:      issuestorage.TypeGate,
		Priority:  issuestorage.PriorityMedium,
		AwaitType: "bead",
		AwaitID:   targetID,
	})

	out := app.Out.(*bytes.Buffer)
	cmd := gateCheckCmd(NewTestProvider(app), nil, false)
	cmd.SetArgs([]string{})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("gate check failed: %v", err)
	}

	output := out.String()
	if !strings.Contains(output, "Checked 3 gate(s)") {
		t.Errorf("expected 'Checked 3 gate(s)' in summary, got: %s", output)
	}
	if !strings.Contains(output, "1 resolved") {
		t.Errorf("expected '1 resolved' in summary, got: %s", output)
	}
	if !strings.Contains(output, "1 pending") {
		t.Errorf("expected '1 pending' in summary, got: %s", output)
	}
	if !strings.Contains(output, "1 skipped") {
		t.Errorf("expected '1 skipped' in summary, got: %s", output)
	}
}

func TestGateCheckGHRunNoAwaitID(t *testing.T) {
	app, store := setupCheckTestApp(t)
	ctx := context.Background()

	_, err := store.Create(ctx, &issuestorage.Issue{
		Title:     "No ID gh:run gate",
		Type:      issuestorage.TypeGate,
		Priority:  issuestorage.PriorityMedium,
		AwaitType: "gh:run",
	})
	if err != nil {
		t.Fatalf("failed to create gate: %v", err)
	}

	out := app.Out.(*bytes.Buffer)
	cmd := gateCheckCmd(NewTestProvider(app), nil, true)
	cmd.SetArgs([]string{})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("gate check failed: %v", err)
	}

	output := out.String()
	if !strings.Contains(output, "no await_id configured") {
		t.Errorf("expected 'no await_id configured' in output, got: %s", output)
	}
}

func TestGateCheckGHPRClosedNoEscalate(t *testing.T) {
	app, store := setupCheckTestApp(t)
	ctx := context.Background()

	_, err := store.Create(ctx, &issuestorage.Issue{
		Title:     "Closed PR",
		Type:      issuestorage.TypeGate,
		Priority:  issuestorage.PriorityMedium,
		AwaitType: "gh:pr",
		AwaitID:   "99",
	})
	if err != nil {
		t.Fatalf("failed to create gate: %v", err)
	}

	executor := mockExecutor(map[string]struct {
		output []byte
		err    error
	}{
		"gh pr view 99 --json state": {
			output: []byte(`{"state":"CLOSED"}`),
		},
	})

	out := app.Out.(*bytes.Buffer)
	// Without --escalate flag
	cmd := gateCheckCmd(NewTestProvider(app), executor, true)
	cmd.SetArgs([]string{})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("gate check failed: %v", err)
	}

	output := out.String()
	// Without --escalate, closed PR should be "pending" not "escalate"
	if !strings.Contains(output, "pending") {
		t.Errorf("expected 'pending' for closed PR without --escalate, got: %s", output)
	}
	if strings.Contains(output, "escalate") {
		t.Errorf("expected no 'escalate' without --escalate flag, got: %s", output)
	}
}
