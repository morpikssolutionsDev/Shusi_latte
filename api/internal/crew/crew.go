// Package crew implements crew records from BR-ADM-09 and TRD §4.1.
//
// Management manages crew records, roles, Stations, Crew IDs and PIN
// resets. This is the Admin Panel view of crew: full HRIS employee
// records, shifts, attendance and leave arrive in Phase 7 (BR-HR-01
// to BR-HR-07), not here.
//
// Sign-in is Crew ID plus a 6-digit PIN for every role (BR-R-24,
// TR-AUTH-01). PINs are hashed with Argon2id and never stored or
// returned in plain text. After 5 consecutive failed attempts an
// account locks for 15 minutes (TR-AUTH-02). The VerifyPIN method
// implements that lockout logic for the auth phase; it is not yet
// exposed as a login endpoint, which also needs session handling
// (TR-AUTH-03) and per-IP rate limiting (TR-AUTH-02).
package crew

import (
	"crypto/rand"
	"encoding/base64"
	"errors"
	"fmt"
	"regexp"
	"strings"
	"time"

	"golang.org/x/crypto/argon2"
)

const (
	RoleManagement  = "management"
	RoleOutletAdmin = "outlet_admin"
	RoleCrew        = "crew"

	StationWaiter  = "waiter"
	StationCashier = "cashier"
	StationKitchen = "kitchen"

	maxFailedAttempts = 5
	lockoutDuration   = 15 * time.Minute
)

var pinRe = regexp.MustCompile(`^[0-9]{6}$`)

// Crew is the public view of a crew record. PinHash is never included
// in JSON responses; it is only used internally by the store.
type Crew struct {
	ID             string   `json:"id"`
	CrewID         string   `json:"crew_id"`
	FullName       string   `json:"full_name"`
	Role           string   `json:"role"`
	ImageKey       *string  `json:"image_key"`
	Stations       []string `json:"stations"`
	FailedAttempts int      `json:"failed_attempts"`
	LockedUntil    *int64   `json:"locked_until"`
	Active         bool     `json:"active"`
	CreatedAt      int64    `json:"created_at"`
	UpdatedAt      int64    `json:"updated_at"`
}

func ValidateRole(role string) error {
	switch role {
	case RoleManagement, RoleOutletAdmin, RoleCrew:
		return nil
	default:
		return errors.New("role must be management, outlet_admin or crew")
	}
}

func ValidateStations(stations []string) ([]string, error) {
	seen := make(map[string]bool)
	out := []string{}
	for _, s := range stations {
		switch s {
		case StationWaiter, StationCashier, StationKitchen:
		default:
			return nil, fmt.Errorf("unknown station: %s", s)
		}
		if !seen[s] {
			seen[s] = true
			out = append(out, s)
		}
	}
	return out, nil
}

func ValidatePIN(pin string) error {
	if !pinRe.MatchString(pin) {
		return errors.New("pin must be exactly 6 digits")
	}
	return nil
}

func NewID() string {
	b := make([]byte, 8)
	_, _ = rand.Read(b)
	return time.Now().Format("20060102150405") + "-" + hexEncode(b)
}

func hexEncode(b []byte) string {
	const digits = "0123456789abcdef"
	var sb strings.Builder
	for _, c := range b {
		sb.WriteByte(digits[c>>4])
		sb.WriteByte(digits[c&0x0f])
	}
	return sb.String()
}

// HashPIN hashes a PIN with Argon2id, in the standard encoded form:
// $argon2id$v=19$m=65536,t=1,p=4$<salt>$<hash>
func HashPIN(pin string) (string, error) {
	if err := ValidatePIN(pin); err != nil {
		return "", err
	}
	salt := make([]byte, 16)
	if _, err := rand.Read(salt); err != nil {
		return "", err
	}
	hash := argon2.IDKey([]byte(pin), salt, 1, 64*1024, 4, 32)
	return fmt.Sprintf("$argon2id$v=19$m=65536,t=1,p=4$%s$%s",
		base64.RawStdEncoding.EncodeToString(salt),
		base64.RawStdEncoding.EncodeToString(hash),
	), nil
}

// VerifyPINHash checks a PIN against an Argon2id hash produced by HashPIN.
// It returns false, not an error, for a malformed hash, so a bad row
// never grants access.
func VerifyPINHash(pin, encoded string) bool {
	parts := strings.Split(encoded, "$")
	if len(parts) != 6 || parts[1] != "argon2id" {
		return false
	}
	var m, t uint32
	var p uint8
	if _, err := fmt.Sscanf(parts[3], "m=%d,t=%d,p=%d", &m, &t, &p); err != nil {
		return false
	}
	salt, err := base64.RawStdEncoding.DecodeString(parts[4])
	if err != nil {
		return false
	}
	expected, err := base64.RawStdEncoding.DecodeString(parts[5])
	if err != nil {
		return false
	}
	got := argon2.IDKey([]byte(pin), salt, t, m, p, uint32(len(expected)))
	if len(got) != len(expected) {
		return false
	}
	var diff byte
	for i := range got {
		diff |= got[i] ^ expected[i]
	}
	return diff == 0
}

// Locked reports whether the account is currently locked.
func (c Crew) Locked(nowMillis int64) bool {
	return c.LockedUntil != nil && nowMillis < *c.LockedUntil
}
