// Design: docs/architecture/config/transaction-protocol.md -- a plugin's verify, apply and rollback of one parsed config
// Related: register.go -- runGeoDNSPlugin, which registers the transaction

package geodns

import (
	"fmt"
	"log/slog"
	"time"

	"github.com/ze-software/ze/internal/core/dnsserver"
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
// The configure installs through in at boot, with lenient binds.
func registerGeoDNSConfigTx(p *sdk.Plugin, tx *sdk.ConfigTx[geodnsConfig], in *geodnsInstaller) {
	p.OnConfigure(countGeoDNSError(func(sections []sdk.ConfigSection) error {
		return in.configureAtBoot(tx, sections)
	}))
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

	// boot is true only while the Stage 2 configure installs, whose binds are
	// lenient (dnsserver.BindLenient): no previous config exists to keep.
	boot bool
}

// configureAtBoot runs the Stage 2 configure of tx with lenient binds, so a
// listener that cannot be bound at boot is logged and the rest serve.
func (in *geodnsInstaller) configureAtBoot(tx *sdk.ConfigTx[geodnsConfig], sections []sdk.ConfigSection) error {
	in.boot = true
	defer func() { in.boot = false }()
	return tx.Configure(sections)
}

// install applies cfg. On a reload a listener that cannot be bound, or
// DoT/DoH TLS material that cannot be loaded, fails the install: dnsserver
// binds the previous listeners again and the previous snapshot is published
// back, so the running config keeps serving and the commit is refused
// (R-13). At boot such a failure is logged, and what bound serves.
func (in *geodnsInstaller) install(cfg geodnsConfig) error {
	prev := loadState()
	var prevSerial uint32
	if prev != nil {
		prevSerial = prev.serial
	}
	st := buildState(cfg)
	st.serial = computeSerial(cfg.SOA, prevSerial, time.Now())
	storeState(st)
	policy := dnsserver.BindStrict
	if in.boot {
		policy = dnsserver.BindLenient
	}
	if aerr := in.mgr.reconcile(cfg, policy); aerr != nil {
		if !in.boot {
			storeState(prev)
			gmetrics().reloadTotal.With("error").Inc()
			return fmt.Errorf("geodns: %w", aerr)
		}
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
