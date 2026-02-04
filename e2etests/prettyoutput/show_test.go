package prettyoutput

import (
	"os"
	"strings"
	"testing"

	"beads-lite/e2etests"
)

func TestShowColorOutput(t *testing.T) {
	bdCmd := os.Getenv("BD_CMD")
	if bdCmd == "" {
		t.Skip("BD_CMD environment variable not set")
	}

	r := &e2etests.Runner{
		BdCmd:    bdCmd,
		ExtraEnv: []string{"CLICOLOR_FORCE=1"},
	}

	sandbox, err := r.SetupSandbox()
	if err != nil {
		t.Fatalf("setup sandbox: %v", err)
	}
	t.Cleanup(func() { r.TeardownSandbox(sandbox) })

	// Create a task with full metadata
	res := r.Run(sandbox, "create", "Main task",
		"--priority=P1", "--type=bug",
		"--assignee=Alice", "--description=This is the description.",
		"--labels=urgent,v2", "--json")
	if res.ExitCode != 0 {
		t.Fatalf("create main task: exit %d, stderr: %s", res.ExitCode, res.Stderr)
	}
	mainID := e2etests.ExtractID([]byte(res.Stdout))

	// Create upstream dependency
	res = r.Run(sandbox, "create", "upstream dep", "--priority=P2", "--type=task", "--json")
	if res.ExitCode != 0 {
		t.Fatalf("create upstream: exit %d, stderr: %s", res.ExitCode, res.Stderr)
	}
	upstreamID := e2etests.ExtractID([]byte(res.Stdout))

	// Create downstream blocker
	res = r.Run(sandbox, "create", "downstream blocker", "--priority=P3", "--type=feature", "--json")
	if res.ExitCode != 0 {
		t.Fatalf("create downstream: exit %d, stderr: %s", res.ExitCode, res.Stderr)
	}
	downstreamID := e2etests.ExtractID([]byte(res.Stdout))

	// Wire up: main depends on upstream
	res = r.Run(sandbox, "dep", "add", mainID, upstreamID)
	if res.ExitCode != 0 {
		t.Fatalf("dep add: exit %d, stderr: %s", res.ExitCode, res.Stderr)
	}
	// downstream depends on main
	res = r.Run(sandbox, "dep", "add", downstreamID, mainID)
	if res.ExitCode != 0 {
		t.Fatalf("dep add: exit %d, stderr: %s", res.ExitCode, res.Stderr)
	}

	// Show the main issue
	res = r.Run(sandbox, "show", mainID)
	if res.ExitCode != 0 {
		t.Fatalf("show: exit %d, stderr: %s", res.ExitCode, res.Stderr)
	}

	output := res.Stdout

	// ANSI escape codes present
	if !strings.Contains(output, "\033[") {
		t.Errorf("expected ANSI escape codes in colored output")
	}

	// Header line: status icon, id, · separator, title, priority, status
	if !strings.Contains(output, "○") {
		t.Errorf("expected open status icon ○")
	}
	if !strings.Contains(output, mainID+" · Main task") {
		t.Errorf("expected '%s · Main task' in header, got:\n%s", mainID, output)
	}
	if !strings.Contains(output, "P1") {
		t.Errorf("expected P1 in output")
	}
	if !strings.Contains(output, "OPEN") {
		t.Errorf("expected OPEN in status bracket")
	}

	// Metadata line
	if !strings.Contains(output, "Assignee: Alice") {
		t.Errorf("expected 'Assignee: Alice' in metadata")
	}
	if !strings.Contains(output, "Type: bug") {
		t.Errorf("expected 'Type: bug' in metadata")
	}

	// Description section (sentence case)
	if !strings.Contains(output, "Description") {
		t.Errorf("expected Description section header")
	}
	if !strings.Contains(output, "  This is the description.") {
		t.Errorf("expected indented description body")
	}

	// Labels section
	if !strings.Contains(output, "Labels: urgent, v2") {
		t.Errorf("expected labels line")
	}

	// Depends On section with enriched line
	if !strings.Contains(output, "Depends On") {
		t.Errorf("expected 'Depends On' section")
	}
	if !strings.Contains(output, "→") {
		t.Errorf("expected → prefix in Depends On section")
	}
	if !strings.Contains(output, upstreamID) {
		t.Errorf("expected upstream ID %s in Depends On", upstreamID)
	}
	if !strings.Contains(output, "upstream dep") {
		t.Errorf("expected upstream title in Depends On section")
	}

	// Blocks section with enriched line
	if !strings.Contains(output, "Blocks") {
		t.Errorf("expected 'Blocks' section")
	}
	if !strings.Contains(output, "←") {
		t.Errorf("expected ← prefix in Blocks section")
	}
	if !strings.Contains(output, downstreamID) {
		t.Errorf("expected downstream ID %s in Blocks", downstreamID)
	}
	if !strings.Contains(output, "downstream blocker") {
		t.Errorf("expected downstream title in Blocks section")
	}
}

