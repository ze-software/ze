package fixture

// register_ze_help_ai_unchanged.go registers the ze help ai golden scenario.
//
// The registration is here rather than beside the driver because the native
// pretool-writeedit check refuses Register inside init() in any file whose name
// does not start with "register" (internal/le/hookruntime/writeedit.go).
//
// Related: ui_fixture_ze_help_ai_unchanged.go -- the driver.
func init() {
	Register("ui/ze-help-ai-unchanged", uiDriver(zeHelpAiUnchanged))
}
