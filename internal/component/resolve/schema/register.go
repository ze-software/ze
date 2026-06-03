package schema

import (
	"codeberg.org/thomas-mangin/ze/internal/component/config/yang"
)

func init() {
	yang.RegisterModule("ze-resolve-api.yang", ZeResolveAPIYANG)
	yang.RegisterModule("ze-resolve-cmd.yang", ZeResolveCmdYANG)
}
