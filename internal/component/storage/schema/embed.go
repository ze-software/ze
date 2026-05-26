// Package schema provides the YANG schema for storage SMART configuration.
package schema

import _ "embed"

//go:embed ze-storage-conf.yang
var ZeStorageConfYANG string
