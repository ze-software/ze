package format

import (
	"os"
	"testing"

	"github.com/ze-software/ze/internal/core/family"
)

func TestMain(m *testing.M) {
	family.RegisterTestFamilies()
	os.Exit(m.Run())
}
