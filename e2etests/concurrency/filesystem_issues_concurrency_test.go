package concurrency

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"sync"
	"testing"

	"beads-lite/e2etests/reference"
)

// TestConcurrentIssueCreates verifies that 20 goroutines can create issues
// concurrently without generating duplicate IDs.
func TestConcurrentIssueCreates(t *testing.T) {
	bdCmd := os.Getenv("BD_CMD")
	if bdCmd == "" {
		t.Skip("BD_CMD environment variable not set")
	}

	r := &reference.Runner{BdCmd: bdCmd}
	sandbox, err := r.SetupSandbox()
	if err != nil {
		t.Fatal(err)
	}
	defer r.TeardownSandbox(sandbox)

	const numGoroutines = 20
	type result struct {
		idx      int
		id       string
		exitCode int
		stderr   string
	}

	results := make([]result, numGoroutines)
	var wg sync.WaitGroup
	wg.Add(numGoroutines)

	for i := 0; i < numGoroutines; i++ {
		go func(idx int) {
			defer wg.Done()
			title := fmt.Sprintf("issue %d", idx)
			res := r.Run(sandbox, "create", title, "--json")
			var id string
			if res.ExitCode == 0 {
				var issue struct {
					ID string `json:"id"`
				}
				if err := json.Unmarshal([]byte(strings.TrimSpace(res.Stdout)), &issue); err == nil {
					id = issue.ID
				}
			}
			results[idx] = result{idx: idx, id: id, exitCode: res.ExitCode, stderr: res.Stderr}
		}(i)
	}
	wg.Wait()

	// Assert all exit 0 and extract unique IDs.
	ids := make(map[string]bool)
	for _, res := range results {
		if res.exitCode != 0 {
			t.Errorf("goroutine %d: exit %d, stderr: %s", res.idx, res.exitCode, res.stderr)
			continue
		}
		if res.id == "" {
			t.Errorf("goroutine %d: empty id", res.idx)
			continue
		}
		if ids[res.id] {
			t.Errorf("duplicate id: %s", res.id)
		}
		ids[res.id] = true
	}

	if len(ids) != numGoroutines {
		t.Errorf("expected %d unique IDs, got %d", numGoroutines, len(ids))
	}

	// Verify via bd list --json.
	listRes := r.Run(sandbox, "list", "--json")
	if listRes.ExitCode != 0 {
		t.Fatalf("bd list --json: exit %d, stderr: %s", listRes.ExitCode, listRes.Stderr)
	}

	var listed []struct {
		ID string `json:"id"`
	}
	if err := json.Unmarshal([]byte(strings.TrimSpace(listRes.Stdout)), &listed); err != nil {
		t.Fatalf("parsing list JSON: %v", err)
	}
	if len(listed) != numGoroutines {
		t.Errorf("expected %d issues in list, got %d", numGoroutines, len(listed))
	}
}

// TestConcurrentIssueUpdates verifies that 20 goroutines can add labels to the
// same issue concurrently and all labels are preserved.
func TestConcurrentIssueUpdates(t *testing.T) {
	bdCmd := os.Getenv("BD_CMD")
	if bdCmd == "" {
		t.Skip("BD_CMD environment variable not set")
	}

	r := &reference.Runner{BdCmd: bdCmd}
	sandbox, err := r.SetupSandbox()
	if err != nil {
		t.Fatal(err)
	}
	defer r.TeardownSandbox(sandbox)

	// Create a single issue.
	createRes := r.Run(sandbox, "create", "test", "--json")
	if createRes.ExitCode != 0 {
		t.Fatalf("bd create: exit %d, stderr: %s", createRes.ExitCode, createRes.Stderr)
	}
	var created struct {
		ID string `json:"id"`
	}
	if err := json.Unmarshal([]byte(strings.TrimSpace(createRes.Stdout)), &created); err != nil {
		t.Fatalf("parsing create JSON: %v", err)
	}
	id := created.ID

	const numGoroutines = 20
	type result struct {
		idx      int
		exitCode int
		stderr   string
	}
	results := make([]result, numGoroutines)
	var wg sync.WaitGroup
	wg.Add(numGoroutines)

	for i := 0; i < numGoroutines; i++ {
		go func(idx int) {
			defer wg.Done()
			label := fmt.Sprintf("label-%d", idx)
			res := r.Run(sandbox, "update", id, "--add-label", label)
			results[idx] = result{idx: idx, exitCode: res.ExitCode, stderr: res.Stderr}
		}(i)
	}
	wg.Wait()

	for _, res := range results {
		if res.exitCode != 0 {
			t.Errorf("goroutine %d: exit %d, stderr: %s", res.idx, res.exitCode, res.stderr)
		}
	}

	// Verify all 20 unique labels are present.
	showRes := r.Run(sandbox, "show", id, "--json")
	if showRes.ExitCode != 0 {
		t.Fatalf("bd show --json: exit %d, stderr: %s", showRes.ExitCode, showRes.Stderr)
	}

	var shown []struct {
		Labels []string `json:"labels"`
	}
	if err := json.Unmarshal([]byte(strings.TrimSpace(showRes.Stdout)), &shown); err != nil {
		t.Fatalf("parsing show JSON: %v", err)
	}
	if len(shown) == 0 {
		t.Fatal("empty show result")
	}

	labels := make(map[string]bool)
	for _, l := range shown[0].Labels {
		labels[l] = true
	}
	if len(labels) != numGoroutines {
		t.Errorf("expected %d unique labels, got %d: %v", numGoroutines, len(labels), shown[0].Labels)
	}
}

