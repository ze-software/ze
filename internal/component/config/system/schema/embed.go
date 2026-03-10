// Package schema provides the YANG schema for system configuration.
package schema

import _ "embed"

//go:embed ze-system-conf.yang
var ZeSystemConfYANG string
