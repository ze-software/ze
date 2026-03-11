// Package schema provides the YANG schema for authorization configuration.
package schema

import _ "embed"

//go:embed ze-authz-conf.yang
var ZeAuthzConfYANG string
