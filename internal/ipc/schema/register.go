package schema

import (
	"codeberg.org/thomas-mangin/ze/internal/component/config/yang/registry"
)

func init() {
	registry.RegisterModule("ze-system-api.yang", ZeSystemAPIYANG)
	registry.RegisterModule("ze-plugin-api.yang", ZePluginAPIYANG)
}
