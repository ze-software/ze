package schema

import (
	"codeberg.org/thomas-mangin/ze/internal/component/config/yang"
)

func init() {
	yang.RegisterModule("ze-bgp-cmd-log-api.yang", ZeBgpCmdLogAPIYANG)
	yang.RegisterModule("ze-log-cmd.yang", ZeLogCmdYANG)
}
