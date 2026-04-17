// Package schema embeds the L2TP command-tree YANG module.
package schema

import _ "embed"

//go:embed ze-l2tp-cmd.yang
var ZeL2TPCmdYANG string
