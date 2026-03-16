package schema

import (
	"codeberg.org/thomas-mangin/ze/internal/component/config/yang"
)

func init() {
	yang.RegisterModule("ze-bgp-cmd-peer-api.yang", ZeBgpCmdPeerAPIYANG)
	yang.RegisterModule("ze-peer-cmd.yang", ZePeerCmdYANG)
}
