// Package schema provides the YANG schema for the bgp-cmd-meta plugin.
package schema

import _ "embed"

//go:embed ze-bgp-cmd-meta-api.yang
var ZeBgpCmdMetaAPIYANG string

//go:embed ze-cli-meta-cmd.yang
var ZeCliMetaCmdYANG string
