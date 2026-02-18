// Package route implements route dynamics scheduling and action types
// for the ze-chaos testing tool. Route dynamics represent normal internet
// behavior (churn, withdrawals) as opposed to chaos (session disruption).
package route

import "fmt"

// ActionType identifies the kind of route dynamics event.
type ActionType int

const (
	// ActionChurn withdraws a small set of routes and re-announces them
	// after a brief delay, simulating normal internet route churn.
	ActionChurn ActionType = iota
	// ActionPartialWithdraw withdraws a random fraction of routes (stays connected).
	ActionPartialWithdraw
	// ActionFullWithdraw withdraws all routes (stays connected).
	ActionFullWithdraw
)

// String returns the kebab-case name of the action type.
func (a ActionType) String() string {
	switch a {
	case ActionChurn:
		return "churn"
	case ActionPartialWithdraw:
		return "partial-withdraw"
	case ActionFullWithdraw:
		return "full-withdraw"
	default:
		return fmt.Sprintf("unknown(%d)", a)
	}
}

// ActionTypeFromString parses a kebab-case action name into an ActionType.
// Returns the zero value and false if the name is unknown.
func ActionTypeFromString(s string) (ActionType, bool) {
	switch s {
	case "churn":
		return ActionChurn, true
	case "partial-withdraw":
		return ActionPartialWithdraw, true
	case "full-withdraw":
		return ActionFullWithdraw, true
	default:
		return 0, false
	}
}

// Action describes a route dynamics event to execute on a peer.
type Action struct {
	// Type identifies what kind of route action to perform.
	Type ActionType

	// WithdrawFraction is the fraction of routes to withdraw (0.0-1.0).
	// Used for ActionPartialWithdraw.
	WithdrawFraction float64

	// ChurnCount is the number of routes to churn (withdraw + re-announce).
	// Used for ActionChurn.
	ChurnCount int
}
