// Design: docs/architecture/fleet-config.md -- the registry of managed devices
// Related: misc_fixture_managed_fleet_registry.go -- registry checks through the hub CLI

package fixture

func init() {
	Register("managed/fleet-devices-connect", managedFleetDevicesConnectDriver)
	Register("managed/fleet-devices-restart", managedFleetDevicesRestartDriver)
	Register("managed/fleet-device-detail", managedFleetDeviceDetailDriver)
	Register("managed/fleet-devices-label", managedFleetDevicesLabelDriver)
	Register("managed/fleet-devices-declared", managedFleetDevicesDeclaredDriver)
	Register("managed/fleet-devices-diverged", managedFleetDevicesDivergedDriver)
	Register("managed/fleet-devices-reload-add", managedFleetDevicesReloadAddDriver)
	Register("managed/fleet-devices-reload-remove", managedFleetDevicesReloadRemoveDriver)
	Register("managed/fleet-reload-listener-verified", managedFleetReloadListenerVerifiedDriver)
	Register("managed/fleet-reload-certificate-rotated", managedFleetReloadCertificateRotatedDriver)
	Register("managed/fleet-certificate-refused-doctor", managedFleetCertificateRefusedDoctorDriver)
}
