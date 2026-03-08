// Package schema provides the YANG schema for the bgp-cmd-raw plugin.
package schema

import _ "embed"

//go:embed ze-bgp-cmd-raw-api.yang
var ZeBgpCmdRawAPIYANG string
