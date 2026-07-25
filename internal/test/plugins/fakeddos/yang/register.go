package yang

import (
	"github.com/ze-software/ze/internal/component/config/yang"
)

func init() {
	yang.RegisterModule("ze-fakeddos-conf.yang", ZeFakeddosConfYANG)
}
