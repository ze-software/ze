// Package schema provides the YANG schema for the community filter plugin.
package schema

import _ "embed"

//go:embed ze-filter-community.yang
var ZeFilterCommunityYANG string
