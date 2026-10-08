package auth

import (
	"crypto/rand"
	"crypto/subtle"
	"encoding/base64"
	"errors"
	"fmt"
	"strings"

	"golang.org/x/crypto/argon2"
)

// Params are the Argon2id settings stored in the PHC string.
type Params struct {
	Memory  uint32
	Time    uint32
	Threads uint8
	KeyLen  uint32
}

// Passwords hashes and verifies Argon2id PHC strings.
type Passwords struct {
	Params Params
	dummy  string
}

// NewPasswords prepares a hasher and a dummy hash so unknown users take the same time.
func NewPasswords(p Params) (*Passwords, error) {
	if p.KeyLen == 0 {
		p.KeyLen = 32
	}
	h, err := Hash("dummy-password-not-used", p)
	if err != nil {
		return nil, err
	}
	return &Passwords{Params: p, dummy: h}, nil
}

// Hash returns a PHC-encoded Argon2id hash.
func Hash(password string, p Params) (string, error) {
	salt := make([]byte, 16)
	if _, err := rand.Read(salt); err != nil {
		return "", err
	}
	if p.KeyLen == 0 {
		p.KeyLen = 32
	}
	sum := argon2.IDKey([]byte(password), salt, p.Time, p.Memory, p.Threads, p.KeyLen)
	return fmt.Sprintf("$argon2id$v=%d$m=%d,t=%d,p=%d$%s$%s",
		argon2.Version, p.Memory, p.Time, p.Threads,
		base64.RawStdEncoding.EncodeToString(salt),
		base64.RawStdEncoding.EncodeToString(sum),
	), nil
}

// Verify checks password against a PHC hash. NeedsRehash is true when the stored parameters differ.
func (p *Passwords) Verify(encoded, password string) (ok, needsRehash bool) {
	mem, tim, threads, salt, sum, err := decodePHC(encoded)
	if err != nil {
		_, _ = verifyRaw(p.dummy, password)
		return false, false
	}
	got := argon2.IDKey([]byte(password), salt, tim, mem, threads, uint32(len(sum)))
	if subtle.ConstantTimeCompare(got, sum) != 1 {
		return false, false
	}
	return true, mem != p.Params.Memory || tim != p.Params.Time || threads != p.Params.Threads
}

// VerifyDummy burns the same Argon2 work as a real check.
func (p *Passwords) VerifyDummy(password string) {
	_, _ = verifyRaw(p.dummy, password)
}

func verifyRaw(encoded, password string) (bool, error) {
	mem, tim, threads, salt, sum, err := decodePHC(encoded)
	if err != nil {
		return false, err
	}
	got := argon2.IDKey([]byte(password), salt, tim, mem, threads, uint32(len(sum)))
	return subtle.ConstantTimeCompare(got, sum) == 1, nil
}

func decodePHC(encoded string) (mem, tim uint32, threads uint8, salt, sum []byte, err error) {
	parts := strings.Split(encoded, "$")
	if len(parts) != 6 || parts[1] != "argon2id" {
		return 0, 0, 0, nil, nil, errors.New("not an argon2id phc string")
	}
	var version int
	if _, err = fmt.Sscanf(parts[2], "v=%d", &version); err != nil {
		return 0, 0, 0, nil, nil, err
	}
	var threadsI int
	if _, err = fmt.Sscanf(parts[3], "m=%d,t=%d,p=%d", &mem, &tim, &threadsI); err != nil {
		return 0, 0, 0, nil, nil, err
	}
	threads = uint8(threadsI)
	salt, err = base64.RawStdEncoding.DecodeString(parts[4])
	if err != nil {
		return 0, 0, 0, nil, nil, err
	}
	sum, err = base64.RawStdEncoding.DecodeString(parts[5])
	if err != nil {
		return 0, 0, 0, nil, nil, err
	}
	return mem, tim, threads, salt, sum, nil
}

// PasswordOK reports whether the password meets the length rule.
func PasswordOK(password string) bool {
	n := len([]rune(password))
	return n >= 12 && n <= 256
}
