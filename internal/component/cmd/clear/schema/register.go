package schema

import (
	"codeberg.org/thomas-mangin/ze/internal/component/config/yang"
)

func init() {
	yang.RegisterModule("ze-cli-clear-api.yang", ZeCliClearAPIYANG)
	yang.RegisterModule("ze-cli-clear-cmd.yang", ZeCliClearCmdYANG)
}
