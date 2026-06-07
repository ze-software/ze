// Package schema provides the YANG schema for MRT dump configuration.
package schema

import _ "embed"

//go:embed ze-mrt-conf.yang
var ZeMRTConfYANG string
