package schema

import (
	"codeberg.org/thomas-mangin/ze/internal/component/config/yang/registry"
)

func init() {
	registry.RegisterModule("ze-rib.yang", ZeRibYANG)
	registry.RegisterModule("ze-rib-api.yang", ZeRibAPIYANG)
}
