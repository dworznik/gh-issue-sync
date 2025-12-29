package localid

import (
	"crypto/rand"
	"encoding/hex"
)

const (
	// IDLength is the number of random bytes (8 chars = 4 bytes hex encoded)
	IDLength = 4
)

// Generate creates a new random 8-character alphanumeric local ID.
// The ID is prefixed with "T" when used as an issue number.
func Generate() (string, error) {
	bytes := make([]byte, IDLength)
	if _, err := rand.Read(bytes); err != nil {
		return "", err
	}
	return hex.EncodeToString(bytes), nil
}
