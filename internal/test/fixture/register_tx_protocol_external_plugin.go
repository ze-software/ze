// Design: docs/architecture/config/transaction-protocol.md -- the participant protocol across a process boundary
// Related: register_config_apply_ordering_coarse_root.go -- the same shape for a root nothing decomposes
//
// The observer behind test/reload/tx-protocol-external-plugin.ci.
//
// It runs as its own process and declares the `bgp` root, which the iface and
// bgp decomposers do decompose. So one transaction carries both kinds of node:
// the decomposed peer operations the bgp root produces, and this plugin's
// coarse node. The line it prints is what proves the second kind crossed the
// process boundary, through `config-apply` rather than through the
// `config-operation-apply` its SDK has no default handler for.
package fixture

import (
	"context"
	"fmt"

	"github.com/ze-software/ze/internal/core/textbuf"
	"github.com/ze-software/ze/pkg/plugin/sdk"
)

func init() {
	Register("reload/tx-protocol-external-plugin-observer", uiDriver(runExternalPluginTransactionObserver))
}

func runExternalPluginTransactionObserver(ctx context.Context) error {
	plugin, err := newObserver("tx-protocol-external-plugin-observer")
	if err != nil {
		return fmt.Errorf("connect external plugin transaction observer: %w", err)
	}
	defer plugin.Close() //nolint:errcheck // Run returns the useful transport error
	plugin.OnConfigApply(func(sections []sdk.ConfigDiffSection) error {
		for _, section := range sections {
			var tb textbuf.Buffer
			tb.Str("OK: external plugin applied through config-apply root=").Str(section.Root).Byte('\n').StdErr() //nolint:errcheck // fixture progress output
		}
		return nil
	})
	return plugin.Run(ctx, sdk.Registration{WantsConfig: []string{namespaceBGP}})
}
