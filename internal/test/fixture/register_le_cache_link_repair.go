package fixture

// register_le_cache_link_repair.go registers the le cache link repair scenario.
//
// The registration is here rather than beside the driver because the native
// pretool-writeedit check refuses Register inside init() in any file whose name
// does not start with "register" (internal/le/hookruntime/writeedit.go).
//
// Related: ui_fixture_le_cache_link_repair.go -- the driver.
func init() {
	Register("ui/le-cache-link-repair", uiDriver(leCacheLinkRepair))
}
