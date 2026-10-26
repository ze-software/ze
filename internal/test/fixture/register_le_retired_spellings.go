package fixture

// register_le_retired_spellings.go registers the le retired spellings scenario.
//
// The registration is here rather than beside the driver because the native
// pretool-writeedit check refuses Register inside init() in any file whose name
// does not start with "register" (internal/le/hookruntime/writeedit.go).
//
// Related: ui_fixture_le_retired_spellings.go -- the driver.
func init() {
	Register("ui/le-retired-spellings", uiDriver(leRetiredSpellings))
}
