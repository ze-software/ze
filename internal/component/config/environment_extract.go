// Design: docs/architecture/config/syntax.md — environment extraction from config tree
// Related: environment.go — env var registrations and listener helpers
// Related: apply_env.go — maps extracted values to OS env vars

package config

// ExtractEnvironment extracts environment configuration values from a parsed
// Tree. Sections are keyed by their container name (`bgp`, `reactor`, etc.).
// Top-level leaves directly under `environment/` land in the empty-string
// section. Nested containers like `environment/exabgp/api` land in the
// dot-joined section key `exabgp.api`.
//
// The environment block is optional: returns nil if not present.
func ExtractEnvironment(tree *Tree) map[string]map[string]string {
	envContainer := tree.GetContainer("environment")
	if envContainer == nil {
		return nil
	}

	result := make(map[string]map[string]string)

	// Top-level leaves under environment/ (e.g. `environment/pprof`).
	topLeaves := make(map[string]string)
	for _, option := range envContainer.Values() {
		value, _ := envContainer.Get(option)
		topLeaves[option] = value
	}
	if len(topLeaves) > 0 {
		result[""] = topLeaves
	}

	for _, section := range extractSections {
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

		// Descend into one level of nested containers for plumbing keys
		// like `environment/exabgp/api/ack` -> section "exabgp.api".
		for _, sub := range sectionContainer.ContainerNames() {
			subContainer := sectionContainer.GetContainer(sub)
			if subContainer == nil {
				continue
			}
			subValues := make(map[string]string)
			for _, option := range subContainer.Values() {
				value, _ := subContainer.Get(option)
				subValues[option] = value
			}
			if len(subValues) > 0 {
				result[section+"."+sub] = subValues
			}
		}
	}

	return result
}
