// Design: docs/architecture/core-design.md -- redistribute YANG registration

package schema

import "codeberg.org/thomas-mangin/ze/internal/component/config/yang"

func init() {
	yang.RegisterModule("ze-redistribute.yang", ZeRedistributeYANG)
}