func TestShowEpicWithChildren(t *testing.T) {
	bdCmd := os.Getenv("BD_CMD")
	if bdCmd == "" {
		t.Skip("BD_CMD environment variable not set")
	}

	r := &e2etests.Runner{
		BdCmd:    bdCmd,
		ExtraEnv: []string{"CLICOLOR_FORCE=1"},
	}

	sandbox, err := r.SetupSandbox()
	if err != nil {
		t.Fatalf("setup sandbox: %v", err)
	}
	t.Cleanup(func() { r.TeardownSandbox(sandbox) })

	// Create epic
	res := r.Run(sandbox, "create", "TestEpic", "--type=epic", "--priority=P2", "--json")
	if res.ExitCode != 0 {
		t.Fatalf("create epic: exit %d, stderr: %s", res.ExitCode, res.Stderr)
	}
	epicID := e2etests.ExtractID([]byte(res.Stdout))

	// Create child tasks under the epic
	res = r.Run(sandbox, "create", "child task 1", "--parent="+epicID, "--priority=P1", "--type=task", "--json")
	if res.ExitCode != 0 {
		t.Fatalf("create child 1: exit %d, stderr: %s", res.ExitCode, res.Stderr)
	}
	child1ID := e2etests.ExtractID([]byte(res.Stdout))

	res = r.Run(sandbox, "create", "child task 2", "--parent="+epicID, "--priority=P3", "--type=bug", "--json")
	if res.ExitCode != 0 {
		t.Fatalf("create child 2: exit %d, stderr: %s", res.ExitCode, res.Stderr)
	}
	child2ID := e2etests.ExtractID([]byte(res.Stdout))

	// Show the epic
	res = r.Run(sandbox, "show", epicID)
	if res.ExitCode != 0 {
		t.Fatalf("show epic: exit %d, stderr: %s", res.ExitCode, res.Stderr)
	}

	output := res.Stdout

	// Header should have EPIC tag (ANSI codes may be inside the brackets)
	if !strings.Contains(output, "EPIC") {
		t.Errorf("expected EPIC in header, got:\n%s", output)
	}

	// Children section
	if !strings.Contains(output, "Children") {
		t.Errorf("expected Children section, got:\n%s", output)
	}
	if !strings.Contains(output, "↳") {
		t.Errorf("expected ↳ prefix in Children section")
	}
	if !strings.Contains(output, child1ID) {
		t.Errorf("expected child 1 ID %s in Children", child1ID)
	}
	if !strings.Contains(output, child2ID) {
		t.Errorf("expected child 2 ID %s in Children", child2ID)
	}
	if !strings.Contains(output, "child task 1") {
		t.Errorf("expected child 1 title in Children section")
	}
	if !strings.Contains(output, "child task 2") {
		t.Errorf("expected child 2 title in Children section")
	}

	// Children should NOT be duplicated in Blocks
	if strings.Contains(output, "Blocks") {
		t.Errorf("parent-child deps should not appear in Blocks section, got:\n%s", output)
	}

	// Show a child — parent should be enriched, not show Depends On
	res = r.Run(sandbox, "show", child1ID)
	if res.ExitCode != 0 {
		t.Fatalf("show child: exit %d, stderr: %s", res.ExitCode, res.Stderr)
	}

	output = res.Stdout
	if !strings.Contains(output, "Parent") {
		t.Errorf("expected Parent section for child, got:\n%s", output)
	}
	if !strings.Contains(output, epicID) {
		t.Errorf("expected parent epic ID %s, got:\n%s", epicID, output)
	}
	// Parent-child should NOT appear in Depends On
	if strings.Contains(output, "Depends On") {
		t.Errorf("parent-child should not appear in Depends On, got:\n%s", output)
	}
}
