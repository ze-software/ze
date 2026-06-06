// Package schema provides the YANG command schema for log inspection and level control.
package schema

import _ "embed"

//go:embed ze-log-cmd.yang
var ZeLogCmdYANG string
