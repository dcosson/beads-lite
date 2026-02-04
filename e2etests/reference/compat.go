package reference

import "beads-lite/e2etests"

// Type aliases so existing case files and helpers continue to compile
// without import changes.
type Runner = e2etests.Runner
type RunResult = e2etests.RunResult

// ExtractID forwards to the parent e2etests package.
func ExtractID(j []byte) string { return e2etests.ExtractID(j) }

// ExtractCommentID forwards to the parent e2etests package.
func ExtractCommentID(j []byte) string { return e2etests.ExtractCommentID(j) }
