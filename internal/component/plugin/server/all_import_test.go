package server

import (
	// Trigger plugin init() registrations needed by inprocess tests.
	_ "codeberg.org/thomas-mangin/ze/internal/component/plugin/all"

	"codeberg.org/thomas-mangin/ze/internal/component/config/yang"
)

// buildTestWireToPath creates a WireMethod->path map from the YANG loader for tests.
func buildTestWireToPath() map[string]string {
	loader := yang.NewLoader()
	_ = loader.LoadEmbedded()
	_ = loader.LoadRegistered()
	_ = loader.Resolve()
	return yang.WireMethodToPath(loader)
}
