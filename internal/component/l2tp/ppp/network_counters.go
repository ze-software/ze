// Design: docs/architecture/l2tp/subscriber-session-model.md -- original network accounting snapshots.
package ppp

import (
	"context"
	"errors"
	"strconv"
	"sync"

	l2tpevents "github.com/ze-software/ze/internal/component/l2tp/events"
	"github.com/ze-software/ze/internal/core/slogutil"
	"github.com/ze-software/ze/internal/core/taskguard"
)

// Native providers are a synchronous task boundary, like subscriber handlers.
// A provider defect must retain its cause and permit the original Close to run.
type networkCounterPanicTasks struct {
	guard    *taskguard.Guard
	provider taskguard.Task
}

var networkCounterPanics = sync.OnceValue(func() *networkCounterPanicTasks {
	guard := taskguard.New("ppp", slogutil.Logger("ppp"))
	return &networkCounterPanicTasks{
		guard: guard,
		provider: guard.Task("counter-provider",
			"the original PPP owner is closed; the subscriber reconnects and other sessions remain usable"),
	}
})

// networkCounterLifetime retains the exact handle, never a replaceable name.
// nativeReleaseMu serializes baseline, activation, disconnect, and final read.
// Readonly receipt observers wait on terminalDone, never native Close.
type networkCounterLifetime struct {
	owner          SessionHandle
	lifetime       l2tpevents.NetworkLifetime
	baseline       l2tpevents.TrafficSample
	description    taskguard.Context
	terminalOnce   sync.Once
	terminalDone   chan struct{}
	terminal       l2tpevents.TrafficSample
	disconnectErr  error
	providerDefect *taskguard.Panic
}

func (network *networkCounterLifetime) finish() l2tpevents.TrafficSample {
	network.terminalOnce.Do(func() {
		network.disconnectErr = disconnectCounterOwner(network.owner, network.description)
		if network.disconnectErr != nil {
			network.terminal = l2tpevents.UnavailableTrafficCounters(network.disconnectErr)
		} else {
			network.terminal, network.providerDefect = readNetworkCounters(network.owner, network.description)
		}
		// Both provider calls contain a panic as an explicit unavailable result.
		// Publish that result before Close, including when the provider is defective.
		close(network.terminalDone)
	})
	return network.terminal
}

func disconnectCounterOwner(owner SessionHandle, description taskguard.Context) (cause error) {
	description.MessageType = "Disconnect before terminal counters"
	defer networkCounterPanics().provider.Contain(func() taskguard.Context { return description },
		func(failure *taskguard.Panic) { cause = failure })
	return owner.Disconnect()
}

// A provider defect is local control information as well as a copied sample.
// Baseline preparation MUST end its original session after a panic; terminal
// callers MUST retain the unavailable sample and continue physical cleanup.
func readNetworkCounters(owner SessionHandle, description taskguard.Context) (sample l2tpevents.TrafficSample, defect *taskguard.Panic) {
	description.MessageType = "original PPP counter read"
	defer networkCounterPanics().provider.Contain(func() taskguard.Context { return description },
		func(failure *taskguard.Panic) {
			sample = l2tpevents.UnavailableTrafficCounters(failure)
			defect = failure
		})
	counters, err := owner.Counters()
	if err != nil {
		return l2tpevents.UnavailableTrafficCounters(err), nil
	}
	return l2tpevents.MeasuredTrafficCounters(l2tpevents.TrafficTotals{
		RxBytes: counters.RxBytes, TxBytes: counters.TxBytes,
		RxPackets: counters.RxPackets, TxPackets: counters.TxPackets,
	}), nil
}

// Called before assignment publication, with no Driver or worker metadata lock.
// The release gate protects the original handle while its baseline is in flight;
// a canceled close waits for this read and cannot expose a half-built record.
func (session *pppSession) prepareNetworkCounters() (l2tpevents.NetworkLifetime, l2tpevents.TrafficSample, error) {
	session.nativeReleaseMu.Lock()
	defer session.nativeReleaseMu.Unlock()
	session.mu.Lock()
	closed := session.filesClosed
	session.mu.Unlock()
	select {
	case <-session.stopCh:
		return l2tpevents.NetworkLifetime{}, l2tpevents.TrafficSample{}, ErrSessionNotFound
	case <-session.sessStop:
		return l2tpevents.NetworkLifetime{}, l2tpevents.TrafficSample{}, ErrSessionNotFound
	default:
	}
	if closed {
		return l2tpevents.NetworkLifetime{}, l2tpevents.TrafficSample{}, ErrSessionNotFound
	}
	session.networkCounterMu.Lock()
	if network := session.networkCurrent; network != nil {
		lifetime, baseline := network.lifetime, network.baseline
		session.networkCounterMu.Unlock()
		return lifetime, baseline, nil
	}
	lifetime, err := session.networkSequence.Next()
	session.networkCounterMu.Unlock()
	if err != nil {
		return l2tpevents.NetworkLifetime{}, l2tpevents.TrafficSample{}, err
	}
	description := taskguard.Context{Unit: "PPP tunnel " + strconv.Itoa(int(session.tunnelID)) +
		" session " + strconv.Itoa(int(session.sessionID))}
	network := &networkCounterLifetime{
		owner: session.dataplane, lifetime: lifetime, description: description,
		terminalDone: make(chan struct{}),
	}
	network.baseline, network.providerDefect = readNetworkCounters(network.owner, network.description)
	if network.providerDefect != nil {
		return l2tpevents.NetworkLifetime{}, l2tpevents.TrafficSample{}, network.providerDefect
	}
	session.networkCounterMu.Lock()
	session.networkSequence = lifetime
	session.networkCurrent = network
	session.networkCounterMu.Unlock()
	return lifetime, network.baseline, nil
}

