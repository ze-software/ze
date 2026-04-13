// Package schema provides the YANG schema for traffic control configuration.
package schema

import _ "embed"

//go:embed ze-traffic-control-conf.yang
var ZeTrafficControlConfYANG string
