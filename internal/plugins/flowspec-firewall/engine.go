// Design: docs/guide/flowspec-protected-router.md -- selected FlowSpec installation
// Related: selected.go -- validated winning-route consumer
package flowspecfirewall

import (
	"errors"
	"fmt"
	"log/slog"
	"net"
	"sync"
	"sync/atomic"

	"github.com/ze-software/ze/internal/component/firewall"
	"github.com/ze-software/ze/internal/core/bgp/ribevents"
	"github.com/ze-software/ze/internal/core/replay"
	"github.com/ze-software/ze/internal/core/slogutil"
	sdk "github.com/ze-software/ze/pkg/plugin/sdk"
)

const maxRulesDefault = 1000

var loggerPtr atomic.Pointer[slog.Logger]

func init() { //nolint:gochecknoinits // logger bootstrap only
	loggerPtr.Store(slogutil.DiscardLogger())
}

func logger() *slog.Logger { return loggerPtr.Load() }

func setLogger(l *slog.Logger) {
	if l != nil {
		loggerPtr.Store(l)
	}
}

type bridge struct {
	selectedMu sync.Mutex
	stopped    bool
	rules      *ruleMap
	log        *slog.Logger
}

func newBridge(log *slog.Logger) *bridge {
	return &bridge{rules: newRuleMap(maxRulesDefault), log: log}
}

func (b *bridge) applyRules() {
	if err := firewall.RegisterTables("flowspec", b.rules.buildTable()); err != nil {
		b.log.Error("flowspec: firewall register failed", "error", err)
		return
	}
	if err := firewall.ApplyAll(); err != nil {
		b.log.Warn("flowspec: firewall apply failed", "error", err)
	}
}

// clearStaleRules takes ownership of the previous process's table before
// removing it. A fresh nft backend cannot remove an unclaimed ze_* table.
func clearStaleRules() error {
	claim := firewall.Table{Name: tableName, Family: firewall.FamilyInet}
	if err := firewall.RegisterTables("flowspec", []firewall.Table{claim}); err != nil {
		return fmt.Errorf("claiming the FlowSpec table: %w", err)
	}
	if err := firewall.ApplyAll(); err != nil {
		_ = firewall.RegisterTables("flowspec", nil) // An empty registration cannot fail name validation.
		return fmt.Errorf("clearing stale FlowSpec rules: %w", err)
	}
	_ = firewall.RegisterTables("flowspec", nil) // An empty registration cannot fail name validation.
	if err := firewall.ApplyAll(); err != nil {
		return fmt.Errorf("removing the empty FlowSpec table: %w", err)
	}
	return nil
}

// removeRules keeps selected-route delivery live when removal fails. Holding
// selectedMu prevents a concurrent update from repopulating the withdrawal.
func (b *bridge) removeRules() error {
	b.selectedMu.Lock()
	defer b.selectedMu.Unlock()
	_ = firewall.RegisterTables("flowspec", nil) // An empty registration cannot fail name validation.
	if err := firewall.ApplyAll(); err != nil {
		restoreErr := firewall.RegisterTables("flowspec", b.rules.buildTable())
		if restoreErr == nil {
			restoreErr = firewall.ApplyAll()
		}
		return errors.Join(err, restoreErr)
	}
	b.stopped = true
	return nil
}

func runEngine(conn net.Conn) int {
	log := logger()
	p := sdk.NewWithConn("flowspec-firewall", conn)
	defer func() { _ = p.Close() }()
	b := newBridge(log)
	eb := getEventBusRef()
	if eb == nil {
		log.Error("flowspec-firewall requires the BGP RIB event bus")
		return 1
	}
	var unsubscribe func()
	initialized := false
	stop := func() {
		if unsubscribe != nil {
			unsubscribe()
		}
		b.selectedMu.Lock()
		defer b.selectedMu.Unlock()
		b.stopped = true
	}
	defer stop()
	p.OnBye(func(reason string) error {
		if reason != "removed" {
			return nil
		}
		// Removal runs while the firewall dependency is still alive. Failure
		// keeps the subscription and prior desired rules available to rollback.
		if err := b.removeRules(); err != nil {
			return err
		}
		if unsubscribe != nil {
			unsubscribe()
		}
		return nil
	})
	p.OnConfigure(func(_ []sdk.ConfigSection) error {
		// Configure also compensates a failed removal. The existing selection
		// and subscription remain live in that case and must not be reset.
		if initialized {
			return nil
		}
		// Dependencies finish configuration before this callback. Replaying
		// earlier could autoload nft before the configured backend is ready.
		if err := clearStaleRules(); err != nil {
			return err
		}
		b.selectedMu.Lock()
		b.stopped = false
		b.selectedMu.Unlock()
		unsubscribe = ribevents.FlowSpecChanged.Subscribe(eb, b.handleSelected)
		if _, err := ribevents.ReplayRequest.Emit(eb, &replay.Request{ReplayID: replay.Broadcast}); err != nil {
			unsubscribe()
			unsubscribe = nil
			b.selectedMu.Lock()
			b.stopped = true
			b.rules = newRuleMap(maxRulesDefault)
			_ = firewall.RegisterTables("flowspec", nil) // An empty registration cannot fail name validation.
			cleanupErr := firewall.ApplyAll()
			b.selectedMu.Unlock()
			return errors.Join(err, cleanupErr)
		}
		initialized = true
		return nil
	})
	ctx, cancel := sdk.SignalContext()
	defer cancel()
	if err := p.Run(ctx, sdk.Registration{}); err != nil {
		log.Error("flowspec-firewall plugin failed", "error", err)
		return 1
	}
	log.Info("flowspec-firewall plugin stopped")
	return 0
}
