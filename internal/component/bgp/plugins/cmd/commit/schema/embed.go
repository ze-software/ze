// Package schema provides the YANG schema for the bgp-cmd-commit plugin.
package schema

import _ "embed"

//go:embed ze-bgp-cmd-commit-api.yang
var ZeBgpCmdCommitAPIYANG string

//go:embed ze-cli-commit-cmd.yang
var ZeCliCommitCmdYANG string
