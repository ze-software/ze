// Package schema provides the YANG schema for the healthcheck plugin.
package schema

import _ "embed"

//go:embed ze-healthcheck-conf.yang
var ZeHealthcheckConfYANG string
