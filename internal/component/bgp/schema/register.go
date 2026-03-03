package schema

import (
	"codeberg.org/thomas-mangin/ze/internal/component/config/yang/registry"
)

func init() {
	registry.RegisterModule("ze-bgp-conf.yang", ZeBGPConfYANG)
	registry.RegisterModule("ze-bgp-api.yang", ZeBGPAPIYANG)
}
