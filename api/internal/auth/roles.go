package auth

import "slices"

// Roles (BRD §5).
const (
	RoleManagement  = "management"
	RoleOutletAdmin = "outlet_admin"
	RoleCrew        = "crew"
)

// Stations a Crew member may work (Glossary: Station).
const (
	StationWaiter  = "waiter"
	StationCashier = "cashier"
	StationKitchen = "kitchen"
)

// Screens that run in screen mode (TR-AUTH-08).
const (
	ScreenKitchen = "kitchen"
	ScreenBar     = "bar"
	ScreenDisplay = "display"
	ScreenHRIS    = "hris"
)

// Override actions and the roles allowed to approve each one (TR-AUTH-06, BRD §5.1).
const (
	ActionManualDiscount = "manual_discount"
	ActionVoid           = "void"
	ActionRemake         = "remake"
	ActionUnpaidBill     = "unpaid_bill"
	ActionQRISUnverified = "qris_unverified"
	ActionRefundInCap    = "refund_within_cap"
	ActionRefundAboveCap = "refund_above_cap"
	ActionDemoReset      = "demo_reset"
)

var overrideApprovers = map[string][]string{
	ActionManualDiscount: {RoleManagement},
	ActionVoid:           {RoleManagement, RoleOutletAdmin},
	ActionRemake:         {RoleManagement, RoleOutletAdmin},
	ActionUnpaidBill:     {RoleManagement, RoleOutletAdmin},
	ActionQRISUnverified: {RoleManagement, RoleOutletAdmin},
	ActionRefundInCap:    {RoleManagement, RoleOutletAdmin},
	ActionRefundAboveCap: {RoleManagement},
	ActionDemoReset:      {RoleManagement},
}

// KnownAction reports whether action is a defined PIN Override action.
func KnownAction(action string) bool {
	_, ok := overrideApprovers[action]
	return ok
}

// CanApprove reports whether a role may approve action with a PIN Override.
func CanApprove(role, action string) bool {
	return slices.Contains(overrideApprovers[action], role)
}
