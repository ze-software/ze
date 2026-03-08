// Package schema provides the YANG schema for the bgp-cmd-ops plugin.
package schema

import _ "embed"

//go:embed ze-bgp-cmd-ops-api.yang
var ZeBgpCmdOpsAPIYANG string
