// Package schema provides the YANG schema for the bgp-cmd-subscribe plugin.
package schema

import _ "embed"

//go:embed ze-bgp-cmd-subscribe-api.yang
var ZeBgpCmdSubscribeAPIYANG string

//go:embed ze-subscribe-cmd.yang
var ZeSubscribeCmdYANG string
