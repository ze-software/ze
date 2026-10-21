package fixture

// register_verify_first_red.go registers the first-red verification scenarios.
//
// The registration is here rather than beside the driver because the native
// pretool-writeedit check refuses Register inside init() in any file whose name
// does not start with "register" (internal/le/hookruntime/writeedit.go).
//
// Related: misc_fixture_runner_first_red.go -- the drivers.
func init() {
	Register("runner/verify-first-red-refuses-a-cut", verifyFirstRedRefusesACutDriver)
	Register("runner/verify-first-red-stops", verifyFirstRedStopsDriver)
}
