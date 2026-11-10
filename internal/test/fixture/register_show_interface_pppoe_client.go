// Design: docs/guide/pppoe.md -- `show interface pppoe-client`
// Related: show_interface_pppoe_client_fixture.go -- the scenario registered below

package fixture

func init() {
	// The client's source interface is the static scenarios' setup link
	// (static/static-kernel-setup), where no access concentrator answers.
	Register("plugin/show-interface-pppoe-client",
		observer02("show-interface-pppoe-client-test", showInterfacePPPoEClient), StopsDaemon)
}
