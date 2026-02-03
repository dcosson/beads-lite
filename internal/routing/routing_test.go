package routing

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// jsonlRoutes builds routes.jsonl content from prefix→path pairs.
func jsonlRoutes(routes ...string) string {
	// routes are alternating prefix, path pairs
	var lines []string
	for i := 0; i < len(routes); i += 2 {
		lines = append(lines, `{"prefix": "`+routes[i]+`", "path": "`+routes[i+1]+`"}`)
	}
	return strings.Join(lines, "\n") + "\n"
}

// --- ExtractPrefix tests ---

func TestExtractPrefix_Standard(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"bl-1jzo", "bl-"},
		{"hq-abc", "hq-"},
		{"gt-xyz", "gt-"},
	}
	for _, tt := range tests {
		got := ExtractPrefix(tt.input)
		if got != tt.want {
			t.Errorf("ExtractPrefix(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

func TestExtractPrefix_NoHyphen(t *testing.T) {
	got := ExtractPrefix("nohyphen")
	if got != "" {
		t.Errorf("ExtractPrefix(%q) = %q, want empty", "nohyphen", got)
	}
}

func TestExtractPrefix_MultipleHyphens(t *testing.T) {
	got := ExtractPrefix("a-b-c")
	if got != "a-" {
		t.Errorf("ExtractPrefix(%q) = %q, want %q", "a-b-c", got, "a-")
	}
}

func TestExtractPrefix_HierarchicalID(t *testing.T) {
	got := ExtractPrefix("bl-abc.1")
	if got != "bl-" {
		t.Errorf("ExtractPrefix(%q) = %q, want %q", "bl-abc.1", got, "bl-")
	}
}

// --- LoadRoutes tests ---

func TestLoadRoutes_Valid(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "routes.jsonl")
	content := jsonlRoutes("hq-", ".", "bl-", "crew/misc")
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	routes, err := LoadRoutes(path)
	if err != nil {
		t.Fatalf("LoadRoutes error: %v", err)
	}
	if len(routes) != 2 {
		t.Fatalf("expected 2 routes, got %d", len(routes))
	}
	if routes["hq-"].Path != "." {
		t.Errorf("hq- path = %q, want %q", routes["hq-"].Path, ".")
	}
	if routes["bl-"].Path != "crew/misc" {
		t.Errorf("bl- path = %q, want %q", routes["bl-"].Path, "crew/misc")
	}
}

func TestLoadRoutes_MissingFile(t *testing.T) {
	routes, err := LoadRoutes("/nonexistent/routes.jsonl")
	if err != nil {
		t.Fatalf("LoadRoutes should not error for missing file: %v", err)
	}
	if len(routes) != 0 {
		t.Errorf("expected empty map, got %d routes", len(routes))
	}
}

func TestLoadRoutes_MalformedJSON(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "routes.jsonl")
	if err := os.WriteFile(path, []byte("{bad json\n"), 0644); err != nil {
		t.Fatal(err)
	}

	_, err := LoadRoutes(path)
	if err == nil {
		t.Fatal("LoadRoutes should error for malformed JSON")
	}
}

func TestLoadRoutes_EmptyFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "routes.jsonl")
	if err := os.WriteFile(path, []byte(""), 0644); err != nil {
		t.Fatal(err)
	}

	routes, err := LoadRoutes(path)
	if err != nil {
		t.Fatalf("LoadRoutes error: %v", err)
	}
	if len(routes) != 0 {
		t.Errorf("expected empty map for empty file, got %d routes", len(routes))
	}
}

func TestLoadRoutes_SkipsEmptyLines(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "routes.jsonl")
	content := `{"prefix": "hq-", "path": "."}

{"prefix": "bl-", "path": "rig"}
`
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	routes, err := LoadRoutes(path)
	if err != nil {
		t.Fatalf("LoadRoutes error: %v", err)
	}
	if len(routes) != 2 {
		t.Fatalf("expected 2 routes, got %d", len(routes))
	}
}

// --- Router tests ---

// setupBeadsDir creates a minimal .beads directory with config.yaml and data dirs.
func setupBeadsDir(t *testing.T, parentDir, prefix string) string {
	t.Helper()
	beadsDir := filepath.Join(parentDir, ".beads")
	if err := os.MkdirAll(filepath.Join(beadsDir, "issues", "open"), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(beadsDir, "issues", "closed"), 0755); err != nil {
		t.Fatal(err)
	}
	config := "actor: test\nproject.name: issues\nissue_prefix: " + prefix + "\n"
	if err := os.WriteFile(filepath.Join(beadsDir, "config.yaml"), []byte(config), 0644); err != nil {
		t.Fatal(err)
	}
	return beadsDir
}

