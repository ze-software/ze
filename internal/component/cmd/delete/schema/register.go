package schema

import (
	"codeberg.org/thomas-mangin/ze/internal/component/config/yang"
)

func init() {
	yang.RegisterModule("ze-cli-delete-api.yang", ZeCliDeleteAPIYANG)
	yang.RegisterModule("ze-cli-delete-cmd.yang", ZeCliDeleteCmdYANG)
}
