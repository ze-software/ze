// Design: docs/architecture/fleet-interface.md -- what the registries of the fleet package share
// Related: tabs.go, filter.go, table.go, tree.go -- the registries
//
// Each registry of this package is filled from init() in a register file. A
// registration it cannot accept is a defect of the build's composition, found
// at process start, so each one panics, through the one function below.

package fleetui

import "strconv"

// The reasons a registration is refused that more than one registry gives.
const (
	bugUnnamed = "has no name"
	bugTwice   = "is registered twice"
)

// registrationBug panics for a registration only a defective build makes.
// what says what was registered, such as "the filter field", name is its
// name, subject is the table or the set it joins, and reason completes the
// sentence.
func registrationBug(what, name, subject, reason string) {
	panic("BUG: fleetui: " + what + " " + strconv.Quote(name) + " of " + subject + " " + reason)
}
