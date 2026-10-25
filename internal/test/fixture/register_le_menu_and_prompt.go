package fixture

// register_le_menu_and_prompt.go registers the le menu and prompt scenario.
//
// The registration is here rather than beside the driver because the native
// pretool-writeedit check refuses Register inside init() in any file whose name
// does not start with "register" (internal/le/hookruntime/writeedit.go).
//
// Related: ui_fixture_le_menu_and_prompt.go -- the driver.
func init() {
	Register("ui/le-menu-and-prompt", uiDriver(leMenuAndPrompt))
}
