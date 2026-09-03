package fixture

// register_resolve_rir.go registers the two RIR delegation scenarios.
//
// The registration is here rather than beside the drivers because the native
// pretool-writeedit check refuses Register inside init() in any file whose name
// does not start with "register" (internal/le/hookruntime/writeedit.go).
//
// Related: plugin_fixture_resolve_rir.go -- the two drivers.
func init() {
	Register("plugin/resolve-rir-lookup", resolveRIRLookup)
	Register("plugin/resolve-rir-refresh", resolveRIRRefresh)
}
