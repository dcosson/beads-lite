package config

import (
	"bufio"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// Paths captures resolved locations for config.
type Paths struct {
	ConfigDir  string // path to .beads directory
	ConfigFile string // path to .beads/config.yaml
}

// ResolvePaths resolves config and data paths.
// Discovery order: BEADS_DIR env var > walk up from CWD (stopping at git root, with worktree fallback).
func ResolvePaths() (Paths, error) {
	// 1. BEADS_DIR env var
	if envDir := os.Getenv(EnvBeadsDir); envDir != "" {
		normalized, err := normalizeBasePath(envDir)
		if err != nil {
			return Paths{}, err
		}
		return ResolveFromBase(normalized)
	}

	// 2. Walk up from CWD, stopping at git root
	cwd, err := os.Getwd()
	if err != nil {
		return Paths{}, fmt.Errorf("cannot get current directory: %w", err)
	}

	configDir, configFile, found, err := findConfigUpward(cwd)
	if err != nil {
		return Paths{}, err
	}

	// 3. If not found and in a git worktree, check the main repo root
	if !found {
		worktreeRoot, wtErr := findGitWorktreeRoot(cwd)
		if wtErr == nil && worktreeRoot != "" {
			configDir, configFile, found, err = findConfigUpward(worktreeRoot)
			if err != nil {
				return Paths{}, err
			}
		}
	}

	if !found {
		configDir = filepath.Join(cwd, ".beads")
		configFile = filepath.Join(configDir, "config.yaml")
		if _, err := os.Stat(configFile); err != nil {
			// Check if a .beads dir exists but isn't valid beads-lite
			if _, dirErr := os.Stat(configDir); dirErr == nil {
				if vErr := validateBeadsDir(configDir); vErr != nil {
					return Paths{}, vErr
				}
			}
			return Paths{}, missingConfigErr(configFile)
		}
	}

	return buildPaths(configDir, configFile)
}

// ResolveFromBase resolves Paths from a known .beads directory path.
// Follows redirect files and reads project.name from config.yaml.
func ResolveFromBase(basePath string) (Paths, error) {
	// Check for redirect
	redirected, err := ReadRedirect(basePath)
	if err != nil {
		return Paths{}, err
	}
	if redirected != "" {
		basePath = redirected
	}

	info, err := os.Stat(basePath)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return Paths{}, missingConfigErr(filepath.Join(basePath, "config.yaml"))
		}
		return Paths{}, fmt.Errorf("cannot access beads directory %s: %w", basePath, err)
	}
	if !info.IsDir() {
		return Paths{}, fmt.Errorf("beads path is not a directory: %s", basePath)
	}

	configFile := filepath.Join(basePath, "config.yaml")
	if _, err := os.Stat(configFile); err != nil {
		// The directory exists but has no config.yaml — check if it's an original beads dir
		if vErr := validateBeadsDir(basePath); vErr != nil {
			return Paths{}, vErr
		}
		return Paths{}, missingConfigErr(configFile)
	}

	return buildPaths(basePath, configFile)
}

// buildPaths constructs Paths from the config directory and file.
func buildPaths(configDir, configFile string) (Paths, error) {
	return Paths{
		ConfigDir:  configDir,
		ConfigFile: configFile,
	}, nil
}

func normalizeBasePath(path string) (string, error) {
	absPath, err := filepath.Abs(path)
	if err != nil {
		return "", fmt.Errorf("resolving path: %w", err)
	}
	if filepath.Base(absPath) != ".beads" {
		absPath = filepath.Join(absPath, ".beads")
	}
	return absPath, nil
}

// findConfigUpward walks from start toward the filesystem root looking for .beads/config.yaml.
// It stops at the git repository root (if inside a git repo) to avoid escaping the repo boundary.
// At each .beads/ found, it checks for a redirect file.
func findConfigUpward(start string) (string, string, bool, error) {
	gitRoot, _ := findGitRoot(start)

	dir := start
	for {
		configDir := filepath.Join(dir, ".beads")
		configFile := filepath.Join(configDir, "config.yaml")
		if info, err := os.Stat(configFile); err == nil && !info.IsDir() {
			// Check for redirect
			redirected, rErr := ReadRedirect(configDir)
			if rErr != nil {
				return "", "", false, rErr
			}
			if redirected != "" {
				configDir = redirected
				configFile = filepath.Join(configDir, "config.yaml")
				// Validate the redirected config exists
				if _, err := os.Stat(configFile); err != nil {
					return "", "", false, fmt.Errorf("redirect target has no config.yaml: %s", configDir)
				}
			}
			return configDir, configFile, true, nil
		} else if err != nil && !errors.Is(err, os.ErrNotExist) {
			return "", "", false, fmt.Errorf("checking config: %w", err)
		}

		// Stop at git root boundary
		if gitRoot != "" && dir == gitRoot {
			return "", "", false, nil
		}

		parent := filepath.Dir(dir)
		if parent == dir {
			return "", "", false, nil
		}
		dir = parent
	}
}

