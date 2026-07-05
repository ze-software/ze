// Package yang provides the YANG schema for the fakeas112 test plugin.
package yang

import _ "embed"

//go:embed ze-fakeas112-api.yang
var ZeFakeas112APIYANG string

//go:embed ze-fakeas112-cmd.yang
var ZeFakeas112CmdYANG string
