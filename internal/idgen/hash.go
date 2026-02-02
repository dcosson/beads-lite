// Package idgen implements deterministic, content-based ID generation
// compatible with the beads ID format.
package idgen

import (
	"crypto/sha256"
	"fmt"
	"math"
	"math/big"
	"time"
)

const (
	// MinLength is the minimum number of base36 characters in a generated ID.
	MinLength = 3
	// MaxLength is the maximum number of base36 characters in a generated ID.
	MaxLength = 8
	// MaxCollisionProbability is the threshold above which the adaptive length
	// is increased. Based on the birthday paradox formula.
	MaxCollisionProbability = 0.25
	// MaxNonce is the maximum nonce value tried before escalating ID length.
	MaxNonce = 9
)

// HashID generates a deterministic ID from the given content fields.
// The algorithm:
//  1. Build content string from title, description, creator, timestamp, nonce
//  2. Compute SHA256 of the content string
//  3. Take the first N bytes needed for the requested length
//  4. Interpret as a big-endian integer, mod by 36^length
//  5. Encode as base36, zero-padded to exactly `length` characters
func HashID(prefix, title, description, creator string, timestamp time.Time, nonce, length int) string {
	content := fmt.Sprintf("%s|%s|%s|%d|%d", title, description, creator, timestamp.UnixNano(), nonce)
	hash := sha256.Sum256([]byte(content))

	// Number of bytes needed: ceil(length * 5 / 8).
	// Each base36 digit encodes ~5.17 bits; we approximate with 5.
	numBytes := (length*5 + 7) / 8

	n := new(big.Int).SetBytes(hash[:numBytes])

	// Mod by 36^length to produce exactly `length` base36 digits.
	mod := new(big.Int).Exp(big.NewInt(36), big.NewInt(int64(length)), nil)
	n.Mod(n, mod)

	// Encode as base36, left-pad with zeros to the target length.
	encoded := n.Text(36)
	for len(encoded) < length {
		encoded = "0" + encoded
	}

	return prefix + encoded
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
		probability := 1 - math.Exp(-(n * n) / (2 * namespace))
		if probability < MaxCollisionProbability {
			return length
		}
	}
	return MaxLength
}
