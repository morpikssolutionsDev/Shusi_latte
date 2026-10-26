// Package tables implements Table management and QR codes from
// BR-ADM-08 and TRD §4.3.
//
// A Table is identified by a human label (for example "T1") and a
// random qr_token. Scanning the QR opens the Client Menu bound to
// that Table (BR-CM-01, Phase 2). Regenerating the token invalidates
// printed QR codes for that Table (TR-AUTH-07): the old token stops
// resolving, so a photographed or stolen QR cannot be reused after
// staff regenerate it.
//
// The QR payload is the Client Menu URL for the token. Its base comes
// from SUSHILATTE_PUBLIC_URL; when that is unset, the payload is the
// relative path /public/tables/{token}, which the demo host operator
// must prefix with the host URL before printing.
package tables

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"strings"
	"time"
)

type Table struct {
	ID        string `json:"id"`
	Label     string `json:"label"`
	QRToken   string `json:"qr_token"`
	Active    bool   `json:"active"`
	CreatedAt int64  `json:"created_at"`
	UpdatedAt int64  `json:"updated_at"`
}

// QRURL returns the Client Menu URL for this Table.
func QRURL(publicBase string, t Table) string {
	base := strings.TrimRight(publicBase, "/")
	if base == "" {
		return "/public/tables/" + t.QRToken
	}
	return base + "/public/tables/" + t.QRToken
}

func NewToken() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}

func NewID() string {
	b := make([]byte, 8)
	_, _ = rand.Read(b)
	return time.Now().Format("20060102150405") + "-" + hex.EncodeToString(b)
}

func ValidateLabel(label string) error {
	if strings.TrimSpace(label) == "" {
		return errors.New("table label is required")
	}
	return nil
}
