package schema

import (
	"codeberg.org/thomas-mangin/ze/internal/component/config/yang"
)

func init() {
	yang.RegisterModule("ze-bgp-cmd-raw-api.yang", ZeBgpCmdRawAPIYANG)
	yang.RegisterModule("ze-raw-cmd.yang", ZeRawCmdYANG)
}
