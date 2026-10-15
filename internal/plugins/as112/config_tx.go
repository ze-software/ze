// Design: docs/architecture/config/transaction-protocol.md -- a plugin's verify, apply and rollback of one parsed config
// Related: register.go -- runAS112Plugin, which registers the transaction

package as112

import (
	"fmt"
	"log/slog"
	"time"

	"github.com/ze-software/ze/internal/component/iface"
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
func registerAS112ConfigTx(p *sdk.Plugin, tx *sdk.ConfigTx[as112Config]) {
	p.OnConfigure(countAS112Error(tx.Configure))
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
}

// install applies cfg. A failed address registration returns before anything
// else changed, so the running config stays as it was. A listener failure is
// logged, not returned, as at boot: the snapshot and producer already serve
// the new config.
func (in *as112Installer) install(cfg as112Config) error {
	if rerr := applyAddressRegistration(cfg, iface.RegisterOwnedAddresses, iface.UnregisterOwnedAddresses); rerr != nil {
		return fmt.Errorf("as112: address registration: %w", rerr)
	}

	var prevSerial uint32
	if prev := loadState(); prev != nil {
		prevSerial = prev.serial
	}
	serial := computeSerial(prevSerial, time.Now())
	storeState(buildState(cfg, serial))

	// Apply the config to the producer; the runtime serving state (anycast
	// listener up/down) is driven separately via prod.onServingChanged from
	// the DNS server's listener transitions. `import as112` gates whether the
	// emitted routes reach the RIB, so as112 still never reads bgp config.
	in.prod.applyConfig(cfg)
	if aerr := in.mgr.applyConfig(cfg); aerr != nil {
		in.log.Error("as112: listener setup failed", "error", aerr)
	}
	ametrics().reloadTotal.With("success").Inc()
	in.log.Info("as112: config applied", "enabled", cfg.Enabled, "address-family", cfg.AddressFamily,
		"dot", cfg.Secure.DoTEnabled, "doh", cfg.Secure.DoHEnabled)
	return nil
}
