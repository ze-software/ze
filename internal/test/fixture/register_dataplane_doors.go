// Design: docs/guide/dataplane-features.md -- registers compiled fixtures for AC-7 and AC-18 data-plane door tests.
// Related: dataplane_door_fixtures.go -- implementations of the registered fixture drivers.

package fixture

func init() {
	Register("plugin/dataplane-door-api", dpDoorAPIDriver, StopsDaemon)
	Register("plugin/dataplane-door-gnmi", dpDoorGNMIDriver, StopsDaemon)
	Register("plugin/dataplane-door-upload", dpDoorUploadDriver, StopsDaemon)
	Register("plugin/dataplane-door-ephemeral-editor", dpDoorEphemeralEditorDriver)
	Register("managed/dataplane-door-pushed-config", dpDoorManagedPushedDriver)
	Register("static/named-table-follows-data-plane", dpDoorStaticDataPlaneDriver, StopsDaemon)
}
