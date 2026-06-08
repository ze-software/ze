// Design: docs/architecture/system-architecture.md -- ze unified entry point
//
// Single main() for all ze binaries. Build tags control which commands
// register: ze_core (ze, ze-appliance), ze_setup, ze_test, ze_chaos,
// ze_perf, ze_analyze. The binary name does not affect dispatch.

package main

import "os"

var (
	version   = "dev"
	buildDate = "unknown"
)

func main() {
	code := dispatchMain(os.Args[1:])
	flushCrashlog()
	os.Exit(code)
}
