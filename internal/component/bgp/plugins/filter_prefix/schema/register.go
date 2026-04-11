// Design: docs/architecture/core-design.md -- prefix-list filter YANG registration

package schema

import (
	"codeberg.org/thomas-mangin/ze/internal/component/config/yang"
)

func init() {
	yang.RegisterModule("ze-filter-prefix.yang", ZeFilterPrefixYANG)
}
