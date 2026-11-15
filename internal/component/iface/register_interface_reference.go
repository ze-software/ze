// Design: docs/architecture/iface/logical-name-resolution.md -- a mirror destination names an interface the config declares
// Related: interface_reference.go -- configuredInterfaceReference, the source registered here

package iface

import "github.com/ze-software/ze/internal/component/config/yang"

func init() {
	// The mirror ingress and egress leaves declare
	// `ze:reference "configured-interface"`: the editor completes the
	// interfaces the config declares through this source, and the commit
	// refuses any other name.
	yang.RegisterReferenceSource(ConfiguredInterfaceReference, configuredInterfaceReference)
}
