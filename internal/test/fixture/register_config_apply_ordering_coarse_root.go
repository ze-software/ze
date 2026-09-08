// Design: docs/architecture/config/apply-ordering.md -- a root with no decomposer
// Related: misc_fixture_shellports.go -- reloadSignalDriver, the shared rewrite-and-SIGHUP trigger
// Related: ../../component/config/transaction/orchestrator.go -- operationNodes, the coarse node this observes
//
// The observer behind test/reload/config-apply-ordering-coarse-root.ci. It declares
// a config root no component decomposes, so the transaction represents it with one
// coarse node and applies it through the section apply. The line it prints on
// config-apply is what the test asserts: a participant the operation path leaves
// out receives no apply at all, and prints nothing.
package fixture

import (
	"context"
	"fmt"

	"github.com/ze-software/ze/internal/core/textbuf"
	"github.com/ze-software/ze/pkg/plugin/sdk"
)

func init() {
	Register("reload/config-apply-ordering-coarse-root", uiDriver(runCoarseRootObserver))
}

func runCoarseRootObserver(ctx context.Context) error {
	plugin, err := newObserver("config-apply-ordering-coarse-root")
	if err != nil {
		return fmt.Errorf("connect coarse root observer: %w", err)
	}
	defer plugin.Close() //nolint:errcheck // Run returns the useful transport error
	plugin.OnConfigApply(func(sections []sdk.ConfigDiffSection) error {
		for _, section := range sections {
			var tb textbuf.Buffer
			tb.Str("OK: coarse root applied root=").Str(section.Root).Byte('\n').StdErr() //nolint:errcheck // fixture progress output
		}
		return nil
	})
	return plugin.Run(ctx, sdk.Registration{WantsConfig: []string{"rsvp-te"}})
}
