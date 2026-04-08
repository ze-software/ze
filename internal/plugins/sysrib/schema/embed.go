// Package schema provides the YANG schema for the rib plugin.
package schema

import _ "embed"

//go:embed ze-rib-conf.yang
var ZeRibConfYANG string
