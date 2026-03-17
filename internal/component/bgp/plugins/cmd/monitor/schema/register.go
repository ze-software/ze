package schema

import (
	"codeberg.org/thomas-mangin/ze/internal/component/config/yang"
)

func init() {
	yang.RegisterModule("ze-bgp-cmd-monitor-api.yang", ZeBgpCmdMonitorAPIYANG)
	yang.RegisterModule("ze-monitor-cmd.yang", ZeMonitorCmdYANG)
}
