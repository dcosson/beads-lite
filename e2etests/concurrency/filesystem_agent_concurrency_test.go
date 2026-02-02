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

// TestConcurrentAgentStateSet verifies that 20 goroutines can set state for
// different agents concurrently without interference.
func TestConcurrentAgentStateSet(t *testing.T) {
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
		exitCode int
		stderr   string
	}

	results := make([]result, numGoroutines)
	var wg sync.WaitGroup
	wg.Add(numGoroutines)

	for i := 0; i < numGoroutines; i++ {
		go func(idx int) {
			defer wg.Done()
			agentID := fmt.Sprintf("agent-%d", idx)
			res := r.Run(sandbox, "agent", "state", agentID, "running")
			results[idx] = result{idx: idx, exitCode: res.ExitCode, stderr: res.Stderr}
		}(i)
	}
	wg.Wait()

	for _, res := range results {
		if res.exitCode != 0 {
			t.Errorf("goroutine %d: exit %d, stderr: %s", res.idx, res.exitCode, res.stderr)
		}
	}

	// Verify each agent's state is "running".
	for i := 0; i < numGoroutines; i++ {
		agentID := fmt.Sprintf("agent-%d", i)
		showRes := r.Run(sandbox, "agent", "show", agentID, "--json")
		if showRes.ExitCode != 0 {
			t.Errorf("bd agent show %s --json: exit %d, stderr: %s", agentID, showRes.ExitCode, showRes.Stderr)
			continue
		}

		var agentInfo struct {
			State string `json:"state"`
		}
		if err := json.Unmarshal([]byte(strings.TrimSpace(showRes.Stdout)), &agentInfo); err != nil {
			t.Errorf("parsing agent show JSON for %s: %v", agentID, err)
			continue
		}

		if agentInfo.State != "running" {
			t.Errorf("agent %s: expected state %q, got %q", agentID, "running", agentInfo.State)
		}
	}
}

// TestConcurrentAgentHeartbeat verifies that 20 goroutines can send heartbeats
// to the same agent concurrently without error.
func TestConcurrentAgentHeartbeat(t *testing.T) {
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

	// Create agent via state set.
	stateRes := r.Run(sandbox, "agent", "state", "agent-1", "running")
	if stateRes.ExitCode != 0 {
		t.Fatalf("bd agent state: exit %d, stderr: %s", stateRes.ExitCode, stateRes.Stderr)
	}

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
			res := r.Run(sandbox, "agent", "heartbeat", "agent-1")
			results[idx] = result{idx: idx, exitCode: res.ExitCode, stderr: res.Stderr}
		}(i)
	}
	wg.Wait()

	for _, res := range results {
		if res.exitCode != 0 {
			t.Errorf("goroutine %d: exit %d, stderr: %s", res.idx, res.exitCode, res.stderr)
		}
	}

	// Verify agent is still running and has a last_activity timestamp.
	showRes := r.Run(sandbox, "agent", "show", "agent-1", "--json")
	if showRes.ExitCode != 0 {
		t.Fatalf("bd agent show --json: exit %d, stderr: %s", showRes.ExitCode, showRes.Stderr)
	}

	var agentInfo struct {
		State        string `json:"state"`
		LastActivity string `json:"last_activity"`
	}
	if err := json.Unmarshal([]byte(strings.TrimSpace(showRes.Stdout)), &agentInfo); err != nil {
		t.Fatalf("parsing agent show JSON: %v", err)
	}

	if agentInfo.State != "running" {
		t.Errorf("expected state %q, got %q", "running", agentInfo.State)
	}
	if agentInfo.LastActivity == "" {
		t.Error("expected non-empty last_activity after heartbeats")
	}
}
