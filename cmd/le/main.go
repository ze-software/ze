// Design: docs/architecture/core-design.md -- le, the repository and development binary
//
// le and ze are two binaries over one engine. They share the command registry,
// the command grammar and the pipe operators; they share no plugins. ze is the
// product. le is repository management and development, and no build ever
// links one's plugins into the other (cmd/le/separation_test.go proves it).
//
// This file holds only the process boundary. Dispatch is dispatch.go and the
// composition root is register.go.

package main

import "os"

func main() {
	os.Exit(dispatch(os.Args[1:]))
}
