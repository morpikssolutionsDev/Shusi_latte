// Package ops implements the escalation and auto-close timers
// from BR-ADM-13 and TRD §4.4 ops_setting.
//
// Management sets these timers; the scheduler jobs that read them
// arrive with the Waiter, Kitchen and HRIS phases:
//   - claim_renotify_seconds: re-notify all on-duty Waiters for
//     unclaimed Orders and unanswered Waiter calls (BR-WT-10)
//   - claim_escalate_seconds: alert the Outlet Admin after this
//     time (BR-WT-10)
//   - pickup_reminder_seconds: Ready items not Served within this
//     time show "waiting pickup" (TR-KT-01)
//   - empty_bill_close_seconds: an open Bill with no Orders closes
//     after this time, freeing the Table (BR-R-26)
//   - auto_clock_out_grace_seconds: a Crew member still clocked in
//     this long after shift end is clocked out automatically and
//     flagged "auto" (BR-HR-06)
//
// There is exactly one settings row. The defaults are the spec
// defaults: 120, 300, 180, 1800 and 7200 seconds.
package ops

import (
	"crypto/rand"
	"errors"
	"strings"
	"time"
)

type Settings struct {
	ClaimRenotifySeconds     int   `json:"claim_renotify_seconds"`
	ClaimEscalateSeconds     int   `json:"claim_escalate_seconds"`
	PickupReminderSeconds    int   `json:"pickup_reminder_seconds"`
	EmptyBillCloseSeconds    int   `json:"empty_bill_close_seconds"`
	AutoClockOutGraceSeconds int   `json:"auto_clock_out_grace_seconds"`
	CreatedAt                int64 `json:"created_at"`
	UpdatedAt                int64 `json:"updated_at"`
}

func Defaults() Settings {
	return Settings{
		ClaimRenotifySeconds:     120,
		ClaimEscalateSeconds:     300,
		PickupReminderSeconds:    180,
		EmptyBillCloseSeconds:    1800,
		AutoClockOutGraceSeconds: 7200,
	}
}

func Validate(s Settings) error {
	for _, v := range []int{s.ClaimRenotifySeconds, s.ClaimEscalateSeconds, s.PickupReminderSeconds, s.EmptyBillCloseSeconds, s.AutoClockOutGraceSeconds} {
		if v <= 0 {
			return errors.New("all ops timers must be greater than 0 seconds")
		}
	}
	if s.ClaimRenotifySeconds >= s.ClaimEscalateSeconds {
		return errors.New("claim_renotify_seconds must be less than claim_escalate_seconds, otherwise escalation would fire before re-notify")
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
