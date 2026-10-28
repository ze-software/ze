// Design: docs/architecture/config/transaction-protocol.md -- real external callback fault boundaries.
// Related: rfc8212_reload_rollback.go -- observations distinguish these preparations from source-owner support.
package fixture

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/ze-software/ze/pkg/plugin/rpc"
	"github.com/ze-software/ze/pkg/plugin/sdk"
)

const rfc8212FaultOperation rpc.ConfigOperationType = "rfc8212-record-candidate"

// rfc8212ReloadFault retains actual delivered source events and an operation's
// candidate. This is a transaction participant, NOT an invented invalidation
// owner. Phase 3 must add the real ownership declaration and lifecycle callbacks.
// Safe for concurrent callback/event delivery; its maps are initialized by driver.
type rfc8212ReloadFault struct {
	mu        sync.Mutex
	routes    map[string]string
	received  map[string]int
	candidate string
	previous  string
	mode      string
	died      bool
	fired     map[string]bool
}

func rfc8212ReloadFaultDriver(ctx context.Context, args []string) error {
	if len(args) != 0 {
		return errors.New("rfc8212 fault participant takes no arguments")
	}
	p, err := newObserver("rfc8212-owner")
	if err != nil {
		return err
	}
	defer p.Close() //nolint:errcheck // The Run result carries transport errors.
	state := &rfc8212ReloadFault{
		fired: make(map[string]bool), routes: make(map[string]string), received: make(map[string]int),
	}
	if err := os.WriteFile("owner.received", []byte("{}"), 0o600); err != nil {
		return err
	}
	wire := newRFC8212ReloadWire()
	p.OnEvent(func(event string) error {
		if err := state.record(event); err != nil {
			return err
		}
		return wire.receive(event)
	})
	p.SetStartupSubscriptions([]string{"update", "state"}, []string{"*"}, "full")
	p.OnConfigApply(func([]sdk.ConfigDiffSection) error {
		return errors.New("operation participant unexpectedly received coarse apply")
	})
	p.OnConfigOperationDecompose(func(input sdk.ConfigOperationDecomposeInput) (*sdk.ConfigOperationDecomposeOutput, error) {
		mode, err := os.ReadFile("rfc8212.fault")
		if err != nil {
			return nil, err
		}
		state.mu.Lock()
		state.mode = strings.TrimSpace(string(mode))
		state.mu.Unlock()
		return &sdk.ConfigOperationDecomposeOutput{Status: rpc.StatusOK, Operations: []rpc.ConfigOperation{{
			ID: "rfc8212-after-source", Root: "environment", Owner: "rfc8212-owner",
			Type: rfc8212FaultOperation, Verb: rpc.VerbModify,
			Target:   rpc.ResourceRef{Kind: "rfc8212-fixture-record", Name: "candidate"},
			Consumes: []rpc.ResourceRef{{Kind: rpc.ResourcePeer, Peer: "peer-1"}},
			Params:   rpc.ConfigOperationParams{Value: input.Candidate.Data, OldValue: input.Active.Data},
		}}}, nil
	})
	p.OnConfigOperationVerify(func(sdk.ConfigOperationVerifyInput) error { return nil })
	p.OnConfigOperationApply(func(input sdk.ConfigOperationApplyInput) (*sdk.ConfigOperationApplyOutput, error) {
		state.mu.Lock()
		mode := state.mode
		alreadyApplied := state.fired[mode]
		state.fired[mode] = true
		state.mu.Unlock()
		// An outer rejection re-applies the previous configuration through a
		// fresh real transaction. The fixture must not inject the same fault
		// into that compensation; its data still follows the actual candidate.
		if alreadyApplied {
			state.mu.Lock()
			state.previous = input.Operation.Params.OldValue
			state.candidate = input.Operation.Params.Value
			state.mu.Unlock()
			return &sdk.ConfigOperationApplyOutput{Status: rpc.StatusOK}, nil
		}
		// The graph consumes the actual BGP modify's produced peer. Verify the
		// runtime boundary too: a reordered test must fail, not test preflight.
		if err := rfc8212ReloadRunning(ctx, p, "reject"); err != nil {
			return nil, fmt.Errorf("fault ran before import apply: %w", err)
		}
		if err := os.WriteFile("fault.reached-"+mode, []byte(input.TransactionID), 0o600); err != nil {
			return nil, err
		}
		if mode != "unsupported" {
			if err := wire.wait(ctx, "10.72.2.0/24", "10.72.3.0/24"); err != nil {
				return nil, fmt.Errorf("source apply did not settle downstream before %s fault: %w", mode, err)
			}
		}
		if mode == "apply" || mode == "commit" || mode == "outer" {
			epoch := map[string]string{"apply": "2", "commit": "3", "outer": "4"}[mode]
			if err := rfc8212ReloadTrigger(ctx, p, "127.0.0.1", epoch); err != nil {
				return nil, err
			}
			if !waitForFile(ctx, "source.buffered-"+epoch, 80, liveStagesPollDelay) {
				return nil, errors.New("source never wrote buffered input on its original socket")
			}
			state.mu.Lock()
			delivered := state.received["10.73."+epoch+".0/24"]
			state.mu.Unlock()
			if delivered != 0 {
				return nil, fmt.Errorf("buffered input crossed admission before %s rollback: %d deliveries", mode, delivered)
			}
		}
		if err := os.WriteFile("fault.injected-"+mode, []byte(input.TransactionID), 0o600); err != nil {
			return nil, err
		}
		switch mode {
		case "apply", "ack":
			return nil, fmt.Errorf("rfc8212 deliberate %s refusal after source apply", mode)
		case "death":
			state.mu.Lock()
			state.died = true
			state.mu.Unlock()
			return nil, p.Close()
		case "deadline":
			// The reload must finish before this callback is released. This is
			// a real missing RPC answer, not a fabricated timeout return.
			if !waitForFile(ctx, "fault.release", 175, 200*time.Millisecond) {
				return nil, errors.New("deadline fault was not released after reload completion")
			}
			return nil, errors.New("late callback after deadline")
		case "commit", "outer", "finalize", "unsupported":
			state.mu.Lock()
			state.previous = input.Operation.Params.OldValue
			state.candidate = input.Operation.Params.Value
			state.mu.Unlock()
			return &sdk.ConfigOperationApplyOutput{Status: rpc.StatusOK}, nil
		default:
			return nil, fmt.Errorf("unknown RFC8212 fault mode %q", mode)
		}
	})
	p.OnConfigOperationRollback(func(input sdk.ConfigOperationRollbackInput) error {
		state.mu.Lock()
		state.candidate = state.previous
		mode := state.mode
		state.mu.Unlock()
		return os.WriteFile("fault.rollback-"+mode, []byte(input.TransactionID), 0o600)
	})
	p.OnConfigOperationCommit(func(input sdk.ConfigOperationCommitInput) error {
		state.mu.Lock()
		mode := state.mode
		refused := state.fired["commit-receipt"]
		if mode == "commit" {
			state.fired["commit-receipt"] = true
		}
		state.mu.Unlock()
		if err := os.WriteFile("fault.commit-"+mode, []byte(input.TransactionID), 0o600); err != nil {
			return err
		}
		if mode == "commit" {
			if !refused {
				return errors.New("rfc8212 deliberate later operation commit refusal")
			}
		}
		return nil
	})
	err = p.Run(ctx, &sdk.Registration{
		WantsConfig: []string{"environment"}, ApplyBudget: 5,
		ConfigOperations: []rpc.ConfigOperationDecl{{Root: "environment", Decompose: true, Operations: []rpc.ConfigOperationType{rfc8212FaultOperation}}},
	})
	state.mu.Lock()
	died := state.died
	state.mu.Unlock()
	if died {
		return nil
	}
	return err
}

