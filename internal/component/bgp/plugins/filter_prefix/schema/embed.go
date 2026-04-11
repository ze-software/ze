// Design: docs/architecture/core-design.md -- prefix-list filter YANG schema
//
// Package schema provides the YANG schema for the prefix-list filter plugin.
package schema

import _ "embed"

//go:embed ze-filter-prefix.yang
var ZeFilterPrefixYANG string
