// Package schema provides the YANG command schema for yang CLI commands.
package schema

import _ "embed"

//go:embed ze-yang-cli-cmd.yang
var ZeYangCliCmdYANG string
