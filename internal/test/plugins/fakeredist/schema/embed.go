// Package schema provides the YANG schema for the fakeredist test plugin.
package schema

import _ "embed"

//go:embed ze-fakeredist-api.yang
var ZeFakeredistAPIYANG string

//go:embed ze-fakeredist-cmd.yang
var ZeFakeredistCmdYANG string
