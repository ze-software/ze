// Package schema provides the YANG schema for the l2tp-shaper plugin.
package schema

import _ "embed"

//go:embed ze-l2tp-shaper-conf.yang
var ZeL2TPShaperConfYANG string
