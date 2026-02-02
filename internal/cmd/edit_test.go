package cmd

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"beads-lite/internal/issuestorage"
	"beads-lite/internal/issuestorage/filesystem"
)

// makeEditorScript creates a shell script that replaces temp file contents
// with the given text. Returns the path to the script.
func makeEditorScript(t *testing.T, newContent string) string {
	t.Helper()
	dir := t.TempDir()

	if runtime.GOOS == "windows" {
		t.Skip("editor tests require unix shell")
	}

	script := filepath.Join(dir, "editor.sh")
	content := "#!/bin/sh\nprintf '%s' '" + escapeShell(newContent) + "' > \"$1\"\n"
	if err := os.WriteFile(script, []byte(content), 0755); err != nil {
		t.Fatalf("creating editor script: %v", err)
	}
	return script
}

// makeNoopEditor creates a script that does nothing (leaves file unchanged).
func makeNoopEditor(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	script := filepath.Join(dir, "editor.sh")
	if err := os.WriteFile(script, []byte("#!/bin/sh\n"), 0755); err != nil {
		t.Fatalf("creating noop editor script: %v", err)
	}
	return script
}

// makeFailingEditor creates a script that exits non-zero.
func makeFailingEditor(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	script := filepath.Join(dir, "editor.sh")
	if err := os.WriteFile(script, []byte("#!/bin/sh\nexit 1\n"), 0755); err != nil {
		t.Fatalf("creating failing editor script: %v", err)
	}
	return script
}

func escapeShell(s string) string {
	// Escape single quotes for shell: replace ' with '\''
	result := ""
	for _, c := range s {
		if c == '\'' {
			result += "'\\''"
		} else {
			result += string(c)
		}
	}
	return result
}

func TestEditCommand(t *testing.T) {
	dir := t.TempDir()
	store := filesystem.New(dir, "bd-")
	ctx := context.Background()
	if err := store.Init(ctx); err != nil {
		t.Fatalf("failed to init storage: %v", err)
	}

	id, err := store.Create(ctx, &issuestorage.Issue{
		Title:       "Test issue",
		Description: "Original description",
		Priority:    issuestorage.PriorityMedium,
	})
	if err != nil {
		t.Fatalf("failed to create issue: %v", err)
	}

	editor := makeEditorScript(t, "Updated description")
	t.Setenv("EDITOR", editor)

	var out bytes.Buffer
	app := &App{
		Storage: store,
		Out:     &out,
	}

	cmd := newEditCmd(NewTestProvider(app))
	cmd.SetArgs([]string{id})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("edit command failed: %v", err)
	}

	output := out.String()
	if !bytes.Contains([]byte(output), []byte("Updated description for")) {
		t.Errorf("expected 'Updated description for' in output, got: %s", output)
	}

	// Verify description was updated
	issue, err := store.Get(ctx, id)
	if err != nil {
		t.Fatalf("failed to get issue: %v", err)
	}
	if issue.Description != "Updated description" {
		t.Errorf("expected description 'Updated description', got: %q", issue.Description)
	}
}

func TestEditNoChanges(t *testing.T) {
	dir := t.TempDir()
	store := filesystem.New(dir, "bd-")
	ctx := context.Background()
	if err := store.Init(ctx); err != nil {
		t.Fatalf("failed to init storage: %v", err)
	}

	id, err := store.Create(ctx, &issuestorage.Issue{
		Title:       "Test issue",
		Description: "Original description",
		Priority:    issuestorage.PriorityMedium,
	})
	if err != nil {
		t.Fatalf("failed to create issue: %v", err)
	}

	editor := makeNoopEditor(t)
	t.Setenv("EDITOR", editor)

	var out bytes.Buffer
	app := &App{
		Storage: store,
		Out:     &out,
	}

	cmd := newEditCmd(NewTestProvider(app))
	cmd.SetArgs([]string{id})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("edit command failed: %v", err)
	}

	output := out.String()
	if !bytes.Contains([]byte(output), []byte("No changes")) {
		t.Errorf("expected 'No changes' in output, got: %s", output)
	}

	// Verify description unchanged
	issue, err := store.Get(ctx, id)
	if err != nil {
		t.Fatalf("failed to get issue: %v", err)
	}
	if issue.Description != "Original description" {
		t.Errorf("expected description unchanged, got: %q", issue.Description)
	}
}

