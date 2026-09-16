// Package cmd owns the whole path MTU diagnostic surface as a dedicated
// feature module (ai/rules/plugins.md, "Dedicated feature modules"):
//
//   - show mtu               (ze-show:mtu)  measure every IPsec peer -- mtu.go
//   - show mtu host <addr>   (ze-show:mtu)  measure one address      -- mtu.go
//
// The module imports neither the IKE engine nor the interface backend. IPsec
// state arrives through the registered inventory snapshot, and the probes go
// through internal/core/probe. The YANG command schema container-merges onto
// the show verb root; see ../../../plugins/mtu-cmd/yang/ze-mtu-cmd.yang. The
// reference address is the environment mtu reference-address leaf of
// ../yang/ze-mtu-conf.yang.
package cmd

import (
	// The blank imports register this module's two YANG modules: the show mtu
	// command tree, and the environment mtu configuration leaf.
	_ "github.com/ze-software/ze/internal/component/mtu/yang"
	_ "github.com/ze-software/ze/internal/plugins/mtu-cmd/yang"
)
