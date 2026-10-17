package filter_irr

import (
	"os"
	"testing"

	"github.com/ze-software/ze/internal/core/family"
)

// TestMain registers the address families, which a test binary of this package
// does not link: rewritable looks an mp block's family up by name to learn
// whether the engine can rewrite it, and fails closed on a name it cannot find.
func TestMain(m *testing.M) {
	family.RegisterTestFamilies()
	os.Exit(m.Run())
}
