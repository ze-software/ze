// Design: docs/architecture/core-design.md -- route modify filter YANG schema
//
// Package schema provides the YANG schema for the route modify filter plugin.
package schema

import _ "embed"

//go:embed ze-filter-modify.yang
var ZeFilterModifyYANG string
