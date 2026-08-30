package fixture

import (
	"context"
	"errors"
	"fmt"

	"github.com/ze-software/ze/pkg/plugin/sdk"
)

func init() {
	Register("ui/web-commit-reject", uiDriver(runWebCommitReject))
}

func runWebCommitReject(ctx context.Context) error {
	plugin, err := newObserver("web-commit-reject")
	if err != nil {
		return fmt.Errorf("connect web reject plugin: %w", err)
	}
	defer plugin.Close() //nolint:errcheck // Run returns the useful transport error
	plugin.OnConfigApply(func([]sdk.ConfigDiffSection) error {
		return errors.New("web test apply rejected")
	})
	return plugin.Run(ctx, sdk.Registration{WantsConfig: []string{namespaceBGP}})
}
