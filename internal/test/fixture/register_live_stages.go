// Design: docs/functional-tests.md -- compiled fixtures a .ci drives

package fixture

// The staged reload driver of spec-bgp-live-receive-binding-and-bfd-keep-session.
// Its body and its script grammar are reload_fixture_live_stages.go. It runs on
// Observe, which dispatches `request shutdown` after the script, so it
// declares StopsDaemon.
func init() {
	Register("reload/live-stages", liveStagesDriver, StopsDaemon)
}
