package main

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// TestMain builds the binary once for all tests in this package.
var testBinary string

func TestMain(m *testing.M) {
	// Build the binary to a temp location
	tmpDir, err := os.MkdirTemp("", "bd-test-*")
	if err != nil {
		panic(err)
	}
	defer os.RemoveAll(tmpDir)

	testBinary = filepath.Join(tmpDir, "bd")
	cmd := exec.Command("go", "build", "-o", testBinary, ".")
	cmd.Dir = "."
	if out, err := cmd.CombinedOutput(); err != nil {
		panic("failed to build test binary: " + string(out))
	}

	os.Exit(m.Run())
}

func runBd(t *testing.T, dir string, args ...string) (stdout, stderr string, exitCode int) {
	t.Helper()
	cmd := exec.Command(testBinary, args...)
	cmd.Dir = dir

	var outBuf, errBuf bytes.Buffer
	cmd.Stdout = &outBuf
	cmd.Stderr = &errBuf

	err := cmd.Run()
	exitCode = 0
	if exitErr, ok := err.(*exec.ExitError); ok {
		exitCode = exitErr.ExitCode()
	} else if err != nil {
		t.Fatalf("failed to run bd: %v", err)
	}

	return outBuf.String(), errBuf.String(), exitCode
}

func TestMain_RunError(t *testing.T) {
	origRun := run
	origExit := osExit
	defer func() {
		run = origRun
		osExit = origExit
	}()

	var gotCode int
	osExit = func(code int) { gotCode = code }
	run = func() error { return fmt.Errorf("something went wrong") }

	// Capture stderr
	origStderr := os.Stderr
	r, w, _ := os.Pipe()
	os.Stderr = w

	main()

	w.Close()
	os.Stderr = origStderr

	var buf bytes.Buffer
	buf.ReadFrom(r)

	if gotCode != 1 {
		t.Errorf("expected exit code 1, got %d", gotCode)
	}
	if !strings.Contains(buf.String(), "something went wrong") {
		t.Errorf("expected error on stderr, got: %s", buf.String())
	}
}

func TestMain_RunSuccess(t *testing.T) {
	origRun := run
	origExit := osExit
	defer func() {
		run = origRun
		osExit = origExit
	}()

	var gotCode int = -1
	osExit = func(code int) { gotCode = code }
	run = func() error { return nil }

	main()

	if gotCode != -1 {
		t.Errorf("expected osExit not to be called, but got code %d", gotCode)
	}
}

func TestRun_HelpFlag(t *testing.T) {
	oldArgs := os.Args
	defer func() { os.Args = oldArgs }()
	os.Args = []string{"bd", "--help"}

	if err := run(); err != nil {
		t.Errorf("run(--help) returned error: %v", err)
	}
}

func TestRun_UnknownCommand(t *testing.T) {
	oldArgs := os.Args
	defer func() { os.Args = oldArgs }()
	os.Args = []string{"bd", "nonexistent-command-xyz"}

	if err := run(); err == nil {
		t.Error("run(unknown command) should return error")
	}
}

func TestHelp(t *testing.T) {
	stdout, _, exitCode := runBd(t, t.TempDir(), "--help")

	if exitCode != 0 {
		t.Errorf("expected exit code 0, got %d", exitCode)
	}
	if !strings.Contains(stdout, "Beads Lite") {
		t.Errorf("expected help to contain 'Beads Lite', got: %s", stdout)
	}
	if !strings.Contains(stdout, "Available Commands:") {
		t.Errorf("expected help to list available commands, got: %s", stdout)
	}
}

func TestVersion(t *testing.T) {
	// Version command may not exist yet, just test it doesn't crash
	_, _, exitCode := runBd(t, t.TempDir(), "--version")
	// Accept either 0 (version works) or 1 (not implemented) - just shouldn't panic
	if exitCode != 0 && exitCode != 1 {
		t.Errorf("unexpected exit code %d for --version", exitCode)
	}
}

func TestUnknownCommand(t *testing.T) {
	_, stderr, exitCode := runBd(t, t.TempDir(), "nonexistent-command")

	if exitCode == 0 {
		t.Error("expected non-zero exit code for unknown command")
	}
	if !strings.Contains(stderr, "unknown command") {
		t.Errorf("expected error about unknown command, got: %s", stderr)
	}
}

func TestSubcommandsRegistered(t *testing.T) {
	stdout, _, _ := runBd(t, t.TempDir(), "--help")

	// Verify all expected commands appear in help output
	expectedCommands := []string{
		"init",
		"create",
		"show",
		"update",
		"delete",
		"list",
		"close",
		"reopen",
		"search",
		"ready",
		"blocked",
		"dep",
		"comment",
		"children",
		"doctor",
		"stats",
		"compact",
	}

	for _, cmd := range expectedCommands {
		if !strings.Contains(stdout, cmd) {
			t.Errorf("expected command %q to be listed in help output", cmd)
		}
	}
}

