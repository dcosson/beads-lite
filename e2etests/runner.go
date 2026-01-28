package e2etests

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
	BdCmd string // path to bd binary
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

	return strings.TrimSpace(stdout.String()), nil
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
// If sandbox is non-empty, BEADS_DIR is set so the command finds the right .beads directory.
// Pass an empty sandbox for commands that don't need one (e.g., --help).
func (r *Runner) Run(sandbox string, args ...string) RunResult {
	cmd := exec.Command(r.BdCmd, args...)
	if sandbox != "" {
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

// projectRoot returns the root directory of the project (parent of e2etests/).
func projectRoot() string {
	// This file is in e2etests/, so project root is one level up
	dir, err := os.Getwd()
	if err != nil {
		panic(fmt.Sprintf("cannot get working directory: %v", err))
	}
	// If we're running from e2etests/, go up one level
	if filepath.Base(dir) == "e2etests" {
		return filepath.Dir(dir)
	}
	return dir
}
