// Package schema provides the YANG command schema for BGP policy inspection and testing.
package schema

import _ "embed"

//go:embed ze-policy-cmd.yang
var ZePolicyCmdYANG string
