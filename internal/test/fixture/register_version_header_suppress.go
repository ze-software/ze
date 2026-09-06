package fixture

// register_version_header_suppress.go names the scenario that proves the
// `hide-version` config leaf keeps the X-Ze-Version build banner off both HTTP
// servers, and that an operator who writes nothing still gets it.
//
// Related: plugin_fixture_version_header_suppress.go -- the scenario body
// Related: test/plugin/version-header-suppress.ci -- the test that runs it

func init() {
	Register("plugin/version-header-suppress", versionHeaderSuppress)
}
