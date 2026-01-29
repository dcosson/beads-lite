package config

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestResolvePaths_BEADS_DIR_Explicit(t *testing.T) {
	tmpDir := t.TempDir()
	beadsDir := filepath.Join(tmpDir, ".beads")
	if err := os.MkdirAll(filepath.Join(beadsDir, "issues", "open"), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(beadsDir, "issues", "closed"), 0755); err != nil {
		t.Fatal(err)
	}
	writeDefaultConfig(t, filepath.Join(beadsDir, "config.yaml"))

	t.Setenv(EnvBeadsDir, beadsDir)

	paths, err := ResolvePaths()
	if err != nil {
		t.Fatalf("ResolvePaths error: %v", err)
	}
	gotConfig, _ := filepath.EvalSymlinks(paths.ConfigDir)
	wantConfig, _ := filepath.EvalSymlinks(beadsDir)
	if gotConfig != wantConfig {
		t.Errorf("ConfigDir = %q, want %q", paths.ConfigDir, beadsDir)
	}
	gotData, _ := filepath.EvalSymlinks(paths.DataDir)
	wantData, _ := filepath.EvalSymlinks(filepath.Join(beadsDir, "issues"))
	if gotData != wantData {
		t.Errorf("DataDir = %q, want %q", paths.DataDir, filepath.Join(beadsDir, "issues"))
	}
}

func TestResolvePaths_SearchUpward(t *testing.T) {
	tmpDir := t.TempDir()
	beadsDir := filepath.Join(tmpDir, ".beads")
	if err := os.MkdirAll(filepath.Join(beadsDir, "issues", "open"), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(beadsDir, "issues", "closed"), 0755); err != nil {
		t.Fatal(err)
	}
	writeDefaultConfig(t, filepath.Join(beadsDir, "config.yaml"))

	deepDir := filepath.Join(tmpDir, "a", "b", "c")
	if err := os.MkdirAll(deepDir, 0755); err != nil {
		t.Fatal(err)
	}

	origDir, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	defer os.Chdir(origDir)

	if err := os.Chdir(deepDir); err != nil {
		t.Fatal(err)
	}

	paths, err := ResolvePaths()
	if err != nil {
		t.Fatalf("ResolvePaths error: %v", err)
	}

	gotConfig := resolvePath(paths.ConfigDir)
	wantConfig := resolvePath(beadsDir)
	if gotConfig != wantConfig {
		t.Errorf("ConfigDir = %q, want %q", paths.ConfigDir, beadsDir)
	}
	gotData := resolvePath(paths.DataDir)
	wantData := resolvePath(filepath.Join(beadsDir, "issues"))
	if gotData != wantData {
		t.Errorf("DataDir = %q, want %q", paths.DataDir, filepath.Join(beadsDir, "issues"))
	}
}

func resolvePath(path string) string {
	resolved, err := filepath.EvalSymlinks(path)
	if err != nil {
		return filepath.Clean(path)
	}
	return filepath.Clean(resolved)
}

func TestResolvePaths_CustomProjectName(t *testing.T) {
	tmpDir := t.TempDir()
	beadsDir := filepath.Join(tmpDir, ".beads")
	if err := os.MkdirAll(filepath.Join(beadsDir, "work", "open"), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(beadsDir, "work", "closed"), 0755); err != nil {
		t.Fatal(err)
	}
	configPath := filepath.Join(beadsDir, "config.yaml")
	writeFlatConfig(t, configPath, map[string]string{
		"project.name": "work",
	})

	t.Setenv(EnvBeadsDir, beadsDir)

	paths, err := ResolvePaths()
	if err != nil {
		t.Fatalf("ResolvePaths error: %v", err)
	}
	wantData := filepath.Join(beadsDir, "work")
	if paths.DataDir != wantData {
		t.Errorf("DataDir = %q, want %q", paths.DataDir, wantData)
	}
}

func TestResolvePaths_MissingConfig(t *testing.T) {
	tmpDir := t.TempDir()
	deepDir := filepath.Join(tmpDir, "a", "b", "c")
	if err := os.MkdirAll(deepDir, 0755); err != nil {
		t.Fatal(err)
	}

	origDir, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	defer os.Chdir(origDir)

	if err := os.Chdir(deepDir); err != nil {
		t.Fatal(err)
	}

	_, err = ResolvePaths()
	if err == nil {
		t.Fatal("ResolvePaths should error when config is missing")
	}
	if !strings.Contains(err.Error(), "bd init") {
		t.Fatalf("expected error to mention bd init, got: %v", err)
	}
}

