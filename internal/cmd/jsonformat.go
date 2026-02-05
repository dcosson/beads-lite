package cmd

import (
	"context"
	"time"

	"beads-lite/internal/issuestorage"
)

// IssueJSON is the JSON output format matching original beads.
// Used for create, show, and list commands.
type IssueJSON struct {
	Assignee        string            `json:"assignee,omitempty"`
	Comments        []CommentJSON     `json:"comments,omitempty"`
	CreatedAt       string            `json:"created_at"`
	CreatedBy       string            `json:"created_by,omitempty"`
	Dependencies    []EnrichedDepJSON `json:"dependencies,omitempty"`
	DependencyCount *int              `json:"dependency_count,omitempty"`
	Dependents      []EnrichedDepJSON `json:"dependents,omitempty"`
	DependentCount  *int              `json:"dependent_count,omitempty"`
	Description     string            `json:"description,omitempty"`
	ID              string            `json:"id"`
	IssueType       string            `json:"issue_type"`
	Labels          []string          `json:"labels,omitempty"`
	Owner           string            `json:"owner,omitempty"`
	Parent          string            `json:"parent,omitempty"`
	Priority        int               `json:"priority"`
	Status          string            `json:"status"`
	Title           string            `json:"title"`
	UpdatedAt       string            `json:"updated_at"`
	CloseReason     string            `json:"close_reason,omitempty"`
	ClosedAt        string            `json:"closed_at,omitempty"`
	AwaitType       string            `json:"await_type,omitempty"`
	AwaitID         string            `json:"await_id,omitempty"`
	TimeoutNS       int64             `json:"timeout_ns,omitempty"`
	Waiters         []string          `json:"waiters,omitempty"`
}

// EnrichedDepJSON is a dependency with full issue details for JSON output.
type EnrichedDepJSON struct {
	CreatedAt      string `json:"created_at"`
	CreatedBy      string `json:"created_by,omitempty"`
	DependencyType string `json:"dependency_type"`
	Description    string `json:"description,omitempty"`
	Ephemeral      *bool  `json:"ephemeral,omitempty"`
	ID             string `json:"id"`
	IssueType      string `json:"issue_type"`
	Owner          string `json:"owner,omitempty"`
	Priority       int    `json:"priority"`
	Status         string `json:"status"`
	Title          string `json:"title"`
	UpdatedAt      string `json:"updated_at"`
}