func TestEditEditorFails(t *testing.T) {
	dir := t.TempDir()
	store := filesystem.New(dir, "bd-")
	ctx := context.Background()
	if err := store.Init(ctx); err != nil {
		t.Fatalf("failed to init storage: %v", err)
	}

	id, err := store.Create(ctx, &issuestorage.Issue{
		Title:       "Test issue",
		Description: "Original description",
		Priority:    issuestorage.PriorityMedium,
	})
	if err != nil {
		t.Fatalf("failed to create issue: %v", err)
	}

	editor := makeFailingEditor(t)
	t.Setenv("EDITOR", editor)

	var out bytes.Buffer
	app := &App{
		Storage: store,
		Out:     &out,
	}

	cmd := newEditCmd(NewTestProvider(app))
	cmd.SetArgs([]string{id})
	err = cmd.Execute()
	if err == nil {
		t.Fatal("expected error when editor exits non-zero")
	}
	if !bytes.Contains([]byte(err.Error()), []byte("editor exited with error")) {
		t.Errorf("expected 'editor exited with error' in error, got: %s", err.Error())
	}

	// Verify description unchanged
	issue, err := store.Get(ctx, id)
	if err != nil {
		t.Fatalf("failed to get issue: %v", err)
	}
	if issue.Description != "Original description" {
		t.Errorf("expected description unchanged after editor failure, got: %q", issue.Description)
	}
}

func TestEditNotFound(t *testing.T) {
	dir := t.TempDir()
	store := filesystem.New(dir, "bd-")
	ctx := context.Background()
	if err := store.Init(ctx); err != nil {
		t.Fatalf("failed to init storage: %v", err)
	}

	var out bytes.Buffer
	app := &App{
		Storage: store,
		Out:     &out,
	}

	cmd := newEditCmd(NewTestProvider(app))
	cmd.SetArgs([]string{"nonexistent"})
	err := cmd.Execute()
	if err == nil {
		t.Fatal("expected error for non-existent issue")
	}
	if !bytes.Contains([]byte(err.Error()), []byte("no issue found")) {
		t.Errorf("expected 'no issue found' error, got: %s", err.Error())
	}
}

func TestEditJSON(t *testing.T) {
	dir := t.TempDir()
	store := filesystem.New(dir, "bd-")
	ctx := context.Background()
	if err := store.Init(ctx); err != nil {
		t.Fatalf("failed to init storage: %v", err)
	}

	id, err := store.Create(ctx, &issuestorage.Issue{
		Title:       "Test issue",
		Description: "Original description",
		Priority:    issuestorage.PriorityMedium,
	})
	if err != nil {
		t.Fatalf("failed to create issue: %v", err)
	}

	editor := makeEditorScript(t, "JSON updated description")
	t.Setenv("EDITOR", editor)

	var out bytes.Buffer
	app := &App{
		Storage: store,
		Out:     &out,
		JSON:    true,
	}

	cmd := newEditCmd(NewTestProvider(app))
	cmd.SetArgs([]string{id})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("edit command JSON failed: %v", err)
	}

	var result []map[string]interface{}
	if err := json.Unmarshal(out.Bytes(), &result); err != nil {
		t.Fatalf("failed to parse JSON output: %v\nraw: %s", err, out.String())
	}
	if len(result) != 1 {
		t.Fatalf("expected 1 issue, got %d", len(result))
	}
	if result[0]["id"].(string) != id {
		t.Errorf("expected id %s, got: %s", id, result[0]["id"])
	}
	if result[0]["description"].(string) != "JSON updated description" {
		t.Errorf("expected updated description in JSON, got: %s", result[0]["description"])
	}
}

