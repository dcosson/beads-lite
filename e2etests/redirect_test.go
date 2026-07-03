package e2etests

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestRedirectOverlay is a standalone integration test for .beads/redirect
// handling with config overlays. A source repo's .beads dir redirects to a
// shared "holder" .beads dir; the source repo's own config.yaml (if present)
// overrides the holder's config in memory — most importantly issue_prefix, so
// multiple repos can share one holder dir with distinct prefixes.
//
// This is a beads-lite specific feature, so it runs directly against the
// beads-lite binary rather than as a reference golden-file case.
func TestRedirectOverlay(t *testing.T) {
	bdCmd := os.Getenv("BD_CMD")
	if bdCmd == "" {
		t.Skip("BD_CMD environment variable not set")
	}

	r := &Runner{BdCmd: bdCmd}

	// Layout:
	//   root/
	//   ├── holder/.beads/          (real storage, issue_prefix: zo)
	//   ├── repo-a/                 (fake git repo)
	//   │   └── .beads/
	//   │       ├── redirect        -> ../../holder/.beads (relative)
	//   │       └── config.yaml     (issue_prefix: zv — overlay)
	//   └── repo-b/                 (fake git repo)
	//       └── .beads/
	//           └── redirect        -> holder (absolute), NO config.yaml
	root := t.TempDir()

	holderBeads := filepath.Join(root, "holder", ".beads")
	mustMkdirAll(t, filepath.Join(holderBeads, "issues", "open"))
	mustMkdirAll(t, filepath.Join(holderBeads, "issues", "closed"))
	mustWriteFile(t, filepath.Join(holderBeads, "config.yaml"),
		"actor: test\nbeads_variant: beads-lite\nissue_prefix: zo\ndefaults.priority: \"2\"\ndefaults.type: task\n")

	seedIssue(t, holderBeads, "zo-0001", "Holder native issue")

	repoA := filepath.Join(root, "repo-a")
	repoABeads := filepath.Join(repoA, ".beads")
	mustMkdirAll(t, filepath.Join(repoA, ".git")) // bound the upward walk like a real repo
	mustMkdirAll(t, repoABeads)
	mustWriteFile(t, filepath.Join(repoABeads, "redirect"), "../../holder/.beads\n")
	mustWriteFile(t, filepath.Join(repoABeads, "config.yaml"), "issue_prefix: zv\n")

	repoB := filepath.Join(root, "repo-b")
	repoBBeads := filepath.Join(repoB, ".beads")
	mustMkdirAll(t, filepath.Join(repoB, ".git"))
	mustMkdirAll(t, repoBBeads)
	mustWriteFile(t, filepath.Join(repoBBeads, "redirect"), holderBeads+"\n")

	var repoAIssueID string

	t.Run("create_uses_overlay_prefix", func(t *testing.T) {
		// bd create from repo-a (cwd discovery) writes into the holder with
		// repo-a's overridden prefix.
		result := r.RunInDir(repoA, "create", "Repo A issue", "--json")
		if result.ExitCode != 0 {
			t.Fatalf("bd create from repo-a: exit %d, stderr: %s", result.ExitCode, result.Stderr)
		}
		repoAIssueID = extractIDFromJSON(t, result.Stdout)
		if !strings.HasPrefix(repoAIssueID, "zv-") {
			t.Errorf("created ID = %q, want zv- prefix from overlay config", repoAIssueID)
		}

		// The issue file must live in the holder, not in repo-a.
		if _, err := os.Stat(filepath.Join(holderBeads, "issues", "open", repoAIssueID+".json")); err != nil {
			t.Errorf("issue file not found in holder .beads: %v", err)
		}
		entries, err := os.ReadDir(repoABeads)
		if err != nil {
			t.Fatal(err)
		}
		for _, e := range entries {
			if e.Name() != "redirect" && e.Name() != "config.yaml" {
				t.Errorf("unexpected entry %q in redirecting .beads dir (data should go to holder)", e.Name())
			}
		}
	})

	t.Run("list_shows_mixed_prefixes", func(t *testing.T) {
		result := r.RunInDir(repoA, "list", "--json")
		if result.ExitCode != 0 {
			t.Fatalf("bd list from repo-a: exit %d, stderr: %s", result.ExitCode, result.Stderr)
		}
		ids := extractIDsFromJSON(t, result.Stdout)
		if !containsString(ids, "zo-0001") {
			t.Errorf("list from repo-a missing holder-native issue zo-0001, got %v", ids)
		}
		if !containsString(ids, repoAIssueID) {
			t.Errorf("list from repo-a missing repo-a issue %s, got %v", repoAIssueID, ids)
		}
	})

	t.Run("show_across_prefixes", func(t *testing.T) {
		result := r.RunInDir(repoA, "show", "zo-0001", "--json")
		if result.ExitCode != 0 {
			t.Fatalf("bd show zo-0001 from repo-a: exit %d, stderr: %s", result.ExitCode, result.Stderr)
		}
		if title := extractTitleFromJSON(t, result.Stdout); title != "Holder native issue" {
			t.Errorf("title = %q, want %q", title, "Holder native issue")
		}
	})

	t.Run("bare_redirect_no_config_yaml", func(t *testing.T) {
		// repo-b has ONLY a redirect file — discovery must still work, and
		// with no overlay the holder's own prefix applies.
		result := r.RunInDir(repoB, "create", "Repo B issue", "--json")
		if result.ExitCode != 0 {
			t.Fatalf("bd create from repo-b: exit %d, stderr: %s", result.ExitCode, result.Stderr)
		}
		id := extractIDFromJSON(t, result.Stdout)
		if !strings.HasPrefix(id, "zo-") {
			t.Errorf("created ID = %q, want zo- prefix from holder config (no overlay)", id)
		}
		if _, err := os.Stat(filepath.Join(holderBeads, "issues", "open", id+".json")); err != nil {
			t.Errorf("issue file not found in holder .beads: %v", err)
		}
	})

	t.Run("beads_dir_env_redirect_overlay", func(t *testing.T) {
		// The BEADS_DIR resolution path honors both the redirect and the overlay.
		result := r.RunWithBeadsDir(repoABeads, "create", "Repo A env issue", "--json")
		if result.ExitCode != 0 {
			t.Fatalf("bd create with BEADS_DIR=repo-a/.beads: exit %d, stderr: %s", result.ExitCode, result.Stderr)
		}
		id := extractIDFromJSON(t, result.Stdout)
		if !strings.HasPrefix(id, "zv-") {
			t.Errorf("created ID = %q, want zv- prefix from overlay config", id)
		}
	})

	t.Run("config_set_writes_to_holder", func(t *testing.T) {
		// bd config set from a redirecting repo persists to the holder's
		// config.yaml, never to the overlay file.
		result := r.RunInDir(repoA, "config", "set", "defaults.priority", "1")
		if result.ExitCode != 0 {
			t.Fatalf("bd config set from repo-a: exit %d, stderr: %s", result.ExitCode, result.Stderr)
		}
		holderConfig, err := os.ReadFile(filepath.Join(holderBeads, "config.yaml"))
		if err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(string(holderConfig), "defaults.priority: \"1\"") {
			t.Errorf("holder config.yaml missing persisted key, got:\n%s", holderConfig)
		}
		overlayConfig, err := os.ReadFile(filepath.Join(repoABeads, "config.yaml"))
		if err != nil {
			t.Fatal(err)
		}
		if strings.Contains(string(overlayConfig), "defaults.priority") {
			t.Errorf("overlay config.yaml should not be written by config set, got:\n%s", overlayConfig)
		}
	})
}

// extractIDFromJSON pulls the issue ID from bd --json output (object or array).
func extractIDFromJSON(t *testing.T, jsonOutput string) string {
	t.Helper()
	ids := extractIDsFromJSON(t, jsonOutput)
	if len(ids) == 0 {
		t.Fatal("no issues in JSON output")
	}
	return ids[0]
}

// extractIDsFromJSON pulls all issue IDs from bd --json output (object or array).
func extractIDsFromJSON(t *testing.T, jsonOutput string) []string {
	t.Helper()
	jsonOutput = strings.TrimSpace(jsonOutput)
	type idHolder struct {
		ID string `json:"id"`
	}
	var issues []idHolder
	if err := json.Unmarshal([]byte(jsonOutput), &issues); err != nil {
		var single idHolder
		if err := json.Unmarshal([]byte(jsonOutput), &single); err != nil || single.ID == "" {
			t.Fatalf("parsing JSON output: %v\nraw: %s", err, jsonOutput)
		}
		return []string{single.ID}
	}
	ids := make([]string, 0, len(issues))
	for _, iss := range issues {
		ids = append(ids, iss.ID)
	}
	return ids
}

func containsString(list []string, want string) bool {
	for _, s := range list {
		if s == want {
			return true
		}
	}
	return false
}
