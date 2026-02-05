// Package schema provides the YANG schema for the RIB plugin.
package schema

import _ "embed"

//go:embed ze-rib.yang
var ZeRibYANG string
