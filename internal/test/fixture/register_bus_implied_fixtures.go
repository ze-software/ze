// Design: docs/architecture/api/architecture.md -- Phase 7 implied display, delivery origin, and lifecycle reception fixture registrations.
// Related: bus_implied_fixtures.go -- driver implementations registered below.

package fixture

func init() {
	Register("plugin/bus-implied-probe", busImpliedProbeDriver)
	Register("plugin/bus-open-validator", busOpenValidatorDriver)
	Register("plugin/bus-assert-implied", busAssertImpliedDriver)
}
