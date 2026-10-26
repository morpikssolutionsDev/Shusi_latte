package auth_test

import (
	"context"
	"errors"
	"net/http"
	"testing"
	"time"

	"sushilatte/api/internal/audit"
	"sushilatte/api/internal/auth"
	"sushilatte/api/internal/dbx"
	"sushilatte/api/internal/httpx"
	"sushilatte/api/internal/testutil"
)

var t0 = time.Date(2026, 10, 26, 12, 0, 0, 0, time.UTC)

type fixture struct {
	db    *dbx.DB
	clock *testutil.Clock
	svc   *auth.Service
}

func setup(t *testing.T) fixture {
	t.Helper()
	db, _ := testutil.NewDB(t)
	clock := testutil.NewClock(t0)
	ctx := context.Background()
	crew := []auth.NewCrew{
		{CrewID: "M001", FullName: "Hendra", Role: auth.RoleManagement, PIN: "111111"},
		{CrewID: "A001", FullName: "Rina", Role: auth.RoleOutletAdmin, PIN: "222222"},
		{CrewID: "W001", FullName: "Andi", Role: auth.RoleCrew, Stations: []string{auth.StationWaiter}, PIN: "333333"},
	}
	if err := db.Write(ctx, func(tx *dbx.Tx) error {
		for _, c := range crew {
			if _, err := auth.CreateCrew(ctx, tx, t0, c); err != nil {
				return err
			}
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	return fixture{db: db, clock: clock, svc: auth.NewService(db, clock)}
}

func problemCode(err error) string {
	var p *httpx.Problem
	if errors.As(err, &p) {
		return p.Code
	}
	return ""
}

// TR-AUTH-01: Crew ID + PIN sign-in returns the principal with stations.
func TestLoginSucceeds(t *testing.T) {
	f := setup(t)
	token, p, err := f.svc.Login(context.Background(), "W001", "333333", "tablet-1")
	if err != nil {
		t.Fatal(err)
	}
	if token == "" || p.CrewID != "W001" || !p.HasStation(auth.StationWaiter) {
		t.Fatalf("unexpected principal %+v", p)
	}
	got, err := f.svc.Authenticate(context.Background(), token)
	if err != nil || got.CrewID != "W001" {
		t.Fatalf("authenticate: %v %+v", err, got)
	}
}

// TR-AUTH-02, BR-R-24: five wrong PINs lock the account for 15 minutes, even with the right PIN.
func TestLockoutAfterFiveFailures(t *testing.T) {
	f := setup(t)
	ctx := context.Background()
	for i := 1; i <= 4; i++ {
		if _, _, err := f.svc.Login(ctx, "W001", "000000", ""); problemCode(err) != "invalid_credentials" {
			t.Fatalf("attempt %d: want invalid_credentials, got %v", i, err)
		}
	}
	if _, _, err := f.svc.Login(ctx, "W001", "000000", ""); problemCode(err) != "account_locked" {
		t.Fatalf("5th attempt: want account_locked, got %v", err)
	}
	if _, _, err := f.svc.Login(ctx, "W001", "333333", ""); problemCode(err) != "account_locked" {
		t.Fatalf("correct PIN while locked: want account_locked, got %v", err)
	}
	f.clock.Advance(15*time.Minute + time.Second)
	if _, _, err := f.svc.Login(ctx, "W001", "333333", ""); err != nil {
		t.Fatalf("after lockout: %v", err)
	}
}

// TR-AUTH-04: Management sessions expire after 30 minutes idle.
func TestManagementIdleTimeout(t *testing.T) {
	f := setup(t)
	ctx := context.Background()
	token, _, err := f.svc.Login(ctx, "M001", "111111", "")
	if err != nil {
		t.Fatal(err)
	}
	f.clock.Advance(29 * time.Minute)
	if _, err := f.svc.Authenticate(ctx, token); err != nil {
		t.Fatalf("within idle window: %v", err)
	}
	f.clock.Advance(31 * time.Minute)
	if _, err := f.svc.Authenticate(ctx, token); problemCode(err) != "session_expired" {
		t.Fatalf("after idle: want session_expired, got %v", err)
	}
}

// TR-AUTH-06: a PIN Override round trip records actor and approver in the audit log, and the
// token is single-use.
func TestOverrideRoundTripAudited(t *testing.T) {
	f := setup(t)
	ctx := context.Background()
	_, waiter, err := f.svc.Login(ctx, "W001", "333333", "")
	if err != nil {
		t.Fatal(err)
	}
	token, _, err := f.svc.IssueOverride(ctx, waiter, "A001", "222222", auth.ActionVoid)
	if err != nil {
		t.Fatal(err)
	}
	consume := func() error {
		return f.db.Write(ctx, func(tx *dbx.Tx) error {
			_, err := auth.ConsumeOverride(ctx, tx, f.clock.Now(), waiter, token, auth.ActionVoid,
				audit.Entry{Action: "order_item.voided", Entity: "order_item", EntityID: "oi-1", Reason: "test"})
			return err
		})
	}
	if err := consume(); err != nil {
		t.Fatal(err)
	}
	rows, err := f.db.Read().ListAudit(ctx, 10)
	if err != nil || len(rows) != 1 {
		t.Fatalf("want 1 audit row, got %d (%v)", len(rows), err)
	}
	admin, _ := f.db.Read().GetCrewByCrewID(ctx, "A001")
	if rows[0].ActorCrewID.String != waiter.ID || rows[0].OverrideCrewID.String != admin.ID {
		t.Fatalf("audit actor/approver wrong: %+v", rows[0])
	}
	if err := consume(); problemCode(err) != "override_invalid" {
		t.Fatalf("second use: want override_invalid, got %v", err)
	}
}

// BRD §5.1: manual discounts need Management; an Outlet Admin PIN is refused.
func TestOverrideApproverRole(t *testing.T) {
	f := setup(t)
	ctx := context.Background()
	_, waiter, _ := f.svc.Login(ctx, "W001", "333333", "")
	_, _, err := f.svc.IssueOverride(ctx, waiter, "A001", "222222", auth.ActionManualDiscount)
	var p *httpx.Problem
	if !errors.As(err, &p) || p.Status != http.StatusForbidden {
		t.Fatalf("want 403 approver_not_allowed, got %v", err)
	}
	if _, _, err := f.svc.IssueOverride(ctx, waiter, "M001", "111111", auth.ActionManualDiscount); err != nil {
		t.Fatalf("management approval: %v", err)
	}
}

// TR-API-03: override tokens expire after 60 seconds.
func TestOverrideExpires(t *testing.T) {
	f := setup(t)
	ctx := context.Background()
	_, waiter, _ := f.svc.Login(ctx, "W001", "333333", "")
	token, _, err := f.svc.IssueOverride(ctx, waiter, "A001", "222222", auth.ActionRemake)
	if err != nil {
		t.Fatal(err)
	}
	f.clock.Advance(61 * time.Second)
	err = f.db.Write(ctx, func(tx *dbx.Tx) error {
		_, err := auth.ConsumeOverride(ctx, tx, f.clock.Now(), waiter, token, auth.ActionRemake, audit.Entry{Action: "x", Entity: "x"})
		return err
	})
	if problemCode(err) != "override_invalid" {
		t.Fatalf("want override_invalid, got %v", err)
	}
}

// TR-AUTH-08: only Outlet Admin or Management can open a screen-mode session.
func TestScreenMode(t *testing.T) {
	f := setup(t)
	ctx := context.Background()
	_, waiter, _ := f.svc.Login(ctx, "W001", "333333", "")
	if _, _, err := f.svc.EnterScreenMode(ctx, waiter, auth.ScreenKitchen, ""); problemCode(err) != "forbidden" {
		t.Fatalf("waiter: want forbidden, got %v", err)
	}
	_, admin, _ := f.svc.Login(ctx, "A001", "222222", "")
	token, p, err := f.svc.EnterScreenMode(ctx, admin, auth.ScreenKitchen, "kitchen-tv")
	if err != nil || p.Kind != "screen" || p.Screen != auth.ScreenKitchen {
		t.Fatalf("screen mode: %v %+v", err, p)
	}
	if _, err := f.svc.Authenticate(ctx, token); err != nil {
		t.Fatal(err)
	}
}
