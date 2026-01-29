package cmd

import (
	"context"
	"os/exec"
	"strings"
	"time"

	"beads-lite/internal/storage"
)

// IssueJSON is the JSON output format matching original beads.
// Used for create, show, and list commands.
type IssueJSON struct {
	Assignee        string           `json:"assignee,omitempty"`
	Children        []string         `json:"children,omitempty"`
	Comments        []CommentJSON    `json:"comments,omitempty"`
	CreatedAt       string           `json:"created_at"`
	CreatedBy       string           `json:"created_by,omitempty"`
	Dependencies    []EnrichedDepJSON `json:"dependencies,omitempty"`
	DependencyCount *int             `json:"dependency_count,omitempty"`
	Dependents      []EnrichedDepJSON `json:"dependents,omitempty"`
	DependentCount  *int             `json:"dependent_count,omitempty"`
	Description     string           `json:"description,omitempty"`
	ID              string           `json:"id"`
	IssueType       string           `json:"issue_type"`
	Labels          []string         `json:"labels,omitempty"`
	Owner           string           `json:"owner,omitempty"`
	Parent          string           `json:"parent,omitempty"`
	Priority        int              `json:"priority"`
	Status          string           `json:"status"`
	Title           string           `json:"title"`
	UpdatedAt       string           `json:"updated_at"`
	ClosedAt        string           `json:"closed_at,omitempty"`
}

// EnrichedDepJSON is a dependency with full issue details for JSON output.
type EnrichedDepJSON struct {
	CreatedAt       string `json:"created_at"`
	CreatedBy       string `json:"created_by,omitempty"`
	DependencyType  string `json:"dependency_type"`
	ID              string `json:"id"`
	IssueType       string `json:"issue_type"`
	Owner           string `json:"owner,omitempty"`
	Priority        int    `json:"priority"`
	Status          string `json:"status"`
	Title           string `json:"title"`
	UpdatedAt       string `json:"updated_at"`
}

// CommentJSON is the JSON output format for comments.
type CommentJSON struct {
	Author    string `json:"author"`
	Body      string `json:"body"`
	CreatedAt string `json:"created_at"`
	ID        string `json:"id"`
}

// ListDepJSON is the dependency format used in list command output.
// Different from EnrichedDepJSON - uses depends_on_id/issue_id instead of full issue data.
type ListDepJSON struct {
	CreatedAt   string `json:"created_at"`
	CreatedBy   string `json:"created_by,omitempty"`
	DependsOnID string `json:"depends_on_id"`
	IssueID     string `json:"issue_id"`
	Type        string `json:"type"`
}

// IssueListJSON is the JSON output format for list command.
type IssueListJSON struct {
	CreatedAt       string        `json:"created_at"`
	CreatedBy       string        `json:"created_by,omitempty"`
	Dependencies    []ListDepJSON `json:"dependencies,omitempty"`
	DependencyCount int           `json:"dependency_count"`
	DependentCount  int           `json:"dependent_count"`
	ID              string        `json:"id"`
	IssueType       string        `json:"issue_type"`
	Owner           string        `json:"owner,omitempty"`
	Priority        int           `json:"priority"`
	Status          string        `json:"status"`
	Title           string        `json:"title"`
	UpdatedAt       string        `json:"updated_at"`
}

// IssueSimpleJSON is a simpler JSON output format for ready/blocked commands (no counts).
type IssueSimpleJSON struct {
	CreatedAt string `json:"created_at"`
	CreatedBy string `json:"created_by,omitempty"`
	ID        string `json:"id"`
	IssueType string `json:"issue_type"`
	Owner     string `json:"owner,omitempty"`
	Priority  int    `json:"priority"`
	Status    string `json:"status"`
	Title     string `json:"title"`
	UpdatedAt string `json:"updated_at"`
}

// ToIssueSimpleJSON converts a storage.Issue to IssueSimpleJSON format.
func ToIssueSimpleJSON(issue *storage.Issue) IssueSimpleJSON {
	name, email := getGitUser()

	return IssueSimpleJSON{
		CreatedAt: formatTime(issue.CreatedAt),
		CreatedBy: name,
		ID:        issue.ID,
		IssueType: string(issue.Type),
		Owner:     email,
		Priority:  priorityToInt(issue.Priority),
		Status:    string(issue.Status),
		Title:     issue.Title,
		UpdatedAt: formatTime(issue.UpdatedAt),
	}
}

// priorityToInt converts storage.Priority to numeric value (0-4).
func priorityToInt(p storage.Priority) int {
	switch p {
	case storage.PriorityCritical:
		return 0
	case storage.PriorityHigh:
		return 1
	case storage.PriorityMedium:
		return 2
	case storage.PriorityLow:
		return 3
	case storage.PriorityBacklog:
		return 4
	default:
		return 2 // default to medium
	}
}

// formatTime formats a time.Time to RFC3339 with nanoseconds.
func formatTime(t time.Time) string {
	return t.Format("2006-01-02T15:04:05.999999999-07:00")
}

