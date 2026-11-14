// Package auth implements the single admin access: password hashing and
// storage, in-memory sessions and the login throttle. It has no HTTP code.
package auth

import (
	"crypto/rand"
	"crypto/subtle"
	"encoding/base64"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"unicode/utf8"

	"golang.org/x/crypto/argon2"
)

const (
	// MinPasswordLength and MaxPasswordLength are counted in Unicode code
	// points. The upper bound keeps a hash request from becoming a DoS vector.
	MinPasswordLength = 10
	MaxPasswordLength = 128

	// Argon2id parameters (OWASP minimum profile: 19 MiB, 2 iterations, 1 lane).
	argonMemoryKiB = 19456
	argonTime      = 2
	argonThreads   = 1
	argonSaltLen   = 16
	argonKeyLen    = 32

	// Sanity limits for parameters read from a stored hash, so a damaged file
	// cannot make Verify allocate gigabytes.
	maxArgonMemoryKiB = 1 << 20
	maxArgonTime      = 16
	maxArgonThreads   = 16
)

// Password validation errors; their text is shown to the user.
var (
	ErrPasswordTooShort = errors.New("Das Passwort muss mindestens 10 Zeichen lang sein.")
	ErrPasswordTooLong  = errors.New("Das Passwort darf höchstens 128 Zeichen lang sein.")
)

var errMalformedHash = errors.New("malformed password hash")

// ValidatePassword checks the length rules. The password is not trimmed:
// spaces are legitimate characters of a passphrase.
func ValidatePassword(pw string) error {
	n := utf8.RuneCountInString(pw)
	switch {
	case n < MinPasswordLength:
		return ErrPasswordTooShort
	case n > MaxPasswordLength:
		return ErrPasswordTooLong
	}
	return nil
}

// HashPassword returns the Argon2id hash of pw as a PHC string. The
// parameters are part of the string, so they can be raised later while old
// hashes keep verifying.
func HashPassword(pw string) (string, error) {
	salt := make([]byte, argonSaltLen)
	if _, err := rand.Read(salt); err != nil {
		return "", fmt.Errorf("generate salt: %w", err)
	}
	key := argon2.IDKey([]byte(pw), salt, argonTime, argonMemoryKiB, argonThreads, argonKeyLen)
	enc := base64.RawStdEncoding
	return fmt.Sprintf("$argon2id$v=%d$m=%d,t=%d,p=%d$%s$%s",
		argon2.Version, argonMemoryKiB, argonTime, argonThreads, enc.EncodeToString(salt), enc.EncodeToString(key)), nil
}

// VerifyPassword reports whether pw matches the PHC string. An error means
// the stored hash is unusable, which is different from a wrong password.
func VerifyPassword(pw, phc string) (bool, error) {
	// "$argon2id$v=19$m=..,t=..,p=..$salt$hash" splits into 6 parts, the first empty.
	parts := strings.Split(phc, "$")
	if len(parts) != 6 || parts[0] != "" || parts[1] != "argon2id" {
		return false, errMalformedHash
	}
	version, ok := strings.CutPrefix(parts[2], "v=")
	if v, err := strconv.Atoi(version); !ok || err != nil || v != argon2.Version {
		return false, errMalformedHash
	}
	var memory, iterations, threads uint64
	for _, kv := range strings.Split(parts[3], ",") {
		k, val, found := strings.Cut(kv, "=")
		n, err := strconv.ParseUint(val, 10, 32)
		if !found || err != nil {
			return false, errMalformedHash
		}
		switch k {
		case "m":
			memory = n
		case "t":
			iterations = n
		case "p":
			threads = n
		default:
			return false, errMalformedHash
		}
	}
	if memory == 0 || memory > maxArgonMemoryKiB || iterations == 0 || iterations > maxArgonTime ||
		threads == 0 || threads > maxArgonThreads {
		return false, errMalformedHash
	}
	salt, err := base64.RawStdEncoding.DecodeString(parts[4])
	if err != nil || len(salt) == 0 {
		return false, errMalformedHash
	}
	want, err := base64.RawStdEncoding.DecodeString(parts[5])
	if err != nil || len(want) == 0 {
		return false, errMalformedHash
	}
	got := argon2.IDKey([]byte(pw), salt, uint32(iterations), uint32(memory), uint8(threads), uint32(len(want)))
	return subtle.ConstantTimeCompare(got, want) == 1, nil
}
