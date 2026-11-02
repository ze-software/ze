// Design: docs/architecture/testing/ci-format.md -- record-answer test plugin

package yang

import (
	"github.com/ze-software/ze/internal/component/config/yang"
)

// The module reaches a DUT only through internal/test/plugins/all, which
// cmd/ze links under the zetest tag alone.
func init() {
	yang.RegisterModule("ze-record-plugin-cmd.yang", ZeRecordPluginCmdYANG)
}
