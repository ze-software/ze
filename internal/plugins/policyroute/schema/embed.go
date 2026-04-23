// Package schema provides the YANG schema for the policy routing plugin.
package schema

import _ "embed"

//go:embed ze-policyroute-conf.yang
var ZePolicyrouteConfYANG string
