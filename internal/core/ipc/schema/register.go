package schema

import (
	"codeberg.org/thomas-mangin/ze/internal/component/config/yang"
)

func init() {
	yang.RegisterModule("ze-system-api.yang", ZeSystemAPIYANG)
	yang.RegisterModule("ze-plugin-api.yang", ZePluginAPIYANG)
	yang.RegisterModule("ze-plugin-callback.yang", ZePluginCallbackYANG)
	yang.RegisterModule("ze-plugin-engine.yang", ZePluginEngineYANG)
}
