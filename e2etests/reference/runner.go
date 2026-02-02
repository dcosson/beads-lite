package reference

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// Runner executes bd commands against a sandbox directory.
type Runner struct {
	BdCmd       string   // path to bd binary
	KillDaemons bool     // kill reference binary daemons between sandboxes
	ExtraArgs   []string // extra args prepended to every command (e.g. --no-daemon)
}

// SetupSandbox creates a fresh beads sandbox directory by running the setup script.
// Returns the sandbox path.
func (r *Runner) SetupSandbox() (string, error) {
	scriptDir := filepath.Join(projectRoot(), "scripts")
	cmd := exec.Command(filepath.Join(scriptDir, "setup_bd_sandbox.sh"))
	cmd.Env = append(os.Environ(), "BD_CMD="+r.BdCmd)

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("setup sandbox failed: %v\nstderr: %s", err, stderr.String())
	}

	sandbox := strings.TrimSpace(stdout.String())

	if r.KillDaemons {
		r.KillAllDaemons(sandbox)
	}

	return sandbox, nil
}

// TeardownSandbox removes a sandbox directory.
func (r *Runner) TeardownSandbox(path string) error {
	scriptDir := filepath.Join(projectRoot(), "scripts")
	cmd := exec.Command(filepath.Join(scriptDir, "teardown_bd_sandbox.sh"), path)
	cmd.Env = append(os.Environ(), "BD_CMD="+r.BdCmd)

	var stderr bytes.Buffer
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("teardown sandbox failed: %v\nstderr: %s", err, stderr.String())
	}
	return nil
}

// RunResult holds the output of a command execution.
type RunResult struct {
	Stdout   string
	Stderr   string
	ExitCode int
}

// Run executes a bd command with the given arguments.
// If sandbox is non-empty, BEADS_DIR is set and the working directory is changed
// to the sandbox so the command finds the right data directory.
// Pass an empty sandbox for commands that don't need one (e.g., --help).
func (r *Runner) Run(sandbox string, args ...string) RunResult {
	allArgs := append(r.ExtraArgs, args...)
	cmd := exec.Command(r.BdCmd, allArgs...)
	if sandbox != "" {
		cmd.Dir = sandbox
		cmd.Env = append(os.Environ(), "BEADS_DIR="+sandbox)
	}

	var stdout, stderr bytes.Buffer
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

// KillAllDaemons kills any running reference binary daemons.
// Requires a valid sandbox path to provide a .beads/ workspace context.
func (r *Runner) KillAllDaemons(sandbox string) {
	cmd := exec.Command(r.BdCmd, "daemon", "killall")
	cmd.Dir = sandbox
	cmd.Env = append(os.Environ(), "BEADS_DIR="+sandbox)
	cmd.Run() // best-effort
}

// projectRoot returns the root directory of the project (parent of e2etests/).
func projectRoot() string {
	dir, err := os.Getwd()
	if err != nil {
		panic(fmt.Sprintf("cannot get working directory: %v", err))
	}
	// This file is in e2etests/reference/, so project root is two levels up
	if filepath.Base(dir) == "reference" && filepath.Base(filepath.Dir(dir)) == "e2etests" {
		return filepath.Dir(filepath.Dir(dir))
	}
	// Fallback: if running from e2etests/, go up one level
	if filepath.Base(dir) == "e2etests" {
		return filepath.Dir(dir)
	}
	return dir
}
