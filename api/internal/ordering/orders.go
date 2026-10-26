package ordering

import (
	"errors"
	"math"
)

// Order and OrderItem types for Phase 2b, following TRD §4.3.
// An Order belongs to one Bill and one Customer; an Order Item
// snapshots Product/Variant/Modifier names and prices at order
// time (TR-DM-02), so later catalog changes never alter it.
// Status values follow TR-SM-01: pending until a Waiter Claims
// and confirms (Phase 4), then confirmed, preparing, ready,
// served, cancelled or voided.

type OrderItemModifier struct {
	ModifierID   string `json:"modifier_id"`
	NameSnapshot string `json:"name_snapshot"`
	Price        int64  `json:"price"`
}

type DietaryWarning struct {
	DietaryTagID string `json:"dietary_tag_id"`
	TagNameID    string `json:"tag_name_id"`
	TagNameEN    string `json:"tag_name_en"`
	Acknowledged bool   `json:"acknowledged"`
}

type OrderItem struct {
	ID              string              `json:"id"`
	OrderID         string              `json:"order_id"`
	ProductID       string              `json:"product_id"`
	VariantID       string              `json:"variant_id"`
	ProductName     string              `json:"product_name_snapshot"`
	VariantName     string              `json:"variant_name_snapshot"`
	Quantity        int                 `json:"quantity"`
	UnitPrice       int64               `json:"unit_price"`
	Note            *string             `json:"note"`
	Status          string              `json:"status"`
	Modifiers       []OrderItemModifier `json:"modifiers"`
	DietaryWarnings []DietaryWarning    `json:"dietary_warnings"`
	PrepTimeSeconds int                 `json:"prep_time_seconds"`
	LineTotal       int64               `json:"line_total"`
}

func (it OrderItem) computeLineTotal() int64 {
	unit := it.UnitPrice
	for _, m := range it.Modifiers {
		unit += m.Price
	}
	return unit * int64(it.Quantity)
}

type Order struct {
	ID                   string      `json:"id"`
	BillID               string      `json:"bill_id"`
	CustomerEntryID      *string     `json:"customer_entry_id"`
	PlacedBy             string      `json:"placed_by"`
	Source               string      `json:"source"`
	Status               string      `json:"status"`
	ClaimedBy            *string     `json:"claimed_by"`
	ClaimedAt            *int64      `json:"claimed_at"`
	ConfirmedAt          *int64      `json:"confirmed_at"`
	CreatedAt            int64       `json:"created_at"`
	Items                []OrderItem `json:"items"`
	EstimatedWaitMinutes int         `json:"estimated_wait_minutes"`
}

// EstimatedWaitMinutes implements TR-KT-04 for the Client Menu:
// the longest Prep Time among non-cancelled, non-voided items,
// rounded up to the next minute. Zero when nothing is outstanding.
func (o Order) estimatedWait() int {
	max := 0
	for _, it := range o.Items {
		if it.Status == "cancelled" || it.Status == "voided" {
			continue
		}
		if it.PrepTimeSeconds > max {
			max = it.PrepTimeSeconds
		}
	}
	if max == 0 {
		return 0
	}
	return int(math.Ceil(float64(max) / 60.0))
}

type OrderItemInput struct {
	ProductID   string   `json:"product_id"`
	VariantID   string   `json:"variant_id"`
	ModifierIDs []string `json:"modifier_ids"`
	Quantity    int      `json:"quantity"`
	Note        *string  `json:"note"`
}

type CreateOrderInput struct {
	Items               []OrderItemInput `json:"items"`
	AcknowledgeWarnings bool             `json:"acknowledge_warnings"`
}

// WarningError is returned when an Order contains Dietary
// Warnings the Customer has not acknowledged (BR-CM-11). The
// handler turns it into a 409 carrying the warnings, so the
// Client Menu can ask for acknowledgement and resubmit.
type WarningError struct {
	Warnings map[string][]DietaryWarning // item index key -> warnings
}

func (e *WarningError) Error() string {
	return "dietary warnings require acknowledgement"
}

var (
	ErrOrderNotFound   = errors.New("order not found")
	ErrOrderNotPending = errors.New("order is not pending")
	ErrOrderClaimed    = errors.New("order has been claimed; ask the waiter to change it")
	ErrBillSettling    = errors.New("bill is being settled — please ask the waiter")
	ErrBillClosed      = errors.New("bill is closed")
	ErrNotOrderOwner   = errors.New("order belongs to another customer")
)
