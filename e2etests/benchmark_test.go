package e2etests

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"strings"
	"testing"
	"time"

	"beads-lite/e2etests/reference"
)

var compare = flag.Bool("compare", false, "compare against BD_REF_CMD")

const (
	benchCreateCount = 20
	benchListCount   = 10
	benchShowPerID   = 5
)

type phaseResult struct {
	name     string
	duration time.Duration
}

func TestBenchmark(t *testing.T) {
	bdCmd := os.Getenv("BD_CMD")
	if bdCmd == "" {
		t.Skip("BD_CMD environment variable not set")
	}

	if *compare {
		refCmd := os.Getenv("BD_REF_CMD")
		if refCmd == "" {
			t.Fatal("compare mode requires BD_REF_CMD environment variable")
		}

		refRunner := &reference.Runner{BdCmd: refCmd, KillDaemons: true}
		liteRunner := &reference.Runner{BdCmd: bdCmd}

		t.Log("Running benchmark against reference binary...")
		refResults := runBenchmarkWorkflow(t, refRunner, "bd (reference)")

		t.Log("Running benchmark against beads-lite binary...")
		liteResults := runBenchmarkWorkflow(t, liteRunner, "beads-lite")

		printComparisonTable(t, liteResults, refResults)
	} else {
		r := &reference.Runner{BdCmd: bdCmd}
		results := runBenchmarkWorkflow(t, r, "beads-lite")
		printSingleResults(t, results)
	}
}

func runBenchmarkWorkflow(t *testing.T, r *reference.Runner, label string) []phaseResult {
	t.Helper()

	sandbox, err := r.SetupSandbox()
	if err != nil {
		t.Fatalf("[%s] setup sandbox: %v", label, err)
	}
	t.Cleanup(func() { r.TeardownSandbox(sandbox) })

	var results []phaseResult
	var ids []string

	// Phase 1: Create issues
	start := time.Now()
	for i := 1; i <= benchCreateCount; i++ {
		result := r.Run(sandbox, "create", fmt.Sprintf("Task %d", i), "--json")
		if result.ExitCode != 0 {
			t.Fatalf("[%s] create task %d: exit %d, stderr: %s", label, i, result.ExitCode, result.Stderr)
		}
		id := reference.ExtractID([]byte(result.Stdout))
		if id == "" {
			t.Fatalf("[%s] create task %d: could not extract ID from: %s", label, i, result.Stdout)
		}
		ids = append(ids, id)
	}
	results = append(results, phaseResult{
		name:     fmt.Sprintf("create (%d)", benchCreateCount),
		duration: time.Since(start),
	})

	// Phase 2: List issues
	start = time.Now()
	for i := 0; i < benchListCount; i++ {
		result := r.Run(sandbox, "list", "--json")
		if result.ExitCode != 0 {
			t.Fatalf("[%s] list iteration %d: exit %d, stderr: %s", label, i, result.ExitCode, result.Stderr)
		}
		count := countJSONIssues(t, result.Stdout)
		if count != benchCreateCount {
			t.Fatalf("[%s] list iteration %d: expected %d issues, got %d", label, i, benchCreateCount, count)
		}
	}
	results = append(results, phaseResult{
		name:     fmt.Sprintf("list (%dx)", benchListCount),
		duration: time.Since(start),
	})

	// Phase 3: Show each issue multiple times
	start = time.Now()
	for _, id := range ids {
		for j := 0; j < benchShowPerID; j++ {
			result := r.Run(sandbox, "show", id, "--json")
			if result.ExitCode != 0 {
				t.Fatalf("[%s] show %s (iter %d): exit %d, stderr: %s", label, id, j, result.ExitCode, result.Stderr)
			}
		}
	}
	results = append(results, phaseResult{
		name:     fmt.Sprintf("show (%dx)", benchCreateCount*benchShowPerID),
		duration: time.Since(start),
	})

	// Phase 4: Update each issue
	start = time.Now()
	for _, id := range ids {
		result := r.Run(sandbox, "update", id, "--status", "in-progress")
		if result.ExitCode != 0 {
			t.Fatalf("[%s] update %s: exit %d, stderr: %s", label, id, result.ExitCode, result.Stderr)
		}
	}
	results = append(results, phaseResult{
		name:     fmt.Sprintf("update (%d)", benchCreateCount),
		duration: time.Since(start),
	})

	// Phase 5: Close each issue
	start = time.Now()
	for _, id := range ids {
		result := r.Run(sandbox, "close", id)
		if result.ExitCode != 0 {
			t.Fatalf("[%s] close %s: exit %d, stderr: %s", label, id, result.ExitCode, result.Stderr)
		}
	}
	results = append(results, phaseResult{
		name:     fmt.Sprintf("close (%d)", benchCreateCount),
		duration: time.Since(start),
	})

	// Phase 6: Final list - verify empty
	start = time.Now()
	result := r.Run(sandbox, "list", "--json")
	if result.ExitCode != 0 {
		t.Fatalf("[%s] final list: exit %d, stderr: %s", label, result.ExitCode, result.Stderr)
	}
	count := countJSONIssues(t, result.Stdout)
	if count != 0 {
		t.Fatalf("[%s] final list: expected 0 open issues, got %d", label, count)
	}
	results = append(results, phaseResult{
		name:     "final list",
		duration: time.Since(start),
	})

	return results
}

func printSingleResults(t *testing.T, results []phaseResult) {
	t.Helper()

	var total time.Duration
	t.Logf("%-20s %s", "Phase", "Time")
	t.Logf("%s", strings.Repeat("─", 35))
	for _, r := range results {
		t.Logf("%-20s %s", r.name, formatDuration(r.duration))
		total += r.duration
	}
	t.Logf("%s", strings.Repeat("─", 35))
	t.Logf("%-20s %s", "TOTAL", formatDuration(total))
}

func printComparisonTable(t *testing.T, lite, ref []phaseResult) {
	t.Helper()

	var liteTotal, refTotal time.Duration
	t.Logf("%-20s %14s %14s %10s", "Phase", "beads-lite", "bd (reference)", "diff")
	t.Logf("%s", strings.Repeat("─", 62))
	for i := range lite {
		diff := percentDiff(lite[i].duration, ref[i].duration)
		t.Logf("%-20s %14s %14s %10s",
			lite[i].name,
			formatDuration(lite[i].duration),
			formatDuration(ref[i].duration),
			diff,
		)
		liteTotal += lite[i].duration
		refTotal += ref[i].duration
	}
	t.Logf("%s", strings.Repeat("─", 62))
	t.Logf("%-20s %14s %14s %10s",
		"TOTAL",
		formatDuration(liteTotal),
		formatDuration(refTotal),
		percentDiff(liteTotal, refTotal),
	)
}

func formatDuration(d time.Duration) string {
	return fmt.Sprintf("%.2fs", d.Seconds())
}

func percentDiff(a, b time.Duration) string {
	if b == 0 {
		return "N/A"
	}
	pct := (float64(a) - float64(b)) / float64(b) * 100
	return fmt.Sprintf("%+.1f%%", pct)
}

func countJSONIssues(t *testing.T, jsonOutput string) int {
	t.Helper()
	jsonOutput = strings.TrimSpace(jsonOutput)
	if jsonOutput == "" || jsonOutput == "null" {
		return 0
	}
	var issues []json.RawMessage
	if err := json.Unmarshal([]byte(jsonOutput), &issues); err != nil {
		t.Fatalf("parsing JSON list output: %v\nraw: %s", err, jsonOutput)
	}
	return len(issues)
}