func TestResolvePaths_MissingDataDir(t *testing.T) {
	tmpDir := t.TempDir()
	beadsDir := filepath.Join(tmpDir, ".beads")
	if err := os.MkdirAll(beadsDir, 0755); err != nil {
		t.Fatal(err)
	}
	writeDefaultConfig(t, filepath.Join(beadsDir, "config.yaml"))

	t.Setenv(EnvBeadsDir, beadsDir)

	_, err := ResolvePaths()
	if err == nil {
		t.Fatal("ResolvePaths should error when data dir is missing")
	}

	expected := "beads data not found at " + filepath.Join(beadsDir, "issues") + " (run `bd init`)"
	if err.Error() != expected {
		t.Fatalf("expected error %q, got %q", expected, err.Error())
	}
}

// setupBeadsDir creates a .beads directory with config and data dirs.
func setupBeadsDir(t *testing.T, parentDir string) string {
	t.Helper()
	beadsDir := filepath.Join(parentDir, ".beads")
	if err := os.MkdirAll(filepath.Join(beadsDir, "issues", "open"), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(beadsDir, "issues", "closed"), 0755); err != nil {
		t.Fatal(err)
	}
	writeDefaultConfig(t, filepath.Join(beadsDir, "config.yaml"))
	return beadsDir
}

func TestResolvePaths_BEADS_DIR(t *testing.T) {
	tmpDir := t.TempDir()
	beadsDir := setupBeadsDir(t, tmpDir)

	// Set BEADS_DIR env var
	t.Setenv(EnvBeadsDir, beadsDir)

	// cd to a different temp dir with no .beads
	otherDir := t.TempDir()
	origDir, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	defer os.Chdir(origDir)
	if err := os.Chdir(otherDir); err != nil {
		t.Fatal(err)
	}

	paths, err := ResolvePaths()
	if err != nil {
		t.Fatalf("ResolvePaths error: %v", err)
	}

	gotConfig := resolvePath(paths.ConfigDir)
	wantConfig := resolvePath(beadsDir)
	if gotConfig != wantConfig {
		t.Errorf("ConfigDir = %q, want %q", gotConfig, wantConfig)
	}
}

func TestResolvePaths_StopsAtGitRoot(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not available")
	}

	// Create a temp dir as a git repo
	repoDir := t.TempDir()
	cmd := exec.Command("git", "init")
	cmd.Dir = repoDir
	if err := cmd.Run(); err != nil {
		t.Fatalf("git init: %v", err)
	}

	// Place .beads ABOVE the git repo root (should NOT be found)
	parentDir := filepath.Dir(repoDir)
	parentBeads := filepath.Join(parentDir, ".beads")
	if err := os.MkdirAll(filepath.Join(parentBeads, "issues", "open"), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(parentBeads, "issues", "closed"), 0755); err != nil {
		t.Fatal(err)
	}
	writeDefaultConfig(t, filepath.Join(parentBeads, "config.yaml"))

	// Create a subdirectory inside the repo
	subDir := filepath.Join(repoDir, "sub", "dir")
	if err := os.MkdirAll(subDir, 0755); err != nil {
		t.Fatal(err)
	}

	origDir, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	defer os.Chdir(origDir)
	if err := os.Chdir(subDir); err != nil {
		t.Fatal(err)
	}

	_, err = ResolvePaths()
	if err == nil {
		t.Fatal("ResolvePaths should not find .beads above git root")
	}
	if !strings.Contains(err.Error(), "bd init") {
		t.Fatalf("expected error to mention bd init, got: %v", err)
	}
}

