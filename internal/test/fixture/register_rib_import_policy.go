// Design: docs/guide/route-policy.md -- import chains hide paths from the Loc-RIB, and show rib candidates lists them
// Related: rib_import_policy_fixture.go -- the two scenarios registered below

package fixture

func init() {
	// The central chain judges every source, and a source chain judges its own.
	// Both scenarios read the hidden paths through `show rib candidates`, which
	// is the only surface that lists a path the Loc-RIB stored but never selects.
	Register("plugin/rib-import-central-bogons",
		observer02("rib-import-test", ribImportCentralBogons))
	Register("plugin/static-import-filter-distance",
		observer02("rib-import-test", staticImportFilterDistance))
}
