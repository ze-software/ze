// Design: docs/guide/pppoe.md -- numbering a LAN unit from a delegated prefix
// Related: delegated_prefix.go -- delegationSourceReference, the source registered here

package iface

import "github.com/ze-software/ze/internal/component/config/yang"

func init() {
	// The delegated-prefix source leaf declares
	// `ze:reference "pppoe-client-delegation"`: the editor completes the
	// pppoe-clients that request a delegated prefix through this source.
	yang.RegisterReferenceSource(DelegationSourceReference, delegationSourceReference)
}
