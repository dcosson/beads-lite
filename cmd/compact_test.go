package cmd

import (
	"bytes"
	"context"
	"encoding/json"
	"testing"
	"time"

	"beads2/filesystem"
	"beads2/storage"
)

func TestCompactCommand_Before(t *testing.T) {
	ctx := context.Background()
	fs := filesystem.New(t.TempDir())
	if err := fs.Init(ctx); err != nil {
		t.Fatal(err)
	}

	// Create and close issues with different closed_at times
	oldIssue, _ := fs.Create(ctx, &storage.Issue{Title: "Old Issue"})
	recentIssue, _ := fs.Create(ctx, &storage.Issue{Title: "Recent Issue"})

	// Close both issues
	fs.Close(ctx, oldIssue)
	fs.Close(ctx, recentIssue)

	// Manually update the old issue's closed_at to be in the past
	old, _ := fs.Get(ctx, oldIssue)
	pastTime := time.Date(2023, 1, 1, 0, 0, 0, 0, time.UTC)
	old.ClosedAt = &pastTime
	fs.Update(ctx, old)

	out := &bytes.Buffer{}
	errOut := &bytes.Buffer{}
	app := &App{Storage: fs, Out: out, Err: errOut}

	cmd := NewCompactCmd(app)
	cmd.SetArgs([]string{"--before", "2024-01-01", "--force"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("Execute failed: %v", err)
	}

	// Verify old issue was deleted
	_, err := fs.Get(ctx, oldIssue)
	if err != storage.ErrNotFound {
		t.Errorf("Expected old issue to be deleted, got err: %v", err)
	}

	// Verify recent issue still exists
	_, err = fs.Get(ctx, recentIssue)
	if err != nil {
		t.Errorf("Expected recent issue to still exist, got err: %v", err)
	}
}

func TestCompactCommand_OlderThan(t *testing.T) {
	ctx := context.Background()
	fs := filesystem.New(t.TempDir())
	if err := fs.Init(ctx); err != nil {
		t.Fatal(err)
	}

	// Create and close an old issue
	oldIssue, _ := fs.Create(ctx, &storage.Issue{Title: "Old Issue"})
	fs.Close(ctx, oldIssue)

	// Manually update the old issue's closed_at to be 100 days ago
	old, _ := fs.Get(ctx, oldIssue)
	pastTime := time.Now().Add(-100 * 24 * time.Hour)
	old.ClosedAt = &pastTime
	fs.Update(ctx, old)

	// Create and close a recent issue
	recentIssue, _ := fs.Create(ctx, &storage.Issue{Title: "Recent Issue"})
	fs.Close(ctx, recentIssue)

	out := &bytes.Buffer{}
	errOut := &bytes.Buffer{}
	app := &App{Storage: fs, Out: out, Err: errOut}

	cmd := NewCompactCmd(app)
	cmd.SetArgs([]string{"--older-than", "90d", "--force"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("Execute failed: %v", err)
	}

	// Verify old issue was deleted
	_, err := fs.Get(ctx, oldIssue)
	if err != storage.ErrNotFound {
		t.Errorf("Expected old issue to be deleted, got err: %v", err)
	}

	// Verify recent issue still exists
	_, err = fs.Get(ctx, recentIssue)
	if err != nil {
		t.Errorf("Expected recent issue to still exist, got err: %v", err)
	}
}

func TestCompactCommand_DryRun(t *testing.T) {
	ctx := context.Background()
	fs := filesystem.New(t.TempDir())
	if err := fs.Init(ctx); err != nil {
		t.Fatal(err)
	}

	// Create and close an old issue
	oldIssue, _ := fs.Create(ctx, &storage.Issue{Title: "Old Issue"})
	fs.Close(ctx, oldIssue)

	// Manually update the old issue's closed_at to be in the past
	old, _ := fs.Get(ctx, oldIssue)
	pastTime := time.Date(2023, 1, 1, 0, 0, 0, 0, time.UTC)
	old.ClosedAt = &pastTime
	fs.Update(ctx, old)

	out := &bytes.Buffer{}
	errOut := &bytes.Buffer{}
	app := &App{Storage: fs, Out: out, Err: errOut}

	cmd := NewCompactCmd(app)
	cmd.SetArgs([]string{"--before", "2024-01-01", "--dry-run"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("Execute failed: %v", err)
	}

	// Verify issue was NOT deleted (dry run)
	_, err := fs.Get(ctx, oldIssue)
	if err != nil {
		t.Errorf("Expected issue to still exist after dry run, got err: %v", err)
	}

	// Check output mentions the issue
	output := out.String()
	if !bytes.Contains([]byte(output), []byte("Would delete")) {
		t.Errorf("Expected dry run output to contain 'Would delete', got: %s", output)
	}
}

func TestCompactCommand_DryRunJSON(t *testing.T) {
	ctx := context.Background()
	fs := filesystem.New(t.TempDir())
	if err := fs.Init(ctx); err != nil {
		t.Fatal(err)
	}

	// Create and close an old issue
	oldIssue, _ := fs.Create(ctx, &storage.Issue{Title: "Old Issue"})
	fs.Close(ctx, oldIssue)

	// Manually update the old issue's closed_at to be in the past
	old, _ := fs.Get(ctx, oldIssue)
	pastTime := time.Date(2023, 1, 1, 0, 0, 0, 0, time.UTC)
	old.ClosedAt = &pastTime
	fs.Update(ctx, old)

	out := &bytes.Buffer{}
	errOut := &bytes.Buffer{}
	app := &App{Storage: fs, Out: out, Err: errOut, JSON: true}

	cmd := NewCompactCmd(app)
	cmd.SetArgs([]string{"--before", "2024-01-01", "--dry-run"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("Execute failed: %v", err)
	}

	// Parse JSON output
	var result map[string]interface{}
	if err := json.Unmarshal(out.Bytes(), &result); err != nil {
		t.Fatalf("Failed to parse JSON output: %v", err)
	}

	if result["count"].(float64) != 1 {
		t.Errorf("Expected count=1, got %v", result["count"])
	}

	wouldDelete := result["would_delete"].([]interface{})
	if len(wouldDelete) != 1 || wouldDelete[0].(string) != oldIssue {
		t.Errorf("Expected would_delete to contain %s, got %v", oldIssue, wouldDelete)
	}
}

func TestCompactCommand_NoMatchingIssues(t *testing.T) {
	ctx := context.Background()
	fs := filesystem.New(t.TempDir())
	if err := fs.Init(ctx); err != nil {
		t.Fatal(err)
	}

	// Create and close a recent issue
	recentIssue, _ := fs.Create(ctx, &storage.Issue{Title: "Recent Issue"})
	fs.Close(ctx, recentIssue)

	out := &bytes.Buffer{}
	errOut := &bytes.Buffer{}
	app := &App{Storage: fs, Out: out, Err: errOut}

	cmd := NewCompactCmd(app)
	cmd.SetArgs([]string{"--before", "2020-01-01", "--force"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("Execute failed: %v", err)
	}

	output := out.String()
	if !bytes.Contains([]byte(output), []byte("No issues to delete")) {
		t.Errorf("Expected output to contain 'No issues to delete', got: %s", output)
	}
}

func TestCompactCommand_RequiresFlag(t *testing.T) {
	ctx := context.Background()
	fs := filesystem.New(t.TempDir())
	if err := fs.Init(ctx); err != nil {
		t.Fatal(err)
	}

	out := &bytes.Buffer{}
	errOut := &bytes.Buffer{}
	app := &App{Storage: fs, Out: out, Err: errOut}

	cmd := NewCompactCmd(app)
	cmd.SetArgs([]string{})

	err := cmd.Execute()
	if err == nil {
		t.Fatal("Expected error when no flags provided")
	}
	if !bytes.Contains([]byte(err.Error()), []byte("must specify --before or --older-than")) {
		t.Errorf("Expected error about missing flags, got: %v", err)
	}
}

func TestCompactCommand_MutuallyExclusiveFlags(t *testing.T) {
	ctx := context.Background()
	fs := filesystem.New(t.TempDir())
	if err := fs.Init(ctx); err != nil {
		t.Fatal(err)
	}

	out := &bytes.Buffer{}
	errOut := &bytes.Buffer{}
	app := &App{Storage: fs, Out: out, Err: errOut}

	cmd := NewCompactCmd(app)
	cmd.SetArgs([]string{"--before", "2024-01-01", "--older-than", "90d"})

	err := cmd.Execute()
	if err == nil {
		t.Fatal("Expected error when both flags provided")
	}
	if !bytes.Contains([]byte(err.Error()), []byte("cannot specify both")) {
		t.Errorf("Expected error about mutually exclusive flags, got: %v", err)
	}
}

func TestParseDuration(t *testing.T) {
	tests := []struct {
		input    string
		expected time.Duration
		wantErr  bool
	}{
		{"90d", 90 * 24 * time.Hour, false},
		{"1d", 24 * time.Hour, false},
		{"2w", 14 * 24 * time.Hour, false},
		{"6m", 180 * 24 * time.Hour, false},
		{"1y", 365 * 24 * time.Hour, false},
		{"24h", 24 * time.Hour, false}, // Standard Go duration
		{"invalid", 0, true},
		{"", 0, true},
	}

	for _, tc := range tests {
		t.Run(tc.input, func(t *testing.T) {
			got, err := parseDuration(tc.input)
			if tc.wantErr {
				if err == nil {
					t.Errorf("Expected error for input %q", tc.input)
				}
				return
			}
			if err != nil {
				t.Errorf("Unexpected error for input %q: %v", tc.input, err)
				return
			}
			if got != tc.expected {
				t.Errorf("parseDuration(%q) = %v, want %v", tc.input, got, tc.expected)
			}
		})
	}
}
