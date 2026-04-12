// Design: docs/architecture/core-design.md -- AS-path filter YANG schema
//
// Package schema provides the YANG schema for the AS-path filter plugin.
package schema

import _ "embed"

//go:embed ze-filter-aspath.yang
var ZeFilterAsPathYANG string
