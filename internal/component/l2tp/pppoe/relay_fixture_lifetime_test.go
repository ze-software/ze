// Design: docs/architecture/l2tp/followup-l2tp-call.md -- relay call ownership

package pppoe

// fixtureServiceCall supplies the call token for existing binding-selection
// tests, which own no actual L2TP call. Lifecycle tests use counted call owners.
type fixtureServiceCall struct{}

func (fixtureServiceCall) Cancel() {}
