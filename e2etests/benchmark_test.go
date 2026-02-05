package e2etests

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

var compare = flag.Bool("compare", false, "compare against BD_REF_CMD")

const (
	benchCreateCount    = 20
	benchListCount      = 20
	benchShowPerID      = 1
	benchListFinalCount = 20

	// Multi-repo benchmark constants
	multiRepoCount         = 4
	multiRepoIssuesPerRepo = 5
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

		refRunner := &Runner{BdCmd: refCmd, KillDaemons: true}
		liteRunner := &Runner{BdCmd: bdCmd}

		t.Log("Running benchmark against reference binary...")
		refResults := runBenchmarkWorkflow(t, refRunner, "bd (reference)")

		t.Log("Running benchmark against beads-lite binary...")
		liteResults := runBenchmarkWorkflow(t, liteRunner, "beads-lite")

		printComparisonTable(t, liteResults, refResults)
	} else {
		r := &Runner{BdCmd: bdCmd}
		results := runBenchmarkWorkflow(t, r, "beads-lite")
		printSingleResults(t, results)
	}
}

func runBenchmarkWorkflow(t *testing.T, r *Runner, label string) []phaseResult {
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
		id := ExtractID([]byte(result.Stdout))
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
	for i := 0; i < benchListFinalCount; i++ {
		result := r.Run(sandbox, "list", "--json")
		if result.ExitCode != 0 {
			t.Fatalf("[%s] final list iteration %d: exit %d, stderr: %s", label, i, result.ExitCode, result.Stderr)
		}
		count := countJSONIssues(t, result.Stdout)
		if count != 0 {
			t.Fatalf("[%s] final list iteration %d: expected 0 open issues, got %d", label, i, count)
		}
	}
	results = append(results, phaseResult{
		name:     fmt.Sprintf("final list (%dx)", benchListFinalCount),
		duration: time.Since(start),
	})

	// Phase 7-8: Multi-repo benchmark (setup + show)
	multiRepoResults := runMultiRepoPhases(t, r, label)
	results = append(results, multiRepoResults...)

	return results
}

// multiRepoConfig describes a repo in the multi-repo setup
type multiRepoConfig struct {
	path   string // relative path from root (empty string for root)
	prefix string // issue prefix
}

