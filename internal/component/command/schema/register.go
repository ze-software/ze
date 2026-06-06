package schema

import (
	"codeberg.org/thomas-mangin/ze/internal/component/config/yang"
)

func init() {
	yang.RegisterModule("ze-command-meta-cmd.yang", ZeCommandMetaCmdYANG)
	yang.RegisterModule("ze-command-monitor-cmd.yang", ZeCommandMonitorCmdYANG)
	yang.RegisterModule("ze-command-meta-api.yang", ZeCommandMetaAPIYANG)
}
