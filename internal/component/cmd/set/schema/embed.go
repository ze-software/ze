// Package schema provides the YANG schema for the set CLI verb.
package schema

import _ "embed"

//go:embed ze-cli-set-api.yang
var ZeCliSetAPIYANG string

//go:embed ze-cli-set-cmd.yang
var ZeCliSetCmdYANG string
