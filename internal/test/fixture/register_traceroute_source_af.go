package fixture

// register_traceroute_source_af.go names the scenario that proves the source
// address of `resolve traceroute` decides the family the target resolves in.
//
// Related: plugin_fixture_traceroute_source_af.go -- the scenario body
// Related: test/plugin/traceroute-source-af.ci -- the test that runs it

func init() {
	Register("plugin/traceroute-source-af", tracerouteSourceAF)
}
