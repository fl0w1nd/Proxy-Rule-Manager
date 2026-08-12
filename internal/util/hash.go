package util

import (
	"crypto/sha256"
	"encoding/hex"
)

// SHA256Hex returns the lower-case hex sha-256 digest of content.
func SHA256Hex(content string) string {
	sum := sha256.Sum256([]byte(content))
	return hex.EncodeToString(sum[:])
}
