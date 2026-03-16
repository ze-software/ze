// Package schema provides the YANG schema for the bgp-cmd-log plugin.
package schema

import _ "embed"

//go:embed ze-bgp-cmd-log-api.yang
var ZeBgpCmdLogAPIYANG string

//go:embed ze-log-cmd.yang
var ZeLogCmdYANG string
