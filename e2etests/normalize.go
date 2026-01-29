package e2etests

import (
	"encoding/json"
	"fmt"
	"regexp"
	"sort"
	"strings"
)

// Normalizer tracks ID mappings across a test case and normalizes
// JSON output for deterministic comparison.
type Normalizer struct {
	issueIDs   map[string]string // "bd-a1f3" -> "ISSUE_1"
	commentIDs map[string]string // "c-ab12" -> "COMMENT_1"
	issueSeq   int
	commentSeq int
}

// NewNormalizer creates a new Normalizer with empty state.
func NewNormalizer() *Normalizer {
	return &Normalizer{
		issueIDs:   make(map[string]string),
		commentIDs: make(map[string]string),
	}
}

var (
	// Match beads-lite IDs (bd-XXXX) or original beads IDs (e2etests-XXX)
	issueIDPattern   = regexp.MustCompile(`(bd-[0-9a-f]{4}|e2etests-[0-9a-z]{3})`)
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
func (n *Normalizer) NormalizeJSONSorted(input []byte) string {
	input = []byte(strings.TrimSpace(string(input)))
	if len(input) == 0 {
		return ""
	}

	var data interface{}
	if err := json.Unmarshal(input, &data); err != nil {
		return n.normalizeText(string(input))
	}

	normalized := n.walkAndNormalize(data)

	// Sort top-level array by title if it's an array of objects
	if arr, ok := normalized.([]interface{}); ok {
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
		normalized = arr
	}

	output, err := json.MarshalIndent(normalized, "", "  ")
	if err != nil {
		return n.normalizeText(string(input))
	}

	return string(output)
}

// walkAndNormalize recursively walks a JSON value and replaces IDs and timestamps.
// Map keys are visited in sorted order to ensure deterministic ID assignment.
func (n *Normalizer) walkAndNormalize(v interface{}) interface{} {
	switch val := v.(type) {
	case map[string]interface{}:
		// Sort keys to ensure deterministic first-seen ID ordering
		keys := make([]string, 0, len(val))
		for k := range val {
			keys = append(keys, k)
		}
		sort.Strings(keys)

		result := make(map[string]interface{})
		for _, k := range keys {
			result[k] = n.walkAndNormalize(val[k])
		}
		return result

	case []interface{}:
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

// normalizeStringValue replaces IDs and timestamps in a string value.
func (n *Normalizer) normalizeStringValue(s string) string {
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
