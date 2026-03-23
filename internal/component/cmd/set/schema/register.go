package schema

import (
	"codeberg.org/thomas-mangin/ze/internal/component/config/yang"
)

func init() {
	yang.RegisterModule("ze-cli-set-api.yang", ZeCliSetAPIYANG)
	yang.RegisterModule("ze-cli-set-cmd.yang", ZeCliSetCmdYANG)
}
