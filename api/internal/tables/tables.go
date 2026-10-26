// Package tables implements Table management and QR codes from
// BR-ADM-08 and TRD §4.3.
//
// A Table is identified by a human label (for example "Table 3"),
// a random qr_token (internal, still used by the ordering API),
// and a static, printable slug (for example "table3").
//
// Scanning the printed QR opens the customer order page for that
// Table at {PUBLIC_URL}/order/{slug} — for example, with a public
// base of https://sushi.latte, Table 3 prints as
// https://sushi.latte/order/table3. That URL is static: it does
// not change when the internal token is regenerated, so a printed
// barcode keeps working. To invalidate a printed barcode, staff
// regenerate the slug (POST /api/v1/tables/{id}/regenerate-slug);
// the old slug then stops resolving. The old token path
// /public/tables/{token} still resolves, for QR codes printed
// before this change.
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
	Slug      string `json:"slug"`
	QRToken   string `json:"qr_token"`
	Active    bool   `json:"active"`
	CreatedAt int64  `json:"created_at"`
	UpdatedAt int64  `json:"updated_at"`
}

// Slugify turns a Table label into its static order slug:
// lowercase, letters and digits only, everything else dropped.
// "Table 3" -> "table3", "T1" -> "t1", "Meja 3" -> "meja3".
func Slugify(label string) string {
	var b strings.Builder
	for _, r := range strings.ToLower(strings.TrimSpace(label)) {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') {
			b.WriteRune(r)
		}
	}
	if b.Len() == 0 {
		return "table"
	}
	return b.String()
}

// QRURL returns the static customer order URL for this Table.
func QRURL(publicBase string, t Table) string {
	base := strings.TrimRight(publicBase, "/")
	slug := t.Slug
	if slug == "" {
		// Tables created before slugs existed: fall back to the
		// old token path until the slug is backfilled on read.
		if base == "" {
			return "/public/tables/" + t.QRToken
		}
		return base + "/public/tables/" + t.QRToken
	}
	if base == "" {
		return "/order/" + slug
	}
	return base + "/order/" + slug
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
