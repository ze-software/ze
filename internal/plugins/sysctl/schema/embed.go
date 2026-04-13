// Package schema provides the YANG schema for the sysctl plugin.
package schema

import _ "embed"

//go:embed ze-sysctl-conf.yang
var ZeSysctlConfYANG string
