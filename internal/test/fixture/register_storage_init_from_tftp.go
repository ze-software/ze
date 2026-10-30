//go:build ze_pxe

// Design: docs/architecture/testing/ci-format.md -- tree storage functional scenarios
// Related: storage_init_from_tftp_fixture.go -- the ze init --from tftp:// scenario

package fixture

func init() {
	Register("storage/init-from-tftp", storageInitFromTFTP)
}
