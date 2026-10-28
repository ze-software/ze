// Design: docs/architecture/config/transaction-protocol.md -- a plugin's verify, apply and rollback of one parsed config
// Related: register.go -- runResolverPlugin, which registers the transaction

package dnsresolver

import (
	"errors"
	"fmt"
	"log/slog"

	"github.com/ze-software/ze/internal/core/dnsserver"
	sdk "github.com/ze-software/ze/pkg/plugin/sdk"
)

// parseResolverSections reads the service section of one delivery. present
// is false when the delivery carries no service section. It changes no state.
func parseResolverSections(sections []sdk.ConfigSection) (resolverConfig, bool, error) {
	for _, s := range sections {
		if s.Root != configRootService {
			continue
		}
		cfg, err := parseConfig(s.Data)
		if err != nil {
			return resolverConfig{}, true, fmt.Errorf("dns-resolver: %w", err)
		}
		return cfg, true, nil
	}
	return resolverConfig{}, false, nil
}

// registerResolverConfigTx binds tx to the plugin's configure, verify, apply
// and rollback. sdk.ConfigTx installs nothing when the parsed config equals
// the running one. The configure installs through in at boot, with lenient
// binds.
func registerResolverConfigTx(p *sdk.Plugin, tx *sdk.ConfigTx[resolverConfig], in *resolverInstaller) {
	p.OnConfigure(func(sections []sdk.ConfigSection) error { return in.configureAtBoot(tx, sections) })
	p.OnConfigVerify(tx.Verify)
	p.OnConfigApply(func(_ []sdk.ConfigDiffSection) error { return tx.Apply() })
	p.OnConfigRollback(func(_ string) error { return tx.Rollback() })
}

// resolverInstaller makes a parsed config the running one by reconciling
// the listeners. Called under the ConfigTx lock; not safe for concurrent use
// on its own.
type resolverInstaller struct {
	srv *resolverServer
	res *resolver
	log *slog.Logger

	// boot is true only while the Stage 2 configure installs, whose binds are
	// lenient (dnsserver.BindLenient): no previous config exists to keep.
	boot bool
}

// configureAtBoot runs the Stage 2 configure of tx with lenient binds, so a
// listener that cannot be bound at boot is logged and the rest serve.
func (in *resolverInstaller) configureAtBoot(tx *sdk.ConfigTx[resolverConfig], sections []sdk.ConfigSection) error {
	in.boot = true
	defer func() { in.boot = false }()
	return tx.Configure(sections)
}

// install applies cfg. On a reload a listener that cannot be bound fails the
// install: dnsserver binds the previous listeners again and the running
// policy is published again, so the running config keeps serving, show
// reports it, and the commit is refused. When a previous listener cannot be
// bound again the error wraps sdk.ErrRunningConfigLost, so the
// transaction's rollback installs the running config again or restarts the
// plugin. At boot such a failure is logged, and what bound serves.
//
// While a reload changes the listeners, the resolver answers under the
// running policy narrowed to the sources both allow-from lists admit
// (resolver.narrow). A listener the reload binds then never answers a
// source the running config refuses, a listener it keeps never answers a
// source the candidate refuses, and a refused reload never serves under the
// candidate's forward list or limits. The candidate is published whole only
// once every listener it names is bound.
func (in *resolverInstaller) install(cfg resolverConfig) error {
	if in.boot {
		// No previous config exists to keep, and the binds are lenient.
		in.res.apply(&cfg)
		if err := in.srv.reconcile(cfg, dnsserver.BindLenient); err != nil {
			in.log.Error("dns-resolver: listener setup failed", "error", err)
		}
		in.applied(cfg)
		return nil
	}
	running := in.res.narrow(&cfg)
	if err := in.srv.reconcile(cfg, dnsserver.BindStrict); err != nil {
		in.res.restore(running)
		if running.shown != nil {
			// The candidate's own listeners were unbound again.
			in.res.listeners.prune(running.shown.Listeners)
		}
		if errors.Is(err, dnsserver.ErrPreviousListenersLost) {
			return fmt.Errorf("dns-resolver: %w (%w).\n%s", err, sdk.ErrRunningConfigLost, sdk.RunningConfigLostNext)
		}
		return fmt.Errorf("dns-resolver: %w", err)
	}
	in.res.listeners.prune(cfg.Listeners)
	in.res.apply(&cfg)
	in.applied(cfg)
	return nil
}

// applied logs that cfg is the running config.
func (in *resolverInstaller) applied(cfg resolverConfig) {
	in.log.Info("dns-resolver: config applied", "enabled", cfg.Enabled,
		"listeners", len(cfg.Listeners), "allow-from", len(cfg.AllowFrom), "forward", len(cfg.Forward))
}
