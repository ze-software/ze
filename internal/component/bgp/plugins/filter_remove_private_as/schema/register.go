// Design: docs/architecture/core-design.md -- remove-private-as filter YANG registration

package schema

import "codeberg.org/thomas-mangin/ze/internal/component/config/yang"

func init() {
	yang.RegisterModule("ze-filter-remove-private-as.yang", ZeFilterRemovePrivateASYANG)
}
