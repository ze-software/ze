// Design: plan/release/fixes/spec-bgp-rfc8212-ebgp-default-reject.md -- AC13/15 fault preparations.
// Related: rfc8212_reload_fault.go -- faults cross the existing SDK transaction protocol.
package fixture

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"net"
	"os"
	"reflect"

	"github.com/ze-software/ze/internal/component/config"
	"github.com/ze-software/ze/internal/component/config/storage"
	"github.com/ze-software/ze/pkg/plugin/sdk"
)

func rfc8212ReloadRollback(ctx context.Context, p *sdk.Plugin, wire *rfc8212ReloadWire, tree *config.Tree, schema *config.Schema) error {
	if err := rfc8212ReloadSeed(ctx, p, wire, true); err != nil {
		return err
	}
	if err := rfc8212ReloadAppliedOnce(ctx, "10.72.1.0/24"); err != nil {
		return err
	}
	prefixes := []string{"10.72.1.0/24", "10.72.2.0/24", "10.72.3.0/24"}
	for index, mode := range []string{"apply", "commit", "outer"} {
		before, err := peerRows03(ctx, p, "*")
		if err != nil {
			return err
		}
		running, err := done03(ctx, p, "show config running")
		if err != nil {
			return err
		}
		active, err := rfc8212ReloadActive()
		if err != nil {
			return err
		}
		candidate := tree.Clone()
		if err := rfc8212ReloadPrepare(candidate, mode); err != nil {
			return err
		}
		var occupied net.Listener
		if mode == "outer" {
			server := candidate.GetContainerPath("environment/looking-glass").GetList("server")["main"]
			if server == nil {
				return errors.New("outer rejection fixture omitted looking-glass main")
			}
			server.Set("ip", "127.0.0.9")
			port, present := server.Get("port")
			if !present {
				return errors.New("outer rejection fixture omitted looking-glass port")
			}
			occupied, err = net.Listen("tcp", net.JoinHostPort("127.0.0.9", port))
			if err != nil {
				return fmt.Errorf("reserve real listener collision: %w", err)
			}
		}
		reloadErr := policyFinalReload(ctx, p, config.Serialize(candidate, schema), "failed")
		if occupied != nil {
			reloadErr = errors.Join(reloadErr, occupied.Close())
		}
		if reloadErr != nil {
			return fmt.Errorf("%s: %w", mode, reloadErr)
		}
		if !waitForFile(ctx, "fault.injected-"+mode, 80, liveStagesPollDelay) {
			return fmt.Errorf("%s never injected its fault after downstream settlement", mode)
		}
		if mode != "apply" {
			if !waitForFile(ctx, "fault.commit-"+mode, 80, liveStagesPollDelay) {
				return fmt.Errorf("%s never reached operation commit", mode)
			}
		}
		if err := rfc8212ReloadRunning(ctx, p, "accept"); err != nil {
			return err
		}
		current, err := done03(ctx, p, "show config running")
		if err != nil {
			return err
		}
		if !reflect.DeepEqual(running, current) {
			return fmt.Errorf("%s rollback changed running configuration", mode)
		}
		if err := rfc8212ReloadDurable(active); err != nil {
			return fmt.Errorf("%s: %w", mode, err)
		}
		prefix := []string{"10.73.2.0/24", "10.73.3.0/24", "10.73.4.0/24"}[index]
		prefixes = append(prefixes, prefix)
		if err := wire.wait(ctx, prefixes...); err != nil {
			return fmt.Errorf("%s rollback did not restore routes and resume buffered input: %w", mode, err)
		}
		wire.mu.Lock()
		received := wire.adds[prefix]
		wire.mu.Unlock()
		if received != 1 {
			return fmt.Errorf("%s buffered prefix announced %d times, want exactly once", mode, received)
		}
		if err := rfc8212ReloadAppliedOnce(ctx, prefix); err != nil {
			return err
		}
		if err := rfc8212ReloadInputDelta(ctx, p, before, 1); err != nil {
			return fmt.Errorf("%s buffered input: %w", mode, err)
		}
		if err := rfc8212ReloadSessions(ctx, p, before, 0); err != nil {
			return err
		}
		if err := rfc8212ReloadBest(ctx, p, "10.72.3.0/24", "127.0.0.1"); err != nil {
			return err
		}
		if err := wire.path(ctx, "10.72.3.0/24", false); err != nil {
			return err
		}
	}
	return nil
}

