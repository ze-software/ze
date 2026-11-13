// Design: docs/architecture/config/transaction-protocol.md -- applied participants undo after a later failure.
// Related: fixture.go -- observeConfigured owns startup, assertion reporting and shutdown.
// Related: plugin_fixture_12_reload.go -- the shared copy-and-SIGHUP trigger.
package fixture

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/ze-software/ze/internal/core/textbuf"
	"github.com/ze-software/ze/pkg/plugin/sdk"
)

func init() {
	Register("reload/igp-runtime-rollback", uiDriver(runIGPRuntimeRollback))
}

// runIGPRuntimeRollback proves that a failed daemon transaction restores both
// protocol runtimes. Its candidate barrier MUST precede the restored-state
// assertion: otherwise the original, not-yet-reloaded runtime could pass it.
func runIGPRuntimeRollback(ctx context.Context) error {
	ctx, cancel := context.WithTimeout(ctx, 45*time.Second)
	defer cancel()

	// One candidate is deliberately refused. The callback publishes its actual
	// runtime verdict once; the scenario MUST consume it before checking undo.
	candidate := make(chan error, 1)
	failed := false
	setup := func(plugin *sdk.Plugin) error {
		plugin.OnConfigApply(func(_ []sdk.ConfigDiffSection) error {
			if failed {
				return nil
			}
			failed = true
			err := igpRuntimeRollbackState(ctx, plugin, "l1", 20, 29, 6)
			candidate <- err
			if err != nil {
				return err
			}
			var output textbuf.Buffer
			output.Str("OK: IGP rollback candidate reached both protocol runtimes\n").StdErr() //nolint:errcheck // The assertions, not progress output, decide the fixture result.
			return errors.New("igp-runtime-rollback: deliberately refusing after both protocol owners applied")
		})
		return nil
	}
	scenario := func(ctx context.Context, plugin *sdk.Plugin) error {
		if err := igpRuntimeRollbackState(ctx, plugin, "l2", 10, 17, 5); err != nil {
			return fmt.Errorf("initial runtime: %w", err)
		}
		var before igpRuntimeReloadStatus
		if err := igpRuntimeRollbackDispatch(ctx, plugin, "show reload-status", &before); err != nil {
			return err
		}
		if before.Outcome != "none" {
			return fmt.Errorf("initial reload status = %+v, want no processed reload", before)
		}
		// Plugin readiness can precede the parent runner's marker publication.
		// Match p12ReloadTrigger's runner-owned prerequisites before SIGHUP.
		for _, path := range []string{fileDaemonPID, fileDaemonReady} {
			if err := p12WaitForFile(ctx, path); err != nil {
				return fmt.Errorf("wait for daemon startup marker %s: %w", path, err)
			}
		}
		if err := p12CopyFile("config2.conf", "ze-bgp.conf"); err != nil {
			return err
		}
		if err := p12SignalDaemon(fileDaemonPID); err != nil {
			return err
		}
		select {
		case err := <-candidate:
			if err != nil {
				return fmt.Errorf("candidate runtime before refusal: %w", err)
			}
		case <-ctx.Done():
			return fmt.Errorf("candidate runtime was never observed: %w", ctx.Err())
		}
		var after igpRuntimeReloadStatus
		var readErr error
		if !Poll(ctx, 100, 100*time.Millisecond, func() bool {
			readErr = igpRuntimeRollbackDispatch(ctx, plugin, "show reload-status", &after)
			if readErr != nil {
				return false
			}
			return after.Generation > before.Generation
		}) {
			if readErr != nil {
				return fmt.Errorf("read reload completion: %w", readErr)
			}
			return fmt.Errorf("reload completion not observed: before=%+v after=%+v", before, after)
		}
		if after.Outcome != "failed" {
			return fmt.Errorf("reload outcome = %q, want failed", after.Outcome)
		}
		if err := igpRuntimeRollbackState(ctx, plugin, "l2", 10, 17, 5); err != nil {
			return fmt.Errorf("runtime after failed reload completed: %w", err)
		}
		var output textbuf.Buffer
		output.Str("OK: failed reload restored IS-IS level and metric, OSPF cost and instances\n").StdErr() //nolint:errcheck // The state assertions above decide success.
		return nil
	}
	return observeConfigured(ctx, "zz-igp-runtime-rollback", &sdk.Registration{
		WantsConfig: []string{"isis", "ospf"},
	}, setup, scenario)
}

