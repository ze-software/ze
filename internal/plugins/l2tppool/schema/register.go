package schema

import "codeberg.org/thomas-mangin/ze/internal/component/config/yang"

func init() {
	yang.RegisterModule("ze-l2tp-pool-conf.yang", ZeL2TPPoolConfYANG)
}
