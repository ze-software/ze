// Design: docs/architecture/doctor-and-health-checks.md -- the managed-diverged check through ze doctor, daemon stopped and running

package fixture

// The real-daemon driver for `ze doctor` on a managed device whose
// configuration is not the last one its hub sent. Its body is
// misc_fixture_managed_fleet_doctor.go.
func init() {
	Register("managed/fleet-diverged-doctor", managedFleetDivergedDoctorDriver)
}
