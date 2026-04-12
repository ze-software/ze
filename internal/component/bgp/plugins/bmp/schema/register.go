package schema

import "codeberg.org/thomas-mangin/ze/internal/component/config/yang"

func init() {
	yang.RegisterModule("ze-bmp-conf.yang", ZeBMPConfYANG)
}
