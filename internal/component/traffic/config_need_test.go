package traffic

import (
	"testing"

	"github.com/ze-software/ze/internal/component/config/configneedtest"
)

// TestConfigNeedPathsExistInSchema walks every config need this package
// registers against the YANG schema its own module builds. Method:
// configneedtest.RequirePathsInSchemaForTest fails naming the path when an element is
// absent, when the path crosses a list, or when it does not end at a leaf.
//
// VALIDATES: AC-16, the need path names the backend leaf the operator writes.
// PREVENTS: a renamed or moved leaf leaving a need that no config can hold.
func TestConfigNeedPathsExistInSchema(t *testing.T) {
	configneedtest.RequirePathsInSchemaForTest(t, "traffic")
}
