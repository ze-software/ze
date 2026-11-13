// Design: docs/architecture/testing/runner-architecture.md -- host load detection for contended-run classification

package hostload

import "os"

// readLoadAverages reads the load averages on Linux from /proc/loadavg, whose
// line reads "3.43 4.65 14.57 1/234 5678". The read error names the file.
func readLoadAverages() (Averages, error) {
	data, err := os.ReadFile("/proc/loadavg")
	if err != nil {
		return Averages{}, err
	}
	return parseLoadAverages(string(data))
}
