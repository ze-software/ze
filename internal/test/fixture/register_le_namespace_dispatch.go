package fixture

// register_le_namespace_dispatch.go registers the le namespace dispatch scenario.
//
// The registration is here rather than beside the driver because the native
// pretool-writeedit check refuses Register inside init() in any file whose name
// does not start with "register" (internal/le/hookruntime/writeedit.go).
//
// Related: ui_fixture_le_namespace_dispatch.go -- the driver.
func init() {
	Register("ui/le-namespace-dispatch", uiDriver(leNamespaceDispatch))
}
