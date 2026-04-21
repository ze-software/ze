// Package schema provides the YANG schema for the l2tp-auth-radius plugin.
package schema

import _ "embed"

//go:embed ze-l2tp-auth-radius-conf.yang
var ZeL2TPAuthRadiusConfYANG string
