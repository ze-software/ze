// Design: docs/architecture/testing/interop.md -- Optional sidecars, ze-sysctls.
// Related: prepare.go -- scenarioPeers, which appends these to Ze's container.
package bgp

import (
	"fmt"
	"strings"
)

// zeSysctlsFile names the network-namespace sysctls a scenario needs set in Ze's
// container before ze starts, one `key=value` per token. Docker mounts /proc/sys
// read-only, so a value ze would write itself at run time (the MPLS label space,
// fib-kernel's repairLabelSpace) is refused there; a `--sysctl` argument is written
// by the runtime into the container's own network namespace and needs no grant
// (interoplab.NetSysctlWriteArguments explains the run-time alternative).
const zeSysctlsFile = "ze-sysctls"

// zeSysctlPrefix is the only sysctl tree the file may name: a net.* key changes the
// container's own network namespace and nothing outside it.
const zeSysctlPrefix = "net."

// zeSysctlArguments answers the docker run arguments scenario's zeSysctlsFile asks
// for, and none when the scenario carries no such file. A token that is not a
// net.* `key=value` assignment is refused, naming the file and the token.
func zeSysctlArguments(scenario string) ([]string, error) {
	assignments, err := readArguments(scenario, zeSysctlsFile)
	if err != nil {
		return nil, err
	}
	arguments := make([]string, 0, 2*len(assignments))
	for _, assignment := range assignments {
		key, value, found := strings.Cut(assignment, "=")
		if !found {
			return nil, fmt.Errorf("%s: %q is not a sysctl assignment: write it as key=value, such as net.mpls.platform_labels=1048575", zeSysctlsFile, assignment)
		}
		if value == "" {
			return nil, fmt.Errorf("%s: %q assigns no value: write the value after '='", zeSysctlsFile, assignment)
		}
		if !strings.HasPrefix(key, zeSysctlPrefix) {
			return nil, fmt.Errorf("%s: %q is outside net.*, and only a network-namespace sysctl stays inside the container: set it on the Docker host instead", zeSysctlsFile, assignment)
		}
		arguments = append(arguments, dockerSysctlFlag, assignment)
	}
	return arguments, nil
}
