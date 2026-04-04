// Package schema provides the YANG schema for interface configuration.
package schema

import _ "embed"

//go:embed ze-iface-conf.yang
var ZeIfaceConfYANG string

//go:embed ze-iface-cmd.yang
var ZeIfaceCmdYANG string