// Caller holds nativeReleaseMu, never a Driver or worker metadata lock.
// Service writers MUST be stopped first. Moving the original record ends its
// epoch before a replacement can be created; retained receipts stay readable.
func (session *pppSession) captureNetworkTerminal() (l2tpevents.NetworkLifetime, l2tpevents.TrafficSample) {
	session.networkCounterMu.Lock()
	network := session.networkCurrent
	if network != nil {
		session.networkCurrent = nil
		session.networkPrevious = network
	} else {
		network = session.networkPrevious
	}
	session.networkCounterMu.Unlock()
	if network == nil {
		return l2tpevents.NetworkLifetime{}, l2tpevents.TrafficSample{}
	}
	return network.lifetime, network.finish()
}

// Caller holds nativeReleaseMu, never the worker metadata mutex.
func (session *pppSession) stopIPv6Writers() error {
	session.mu.Lock()
	service := session.ipv6Svc
	session.mu.Unlock()
	return service.stopWriters()
}

// resetNetwork ends writers and forwarding before the final measurement.
// It permits no native I/O under metadata locks and calls Disconnect once per
// measured epoch, even when Down publication and final Close follow reset.
func (session *pppSession) resetNetwork() error {
	session.nativeReleaseMu.Lock()
	defer session.nativeReleaseMu.Unlock()
	session.mu.Lock()
	closed, connected := session.filesClosed, session.channelConnected
	session.mu.Unlock()
	if closed {
		return ErrSessionNotFound
	}
	if err := session.stopIPv6Writers(); err != nil {
		return err
	}
	session.networkCounterMu.Lock()
	network := session.networkCurrent
	session.networkCounterMu.Unlock()
	if network != nil {
		session.captureNetworkTerminal()
		if network.disconnectErr != nil {
			return network.disconnectErr
		}
		if network.providerDefect != nil {
			return network.providerDefect
		}
	} else if connected {
		// An unmeasured/legacy network still owns its connected channel. There is
		// no receipt to finish, so perform the same single disconnect directly.
		if err := disconnectCounterOwner(session.dataplane, taskguard.Context{Unit: "unmeasured PPP network"}); err != nil {
			return err
		}
	}
	session.mu.Lock()
	if connected {
		session.channelConnected = false
		session.unitDrainOwed = session.unitConnected != nil
	}
	session.mu.Unlock()
	return session.teardownNCPResources()
}

// NetworkCounterReceipt is a readonly capability for one original network's
// copied terminal sample. It owns no cleanup and grants no native operation.
// The accounting task MUST retain it until its terminal record completes.
// Safe to copy and observe concurrently; the zero receipt is invalid.
type NetworkCounterReceipt struct{ network *networkCounterLifetime }

func (receipt NetworkCounterReceipt) IsValid() bool { return receipt.network != nil }

// CaptureNetworkCounters captures the actual existing receipt, never a fresh
// lookup at session-down. The transport MUST pair its epoch with the complete
// Birth carried at the same assignment. No native I/O or Close wait occurs.
func (control SessionControl) CaptureNetworkCounters(lifetime l2tpevents.NetworkLifetime) (NetworkCounterReceipt, error) {
	if !control.capturedWorker() {
		return NetworkCounterReceipt{}, ErrSessionNotFound
	}
	if !lifetime.IsValid() {
		return NetworkCounterReceipt{}, errors.New("ppp: counter receipt requires an original network lifetime")
	}
	session := control.worker
	session.networkCounterMu.Lock()
	defer session.networkCounterMu.Unlock()
	network := session.networkCurrent
	if network != nil {
		if network.lifetime != lifetime {
			network = session.networkPrevious
		}
	} else {
		network = session.networkPrevious
	}
	if network == nil {
		return NetworkCounterReceipt{}, ErrSessionNotFound
	}
	if network.lifetime != lifetime {
		return NetworkCounterReceipt{}, ErrSessionNotFound
	}
	return NetworkCounterReceipt{network: network}, nil
}

// TerminalCounters waits only for the original measurement, whose producer
// closes terminalDone BEFORE it enters actual native Close. The caller MUST
// use a nonnil context and MUST NOT await it on a publication callback. A
// context error is pending/unresolved work, never a provider-unknown sample.
func (receipt NetworkCounterReceipt) TerminalCounters(ctx context.Context) (l2tpevents.TrafficSample, error) {
	if receipt.network == nil {
		return l2tpevents.TrafficSample{}, ErrSessionNotFound
	}
	if ctx == nil {
		return l2tpevents.TrafficSample{}, errors.New("ppp: terminal counter wait requires a context")
	}
	if err := ctx.Err(); err != nil {
		return l2tpevents.TrafficSample{}, err
	}
	select {
	case <-receipt.network.terminalDone:
		return receipt.network.terminal, nil
	case <-ctx.Done():
		return l2tpevents.TrafficSample{}, ctx.Err()
	}
}

// TerminalSnapshot is the publisher's nonblocking copied-value observation.
// A false result means not-yet-captured (or absent), never permanently Unknown;
// the already-bound accounting task retains this receipt and awaits it itself.
func (receipt NetworkCounterReceipt) TerminalSnapshot() (l2tpevents.TrafficSample, bool) {
	if receipt.network == nil {
		return l2tpevents.TrafficSample{}, false
	}
	select {
	case <-receipt.network.terminalDone:
		return receipt.network.terminal, true
	default:
		return l2tpevents.TrafficSample{}, false
	}
}
