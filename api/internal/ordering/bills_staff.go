package ordering

import (
	"database/sql"
	"errors"
	"time"

	"github.com/morpikssolutionsDev/Shusi_latte/api/internal/pricing"
)

// BillSummary is the staff view of a Bill (BR-WT-07). Totals
// use the pricing engine with no discounts yet in Phase 4;
// Phase 6 adds applied discounts and payments to this view.
type BillSummary struct {
	Bill          Bill       `json:"bill"`
	TableIDs      []string   `json:"table_ids"`
	TableLabels   []string   `json:"table_labels"`
	Customers     []Customer `json:"customers"`
	Orders        []Order    `json:"orders"`
	Subtotal      int64      `json:"subtotal"`
	ServiceCharge int64      `json:"service_charge"`
	Tax           int64      `json:"tax"`
	Total         int64      `json:"total"`
	SubBills      []SubBill  `json:"sub_bills"`
}

type SubBill struct {
	ID         string `json:"id"`
	BillID     string `json:"bill_id"`
	Method     string `json:"method"`
	ShareIndex int    `json:"share_index"`
	ShareCount int    `json:"share_count"`
	Status     string `json:"status"`
}

func (s *Store) BillSummary(billID string, pricingStore *pricing.Store) (BillSummary, error) {
	bill, err := s.loadBill(billID)
	if err != nil {
		return BillSummary{}, err
	}
	rows, err := s.db.Query(`SELECT bt.table_id, dt.label FROM bill_table bt JOIN dining_table dt ON dt.id = bt.table_id WHERE bt.bill_id = ?`, billID)
	tableIDs := []string{}
	labels := []string{}
	if err == nil {
		for rows.Next() {
			var tid, label string
			if err := rows.Scan(&tid, &label); err == nil {
				tableIDs = append(tableIDs, tid)
				labels = append(labels, label)
			}
		}
		rows.Close()
	}
	customers := s.loadCustomersForBill(billID)
	orows, err := s.db.Query(`SELECT id FROM orders WHERE bill_id = ? ORDER BY created_at`, billID)
	orders := []Order{}
	if err == nil {
		var ids []string
		for orows.Next() {
			var id string
			if err := orows.Scan(&id); err == nil {
				ids = append(ids, id)
			}
		}
		orows.Close()
		for _, id := range ids {
			if o, err := s.loadOrder(id); err == nil {
				orders = append(orders, o)
			}
		}
	}
	var inputs []pricing.ItemInput
	for _, o := range orders {
		for _, it := range o.Items {
			if it.Status == "cancelled" || it.Status == "voided" {
				continue
			}
			modPrices := []int64{}
			for _, m := range it.Modifiers {
				modPrices = append(modPrices, m.Price)
			}
			inputs = append(inputs, pricing.ItemInput{VariantPrice: it.UnitPrice, ModifierPrices: modPrices, Quantity: it.Quantity})
		}
	}
	settings, err := pricingStore.GetSettings()
	if err != nil {
		settings = pricing.DefaultSettings()
	}
	breakdown, err := pricing.Calculate(inputs, nil, settings, "")
	if err != nil {
		return BillSummary{}, err
	}
	srows, err := s.db.Query(`SELECT id, bill_id, method, share_index, share_count, status FROM sub_bill WHERE bill_id = ? ORDER BY share_index`, billID)
	subs := []SubBill{}
	if err == nil {
		for srows.Next() {
			var sb SubBill
			if err := srows.Scan(&sb.ID, &sb.BillID, &sb.Method, &sb.ShareIndex, &sb.ShareCount, &sb.Status); err == nil {
				subs = append(subs, sb)
			}
		}
		srows.Close()
	}
	return BillSummary{Bill: bill, TableIDs: tableIDs, TableLabels: labels, Customers: customers, Orders: orders, Subtotal: breakdown.Subtotal, ServiceCharge: breakdown.ServiceCharge, Tax: breakdown.Tax, Total: breakdown.Total, SubBills: subs}, nil
}

// SettleBill puts a Bill into Settling (BR-WT-15, BR-R-27).
func (s *Store) SettleBill(crewID, billID string) (Bill, error) {
	res, err := s.db.Exec(`UPDATE bill SET status = 'settling' WHERE id = ? AND status = 'open'`, billID)
	if err != nil {
		return Bill{}, err
	}
	if n, _ := res.RowsAffected(); n != 1 {
		return Bill{}, errors.New("only an open bill can be settled")
	}
	s.audit(crewID, "settle", "bill", billID, "")
	return s.loadBill(billID)
}

// UnlockBill returns a Settling Bill to open.
func (s *Store) UnlockBill(crewID, billID string) (Bill, error) {
	res, err := s.db.Exec(`UPDATE bill SET status = 'open' WHERE id = ? AND status = 'settling'`, billID)
	if err != nil {
		return Bill{}, err
	}
	if n, _ := res.RowsAffected(); n != 1 {
		return Bill{}, errors.New("only a settling bill can be unlocked")
	}
	s.audit(crewID, "unlock", "bill", billID, "")
	return s.loadBill(billID)
}

