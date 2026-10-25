package fixture

// register_le_exec_cache_env.go registers the le exec cache environment
// scenario.
//
// The registration is here rather than beside the driver because the native
// pretool-writeedit check refuses Register inside init() in any file whose name
// does not start with "register" (internal/le/hookruntime/writeedit.go).
//
// Related: ui_fixture_le_exec_cache_env.go -- the driver.
func init() {
	Register("ui/le-exec-cache-env", uiDriver(leExecCacheEnv))
}
