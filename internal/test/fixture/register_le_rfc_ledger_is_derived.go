package fixture

// register_le_rfc_ledger_is_derived.go registers the RFC ledger scenario.
//
// The registration is here rather than beside the driver because the native
// pretool-writeedit check refuses Register inside init() in any file whose name
// does not start with "register" (internal/le/hookruntime/writeedit.go).
//
// Related: misc_fixture_runner_rfcledger.go -- the driver.
func init() {
	Register("runner/le-rfc-ledger-is-derived", leRFCLedgerIsDerivedDriver)
}
