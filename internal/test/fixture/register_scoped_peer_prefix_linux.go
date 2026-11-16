//go:build linux

// Design: docs/functional-tests.md -- causal namespace fixtures for BGP.
// Related: plugin_fixture_scoped_peer_prefix_linux.go -- topology owner.
package fixture

func init() {
	Register("plugin/scoped-peer-prefix-refresh", scopedPrefixDriver)
	Register("plugin/scoped-peer-prefix-status", scopedPrefixStatusDriver, StopsDaemon)
}
