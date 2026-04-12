// Design: docs/architecture/core-design.md -- AS-path filter YANG registration

package schema

import (
	"codeberg.org/thomas-mangin/ze/internal/component/config/yang"
)

func init() {
	yang.RegisterModule("ze-filter-aspath.yang", ZeFilterAsPathYANG)
}
