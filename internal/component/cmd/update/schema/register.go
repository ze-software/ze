package schema

import (
	"codeberg.org/thomas-mangin/ze/internal/component/config/yang"
)

func init() {
	yang.RegisterModule("ze-cli-update-api.yang", ZeCliUpdateAPIYANG)
	yang.RegisterModule("ze-cli-update-cmd.yang", ZeCliUpdateCmdYANG)
}
