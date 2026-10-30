// Design: docs/architecture/update-building.md -- the third-party Link-Local an RFC 2545 next hop carries
// Overview: plugin_fixture_rfc2545_third_party_linux.go -- the driver this file names

//go:build linux

package fixture

// The driver builds network namespaces and seeds neighbor entries, which only
// Linux has. test/plugin/rfc2545-third-party-linklocal.ci declares
// needs-linux, so no other host names it.
func init() {
	Register("plugin/rfc2545-third-party-linklocal", rfc2545ThirdPartyDriver)
}
