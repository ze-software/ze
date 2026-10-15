// Design: docs/architecture/config/transaction-protocol.md -- a plugin's verify, apply and rollback of one parsed config
// Related: register.go -- runAS112Plugin, which registers the transaction

package as112

import (
	"errors"
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
// registration, the resolver snapshot, the listeners and the redistribute
// producer. Called under the ConfigTx lock; not safe for concurrent use on
// its own.
type as112Installer struct {
	prod *as112Producer
	mgr  *as112Server
	log  *slog.Logger
	// register and unregister own the loopback address registration
	// (iface.RegisterOwnedAddresses and iface.UnregisterOwnedAddresses).
	register   func(ifaceName, owner string, addrs []string) error
	unregister func(owner string)

	// running is the config the last install made running, which a reload
	// whose listeners cannot be bound installs back.
	running as112Config
	// boot is true only while the Stage 2 configure installs, whose binds are
	// lenient (dnsserver.BindLenient): no previous config exists to keep, and
	// the anycast addresses can appear after the daemon.
	boot bool
}

// newAS112Installer returns the installer over prod and mgr, registering the
// loopback addresses with the iface component.
func newAS112Installer(prod *as112Producer, mgr *as112Server, log *slog.Logger) *as112Installer {
	return &as112Installer{prod: prod, mgr: mgr, log: log,
		register: iface.RegisterOwnedAddresses, unregister: iface.UnregisterOwnedAddresses}
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
// registration and snapshot are put back (refuse), so the running config
// keeps serving and the commit is refused (R-13). At boot such a failure is
// logged, and what bound serves.
//
// The producer takes cfg only after the listeners are bound, so a refused
// reload never announces under the candidate's ASN or community. A reload
// that changes the listener set still stops every listener before it binds
// the new set, so the producer withdraws the covering prefixes while no
// listener serves and announces them again once one is up (RFC 7534 Section
// 3.3 ties the announcement to serving).
func (in *as112Installer) install(cfg as112Config) error {
	if rerr := applyAddressRegistration(cfg, in.register, in.unregister); rerr != nil {
		return fmt.Errorf("as112: address registration: %w", rerr)
	}

	prev := loadState()
	var prevSerial uint32
	if prev != nil {
		prevSerial = prev.serial
	}
	serial := computeSerial(prevSerial, time.Now())
	storeState(buildState(cfg, serial))

	policy := dnsserver.BindStrict
	if in.boot {
		policy = dnsserver.BindLenient
	}
	if aerr := in.mgr.applyConfig(cfg, policy); aerr != nil {
		if !in.boot {
			ametrics().reloadTotal.With("error").Inc()
			return in.refuse(prev, aerr)
		}
		in.log.Error("as112: listener setup failed", "error", aerr)
	}
	// The runtime serving state (anycast listener up/down) is driven
	// separately via prod.onServingChanged from the DNS server's listener
	// transitions. `import as112` gates whether the emitted routes reach the
	// RIB, so as112 still never reads bgp config.
	in.prod.applyConfig(cfg)
	in.running = cfg
	ametrics().reloadTotal.With("success").Inc()
	in.log.Info("as112: config applied", "enabled", cfg.Enabled, "address-family", cfg.AddressFamily,
		"dot", cfg.Secure.DoTEnabled, "doh", cfg.Secure.DoHEnabled)
	return nil
}

// refuse puts back what a reload refused with cause changed before its
// listeners failed: the snapshot and the address registration of the running
// config (the producer never took the candidate, and dnsserver already bound
// the running listeners again). It answers the install's error, which wraps
// sdk.ErrRunningConfigLost when the running listeners or the registration
// could not be put back, so the transaction's rollback installs the running
// config again or restarts the plugin.
func (in *as112Installer) refuse(prev *as112State, cause error) error {
	storeState(prev)
	if rerr := applyAddressRegistration(in.running, in.register, in.unregister); rerr != nil {
		return fmt.Errorf("as112: %w; the running config's loopback address registration could not be "+
			"restored either (%w), %w", cause, rerr, sdk.ErrRunningConfigLost)
	}
	if errors.Is(cause, dnsserver.ErrPreviousListenersLost) {
		return fmt.Errorf("as112: %w (%w)", cause, sdk.ErrRunningConfigLost)
	}
	return fmt.Errorf("as112: %w", cause)
}
