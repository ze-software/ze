// Package schema provides the YANG schema for the generic monitor verb root.
package schema

import _ "embed"

//go:embed ze-cli-monitor-cmd.yang
var ZeCliMonitorCmdYANG string
