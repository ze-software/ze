// Design: docs/architecture/core-design.md -- YANG module registration

package schema

import (
	"codeberg.org/thomas-mangin/ze/internal/component/config/yang"
)

func init() {
	yang.RegisterModule("ze-kernel-conf.yang", ZeKernelConfYANG)
}
