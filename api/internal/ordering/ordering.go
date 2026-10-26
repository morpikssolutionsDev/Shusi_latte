// Package ordering implements Phase 2a: table scan, join and the
// single open Bill, from BR-CM-01, BR-CM-02, BR-CM-12, BR-CM-13,
// BR-R-01, TR-DM-01 and TR-AUTH-07.
//
// Scanning a Table QR opens the Client Menu bound to that Table
// and opens or joins that Table's single open Bill. Before
// ordering, a Customer enters a full name (required) and may tick
// Dietary Restrictions and add a Dietary Note, with explicit
// consent collected before either is saved (BR-CM-02, TR-RET-03).
// Several Customers at the same Table may each order on the same
// Bill (BR-CM-06). A Customer is bound to the Bill, not the
// device: rejoin issues a new guest token, revokes previous ones
// and stores the new device as active (BR-CM-13, TR-AUTH-07).
// While a Bill is open, anyone who scans its Table QR sees the
// full names of the Customers on it (BR-CM-12, BR-P-07, an
// accepted owner decision).
//
// This step covers scan, join, rejoin, not-mine and waiter calls,
// plus the public catalog. Placing, editing and cancelling Orders,
// and the Customer bill view, are the next Phase 2 step.
package ordering

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"strings"
	"time"
)

type Bill struct {
	ID       string `json:"id"`
	Number   string `json:"number"`
	Type     string `json:"type"`
	Source   string `json:"source"`
	Status   string `json:"status"`
	OpenedAt int64  `json:"opened_at"`
	ClosedAt *int64 `json:"closed_at"`
}

type Customer struct {
	ID              string   `json:"customer_entry_id"`
	BillID          string   `json:"bill_id"`
	Name            string   `json:"name"`
	RestrictionTags []string `json:"restriction_tag_ids"`
	DietaryNote     *string  `json:"dietary_note"`
	ConsentAt       int64    `json:"consent_at"`
	ActiveDeviceID  *string  `json:"active_device_id"`
	RejoinedAt      *int64   `json:"rejoined_at"`
}

type TableScan struct {
	TableID    string     `json:"table_id"`
	TableLabel string     `json:"table_label"`
	Status     string     `json:"status"` // "free" or "occupied"
	BillID     *string    `json:"bill_id"`
	Customers  []Customer `json:"customers"`
}

type JoinResult struct {
	Bill       Bill     `json:"bill"`
	Customer   Customer `json:"customer"`
	GuestToken string   `json:"guest_token"`
	DeviceID   string   `json:"device_id"`
}

func NewID() string {
	b := make([]byte, 8)
	_, _ = rand.Read(b)
	return time.Now().Format("20060102150405") + "-" + hex.EncodeToString(b)
}

func NewToken() string {
	b := make([]byte, 32)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}

func ValidateJoin(name string, consent bool) error {
	if strings.TrimSpace(name) == "" {
		return errors.New("full name is required")
	}
	if !consent {
		return errors.New("explicit consent is required before name or dietary data is saved")
	}
	return nil
}