// TestConcurrentIssueDependencies verifies that 20 goroutines can add
// dependencies concurrently without deadlock, and that bidirectional
// consistency is maintained.
func TestConcurrentIssueDependencies(t *testing.T) {
	bdCmd := os.Getenv("BD_CMD")
	if bdCmd == "" {
		t.Skip("BD_CMD environment variable not set")
	}

	r := &reference.Runner{BdCmd: bdCmd}
	sandbox, err := r.SetupSandbox()
	if err != nil {
		t.Fatal(err)
	}
	defer r.TeardownSandbox(sandbox)

	// Create 10 issues sequentially.
	const numIssues = 10
	issueIDs := make([]string, numIssues)
	for i := 0; i < numIssues; i++ {
		res := r.Run(sandbox, "create", fmt.Sprintf("dep-issue-%d", i), "--json")
		if res.ExitCode != 0 {
			t.Fatalf("create issue %d: exit %d, stderr: %s", i, res.ExitCode, res.Stderr)
		}
		var issue struct {
			ID string `json:"id"`
		}
		if err := json.Unmarshal([]byte(strings.TrimSpace(res.Stdout)), &issue); err != nil {
			t.Fatalf("parsing create JSON for issue %d: %v", i, err)
		}
		issueIDs[i] = issue.ID
	}

	// 20 goroutines each add a dependency between adjacent issues (mod 10).
	const numGoroutines = 20
	type result struct {
		idx      int
		exitCode int
		stderr   string
	}
	results := make([]result, numGoroutines)
	var wg sync.WaitGroup
	wg.Add(numGoroutines)

	for i := 0; i < numGoroutines; i++ {
		go func(idx int) {
			defer wg.Done()
			from := issueIDs[idx%numIssues]
			to := issueIDs[(idx+1)%numIssues]
			res := r.Run(sandbox, "dep", "add", from, to)
			results[idx] = result{idx: idx, exitCode: res.ExitCode, stderr: res.Stderr}
		}(i)
	}
	wg.Wait()

	// All should complete (no deadlock). Non-zero exit from cycle detection is OK.
	for _, res := range results {
		if res.exitCode != 0 && !strings.Contains(res.stderr, "cycle") {
			t.Errorf("goroutine %d: unexpected exit %d, stderr: %s", res.idx, res.exitCode, res.stderr)
		}
	}

	// Verify bidirectional consistency: if A's dependencies contain B,
	// then B's dependents must contain A.
	for _, id := range issueIDs {
		showRes := r.Run(sandbox, "show", id, "--json")
		if showRes.ExitCode != 0 {
			t.Errorf("bd show %s: exit %d, stderr: %s", id, showRes.ExitCode, showRes.Stderr)
			continue
		}

		var issues []struct {
			ID           string `json:"id"`
			Dependencies []struct {
				ID string `json:"id"`
			} `json:"dependencies"`
			Dependents []struct {
				ID string `json:"id"`
			} `json:"dependents"`
		}
		if err := json.Unmarshal([]byte(strings.TrimSpace(showRes.Stdout)), &issues); err != nil {
			t.Errorf("parsing show JSON for %s: %v", id, err)
			continue
		}
		if len(issues) == 0 {
			continue
		}

		issue := issues[0]
		for _, dep := range issue.Dependencies {
			// Verify B's dependents contain A.
			depShowRes := r.Run(sandbox, "show", dep.ID, "--json")
			if depShowRes.ExitCode != 0 {
				t.Errorf("bd show %s (dep of %s): exit %d", dep.ID, id, depShowRes.ExitCode)
				continue
			}
			var depIssues []struct {
				Dependents []struct {
					ID string `json:"id"`
				} `json:"dependents"`
			}
			if err := json.Unmarshal([]byte(strings.TrimSpace(depShowRes.Stdout)), &depIssues); err != nil {
				t.Errorf("parsing show JSON for dep %s: %v", dep.ID, err)
				continue
			}
			if len(depIssues) == 0 {
				t.Errorf("empty show result for dep %s", dep.ID)
				continue
			}
			found := false
			for _, d := range depIssues[0].Dependents {
				if d.ID == id {
					found = true
					break
				}
			}
			if !found {
				t.Errorf("asymmetric dependency: %s depends on %s, but %s's dependents do not contain %s",
					id, dep.ID, dep.ID, id)
			}
		}
	}
}
