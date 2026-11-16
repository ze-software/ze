// Design: docs/architecture/iface/logical-name-resolution.md -- a leaf that configures an interface Ze owns names one the config declares
// Related: interface_reference.go -- configuredInterfaceReference, the source registered here

package iface

import "github.com/ze-software/ze/internal/component/config/yang"

func init() {
	// The leaves that configure an interface Ze owns declare
	// `ze:reference "configured-interface"`: mirror ingress and egress,
	// bridge member, both tunnel local interfaces and pppoe-client
	// source-interface here, and the PPPoE server interface key in
	// ze-pppoe-conf. The editor completes the interfaces the config declares
	// through this source, and the commit refuses any other name.
	yang.RegisterReferenceSource(ConfiguredInterfaceReference, configuredInterfaceReference)
}
