// Design: docs/architecture/site-facts.md -- one committed scenario partition.
// Detail: facts/sitefacts.go owns counting; this reader never scans test trees.
package site

import (
	"encoding/json"
	"fmt"
	"strings"

	sitefacts "github.com/ze-software/ze/internal/le/site/facts"
)

// factsFromInteropInventory exposes the recorded partitions and refuses missing
// measurements or totals that could describe different scenario populations.
// The caller MUST supply the same bytes used for the legacy repository facts,
// after setting Interop.Scenarios from them. No working-tree recount is allowed.
func factsFromInteropInventory(content json.RawMessage, facts *siteFacts) error {
	var recorded struct {
		Facts map[string]struct {
			Value  *int   `json:"value"`
			Source string `json:"source"`
		} `json:"facts"`
	}
	if err := json.Unmarshal(content, &recorded); err != nil {
		return fmt.Errorf("read website/data/%s interoperability inventory: %w", repositoryFactsFile, err)
	}
	measurement := func(key string) (int, error) {
		entry, present := recorded.Facts[key]
		if !present {
			return 0, interopInventoryError("states no " + key)
		}
		if entry.Value == nil {
			return 0, interopInventoryError("states no numeric value for " + key)
		}
		if *entry.Value < 0 {
			return 0, interopInventoryError("states negative " + key)
		}
		facts.Sources[key] = "website/data/" + repositoryFactsFile + ": " + entry.Source
		return *entry.Value, nil
	}
	suiteCount, err := measurement("interop.suite_count")
	if err != nil {
		return err
	}
	knownProtocols := make(map[string]bool)
	for _, protocol := range sitefacts.InteropProtocolKeys() {
		knownProtocols[protocol] = true
		if _, err := measurement("interop.protocols." + protocol); err != nil {
			return err
		}
	}
	facts.Interop.SuiteCount = suiteCount
	facts.Interop.Suites = make(map[string]int)
	facts.Interop.Protocols = make(map[string]int)
	for _, partition := range []struct {
		prefix string
		counts map[string]int
	}{
		{"interop.suites.", facts.Interop.Suites},
		{"interop.protocols.", facts.Interop.Protocols},
	} {
		total := 0
		for key := range recorded.Facts {
			name, belongs := strings.CutPrefix(key, partition.prefix)
			if !belongs {
				continue
			}
			if name == "" {
				return interopInventoryError("states an unnamed " + partition.prefix + " entry")
			}
			if strings.Contains(name, ".") {
				return interopInventoryError("states a nested partition entry " + key)
			}
			if partition.prefix == "interop.protocols." {
				if !knownProtocols[name] {
					return interopInventoryError("states unknown protocol key " + key)
				}
			}
			count, err := measurement(key)
			if err != nil {
				return err
			}
			partition.counts[name] = count
			total += count
		}
		if total != facts.Interop.Scenarios {
			return interopInventoryError(fmt.Sprintf("%s total %d disagrees with interop.scenarios %d",
				partition.prefix, total, facts.Interop.Scenarios))
		}
	}
	for suite, count := range facts.Interop.Suites {
		if count == 0 {
			return interopInventoryError("states a scenario-free suite interop.suites." + suite)
		}
	}
	if len(facts.Interop.Suites) != facts.Interop.SuiteCount {
		return interopInventoryError("interop.suite_count disagrees with the scenario-bearing suite partition")
	}
	return nil
}

// interopInventoryError keeps the correction command beside a refused claim.
func interopInventoryError(problem string) error {
	return fmt.Errorf("website/data/%s %s; run `./le site facts update`", repositoryFactsFile, problem)
}