// CommentJSON is the JSON output format for comments.
type CommentJSON struct {
	Author    string `json:"author"`
	CreatedAt string `json:"created_at"`
	ID        int    `json:"id"`
	IssueID   string `json:"issue_id"`
	Text      string `json:"text"`
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
	Assignee        string        `json:"assignee,omitempty"`
	CloseReason     string        `json:"close_reason,omitempty"`
	ClosedAt        string        `json:"closed_at,omitempty"`
	CreatedAt       string        `json:"created_at"`
	CreatedBy       string        `json:"created_by,omitempty"`
	DeleteReason    string        `json:"delete_reason,omitempty"`
	DeletedAt       string        `json:"deleted_at,omitempty"`
	DeletedBy       string        `json:"deleted_by,omitempty"`
	Dependencies    []ListDepJSON `json:"dependencies,omitempty"`
	DependencyCount int           `json:"dependency_count"`
	DependentCount  int           `json:"dependent_count"`
	Description     string        `json:"description,omitempty"`
	ID              string        `json:"id"`
	IssueType       string        `json:"issue_type"`
	Labels          []string      `json:"labels,omitempty"`
	OriginalType    string        `json:"original_type,omitempty"`
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

// ToIssueSimpleJSON converts a issuestorage.Issue to IssueSimpleJSON format.
func ToIssueSimpleJSON(issue *issuestorage.Issue) IssueSimpleJSON {
	return IssueSimpleJSON{
		CreatedAt: formatTime(issue.CreatedAt),
		CreatedBy: issue.CreatedBy,
		ID:        issue.ID,
		IssueType: string(issue.Type),
		Owner:     issue.Owner,
		Priority:  priorityToInt(issue.Priority),
		Status:    string(issue.Status),
		Title:     issue.Title,
		UpdatedAt: formatTime(issue.UpdatedAt),
	}
}

// priorityToInt converts issuestorage.Priority to numeric value (0-4).
func priorityToInt(p issuestorage.Priority) int {
	return int(p)
}

// formatTime formats a time.Time to RFC3339 with nanoseconds.
func formatTime(t time.Time) string {
	return t.Format("2006-01-02T15:04:05.999999999-07:00")
}

// ToIssueJSON converts a issuestorage.Issue to IssueJSON format.
// If enrichDeps is true, fetches full issue details for dependencies.
// If useCounts is true, uses dependency_count/dependent_count instead of full arrays.
func ToIssueJSON(ctx context.Context, store issuestorage.IssueGetter, issue *issuestorage.Issue, enrichDeps bool, useCounts bool) IssueJSON {
	out := IssueJSON{
		Assignee:    issue.Assignee,
		CreatedAt:   formatTime(issue.CreatedAt),
		CreatedBy:   issue.CreatedBy,
		Description: issue.Description,
		ID:          issue.ID,
		IssueType:   string(issue.Type),
		Labels:      issue.Labels,
		Owner:       issue.Owner,
		Parent:      issue.Parent,
		Priority:    priorityToInt(issue.Priority),
		Status:      string(issue.Status),
		Title:       issue.Title,
		UpdatedAt:   formatTime(issue.UpdatedAt),
	}

	if issue.CloseReason != "" {
		out.CloseReason = issue.CloseReason
	}
	if issue.ClosedAt != nil {
		out.ClosedAt = formatTime(*issue.ClosedAt)
	}

	// Gate fields
	out.AwaitType = issue.AwaitType
	out.AwaitID = issue.AwaitID
	out.TimeoutNS = issue.TimeoutNS
	out.Waiters = issue.Waiters

	// Comments
	if len(issue.Comments) > 0 {
		out.Comments = make([]CommentJSON, len(issue.Comments))
		for i, c := range issue.Comments {
			out.Comments[i] = CommentJSON{
				Author:    c.Author,
				CreatedAt: formatTime(c.CreatedAt),
				ID:        c.ID,
				IssueID:   issue.ID,
				Text:      c.Text,
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

// ToIssueListJSON converts a issuestorage.Issue to IssueListJSON format for list command.
func ToIssueListJSON(issue *issuestorage.Issue) IssueListJSON {
	// Convert dependencies to list format
	var deps []ListDepJSON
	if len(issue.Dependencies) > 0 {
		deps = make([]ListDepJSON, len(issue.Dependencies))
		for i, dep := range issue.Dependencies {
			deps[i] = ListDepJSON{
				CreatedAt:   formatTime(issue.CreatedAt), // Use issue's created_at as proxy
				CreatedBy:   issue.CreatedBy,
				DependsOnID: dep.ID,
				IssueID:     issue.ID,
				Type:        string(dep.Type),
			}
		}
	}

	out := IssueListJSON{
		Assignee:        issue.Assignee,
		CreatedAt:       formatTime(issue.CreatedAt),
		CreatedBy:       issue.CreatedBy,
		Dependencies:    deps,
		DependencyCount: len(issue.Dependencies),
		DependentCount:  len(issue.Dependents),
		Description:     issue.Description,
		ID:              issue.ID,
		IssueType:       string(issue.Type),
		Labels:          issue.Labels,
		Owner:           issue.Owner,
		Priority:        priorityToInt(issue.Priority),
		Status:          string(issue.Status),
		Title:           issue.Title,
		UpdatedAt:       formatTime(issue.UpdatedAt),
	}

	if issue.CloseReason != "" {
		out.CloseReason = issue.CloseReason
	}
	if issue.ClosedAt != nil {
		out.ClosedAt = formatTime(*issue.ClosedAt)
	}
	if issue.DeleteReason != "" {
		out.DeleteReason = issue.DeleteReason
	}
	if issue.DeletedAt != nil {
		out.DeletedAt = formatTime(*issue.DeletedAt)
	}
	if issue.DeletedBy != "" {
		out.DeletedBy = issue.DeletedBy
	}
	if issue.OriginalType != "" {
		out.OriginalType = string(issue.OriginalType)
	}

	return out
}

// CloseWithContinueJSON is the JSON output format for "close --continue".
type CloseWithContinueJSON struct {
	Closed   []IssueJSON        `json:"closed"`
	Continue *CloseContinueJSON `json:"continue"`
}

// CloseContinueJSON holds the continue/advance info for close --continue.
type CloseContinueJSON struct {
	AutoAdvanced     bool          `json:"auto_advanced"`
	ClosedStep       *MolIssueJSON `json:"closed_step"`
	MoleculeComplete bool          `json:"molecule_complete"`
	MoleculeID       string        `json:"molecule_id"`
	NextStep         *MolIssueJSON `json:"next_step"`
}

// MolIssueJSON is the JSON format for issues within molecule commands
// (mol current, mol show). Includes optional fields that appear only when set.
type MolIssueJSON struct {
	Assignee    string `json:"assignee,omitempty"`
	CloseReason string `json:"close_reason,omitempty"`
	ClosedAt    string `json:"closed_at,omitempty"`
	CreatedAt   string `json:"created_at"`
	Description string `json:"description,omitempty"`
	ID          string `json:"id"`
	IssueType   string `json:"issue_type"`
	Priority    int    `json:"priority"`
	Status      string `json:"status"`
	Title       string `json:"title"`
	UpdatedAt   string `json:"updated_at"`
}

// ToMolIssueJSON converts a issuestorage.Issue to MolIssueJSON format.
func ToMolIssueJSON(issue *issuestorage.Issue) MolIssueJSON {
	out := MolIssueJSON{
		Assignee:    issue.Assignee,
		CreatedAt:   formatTime(issue.CreatedAt),
		Description: issue.Description,
		ID:          issue.ID,
		IssueType:   string(issue.Type),
		Priority:    priorityToInt(issue.Priority),
		Status:      string(issue.Status),
		Title:       issue.Title,
		UpdatedAt:   formatTime(issue.UpdatedAt),
	}
	if issue.CloseReason != "" {
		out.CloseReason = issue.CloseReason
	}
	if issue.ClosedAt != nil {
		out.ClosedAt = formatTime(*issue.ClosedAt)
	}
	return out
}

// MolCurrentJSON is the JSON output format for "mol current".
type MolCurrentJSON struct {
	Completed     int                  `json:"completed"`
	MoleculeID    string               `json:"molecule_id"`
	MoleculeTitle string               `json:"molecule_title"`
	NextStep      *MolIssueJSON        `json:"next_step"`
	Steps         []MolCurrentStepJSON `json:"steps"`
	Total         int                  `json:"total"`
}

// MolCurrentStepJSON is a single step entry in mol current output.
type MolCurrentStepJSON struct {
	IsCurrent bool         `json:"is_current"`
	Issue     MolIssueJSON `json:"issue"`
	Status    string       `json:"status"`
}

// MolShowJSON is the JSON output format for "mol show".
type MolShowJSON struct {
	BondedFrom   interface{}    `json:"bonded_from"`
	Dependencies []ListDepJSON  `json:"dependencies"`
	IsCompound   bool           `json:"is_compound"`
	Issues       []MolIssueJSON `json:"issues"`
	Root         MolIssueJSON   `json:"root"`
	Variables    interface{}    `json:"variables"`
}

// MolProgressJSON is the JSON output format for "mol progress".
type MolProgressJSON struct {
	Completed     int     `json:"completed"`
	CurrentStepID string  `json:"current_step_id"`
	ETAHours      float64 `json:"eta_hours"`
	InProgress    int     `json:"in_progress"`
	MoleculeID    string  `json:"molecule_id"`
	MoleculeTitle string  `json:"molecule_title"`
	Percent       float64 `json:"percent"`
	RatePerHour   float64 `json:"rate_per_hour"`
	Total         int     `json:"total"`
}

// enrichDependencies fetches full issue details for each dependency.
func enrichDependencies(ctx context.Context, store issuestorage.IssueGetter, deps []issuestorage.Dependency) []EnrichedDepJSON {
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

		enriched := EnrichedDepJSON{
			CreatedAt:      formatTime(issue.CreatedAt),
			CreatedBy:      issue.CreatedBy,
			DependencyType: string(dep.Type),
			Description:    issue.Description,
			ID:             issue.ID,
			IssueType:      string(issue.Type),
			Owner:          issue.Owner,
			Priority:       priorityToInt(issue.Priority),
			Status:         string(issue.Status),
			Title:          issue.Title,
			UpdatedAt:      formatTime(issue.UpdatedAt),
		}
		if issue.Ephemeral {
			eph := true
			enriched.Ephemeral = &eph
		}
		result = append(result, enriched)
	}

	return result
}
