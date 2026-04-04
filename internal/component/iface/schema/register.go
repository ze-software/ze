package schema

import (
	"codeberg.org/thomas-mangin/ze/internal/component/config/yang"
)

func init() {
	yang.RegisterModule("ze-iface-api.yang", ZeIfaceAPIYANG)
	yang.RegisterModule("ze-iface-conf.yang", ZeIfaceConfYANG)
	yang.RegisterModule("ze-iface-cmd.yang", ZeIfaceCmdYANG)
}
