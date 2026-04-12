// Package schema provides the YANG schemas for the bfd plugin.
package schema

import _ "embed"

//go:embed ze-bfd-conf.yang
var ZeBFDConfYANG string

//go:embed ze-bfd-api.yang
var ZeBFDAPIYANG string
