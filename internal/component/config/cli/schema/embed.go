// Package schema provides the YANG command schema for config CLI commands.
package schema

import _ "embed"

//go:embed ze-config-cli-cmd.yang
var ZeConfigCliCmdYANG string