// MoveBill moves a Bill to another Table (BR-WT-06, BR-R-28).
func (s *Store) MoveBill(crewID, billID, tableID string) (Bill, error) {
	var existing string
	if err := s.db.QueryRow(`SELECT bill_id FROM bill_table WHERE table_id = ? AND is_open = 1`, tableID).Scan(&existing); err == nil {
		return Bill{}, errors.New("target table already has an open bill; merge instead")
	}
	tx, err := s.db.Begin()
	if err != nil {
		return Bill{}, err
	}
	defer tx.Rollback()
	if _, err := tx.Exec(`UPDATE bill_table SET is_open = 0 WHERE bill_id = ? AND is_open = 1`, billID); err != nil {
		return Bill{}, err
	}
	if _, err := tx.Exec(`INSERT INTO bill_table (bill_id, table_id, is_open) VALUES (?, ?, 1)`, billID, tableID); err != nil {
		return Bill{}, err
	}
	if err := tx.Commit(); err != nil {
		return Bill{}, err
	}
	s.audit(crewID, "move", "bill", billID, "to table "+tableID)
	return s.loadBill(billID)
}

// MergeBills merges absorbed Bills into a surviving Bill
// (BR-WT-06, BR-R-03, TR-DM-03). There is no unmerge.
func (s *Store) MergeBills(crewID, survivingID string, absorbedIDs []string) (Bill, error) {
	tx, err := s.db.Begin()
	if err != nil {
		return Bill{}, err
	}
	defer tx.Rollback()
	for _, absorbed := range absorbedIDs {
		if absorbed == survivingID {
			return Bill{}, errors.New("cannot merge a bill into itself")
		}
		// Tables: the survivor takes over each absorbed Table,
		// unless it already covers that Table.
		trows, err := tx.Query(`SELECT table_id FROM bill_table WHERE bill_id = ? AND is_open = 1`, absorbed)
		if err != nil {
			return Bill{}, err
		}
		var tableIDs []string
		for trows.Next() {
			var tid string
			if err := trows.Scan(&tid); err == nil {
				tableIDs = append(tableIDs, tid)
			}
		}
		trows.Close()
		for _, tid := range tableIDs {
			if _, err := tx.Exec(`DELETE FROM bill_table WHERE bill_id = ? AND table_id = ?`, absorbed, tid); err != nil {
				return Bill{}, err
			}
			var exists int
			_ = tx.QueryRow(`SELECT COUNT(*) FROM bill_table WHERE bill_id = ? AND table_id = ? AND is_open = 1`, survivingID, tid).Scan(&exists)
			if exists == 0 {
				if _, err := tx.Exec(`INSERT INTO bill_table (bill_id, table_id, is_open) VALUES (?, ?, 1)`, survivingID, tid); err != nil {
					return Bill{}, err
				}
			}
		}
		if _, err := tx.Exec(`UPDATE orders SET bill_id = ? WHERE bill_id = ?`, survivingID, absorbed); err != nil {
			return Bill{}, err
		}
		if _, err := tx.Exec(`UPDATE customer_entry SET bill_id = ? WHERE bill_id = ?`, survivingID, absorbed); err != nil {
			return Bill{}, err
		}
		if _, err := tx.Exec(`UPDATE bill SET status = 'merged' WHERE id = ?`, absorbed); err != nil {
			return Bill{}, err
		}
	}
	if err := tx.Commit(); err != nil {
		return Bill{}, err
	}
	s.audit(crewID, "merge", "bill", survivingID, "")
	return s.loadBill(survivingID)
}

// SplitBill creates Sub-bills (BR-WT-06, BR-R-04). By-item
// assigns the given order items to sub-bills; equal creates
// the requested number of shares without item assignment.
func (s *Store) SplitBill(crewID, billID, method string, shareCount int, assignments map[string][]string) ([]SubBill, error) {
	if method != "by_item" && method != "equal" {
		return nil, errors.New("split method must be by_item or equal")
	}
	if shareCount < 2 {
		return nil, errors.New("split requires at least 2 shares")
	}
	tx, err := s.db.Begin()
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()
	subs := []SubBill{}
	for i := 0; i < shareCount; i++ {
		sb := SubBill{ID: NewID(), BillID: billID, Method: method, ShareIndex: i, ShareCount: shareCount, Status: "open"}
		if _, err := tx.Exec(`INSERT INTO sub_bill (id, bill_id, method, share_index, share_count, status) VALUES (?, ?, ?, ?, ?, 'open')`, sb.ID, billID, method, i, shareCount); err != nil {
			return nil, err
		}
		subs = append(subs, sb)
	}
	if method == "by_item" {
		for subID, itemIDs := range assignments {
			for _, itemID := range itemIDs {
				if _, err := tx.Exec(`UPDATE order_item SET sub_bill_id = ? WHERE id = ? AND order_id IN (SELECT id FROM orders WHERE bill_id = ?)`, subID, itemID, billID); err != nil {
					return nil, err
				}
			}
		}
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	s.audit(crewID, "split", "bill", billID, method)
	return subs, nil
}

// ReprintReceipt records a reprint (BR-WT-13, TR-PRN-08).
func (s *Store) ReprintReceipt(crewID, billID string) error {
	if _, err := s.db.Exec(`INSERT INTO receipt_print (id, bill_id, copy_type, is_reprint, printed_by, printed_at) VALUES (?, ?, 'customer', 1, ?, ?)`, NewID(), billID, crewID, time.Now().UnixMilli()); err != nil {
		return err
	}
	s.audit(crewID, "reprint", "bill", billID, "REPRINT")
	return nil
}

var _ = sql.ErrNoRows