func rfc8212ReloadFinalizer(ctx context.Context, args []string) error {
	if len(args) != 0 {
		return errors.New("rfc8212 finalizer takes no arguments")
	}
	p, err := newObserver("rfc8212-finalizer")
	if err != nil {
		return err
	}
	defer p.Close() //nolint:errcheck // The Run result carries transport errors.
	p.OnConfigApply(func([]sdk.ConfigDiffSection) error { return nil })
	p.OnConfigRollback(func(tx string) error { return os.WriteFile("finalizer.rollback", []byte(tx), 0o600) })
	p.OnConfigCommitted(func(tx string) error {
		mode, err := os.ReadFile("rfc8212.fault")
		if err != nil {
			return err
		}
		if strings.TrimSpace(string(mode)) != "finalize" {
			return nil
		}
		if err := os.WriteFile("fault.finalize", []byte(tx), 0o600); err != nil {
			return err
		}
		return errors.New("rfc8212 deliberate finalization receipt failure")
	})
	return p.Run(ctx, &sdk.Registration{WantsConfig: []string{"bgp"}})
}

func rfc8212ReloadRefusalGuard(ctx context.Context, args []string) error {
	if len(args) != 0 {
		return errors.New("rfc8212 refusal guard takes no arguments")
	}
	p, err := newObserver("rfc8212-refusal-guard")
	if err != nil {
		return err
	}
	defer p.Close() //nolint:errcheck // The Run result carries transport errors.
	p.OnConfigApply(func([]sdk.ConfigDiffSection) error {
		return os.WriteFile("refusal.applied", []byte("apply reached"), 0o600)
	})
	return p.Run(ctx, &sdk.Registration{WantsConfig: []string{"bgp"}})
}
