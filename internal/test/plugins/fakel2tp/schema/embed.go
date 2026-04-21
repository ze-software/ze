// Package schema provides the YANG schema for the fakel2tp test plugin.
package schema

import _ "embed"

//go:embed ze-fakel2tp-api.yang
var ZeFakel2tpAPIYANG string

//go:embed ze-fakel2tp-cmd.yang
var ZeFakel2tpCmdYANG string
