// Package schema provides the YANG command schema for the IPsec show commands
// (show vpn ipsec sa / status / peer). It lives with the ike component so that
// removing the IPsec surface removes the schema, the command registration, and
// the handlers together. See ai/rules/plugin-self-containment.md.
package schema

import _ "embed"

//go:embed ze-ipsec-cmd.yang
var ZeIPsecCmdYANG string
