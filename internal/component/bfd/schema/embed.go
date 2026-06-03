// Package schema embeds the BFD command-tree YANG module.
package schema

import _ "embed"

//go:embed ze-bfd-cmd.yang
var ZeBFDCmdYANG string
