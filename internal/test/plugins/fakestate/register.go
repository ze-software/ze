// Design: docs/architecture/storage/zefs.md -- durable plugin state across restart
// Related: ../../../test/fixture/storage_tree_fixture.go -- storageStateRestart drives this probe

// Package fakestate registers a test-only compiled-in plugin with an explicit
// statestore grant so test/plugin/statestore-on-tree.ci can verify durable
// StatePut/StateGet/StateRemove across daemon restarts without spoofing a
// real plugin's identity from an external process.
package fakestate

import (
	"context"
	"fmt"
	"net"
	"os"
	"time"

	"github.com/ze-software/ze/internal/component/plugin/registry"
	"github.com/ze-software/ze/internal/core/statestore"
	"github.com/ze-software/ze/internal/core/textbuf"
	"github.com/ze-software/ze/pkg/plugin/sdk"
	"github.com/ze-software/ze/pkg/zefs"
)

// Name is the compiled-in test plugin name configured by statestore-on-tree.
const Name = "fake-state-probe"

const stateKey = "meta/ospf/gr-fact-storage-fixture"

func init() {
	statestore.RegisterPluginKeys(Name, zefs.KeyOSPFGRFact)
	reg := registry.Registration{
		Name:        Name,
		Description: "Test-only compiled-in plugin for statestore restart durability (statestore-on-tree)",
		Sends: []sdk.MessageSendDecl{
			{Message: "cli:request shutdown"},
		},
		RunEngine:  runPlugin,
		CLIHandler: func(_ []string) int { return 1 },
	}
	if err := registry.Register(reg); err != nil {
		panic("BUG: " + Name + " registration failed: " + err.Error())
	}
}

func runPlugin(conn net.Conn) int {
	p := sdk.NewWithConn(Name, conn)
	defer p.Close() //nolint:errcheck // best-effort teardown
	mode := os.Getenv("ZE_FAKE_STATE_MODE")
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	done := make(chan struct{})
	p.OnAllPluginsReady(func() error {
		go runStateProbe(ctx, p, mode, done)
		return nil
	})
	runErr := p.Run(ctx, &sdk.Registration{
		Sends: []sdk.MessageSendDecl{{Message: "cli:request shutdown"}},
	})
	cancel()
	select {
	case <-done:
	default:
	}
	if runErr != nil {
		return 1
	}
	return 0
}

func runStateProbe(ctx context.Context, p *sdk.Plugin, mode string, done chan struct{}) {
	defer close(done)
	var tb textbuf.Buffer
	if err := exerciseState(ctx, p, mode); err != nil {
		tb.Str("ZE-OBSERVER-FAIL: fake-state-probe (").Str(mode).Str("): ").Err(err).Byte('\n')
	} else {
		tb.Str("OK: fake-state-probe ").Str(mode).Byte('\n')
	}
	_, _ = os.Stderr.WriteString(tb.String())
	shutdownCtx, shutdownCancel := context.WithTimeout(ctx, 5*time.Second)
	defer shutdownCancel()
	_, _, _ = p.DispatchCommand(shutdownCtx, "request shutdown")
}

func exerciseState(ctx context.Context, p *sdk.Plugin, mode string) error {
	if mode != "write" && mode != "read" {
		return fmt.Errorf("ZE_FAKE_STATE_MODE=%q, want write or read", mode)
	}
	if mode == "write" {
		_, found, err := p.StateGet(ctx, stateKey)
		if err != nil {
			return fmt.Errorf("new state key: err=%w", err)
		}
		if found {
			return fmt.Errorf("new state key: unexpectedly found")
		}
		if err := p.StatePut(ctx, stateKey, []byte("durable fixture state")); err != nil {
			return err
		}
	}
	value, found, err := p.StateGet(ctx, stateKey)
	if err != nil {
		return fmt.Errorf("persisted state: err=%w", err)
	}
	if !found {
		return fmt.Errorf("persisted state: not found")
	}
	if string(value) != "durable fixture state" {
		return fmt.Errorf("persisted state: value=%q", value)
	}
	if mode == "read" {
		if err := p.StateRemove(ctx, stateKey); err != nil {
			return err
		}
		_, found, err := p.StateGet(ctx, stateKey)
		if err != nil {
			return fmt.Errorf("removed state check: err=%w", err)
		}
		if found {
			return fmt.Errorf("removed state still present")
		}
	}
	return nil
}