// runMultiRepoPhases sets up a multi-repo environment and benchmarks
// creating and showing issues across repos via routing.
// Tests both BEADS_DIR (fast path) and cwd-based discovery variants.
func runMultiRepoPhases(t *testing.T, r *Runner, label string) []phaseResult {
	t.Helper()

	// Create a temp directory for the multi-repo setup
	rootDir, err := os.MkdirTemp("", "bd-multirepo-*")
	if err != nil {
		t.Fatalf("[%s] create temp dir: %v", label, err)
	}
	t.Cleanup(func() { os.RemoveAll(rootDir) })

	// Initialize git repo (needed for beads-lite cwd discovery)
	gitInit := exec.Command("git", "init")
	gitInit.Dir = rootDir
	if err := gitInit.Run(); err != nil {
		t.Fatalf("[%s] git init: %v", label, err)
	}

	// Define the repos
	repos := []multiRepoConfig{
		{path: "", prefix: "root"},
		{path: "repo1", prefix: "r1"},
		{path: "repo2", prefix: "r2"},
		{path: "repo2/repo3", prefix: "r3"},
	}

	// Create directories and init each repo (not timed - setup only)
	for _, repo := range repos {
		repoPath := rootDir
		if repo.path != "" {
			repoPath = filepath.Join(rootDir, repo.path)
			if err := os.MkdirAll(repoPath, 0755); err != nil {
				t.Fatalf("[%s] create dir %s: %v", label, repo.path, err)
			}
		}
		beadsDir := filepath.Join(repoPath, ".beads")
		result := r.RunWithBeadsDir(beadsDir, "init", "--prefix", repo.prefix)
		if result.ExitCode != 0 {
			t.Fatalf("[%s] init %s: exit %d, stderr: %s", label, repo.path, result.ExitCode, result.Stderr)
		}
	}

	// Write routes.jsonl in root .beads directory
	routesContent := `{"prefix": "root-", "path": "."}
{"prefix": "r1-", "path": "repo1"}
{"prefix": "r2-", "path": "repo2"}
{"prefix": "r3-", "path": "repo2/repo3"}
`
	routesPath := filepath.Join(rootDir, ".beads", "routes.jsonl")
	if err := os.WriteFile(routesPath, []byte(routesContent), 0644); err != nil {
		t.Fatalf("[%s] write routes.jsonl: %v", label, err)
	}

	var results []phaseResult
	totalIssues := len(repos) * multiRepoIssuesPerRepo
	rootBeadsDir := filepath.Join(rootDir, ".beads")
	childDir := filepath.Join(rootDir, "repo2", "repo3")

	// === BEADS_DIR variants (fast path) ===

	var beadsDirIDs []string

	// Phase: Create issues using BEADS_DIR
	start := time.Now()
	for _, repo := range repos {
		repoPath := rootDir
		if repo.path != "" {
			repoPath = filepath.Join(rootDir, repo.path)
		}
		beadsDir := filepath.Join(repoPath, ".beads")
		for i := 1; i <= multiRepoIssuesPerRepo; i++ {
			result := r.RunWithBeadsDir(beadsDir, "create", fmt.Sprintf("%s task %d", repo.prefix, i), "--json")
			if result.ExitCode != 0 {
				t.Fatalf("[%s] create %s task %d: exit %d, stderr: %s", label, repo.prefix, i, result.ExitCode, result.Stderr)
			}
			id := ExtractID([]byte(result.Stdout))
			if id == "" {
				t.Fatalf("[%s] create %s task %d: could not extract ID from: %s", label, repo.prefix, i, result.Stdout)
			}
			beadsDirIDs = append(beadsDirIDs, id)
		}
	}
	results = append(results, phaseResult{
		name:     fmt.Sprintf("create (BEADS_DIR, %dx%d)", multiRepoCount, multiRepoIssuesPerRepo),
		duration: time.Since(start),
	})

	// Phase: Show all issues using BEADS_DIR
	start = time.Now()
	for _, id := range beadsDirIDs {
		result := r.RunWithBeadsDir(rootBeadsDir, "show", id, "--json")
		if result.ExitCode != 0 {
			t.Fatalf("[%s] show %s: exit %d, stderr: %s", label, id, result.ExitCode, result.Stderr)
		}
	}
	results = append(results, phaseResult{
		name:     fmt.Sprintf("show (BEADS_DIR, %d)", totalIssues),
		duration: time.Since(start),
	})

	// === cwd variants (discovery path) ===

	var cwdIDs []string

	// Phase: Create issues using cwd (cd to each repo dir)
	start = time.Now()
	for _, repo := range repos {
		repoPath := rootDir
		if repo.path != "" {
			repoPath = filepath.Join(rootDir, repo.path)
		}
		for i := 1; i <= multiRepoIssuesPerRepo; i++ {
			result := r.RunInDir(repoPath, "create", fmt.Sprintf("%s task %d", repo.prefix, i), "--json")
			if result.ExitCode != 0 {
				t.Fatalf("[%s] create %s task %d: exit %d, stderr: %s", label, repo.prefix, i, result.ExitCode, result.Stderr)
			}
			id := ExtractID([]byte(result.Stdout))
			if id == "" {
				t.Fatalf("[%s] create %s task %d: could not extract ID from: %s", label, repo.prefix, i, result.Stdout)
			}
			cwdIDs = append(cwdIDs, id)
		}
	}
	results = append(results, phaseResult{
		name:     fmt.Sprintf("create (cwd, %dx%d)", multiRepoCount, multiRepoIssuesPerRepo),
		duration: time.Since(start),
	})

	// Phase: Show all issues from root directory (cwd)
	start = time.Now()
	for _, id := range cwdIDs {
		result := r.RunInDir(rootDir, "show", id, "--json")
		if result.ExitCode != 0 {
			t.Fatalf("[%s] show %s from root: exit %d, stderr: %s", label, id, result.ExitCode, result.Stderr)
		}
	}
	results = append(results, phaseResult{
		name:     fmt.Sprintf("show (cwd root, %d)", totalIssues),
		duration: time.Since(start),
	})

	// Phase: Show all issues from child directory (cwd)
	start = time.Now()
	for _, id := range cwdIDs {
		result := r.RunInDir(childDir, "show", id, "--json")
		if result.ExitCode != 0 {
			t.Fatalf("[%s] show %s from child: exit %d, stderr: %s", label, id, result.ExitCode, result.Stderr)
		}
	}
	results = append(results, phaseResult{
		name:     fmt.Sprintf("show (cwd child, %d)", totalIssues),
		duration: time.Since(start),
	})

	return results
}