func writeRoutes(t *testing.T, beadsDir string, routes ...string) {
	t.Helper()
	content := jsonlRoutes(routes...)
	if err := os.WriteFile(filepath.Join(beadsDir, routesFileName), []byte(content), 0644); err != nil {
		t.Fatal(err)
	}
}

func TestNew_WithRoutesJSONL(t *testing.T) {
	// Create town root with routes.jsonl
	townRoot := t.TempDir()
	townBeads := setupBeadsDir(t, townRoot, "hq-")
	writeRoutes(t, townBeads, "hq-", ".", "bl-", "rig")

	// Create rig .beads inside town
	rigDir := filepath.Join(townRoot, "rig")
	setupBeadsDir(t, rigDir, "bl-")

	router, err := New(filepath.Join(rigDir, ".beads"))
	if err != nil {
		t.Fatalf("New error: %v", err)
	}
	if router == nil {
		t.Fatal("expected non-nil Router")
	}
	if len(router.routes) != 2 {
		t.Errorf("expected 2 routes, got %d", len(router.routes))
	}
}

func TestNew_WithoutRoutesJSONL(t *testing.T) {
	// Create a standalone .beads dir with no routes.jsonl anywhere
	dir := t.TempDir()
	setupBeadsDir(t, dir, "bl-")

	router, err := New(filepath.Join(dir, ".beads"))
	if err != nil {
		t.Fatalf("New error: %v", err)
	}
	if router != nil {
		t.Fatal("expected nil Router when no routes.jsonl exists")
	}
}

func TestNew_WalksUpToFindRoutesJSONL(t *testing.T) {
	townRoot := t.TempDir()
	townBeads := setupBeadsDir(t, townRoot, "hq-")
	writeRoutes(t, townBeads, "hq-", ".", "bl-", "a/b/rig")

	// Create deeply nested rig
	rigDir := filepath.Join(townRoot, "a", "b", "rig")
	setupBeadsDir(t, rigDir, "bl-")

	router, err := New(filepath.Join(rigDir, ".beads"))
	if err != nil {
		t.Fatalf("New error: %v", err)
	}
	if router == nil {
		t.Fatal("expected non-nil Router (should walk up to find routes.jsonl)")
	}
}

func TestResolve_LocalPrefix(t *testing.T) {
	townRoot := t.TempDir()
	townBeads := setupBeadsDir(t, townRoot, "hq-")
	writeRoutes(t, townBeads, "hq-", ".", "bl-", "rig")

	rigDir := filepath.Join(townRoot, "rig")
	setupBeadsDir(t, rigDir, "bl-")

	// Router created from rig's .beads
	router, err := New(filepath.Join(rigDir, ".beads"))
	if err != nil {
		t.Fatalf("New error: %v", err)
	}

	// Resolve a bl- ID (local to rig)
	paths, prefix, isRemote, err := router.Resolve("bl-1234")
	if err != nil {
		t.Fatalf("Resolve error: %v", err)
	}
	if prefix != "bl-" {
		t.Errorf("prefix = %q, want %q", prefix, "bl-")
	}
	if isRemote {
		t.Error("expected isRemote=false for local prefix")
	}
	if paths.ConfigDir == "" {
		t.Error("expected non-empty ConfigDir")
	}
}

func TestResolve_RemotePrefix(t *testing.T) {
	townRoot := t.TempDir()
	townBeads := setupBeadsDir(t, townRoot, "hq-")
	writeRoutes(t, townBeads, "hq-", ".", "bl-", "rig")

	rigDir := filepath.Join(townRoot, "rig")
	setupBeadsDir(t, rigDir, "bl-")

	// Router created from rig's .beads
	router, err := New(filepath.Join(rigDir, ".beads"))
	if err != nil {
		t.Fatalf("New error: %v", err)
	}

	// Resolve an hq- ID (remote, lives at town root)
	paths, prefix, isRemote, err := router.Resolve("hq-abc")
	if err != nil {
		t.Fatalf("Resolve error: %v", err)
	}
	if prefix != "hq-" {
		t.Errorf("prefix = %q, want %q", prefix, "hq-")
	}
	if !isRemote {
		t.Error("expected isRemote=true for remote prefix")
	}
	if paths.ConfigDir == "" {
		t.Error("expected non-empty ConfigDir for remote resolution")
	}
}

