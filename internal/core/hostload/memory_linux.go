// Design: docs/architecture/testing/runner-architecture.md -- host load detection for contended-run classification
// Related: parse.go -- parseMeminfo

package hostload

import "os"

// readMemoryTotal reads MemTotal out of /proc/meminfo, in bytes.
func readMemoryTotal() (uint64, error) {
	return readMeminfo("MemTotal")
}

// readMemoryAvailable reads MemAvailable out of /proc/meminfo, in bytes: the
// kernel's own estimate of what a new process can use without swapping.
func readMemoryAvailable() (uint64, error) {
	return readMeminfo("MemAvailable")
}

// readMeminfo reads one field of /proc/meminfo. The read error names the file.
func readMeminfo(name string) (uint64, error) {
	data, err := os.ReadFile("/proc/meminfo")
	if err != nil {
		return 0, err
	}
	return parseMeminfo(string(data), name)
}