// RunInDir executes a bd command in a specific directory without setting BEADS_DIR.
// This lets bd discover the .beads directory by walking up from cwd.
func (r *Runner) RunInDir(dir string, args ...string) RunResult {
	allArgs := append(r.ExtraArgs, args...)
	cmd := exec.Command(r.BdCmd, allArgs...)
	cmd.Dir = dir
	cmd.Env = os.Environ()
	for _, env := range r.ExtraEnv {
		cmd.Env = append(cmd.Env, env)
	}

	var stdout, stderr strings.Builder
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()
	exitCode := 0
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			exitCode = exitErr.ExitCode()
		} else {
			exitCode = -1
		}
	}

	return RunResult{
		Stdout:   stdout.String(),
		Stderr:   stderr.String(),
		ExitCode: exitCode,
	}
}

// RunWithBeadsDir executes a bd command with BEADS_DIR set to the specified path.
func (r *Runner) RunWithBeadsDir(beadsDir string, args ...string) RunResult {
	allArgs := append(r.ExtraArgs, args...)
	cmd := exec.Command(r.BdCmd, allArgs...)
	cmd.Env = append(os.Environ(), "BEADS_DIR="+beadsDir)
	for _, env := range r.ExtraEnv {
		cmd.Env = append(cmd.Env, env)
	}

	var stdout, stderr strings.Builder
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()
	exitCode := 0
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			exitCode = exitErr.ExitCode()
		} else {
			exitCode = -1
		}
	}

	return RunResult{
		Stdout:   stdout.String(),
		Stderr:   stderr.String(),
		ExitCode: exitCode,
	}
}

func printSingleResults(t *testing.T, results []phaseResult) {
	t.Helper()

	var total time.Duration
	t.Logf("%-25s %s", "Phase", "Time")
	t.Logf("%s", strings.Repeat("─", 40))
	for _, r := range results {
		t.Logf("%-25s %s", r.name, formatDuration(r.duration))
		total += r.duration
	}
	t.Logf("%s", strings.Repeat("─", 40))
	t.Logf("%-25s %s", "TOTAL", formatDuration(total))
}

func printComparisonTable(t *testing.T, lite, ref []phaseResult) {
	t.Helper()

	var liteTotal, refTotal time.Duration
	t.Logf("%-25s %14s %14s %10s", "Phase", "beads-lite", "bd (reference)", "diff")
	t.Logf("%s", strings.Repeat("─", 67))
	for i := range lite {
		diff := percentDiff(lite[i].duration, ref[i].duration)
		t.Logf("%-25s %14s %14s %10s",
			lite[i].name,
			formatDuration(lite[i].duration),
			formatDuration(ref[i].duration),
			diff,
		)
		liteTotal += lite[i].duration
		refTotal += ref[i].duration
	}
	t.Logf("%s", strings.Repeat("─", 67))
	t.Logf("%-25s %14s %14s %10s",
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
