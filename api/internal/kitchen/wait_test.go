package kitchen_test

import (
	"testing"

	"sushilatte/api/internal/kitchen"
)

const min = 60

// TR-KT-03 worked examples.
func TestEstimatedWait(t *testing.T) {
	now := int64(1_000_000)
	capacity := map[string]int64{"kitchen": 2, "bar": 1}
	queue := []kitchen.QueueItem{
		// Ahead in the Kitchen: one Preparing with 4 min left, one waiting 10 min.
		{ID: "a1", OrderID: "A", PrepAreaID: "kitchen", PrepTimeSeconds: 12 * min, Preparing: true, PreparingAt: now - 8*min*1000, ConfirmedAt: 1},
		{ID: "a2", OrderID: "A", PrepAreaID: "kitchen", PrepTimeSeconds: 10 * min, ConfirmedAt: 1},
		// Ahead at the Bar: 4 min.
		{ID: "b1", OrderID: "B", PrepAreaID: "bar", PrepTimeSeconds: 4 * min, ConfirmedAt: 2},
		// Our Order X: ramen 12 min + gyoza 10 min in the Kitchen, latte 4 min at the Bar.
		{ID: "x1", OrderID: "X", PrepAreaID: "kitchen", PrepTimeSeconds: 12 * min, ConfirmedAt: 3},
		{ID: "x2", OrderID: "X", PrepAreaID: "kitchen", PrepTimeSeconds: 10 * min, ConfirmedAt: 3},
		{ID: "x3", OrderID: "X", PrepAreaID: "bar", PrepTimeSeconds: 4 * min, ConfirmedAt: 3},
		// Behind us: never counted.
		{ID: "y1", OrderID: "Y", PrepAreaID: "kitchen", PrepTimeSeconds: 30 * min, ConfirmedAt: 4},
	}
	// Kitchen: (4 + 10) / 2 + 12 = 19 min. Bar: 4 / 1 + 4 = 8 min. Max = 19 min.
	got, ok := kitchen.EstimatedWait(queue, "X", capacity, now)
	if !ok || got != 19*min {
		t.Fatalf("wait = %d s (%v), want %d", got, ok, 19*min)
	}

	// A Remake jumps the queue, even though it was confirmed after us.
	withRemake := append(queue, kitchen.QueueItem{ID: "r1", OrderID: "R", PrepAreaID: "kitchen", PrepTimeSeconds: 6 * min, ConfirmedAt: 9, Remake: true})
	if got, _ := kitchen.EstimatedWait(withRemake, "X", capacity, now); got != 22*min {
		t.Fatalf("with remake = %d s, want %d", got, 22*min)
	}

	// An overdue Preparing item counts 0, not negative.
	late := []kitchen.QueueItem{
		{ID: "l1", OrderID: "L", PrepAreaID: "bar", PrepTimeSeconds: 4 * min, Preparing: true, PreparingAt: now - 9*min*1000, ConfirmedAt: 1},
		{ID: "x3", OrderID: "X", PrepAreaID: "bar", PrepTimeSeconds: 4 * min, ConfirmedAt: 3},
	}
	if got, _ := kitchen.EstimatedWait(late, "X", capacity, now); got != 4*min {
		t.Fatalf("late = %d s, want %d", got, 4*min)
	}

	if _, ok := kitchen.EstimatedWait(queue, "pending-order", capacity, now); ok {
		t.Fatal("an Order with nothing queued has no Estimated Wait")
	}
}
