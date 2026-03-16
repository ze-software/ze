// Package schema provides the YANG schema for the bgp-cmd-rib plugin.
package schema

import _ "embed"

//go:embed ze-rib-cmd.yang
var ZeRibCmdYANG string
