// Design: plan/learned/631-host-0-inventory.md — host command registration

package cmd

func init() {
	RegisterShowHost()
	RegisterShowKernelLog()
	RegisterSetFD()
}
