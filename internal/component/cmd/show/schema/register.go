package schema

import (
	"codeberg.org/thomas-mangin/ze/internal/component/config/yang"
)

func init() {
	yang.RegisterModule("ze-cli-show-api.yang", ZeCliShowAPIYANG)
	yang.RegisterModule("ze-cli-show-cmd.yang", ZeCliShowCmdYANG)
}