func TestResolvePaths_RedirectFile(t *testing.T) {
	// Create the actual .beads dir at an external location
	externalDir := t.TempDir()
	externalBeads := setupBeadsDir(t, externalDir)

	// Create a .beads dir with a redirect file
	localDir := t.TempDir()
	localBeads := filepath.Join(localDir, ".beads")
	if err := os.MkdirAll(localBeads, 0755); err != nil {
		t.Fatal(err)
	}
	// Write config.yaml so findConfigUpward finds it
	writeDefaultConfig(t, filepath.Join(localBeads, "config.yaml"))
	// Write redirect file pointing to external location
	if err := os.WriteFile(filepath.Join(localBeads, "redirect"), []byte(externalBeads+"\n"), 0644); err != nil {
		t.Fatal(err)
	}

	origDir, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	defer os.Chdir(origDir)
	if err := os.Chdir(localDir); err != nil {
		t.Fatal(err)
	}

	paths, err := ResolvePaths()
	if err != nil {
		t.Fatalf("ResolvePaths error: %v", err)
	}

	gotConfig := resolvePath(paths.ConfigDir)
	wantConfig := resolvePath(externalBeads)
	if gotConfig != wantConfig {
		t.Errorf("ConfigDir = %q, want %q (should follow redirect)", gotConfig, wantConfig)
	}
}

func TestResolvePaths_RedirectRelativePath(t *testing.T) {
	// Create a parent dir structure
	parentDir := t.TempDir()

	// External .beads at parentDir/external/.beads
	externalBeads := setupBeadsDir(t, filepath.Join(parentDir, "external"))

	// Local .beads at parentDir/local/.beads with relative redirect
	localBeads := filepath.Join(parentDir, "local", ".beads")
	if err := os.MkdirAll(localBeads, 0755); err != nil {
		t.Fatal(err)
	}
	writeDefaultConfig(t, filepath.Join(localBeads, "config.yaml"))
	// Relative path from local/.beads to external/.beads
	if err := os.WriteFile(filepath.Join(localBeads, "redirect"), []byte("../../external/.beads\n"), 0644); err != nil {
		t.Fatal(err)
	}

	origDir, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	defer os.Chdir(origDir)
	if err := os.Chdir(filepath.Join(parentDir, "local")); err != nil {
		t.Fatal(err)
	}

	paths, err := ResolvePaths()
	if err != nil {
		t.Fatalf("ResolvePaths error: %v", err)
	}

	gotConfig := resolvePath(paths.ConfigDir)
	wantConfig := resolvePath(externalBeads)
	if gotConfig != wantConfig {
		t.Errorf("ConfigDir = %q, want %q (should follow relative redirect)", gotConfig, wantConfig)
	}
}

func TestResolvePaths_RedirectInvalid(t *testing.T) {
	tmpDir := t.TempDir()
	beadsDir := filepath.Join(tmpDir, ".beads")
	if err := os.MkdirAll(beadsDir, 0755); err != nil {
		t.Fatal(err)
	}
	writeDefaultConfig(t, filepath.Join(beadsDir, "config.yaml"))
	// Write redirect to a nonexistent directory
	if err := os.WriteFile(filepath.Join(beadsDir, "redirect"), []byte("/nonexistent/path\n"), 0644); err != nil {
		t.Fatal(err)
	}

	origDir, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	defer os.Chdir(origDir)
	if err := os.Chdir(tmpDir); err != nil {
		t.Fatal(err)
	}

	_, err = ResolvePaths()
	if err == nil {
		t.Fatal("ResolvePaths should error when redirect target doesn't exist")
	}
	if !strings.Contains(err.Error(), "redirect target") {
		t.Fatalf("expected error about redirect target, got: %v", err)
	}
}

func TestFindGitRoot(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not available")
	}

	repoDir := t.TempDir()
	cmd := exec.Command("git", "init")
	cmd.Dir = repoDir
	if err := cmd.Run(); err != nil {
		t.Fatalf("git init: %v", err)
	}

	subDir := filepath.Join(repoDir, "a", "b")
	if err := os.MkdirAll(subDir, 0755); err != nil {
		t.Fatal(err)
	}

	root, err := findGitRoot(subDir)
	if err != nil {
		t.Fatalf("findGitRoot error: %v", err)
	}

	gotRoot := resolvePath(root)
	wantRoot := resolvePath(repoDir)
	if gotRoot != wantRoot {
		t.Errorf("findGitRoot = %q, want %q", gotRoot, wantRoot)
	}
}

