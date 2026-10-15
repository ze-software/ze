// Design: docs/architecture/config/transaction-protocol.md -- a plugin's verify, apply and rollback of one parsed config

package fakeddos

import (
	"sync/atomic"
	"testing"
	"time"

	"github.com/ze-software/ze/internal/core/ddosevent"
	sdk "github.com/ze-software/ze/pkg/plugin/sdk"
	"github.com/ze-software/ze/pkg/ze"
)

func fakeSection(enabled string) []sdk.ConfigSection {
	return []sdk.ConfigSection{{Root: configRoot, Data: `{"ddos":{"fake":{"enabled":"` + enabled + `"}}}`}}
}

// TestConfigTxReloadStartsAndStopsTheInjector proves row 32 for the fakeddos
// fixture: a reload that turns `enabled` on starts the injector at apply, not
// at verify, and its rollback stops it again. Method: the injector's stop
// handle is read after each step, and the bus is watched for the first
// attack-detected event.
// VALIDATES: spec-plugin-reload-applies-or-refuses row 32, AC-1 to AC-4.
// PREVENTS: the fixture ignoring a reload once Phase 5 refuses a participant
// with no apply handler.
// Not parallel: eventBusPtr is package state.
func TestConfigTxReloadStartsAndStopsTheInjector(t *testing.T) {
	bus := newCountingBus()
	var eb ze.EventBus = bus
	eventBusPtr.Store(&eb)
	t.Cleanup(func() { eventBusPtr.Store(nil) })
	var detected atomic.Int32
	ddosevent.Detected.Subscribe(bus, func(*ddosevent.AttackDetected) { detected.Add(1) })

	inj := &injector{lifetime: t.Context()}
	tx := sdk.NewConfigTx(false, activationFromSections, inj.install)
	if err := tx.Configure(fakeSection("false")); err != nil {
		t.Fatalf("configure: %v", err)
	}
	if err := tx.Verify(fakeSection("true")); err != nil {
		t.Fatalf("verify: %v", err)
	}
	if inj.stop != nil {
		t.Fatal("verify started the injector")
	}
	if err := tx.Apply(); err != nil {
		t.Fatalf("apply: %v", err)
	}
	if inj.stop == nil {
		t.Fatal("apply of enabled true left the injector inert")
	}
	deadline := time.Now().Add(2 * time.Second)
	for detected.Load() == 0 && time.Now().Before(deadline) {
		time.Sleep(10 * time.Millisecond)
	}
	if detected.Load() == 0 {
		t.Fatal("the started injector emitted no attack-detected event")
	}
	if err := tx.Rollback(); err != nil {
		t.Fatalf("rollback: %v", err)
	}
	if inj.stop != nil {
		t.Fatal("rollback left the injector running")
	}
}
