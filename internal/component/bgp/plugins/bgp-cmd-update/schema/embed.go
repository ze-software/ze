// Package schema provides the YANG schema for the bgp-cmd-update plugin.
package schema

import _ "embed"

//go:embed ze-bgp-cmd-update-api.yang
var ZeBgpCmdUpdateAPIYANG string
