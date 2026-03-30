// Package schema provides the YANG schema for looking glass configuration.
package schema

import _ "embed"

//go:embed ze-lg-conf.yang
var ZeLGConfYANG string
