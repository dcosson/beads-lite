// Package configservice provides path resolution and detection logic for beads configuration.
// It wraps the config storage layer and handles beads-lite vs original beads detection.
package configservice

import (
	"bufio"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"beads-lite/internal/config"
	"beads-lite/internal/config/yamlstore"
)

// ResolvePaths resolves config and data paths.
// Discovery order: BEADS_DIR env var > walk up from CWD (stopping at git root, with worktree fallback).
func ResolvePaths() (config.Paths, error) {
	// 1. BEADS_DIR env var
	if envDir := os.Getenv(config.EnvBeadsDir); envDir != "" {
		normalized, err := normalizeBasePath(envDir)
		if err != nil {
			return config.Paths{}, err
		}
		return ResolveFromBase(normalized)
	}

	// 2. Walk up from CWD, stopping at git root
	cwd, err := os.Getwd()
	if err != nil {
		return config.Paths{}, fmt.Errorf("cannot get current directory: %w", err)
	}

	paths, found, err := findConfigUpward(cwd)
	if err != nil {
		return config.Paths{}, err
	}

	// 3. If not found and in a git worktree, check the main repo root
	if !found {
		worktreeRoot, wtErr := findGitWorktreeRoot(cwd)
		if wtErr == nil && worktreeRoot != "" {
			paths, found, err = findConfigUpward(worktreeRoot)
			if err != nil {
				return config.Paths{}, err
			}
		}
	}

	if !found {
		configDir := filepath.Join(cwd, ".beads")
		configFile := filepath.Join(configDir, "config.yaml")
		if _, err := os.Stat(configFile); err != nil {
			// Check if a .beads dir exists but isn't valid beads-lite
			if _, dirErr := os.Stat(configDir); dirErr == nil {
				if vErr := ValidateBeadsDir(configDir); vErr != nil {
					return config.Paths{}, vErr
				}
			}
			return config.Paths{}, missingConfigErr(configFile)
		}
		paths = config.Paths{ConfigDir: configDir, ConfigFile: configFile}
	}

	return paths, nil
}

// ResolveFromBase resolves Paths from a known .beads directory path.
// Follows redirect files; when the redirecting directory has its own
// config.yaml, that file is recorded as OverlayConfigFile.
func ResolveFromBase(basePath string) (config.Paths, error) {
	// Check for redirect
	redirected, err := ReadRedirect(basePath)
	if err != nil {
		return config.Paths{}, err
	}
	overlayFile := ""
	if redirected != "" {
		overlayFile = overlayConfigFile(basePath)
		basePath = redirected
	}

	info, err := os.Stat(basePath)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return config.Paths{}, missingConfigErr(filepath.Join(basePath, "config.yaml"))
		}
		return config.Paths{}, fmt.Errorf("cannot access beads directory %s: %w", basePath, err)
	}
	if !info.IsDir() {
		return config.Paths{}, fmt.Errorf("beads path is not a directory: %s", basePath)
	}

	configFile := filepath.Join(basePath, "config.yaml")
	if _, err := os.Stat(configFile); err != nil {
		// The directory exists but has no config.yaml — check if it's an original beads dir
		if vErr := ValidateBeadsDir(basePath); vErr != nil {
			return config.Paths{}, vErr
		}
		return config.Paths{}, missingConfigErr(configFile)
	}

	return config.Paths{
		ConfigDir:         basePath,
		ConfigFile:        configFile,
		OverlayConfigFile: overlayFile,
	}, nil
}

// overlayConfigFile returns the path to config.yaml in a redirecting .beads
// directory if one exists, or "" otherwise.
func overlayConfigFile(beadsDir string) string {
	configFile := filepath.Join(beadsDir, "config.yaml")
	if info, err := os.Stat(configFile); err == nil && !info.IsDir() {
		return configFile
	}
	return ""
}

