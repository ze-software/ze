// Package config_test links bgp/plugin into the config test binary, so its
// init() registers the bgp event namespace before any test of package config
// runs. It is an external test package because bgp/plugin imports
// internal/component/policy, which imports config: an internal test file
// importing it is an import cycle.
package config_test

// Import bgp/plugin to trigger bgp namespace registration via init().
import _ "github.com/ze-software/ze/internal/component/bgp/plugin"
