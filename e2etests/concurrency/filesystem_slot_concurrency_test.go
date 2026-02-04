package concurrency

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"sync"
	"testing"

	"beads-lite/e2etests"
)

// TestConcurrentSlotSetRole verifies that 20 goroutines can set the role slot
// for the same agent concurrently. Since role has no cardinality constraint
// (last-writer-wins), all should exit 0 and the final role should be one of
// the 20 bead IDs.
func TestConcurrentSlotSetRole(t *testing.T) {
	bdCmd := os.Getenv("BD_CMD")
	if bdCmd == "" {
		t.Skip("BD_CMD environment variable not set")
	}

	r := &e2etests.Runner{BdCmd: bdCmd}
	sandbox, err := r.SetupSandbox()
	if err != nil {
		t.Fatal(err)
	}
	defer r.TeardownSandbox(sandbox)

	// Create 20 beads (issues) sequentially.
	const numBeads = 20
	beadIDs := make([]string, numBeads)
	for i := 0; i < numBeads; i++ {
		res := r.Run(sandbox, "create", fmt.Sprintf("bead-%d", i), "--json")
		if res.ExitCode != 0 {
			t.Fatalf("create bead %d: exit %d, stderr: %s", i, res.ExitCode, res.Stderr)
		}
		var issue struct {
			ID string `json:"id"`
		}
		if err := json.Unmarshal([]byte(strings.TrimSpace(res.Stdout)), &issue); err != nil {
			t.Fatalf("parsing create JSON for bead %d: %v", i, err)
		}
		beadIDs[i] = issue.ID
	}

	type result struct {
		idx      int
		exitCode int
		stderr   string
	}

	results := make([]result, numBeads)
	var wg sync.WaitGroup
	wg.Add(numBeads)

	for i := 0; i < numBeads; i++ {
		go func(idx int) {
			defer wg.Done()
			res := r.Run(sandbox, "slot", "set", "agent-1", "role", beadIDs[idx])
			results[idx] = result{idx: idx, exitCode: res.ExitCode, stderr: res.Stderr}
		}(i)
	}
	wg.Wait()

	for _, res := range results {
		if res.exitCode != 0 {
			t.Errorf("goroutine %d: exit %d, stderr: %s", res.idx, res.exitCode, res.stderr)
		}
	}

	// Verify the role is one of the 20 bead IDs.
	showRes := r.Run(sandbox, "slot", "show", "agent-1", "--json")
	if showRes.ExitCode != 0 {
		t.Fatalf("bd slot show --json: exit %d, stderr: %s", showRes.ExitCode, showRes.Stderr)
	}

	var slotInfo struct {
		Role string `json:"role"`
	}
	if err := json.Unmarshal([]byte(strings.TrimSpace(showRes.Stdout)), &slotInfo); err != nil {
		t.Fatalf("parsing slot show JSON: %v", err)
	}

	validRole := false
	for _, id := range beadIDs {
		if slotInfo.Role == id {
			validRole = true
			break
		}
	}
	if !validRole {
		t.Errorf("role %q is not one of the %d bead IDs", slotInfo.Role, numBeads)
	}
}

// TestConcurrentSlotSetDifferentAgents verifies that 20 goroutines can set
// role slots for different agents concurrently without interference.
func TestConcurrentSlotSetDifferentAgents(t *testing.T) {
	bdCmd := os.Getenv("BD_CMD")
	if bdCmd == "" {
		t.Skip("BD_CMD environment variable not set")
	}

	r := &e2etests.Runner{BdCmd: bdCmd}
	sandbox, err := r.SetupSandbox()
	if err != nil {
		t.Fatal(err)
	}
	defer r.TeardownSandbox(sandbox)

	// Create 20 beads sequentially.
	const numAgents = 20
	beadIDs := make([]string, numAgents)
	for i := 0; i < numAgents; i++ {
		res := r.Run(sandbox, "create", fmt.Sprintf("bead-%d", i), "--json")
		if res.ExitCode != 0 {
			t.Fatalf("create bead %d: exit %d, stderr: %s", i, res.ExitCode, res.Stderr)
		}
		var issue struct {
			ID string `json:"id"`
		}
		if err := json.Unmarshal([]byte(strings.TrimSpace(res.Stdout)), &issue); err != nil {
			t.Fatalf("parsing create JSON for bead %d: %v", i, err)
		}
		beadIDs[i] = issue.ID
	}

	type result struct {
		idx      int
		exitCode int
		stderr   string
	}

	results := make([]result, numAgents)
	var wg sync.WaitGroup
	wg.Add(numAgents)

	for i := 0; i < numAgents; i++ {
		go func(idx int) {
			defer wg.Done()
			agentID := fmt.Sprintf("agent-%d", idx)
			res := r.Run(sandbox, "slot", "set", agentID, "role", beadIDs[idx])
			results[idx] = result{idx: idx, exitCode: res.ExitCode, stderr: res.Stderr}
		}(i)
	}
	wg.Wait()

	for _, res := range results {
		if res.exitCode != 0 {
			t.Errorf("goroutine %d: exit %d, stderr: %s", res.idx, res.exitCode, res.stderr)
		}
	}

	// Verify each agent's role matches the expected bead.
	for i := 0; i < numAgents; i++ {
		agentID := fmt.Sprintf("agent-%d", i)
		showRes := r.Run(sandbox, "slot", "show", agentID, "--json")
		if showRes.ExitCode != 0 {
			t.Errorf("bd slot show %s --json: exit %d, stderr: %s", agentID, showRes.ExitCode, showRes.Stderr)
			continue
		}

		var slotInfo struct {
			Role string `json:"role"`
		}
		if err := json.Unmarshal([]byte(strings.TrimSpace(showRes.Stdout)), &slotInfo); err != nil {
			t.Errorf("parsing slot show JSON for %s: %v", agentID, err)
			continue
		}

		if slotInfo.Role != beadIDs[i] {
			t.Errorf("agent %s: expected role %s, got %s", agentID, beadIDs[i], slotInfo.Role)
		}
	}
}
