// Package schema provides the YANG schema for the Adj-RIB-In plugin.
package schema

import _ "embed"

//go:embed ze-adj-rib-in-api.yang
var ZeAdjRibInYANG string
