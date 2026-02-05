// Package idgen implements ID generation and parsing for beads IDs.
//
// beads IDs have a prefix-suffix format: "bd-a3f8", "ext-42", "bd-mol-xyz".
// Hierarchical child IDs use dot notation: "bd-a3f8.1", "bd-a3f8.1.2".
package idgen

import (
	"crypto/rand"
	"fmt"
	"math"
	"math/big"
	"strconv"
	"strings"
	"unicode"
)

const (
	// MinLength is the minimum number of base36 characters in a generated ID.
	MinLength = 3
	// MaxLength is the maximum number of base36 characters in a generated ID.
	MaxLength = 8
	// MaxCollisionProbability is the threshold above which the adaptive length
	// is increased. Based on the birthday paradox formula.
	MaxCollisionProbability = 0.25
)

// RandomID generates a random ID with the given prefix and length.
// It uses crypto/rand to generate length random base36 characters.
// Returns an error if length is outside [MinLength, MaxLength].
func RandomID(prefix string, length int) (string, error) {
	if length < MinLength || length > MaxLength {
		return "", fmt.Errorf("idgen: length %d out of range [%d, %d]", length, MinLength, MaxLength)
	}

	// Generate a random number in [0, 36^length).
	mod := new(big.Int).Exp(big.NewInt(36), big.NewInt(int64(length)), nil)
	n, err := rand.Int(rand.Reader, mod)
	if err != nil {
		return "", fmt.Errorf("idgen: crypto/rand: %w", err)
	}

	// Encode as base36, left-pad with zeros to the target length.
	encoded := n.Text(36)
	for len(encoded) < length {
		encoded = "0" + encoded
	}

	return prefix + encoded, nil
}

// AdaptiveLength calculates the minimum ID length needed for the given
// number of existing issues, using the birthday paradox collision formula:
//
//	P(collision) ≈ 1 - e^(-n²/2N)
//
// where n = existingCount and N = 36^length. Starting from MinLength,
// the length is incremented until the probability falls below
// MaxCollisionProbability, up to MaxLength.
func AdaptiveLength(existingCount int) int {
	for length := MinLength; length <= MaxLength; length++ {
		namespace := math.Pow(36, float64(length))
		n := float64(existingCount)
		probability := 1 - math.Exp(-(n*n)/(2*namespace))
		if probability < MaxCollisionProbability {
			return length
		}
	}
	return MaxLength
}

// --- Prefix handling ---

// BuildPrefix composes a full ID prefix from a base prefix and an optional
// addition. It normalises dashes so the result always ends with exactly one
// dash and never contains double-dashes.
//
//	BuildPrefix("bd-", "")    → "bd-"
//	BuildPrefix("bd-", "mol") → "bd-mol-"
//	BuildPrefix("bd",  "mol") → "bd-mol-"
//	BuildPrefix("bd-", "-mol-") → "bd-mol-"
func BuildPrefix(base, addition string) string {
	base = strings.TrimRight(base, "-")
	addition = strings.Trim(addition, "-")
	if addition == "" {
		return base + "-"
	}
	return base + "-" + addition + "-"
}

// --- Hierarchical ID parsing ---

// IsHierarchicalID reports whether id is a hierarchical child ID.
// An ID is hierarchical if it contains a dot and the suffix after the last
// dot is purely numeric (e.g. "bd-a3f8.1" is hierarchical, but
// "my.project-abc" is not).
func IsHierarchicalID(id string) bool {
	dot := strings.LastIndex(id, ".")
	if dot < 0 || dot == len(id)-1 {
		return false
	}
	suffix := id[dot+1:]
	for _, r := range suffix {
		if !unicode.IsDigit(r) {
			return false
		}
	}
	return true
}

// HierarchyDepth returns the nesting depth of an ID by counting dots.
// A root ID like "bd-a3f8" has depth 0; "bd-a3f8.1" has depth 1, etc.
func HierarchyDepth(id string) int {
	return strings.Count(id, ".")
}

// ChildID returns the composite child ID given a parent ID and child number.
func ChildID(parentID string, childNum int) string {
	return fmt.Sprintf("%s.%d", parentID, childNum)
}

// ParseHierarchicalID splits a hierarchical ID into its immediate parent and
// child number. For example, "bd-a3f8.2" returns ("bd-a3f8", 2, true).
// Returns ("", 0, false) if the ID is not hierarchical.
func ParseHierarchicalID(id string) (parentID string, childNum int, ok bool) {
	if !IsHierarchicalID(id) {
		return "", 0, false
	}
	dot := strings.LastIndex(id, ".")
	parentID = id[:dot]
	childNum, _ = strconv.Atoi(id[dot+1:])
	return parentID, childNum, true
}

// RootParentID returns the root parent portion of a (possibly hierarchical) ID.
// For hierarchical IDs this is everything before the first dot
// (e.g. "bd-a3f8.1.2" → "bd-a3f8"). For non-hierarchical IDs the full ID
// is returned unchanged.
func RootParentID(id string) string {
	dot := strings.Index(id, ".")
	if dot < 0 {
		return id
	}
	return id[:dot]
}

// --- Hierarchy depth validation ---

// DefaultMaxHierarchyDepth is the default maximum number of dot-notation levels
// allowed in hierarchical child IDs (e.g., bd-a3f8.1.2.3 = depth 3).
const DefaultMaxHierarchyDepth = 3

// ErrMaxDepthExceeded is returned when an operation would exceed the maximum
// hierarchy depth for child IDs.
var ErrMaxDepthExceeded = fmt.Errorf("maximum hierarchy depth exceeded")

// CheckHierarchyDepth verifies that parentID is not already at the maximum
// hierarchy depth. If adding a child to parentID would exceed maxDepth,
// it returns ErrMaxDepthExceeded with a descriptive message.
// For example, with maxDepth=3, a parent "bd-x.1.2.3" (depth 3) is rejected
// because a child would be at depth 4.
func CheckHierarchyDepth(parentID string, maxDepth int) error {
	depth := HierarchyDepth(parentID)
	if depth >= maxDepth {
		return fmt.Errorf("cannot add child to %s (depth %d): maximum hierarchy depth is %d: %w",
			parentID, depth, maxDepth, ErrMaxDepthExceeded)
	}
	return nil
}
