// Package idgen implements random ID generation for the beads ID format.
package idgen

import (
	"crypto/rand"
	"fmt"
	"math"
	"math/big"
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
