// Package schema provides the YANG schema for the clear CLI verb.
package schema

import _ "embed"

//go:embed ze-cli-clear-api.yang
var ZeCliClearAPIYANG string

//go:embed ze-cli-clear-cmd.yang
var ZeCliClearCmdYANG string
