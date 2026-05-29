// Package schema provides the YANG schema for LDP configuration.
package schema

import _ "embed"

//go:embed ze-ldp-conf.yang
var ZeLDPConfYANG string
