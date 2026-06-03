package schema

import (
	"codeberg.org/thomas-mangin/ze/internal/component/config/yang"
)

func init() {
	yang.RegisterModule("ze-pki-conf.yang", ZePKIConfYANG)
	yang.RegisterModule("ze-pki-api.yang", ZePKIAPIYANG)
	yang.RegisterModule("ze-pki-cmd.yang", ZePKICmdYANG)
}
