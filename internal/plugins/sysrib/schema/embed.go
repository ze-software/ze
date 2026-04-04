// Package schema provides the YANG schema for the sysrib plugin.
package schema

import _ "embed"

//go:embed ze-sysrib-conf.yang
var ZeSysribConfYANG string