func TestResolve_UnknownPrefix(t *testing.T) {
	townRoot := t.TempDir()
	townBeads := setupBeadsDir(t, townRoot, "hq-")
	writeRoutes(t, townBeads, "hq-", ".")

	router, err := New(townBeads)
	if err != nil {
		t.Fatalf("New error: %v", err)
	}

	paths, prefix, isRemote, err := router.Resolve("xx-unknown")
	if err != nil {
		t.Fatalf("Resolve error: %v", err)
	}
	if prefix != "" {
		t.Errorf("prefix = %q, want empty for unknown prefix", prefix)
	}
	if isRemote {
		t.Error("expected isRemote=false for unknown prefix")
	}
	if paths.ConfigDir != "" {
		t.Error("expected empty ConfigDir for unknown prefix")
	}
}

func TestResolve_NilRouter(t *testing.T) {
	var r *Router
	paths, prefix, isRemote, err := r.Resolve("bl-1234")
	if err != nil {
		t.Fatalf("Resolve on nil Router should not error: %v", err)
	}
	if prefix != "" {
		t.Errorf("prefix = %q, want empty for nil Router", prefix)
	}
	if isRemote {
		t.Error("expected isRemote=false for nil Router")
	}
	if paths.ConfigDir != "" {
		t.Error("expected zero Paths for nil Router")
	}
}

func TestResolve_FollowsRedirect(t *testing.T) {
	townRoot := t.TempDir()
	townBeads := setupBeadsDir(t, townRoot, "hq-")
	writeRoutes(t, townBeads, "hq-", ".", "bl-", "rig")

	// Create an actual .beads at a different location
	actualDir := t.TempDir()
	actualBeads := setupBeadsDir(t, actualDir, "bl-")

	// Create rig with redirect
	rigDir := filepath.Join(townRoot, "rig")
	rigBeads := filepath.Join(rigDir, ".beads")
	if err := os.MkdirAll(rigBeads, 0755); err != nil {
		t.Fatal(err)
	}
	// Write config so it's a valid beads dir for discovery
	if err := os.WriteFile(filepath.Join(rigBeads, "config.yaml"), []byte("project.name: issues\n"), 0644); err != nil {
		t.Fatal(err)
	}
	// Redirect to actual location
	if err := os.WriteFile(filepath.Join(rigBeads, "redirect"), []byte(actualBeads+"\n"), 0644); err != nil {
		t.Fatal(err)
	}

	router, err := New(townBeads)
	if err != nil {
		t.Fatalf("New error: %v", err)
	}

	paths, prefix, isRemote, err := router.Resolve("bl-1234")
	if err != nil {
		t.Fatalf("Resolve error: %v", err)
	}
	if prefix != "bl-" {
		t.Errorf("prefix = %q, want %q", prefix, "bl-")
	}
	if !isRemote {
		t.Error("expected isRemote=true for redirected remote prefix")
	}
	// The resolved ConfigDir should be the actual (redirected) location
	resolvedAbs, _ := filepath.Abs(paths.ConfigDir)
	actualAbs, _ := filepath.Abs(actualBeads)
	if resolvedAbs != actualAbs {
		t.Errorf("ConfigDir = %q, want %q (should follow redirect)", resolvedAbs, actualAbs)
	}
}

func TestResolve_SelfRouting(t *testing.T) {
	// When the route for a prefix points back to the local .beads, isRemote should be false
	townRoot := t.TempDir()
	townBeads := setupBeadsDir(t, townRoot, "hq-")
	writeRoutes(t, townBeads, "hq-", ".")

	// Create Router from the town root's own .beads
	router, err := New(townBeads)
	if err != nil {
		t.Fatalf("New error: %v", err)
	}

	paths, prefix, isRemote, err := router.Resolve("hq-abc")
	if err != nil {
		t.Fatalf("Resolve error: %v", err)
	}
	if prefix != "hq-" {
		t.Errorf("prefix = %q, want %q", prefix, "hq-")
	}
	if isRemote {
		t.Error("expected isRemote=false for self-routing (prefix maps to own .beads)")
	}
	if paths.ConfigDir == "" {
		t.Error("expected non-empty ConfigDir for self-routing")
	}
}
