package schema

import (
	"codeberg.org/thomas-mangin/ze/internal/component/config/yang"
)

func init() {
	yang.RegisterModule("ze-ldp-conf.yang", ZeLDPConfYANG)
	yang.RegisterModule("ze-ldp-cmd.yang", ZeLDPCmdYANG)
}
