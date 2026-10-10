// Design: docs/contributing/gh-pages.md -- terminal demo scenarios
// Related: scenarios_network.go -- runBFD, runOSPF, runTraffic, runVRRP

package siteterminaldemo

import "io"

func init() {
	scenarios.add("bfd-failover", scenario{run: runBFD, validate: validateBFD})
	scenarios.add("ospf-adjacency", scenario{run: runOSPF, validate: validateOSPF})
	scenarios.add("traffic-anomaly", scenario{
		run:      func(action string, _ []string, stdout io.Writer) error { return runTraffic(action, stdout) },
		validate: validateTraffic,
	})
	scenarios.add("vrrp-failover", scenario{
		run:      func(action string, _ []string, stdout io.Writer) error { return runVRRP(action, stdout) },
		validate: validateVRRP,
	})
}
