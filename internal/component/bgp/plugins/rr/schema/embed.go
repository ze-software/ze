// Package schema provides the YANG command schema for the route-reflector
// show commands (show rr status / show rr peers). It lives with the bgp-rr
// plugin so that removing the route-reflector surface removes the schema, the
// command registration, and the handlers together. See
// ai/rules/plugin-self-containment.md.
package schema

import _ "embed"

//go:embed ze-rr-cmd.yang
var ZeRRCmdYANG string
