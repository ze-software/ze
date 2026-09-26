// Design: docs/architecture/testing/ci-format.md -- tree storage functional scenarios
// Related: storage_live_fixture.go -- backup and restore against a running daemon

package fixture

func init() {
	Register("storage/live-init", storageLiveInit)
	Register("storage/data-backup-live", storageLiveBackup)
	Register("storage/data-backup-refused-live", storageLiveBackupRefused)
	Register("storage/data-restore-full-refused-live", storageLiveRestoreFullRefused)
	Register("storage/data-restore-config-live", storageLiveRestoreConfig)
	Register("storage/data-restore-client-live", storageLiveRestoreClient)
}
