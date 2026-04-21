package schema

import (
	"codeberg.org/thomas-mangin/ze/internal/component/config/yang"
)

func init() {
	yang.RegisterModule("ze-fakel2tp-api.yang", ZeFakel2tpAPIYANG)
	yang.RegisterModule("ze-fakel2tp-cmd.yang", ZeFakel2tpCmdYANG)
}
