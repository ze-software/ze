// Design: docs/architecture/core-design.md -- community match filter YANG registration

package schema

import (
	"codeberg.org/thomas-mangin/ze/internal/component/config/yang"
)

func init() {
	yang.RegisterModule("ze-filter-community-match.yang", ZeFilterCommunityMatchYANG)
}
