// Package schema provides the YANG schema for BGP configuration.
package schema

import _ "embed"

//go:embed ze-bgp-conf.yang
var ZeBGPConfYANG string
