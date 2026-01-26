// Package schema provides the YANG schema for hub/environment configuration.
package schema

import _ "embed"

//go:embed ze-hub.yang
var ZeHubYANG string
