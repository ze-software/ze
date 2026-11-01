package yang

import (
	"github.com/ze-software/ze/internal/component/config/yang"
)

func init() {
	yang.RegisterModule("ze-fakefilter-conf.yang", ZeFakefilterConfYANG)
}
