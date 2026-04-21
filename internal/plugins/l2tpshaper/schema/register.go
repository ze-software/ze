package schema

import "codeberg.org/thomas-mangin/ze/internal/component/config/yang"

func init() {
	yang.RegisterModule("ze-l2tp-shaper-conf.yang", ZeL2TPShaperConfYANG)
}
