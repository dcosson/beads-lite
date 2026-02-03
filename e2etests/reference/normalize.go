package reference

import (
	"encoding/json"
	"fmt"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
)

// Normalizer tracks ID mappings across a test case and normalizes
// JSON output for deterministic comparison.
type Normalizer struct {
	issueIDs    map[string]string // "bd-a1f3" -> "ISSUE_1"
	commentIDs  map[string]string // "c-ab12" -> "COMMENT_1"
	issueTitles map[string]string // issue ID -> title (for stable sorting)
	issueSeq    int
	commentSeq  int
	sandboxPath string // if set, replaced with SANDBOX_PATH in output
}

// NewNormalizer creates a new Normalizer with empty state.
func NewNormalizer() *Normalizer {
	return &Normalizer{
		issueIDs:    make(map[string]string),
		commentIDs:  make(map[string]string),
		issueTitles: make(map[string]string),
	}
}

// SetSandboxPath registers the sandbox directory path so it can be
// replaced with SANDBOX_PATH in normalized output. Resolves symlinks
// so it matches paths that have been through filepath.EvalSymlinks
// (e.g., /var -> /private/var on macOS).
func (n *Normalizer) SetSandboxPath(path string) {
	if resolved, err := filepath.EvalSymlinks(path); err == nil {
		n.sandboxPath = resolved
	} else {
		n.sandboxPath = path
	}
}

var (
	// Match beads-lite IDs (bd-XXXX with optional .N.N suffix), original beads IDs
	// (e2etests-XXX with optional .N.N suffix), or sandbox IDs
	// (beads-sandbox-XXXXXXXX-XXX-YYY with optional extra suffix and .N.N suffix).
	// The (\.\d+)* captures hierarchical child IDs like bd-a1f3.1 or e2etests-abc.1.2
	// The (-[0-9a-z]+)? captures the extra ID suffix the reference binary adds
	// in non-daemon mode (e.g., beads-sandbox-XXXXXXXX-abc-43c).
	issueIDPattern   = regexp.MustCompile(`(bd-(?:mol|wisp)-[0-9a-z]{3,8}(\.\d+)*|bd-[0-9a-z]{3,8}(\.\d+)*|bd-[0-9a-f]{4}(\.\d+)*|bd-[0-9a-z]{2}-[0-9a-z]{3}(\.\d+)*|e2etests-[0-9a-z]{3}(\.\d+)*|e2etests-[0-9a-z]{2}-[0-9a-z]{3}(\.\d+)*|ISSUE_[0-9A-Za-z]{2}-[0-9A-Za-z]{3}(\.\d+)*|beads-sandbox-[A-Za-z0-9]+-[0-9a-z]{3,8}(-[0-9a-z]+)?(\.\d+)*|beads-sandbox-[A-Za-z0-9]+-[0-9a-z]{2}-[0-9a-z]{3}(-[0-9a-z]+)?(\.\d+)*|beads-sandbox-[A-Za-z0-9]+(\.\d+)*)`)
	commentIDPattern = regexp.MustCompile(`c-[0-9a-f]{4}`)
	timestampPattern = regexp.MustCompile(`\d{4}-\d{2}-\d{2}T\d{2}:\d{2}:\d{2}(\.\d+)?([+-]\d{2}:\d{2}|Z)`)
)

// mapIssueID returns the stable placeholder for an issue ID,
// assigning a new one if this is the first time we've seen it.
func (n *Normalizer) mapIssueID(id string) string {
	if mapped, ok := n.issueIDs[id]; ok {
		return mapped
	}
	n.issueSeq++
	mapped := fmt.Sprintf("ISSUE_%d", n.issueSeq)
	n.issueIDs[id] = mapped
	return mapped
}

// mapCommentID returns the stable placeholder for a comment ID.
func (n *Normalizer) mapCommentID(id string) string {
	if mapped, ok := n.commentIDs[id]; ok {
		return mapped
	}
	n.commentSeq++
	mapped := fmt.Sprintf("COMMENT_%d", n.commentSeq)
	n.commentIDs[id] = mapped
	return mapped
}

