// Package schema provides the YANG schema for the routing-table plugin.
package schema

import _ "embed"

//go:embed ze-routing-table-conf.yang
var ZeRoutingTableConfYANG string
