package schema

import (
	"codeberg.org/thomas-mangin/ze/internal/component/config/yang"
)

func init() {
	yang.RegisterModule("ze-policyroute-conf.yang", ZePolicyrouteConfYANG)
	yang.RegisterModule("ze-policyroute-cmd.yang", ZePolicyrouteCmdYANG)
}
