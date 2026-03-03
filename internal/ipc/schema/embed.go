// Package schema provides the YANG schemas for Ze IPC and plugin protocol modules.
package schema

import _ "embed"

//go:embed ze-system-api.yang
var ZeSystemAPIYANG string

//go:embed ze-plugin-api.yang
var ZePluginAPIYANG string

//go:embed ze-plugin-callback.yang
var ZePluginCallbackYANG string

//go:embed ze-plugin-engine.yang
var ZePluginEngineYANG string
