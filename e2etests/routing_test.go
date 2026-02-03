package e2etests

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"beads-lite/e2etests/reference"
)

// seedIssue writes a minimal issue JSON file into the given .beads data directory.
// Returns the full issue ID.
func seedIssue(t *testing.T, beadsDir, id, title string) {
	t.Helper()
	now := time.Now().UTC().Format(time.RFC3339)
	issueJSON := map[string]interface{}{
		"id":         id,
		"title":      title,
		"status":     "open",
		"priority":   "medium",
		"issue_type": "task",
		"created_at": now,
		"updated_at": now,
	}
	data, err := json.MarshalIndent(issueJSON, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(beadsDir, "issues", "open", id+".json")
	if err := os.WriteFile(path, data, 0644); err != nil {
		t.Fatal(err)
	}
}

// TestRouting is a standalone integration test for cross-prefix issue routing.
// This runs against the beads-lite binary since routing is a beads-lite
// specific feature (not present in the reference beads implementation).
//
// Since generateID() always produces bd- prefix IDs, we seed issues by
// writing JSON files directly with the correct prefixes.
func TestRouting(t *testing.T) {
	bdCmd := os.Getenv("BD_CMD")
	if bdCmd == "" {
		t.Skip("BD_CMD environment variable not set")
	}

	r := &reference.Runner{BdCmd: bdCmd}

	// Build a multi-rig town layout:
	//   town_root/
	//   ├── .beads/
	//   │   ├── config.yaml   (id.prefix: hq-)
	//   │   ├── issues/open/  (hq- issues live here)
	//   │   └── routes.jsonl   (hq- -> ".", bl- -> "crew/misc")
	//   └── crew/
	//       └── misc/
	//           └── .beads/
	//               ├── config.yaml   (id.prefix: bl-)
	//               └── issues/open/  (bl- issues live here)

	townRoot := t.TempDir()

	hqBeads := filepath.Join(townRoot, ".beads")
	mustMkdirAll(t, filepath.Join(hqBeads, "issues", "open"))
	mustMkdirAll(t, filepath.Join(hqBeads, "issues", "closed"))
	mustWriteFile(t, filepath.Join(hqBeads, "config.yaml"),
		"actor: test\nproject.name: issues\nid.prefix: hq-\ndefaults.priority: medium\ndefaults.type: task\n")
	mustWriteFile(t, filepath.Join(hqBeads, "routes.jsonl"),
		"{\"prefix\": \"hq-\", \"path\": \".\"}\n{\"prefix\": \"bl-\", \"path\": \"crew/misc\"}\n")

	rigDir := filepath.Join(townRoot, "crew", "misc")
	rigBeads := filepath.Join(rigDir, ".beads")
	mustMkdirAll(t, filepath.Join(rigBeads, "issues", "open"))
	mustMkdirAll(t, filepath.Join(rigBeads, "issues", "closed"))
	mustWriteFile(t, filepath.Join(rigBeads, "config.yaml"),
		"actor: test\nproject.name: issues\nid.prefix: bl-\ndefaults.priority: medium\ndefaults.type: task\n")

	// Seed issues with correct prefixes
	hqID := "hq-0001"
	blID := "bl-0002"
	seedIssue(t, hqBeads, hqID, "HQ routing test issue")
	seedIssue(t, rigBeads, blID, "BL routing test issue")

	t.Run("rig_shows_remote_hq_issue", func(t *testing.T) {
		// From crew/misc rig, bd show <hq-id> resolves via routing
		result := r.Run(rigBeads, "show", hqID, "--json")
		if result.ExitCode != 0 {
			t.Fatalf("bd show %s from rig: exit %d, stderr: %s", hqID, result.ExitCode, result.Stderr)
		}
		title := extractTitleFromJSON(t, result.Stdout)
		if title != "HQ routing test issue" {
			t.Errorf("expected title %q, got %q", "HQ routing test issue", title)
		}
	})

	t.Run("rig_shows_local_bl_issue", func(t *testing.T) {
		// From crew/misc rig, bd show <bl-id> resolves locally
		result := r.Run(rigBeads, "show", blID, "--json")
		if result.ExitCode != 0 {
			t.Fatalf("bd show %s from rig: exit %d, stderr: %s", blID, result.ExitCode, result.Stderr)
		}
		title := extractTitleFromJSON(t, result.Stdout)
		if title != "BL routing test issue" {
			t.Errorf("expected title %q, got %q", "BL routing test issue", title)
		}
	})

	t.Run("hq_shows_remote_bl_issue", func(t *testing.T) {
		// From town root rig, bd show <bl-id> resolves via routing
		result := r.Run(hqBeads, "show", blID, "--json")
		if result.ExitCode != 0 {
			t.Fatalf("bd show %s from hq: exit %d, stderr: %s", blID, result.ExitCode, result.Stderr)
		}
		title := extractTitleFromJSON(t, result.Stdout)
		if title != "BL routing test issue" {
			t.Errorf("expected title %q, got %q", "BL routing test issue", title)
		}
	})

	t.Run("hq_shows_local_hq_issue", func(t *testing.T) {
		// From town root rig, bd show <hq-id> resolves locally (self-routing)
		result := r.Run(hqBeads, "show", hqID, "--json")
		if result.ExitCode != 0 {
			t.Fatalf("bd show %s from hq: exit %d, stderr: %s", hqID, result.ExitCode, result.Stderr)
		}
		title := extractTitleFromJSON(t, result.Stdout)
		if title != "HQ routing test issue" {
			t.Errorf("expected title %q, got %q", "HQ routing test issue", title)
		}
	})

	t.Run("no_routes_file_not_found", func(t *testing.T) {
		// Without routes.jsonl, a remote-prefix ID is not found locally.
		// In JSON mode, show returns exit 0 with empty output for not-found.
		noRoutesDir := t.TempDir()
		noRoutesBeads := filepath.Join(noRoutesDir, ".beads")
		mustMkdirAll(t, filepath.Join(noRoutesBeads, "issues", "open"))
		mustMkdirAll(t, filepath.Join(noRoutesBeads, "issues", "closed"))
		mustWriteFile(t, filepath.Join(noRoutesBeads, "config.yaml"),
			"actor: test\nproject.name: issues\nid.prefix: xx-\ndefaults.priority: medium\ndefaults.type: task\n")

		result := r.Run(noRoutesBeads, "show", hqID)
		// Non-JSON show returns non-zero exit for not-found
		if result.ExitCode == 0 {
			t.Fatal("expected non-zero exit when showing remote ID without routes.jsonl")
		}
		if !strings.Contains(result.Stderr, "no issue found") {
			t.Errorf("expected not-found error, got stderr: %s", result.Stderr)
		}
	})

	t.Run("redirect_followed", func(t *testing.T) {
		redirectTown := t.TempDir()

		rdHQBeads := filepath.Join(redirectTown, ".beads")
		mustMkdirAll(t, filepath.Join(rdHQBeads, "issues", "open"))
		mustMkdirAll(t, filepath.Join(rdHQBeads, "issues", "closed"))
		mustWriteFile(t, filepath.Join(rdHQBeads, "config.yaml"),
			"actor: test\nproject.name: issues\nid.prefix: hq-\ndefaults.priority: medium\ndefaults.type: task\n")
		mustWriteFile(t, filepath.Join(rdHQBeads, "routes.jsonl"),
			"{\"prefix\": \"hq-\", \"path\": \".\"}\n{\"prefix\": \"rd-\", \"path\": \"rig\"}\n")

		// Actual .beads at a separate location
		actualDir := t.TempDir()
		actualBeads := filepath.Join(actualDir, ".beads")
		mustMkdirAll(t, filepath.Join(actualBeads, "issues", "open"))
		mustMkdirAll(t, filepath.Join(actualBeads, "issues", "closed"))
		mustWriteFile(t, filepath.Join(actualBeads, "config.yaml"),
			"actor: test\nproject.name: issues\nid.prefix: rd-\ndefaults.priority: medium\ndefaults.type: task\n")

		// Seed an issue in the actual location
		rdID := "rd-0003"
		seedIssue(t, actualBeads, rdID, "Redirected issue")

		// Rig dir with redirect pointing to actual location
		rdRigBeads := filepath.Join(redirectTown, "rig", ".beads")
		mustMkdirAll(t, rdRigBeads)
		mustWriteFile(t, filepath.Join(rdRigBeads, "config.yaml"),
			"actor: test\nproject.name: issues\nid.prefix: rd-\n")
		mustWriteFile(t, filepath.Join(rdRigBeads, "redirect"), actualBeads+"\n")

		// From town root, show rd- issue should follow redirect
		result := r.Run(rdHQBeads, "show", rdID, "--json")
		if result.ExitCode != 0 {
			t.Fatalf("bd show %s with redirect: exit %d, stderr: %s", rdID, result.ExitCode, result.Stderr)
		}
		title := extractTitleFromJSON(t, result.Stdout)
		if title != "Redirected issue" {
			t.Errorf("expected title %q, got %q", "Redirected issue", title)
		}
	})
}

func mustMkdirAll(t *testing.T, path string) {
	t.Helper()
	if err := os.MkdirAll(path, 0755); err != nil {
		t.Fatal(err)
	}
}

func mustWriteFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}
}

func extractTitleFromJSON(t *testing.T, jsonOutput string) string {
	t.Helper()
	jsonOutput = strings.TrimSpace(jsonOutput)
	// bd show --json returns an array
	var issues []struct {
		Title string `json:"title"`
	}
	if err := json.Unmarshal([]byte(jsonOutput), &issues); err != nil {
		t.Fatalf("parsing JSON output: %v\nraw: %s", err, jsonOutput)
	}
	if len(issues) == 0 {
		t.Fatal("empty issue array in JSON output")
	}
	return issues[0].Title
}
