// Package schema provides the YANG schema for hub configuration.
package schema

import _ "embed"

//go:embed ze-hub-conf.yang
var ZeHubConfYANG string
