package schema

import (
	"codeberg.org/thomas-mangin/ze/internal/component/config/yang"
)

func init() {
	yang.RegisterModule("ze-host-cmd.yang", ZeHostCmdYANG)
	yang.RegisterModule("ze-host-set-cmd.yang", ZeHostSetCmdYANG)
}
