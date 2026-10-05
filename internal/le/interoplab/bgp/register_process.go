// Design: docs/architecture/testing/interop.md -- registered scenario process helpers.
package bgp

var processHelpers = map[string]func(string) error{
	pathsLimitScenario:                       runPathsLimitProcess,
	scenarioMEDIBGPPostSelectionRemovalGoBGP: runRawMEDFilter,
	scenarioRPKIFRR:                          runRPKIObserver,
	"lg-graph-lab":                           runLGLab,
}

// registerProcessHelper is called during package initialization. A scenario
// owns its custom process; declarative announcement plans need no handler.
func registerProcessHelper(scenario string, handler func(string) error) {
	if handler == nil {
		panic("BUG: nil native interoperability process helper")
	}
	if _, exists := processHelpers[scenario]; exists {
		panic("BUG: duplicate native interoperability process helper: " + scenario)
	}
	processHelpers[scenario] = handler
}
