// Package schema provides the YANG schema for the l2tp-pool plugin.
package schema

import _ "embed"

//go:embed ze-l2tp-pool-conf.yang
var ZeL2TPPoolConfYANG string
