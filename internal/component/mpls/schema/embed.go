// Package schema provides the YANG schema for MPLS commands.
package schema

import _ "embed"

//go:embed ze-mpls-cmd.yang
var ZeMPLSCmdYANG string
