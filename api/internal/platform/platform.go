// Package platform holds small shared primitives: ids, time and money conventions (TRD §4).
package platform

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"time"
	_ "time/tzdata" // Asia/Jakarta even on hosts without a zoneinfo database

	"github.com/google/uuid"
)

// Jakarta is the outlet's display time zone (WIB).
var Jakarta = mustLoad("Asia/Jakarta")

func mustLoad(name string) *time.Location {
	loc, err := time.LoadLocation(name)
	if err != nil {
		panic(err)
	}
	return loc
}

// NewID returns a UUIDv7 string: time-ordered, so primary keys stay roughly insertion-ordered.
func NewID() string {
	return uuid.Must(uuid.NewV7()).String()
}

// Clock lets tests and the demo seed control "now".
type Clock interface{ Now() time.Time }

type SystemClock struct{}

func (SystemClock) Now() time.Time { return time.Now() }

// Millis converts a time to the storage format: UTC Unix milliseconds.
func Millis(t time.Time) int64 { return t.UnixMilli() }

// FromMillis converts storage milliseconds back to a time in WIB.
func FromMillis(ms int64) time.Time { return time.UnixMilli(ms).In(Jakarta) }

// NewToken returns a random URL-safe secret and its SHA-256 hex digest.
// Only the digest is stored, so a database leak does not leak usable tokens.
func NewToken() (token, digest string) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		panic(err)
	}
	token = base64.RawURLEncoding.EncodeToString(b)
	return token, Digest(token)
}

// Digest is the stored form of a secret token.
func Digest(token string) string {
	sum := sha256.Sum256([]byte(token))
	return hex.EncodeToString(sum[:])
}
