// Package schema provides the YANG schema for the bfd plugin.
package schema

import _ "embed"

//go:embed ze-bfd-conf.yang
var ZeBFDConfYANG string
