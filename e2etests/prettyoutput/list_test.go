package prettyoutput

import (
	"os"
	"strings"
	"testing"

	"beads-lite/e2etests"
)

// setupListSandbox creates a sandbox with issues in various states for list tests.
// Returns the sandbox path and a cleanup function.
func setupListSandbox(t *testing.T, r *e2etests.Runner) string {
	t.Helper()

	sandbox, err := r.SetupSandbox()
	if err != nil {
		t.Fatalf("setup sandbox: %v", err)
	}
	t.Cleanup(func() { r.TeardownSandbox(sandbox) })

	// Create issues with different priorities, types, statuses, and assignees.
	// P0 critical task, assigned
	res := r.Run(sandbox, "create", "Critical task", "--priority=P0", "--type=task", "--assignee=Danny Cosson", "--json")
	if res.ExitCode != 0 {
		t.Fatalf("create critical task: exit %d, stderr: %s", res.ExitCode, res.Stderr)
	}
	criticalID := e2etests.ExtractID([]byte(res.Stdout))

	// P1 high bug, in-progress
	res = r.Run(sandbox, "create", "High bug", "--priority=P1", "--type=bug", "--json")
	if res.ExitCode != 0 {
		t.Fatalf("create high bug: exit %d, stderr: %s", res.ExitCode, res.Stderr)
	}
	highBugID := e2etests.ExtractID([]byte(res.Stdout))
	res = r.Run(sandbox, "update", highBugID, "--status=in-progress")
	if res.ExitCode != 0 {
		t.Fatalf("update high bug to in-progress: exit %d, stderr: %s", res.ExitCode, res.Stderr)
	}

	// P2 medium epic
	res = r.Run(sandbox, "create", "Medium epic", "--priority=P2", "--type=epic", "--json")
	if res.ExitCode != 0 {
		t.Fatalf("create medium epic: exit %d, stderr: %s", res.ExitCode, res.Stderr)
	}

	// P3 low feature, closed
	res = r.Run(sandbox, "create", "Low feature", "--priority=P3", "--type=feature", "--json")
	if res.ExitCode != 0 {
		t.Fatalf("create low feature: exit %d, stderr: %s", res.ExitCode, res.Stderr)
	}
	lowFeatureID := e2etests.ExtractID([]byte(res.Stdout))
	res = r.Run(sandbox, "close", lowFeatureID)
	if res.ExitCode != 0 {
		t.Fatalf("close low feature: exit %d, stderr: %s", res.ExitCode, res.Stderr)
	}

	// P4 backlog task
	res = r.Run(sandbox, "create", "Backlog task", "--priority=P4", "--type=task", "--json")
	if res.ExitCode != 0 {
		t.Fatalf("create backlog task: exit %d, stderr: %s", res.ExitCode, res.Stderr)
	}

	_ = criticalID // used implicitly via list output
	return sandbox
}

