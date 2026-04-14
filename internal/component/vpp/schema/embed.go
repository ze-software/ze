// Package schema provides the YANG schema for the VPP component.
package schema

import _ "embed"

//go:embed ze-vpp-conf.yang
var ZeVppConfYANG string
