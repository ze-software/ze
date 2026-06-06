// Package schema provides the YANG command schemas owned by the command component.
package schema

import _ "embed"

//go:embed ze-command-meta-cmd.yang
var ZeCommandMetaCmdYANG string

//go:embed ze-command-monitor-cmd.yang
var ZeCommandMonitorCmdYANG string

//go:embed ze-command-meta-api.yang
var ZeCommandMetaAPIYANG string
