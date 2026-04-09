// Design: docs/architecture/core-design.md -- loop detection filter YANG registration

package schema

import (
	"codeberg.org/thomas-mangin/ze/internal/component/config/yang"
)

func init() {
	yang.RegisterModule("ze-loop-detection.yang", ZeLoopDetectionYANG)
}
