package cli

import (
	"fmt"
	"os"

	"github.com/ze-software/ze/internal/test/localdatacoverage"
)

// CmdLocalDataCoverage is the harness command `le test local-data-coverage`,
// registered by internal/le/test/localdatacoverage. It answers the process exit
// code.
func CmdLocalDataCoverage(args []string) int {
	if len(args) != 0 {
		fmt.Fprintln(os.Stderr, "local-data-coverage: takes no arguments")
		return 2
	}
	if err := localdatacoverage.Run(os.Stdout); err != nil {
		fmt.Fprintf(os.Stderr, "local-data-coverage: %v\n", err)
		return 1
	}
	return 0
}
