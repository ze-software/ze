package gr

import (
	"os"
	"testing"

	"github.com/ze-software/ze/internal/core/family"
	"github.com/ze-software/ze/pkg/plugin/sdk"
)

func TestMain(m *testing.M) {
	family.RegisterTestFamilies()
	// Registered RIB engines call SignalContext inside synctest bubbles.
	// Initialize Go's process-wide signal thread and channels outside any bubble,
	// then release this registration. Each engine still owns its signal context.
	_, stopSignals := sdk.SignalContext()
	stopSignals()
	os.Exit(m.Run())
}
