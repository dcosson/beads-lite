package cmd

import (
	"os"
	"path/filepath"
	"testing"
)

func TestInit(t *testing.T) {
	t.Run("creates beads directory structure", func(t *testing.T) {
		tmpDir := t.TempDir()

		err := Init(InitOptions{Path: tmpDir})
		if err != nil {
			t.Fatalf("Init failed: %v", err)
		}

		// Check .beads directory exists
		beadsPath := filepath.Join(tmpDir, ".beads")
		if _, err := os.Stat(beadsPath); os.IsNotExist(err) {
			t.Error(".beads directory was not created")
		}

		// Check open/ subdirectory exists
		openPath := filepath.Join(beadsPath, "open")
		if _, err := os.Stat(openPath); os.IsNotExist(err) {
			t.Error(".beads/open directory was not created")
		}

		// Check closed/ subdirectory exists
		closedPath := filepath.Join(beadsPath, "closed")
		if _, err := os.Stat(closedPath); os.IsNotExist(err) {
			t.Error(".beads/closed directory was not created")
		}
	})

	t.Run("fails if beads already exists", func(t *testing.T) {
		tmpDir := t.TempDir()

		// Create .beads directory first
		beadsPath := filepath.Join(tmpDir, ".beads")
		if err := os.MkdirAll(beadsPath, 0755); err != nil {
			t.Fatalf("setup failed: %v", err)
		}

		err := Init(InitOptions{Path: tmpDir})
		if err == nil {
			t.Error("Init should have failed when .beads already exists")
		}
	})

	t.Run("force flag allows reinitialization", func(t *testing.T) {
		tmpDir := t.TempDir()

		// Create .beads directory first
		beadsPath := filepath.Join(tmpDir, ".beads")
		if err := os.MkdirAll(beadsPath, 0755); err != nil {
			t.Fatalf("setup failed: %v", err)
		}

		err := Init(InitOptions{Path: tmpDir, Force: true})
		if err != nil {
			t.Errorf("Init with --force should succeed: %v", err)
		}

		// Verify directories were created
		openPath := filepath.Join(beadsPath, "open")
		if _, err := os.Stat(openPath); os.IsNotExist(err) {
			t.Error(".beads/open directory was not created after --force")
		}
	})

	t.Run("defaults to current directory", func(t *testing.T) {
		tmpDir := t.TempDir()

		// Change to temp directory
		oldWd, err := os.Getwd()
		if err != nil {
			t.Fatalf("getting current directory: %v", err)
		}
		defer os.Chdir(oldWd)

		if err := os.Chdir(tmpDir); err != nil {
			t.Fatalf("changing directory: %v", err)
		}

		err = Init(InitOptions{})
		if err != nil {
			t.Fatalf("Init failed: %v", err)
		}

		// Check .beads was created in tmpDir
		beadsPath := filepath.Join(tmpDir, ".beads")
		if _, err := os.Stat(beadsPath); os.IsNotExist(err) {
			t.Error(".beads directory was not created in current directory")
		}
	})
}
