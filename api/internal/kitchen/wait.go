// Package kitchen owns the Prep Area queues and the Estimated Wait (TRD §11).
package kitchen

import "sort"

// QueueItem is one Confirmed or Preparing Order Item in a Prep Area queue.
type QueueItem struct {
	ID              string
	OrderID         string
	PrepAreaID      string
	PrepTimeSeconds int64
	Preparing       bool
	PreparingAt     int64 // ms, when Preparing
	ConfirmedAt     int64 // ms, the Order's confirmation
	Remake          bool
}

// Remaining is the prep time still to go: the full Prep Time until Preparing starts, then
// max(Prep Time − elapsed, 0) (TR-KT-03).
func (q QueueItem) Remaining(nowMs int64) int64 {
	if !q.Preparing {
		return q.PrepTimeSeconds
	}
	return max(q.PrepTimeSeconds-(nowMs-q.PreparingAt)/1000, 0)
}

// before is the queue order of TR-KT-01: Remakes first, then by confirmation time.
func before(a, b QueueItem) bool {
	if a.Remake != b.Remake {
		return a.Remake
	}
	if a.ConfirmedAt != b.ConfirmedAt {
		return a.ConfirmedAt < b.ConfirmedAt
	}
	return a.ID < b.ID
}

// SortQueue orders items as the Prep Area display shows them.
func SortQueue(items []QueueItem) {
	sort.Slice(items, func(i, j int) bool { return before(items[i], items[j]) })
}

// EstimatedWait is TR-KT-03, in seconds: the maximum over the Order's Prep Areas of
// (Σ remaining time of items ahead ÷ parallel capacity) + the longest remaining time among
// the Order's own items there. ok is false when the Order has nothing in any queue (still
// Pending, or everything already Ready).
func EstimatedWait(queue []QueueItem, orderID string, capacity map[string]int64, nowMs int64) (seconds int64, ok bool) {
	own := map[string][]QueueItem{}
	for _, q := range queue {
		if q.OrderID == orderID {
			own[q.PrepAreaID] = append(own[q.PrepAreaID], q)
		}
	}
	for area, mine := range own {
		first := mine[0]
		longest := int64(0)
		for _, m := range mine {
			longest = max(longest, m.Remaining(nowMs))
			if before(m, first) {
				first = m
			}
		}
		var ahead int64
		for _, q := range queue {
			if q.PrepAreaID == area && q.OrderID != orderID && before(q, first) {
				ahead += q.Remaining(nowMs)
			}
		}
		c := max(capacity[area], 1)
		seconds = max(seconds, (ahead+c-1)/c+longest)
		ok = true
	}
	return seconds, ok
}
