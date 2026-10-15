// Design: docs/architecture/config/transaction-protocol.md -- a plugin's verify, apply and rollback of one parsed config
// Related: register.go -- runGeoDNSPlugin, which registers the transaction

package geodns

import (
	"fmt"
	"log/slog"
	"time"

	sdk "github.com/ze-software/ze/pkg/plugin/sdk"
)

// parseGeoDNSSections reads the service section of one delivery. present is
// false when the delivery carries no service section. It changes no state.
func parseGeoDNSSections(sections []sdk.ConfigSection) (geodnsConfig, bool, error) {
	for _, s := range sections {
		if s.Root != configRootService {
			continue
		}
		cfg, err := parseConfig(s.Data)
		if err != nil {
			return geodnsConfig{}, true, fmt.Errorf("geodns: %w", err)
		}
		return cfg, true, nil
	}
	return geodnsConfig{}, false, nil
}

// registerGeoDNSConfigTx binds tx to the plugin's configure, verify, apply and
// rollback, counting a refused configure or verify in reloadTotal{error}.
// sdk.ConfigTx installs nothing when the parsed config equals the running one,
// so an edit elsewhere under service restarts no listener and counts nothing.
func registerGeoDNSConfigTx(p *sdk.Plugin, tx *sdk.ConfigTx[geodnsConfig]) {
	p.OnConfigure(countGeoDNSError(tx.Configure))
	p.OnConfigVerify(countGeoDNSError(tx.Verify))
	p.OnConfigApply(func(_ []sdk.ConfigDiffSection) error { return tx.Apply() })
	p.OnConfigRollback(func(_ string) error { return tx.Rollback() })
}

func countGeoDNSError(step func([]sdk.ConfigSection) error) func([]sdk.ConfigSection) error {
	return func(sections []sdk.ConfigSection) error {
		err := step(sections)
		if err != nil {
			gmetrics().reloadTotal.With("error").Inc()
		}
		return err
	}
}

// geodnsInstaller makes a parsed config the running one: it computes the SOA
// serial for the generation, publishes the resolver snapshot and reconciles
// the UDP and TCP listeners (rebinding only when the endpoint set changes).
// Called under the ConfigTx lock; not safe for concurrent use on its own.
type geodnsInstaller struct {
	mgr *geodnsServer
	log *slog.Logger
}

// install applies cfg. It cannot fail: a listener failure is logged, as at
// boot, because the snapshot already serves the new config.
func (in *geodnsInstaller) install(cfg geodnsConfig) error {
	var prevSerial uint32
	if prev := loadState(); prev != nil {
		prevSerial = prev.serial
	}
	st := buildState(cfg)
	st.serial = computeSerial(cfg.SOA, prevSerial, time.Now())
	storeState(st)
	if aerr := in.mgr.apply(cfg); aerr != nil {
		in.log.Error("geodns: listener setup failed", "error", aerr)
	}
	gmetrics().reloadTotal.With("success").Inc()
	in.log.Info("geodns: config applied",
		"enabled", cfg.Enabled,
		"listeners", len(cfg.Listeners),
		"zones", len(cfg.Zones),
		"host-sets", len(cfg.HostSets),
		"sources", len(cfg.Sources))
	return nil
}
