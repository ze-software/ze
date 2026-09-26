// Design: docs/architecture/testing/ci-format.md -- tree storage functional scenarios
// Related: storage_init_from_fixture.go -- the ze init --from scenarios

package fixture

func init() {
	Register("storage/init-from-path", storageInitFromPath)
	Register("storage/init-from-url", storageInitFromURL)
	Register("storage/init-from-refused", storageInitFromRefused)
}
