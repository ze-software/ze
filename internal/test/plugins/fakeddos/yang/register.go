package yang

import (
	"codeberg.org/thomas-mangin/ze/internal/component/config/yang"
)

func init() {
	yang.RegisterModule("ze-fakeddos-conf.yang", ZeFakeddosConfYANG)
}
