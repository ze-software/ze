// Design: docs/architecture/update-building.md -- the third-party Link-Local an RFC 2545 next hop carries
// Overview: plugin_fixture_rfc2545_third_party_linux.go -- the setup and teardown this file names

//go:build linux

package fixture

// The fixtures build a veth link and seed neighbor entries, which only Linux
// has. test/plugin/rfc2545-third-party-linklocal.ci declares needs-linux, so
// no other host names them.
func init() {
	Register("plugin/rfc2545-third-party-linklocal-setup", rfc2545ThirdPartySetup)
	Register("plugin/rfc2545-third-party-linklocal-teardown", rfc2545ThirdPartyTeardown)
}
