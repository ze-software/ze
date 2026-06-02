// Package schema provides the YANG command schema for the offline BGP
// protocol tools (show bgp decode / show bgp encode). It lives with the
// cmd/ze/bgp handlers so that removing the BGP command surface removes the
// schema, the local registration, and the handlers together. See
// ai/rules/plugin-self-containment.md.
package schema

import _ "embed"

//go:embed ze-bgp-tools-cmd.yang
var ZeBGPToolsCmdYANG string
