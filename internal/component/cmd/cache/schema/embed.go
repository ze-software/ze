// Package schema provides the YANG schema for the bgp-cmd-cache plugin.
package schema

import _ "embed"

//go:embed ze-bgp-cmd-cache-api.yang
var ZeBgpCmdCacheAPIYANG string

//go:embed ze-cache-cmd.yang
var ZeCacheCmdYANG string