func rfc8212ReloadPrepare(tree *config.Tree, mode string) error {
	if err := os.WriteFile("rfc8212.fault", []byte(mode), 0o600); err != nil {
		return err
	}
	if err := rfc8212ReloadPolicy(tree, "reject"); err != nil {
		return err
	}
	format := tree.GetContainerPath("environment/cli/format")
	if format == nil {
		return errors.New("fault fixture omitted environment cli format")
	}
	format.Set("default", "json")
	return nil
}

// The unsupported case deliberately has no fabricated ownership declaration:
// today's SDK cannot name it. Its retained event state must make the missing
// registered-owner preflight visible, not be mistaken for a passing observer.
// The other modes reach existing callback transport failure boundaries; they
// become source-lifecycle proof only after Phase 3 binds the real owner contract.
func rfc8212ReloadOwner(ctx context.Context, p *sdk.Plugin, wire *rfc8212ReloadWire, tree *config.Tree, schema *config.Schema) error {
	if err := rfc8212ReloadSeed(ctx, p, wire, true); err != nil {
		return err
	}
	if err := rfc8212ReloadAppliedOnce(ctx, "10.72.1.0/24"); err != nil {
		return err
	}
	// A received frame, not the last send action, must end each native
	// connection script. Both peers then linger until this daemon closes them.
	for _, peer := range []string{"127.0.0.1", "127.0.0.3"} {
		if err := rfc8212ReloadTrigger(ctx, p, peer, "9"); err != nil {
			return err
		}
	}
	modeData, err := os.ReadFile("owner.mode")
	if err != nil {
		return err
	}
	mode := string(modeData)
	before, err := peerRows03(ctx, p, "*")
	if err != nil {
		return err
	}
	active, err := rfc8212ReloadActive()
	if err != nil {
		return err
	}
	candidate := tree.Clone()
	if err := rfc8212ReloadPrepare(candidate, mode); err != nil {
		return err
	}
	err = policyFinalReload(ctx, p, config.Serialize(candidate, schema), "failed")
	if mode == "deadline" {
		err = errors.Join(err, os.WriteFile("fault.release", []byte("reload completed"), 0o600))
	}
	if err != nil {
		return fmt.Errorf("owner %s boundary: %w", mode, err)
	}
	if mode == "unsupported" {
		if _, err := os.Stat("fault.reached-unsupported"); !errors.Is(err, os.ErrNotExist) {
			return fmt.Errorf("unsupported state owner reached mutation: marker error=%v", err)
		}
	} else if mode == "finalize" {
		if !waitForFile(ctx, "fault.finalize", 80, liveStagesPollDelay) {
			return errors.New("finalization failure callback never ran")
		}
		if err := rfc8212ReloadRunning(ctx, p, "reject"); err != nil {
			return err
		}
		if err := wire.wait(ctx, "10.72.2.0/24", "10.72.3.0/24"); err != nil {
			return err
		}
		accepted, err := rfc8212ReloadActive()
		if err != nil {
			return err
		}
		if bytes.Equal(active, accepted) {
			return errors.New("finalization failure pretended the accepted candidate was rolled back")
		}
		return rfc8212ReloadSessions(ctx, p, before, 0)
	} else if !waitForFile(ctx, "fault.injected-"+mode, 80, liveStagesPollDelay) {
		return fmt.Errorf("owner %s callback never reached", mode)
	}
	if err := rfc8212ReloadRunning(ctx, p, "accept"); err != nil {
		return err
	}
	if err := rfc8212ReloadDurable(active); err != nil {
		return err
	}
	if err := wire.wait(ctx, "10.72.1.0/24", "10.72.2.0/24", "10.72.3.0/24"); err != nil {
		return err
	}
	return rfc8212ReloadSessions(ctx, p, before, 0)
}

func rfc8212ReloadActive() ([]byte, error) {
	store, err := storage.OpenReadOnly(".")
	if err != nil {
		return nil, err
	}
	active, readErr := storage.ReadActiveConfig(store, fileBGPConf)
	return active, errors.Join(readErr, store.Close())
}

func rfc8212ReloadDurable(want []byte) error {
	store, err := storage.OpenReadOnly(".")
	if err != nil {
		return err
	}
	defer store.Close() //nolint:errcheck // Only reads are performed; read errors are returned.
	active, err := storage.ReadActiveConfig(store, fileBGPConf)
	if err != nil {
		return err
	}
	if !bytes.Equal(active, want) {
		return errors.New("failed reload changed durable active configuration")
	}
	_, _, candidate, err := storage.ReadCandidateConfig(store, fileBGPConf)
	if err != nil {
		return err
	}
	if candidate {
		return errors.New("failed reload retained candidate pointer")
	}
	return nil
}