// getGitUser returns the git user name and email from git config.
// Returns empty strings if git config is not available.
func getGitUser() (name, email string) {
	if out, err := exec.Command("git", "config", "user.name").Output(); err == nil {
		name = strings.TrimSpace(string(out))
	}
	if out, err := exec.Command("git", "config", "user.email").Output(); err == nil {
		email = strings.TrimSpace(string(out))
	}
	return
}

// ToIssueJSON converts a storage.Issue to IssueJSON format.
// If enrichDeps is true, fetches full issue details for dependencies.
// If useCounts is true, uses dependency_count/dependent_count instead of full arrays.
func ToIssueJSON(ctx context.Context, store storage.Storage, issue *storage.Issue, enrichDeps bool, useCounts bool) IssueJSON {
	name, email := getGitUser()

	out := IssueJSON{
		Assignee:    issue.Assignee,
		CreatedAt:   formatTime(issue.CreatedAt),
		CreatedBy:   name,
		Description: issue.Description,
		ID:          issue.ID,
		IssueType:   string(issue.Type),
		Labels:      issue.Labels,
		Owner:       email,
		Parent:      issue.Parent,
		Priority:    priorityToInt(issue.Priority),
		Status:      string(issue.Status),
		Title:       issue.Title,
		UpdatedAt:   formatTime(issue.UpdatedAt),
	}

	if issue.ClosedAt != nil {
		out.ClosedAt = formatTime(*issue.ClosedAt)
	}

	// Children from dependents with parent-child type
	children := issue.Children()
	if len(children) > 0 {
		out.Children = children
	}

	// Comments
	if len(issue.Comments) > 0 {
		out.Comments = make([]CommentJSON, len(issue.Comments))
		for i, c := range issue.Comments {
			out.Comments[i] = CommentJSON{
				Author:    c.Author,
				Body:      c.Body,
				CreatedAt: formatTime(c.CreatedAt),
				ID:        c.ID,
			}
		}
	}

	if useCounts {
		// Use dependency/dependent counts
		depCount := len(issue.Dependencies)
		out.DependencyCount = &depCount
		dentCount := len(issue.Dependents)
		out.DependentCount = &dentCount
	} else if enrichDeps && store != nil {
		// Enrich dependencies with full issue details
		if len(issue.Dependencies) > 0 {
			out.Dependencies = enrichDependencies(ctx, store, issue.Dependencies)
		}
		if len(issue.Dependents) > 0 {
			out.Dependents = enrichDependencies(ctx, store, issue.Dependents)
		}
	}

	return out
}

// ToIssueListJSON converts a storage.Issue to IssueListJSON format for list command.
func ToIssueListJSON(issue *storage.Issue) IssueListJSON {
	name, email := getGitUser()

	// Convert dependencies to list format
	var deps []ListDepJSON
	if len(issue.Dependencies) > 0 {
		deps = make([]ListDepJSON, len(issue.Dependencies))
		for i, dep := range issue.Dependencies {
			deps[i] = ListDepJSON{
				CreatedAt:   formatTime(issue.CreatedAt), // Use issue's created_at as proxy
				CreatedBy:   name,
				DependsOnID: dep.ID,
				IssueID:     issue.ID,
				Type:        string(dep.Type),
			}
		}
	}

	return IssueListJSON{
		CreatedAt:       formatTime(issue.CreatedAt),
		CreatedBy:       name,
		Dependencies:    deps,
		DependencyCount: len(issue.Dependencies),
		DependentCount:  len(issue.Dependents),
		ID:              issue.ID,
		IssueType:       string(issue.Type),
		Owner:           email,
		Priority:        priorityToInt(issue.Priority),
		Status:          string(issue.Status),
		Title:           issue.Title,
		UpdatedAt:       formatTime(issue.UpdatedAt),
	}
}

// enrichDependencies fetches full issue details for each dependency.
func enrichDependencies(ctx context.Context, store storage.Storage, deps []storage.Dependency) []EnrichedDepJSON {
	name, email := getGitUser()
	result := make([]EnrichedDepJSON, 0, len(deps))

	for _, dep := range deps {
		issue, err := store.Get(ctx, dep.ID)
		if err != nil {
			// Include minimal info if we can't load the issue
			result = append(result, EnrichedDepJSON{
				ID:             dep.ID,
				DependencyType: string(dep.Type),
			})
			continue
		}

		result = append(result, EnrichedDepJSON{
			CreatedAt:      formatTime(issue.CreatedAt),
			CreatedBy:      name,
			DependencyType: string(dep.Type),
			ID:             issue.ID,
			IssueType:      string(issue.Type),
			Owner:          email,
			Priority:       priorityToInt(issue.Priority),
			Status:         string(issue.Status),
			Title:          issue.Title,
			UpdatedAt:      formatTime(issue.UpdatedAt),
		})
	}

	return result
}