// NormalizeJSON takes raw JSON bytes, normalizes IDs and timestamps,
// and returns pretty-printed JSON with sorted keys.
func (n *Normalizer) NormalizeJSON(input []byte) string {
	input = []byte(strings.TrimSpace(string(input)))
	if len(input) == 0 {
		return ""
	}

	// Try to parse as JSON
	var data interface{}
	if err := json.Unmarshal(input, &data); err != nil {
		// Not valid JSON, normalize as plain text
		return n.normalizeText(string(input))
	}

	// Walk and normalize the data
	normalized := n.walkAndNormalize(data)

	// Re-marshal with sorted keys and pretty-printing
	output, err := json.MarshalIndent(normalized, "", "  ")
	if err != nil {
		return n.normalizeText(string(input))
	}

	return string(output)
}

// NormalizeJSONSorted is like NormalizeJSON but also sorts arrays of objects
// by their "title" field for deterministic ordering.
// IMPORTANT: Sorting happens BEFORE normalization so that IDs are encountered
// in a deterministic order regardless of the binary's output order.
func (n *Normalizer) NormalizeJSONSorted(input []byte) string {
	input = []byte(strings.TrimSpace(string(input)))
	if len(input) == 0 {
		return ""
	}

	var data interface{}
	if err := json.Unmarshal(input, &data); err != nil {
		return n.normalizeText(string(input))
	}

	// Sort top-level array by title BEFORE normalization so IDs are
	// encountered in a deterministic order
	if arr, ok := data.([]interface{}); ok {
		sort.SliceStable(arr, func(i, j int) bool {
			mi, oki := arr[i].(map[string]interface{})
			mj, okj := arr[j].(map[string]interface{})
			if !oki || !okj {
				return false
			}
			ti, _ := mi["title"].(string)
			tj, _ := mj["title"].(string)
			return ti < tj
		})
		data = arr
	}

	normalized := n.walkAndNormalize(data)

	output, err := json.MarshalIndent(normalized, "", "  ")
	if err != nil {
		return n.normalizeText(string(input))
	}

	return string(output)
}

// Keys whose values should be replaced with "FLOAT" for deterministic comparison
var floatKeys = map[string]bool{
	"average_lead_time_hours": true,
	"rate_per_hour":           true,
}

// walkAndNormalize recursively walks a JSON value and replaces IDs and timestamps.
// Map keys are visited in sorted order to ensure deterministic ID assignment.
func (n *Normalizer) walkAndNormalize(v interface{}) interface{} {
	switch val := v.(type) {
	case map[string]interface{}:
		if issues, ok := val["issues"].([]interface{}); ok {
			n.captureIssueTitles(issues)
		}

		// Sort keys to ensure deterministic first-seen ID ordering
		keys := make([]string, 0, len(val))
		for k := range val {
			keys = append(keys, k)
		}
		sort.Strings(keys)

		result := make(map[string]interface{})
		for _, k := range keys {
			if floatKeys[k] {
				result[k] = "FLOAT"
			} else {
				result[k] = n.walkAndNormalize(val[k])
			}
		}
		return result

	case []interface{}:
		if n.isIssueIDStringList(val) {
			mapped := make([]interface{}, len(val))
			for i, item := range val {
				mapped[i] = n.normalizeStringValue(item.(string))
			}
			sort.SliceStable(mapped, func(i, j int) bool {
				return mapped[i].(string) < mapped[j].(string)
			})
			return mapped
		}

		// Sort arrays of objects by stable keys for deterministic ordering.
		// Sort BEFORE normalization so IDs are encountered in deterministic order.
		sort.SliceStable(val, func(i, j int) bool {
			keyI, okI := n.objectSortKey(val[i])
			keyJ, okJ := n.objectSortKey(val[j])
			if !okI || !okJ {
				return false
			}
			return keyI < keyJ
		})
		result := make([]interface{}, len(val))
		for i, v := range val {
			result[i] = n.walkAndNormalize(v)
		}
		return result

	case string:
		return n.normalizeStringValue(val)

	default:
		return val
	}
}

