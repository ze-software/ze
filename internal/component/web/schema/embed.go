// Package schema provides the YANG schema for web interface configuration.
package schema

import _ "embed"

//go:embed ze-web-conf.yang
var ZeWebConfYANG string
