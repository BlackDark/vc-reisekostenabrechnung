package auth

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
)

// NewAPIToken returns a fresh 256-bit token and the hex SHA-256 hash to store.
// The plaintext is shown to the user once; only the hash is persisted (hashed at
// rest). A token is high-entropy, so SHA-256 is the right store-side hash here
// and keeps bearer lookup fast.
func NewAPIToken() (plain, hashHex string, err error) {
	raw := make([]byte, 32)
	if _, err = rand.Read(raw); err != nil {
		return "", "", err
	}
	plain = base64.RawURLEncoding.EncodeToString(raw)
	return plain, HashBearer(plain), nil
}

// HashBearer hashes a bearer secret for lookup against the stored hash. API
// tokens and session tokens both carry 256 bits of entropy, so SHA-256 is enough
// to keep a reader of the database from using what it finds.
func HashBearer(secret string) string {
	sum := sha256.Sum256([]byte(secret))
	return hex.EncodeToString(sum[:])
}
