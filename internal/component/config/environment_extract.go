// Design: docs/architecture/config/syntax.md — environment extraction from config tree
// Overview: environment.go — environment configuration loading and defaults
// Related: system.go — system identity extraction (same tree-walking pattern)

package config

// ExtractEnvironment extracts environment configuration values from a parsed Tree.
// Returns a map suitable for passing to LoadEnvironmentWithConfig.
// The environment block is optional - returns empty map if not present.
func ExtractEnvironment(tree *Tree) map[string]map[string]string {
	envContainer := tree.GetContainer("environment")
	if envContainer == nil {
		return nil
	}

	result := make(map[string]map[string]string)

	// Extract each section (daemon, log, tcp, bgp, cache, api, reactor, debug)
	sections := []string{"daemon", "log", "tcp", "bgp", "cache", "api", "reactor", "debug"}
	for _, section := range sections {
		sectionContainer := envContainer.GetContainer(section)
		if sectionContainer == nil {
			continue
		}

		sectionValues := make(map[string]string)
		for _, option := range sectionContainer.Values() {
			value, _ := sectionContainer.Get(option)
			sectionValues[option] = value
		}

		if len(sectionValues) > 0 {
			result[section] = sectionValues
		}
	}

	return result
}
