// Design: docs/architecture/system-architecture.md -- ze unified entry point
//
// Single main() for all ze binaries. Build tags control which commands
// register: ze (default), ze-test, ze-chaos, ze-perf, ze-analyze.
// Multi-call dispatch: binary name "ze-test" prepends "test" to args.

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
