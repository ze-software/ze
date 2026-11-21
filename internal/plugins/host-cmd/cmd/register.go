// Design: docs/architecture/host/inventory.md -- host command registration

package cmd

func init() {
	registerShowHost()
	registerShowTrend()
	registerShowKernelLog()
	registerSetFD()
	registerStorageGrow()
}
