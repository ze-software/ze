// Package schema provides the YANG schema for the update CLI verb.
package schema

import _ "embed"

//go:embed ze-cli-update-api.yang
var ZeCliUpdateAPIYANG string

//go:embed ze-cli-update-cmd.yang
var ZeCliUpdateCmdYANG string
