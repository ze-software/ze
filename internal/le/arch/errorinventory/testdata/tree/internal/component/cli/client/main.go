// Fixture: the local CLI site, a stderr write.
package client

import (
	"fmt"
	"os"
)

// run is the fixture's one local CLI site.
func run() int {
	fmt.Fprintln(os.Stderr, "hint: is the daemon running?")
	return 1
}