// ApplyOverlay loads the given overlay config file and applies its keys as
// in-memory overrides on the store, without persisting them. This gives a
// redirecting .beads directory's config.yaml precedence over the redirect
// target's config (e.g. a per-repo issue_prefix over a shared beads
// directory). No-op when overlayFile is empty.
func ApplyOverlay(s config.Store, overlayFile string) error {
	if overlayFile == "" {
		return nil
	}
	overlay, err := yamlstore.New(overlayFile)
	if err != nil {
		return fmt.Errorf("loading redirect overlay config %s: %w", overlayFile, err)
	}
	for k, v := range overlay.All() {
		s.SetInMemory(k, v)
	}
	return nil
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

// findConfigUpward walks from start toward the filesystem root looking for a
// .beads directory containing either a redirect file or a config.yaml.
// It stops at the git repository root (if inside a git repo) to avoid escaping
// the repo boundary. A redirect file takes precedence and does not require a
// config.yaml alongside it; when one is present it becomes OverlayConfigFile.
func findConfigUpward(start string) (config.Paths, bool, error) {
	gitRoot, _ := FindGitRoot(start)

	dir := start
	for {
		configDir := filepath.Join(dir, ".beads")
		configFile := filepath.Join(configDir, "config.yaml")

		redirected, rErr := ReadRedirect(configDir)
		if rErr != nil {
			return config.Paths{}, false, rErr
		}
		if redirected != "" {
			targetConfigFile := filepath.Join(redirected, "config.yaml")
			if _, err := os.Stat(targetConfigFile); err != nil {
				return config.Paths{}, false, fmt.Errorf("redirect target has no config.yaml: %s", redirected)
			}
			return config.Paths{
				ConfigDir:         redirected,
				ConfigFile:        targetConfigFile,
				OverlayConfigFile: overlayConfigFile(configDir),
			}, true, nil
		}

		if info, err := os.Stat(configFile); err == nil && !info.IsDir() {
			return config.Paths{ConfigDir: configDir, ConfigFile: configFile}, true, nil
		} else if err != nil && !errors.Is(err, os.ErrNotExist) {
			return config.Paths{}, false, fmt.Errorf("checking config: %w", err)
		}

		// Stop at git root boundary
		if gitRoot != "" && dir == gitRoot {
			return config.Paths{}, false, nil
		}

		parent := filepath.Dir(dir)
		if parent == dir {
			return config.Paths{}, false, nil
		}
		dir = parent
	}
}

// FindGitRoot returns the git repository root for the given directory.
// Returns "" if not in a git repo. Uses file walk-up instead of subprocess for speed.
func FindGitRoot(startDir string) (string, error) {
	dir := startDir
	for {
		gitPath := filepath.Join(dir, ".git")
		if info, err := os.Stat(gitPath); err == nil {
			// .git can be a directory (normal repo) or a file (worktree)
			if info.IsDir() || info.Mode().IsRegular() {
				return dir, nil
			}
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return "", nil // reached filesystem root
		}
		dir = parent
	}
}

// findGitWorktreeRoot detects if startDir is in a git worktree and returns
// the main repository root. Returns "" if not in a worktree.
// Uses file reading instead of subprocess for speed.
func findGitWorktreeRoot(startDir string) (string, error) {
	gitRoot, err := FindGitRoot(startDir)
	if err != nil || gitRoot == "" {
		return "", err
	}

	gitPath := filepath.Join(gitRoot, ".git")
	info, err := os.Stat(gitPath)
	if err != nil {
		return "", err
	}

	// If .git is a directory, this is a normal repo, not a worktree
	if info.IsDir() {
		return "", nil
	}

	// .git is a file — this is a worktree. Read it to find the gitdir.
	// Format: "gitdir: /path/to/.git/worktrees/name"
	content, err := os.ReadFile(gitPath)
	if err != nil {
		return "", err
	}

	line := strings.TrimSpace(string(content))
	if !strings.HasPrefix(line, "gitdir: ") {
		return "", nil // unexpected format
	}
	worktreeGitDir := strings.TrimPrefix(line, "gitdir: ")

	// Make path absolute if relative
	if !filepath.IsAbs(worktreeGitDir) {
		worktreeGitDir = filepath.Join(gitRoot, worktreeGitDir)
	}
	worktreeGitDir = filepath.Clean(worktreeGitDir)

	// The worktree gitdir is typically .git/worktrees/<name>
	// The main repo root is the parent of .git (i.e., grandparent of worktrees)
	// Check for commondir file which points to the main .git
	commondirPath := filepath.Join(worktreeGitDir, "commondir")
	commondirContent, err := os.ReadFile(commondirPath)
	if err != nil {
		// No commondir file, try to infer from path structure
		// worktreeGitDir is typically /path/to/main/.git/worktrees/name
		if strings.Contains(worktreeGitDir, string(filepath.Separator)+"worktrees"+string(filepath.Separator)) {
			// Go up: name -> worktrees -> .git -> main repo root
			mainGitDir := filepath.Dir(filepath.Dir(worktreeGitDir))
			return filepath.Dir(mainGitDir), nil
		}
		return "", nil
	}

	// commondir contains relative path to main .git (usually "..")
	commondir := strings.TrimSpace(string(commondirContent))
	if !filepath.IsAbs(commondir) {
		commondir = filepath.Join(worktreeGitDir, commondir)
	}
	commondir = filepath.Clean(commondir)

	// Main repo root is parent of .git
	return filepath.Dir(commondir), nil
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

// readBeadsVariant reads the beads_variant config value from a .beads directory.
// Returns the value and true if found, or "" and false if not found or on error.
func readBeadsVariant(dir string) (string, bool) {
	configPath := filepath.Join(dir, "config.yaml")
	store, err := yamlstore.New(configPath)
	if err != nil {
		return "", false
	}
	return store.Get(config.BeadsVariantKey)
}

// IsValidBeadsLiteDir checks whether a directory looks like a valid beads-lite .beads directory.
// A valid beads-lite directory contains a config.yaml file with beads_variant set to "beads-lite".
func IsValidBeadsLiteDir(dir string) bool {
	variant, ok := readBeadsVariant(dir)
	return ok && variant == config.BeadsVariantValue
}

// IsOriginalBeadsDir checks whether a directory looks like an original beads .beads directory
// (as opposed to beads-lite). Original beads directories contain metadata.json and/or issues.jsonl,
// and do not have beads_variant set to "beads-lite".
func IsOriginalBeadsDir(dir string) bool {
	// Must have at least one of the original beads marker files
	hasMetadata := false
	if _, err := os.Stat(filepath.Join(dir, "metadata.json")); err == nil {
		hasMetadata = true
	}
	hasIssuesJSONL := false
	if _, err := os.Stat(filepath.Join(dir, "issues.jsonl")); err == nil {
		hasIssuesJSONL = true
	}
	if !hasMetadata && !hasIssuesJSONL {
		return false
	}

	// Check that beads_variant is NOT set to beads-lite
	variant, _ := readBeadsVariant(dir)
	return variant != config.BeadsVariantValue
}

// ValidateBeadsDir checks a .beads directory and returns a descriptive error if it
// is not a valid beads-lite directory. It distinguishes between original beads directories
// and directories that are simply not valid.
func ValidateBeadsDir(dir string) error {
	if IsOriginalBeadsDir(dir) {
		return fmt.Errorf(
			"found .beads directory at %s, but it belongs to the original beads application, not beads-lite. "+
				"Run `bd init` to create a beads-lite repository, or see `bd migrate-v2` for migration options", dir)
	}
	if !IsValidBeadsLiteDir(dir) {
		return fmt.Errorf(
			".beads directory at %s is not a valid beads-lite repository (missing config.yaml or beads_variant). "+
				"Run `bd init` to initialize it", dir)
	}
	return nil
}

func missingConfigErr(path string) error {
	return fmt.Errorf("beads config not found at %s (run `bd init`)", path)
}
