package yang

import (
	"github.com/ze-software/ze/internal/component/config/yang"
)

func init() {
	yang.RegisterModule("ze-fakeas112-api.yang", ZeFakeas112APIYANG)
	yang.RegisterModule("ze-fakeas112-cmd.yang", ZeFakeas112CmdYANG)
}