func TestFindGitRoot_NotGitRepo(t *testing.T) {
	tmpDir := t.TempDir()
	root, err := findGitRoot(tmpDir)
	if err == nil && root != "" {
		t.Errorf("findGitRoot in non-git dir should fail, got root=%q", root)
	}
}

func TestReadRedirect_NoFile(t *testing.T) {
	tmpDir := t.TempDir()
	result, err := readRedirect(tmpDir)
	if err != nil {
		t.Fatalf("readRedirect error: %v", err)
	}
	if result != "" {
		t.Errorf("readRedirect with no file = %q, want empty", result)
	}
}

func TestReadRedirect_EmptyFile(t *testing.T) {
	tmpDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(tmpDir, "redirect"), []byte(""), 0644); err != nil {
		t.Fatal(err)
	}
	result, err := readRedirect(tmpDir)
	if err != nil {
		t.Fatalf("readRedirect error: %v", err)
	}
	if result != "" {
		t.Errorf("readRedirect with empty file = %q, want empty", result)
	}
}

func TestReadRedirect_AbsolutePath(t *testing.T) {
	targetDir := t.TempDir()
	srcDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(srcDir, "redirect"), []byte(targetDir+"\n"), 0644); err != nil {
		t.Fatal(err)
	}
	result, err := readRedirect(srcDir)
	if err != nil {
		t.Fatalf("readRedirect error: %v", err)
	}
	gotResult := resolvePath(result)
	wantResult := resolvePath(targetDir)
	if gotResult != wantResult {
		t.Errorf("readRedirect = %q, want %q", gotResult, wantResult)
	}
}

func TestReadRedirect_RelativePath(t *testing.T) {
	parentDir := t.TempDir()
	targetDir := filepath.Join(parentDir, "target")
	srcDir := filepath.Join(parentDir, "source")
	if err := os.MkdirAll(targetDir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(srcDir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(srcDir, "redirect"), []byte("../target\n"), 0644); err != nil {
		t.Fatal(err)
	}
	result, err := readRedirect(srcDir)
	if err != nil {
		t.Fatalf("readRedirect error: %v", err)
	}
	gotResult := resolvePath(result)
	wantResult := resolvePath(targetDir)
	if gotResult != wantResult {
		t.Errorf("readRedirect = %q, want %q", gotResult, wantResult)
	}
}

func TestReadRedirect_NonexistentTarget(t *testing.T) {
	srcDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(srcDir, "redirect"), []byte("/nonexistent/path\n"), 0644); err != nil {
		t.Fatal(err)
	}
	_, err := readRedirect(srcDir)
	if err == nil {
		t.Fatal("readRedirect should error for nonexistent target")
	}
	if !strings.Contains(err.Error(), "redirect target") {
		t.Fatalf("expected error about redirect target, got: %v", err)
	}
}

func TestIsValidBeadsLiteDir_WithConfig(t *testing.T) {
	tmpDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(tmpDir, "config.yaml"), []byte("actor: test\n"), 0644); err != nil {
		t.Fatal(err)
	}
	if !isValidBeadsLiteDir(tmpDir) {
		t.Error("isValidBeadsLiteDir should return true when config.yaml exists")
	}
}

func TestIsValidBeadsLiteDir_WithProjectDirs(t *testing.T) {
	tmpDir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(tmpDir, "issues", "open"), 0755); err != nil {
		t.Fatal(err)
	}
	if !isValidBeadsLiteDir(tmpDir) {
		t.Error("isValidBeadsLiteDir should return true when project dirs with open/ exist")
	}
}

func TestIsValidBeadsLiteDir_Empty(t *testing.T) {
	tmpDir := t.TempDir()
	if isValidBeadsLiteDir(tmpDir) {
		t.Error("isValidBeadsLiteDir should return false for empty dir")
	}
}

func TestIsOriginalBeadsDir_MetadataJSON(t *testing.T) {
	tmpDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(tmpDir, "metadata.json"), []byte("{}"), 0644); err != nil {
		t.Fatal(err)
	}
	if !isOriginalBeadsDir(tmpDir) {
		t.Error("isOriginalBeadsDir should return true when metadata.json exists")
	}
}

