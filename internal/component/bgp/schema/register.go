package schema

import (
	"codeberg.org/thomas-mangin/ze/internal/component/config/yang"
)

func init() {
	yang.RegisterModule("ze-bgp-conf.yang", ZeBGPConfYANG)
	yang.RegisterModule("ze-bgp-api.yang", ZeBGPAPIYANG)
}