func TestInitAndCreate(t *testing.T) {
	dir := t.TempDir()

	// Initialize beads
	stdout, stderr, exitCode := runBd(t, dir, "init", "--prefix", "bd")
	if exitCode != 0 {
		t.Fatalf("init failed (exit %d): stdout=%s stderr=%s", exitCode, stdout, stderr)
	}
	if !strings.Contains(stdout, "Initialized") {
		t.Errorf("expected init success message, got: %s", stdout)
	}

	// Verify .beads directory was created
	beadsDir := filepath.Join(dir, ".beads")
	if _, err := os.Stat(beadsDir); os.IsNotExist(err) {
		t.Error(".beads directory not created")
	}

	// Create an issue
	stdout, stderr, exitCode = runBd(t, dir, "create", "Test issue from CLI")
	if exitCode != 0 {
		t.Fatalf("create failed (exit %d): stdout=%s stderr=%s", exitCode, stdout, stderr)
	}
	if !strings.Contains(stdout, "Created issue:") {
		t.Errorf("expected create success message, got: %s", stdout)
	}
	if !strings.Contains(stdout, "bd-") {
		t.Errorf("expected issue ID in output, got: %s", stdout)
	}
}

func TestListWithoutInit(t *testing.T) {
	dir := t.TempDir()

	// Running list without init should fail
	_, stderr, exitCode := runBd(t, dir, "list")
	if exitCode == 0 {
		t.Error("expected non-zero exit code when running list without init")
	}
	if stderr == "" {
		t.Error("expected error message when running list without init")
	}
}

func TestJSONFlag(t *testing.T) {
	dir := t.TempDir()

	// Init first
	runBd(t, dir, "init")

	// Create with JSON output
	stdout, _, exitCode := runBd(t, dir, "--json", "create", "JSON test issue")
	if exitCode != 0 {
		t.Fatalf("create with --json failed (exit %d)", exitCode)
	}

	// Should be valid JSON with "id" field
	if !strings.Contains(stdout, `"id"`) {
		t.Errorf("expected JSON output with id field, got: %s", stdout)
	}
	if !strings.HasPrefix(strings.TrimSpace(stdout), "{") {
		t.Errorf("expected JSON object output, got: %s", stdout)
	}
}

func TestBD_JSONEnvVar(t *testing.T) {
	dir := t.TempDir()

	// Init first
	runBd(t, dir, "init")

	// Create with BD_JSON env var
	cmd := exec.Command(testBinary, "create", "Env var JSON test")
	cmd.Dir = dir
	cmd.Env = append(os.Environ(), "BD_JSON=1")

	var outBuf bytes.Buffer
	cmd.Stdout = &outBuf

	if err := cmd.Run(); err != nil {
		t.Fatalf("create with BD_JSON=1 failed: %v", err)
	}

	stdout := outBuf.String()
	if !strings.Contains(stdout, `"id"`) {
		t.Errorf("expected JSON output when BD_JSON=1, got: %s", stdout)
	}
}

func TestWorkflow(t *testing.T) {
	dir := t.TempDir()

	// Init
	runBd(t, dir, "init", "--prefix", "bd")

	// Create issue
	stdout, _, _ := runBd(t, dir, "--json", "create", "Workflow test issue")
	// Extract ID from JSON output
	// Output is like {"id":"bd-xxxx",...}
	idStart := strings.Index(stdout, `"id":"`) + 6
	idEnd := strings.Index(stdout[idStart:], `"`) + idStart
	issueID := stdout[idStart:idEnd]

	if !strings.HasPrefix(issueID, "bd-") {
		t.Fatalf("failed to extract issue ID from: %s", stdout)
	}

	// Show the issue
	stdout, _, exitCode := runBd(t, dir, "show", issueID)
	if exitCode != 0 {
		t.Errorf("show failed for %s", issueID)
	}
	if !strings.Contains(stdout, "Workflow test issue") {
		t.Errorf("show should display issue title, got: %s", stdout)
	}

	// List issues (should show newly created issue which has status=open)
	stdout, _, exitCode = runBd(t, dir, "list")
	if exitCode != 0 {
		t.Error("list failed")
	}
	if !strings.Contains(stdout, issueID) {
		t.Errorf("list should show the issue, got: %s", stdout)
	}

	// Update the issue status
	_, _, exitCode = runBd(t, dir, "update", issueID, "--status", "in_progress")
	if exitCode != 0 {
		t.Error("update status failed")
	}

	// List with --all should show issues regardless of status
	stdout, _, exitCode = runBd(t, dir, "list", "--all")
	if exitCode != 0 {
		t.Error("list --all failed")
	}
	if !strings.Contains(stdout, issueID) {
		t.Errorf("list --all should show the issue, got: %s", stdout)
	}

	// Close the issue
	_, _, exitCode = runBd(t, dir, "close", issueID)
	if exitCode != 0 {
		t.Error("close failed")
	}

	// List should not show closed issues by default
	stdout, _, _ = runBd(t, dir, "list")
	if strings.Contains(stdout, issueID) {
		t.Errorf("list should not show closed issues by default, got: %s", stdout)
	}

	// List with --closed should show it
	stdout, _, _ = runBd(t, dir, "list", "--closed")
	if !strings.Contains(stdout, issueID) {
		t.Errorf("list --closed should show closed issues, got: %s", stdout)
	}

	// Reopen the issue
	_, _, exitCode = runBd(t, dir, "reopen", issueID)
	if exitCode != 0 {
		t.Error("reopen failed")
	}

	// Delete the issue (hard delete for permanent removal)
	_, _, exitCode = runBd(t, dir, "delete", "--force", "--hard", issueID)
	if exitCode != 0 {
		t.Error("delete failed")
	}

	// Show should fail now
	_, _, exitCode = runBd(t, dir, "show", issueID)
	if exitCode == 0 {
		t.Error("show should fail for deleted issue")
	}
}