func TestIsOriginalBeadsDir_IssuesJSONL(t *testing.T) {
	tmpDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(tmpDir, "issues.jsonl"), []byte(""), 0644); err != nil {
		t.Fatal(err)
	}
	if !isOriginalBeadsDir(tmpDir) {
		t.Error("isOriginalBeadsDir should return true when issues.jsonl exists")
	}
}

func TestIsOriginalBeadsDir_Neither(t *testing.T) {
	tmpDir := t.TempDir()
	if isOriginalBeadsDir(tmpDir) {
		t.Error("isOriginalBeadsDir should return false for empty dir")
	}
}

func TestIsOriginalBeadsDir_BeadsLiteDir(t *testing.T) {
	tmpDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(tmpDir, "config.yaml"), []byte("actor: test\n"), 0644); err != nil {
		t.Fatal(err)
	}
	if isOriginalBeadsDir(tmpDir) {
		t.Error("isOriginalBeadsDir should return false for beads-lite dir with only config.yaml")
	}
}

func TestValidateBeadsDir_OriginalBeads(t *testing.T) {
	tmpDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(tmpDir, "metadata.json"), []byte("{}"), 0644); err != nil {
		t.Fatal(err)
	}
	err := validateBeadsDir(tmpDir)
	if err == nil {
		t.Fatal("validateBeadsDir should error for original beads dir")
	}
	if !strings.Contains(err.Error(), "original beads") {
		t.Errorf("expected error mentioning 'original beads', got: %v", err)
	}
}

func TestValidateBeadsDir_InvalidDir(t *testing.T) {
	tmpDir := t.TempDir()
	err := validateBeadsDir(tmpDir)
	if err == nil {
		t.Fatal("validateBeadsDir should error for empty/invalid dir")
	}
	if !strings.Contains(err.Error(), "not a valid beads-lite") {
		t.Errorf("expected error mentioning 'not a valid beads-lite', got: %v", err)
	}
}

func TestValidateBeadsDir_ValidBeadsLite(t *testing.T) {
	tmpDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(tmpDir, "config.yaml"), []byte("actor: test\n"), 0644); err != nil {
		t.Fatal(err)
	}
	err := validateBeadsDir(tmpDir)
	if err != nil {
		t.Errorf("validateBeadsDir should return nil for valid beads-lite dir, got: %v", err)
	}
}

func TestResolvePaths_OriginalBeadsDir(t *testing.T) {
	// Create a .beads dir that looks like original beads (has metadata.json)
	tmpDir := t.TempDir()
	beadsDir := filepath.Join(tmpDir, ".beads")
	if err := os.MkdirAll(beadsDir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(beadsDir, "metadata.json"), []byte("{}"), 0644); err != nil {
		t.Fatal(err)
	}

	t.Setenv(EnvBeadsDir, beadsDir)

	_, err := ResolvePaths()
	if err == nil {
		t.Fatal("ResolvePaths should error for original beads dir")
	}
	if !strings.Contains(err.Error(), "original beads") {
		t.Errorf("expected error mentioning 'original beads', got: %v", err)
	}
}

func TestFindGitWorktreeRoot_NotWorktree(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not available")
	}

	repoDir := t.TempDir()
	cmd := exec.Command("git", "init")
	cmd.Dir = repoDir
	if err := cmd.Run(); err != nil {
		t.Fatalf("git init: %v", err)
	}

	root, err := findGitWorktreeRoot(repoDir)
	if err != nil {
		t.Fatalf("findGitWorktreeRoot error: %v", err)
	}
	if root != "" {
		t.Errorf("findGitWorktreeRoot in non-worktree should return empty, got %q", root)
	}
}

// writeDefaultConfig writes a flat key-value config file with default values.
func writeDefaultConfig(t *testing.T, path string) {
	t.Helper()
	content := "actor: ${USER}\ndefaults.priority: medium\ndefaults.type: task\nid.length: \"4\"\nid.prefix: bd-\nproject.name: issues\n"
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}
}

// writeFlatConfig writes a flat key-value YAML config file.
func writeFlatConfig(t *testing.T, path string, values map[string]string) {
	t.Helper()
	var lines []string
	for k, v := range values {
		lines = append(lines, k+": "+v)
	}
	content := strings.Join(lines, "\n") + "\n"
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}
}
