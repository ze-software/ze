// Package schema provides the YANG schema for API engine configuration.
package schema

import _ "embed"

//go:embed ze-api-conf.yang
var ZeAPIConfYANG string