// captureIssueTitles records issue ID -> title mappings for stable sorting.
func (n *Normalizer) captureIssueTitles(issues []interface{}) {
	for _, item := range issues {
		m, ok := item.(map[string]interface{})
		if !ok {
			continue
		}
		id, _ := m["id"].(string)
		title, _ := m["title"].(string)
		if id != "" && title != "" {
			n.issueTitles[id] = title
		}
	}
}

// objectSortKey returns a stable sort key for objects when available.
func (n *Normalizer) objectSortKey(v interface{}) (string, bool) {
	m, ok := v.(map[string]interface{})
	if !ok {
		return "", false
	}
	if title, ok := m["title"].(string); ok && title != "" {
		return "title:" + title, true
	}
	if issue, ok := m["issue"].(map[string]interface{}); ok {
		if title, ok := issue["title"].(string); ok && title != "" {
			return "issue.title:" + title, true
		}
	}
	issueID, _ := m["issue_id"].(string)
	dependsOnID, _ := m["depends_on_id"].(string)
	if issueID != "" || dependsOnID != "" {
		issueTitle := n.issueTitles[issueID]
		dependsTitle := n.issueTitles[dependsOnID]
		if issueTitle != "" || dependsTitle != "" {
			depType, _ := m["type"].(string)
			return fmt.Sprintf("dep:%s|%s|%s", issueTitle, dependsTitle, depType), true
		}
	}
	return "", false
}

func (n *Normalizer) isIssueIDStringList(items []interface{}) bool {
	if len(items) == 0 {
		return false
	}
	for _, item := range items {
		s, ok := item.(string)
		if !ok {
			return false
		}
		match := issueIDPattern.FindString(s)
		if match == "" || match != s {
			return false
		}
	}
	return true
}

// normalizeStringValue replaces IDs and timestamps in a string value.
func (n *Normalizer) normalizeStringValue(s string) string {
	// Replace sandbox path prefix before other normalization.
	if n.sandboxPath != "" && strings.Contains(s, n.sandboxPath) {
		s = strings.ReplaceAll(s, n.sandboxPath, "SANDBOX_PATH")
	}

	// Check if the entire string is a timestamp
	if timestampPattern.MatchString(s) {
		return "TIMESTAMP"
	}

	// Check if it's an issue ID
	if issueIDPattern.MatchString(s) {
		return issueIDPattern.ReplaceAllStringFunc(s, func(id string) string {
			return n.mapIssueID(id)
		})
	}
	if strings.HasPrefix(s, "ISSUE_") && strings.Contains(s, "-") {
		return n.mapIssueID(s)
	}

	// Check if it's a comment ID
	if commentIDPattern.MatchString(s) {
		return commentIDPattern.ReplaceAllStringFunc(s, func(id string) string {
			return n.mapCommentID(id)
		})
	}

	return s
}

// normalizeText normalizes plain text (non-JSON) output.
func (n *Normalizer) normalizeText(s string) string {
	if n.sandboxPath != "" {
		s = strings.ReplaceAll(s, n.sandboxPath, "SANDBOX_PATH")
	}
	s = issueIDPattern.ReplaceAllStringFunc(s, func(id string) string {
		return n.mapIssueID(id)
	})
	s = commentIDPattern.ReplaceAllStringFunc(s, func(id string) string {
		return n.mapCommentID(id)
	})
	s = timestampPattern.ReplaceAllString(s, "TIMESTAMP")
	return s
}

// ExtractID extracts the issue ID from a JSON create response.
func ExtractID(jsonOutput []byte) string {
	var result struct {
		ID string `json:"id"`
	}
	if err := json.Unmarshal(jsonOutput, &result); err != nil {
		return ""
	}
	return result.ID
}

// ExtractCommentID extracts the comment ID from a JSON comment add response.
func ExtractCommentID(jsonOutput []byte) string {
	var result struct {
		Comment struct {
			ID string `json:"id"`
		} `json:"comment"`
	}
	if err := json.Unmarshal(jsonOutput, &result); err != nil {
		return ""
	}
	return result.Comment.ID
}
