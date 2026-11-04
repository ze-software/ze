package server

import (
	"context"
	"fmt"
	"strconv"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/ze-software/ze/internal/component/plugin"
	"github.com/ze-software/ze/pkg/plugin/rpc"
)

// TestCapabilityTransactionAtomicOPENReplacement races the real transaction
// install against the same selector reader OPEN construction calls. Channel
// barriers establish reader liveness before writing; every observed snapshot
// must contain the entire old or new generation, including unchanged declarations.
// The deadline bounds the workers rather than choosing when the race occurs.
func TestCapabilityTransactionAtomicOPENReplacement(t *testing.T) {
	const owners = 64
	const replacements = 256
	const readers = 4
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	s := &Server{capInjector: plugin.NewCapabilityInjector()}
	declarations := [2]map[string]*plugin.PluginCapabilities{
		make(map[string]*plugin.PluginCapabilities, owners),
		make(map[string]*plugin.PluginCapabilities, owners),
	}
	for i := range owners {
		name := "owner-" + strconv.Itoa(i)
		for generation := range declarations {
			declarations[generation][name] = &plugin.PluginCapabilities{PluginName: name,
				Capabilities: []plugin.PluginCapability{{Code: uint8(i + 1),
					Encoding: rpc.CapEncodingText, Payload: string(rune('A' + generation)),
					Peers: []string{capabilityReloadPeer}}}}
		}
		if err := s.capInjector.AddPluginCapabilities(declarations[0][name]); err != nil {
			t.Fatal(err)
		}
	}
	if err := s.capInjector.AddPluginCapabilities(&plugin.PluginCapabilities{
		PluginName: "unchanged", Capabilities: []plugin.PluginCapability{{
			Code: 200, Encoding: rpc.CapEncodingText, Payload: "kept",
		}},
	}); err != nil {
		t.Fatal(err)
	}

	start := make(chan struct{})
	done := make(chan struct{})
	ready := make(chan struct{}, readers)
	failures := make(chan error, readers)
	var active atomic.Bool
	var concurrentReads atomic.Int64
	var workers sync.WaitGroup
	workers.Add(readers)
	for range readers {
		go func() {
			defer workers.Done()
			if err := checkCapabilityOPENGeneration(s, owners); err != nil {
				failures <- err
				ready <- struct{}{}
				return
			}
			ready <- struct{}{}
			select {
			case <-start:
			case <-ctx.Done():
				failures <- ctx.Err()
				return
			}
			// The writer closes done after its bounded replacement loop; ctx
			// also releases the worker when a defective writer stalls.
			for {
				select {
				case <-done:
					return
				case <-ctx.Done():
					failures <- ctx.Err()
					return
				default:
				}
				readingWhileActive := active.Load()
				if err := checkCapabilityOPENGeneration(s, owners); err != nil {
					failures <- err
					return
				}
				if readingWhileActive && active.Load() {
					concurrentReads.Add(1)
				}
			}
		}()
	}
	for range readers {
		select {
		case <-ready:
		case <-ctx.Done():
			t.Fatal("OPEN readers did not become ready:", ctx.Err())
		}
	}
	active.Store(true)
	close(start)
	var replaceErr error
	for i := range replacements {
		if ctx.Err() != nil {
			replaceErr = ctx.Err()
			break
		}
		bridge := configTxBridge{server: s, capabilities: capabilityStage{
			staged: declarations[(i/2)%2],
		}}
		if err := bridge.installStagedCapabilities(); err != nil {
			replaceErr = err
			break
		}
	}
	active.Store(false)
	close(done)
	workers.Wait()
	close(failures)
	for err := range failures {
		t.Error(err)
	}
	if replaceErr != nil {
		t.Fatal("replace candidate:", replaceErr)
	}
	if concurrentReads.Load() == 0 {
		t.Fatal("no OPEN reader sampled while replacements were active")
	}
	if err := checkCapabilityOPENGeneration(s, owners); err != nil {
		t.Fatal("final snapshot:", err)
	}
}

// checkCapabilityOPENGeneration checks useful complete output rather than
// accepting an empty result as a snapshot of a generation.
func checkCapabilityOPENGeneration(s *Server, owners int) error {
	got := s.GetPluginCapabilitiesForSelectors(capabilityReloadPeer)
	if len(got) != owners+1 {
		return fmt.Errorf("OPEN saw partial candidate: %d capabilities, want %d", len(got), owners+1)
	}
	var generation byte
	seen := make(map[uint8]bool, owners)
	for _, cap := range got {
		if cap.Code == 200 {
			if cap.Plugin != "unchanged" || string(cap.Value) != "kept" {
				return fmt.Errorf("unchanged owner lost: %+v", cap)
			}
			continue
		}
		if cap.Code == 0 || int(cap.Code) > owners || seen[cap.Code] {
			return fmt.Errorf("OPEN saw invalid or duplicate code: %+v", cap)
		}
		seen[cap.Code] = true
		if cap.Plugin != "owner-"+strconv.Itoa(int(cap.Code)-1) || len(cap.Value) != 1 {
			return fmt.Errorf("OPEN saw changed capability ownership or value: %+v", cap)
		}
		if cap.Value[0] != 'A' && cap.Value[0] != 'B' {
			return fmt.Errorf("OPEN saw invalid generation: %+v", cap)
		}
		if generation == 0 {
			generation = cap.Value[0]
		}
		if cap.Value[0] != generation {
			return fmt.Errorf("OPEN saw mixed candidate generations: %+v", got)
		}
	}
	if len(seen) != owners {
		return fmt.Errorf("OPEN lost candidate owners: %d, want %d", len(seen), owners)
	}
	return nil
}
