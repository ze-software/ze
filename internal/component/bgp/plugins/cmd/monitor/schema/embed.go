// Package schema provides the YANG schema for the bgp-cmd-monitor plugin.
package schema

import _ "embed"

//go:embed ze-bgp-cmd-monitor-api.yang
var ZeBgpCmdMonitorAPIYANG string

//go:embed ze-monitor-cmd.yang
var ZeMonitorCmdYANG string
