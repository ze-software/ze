package schema

import (
	"codeberg.org/thomas-mangin/ze/internal/component/config/yang"
)

func init() {
	yang.RegisterModule("ze-bgp-cmd-meta-api.yang", ZeBgpCmdMetaAPIYANG)
	yang.RegisterModule("ze-meta-cmd.yang", ZeMetaCmdYANG)
}
