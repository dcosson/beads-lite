package e2etests

import "encoding/json"

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
