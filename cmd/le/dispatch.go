// Design: docs/architecture/core-design.md -- le's dispatch loop
// Overview: main.go -- the process boundary
//
// The loop is leroot.Dispatch (letools/leroot/dispatch.go), and it is shared
// rather than owned: a ze built with the ze_le tag runs the same commands under
// one root, `ze le` (letools/zele), through that same function. What stays here
// is the one thing that differs between the two, which is the name the program
// calls itself.

package main

import "github.com/ze-software/ze/letools/leroot"

// binaryName is what the usage page calls this program. It is a constant
// rather than os.Args[0] because le has one name; ze reads argv[0] because its
// personalities are one codebase under several names.
const binaryName = "le"

// dispatch answers the exit code for one invocation of le.
func dispatch(args []string) int {
	return leroot.Dispatch(binaryName, args)
}
