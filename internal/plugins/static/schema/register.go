package schema

import (
	"codeberg.org/thomas-mangin/ze/internal/component/config/yang"
)

func init() {
	yang.RegisterModule("ze-static-conf.yang", ZeStaticConfYANG)
	yang.RegisterModule("ze-static-cmd.yang", ZeStaticCmdYANG)
}
