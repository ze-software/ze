// Package schema provides the YANG schema for BGP configuration.
package schema

import _ "embed"

//go:embed ze-bgp.yang
var ZeBGPYANG string
