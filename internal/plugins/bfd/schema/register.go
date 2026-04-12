package schema

import (
	"codeberg.org/thomas-mangin/ze/internal/component/config/yang"
)

func init() {
	yang.RegisterModule("ze-bfd-conf.yang", ZeBFDConfYANG)
	yang.RegisterModule("ze-bfd-api.yang", ZeBFDAPIYANG)
}
