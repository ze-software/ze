package schema

import (
	"codeberg.org/thomas-mangin/ze/internal/component/config/yang"
)

func init() {
	yang.RegisterModule("ze-fib-p4-conf.yang", ZeFibP4ConfYANG)
}
