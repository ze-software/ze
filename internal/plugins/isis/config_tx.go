// Design: docs/architecture/isis/isis-4-component-config.md -- one parsed runtime transaction.
// Related: register.go -- startup staging and runtime installation.
package isis

import (
	"fmt"

	"github.com/ze-software/ze/internal/core/opererr"
	"github.com/ze-software/ze/internal/core/textbuf"
	"github.com/ze-software/ze/pkg/plugin/sdk"
)

// runtimeConfig is the typed protocol config and the compiled export chain that
// must move together. Its zero is the unconfigured, idle engine with no export
// chain. An owned root with no NET is also valid and keeps the engine idle.
type runtimeConfig struct {
	cfg    Config
	export exportChainConfig
}

// parseRuntimeConfig builds a candidate without changing the running engine or
// export chain. A missing owned root leaves the running value unchanged; an
// explicitly empty section removes the config. ConfigReads delivers the policy
// section with IS-IS so export filters compile against the same candidate.
func parseRuntimeConfig(sections []sdk.ConfigSection) (runtimeConfig, bool, error) {
	present := false
	for _, section := range sections {
		if section.Root == configRoot {
			present = true
		}
	}
	if !present {
		return runtimeConfig{}, false, nil
	}
	values := toConfigSections(sections)
	cfg, err := parseISISConfig(values)
	if err != nil {
		return runtimeConfig{}, false, err
	}
	if cfg.Present() {
		if err := validateConfig(cfg); err != nil {
			return runtimeConfig{}, false, err
		}
	} else if err := validateInterfaceLevels(cfg); err != nil {
		return runtimeConfig{}, false, err
	}
	export, err := parseExportChain(values)
	if err != nil {
		return runtimeConfig{}, false, err
	}
	return runtimeConfig{cfg: cfg, export: export}, true, nil
}

// circuitOpenError retains the backend cause while naming the configured
// interface and the readiness command that diagnoses its dependency failure.
func circuitOpenError(name string, cause error) error {
	var tb textbuf.Buffer
	return opererr.Wrap(cause,
		tb.Str("IS-IS cannot open interface ").Str(name).String(),
		"The requested configuration cannot become active without its raw circuit.",
		"ze doctor")
}

// runningConfigLost admits the SDK compensation path only when installing the
// candidate and restoring the running config both failed. The first failure is
// retained beside the repair failure, so an operator can diagnose both.
func runningConfigLost(applyErr, undoErr error) error {
	return sdk.RunningConfigLost(fmt.Errorf("%w; restoring the running IS-IS config: %w", applyErr, undoErr),
		"IS-IS could not install the requested config or restore its running circuits",
		"run `ze doctor` and fix the interface or raw-socket problem named above")
}
