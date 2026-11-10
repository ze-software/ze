// Design: docs/guide/pppoe.md -- `show interface delegated-prefix`
// Related: show_interface_delegated_prefix_fixture.go -- the scenario registered below

package fixture

func init() {
	// The pppoe-client's source interface is the static scenarios' setup link
	// (static/static-kernel-setup), where no access concentrator answers, so
	// no lease ever delegates a prefix.
	Register("plugin/show-interface-delegated-prefix",
		observer02("show-interface-delegated-prefix-test", showInterfaceDelegatedPrefix), StopsDaemon)
}
