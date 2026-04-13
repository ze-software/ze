package schema

import (
	"codeberg.org/thomas-mangin/ze/internal/component/config/yang"
)

func init() {
	yang.RegisterModule("ze-sysctl-conf.yang", ZeSysctlConfYANG)
}
