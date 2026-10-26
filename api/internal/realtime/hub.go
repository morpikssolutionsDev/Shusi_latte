package realtime

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"slices"
	"sync"
	"time"

	"github.com/coder/websocket"
	"github.com/coder/websocket/wsjson"
)

// SystemTopic is delivered to every connection without subscribing (e.g. demo.reset).
const SystemTopic = "system"

const (
	heartbeatEvery = 20 * time.Second
	writeTimeout   = 10 * time.Second
	sendBuffer     = 64
)

// Subscriber is the authenticated identity behind one WebSocket connection.
type Subscriber struct {
	ID string
	// Allowed reports whether this subscriber may receive a topic.
	Allowed func(topic string) bool
}

// Hub accepts WebSocket connections and forwards bus events on subscribed topics (TR-RT-01..04).
type Hub struct {
	bus            *Bus
	authenticate   func(*http.Request) (*Subscriber, error)
	originPatterns []string
	log            *slog.Logger
}

func NewHub(bus *Bus, authenticate func(*http.Request) (*Subscriber, error), originPatterns []string, log *slog.Logger) *Hub {
	return &Hub{bus: bus, authenticate: authenticate, originPatterns: originPatterns, log: log}
}

type clientMessage struct {
	Op     string   `json:"op"`
	Topics []string `json:"topics"`
}

// serverMessage is either {"type":"subscribed",...} or {"type":"event","event":{...}}.
type serverMessage struct {
	Type   string   `json:"type"`
	Topics []string `json:"topics,omitempty"`
	Denied []string `json:"denied,omitempty"`
	Event  *Event   `json:"event,omitempty"`
}

func (h *Hub) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	sub, err := h.authenticate(r)
	if err != nil {
		http.Error(w, "unauthorised", http.StatusUnauthorized)
		return
	}
	conn, err := websocket.Accept(w, r, &websocket.AcceptOptions{OriginPatterns: h.originPatterns})
	if err != nil {
		return
	}
	ctx, cancel := context.WithCancel(r.Context())
	defer cancel()

	c := &client{sub: sub, topics: map[string]bool{SystemTopic: true}, send: make(chan Event, sendBuffer), cancel: cancel}
	unsubscribe := h.bus.Subscribe(c.offer)
	defer unsubscribe()

	go h.readLoop(ctx, conn, c)
	err = h.writeLoop(ctx, conn, c)
	if err != nil && !errors.Is(err, context.Canceled) && websocket.CloseStatus(err) == -1 {
		h.log.Debug("websocket closed", "subscriber", sub.ID, "err", err)
	}
	conn.CloseNow()
}

type client struct {
	sub    *Subscriber
	mu     sync.RWMutex
	topics map[string]bool
	send   chan Event
	cancel context.CancelFunc
}

// offer is called by the bus. A slow client is disconnected rather than blocking the bus;
// on reconnect it refetches everything (TR-RT-04).
func (c *client) offer(e Event) {
	c.mu.RLock()
	wanted := c.topics[e.Topic]
	c.mu.RUnlock()
	if !wanted {
		return
	}
	select {
	case c.send <- e:
	default:
		c.cancel()
	}
}

func (h *Hub) readLoop(ctx context.Context, conn *websocket.Conn, c *client) {
	defer c.cancel()
	for {
		var msg clientMessage
		if err := wsjson.Read(ctx, conn, &msg); err != nil {
			return
		}
		var granted, denied []string
		c.mu.Lock()
		for _, t := range msg.Topics {
			switch msg.Op {
			case "subscribe":
				if c.sub.Allowed(t) {
					c.topics[t] = true
					granted = append(granted, t)
				} else {
					denied = append(denied, t)
				}
			case "unsubscribe":
				if t != SystemTopic {
					delete(c.topics, t)
				}
			}
		}
		c.mu.Unlock()
		if msg.Op == "subscribe" {
			ack := serverMessage{Type: "subscribed", Topics: granted, Denied: denied}
			wctx, cancel := context.WithTimeout(ctx, writeTimeout)
			_ = wsjson.Write(wctx, conn, ack)
			cancel()
		}
	}
}

func (h *Hub) writeLoop(ctx context.Context, conn *websocket.Conn, c *client) error {
	ticker := time.NewTicker(heartbeatEvery)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case e := <-c.send:
			payload, err := json.Marshal(serverMessage{Type: "event", Event: &e})
			if err != nil {
				return err
			}
			wctx, cancel := context.WithTimeout(ctx, writeTimeout)
			err = conn.Write(wctx, websocket.MessageText, payload)
			cancel()
			if err != nil {
				return err
			}
		case <-ticker.C:
			pctx, cancel := context.WithTimeout(ctx, writeTimeout)
			err := conn.Ping(pctx)
			cancel()
			if err != nil {
				return err
			}
		}
	}
}

// TopicMatcher returns an Allowed func for exact topics and "prefix:*" patterns.
func TopicMatcher(patterns ...string) func(string) bool {
	return func(topic string) bool {
		for _, p := range patterns {
			if p == topic {
				return true
			}
			if n := len(p); n > 1 && p[n-1] == '*' && len(topic) >= n-1 && topic[:n-1] == p[:n-1] {
				return true
			}
		}
		return slices.Contains(patterns, "*")
	}
}
