package server_test

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/cookiejar"
	"net/http/httptest"
	"testing"
	"time"

	"sushilatte/api/internal/auth"
	"sushilatte/api/internal/catalog"
	"sushilatte/api/internal/config"
	"sushilatte/api/internal/dbx"
	"sushilatte/api/internal/server"
	"sushilatte/api/internal/store"
	"sushilatte/api/internal/testutil"
)

// adminServer seeds Management, an Outlet Admin, a Waiter and one Product.
func adminServer(t *testing.T) (*httptest.Server, catalog.Product) {
	t.Helper()
	db, _ := testutil.NewDB(t)
	clock := testutil.NewClock(time.Now())
	ctx := context.Background()
	var mgmt string
	if err := db.Write(ctx, func(tx *dbx.Tx) error {
		for _, c := range []auth.NewCrew{
			{CrewID: "M001", FullName: "Hendra", Role: auth.RoleManagement, PIN: "111111"},
			{CrewID: "A001", FullName: "Rina", Role: auth.RoleOutletAdmin, PIN: "222222"},
			{CrewID: "W001", FullName: "Andi", Role: auth.RoleCrew, Stations: []string{auth.StationWaiter}, PIN: "333333"},
		} {
			id, err := auth.CreateCrew(ctx, tx, clock.Now(), c)
			if err != nil {
				return err
			}
			if c.CrewID == "M001" {
				mgmt = id
			}
		}
		return tx.CreatePrepArea(ctx, store.CreatePrepAreaParams{ID: "pa", Code: "kitchen", NameID: "Dapur", NameEn: "Kitchen", ParallelCapacity: 2})
	}); err != nil {
		t.Fatal(err)
	}
	svc := catalog.NewService(db, clock)
	cat, err := svc.SaveCategory(ctx, mgmt, catalog.Category{NameID: "Kopi", NameEN: "Coffee", PrepAreaID: "pa"})
	if err != nil {
		t.Fatal(err)
	}
	p, err := svc.SaveProduct(ctx, mgmt, catalog.Product{CategoryID: cat.ID, NameID: "Latte", NameEN: "Latte", PrepTimeSeconds: 240,
		Variants: []catalog.Variant{{NameID: "Panas", NameEN: "Hot", Price: 38000}}})
	if err != nil {
		t.Fatal(err)
	}
	cfg := config.Config{SecureCookie: false, OriginPatterns: []string{"*"}, DataDir: t.TempDir()}
	srv := httptest.NewServer(server.New(server.Deps{Config: cfg, DB: db, Auth: auth.NewService(db, clock), Clock: clock,
		Log: slog.New(slog.NewTextHandler(io.Discard, nil))}))
	t.Cleanup(srv.Close)
	return srv, p
}

func signIn(t *testing.T, srv *httptest.Server, crewID, pin string) *http.Client {
	t.Helper()
	jar, _ := cookiejar.New(nil)
	c := &http.Client{Jar: jar}
	res, err := c.Post(srv.URL+"/api/v1/auth/login", "application/json", bytes.NewBufferString(`{"crewId":"`+crewID+`","pin":"`+pin+`"}`))
	if err != nil || res.StatusCode != http.StatusOK {
		t.Fatalf("login %s: %v %v", crewID, err, res.Status)
	}
	res.Body.Close()
	return c
}

func send(t *testing.T, c *http.Client, method, url string, body any) int {
	t.Helper()
	b, _ := json.Marshal(body)
	req, _ := http.NewRequest(method, url, bytes.NewReader(b))
	req.Header.Set("Content-Type", "application/json")
	res, err := c.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	res.Body.Close()
	return res.StatusCode
}

// Phase 1 done-when / BRD §5.1: Outlet Admin can toggle Sold Out but cannot edit prices;
// Waiters can do neither; Management can do both.
func TestAdminPermissions(t *testing.T) {
	srv, p := adminServer(t)
	soldOut := srv.URL + "/api/v1/catalog/variants/" + p.Variants[0].ID + "/sold-out"
	productURL := srv.URL + "/api/v1/catalog/products/" + p.ID
	repriced := p
	repriced.Variants = append([]catalog.Variant(nil), p.Variants...)
	repriced.Variants[0].Price = 99000

	admin := signIn(t, srv, "A001", "222222")
	if got := send(t, admin, http.MethodPut, soldOut, map[string]bool{"soldOut": true}); got != http.StatusOK {
		t.Fatalf("outlet admin sold out: %d", got)
	}
	if got := send(t, admin, http.MethodPut, productURL, repriced); got != http.StatusForbidden {
		t.Fatalf("outlet admin price edit: want 403, got %d", got)
	}

	waiter := signIn(t, srv, "W001", "333333")
	if got := send(t, waiter, http.MethodPut, soldOut, map[string]bool{"soldOut": false}); got != http.StatusForbidden {
		t.Fatalf("waiter sold out: want 403, got %d", got)
	}
	if got := send(t, waiter, http.MethodGet, srv.URL+"/api/v1/catalog", nil); got != http.StatusOK {
		t.Fatalf("waiter catalog read: %d", got)
	}

	mgmt := signIn(t, srv, "M001", "111111")
	if got := send(t, mgmt, http.MethodPut, productURL, repriced); got != http.StatusOK {
		t.Fatalf("management price edit: %d", got)
	}
	if got := send(t, mgmt, http.MethodGet, srv.URL+"/api/v1/crew", nil); got != http.StatusOK {
		t.Fatalf("management crew list: %d", got)
	}
	if got := send(t, admin, http.MethodGet, srv.URL+"/api/v1/crew", nil); got != http.StatusForbidden {
		t.Fatalf("outlet admin crew list: want 403, got %d", got)
	}
}
