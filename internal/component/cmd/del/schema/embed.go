// Package schema provides the YANG schema for the del/delete CLI verb.
package schema

import _ "embed"

//go:embed ze-cli-del-api.yang
var ZeCliDelAPIYANG string

//go:embed ze-cli-del-cmd.yang
var ZeCliDelCmdYANG string
