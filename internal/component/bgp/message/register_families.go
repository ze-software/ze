// Design: docs/architecture/wire/messages.md — BGP message types
// RFC: rfc/short/rfc4760.md — multiprotocol address families
// Related: family.go — builtinFamilies, the names this file registers
//
// The registration sits in a register file because every call to a
// registering function of the plugin registry does
// (`./le plugin registration-sites check`).

package message

import (
	"github.com/ze-software/ze/internal/component/plugin/registry"
)

// registerBuiltinFamilies records the four RFC 4760 base families in the plugin
// registry's "builtin" source so they appear in completion and inventory output.
// The families themselves are registered in the family package via MustRegister;
// this is a separate concern (telling the plugin registry "these are not from a
// plugin"). Kept in this package because the family package cannot import
// plugin/registry.
var _ = registerBuiltinFamilies()

func registerBuiltinFamilies() bool {
	registry.RegisterBuiltinFamilies("builtin", builtinFamilies())
	return true
}
