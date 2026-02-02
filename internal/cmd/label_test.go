package cmd

import (
	"bytes"
	"context"
	"encoding/json"
	"strings"
	"testing"

	"beads-lite/internal/issuestorage"
)

func TestLabelAddBasic(t *testing.T) {
	app, store := setupTestApp(t)
	out := app.Out.(*bytes.Buffer)

	issue := &issuestorage.Issue{
		Title:    "Test Issue",
		Status:   issuestorage.StatusOpen,
		Priority: issuestorage.PriorityMedium,
		Type:     issuestorage.TypeTask,
	}
	id, err := store.Create(context.Background(), issue)
	if err != nil {
		t.Fatalf("failed to create issue: %v", err)
	}

	cmd := newLabelAddCmd(NewTestProvider(app))
	cmd.SetArgs([]string{id, "urgent"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("label add failed: %v", err)
	}

	output := out.String()
	if !strings.Contains(output, "Added label") {
		t.Errorf("expected output to contain 'Added label', got %q", output)
	}
	if !strings.Contains(output, "urgent") {
		t.Errorf("expected output to contain 'urgent', got %q", output)
	}

	got, err := store.Get(context.Background(), id)
	if err != nil {
		t.Fatalf("failed to get issue: %v", err)
	}
	if !contains(got.Labels, "urgent") {
		t.Errorf("expected labels to contain 'urgent', got %v", got.Labels)
	}
}

func TestLabelAddDedup(t *testing.T) {
	app, store := setupTestApp(t)

	issue := &issuestorage.Issue{
		Title:    "Test Issue",
		Status:   issuestorage.StatusOpen,
		Priority: issuestorage.PriorityMedium,
		Type:     issuestorage.TypeTask,
		Labels:   []string{"urgent"},
	}
	id, err := store.Create(context.Background(), issue)
	if err != nil {
		t.Fatalf("failed to create issue: %v", err)
	}

	cmd := newLabelAddCmd(NewTestProvider(app))
	cmd.SetArgs([]string{id, "urgent"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("label add failed: %v", err)
	}

	got, err := store.Get(context.Background(), id)
	if err != nil {
		t.Fatalf("failed to get issue: %v", err)
	}
	count := 0
	for _, l := range got.Labels {
		if l == "urgent" {
			count++
		}
	}
	if count != 1 {
		t.Errorf("expected exactly 1 'urgent' label, got %d in %v", count, got.Labels)
	}
}

func TestLabelAddJSON(t *testing.T) {
	app, store := setupTestApp(t)
	app.JSON = true
	out := app.Out.(*bytes.Buffer)

	issue := &issuestorage.Issue{
		Title:    "Test Issue",
		Status:   issuestorage.StatusOpen,
		Priority: issuestorage.PriorityMedium,
		Type:     issuestorage.TypeTask,
	}
	id, err := store.Create(context.Background(), issue)
	if err != nil {
		t.Fatalf("failed to create issue: %v", err)
	}

	cmd := newLabelAddCmd(NewTestProvider(app))
	cmd.SetArgs([]string{id, "urgent"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("label add failed: %v", err)
	}

	var result []IssueJSON
	if err := json.Unmarshal(out.Bytes(), &result); err != nil {
		t.Fatalf("failed to parse JSON: %v", err)
	}
	if len(result) != 1 {
		t.Fatalf("expected 1 issue, got %d", len(result))
	}
	if !contains(result[0].Labels, "urgent") {
		t.Errorf("expected labels to contain 'urgent', got %v", result[0].Labels)
	}
}

func TestLabelRemoveBasic(t *testing.T) {
	app, store := setupTestApp(t)
	out := app.Out.(*bytes.Buffer)

	issue := &issuestorage.Issue{
		Title:    "Test Issue",
		Status:   issuestorage.StatusOpen,
		Priority: issuestorage.PriorityMedium,
		Type:     issuestorage.TypeTask,
		Labels:   []string{"urgent", "bug-fix"},
	}
	id, err := store.Create(context.Background(), issue)
	if err != nil {
		t.Fatalf("failed to create issue: %v", err)
	}

	cmd := newLabelRemoveCmd(NewTestProvider(app))
	cmd.SetArgs([]string{id, "urgent"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("label remove failed: %v", err)
	}

	output := out.String()
	if !strings.Contains(output, "Removed label") {
		t.Errorf("expected output to contain 'Removed label', got %q", output)
	}

	got, err := store.Get(context.Background(), id)
	if err != nil {
		t.Fatalf("failed to get issue: %v", err)
	}
	if contains(got.Labels, "urgent") {
		t.Errorf("expected labels to NOT contain 'urgent', got %v", got.Labels)
	}
	if !contains(got.Labels, "bug-fix") {
		t.Errorf("expected labels to still contain 'bug-fix', got %v", got.Labels)
	}
}

func TestLabelRemoveNotPresent(t *testing.T) {
	app, store := setupTestApp(t)

	issue := &issuestorage.Issue{
		Title:    "Test Issue",
		Status:   issuestorage.StatusOpen,
		Priority: issuestorage.PriorityMedium,
		Type:     issuestorage.TypeTask,
		Labels:   []string{"urgent"},
	}
	id, err := store.Create(context.Background(), issue)
	if err != nil {
		t.Fatalf("failed to create issue: %v", err)
	}

	cmd := newLabelRemoveCmd(NewTestProvider(app))
	cmd.SetArgs([]string{id, "nonexistent"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("label remove should succeed even if label not present: %v", err)
	}

	got, err := store.Get(context.Background(), id)
	if err != nil {
		t.Fatalf("failed to get issue: %v", err)
	}
	if !contains(got.Labels, "urgent") {
		t.Errorf("expected labels to still contain 'urgent', got %v", got.Labels)
	}
}

func TestLabelRemoveJSON(t *testing.T) {
	app, store := setupTestApp(t)
	app.JSON = true
	out := app.Out.(*bytes.Buffer)

	issue := &issuestorage.Issue{
		Title:    "Test Issue",
		Status:   issuestorage.StatusOpen,
		Priority: issuestorage.PriorityMedium,
		Type:     issuestorage.TypeTask,
		Labels:   []string{"urgent", "bug-fix"},
	}
	id, err := store.Create(context.Background(), issue)
	if err != nil {
		t.Fatalf("failed to create issue: %v", err)
	}

	cmd := newLabelRemoveCmd(NewTestProvider(app))
	cmd.SetArgs([]string{id, "urgent"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("label remove failed: %v", err)
	}

	var result []IssueJSON
	if err := json.Unmarshal(out.Bytes(), &result); err != nil {
		t.Fatalf("failed to parse JSON: %v", err)
	}
	if len(result) != 1 {
		t.Fatalf("expected 1 issue, got %d", len(result))
	}
	if contains(result[0].Labels, "urgent") {
		t.Errorf("expected labels to NOT contain 'urgent', got %v", result[0].Labels)
	}
	if !contains(result[0].Labels, "bug-fix") {
		t.Errorf("expected labels to contain 'bug-fix', got %v", result[0].Labels)
	}
}

func TestLabelListBasic(t *testing.T) {
	app, store := setupTestApp(t)
	out := app.Out.(*bytes.Buffer)

	issue := &issuestorage.Issue{
		Title:    "Test Issue",
		Status:   issuestorage.StatusOpen,
		Priority: issuestorage.PriorityMedium,
		Type:     issuestorage.TypeTask,
		Labels:   []string{"urgent", "bug-fix"},
	}
	id, err := store.Create(context.Background(), issue)
	if err != nil {
		t.Fatalf("failed to create issue: %v", err)
	}

	cmd := newLabelListCmd(NewTestProvider(app))
	cmd.SetArgs([]string{id})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("label list failed: %v", err)
	}

	output := out.String()
	if !strings.Contains(output, "urgent") {
		t.Errorf("expected output to contain 'urgent', got %q", output)
	}
	if !strings.Contains(output, "bug-fix") {
		t.Errorf("expected output to contain 'bug-fix', got %q", output)
	}
}

func TestLabelListEmpty(t *testing.T) {
	app, store := setupTestApp(t)
	out := app.Out.(*bytes.Buffer)

	issue := &issuestorage.Issue{
		Title:    "Test Issue",
		Status:   issuestorage.StatusOpen,
		Priority: issuestorage.PriorityMedium,
		Type:     issuestorage.TypeTask,
	}
	id, err := store.Create(context.Background(), issue)
	if err != nil {
		t.Fatalf("failed to create issue: %v", err)
	}

	cmd := newLabelListCmd(NewTestProvider(app))
	cmd.SetArgs([]string{id})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("label list failed: %v", err)
	}

	output := out.String()
	if !strings.Contains(output, "no labels") {
		t.Errorf("expected output to contain 'no labels', got %q", output)
	}
}

func TestLabelListJSON(t *testing.T) {
	app, store := setupTestApp(t)
	app.JSON = true
	out := app.Out.(*bytes.Buffer)

	issue := &issuestorage.Issue{
		Title:    "Test Issue",
		Status:   issuestorage.StatusOpen,
		Priority: issuestorage.PriorityMedium,
		Type:     issuestorage.TypeTask,
		Labels:   []string{"urgent"},
	}
	id, err := store.Create(context.Background(), issue)
	if err != nil {
		t.Fatalf("failed to create issue: %v", err)
	}

	cmd := newLabelListCmd(NewTestProvider(app))
	cmd.SetArgs([]string{id})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("label list failed: %v", err)
	}

	var result []IssueJSON
	if err := json.Unmarshal(out.Bytes(), &result); err != nil {
		t.Fatalf("failed to parse JSON: %v", err)
	}
	if len(result) != 1 {
		t.Fatalf("expected 1 issue, got %d", len(result))
	}
	if result[0].ID != id {
		t.Errorf("expected id %q, got %q", id, result[0].ID)
	}
	if !contains(result[0].Labels, "urgent") {
		t.Errorf("expected labels to contain 'urgent', got %v", result[0].Labels)
	}
}

func TestLabelNonExistentIssue(t *testing.T) {
	app, _ := setupTestApp(t)

	cmd := newLabelAddCmd(NewTestProvider(app))
	cmd.SetArgs([]string{"bd-nonexistent", "urgent"})
	err := cmd.Execute()
	if err == nil {
		t.Error("expected error for non-existent issue")
	}
}
