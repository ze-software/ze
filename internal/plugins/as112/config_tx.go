// Design: docs/architecture/config/transaction-protocol.md -- a plugin's verify, apply and rollback of one parsed config
// Related: register.go -- runAS112Plugin, which registers the transaction

package as112

import (
	"fmt"
	"log/slog"
	"time"

	"github.com/ze-software/ze/internal/component/iface"
	"github.com/ze-software/ze/internal/core/dnsserver"
	sdk "github.com/ze-software/ze/pkg/plugin/sdk"
)

// parseAS112Sections reads the service section of one delivery. present is
// false when the delivery carries no service section. It changes no state.
func parseAS112Sections(sections []sdk.ConfigSection) (as112Config, bool, error) {
	for _, s := range sections {
		if s.Root != configRootService {
			continue
		}
		cfg, err := parseConfig(s.Data)
		if err != nil {
			return as112Config{}, true, fmt.Errorf("as112: %w", err)
		}
		return cfg, true, nil
	}
	return as112Config{}, false, nil
}

// registerAS112ConfigTx binds tx to the plugin's configure, verify, apply and
// rollback, counting a refused configure or verify in reloadTotal{error}.
// sdk.ConfigTx installs nothing when the parsed config equals the running one,
// so an edit elsewhere under service restarts no listener and counts nothing.
// The configure installs through in at boot, with lenient binds.
func registerAS112ConfigTx(p *sdk.Plugin, tx *sdk.ConfigTx[as112Config], in *as112Installer) {
	p.OnConfigure(countAS112Error(func(sections []sdk.ConfigSection) error {
		return in.configureAtBoot(tx, sections)
	}))
	p.OnConfigVerify(countAS112Error(tx.Verify))
	p.OnConfigApply(func(_ []sdk.ConfigDiffSection) error { return tx.Apply() })
	p.OnConfigRollback(func(_ string) error { return tx.Rollback() })
}

func countAS112Error(step func([]sdk.ConfigSection) error) func([]sdk.ConfigSection) error {
	return func(sections []sdk.ConfigSection) error {
		err := step(sections)
		if err != nil {
			ametrics().reloadTotal.With("error").Inc()
		}
		return err
	}
}

// as112Installer makes a parsed config the running one: the loopback address
// registration, the resolver snapshot, the redistribute producer and the
// listeners. Called under the ConfigTx lock; not safe for concurrent use on
// its own.
type as112Installer struct {
	prod *as112Producer
	mgr  *as112Server
	log  *slog.Logger

	// running is the config the last install made running, which a reload
	// whose listeners cannot be bound installs back.
	running as112Config
	// boot is true only while the Stage 2 configure installs, whose binds are
	// lenient (dnsserver.BindLenient): no previous config exists to keep, and
	// the anycast addresses can appear after the daemon.
	boot bool
}

// configureAtBoot runs the Stage 2 configure of tx with lenient binds, so a
// listener that cannot be bound at boot is logged and the rest serve.
func (in *as112Installer) configureAtBoot(tx *sdk.ConfigTx[as112Config], sections []sdk.ConfigSection) error {
	in.boot = true
	defer func() { in.boot = false }()
	return tx.Configure(sections)
}

// install applies cfg. A failed address registration returns before anything
// else changed, so the running config stays as it was. On a reload a listener
// that cannot be bound, or DoT/DoH TLS material that cannot be loaded, fails
// the install: dnsserver binds the previous listeners again, and the
// registration, snapshot and producer are put back, so the running config
// keeps serving and the commit is refused (R-13). At boot such a failure is
// logged, and what bound serves.
func (in *as112Installer) install(cfg as112Config) error {
	if rerr := applyAddressRegistration(cfg, iface.RegisterOwnedAddresses, iface.UnregisterOwnedAddresses); rerr != nil {
		return fmt.Errorf("as112: address registration: %w", rerr)
	}

	prev := loadState()
	var prevSerial uint32
	if prev != nil {
		prevSerial = prev.serial
	}
	serial := computeSerial(prevSerial, time.Now())
	storeState(buildState(cfg, serial))

	// Apply the config to the producer; the runtime serving state (anycast
	// listener up/down) is driven separately via prod.onServingChanged from
	// the DNS server's listener transitions. `import as112` gates whether the
	// emitted routes reach the RIB, so as112 still never reads bgp config.
	in.prod.applyConfig(cfg)
	policy := dnsserver.BindStrict
	if in.boot {
		policy = dnsserver.BindLenient
	}
	if aerr := in.mgr.applyConfig(cfg, policy); aerr != nil {
		if !in.boot {
			in.restore(prev)
			ametrics().reloadTotal.With("error").Inc()
			return fmt.Errorf("as112: %w", aerr)
		}
		in.log.Error("as112: listener setup failed", "error", aerr)
	}
	in.running = cfg
	ametrics().reloadTotal.With("success").Inc()
	in.log.Info("as112: config applied", "enabled", cfg.Enabled, "address-family", cfg.AddressFamily,
		"dot", cfg.Secure.DoTEnabled, "doh", cfg.Secure.DoHEnabled)
	return nil
}

// restore puts back what a refused reload changed before its listeners
// failed: the producer's config, the snapshot and the address registration of
// the running config. dnsserver already bound the running listeners again.
func (in *as112Installer) restore(prev *as112State) {
	in.prod.applyConfig(in.running)
	storeState(prev)
	if rerr := applyAddressRegistration(in.running, iface.RegisterOwnedAddresses, iface.UnregisterOwnedAddresses); rerr != nil {
		in.log.Error("as112: restoring the running address registration failed", "error", rerr)
	}
}
