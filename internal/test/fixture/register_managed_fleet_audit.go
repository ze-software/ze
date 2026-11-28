// Design: docs/architecture/fleet-config.md -- centralized hub-side fleet audit log functional fixture registration
// Related: misc_fixture_managed_fleet_audit.go -- managedFleetAuditLogDriver

package fixture

func init() {
	Register("managed/fleet-audit-log", managedFleetAuditLogDriver)
}