// igpRuntimeReloadStatus is the JSON boundary of the existing reload fence.
// Generation advances after success or failure, never merely when queued.
type igpRuntimeReloadStatus struct {
	Generation uint64 `json:"generation"`
	Outcome    string `json:"last-outcome"`
}

// These rows are the JSON boundaries of the protocol owners' show commands.
// They avoid importing feature-gated protocol implementations into the runner.
type igpRuntimeISISInterface struct {
	Name    string `json:"name"`
	Level   string `json:"level"`
	Metric  uint32 `json:"metric"`
	Passive bool   `json:"passive"`
}

type igpRuntimeOSPFInterface struct {
	Name    string `json:"name"`
	Cost    uint16 `json:"cost"`
	Passive bool   `json:"passive"`
}

type igpRuntimeOSPFInstance struct {
	ID         uint8 `json:"instance-id"`
	Interfaces int   `json:"interface-count"`
}

// igpRuntimeRollbackState reads the protocol owners, not the accepted config
// tree: retaining the old tree alone cannot satisfy these runtime assertions.
func igpRuntimeRollbackState(ctx context.Context, plugin *sdk.Plugin, level string, metric uint32, cost uint16, child uint8) error {
	var isis []igpRuntimeISISInterface
	if err := igpRuntimeRollbackDispatch(ctx, plugin, "show isis interface", &isis); err != nil {
		return err
	}
	if len(isis) != 1 {
		return fmt.Errorf("IS-IS interfaces = %+v, want one passive lo", isis)
	}
	wantISIS := igpRuntimeISISInterface{Name: "lo", Level: level, Metric: metric, Passive: true}
	if isis[0] != wantISIS {
		return fmt.Errorf("IS-IS interface = %+v, want %+v", isis[0], wantISIS)
	}
	var ospf []igpRuntimeOSPFInterface
	if err := igpRuntimeRollbackDispatch(ctx, plugin, "show ospf interface", &ospf); err != nil {
		return err
	}
	if len(ospf) != 1 {
		return fmt.Errorf("OSPF interfaces = %+v, want one passive lo", ospf)
	}
	wantOSPF := igpRuntimeOSPFInterface{Name: "lo", Cost: cost, Passive: true}
	if ospf[0] != wantOSPF {
		return fmt.Errorf("OSPF interface = %+v, want %+v", ospf[0], wantOSPF)
	}
	var instances []igpRuntimeOSPFInstance
	if err := igpRuntimeRollbackDispatch(ctx, plugin, "show ospf instance", &instances); err != nil {
		return err
	}
	if len(instances) != 2 {
		return fmt.Errorf("OSPF instances = %+v, want only 0 and %d", instances, child)
	}
	baseFound, childFound := false, false
	for _, instance := range instances {
		if instance.Interfaces != 1 {
			return fmt.Errorf("OSPF instance = %+v, want one interface", instance)
		}
		switch instance.ID {
		case 0:
			baseFound = true
		case child:
			childFound = true
		default:
			return fmt.Errorf("unexpected OSPF instance %d, want 0 or %d", instance.ID, child)
		}
	}
	if !baseFound {
		return errors.New("OSPF base instance missing")
	}
	if !childFound {
		return fmt.Errorf("OSPF instance %d missing", child)
	}
	return nil
}

func igpRuntimeRollbackDispatch(ctx context.Context, plugin *sdk.Plugin, command string, value any) error {
	status, err := Dispatch(ctx, plugin, command, value)
	if err != nil {
		return err
	}
	if status != statusDone {
		return fmt.Errorf("%s status = %q, want done", command, status)
	}
	return nil
}