func TestListColorOutput(t *testing.T) {
	bdCmd := os.Getenv("BD_CMD")
	if bdCmd == "" {
		t.Skip("BD_CMD environment variable not set")
	}

	r := &e2etests.Runner{
		BdCmd:    bdCmd,
		ExtraEnv: []string{"CLICOLOR_FORCE=1"},
	}

	sandbox := setupListSandbox(t, r)

	res := r.Run(sandbox, "list", "--all")
	if res.ExitCode != 0 {
		t.Fatalf("list --all: exit %d, stderr: %s", res.ExitCode, res.Stderr)
	}

	output := res.Stdout

	// Verify ANSI escape codes are present
	if !strings.Contains(output, "\033[") {
		t.Errorf("expected ANSI escape codes in colored output, got:\n%s", output)
	}

	// Verify status icons are present
	if !strings.Contains(output, "○") {
		t.Errorf("expected open status icon ○, got:\n%s", output)
	}
	if !strings.Contains(output, "◐") {
		t.Errorf("expected in-progress status icon ◐, got:\n%s", output)
	}
	if !strings.Contains(output, "✓") {
		t.Errorf("expected closed status icon ✓, got:\n%s", output)
	}

	// Verify priority display format (P0-P4)
	if !strings.Contains(output, "P0") {
		t.Errorf("expected P0 in output, got:\n%s", output)
	}
	if !strings.Contains(output, "P1") {
		t.Errorf("expected P1 in output, got:\n%s", output)
	}

	// Verify closed issue has no dot: [P3] (not [● P3])
	// Find the line with "Low feature" (closed)
	for _, line := range strings.Split(output, "\n") {
		if strings.Contains(line, "Low feature") {
			if strings.Contains(line, "●") {
				t.Errorf("closed issue should not have ● in priority bracket, got: %s", line)
			}
			if !strings.Contains(line, "✓") {
				t.Errorf("closed issue should have ✓ icon, got: %s", line)
			}
		}
	}

	// Verify non-closed issues have dot: [● P0], [● P1], etc.
	for _, line := range strings.Split(output, "\n") {
		if strings.Contains(line, "Critical task") || strings.Contains(line, "High bug") || strings.Contains(line, "Medium epic") || strings.Contains(line, "Backlog task") {
			if !strings.Contains(line, "●") {
				t.Errorf("non-closed issue should have ● in priority bracket, got: %s", line)
			}
		}
	}

	// Verify assignee format
	if !strings.Contains(output, "@Danny Cosson") {
		t.Errorf("expected @Danny Cosson in output, got:\n%s", output)
	}

	// Verify title format with " - " separator
	if !strings.Contains(output, "- Critical task") {
		t.Errorf("expected '- Critical task' in output, got:\n%s", output)
	}

	// Verify type brackets
	if !strings.Contains(output, "task") {
		t.Errorf("expected [task] in output, got:\n%s", output)
	}
	if !strings.Contains(output, "bug") {
		t.Errorf("expected [bug] in output, got:\n%s", output)
	}
	if !strings.Contains(output, "epic") {
		t.Errorf("expected [epic] in output, got:\n%s", output)
	}
	if !strings.Contains(output, "feature") {
		t.Errorf("expected [feature] in output, got:\n%s", output)
	}

	// Verify specific color codes are present
	// In-progress status icon should be yellow-orange (38;5;214)
	if !strings.Contains(output, "\033[38;5;214m") {
		t.Errorf("expected yellow-orange color code (38;5;214) in output")
	}
	// Closed status icon should be grey (90)
	if !strings.Contains(output, "\033[90m") {
		t.Errorf("expected grey color code (90) in output")
	}
	// P0 critical should be red (31)
	if !strings.Contains(output, "\033[31m") {
		t.Errorf("expected red color code (31) in output")
	}
	// Epic type should be purple (35)
	if !strings.Contains(output, "\033[35m") {
		t.Errorf("expected purple color code (35) in output")
	}
	// Feature type should be green (32)
	if !strings.Contains(output, "\033[32m") {
		t.Errorf("expected green color code (32) in output")
	}
}

func TestListPlainOutput(t *testing.T) {
	bdCmd := os.Getenv("BD_CMD")
	if bdCmd == "" {
		t.Skip("BD_CMD environment variable not set")
	}

	// No CLICOLOR_FORCE — default piped/non-TTY behavior
	r := &e2etests.Runner{BdCmd: bdCmd}

	sandbox := setupListSandbox(t, r)

	res := r.Run(sandbox, "list", "--all")
	if res.ExitCode != 0 {
		t.Fatalf("list --all: exit %d, stderr: %s", res.ExitCode, res.Stderr)
	}

	output := res.Stdout

	// Verify NO ANSI escape codes
	if strings.Contains(output, "\033[") {
		t.Errorf("expected no ANSI escape codes in plain output, got:\n%s", output)
	}

	// Verify same content is present
	if !strings.Contains(output, "○") {
		t.Errorf("expected open status icon ○, got:\n%s", output)
	}
	if !strings.Contains(output, "◐") {
		t.Errorf("expected in-progress status icon ◐, got:\n%s", output)
	}
	if !strings.Contains(output, "✓") {
		t.Errorf("expected closed status icon ✓, got:\n%s", output)
	}
	if !strings.Contains(output, "P0") {
		t.Errorf("expected P0 in output, got:\n%s", output)
	}
	if !strings.Contains(output, "@Danny Cosson") {
		t.Errorf("expected @Danny Cosson in output, got:\n%s", output)
	}
	if !strings.Contains(output, "- Critical task") {
		t.Errorf("expected '- Critical task' in output, got:\n%s", output)
	}

	// Verify layout: each issue on one line with expected elements
	lines := strings.Split(strings.TrimSpace(output), "\n")
	if len(lines) != 5 {
		t.Errorf("expected 5 issue lines, got %d:\n%s", len(lines), output)
	}
	for _, line := range lines {
		// Each line should have format: <icon> <id> [<priority>] [<type>] ... - <title>
		if !strings.Contains(line, " - ") {
			t.Errorf("expected ' - ' separator in line: %s", line)
		}
		if !strings.Contains(line, "[") || !strings.Contains(line, "]") {
			t.Errorf("expected brackets in line: %s", line)
		}
	}
}
