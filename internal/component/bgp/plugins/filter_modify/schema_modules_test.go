// Design: docs/architecture/config/syntax.md -- YANG schema default application
// Related: config.go -- parseAttributeDefaults, the reader of bgp/defaults/attribute
//
// The YANG loader resolves the module CLOSURE, so a lookup in ze-bgp-conf needs
// every module ze-bgp-conf imports to be registered too. The ze binary links
// them all through the generated composition root; this package's own test
// binary links what it imports and nothing else, so the closure is named here.
//
// ze-types and ze-extensions arrive with LoadEmbedded (config/yang/modules).
// ze-hub-conf is a component module and registers from its own package's init.

package filter_modify

import (
	_ "github.com/ze-software/ze/internal/component/hub/yang"
)
