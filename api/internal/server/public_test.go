package server_test

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/coder/websocket"
	"github.com/coder/websocket/wsjson"

	"sushilatte/api/internal/auth"
	"sushilatte/api/internal/catalog"
	"sushilatte/api/internal/dbx"
	"sushilatte/api/internal/outlet"
	"sushilatte/api/internal/platform"
	"sushilatte/api/internal/store"
)

// publicFixture adds one Table and one Product to the test server's database.
func publicFixture(t *testing.T, db *dbx.DB) (qr, variantID string) {
	t.Helper()
	ctx := context.Background()
	clock := platform.SystemClock{}
	var actor string
	if err := db.Write(ctx, func(tx *dbx.Tx) error {
		var err error
		actor, err = auth.CreateCrew(ctx, tx, clock.Now(), auth.NewCrew{CrewID: "M001", FullName: "Hendra", Role: auth.RoleManagement, PIN: "111111"})
		if err != nil {
			return err
		}
		return tx.CreatePrepArea(ctx, store.CreatePrepAreaParams{ID: "kitchen", Code: "kitchen", NameID: "Dapur", NameEn: "Kitchen", ParallelCapacity: 2})
	}); err != nil {
		t.Fatal(err)
	}
	cat := catalog.NewService(db, clock)
	c, _ := cat.SaveCategory(ctx, actor, catalog.Category{NameID: "Nigiri", NameEN: "Nigiri", PrepAreaID: "kitchen", Active: true})
	p, err := cat.SaveProduct(ctx, actor, catalog.Product{CategoryID: c.ID, NameID: "Salmon Aburi", NameEN: "Salmon Aburi", PrepTimeSeconds: 480,
		Status: catalog.StatusActive, Variants: []catalog.Variant{{NameID: "2 pcs", NameEN: "2 pcs", Price: 48000, IsDefault: true}}})
	if err != nil {
		t.Fatal(err)
	}
	tbl, err := outlet.NewService(db, clock).SaveTable(ctx, actor, outlet.Table{Number: 3, Label: "Meja 3", Active: true})
	if err != nil {
		t.Fatal(err)
	}
	return tbl.QRToken, p.Variants[0].ID
}

func doJSON(t *testing.T, method, url, token, idemKey, body string) (*http.Response, []byte) {
	t.Helper()
	req, _ := http.NewRequest(method, url, bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	if token != "" {
		req.Header.Set("X-Guest-Token", token)
	}
	if idemKey != "" {
		req.Header.Set("Idempotency-Key", idemKey)
	}
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	b, _ := io.ReadAll(res.Body)
	res.Body.Close()
	return res, b
}

// BR-CM-01, BR-CM-05, TR-API-01, TR-RT-01: a Customer joins over HTTP, orders once even when the
// phone retries, and follows only their own Bill on the WebSocket.
func TestPublicOrderingFlow(t *testing.T) {
	srv, db := newServer(t)
	qr, variant := publicFixture(t, db)
	api := srv.URL + "/api/v1/public"

	res, body := doJSON(t, http.MethodGet, api+"/tables/"+qr, "", "", "")
	if res.StatusCode != 200 || !strings.Contains(string(body), `"state":"free"`) {
		t.Fatalf("lookup %d %s", res.StatusCode, body)
	}
	res, body = doJSON(t, http.MethodPost, api+"/tables/"+qr+"/join", "", "", `{"name":"Raka"}`)
	if res.StatusCode != http.StatusCreated {
		t.Fatalf("join %d %s", res.StatusCode, body)
	}
	var session struct {
		Token  string `json:"token"`
		BillID string `json:"billId"`
	}
	_ = json.Unmarshal(body, &session)

	// Without a token, guest endpoints refuse.
	if res, _ := doJSON(t, http.MethodGet, api+"/bills/me", "", "", ""); res.StatusCode != http.StatusUnauthorized {
		t.Fatalf("me without token: %d", res.StatusCode)
	}

	// Guest WebSocket: own Bill allowed, another Bill denied.
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	conn, _, err := websocket.Dial(ctx, "ws"+strings.TrimPrefix(srv.URL, "http")+"/ws?guest="+session.Token, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.CloseNow()
	_ = wsjson.Write(ctx, conn, map[string]any{"op": "subscribe", "topics": []string{"bill:" + session.BillID, "bill:someone-else", "waiters"}})
	var ack struct {
		Topics []string `json:"topics"`
		Denied []string `json:"denied"`
	}
	if err := wsjson.Read(ctx, conn, &ack); err != nil || len(ack.Topics) != 1 || len(ack.Denied) != 2 {
		t.Fatalf("ack %+v %v", ack, err)
	}

	// The same Idempotency-Key twice creates one Order (double tap on "Kirim pesanan").
	order := `{"items":[{"variantId":"` + variant + `","quantity":2,"modifierIds":[]}]}`
	r1, b1 := doJSON(t, http.MethodPost, api+"/orders", session.Token, "tap-1", order)
	r2, b2 := doJSON(t, http.MethodPost, api+"/orders", session.Token, "tap-1", order)
	if r1.StatusCode != http.StatusCreated || r2.Header.Get("Idempotent-Replay") != "true" || !bytes.Equal(b1, b2) {
		t.Fatalf("order replay: %d %s / %d %s", r1.StatusCode, b1, r2.StatusCode, r2.Header.Get("Idempotent-Replay"))
	}
	_, me := doJSON(t, http.MethodGet, api+"/bills/me", session.Token, "", "")
	var view struct {
		Orders []struct {
			Status   string `json:"status"`
			Subtotal int64  `json:"subtotal"`
		} `json:"orders"`
	}
	_ = json.Unmarshal(me, &view)
	if len(view.Orders) != 1 || view.Orders[0].Status != "pending" || view.Orders[0].Subtotal != 96000 {
		t.Fatalf("me = %s", me)
	}

	// The commit reached the Customer's socket.
	var msg struct {
		Event struct {
			Type string `json:"type"`
		} `json:"event"`
	}
	if err := wsjson.Read(ctx, conn, &msg); err != nil || msg.Event.Type != "order.created" {
		t.Fatalf("event %+v %v", msg, err)
	}

	// Public catalog carries an ETag and answers 304 when unchanged.
	res, _ = doJSON(t, http.MethodGet, api+"/catalog", "", "", "")
	etag := res.Header.Get("ETag")
	req, _ := http.NewRequest(http.MethodGet, api+"/catalog", nil)
	req.Header.Set("If-None-Match", etag)
	res304, _ := http.DefaultClient.Do(req)
	res304.Body.Close()
	if etag == "" || res304.StatusCode != http.StatusNotModified {
		t.Fatalf("etag %q → %d", etag, res304.StatusCode)
	}
}
