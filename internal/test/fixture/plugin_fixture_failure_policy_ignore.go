// Design: docs/architecture/doctor-and-health-checks.md -- plugin state a runtime check reads
// Related: plugin_fixture_failure_policy.go -- failurePolicyRun, the shared run this driver uses

package fixture

import (
	"context"
	"errors"
	"fmt"
	"os"

	"github.com/ze-software/ze/pkg/plugin/sdk"
)

// failurePolicyIgnoreDriver declares ignore and exits once it is running. The
// daemon must keep running without it, and `show doctor` must name it under
// doctor-plugin-stopped.
func failurePolicyIgnoreDriver(ctx context.Context, args []string) error {
	if len(args) != 0 {
		return errors.New("plugin/failure-policy-ignore takes no arguments")
	}
	return failurePolicyRun(ctx, "failure-ignore", sdk.FailureIgnore,
		func(context.Context, *sdk.Plugin) error {
			fmt.Fprintln(os.Stderr, "FIXTURE: failure-ignore is exiting")
			return nil
		})
}
