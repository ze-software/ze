// Design: docs/architecture/core-design.md -- route modify filter YANG registration

package schema

import (
	"codeberg.org/thomas-mangin/ze/internal/component/config/yang"
)

func init() {
	yang.RegisterModule("ze-filter-modify.yang", ZeFilterModifyYANG)
}
