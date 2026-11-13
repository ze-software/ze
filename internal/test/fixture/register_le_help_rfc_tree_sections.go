package fixture

// register_le_help_rfc_tree_sections.go registers the le help rfc tree
// section-words scenario.
//
// The registration is here rather than beside the driver because the native
// pretool-writeedit check refuses Register inside init() in any file whose name
// does not start with "register" (internal/le/hookruntime/writeedit.go).
//
// Related: ui_fixture_le_help_rfc_tree_sections.go -- the driver body.
func init() {
	Register("ui/le-help-rfc-tree-sections", uiDriver(leRFCPartialDriver("tree-sections", leHelpRFCTreeSections)))
}
