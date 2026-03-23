package schema

import (
	"codeberg.org/thomas-mangin/ze/internal/component/config/yang"
)

func init() {
	yang.RegisterModule("ze-cli-del-api.yang", ZeCliDelAPIYANG)
	yang.RegisterModule("ze-cli-del-cmd.yang", ZeCliDelCmdYANG)
}
