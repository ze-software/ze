// Package schema provides the YANG schemas for Ze IPC API modules.
package schema

import _ "embed"

//go:embed ze-system-api.yang
var ZeSystemAPIYANG string

//go:embed ze-plugin-api.yang
var ZePluginAPIYANG string
