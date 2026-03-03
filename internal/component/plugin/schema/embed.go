// Package schema provides the YANG schema for plugin configuration.
package schema

import _ "embed"

//go:embed ze-plugin-conf.yang
var ZePluginConfYANG string
