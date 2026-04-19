package schema

import (
	"codeberg.org/thomas-mangin/ze/internal/component/config/yang"
)

func init() {
	yang.RegisterModule("ze-fakeredist-api.yang", ZeFakeredistAPIYANG)
	yang.RegisterModule("ze-fakeredist-cmd.yang", ZeFakeredistCmdYANG)
}