func TestEditJSONNoChanges(t *testing.T) {
	dir := t.TempDir()
	store := filesystem.New(dir, "bd-")
	ctx := context.Background()
	if err := store.Init(ctx); err != nil {
		t.Fatalf("failed to init storage: %v", err)
	}

	id, err := store.Create(ctx, &issuestorage.Issue{
		Title:       "Test issue",
		Description: "Original description",
		Priority:    issuestorage.PriorityMedium,
	})
	if err != nil {
		t.Fatalf("failed to create issue: %v", err)
	}

	editor := makeNoopEditor(t)
	t.Setenv("EDITOR", editor)

	var out bytes.Buffer
	app := &App{
		Storage: store,
		Out:     &out,
		JSON:    true,
	}

	cmd := newEditCmd(NewTestProvider(app))
	cmd.SetArgs([]string{id})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("edit command JSON no-changes failed: %v", err)
	}

	var result []map[string]interface{}
	if err := json.Unmarshal(out.Bytes(), &result); err != nil {
		t.Fatalf("failed to parse JSON output: %v\nraw: %s", err, out.String())
	}
	if len(result) != 1 {
		t.Fatalf("expected 1 issue, got %d", len(result))
	}
	if result[0]["description"].(string) != "Original description" {
		t.Errorf("expected original description in JSON, got: %s", result[0]["description"])
	}
}

func TestEditEmptyDescription(t *testing.T) {
	dir := t.TempDir()
	store := filesystem.New(dir, "bd-")
	ctx := context.Background()
	if err := store.Init(ctx); err != nil {
		t.Fatalf("failed to init storage: %v", err)
	}

	id, err := store.Create(ctx, &issuestorage.Issue{
		Title:       "Test issue",
		Description: "Some description",
		Priority:    issuestorage.PriorityMedium,
	})
	if err != nil {
		t.Fatalf("failed to create issue: %v", err)
	}

	// Editor that clears the file
	editor := makeEditorScript(t, "")
	t.Setenv("EDITOR", editor)

	var out bytes.Buffer
	app := &App{
		Storage: store,
		Out:     &out,
	}

	cmd := newEditCmd(NewTestProvider(app))
	cmd.SetArgs([]string{id})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("edit command failed for empty description: %v", err)
	}

	// Verify description was cleared
	issue, err := store.Get(ctx, id)
	if err != nil {
		t.Fatalf("failed to get issue: %v", err)
	}
	if issue.Description != "" {
		t.Errorf("expected empty description, got: %q", issue.Description)
	}
}

func TestEditFallbackToVisual(t *testing.T) {
	dir := t.TempDir()
	store := filesystem.New(dir, "bd-")
	ctx := context.Background()
	if err := store.Init(ctx); err != nil {
		t.Fatalf("failed to init storage: %v", err)
	}

	id, err := store.Create(ctx, &issuestorage.Issue{
		Title:       "Test issue",
		Description: "Original",
		Priority:    issuestorage.PriorityMedium,
	})
	if err != nil {
		t.Fatalf("failed to create issue: %v", err)
	}

	editor := makeEditorScript(t, "Via VISUAL")
	t.Setenv("EDITOR", "")
	t.Setenv("VISUAL", editor)

	var out bytes.Buffer
	app := &App{
		Storage: store,
		Out:     &out,
	}

	cmd := newEditCmd(NewTestProvider(app))
	cmd.SetArgs([]string{id})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("edit command with VISUAL failed: %v", err)
	}

	issue, err := store.Get(ctx, id)
	if err != nil {
		t.Fatalf("failed to get issue: %v", err)
	}
	if issue.Description != "Via VISUAL" {
		t.Errorf("expected 'Via VISUAL', got: %q", issue.Description)
	}
}
