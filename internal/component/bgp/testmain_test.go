package bgp

import (
	"os"
	"testing"

	"codeberg.org/thomas-mangin/ze/internal/core/family"
)

func TestMain(m *testing.M) {
	family.RegisterTestFamilies()
	os.Exit(m.Run())
}
