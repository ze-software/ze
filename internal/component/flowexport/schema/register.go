package schema

import (
	"codeberg.org/thomas-mangin/ze/internal/component/config/yang"
)

func init() {
	yang.RegisterModule("ze-flowexport-conf.yang", ZeFlowExportConfYANG)
	yang.RegisterModule("ze-flowexport-cmd.yang", ZeFlowExportCmdYANG)
}
