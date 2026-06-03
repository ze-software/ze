package schema

import (
	"codeberg.org/thomas-mangin/ze/internal/component/config/yang"
)

func init() {
	yang.RegisterModule("ze-pppoe-conf.yang", ZePPPoEConfYANG)
	yang.RegisterModule("ze-pppoe-api.yang", ZePPPoEAPIYANG)
	yang.RegisterModule("ze-pppoe-cmd.yang", ZePPPoECmdYANG)
}
