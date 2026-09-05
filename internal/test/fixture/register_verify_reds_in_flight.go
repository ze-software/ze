package fixture

// register_verify_reds_in_flight.go registers the in-flight verification query
// scenario.
//
// The registration is here rather than beside the driver because the native
// pretool-writeedit check refuses Register inside init() in any file whose name
// does not start with "register" (internal/le/hookruntime/writeedit.go).
//
// Related: misc_fixture_runner_reds.go -- the driver.
func init() {
	Register("runner/verify-reds-in-flight", verifyRedsInFlightDriver)
}
