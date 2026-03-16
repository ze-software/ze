// Package schema provides the YANG schema for the bgp-cmd-peer plugin.
package schema

import _ "embed"

//go:embed ze-bgp-cmd-peer-api.yang
var ZeBgpCmdPeerAPIYANG string

//go:embed ze-peer-cmd.yang
var ZePeerCmdYANG string
