// Package schema provides the YANG schema for the delete CLI verb.
package schema

import _ "embed"

//go:embed ze-cli-delete-api.yang
var ZeCliDeleteAPIYANG string

//go:embed ze-cli-delete-cmd.yang
var ZeCliDeleteCmdYANG string
