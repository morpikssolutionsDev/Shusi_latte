// Package realtime is the in-process event bus and the WebSocket hub that fans events out
// to subscribed clients (TR-ARC-03, TRD §7).
package realtime

import "sync"

// Event carries only what a client needs to refetch: topic, type, entity id and an
// optional small payload (TR-RT-03). Clients refetch the entity over REST.
type Event struct {
	Topic string `json:"topic"`
	Type  string `json:"type"`
	ID    string `json:"id,omitempty"`
	Data  any    `json:"data,omitempty"`
}

// Bus is a synchronous publish/subscribe fan-out. Subscribers must not block.
type Bus struct {
	mu   sync.RWMutex
	next int
	subs map[int]func(Event)
}

func NewBus() *Bus { return &Bus{subs: map[int]func(Event){}} }

// Subscribe registers fn and returns a function that removes it.
func (b *Bus) Subscribe(fn func(Event)) (unsubscribe func()) {
	b.mu.Lock()
	id := b.next
	b.next++
	b.subs[id] = fn
	b.mu.Unlock()
	return func() {
		b.mu.Lock()
		delete(b.subs, id)
		b.mu.Unlock()
	}
}

// Publish delivers events to every subscriber. Call it only after the transaction that
// produced the events has committed.
func (b *Bus) Publish(events ...Event) {
	b.mu.RLock()
	defer b.mu.RUnlock()
	for _, e := range events {
		for _, fn := range b.subs {
			fn(e)
		}
	}
}
