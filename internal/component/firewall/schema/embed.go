// Package schema provides the YANG schema for firewall configuration.
package schema

import _ "embed"

//go:embed ze-firewall-conf.yang
var ZeFirewallConfYANG string