// findGitRoot returns the git repository root for the given directory.
// Returns "" if not in a git repo or git is not available.
func findGitRoot(startDir string) (string, error) {
	cmd := exec.Command("git", "rev-parse", "--show-toplevel")
	cmd.Dir = startDir
	out, err := cmd.Output()
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(out)), nil
}

// findGitWorktreeRoot detects if startDir is in a git worktree and returns
// the main repository root. Returns "" if not in a worktree.
func findGitWorktreeRoot(startDir string) (string, error) {
	gitDirCmd := exec.Command("git", "rev-parse", "--git-dir")
	gitDirCmd.Dir = startDir
	gitDirOut, err := gitDirCmd.Output()
	if err != nil {
		return "", err
	}
	gitDir := strings.TrimSpace(string(gitDirOut))

	commonDirCmd := exec.Command("git", "rev-parse", "--git-common-dir")
	commonDirCmd.Dir = startDir
	commonDirOut, err := commonDirCmd.Output()
	if err != nil {
		return "", err
	}
	commonDir := strings.TrimSpace(string(commonDirOut))

	// If --git-dir and --git-common-dir are the same, this is not a worktree
	absGitDir, _ := filepath.Abs(filepath.Join(startDir, gitDir))
	absCommonDir, _ := filepath.Abs(filepath.Join(startDir, commonDir))
	if absGitDir == absCommonDir {
		return "", nil
	}

	// The common dir is the .git dir of the main repo; its parent is the main repo root
	mainRepoRoot := filepath.Dir(absCommonDir)
	return mainRepoRoot, nil
}

// ReadRedirect reads a redirect file from a .beads directory.
// The redirect file contains a single line with an absolute or relative path
// to the actual .beads directory. Returns "" if no redirect file exists.
func ReadRedirect(beadsDir string) (string, error) {
	redirectPath := filepath.Join(beadsDir, "redirect")
	f, err := os.Open(redirectPath)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return "", nil
		}
		return "", fmt.Errorf("reading redirect file: %w", err)
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	if !scanner.Scan() {
		return "", nil // empty file
	}
	target := strings.TrimSpace(scanner.Text())
	if target == "" {
		return "", nil
	}

	// Resolve relative paths against the beads directory
	if !filepath.IsAbs(target) {
		target = filepath.Join(beadsDir, target)
	}
	target = filepath.Clean(target)

	// Validate target exists and is a directory
	info, err := os.Stat(target)
	if err != nil {
		return "", fmt.Errorf("redirect target does not exist: %s", target)
	}
	if !info.IsDir() {
		return "", fmt.Errorf("redirect target is not a directory: %s", target)
	}

	return target, nil
}

// isValidBeadsLiteDir checks whether a directory looks like a valid beads-lite .beads directory.
// A valid beads-lite directory contains a config.yaml file.
func isValidBeadsLiteDir(dir string) bool {
	_, err := os.Stat(filepath.Join(dir, "config.yaml"))
	return err == nil
}

// isOriginalBeadsDir checks whether a directory looks like an original beads .beads directory
// (as opposed to beads-lite). Original beads directories contain metadata.json and/or issues.jsonl.
func isOriginalBeadsDir(dir string) bool {
	if _, err := os.Stat(filepath.Join(dir, "metadata.json")); err == nil {
		return true
	}
	if _, err := os.Stat(filepath.Join(dir, "issues.jsonl")); err == nil {
		return true
	}
	return false
}

// validateBeadsDir checks a .beads directory and returns a descriptive error if it
// is not a valid beads-lite directory. It distinguishes between original beads directories
// and directories that are simply not valid.
func validateBeadsDir(dir string) error {
	if isOriginalBeadsDir(dir) {
		return fmt.Errorf(
			"found .beads directory at %s, but it belongs to the original beads application, not beads-lite. "+
				"Run `bd init` to create a beads-lite repository, or see `bd migrate-v2` for migration options", dir)
	}
	if !isValidBeadsLiteDir(dir) {
		return fmt.Errorf(
			".beads directory at %s is not a valid beads-lite repository (missing config.yaml). "+
				"Run `bd init` to initialize it", dir)
	}
	return nil
}

func missingConfigErr(path string) error {
	return fmt.Errorf("beads config not found at %s (run `bd init`)", path)
}
