// Package schema provides the YANG schema for the static route plugin.
package schema

import _ "embed"

//go:embed ze-static-conf.yang
var ZeStaticConfYANG string
