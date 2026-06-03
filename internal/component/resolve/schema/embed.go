// Package schema provides the YANG schema for the resolve command handlers.
package schema

import _ "embed"

//go:embed ze-resolve-api.yang
var ZeResolveAPIYANG string

//go:embed ze-resolve-cmd.yang
var ZeResolveCmdYANG string
