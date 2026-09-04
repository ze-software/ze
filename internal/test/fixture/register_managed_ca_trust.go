// Design: docs/architecture/fleet-config.md -- how a managed client authenticates its hub

package fixture

// The two-daemon AC-12 driver and the export plugin the hub runs. Their bodies,
// and why the hub is started twice, are misc_fixture_managed_ca_trust.go.
func init() {
	Register("managed/hub-ca-trust", managedHubCATrustDriver)
	Register("managed/hub-ca-export", managedHubCAExportDriver)
}
