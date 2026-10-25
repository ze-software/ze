package fixture

// register_le_help_pages.go registers the le help pages scenario.
//
// The registration is here rather than beside the driver because the native
// pretool-writeedit check refuses Register inside init() in any file whose name
// does not start with "register" (internal/le/hookruntime/writeedit.go).
//
// Related: ui_fixture_le_help_pages.go -- the driver.
func init() {
	Register("ui/le-help-pages", uiDriver(leHelpPages))
}
