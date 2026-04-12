// Design: docs/architecture/core-design.md -- community match filter YANG schema
//
// Package schema provides the YANG schema for the community match filter plugin.
package schema

import _ "embed"

//go:embed ze-filter-community-match.yang
var ZeFilterCommunityMatchYANG string
