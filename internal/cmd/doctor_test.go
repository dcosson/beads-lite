package cmd

import (
	"bytes"
	"context"
	"encoding/json"
	"strings"
	"testing"

	"beads-lite/internal/storage"
	"beads-lite/internal/storage/filesystem"
)

func TestDoctorCmd_NoProblems(t *testing.T) {
	dir := t.TempDir()
	s := filesystem.New(dir)
	if err := s.Init(context.Background()); err != nil {
		t.Fatalf("failed to init storage: %v", err)
	}

	// Create a valid issue
	_, err := s.Create(context.Background(), &storage.Issue{
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
	s := filesystem.New(dir)
	if err := s.Init(context.Background()); err != nil {
		t.Fatalf("failed to init storage: %v", err)
	}

	// Create an issue with a broken parent reference
	issue := &storage.Issue{
		Title:  "Test Issue",
		Parent: "bd-nonexistent",
	}
	id, err := s.Create(context.Background(), issue)
	if err != nil {
		t.Fatalf("failed to create issue: %v", err)
	}

	// Update with the broken parent reference
	issue.ID = id
	if err := s.Update(context.Background(), issue); err != nil {
		t.Fatalf("failed to update issue: %v", err)
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
	s := filesystem.New(dir)
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
	s := filesystem.New(dir)
	if err := s.Init(context.Background()); err != nil {
		t.Fatalf("failed to init storage: %v", err)
	}

	// Create an issue with a broken parent reference
	issue := &storage.Issue{
		Title:  "Test Issue",
		Parent: "bd-nonexistent",
	}
	id, err := s.Create(context.Background(), issue)
	if err != nil {
		t.Fatalf("failed to create issue: %v", err)
	}

	// Update with the broken parent reference
	issue.ID = id
	if err := s.Update(context.Background(), issue); err != nil {
		t.Fatalf("failed to update issue: %v", err)
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
