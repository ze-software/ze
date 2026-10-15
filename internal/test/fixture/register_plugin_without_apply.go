// Design: docs/architecture/config/transaction-protocol.md -- a participant with no apply handler is refused at verify
// Related: register_config_apply_ordering_rollback.go -- observers that declare they apply nothing
//
// The plugin behind test/reload/plugin-without-apply-refused.ci: it wants the
// bgp root and registers neither OnConfigApply nor DeclareAppliesNothing, so
// the SDK refuses every reload that changes bgp at config-verify.
package fixture

import (
	"context"
	"fmt"

	"github.com/ze-software/ze/pkg/plugin/sdk"
)

func init() {
	Register("reload/plugin-without-apply", uiDriver(runPluginWithoutApply))
}

// pluginWithoutApplyName is the name the refusal carries; the test's config
// gives the external plugin the same name.
const pluginWithoutApplyName = "plugin-without-apply"

// runPluginWithoutApply connects, wants bgp, and registers no way to apply a
// change: the SDK default is the only answer a reload gets from it.
func runPluginWithoutApply(ctx context.Context) error {
	plugin, err := newObserver(pluginWithoutApplyName)
	if err != nil {
		return fmt.Errorf("connect %s: %w", pluginWithoutApplyName, err)
	}
	defer plugin.Close() //nolint:errcheck // Run returns the useful transport error
	return plugin.Run(ctx, sdk.Registration{WantsConfig: []string{namespaceBGP}})
}
