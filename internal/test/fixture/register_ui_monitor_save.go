// Design: docs/functional-tests.md -- waiting on atomic stream-save files in .ci tests

package fixture

func init() {
	Register("ui/await-save-file", uiAwaitSaveFile)
	Register("ui/check-committed-save", uiCheckCommittedSave)
}
