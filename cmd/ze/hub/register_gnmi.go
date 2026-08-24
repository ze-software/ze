// Design: ai/rules/plugins.md -- ze_gnmi compile-out seam
//
// Installs the gNMI build + reload implementations into the compile-out seam
// (gnmi_infra.go). Compiled only under //go:build ze_gnmi; absent the tag this
// init() does not run, the seam stays nil, and gNMI is dropped from the binary.

//go:build ze_gnmi

package hub

func init() {
	gnmiBuild = gnmiBuildImpl
	gnmiReloadNotify = gnmiReloadNotifyImpl
}
