package server_test

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/cookiejar"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/coder/websocket"
	"github.com/coder/websocket/wsjson"

	"io"
	"log/slog"
	"sushilatte/api/internal/auth"
	"sushilatte/api/internal/config"
	"sushilatte/api/internal/dbx"
	"sushilatte/api/internal/realtime"
	"sushilatte/api/internal/server"
	"sushilatte/api/internal/testutil"
)

func newServer(t *testing.T) (*httptest.Server, *dbx.DB) {
	t.Helper()
	db, _ := testutil.NewDB(t)
	clock := testutil.NewClock(time.Now())
	ctx := context.Background()
	if err := db.Write(ctx, func(tx *dbx.Tx) error {
		_, err := auth.CreateCrew(ctx, tx, clock.Now(), auth.NewCrew{CrewID: "W001", FullName: "Andi", Role: auth.RoleCrew,
			Stations: []string{auth.StationWaiter}, PIN: "333333"})
		return err
	}); err != nil {
		t.Fatal(err)
	}
	cfg := config.Config{DemoMode: true, SecureCookie: false, OriginPatterns: []string{"*"}}
	h := server.New(server.Deps{Config: cfg, DB: db, Auth: auth.NewService(db, clock), Clock: clock,
		Log: slog.New(slog.NewTextHandler(io.Discard, nil))})
	srv := httptest.NewServer(h)
	t.Cleanup(srv.Close)
	return srv, db
}

func login(t *testing.T, srv *httptest.Server) *http.Client {
	t.Helper()
	jar, _ := cookiejar.New(nil)
	client := &http.Client{Jar: jar}
	res, err := client.Post(srv.URL+"/api/v1/auth/login", "application/json",
		strings.NewReader(`{"crewId":"W001","pin":"333333"}`))
	if err != nil {
		t.Fatal(err)
	}
	res.Body.Close()
	if res.StatusCode != http.StatusOK {
		t.Fatalf("login status %d", res.StatusCode)
	}
	return client
}

// TR-RT-01..03: a signed-in client subscribed to a topic receives events committed on it,
// and nothing from topics it may not subscribe to.
func TestWebSocketDeliversCommittedEvents(t *testing.T) {
	srv, db := newServer(t)
	client := login(t, srv)
	u, _ := url.Parse(srv.URL)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	header := http.Header{}
	for _, c := range client.Jar.Cookies(u) {
		header.Add("Cookie", c.String())
	}
	conn, _, err := websocket.Dial(ctx, "ws"+strings.TrimPrefix(srv.URL, "http")+"/ws", &websocket.DialOptions{HTTPHeader: header})
	if err != nil {
		t.Fatal(err)
	}
	defer conn.CloseNow()

	if err := wsjson.Write(ctx, conn, map[string]any{"op": "subscribe", "topics": []string{"waiters", "prep-area:1"}}); err != nil {
		t.Fatal(err)
	}
	var ack struct {
		Type   string   `json:"type"`
		Topics []string `json:"topics"`
		Denied []string `json:"denied"`
	}
	if err := wsjson.Read(ctx, conn, &ack); err != nil {
		t.Fatal(err)
	}
	if ack.Type != "subscribed" || len(ack.Topics) != 1 || ack.Topics[0] != "waiters" || len(ack.Denied) != 1 {
		t.Fatalf("unexpected ack %+v", ack)
	}

	if err := db.Write(ctx, func(tx *dbx.Tx) error {
		tx.Publish(realtime.Event{Topic: "prep-area:1", Type: "should.not.arrive"})
		tx.Publish(realtime.Event{Topic: "waiters", Type: "order.pending", ID: "o-1"})
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	var msg struct {
		Type  string         `json:"type"`
		Event realtime.Event `json:"event"`
	}
	if err := wsjson.Read(ctx, conn, &msg); err != nil {
		t.Fatal(err)
	}
	if msg.Type != "event" || msg.Event.Type != "order.pending" || msg.Event.ID != "o-1" {
		t.Fatalf("unexpected message %+v", msg)
	}
}

// TR-API-01: repeating a mutating request with the same Idempotency-Key replays the response.
func TestIdempotencyReplay(t *testing.T) {
	srv, _ := newServer(t)
	client := login(t, srv)
	post := func() *http.Response {
		req, _ := http.NewRequest(http.MethodPost, srv.URL+"/api/v1/auth/override", bytes.NewBufferString(`{"crewId":"W001","pin":"333333","action":"void"}`))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Idempotency-Key", "k-1")
		res, err := client.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		return res
	}
	first := post()
	firstBody, _ := io.ReadAll(first.Body)
	first.Body.Close()
	second := post()
	secondBody, _ := io.ReadAll(second.Body)
	second.Body.Close()
	if second.Header.Get("Idempotent-Replay") != "true" || !bytes.Equal(firstBody, secondBody) || first.StatusCode != second.StatusCode {
		t.Fatalf("not replayed: %d %s / %d %s", first.StatusCode, firstBody, second.StatusCode, secondBody)
	}
}

// DEMO-AND-SEED §2: demo accounts are listed in Demo mode.
func TestDemoAccounts(t *testing.T) {
	srv, _ := newServer(t)
	res, err := http.Get(srv.URL + "/api/v1/demo/accounts")
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	var accounts []map[string]any
	if err := json.NewDecoder(res.Body).Decode(&accounts); err != nil || len(accounts) != 12 {
		t.Fatalf("want 12 accounts, got %d (%v)", len(accounts), err)
	}
}
