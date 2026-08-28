package cli

import (
	"fmt"
	"os"

	"github.com/ze-software/ze/internal/test/localdatacoverage"
)

func init() {
	registerRoot("local-data-coverage", cmdLocalDataCoverage, "Exercise every local data command and its output contract")
}

func cmdLocalDataCoverage(args []string) int {
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
