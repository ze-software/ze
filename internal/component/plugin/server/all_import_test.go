package server

import (
	// Trigger plugin init() registrations needed by inprocess tests.
	_ "codeberg.org/thomas-mangin/ze/internal/component/plugin/all"

	"codeberg.org/thomas-mangin/ze/internal/component/config/yang"
)

// buildTestWireToPath creates a WireMethod->path map from the shared YANG loader.
func buildTestWireToPath() map[string]string {
	loader, _ := yang.DefaultLoader()
	return yang.WireMethodToPath(loader)
}
