package server

import (
	"codeberg.org/thomas-mangin/ze/internal/component/config/yang"
)

// internalBuildTestWireToPath creates a WireMethod->path map from the YANG loader.
// This is the internal-package version used by benchmarks and tests that need
// unexported symbols. The external test file (package server_test) has its own
// version that also imports plugin/all for full registration.
func internalBuildTestWireToPath() map[string]string {
	loader, _ := yang.DefaultLoader()
	return yang.WireMethodToPath(loader)
}
