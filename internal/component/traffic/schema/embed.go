// Package schema provides the YANG schema for traffic control configuration
// and command surface.
package schema

import _ "embed"

//go:embed ze-traffic-control-conf.yang
var ZeTrafficControlConfYANG string

//go:embed ze-traffic-cmd.yang
var ZeTrafficCmdYANG string
