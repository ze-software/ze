// Package schema provides the YANG schema for the show CLI verb.
package schema

import _ "embed"

//go:embed ze-cli-show-api.yang
var ZeCliShowAPIYANG string

//go:embed ze-cli-show-cmd.yang
var ZeCliShowCmdYANG string
