// Package schema provides the YANG command schema for the VPP dataplane show
// commands (show vpp trace ..., show vpp runtime). It lives with the VPP backend
// plugin so that removing the VPP surface removes the schema, the command
// registration, and the handlers together. See ai/rules/plugin-self-containment.md.
package schema

import _ "embed"

//go:embed ze-vpp-cmd.yang
var ZeVPPCmdYANG string
