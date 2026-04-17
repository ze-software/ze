package schema

import (
	"codeberg.org/thomas-mangin/ze/internal/component/config/yang"
)

func init() {
	yang.RegisterModule("ze-l2tp-conf.yang", ZeL2TPConfYANG)
	yang.RegisterModule("ze-l2tp-api.yang", ZeL2TPAPIYANG)
}
