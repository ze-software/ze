package schema

import (
	"codeberg.org/thomas-mangin/ze/internal/component/config/yang"
)

func init() {
	yang.RegisterModule("ze-iface-api.yang", ZeIfaceAPIYANG)
	yang.RegisterModule("ze-iface-conf.yang", ZeIfaceConfYANG)
	yang.RegisterModule("ze-iface-cmd.yang", ZeIfaceCmdYANG)
	yang.RegisterModule("ze-iface-show-cmd.yang", ZeIfaceShowCmdYANG)
	yang.RegisterModule("ze-iface-interface-cmd.yang", ZeIfaceInterfaceCmdYANG)
	yang.RegisterModule("ze-iface-monitor-cmd.yang", ZeIfaceMonitorCmdYANG)
}
