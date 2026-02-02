package cmd

import (
	"bytes"
	"context"
	"encoding/json"
	"strings"
	"testing"

	"beads-lite/internal/issuestorage"
	"beads-lite/internal/issuestorage/filesystem"
)

func TestDoctorCmd_NoProblems(t *testing.T) {
	dir := t.TempDir()
	s := filesystem.New(dir, "bd-")
	if err := s.Init(context.Background()); err != nil {
		t.Fatalf("failed to init storage: %v", err)
	}

	// Create a valid issue
	_, err := s.Create(context.Background(), &issuestorage.Issue{
		Title: "Test Issue",
	})
	if err != nil {
		t.Fatalf("failed to create issue: %v", err)
	}

	var out bytes.Buffer
	app := &App{
		Storage: s,
		Out:     &out,
	}

	cmd := newDoctorCmd(NewTestProvider(app))
	if err := cmd.Execute(); err != nil {
		t.Fatalf("doctor command failed: %v", err)
	}

	if !strings.Contains(out.String(), "No problems found") {
		t.Errorf("expected 'No problems found', got: %s", out.String())
	}
}

func TestDoctorCmd_WithProblems(t *testing.T) {
	dir := t.TempDir()
	s := filesystem.New(dir, "bd-")
	if err := s.Init(context.Background()); err != nil {
		t.Fatalf("failed to init storage: %v", err)
	}

	// Create an issue with a broken parent reference
	issue := &issuestorage.Issue{
		Title:  "Test Issue",
		Parent: "bd-nonexistent",
	}
	id, err := s.Create(context.Background(), issue)
	if err != nil {
		t.Fatalf("failed to create issue: %v", err)
	}

	// Ensure the broken parent reference is set
	if err := s.Modify(context.Background(), id, func(i *issuestorage.Issue) error {
		i.Parent = "bd-nonexistent"
		return nil
	}); err != nil {
		t.Fatalf("failed to modify issue: %v", err)
	}

	var out bytes.Buffer
	app := &App{
		Storage: s,
		Out:     &out,
	}

	cmd := newDoctorCmd(NewTestProvider(app))
	if err := cmd.Execute(); err != nil {
		t.Fatalf("doctor command failed: %v", err)
	}

	if !strings.Contains(out.String(), "problem") {
		t.Errorf("expected problems to be found, got: %s", out.String())
	}
	if !strings.Contains(out.String(), "broken parent reference") {
		t.Errorf("expected 'broken parent reference', got: %s", out.String())
	}
}

func TestDoctorCmd_JSON(t *testing.T) {
	dir := t.TempDir()
	s := filesystem.New(dir, "bd-")
	if err := s.Init(context.Background()); err != nil {
		t.Fatalf("failed to init storage: %v", err)
	}

	var out bytes.Buffer
	app := &App{
		Storage: s,
		Out:     &out,
		JSON:    true,
	}

	cmd := newDoctorCmd(NewTestProvider(app))
	if err := cmd.Execute(); err != nil {
		t.Fatalf("doctor command failed: %v", err)
	}

	var result DoctorResult
	if err := json.Unmarshal(out.Bytes(), &result); err != nil {
		t.Fatalf("failed to parse JSON output: %v", err)
	}

	if len(result.Problems) != 0 {
		t.Errorf("expected 0 problems, got %d", len(result.Problems))
	}
	if result.Fixed {
		t.Errorf("expected fixed=false, got true")
	}
}

func TestDoctorCmd_Fix(t *testing.T) {
	dir := t.TempDir()
	s := filesystem.New(dir, "bd-")
	if err := s.Init(context.Background()); err != nil {
		t.Fatalf("failed to init storage: %v", err)
	}

	// Create an issue with a broken parent reference
	issue := &issuestorage.Issue{
		Title:  "Test Issue",
		Parent: "bd-nonexistent",
	}
	id, err := s.Create(context.Background(), issue)
	if err != nil {
		t.Fatalf("failed to create issue: %v", err)
	}

	// Ensure the broken parent reference is set
	if err := s.Modify(context.Background(), id, func(i *issuestorage.Issue) error {
		i.Parent = "bd-nonexistent"
		return nil
	}); err != nil {
		t.Fatalf("failed to modify issue: %v", err)
	}

	var out bytes.Buffer
	app := &App{
		Storage: s,
		Out:     &out,
	}

	cmd := newDoctorCmd(NewTestProvider(app))
	cmd.SetArgs([]string{"--fix"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("doctor command failed: %v", err)
	}

	if !strings.Contains(out.String(), "Fixed") {
		t.Errorf("expected 'Fixed', got: %s", out.String())
	}

	// Verify the issue is fixed
	out.Reset()
	cmd = newDoctorCmd(NewTestProvider(app))
	if err := cmd.Execute(); err != nil {
		t.Fatalf("doctor command failed: %v", err)
	}

	if !strings.Contains(out.String(), "No problems found") {
		t.Errorf("expected 'No problems found' after fix, got: %s", out.String())
	}
}
