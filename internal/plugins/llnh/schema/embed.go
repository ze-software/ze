// Package schema provides the YANG schema for the link-local-nexthop plugin.
package schema

import _ "embed"

//go:embed ze-link-local-nexthop.yang
var ZeLinkLocalNexthopYANG string
