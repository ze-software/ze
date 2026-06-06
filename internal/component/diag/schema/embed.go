// Package schema provides the YANG command schema for the diag component.
package schema

import _ "embed"

//go:embed ze-diag-cmd.yang
var ZeDiagCmdYANG string
