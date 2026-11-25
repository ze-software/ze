// Design: docs/architecture/exabgp-bridge.md -- script runtime for an ExaBGP bus endpoint.
// Detail: script.go -- one script's fork, stdout batch reader, stdin writer, and single-run exit.
// Related: ../main_sdk.go -- the external wrapper that owns a Fleet.
//
// Safe for concurrent use.

package bridgerun

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/netip"
	"slices"
	"sync"
	"sync/atomic"
	"time"

	bgpevents "github.com/ze-software/ze/internal/core/bgp/events"
	"github.com/ze-software/ze/internal/core/events"
	"github.com/ze-software/ze/internal/exabgp/bridge"
	"github.com/ze-software/ze/pkg/plugin/rpc"
)

const (
	busCmdSendBGP    = events.NamespaceCLI + ":send bgp"
	busMsgNegotiated = bgpevents.Namespace + ":" + bgpevents.EventNegotiated
)

// Dispatcher is the engine surface a Fleet needs: one command in, one answer
// out. *sdk.Plugin implements it, and a test implements it with a recorder.
type Dispatcher interface {
	DispatchCommand(ctx context.Context, command string) (string, json.RawMessage, error)
}

// Script is one operator script the ExaBGP wrapper runs.
//
// Argv is the command already split into its executable and its arguments.
// Encoder is the script-side event encoding (`bridge.EncoderText` or
// `bridge.EncoderJSON`); every construction site states it explicitly because
// the zero value names no format. Forms holds any `--form` rendering modifiers.
type Script struct {
	Name    string
	Argv    []string
	Encoder bridge.Encoder
	Forms   FormSpecs
}

// Fleet runs the ExaBGP script(s) owned by one wrapper process.
//
// The sequence is New, optional SetOnExit/ApplyFilter, Start, Broadcast for
// each event, then Shutdown when the daemon stops, then Stop. Shutdown MUST
// come before the caller cancels the context it passed to Start, and the caller
// MUST cancel that context before calling Stop.
type Fleet struct {
	log     *slog.Logger
	scripts []*script
	wg      sync.WaitGroup

	// stopping tells each script runner that EOF is a daemon-initiated shutdown
	// rather than an unexpected script exit.
	stopping bool
	onExit   func(exitCode int)
	capPeers []string
	mu       sync.Mutex

	// apiSetting is `exabgp.api.version` as a bridge.APIVersion: what each
	// script starts held to. Start reads it from the env, and a script's
	// `api version <v>` line moves it for this Fleet alone (E-5).
	apiSetting atomic.Uint32

	// sessions holds each peer's negotiated block for its UPDATE lines.
	sessions bridge.Sessions
}

// New builds a Fleet for the given scripts. It forks nothing: Start does.
func New(log *slog.Logger, families []string, scripts []Script) *Fleet {
	f := &Fleet{log: log}
	for _, s := range scripts {
		translator := bridge.Translator{Families: families}
		f.scripts = append(f.scripts, newScript(log, translator, &f.apiSetting, s))
	}
	return f
}

// SetOnExit registers a callback invoked when a script exits on its own (while
// the fleet is not stopping and ctx is not canceled), passing the script's
// process exit status so the wrapper exits with the same code (E-6, AC-45).
func (f *Fleet) SetOnExit(fn func(exitCode int)) {
	f.mu.Lock()
	f.onExit = fn
	f.mu.Unlock()
}

// notifyScriptExit invokes the registered onExit callback, if any.
func (f *Fleet) notifyScriptExit(exitCode int) {
	f.mu.Lock()
	fn := f.onExit
	f.mu.Unlock()
	if fn != nil {
		fn(exitCode)
	}
}

// ApplyFilter updates every script's send peer scope (`Translator.Peers` and
// `Translator.NoPeers`) and `bgp:negotiated` grant set from the participant's
// compiled bus filter. Safe for concurrent calls with Broadcast and script
// command translation.
func (f *Fleet) ApplyFilter(filter *rpc.BusParticipantFilter) {
	peers, noPeers := deriveSendScope(filter)
	negScope, negPeers := deriveNegotiatedGrant(filter)

	f.mu.Lock()
	f.capPeers = slices.Clone(peers)
	f.mu.Unlock()

	for _, s := range f.scripts {
		s.setFilter(peers, noPeers, negScope, negPeers)
	}
}

// CapabilityPeers returns a copy of the peer IP addresses scoped by `send > to`
// for Stage 3 capability declarations, or nil when send scope is unscoped
// (`SendToAll`) or empty.
func (f *Fleet) CapabilityPeers() []string {
	f.mu.Lock()
	defer f.mu.Unlock()
	if len(f.capPeers) == 0 {
		return nil
	}
	return slices.Clone(f.capPeers)
}

// TranslatorScope returns the first script's current `(Peers, NoPeers)` send
// scope, for inspection and tests.
func (f *Fleet) TranslatorScope() ([]string, bool) {
	if len(f.scripts) == 0 {
		return nil, false
	}
	tr := f.scripts[0].snapshotTranslator()
	return slices.Clone(tr.Peers), tr.NoPeers
}

// GrantsNegotiated reports whether the first script currently attaches the
// peer's negotiated block for the named peer address.
func (f *Fleet) GrantsNegotiated(peer string) bool {
	if len(f.scripts) == 0 {
		return false
	}
	return f.scripts[0].grantsNegotiated(peer)
}

// Count answers how many scripts this Fleet holds.
func (f *Fleet) Count() int { return len(f.scripts) }

// Start forks every script and starts the goroutines that serve it. It answers
// the first fork error and names the script that produced it, and logs that
// error at ERROR with the run command and the fix (script.logStartFailure).
//
// A script that started before the failing one keeps running, and Stop ends it.
func (f *Fleet) Start(ctx context.Context, d Dispatcher) error {
	setting, err := bridge.APIVersionFromEnv()
	if err != nil {
		return fmt.Errorf("exabgp-bridge: %w", err)
	}
	f.apiSetting.Store(uint32(setting))
	for _, s := range f.scripts {
		if s.api.Encoder() == bridge.EncoderUnspecified {
			return fmt.Errorf("exabgp-bridge: script %s: no encoder was set for it", s.name)
		}
		if err := s.start(ctx); err != nil {
			s.logStartFailure(err)
			return fmt.Errorf("exabgp-bridge: script %s: %w", s.name, err)
		}
		drained := make(chan struct{})
		reaped := make(chan struct{})
		s.mu.Lock()
		s.drained = drained
		s.reaped = reaped
		s.mu.Unlock()

		f.wg.Add(2)
		go func() {
			defer f.wg.Done()
			s.runOnce(ctx, f, d, reaped)
		}()
		go func() {
			defer f.wg.Done()
			s.writeLoop(drained)
		}()
	}
	return nil
}

// Broadcast translates one ze event delivered by the bus and queues it for each
// script in this Fleet. The bus already filters which peer, message, and
// direction reach this endpoint (Q-E4), so Broadcast translates every BGP
// message it receives, applying each script's `bgp:negotiated` grant and
// `--form` settings.
func (f *Fleet) Broadcast(event string) {
	var zebgp map[string]any
	if err := json.Unmarshal([]byte(event), &zebgp); err != nil {
		f.log.Warn("exabgp-bridge: invalid JSON event", "error", err)
		return
	}
	read := bridge.ReadEvent(zebgp)
	if read.Kind == "" {
		f.log.Debug("exabgp-bridge: event carries no BGP message", "event", event)
		return
	}
	f.sessions.Note(&read)

	// rendered[0] holds lines with the session negotiated block attached;
	// rendered[1] holds lines without it (Event.ForGrant).
	var rendered [2][bridge.FormsCount][bridge.FormatCount][]byte
	variants := [2]bridge.Event{read.ForGrant(true), read.ForGrant(false)}
	for _, s := range f.scripts {
		forms := s.forms.For(read.Peer, read.Direction)
		grant := 0
		if !s.grantsNegotiated(read.Peer) {
			grant = 1
		}
		variant, cache := variants[grant], &rendered[grant][forms]
		line, ok := f.render(cache, s.api.Format(), forms, variant)
		if !ok {
			continue
		}
		s.send(line)
	}
}

// render answers the event in one format, rendering it on the first script that
// asks for it and reusing the answer for any sibling script in the same format.
func (f *Fleet) render(cache *[bridge.FormatCount][]byte, format bridge.Format, forms bridge.Forms, event bridge.Event) ([]byte, bool) {
	slot := format.Slot()
	if cache[slot] != nil {
		return cache[slot], len(cache[slot]) > 0
	}

	line := event.Lines(format, forms, f.log)
	if line == nil {
		line = []byte{}
	}
	cache[slot] = line
	return line, len(line) > 0
}

// Shutdown writes every script ExaBGP's shutdown line (bridge.ShutdownLine),
// closes each started script's queue, and waits up to shutdownWait for each
// child to read the line and exit on its stdin EOF.
//
// Shutdown MUST be called before Stop, and before the caller cancels the
// context passed to Start.
func (f *Fleet) Shutdown() {
	f.mu.Lock()
	f.stopping = true
	f.mu.Unlock()

	var started []*script
	for _, s := range f.scripts {
		if drained, _ := s.ended(); drained == nil {
			continue
		}
		line, err := bridge.ShutdownLine(s.api.Format())
		if err != nil {
			f.log.Warn("exabgp-bridge: the shutdown line could not be written", "script", s.name, "error", err)
		} else {
			s.send(line)
		}
		s.closeQueue()
		started = append(started, s)
	}

	bound := time.NewTimer(shutdownWait)
	defer bound.Stop()

	for i, s := range started {
		drained, reaped := s.ended()
		if !untilEnded(bound.C, drained) {
			f.lateAtShutdown(started[i:])
			return
		}
		if !untilEnded(bound.C, reaped) {
			f.lateAtShutdown(started[i:])
			return
		}
	}
}

// shutdownWait bounds how long Fleet.Shutdown waits, for the whole fleet, for
// the scripts to receive the shutdown line and exit on their EOF.
const shutdownWait = time.Second

// untilEnded answers true once ended is closed, and false if the bound fires
// first.
func untilEnded(bound <-chan time.Time, ended <-chan struct{}) bool {
	select {
	case <-ended:
		return true
	default:
	}
	select {
	case <-ended:
		return true
	case <-bound:
		return false
	}
}

// lateAtShutdown logs each script Shutdown stopped waiting for.
func (f *Fleet) lateAtShutdown(late []*script) {
	for _, s := range late {
		f.log.Warn("exabgp-bridge: script still running when the shutdown wait ended; it is killed and may not have read the shutdown line",
			"script", s.name, "bound", shutdownWait)
	}
}

// Stop closes every script's stdin queue and waits for every goroutine this
// Fleet started. It MUST be called after the context passed to Start is
// canceled, and MAY be called when Start failed.
func (f *Fleet) Stop() {
	f.mu.Lock()
	f.stopping = true
	f.mu.Unlock()

	for _, s := range f.scripts {
		s.closeQueue()
	}
	f.wg.Wait()

	for _, s := range f.scripts {
		s.reportDrops()
	}
}

// stopped answers whether Shutdown or Stop has been called.
func (f *Fleet) stopped() bool {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.stopping
}

// deriveSendScope extracts the sorted peer IP scope and NoPeers flag from a
// participant's compiled bus filter.
func deriveSendScope(filter *rpc.BusParticipantFilter) (peers []string, noPeers bool) {
	if filter == nil {
		return nil, false
	}
	if filter.SendNone {
		return nil, true
	}
	if filter.SendToNone {
		return nil, true
	}

	// Prefer the explicit `cli:send bgp` rule when present, since route
	// announcements and withdrawals dispatch under that prefix (Q-T5).
	// When `cli:send bgp` is disallowed by `send > allow` (Allowed == false)
	// while target peers remain in scope, preserve those peer addresses so the
	// translator still dispatches `send bgp ...` to the daemon, where the bus
	// send gate refuses it and increments `ze_bus_send_refused_total`.
	for _, rule := range filter.SendRules {
		if rule.Message != busCmdSendBGP {
			continue
		}
		if rule.TargetNone {
			return nil, true
		}
		if rule.TargetAll {
			return nil, false
		}
		addrs := collectTargetAddresses(rule.Targets)
		if len(addrs) > 0 {
			return addrs, false
		}
		if rule.Allowed {
			return nil, true
		}
		// Disallowed rule with empty Targets: fall through to filter.SendToAll
		// and remaining send rules' targets so daemon-side send gating still runs.
		break
	}

	if filter.SendToAll {
		return nil, false
	}

	seen := make(map[string]struct{})
	for _, rule := range filter.SendRules {
		if !rule.Allowed {
			continue
		}
		if rule.TargetNone {
			continue
		}
		if rule.TargetAll {
			return nil, false
		}
		for _, target := range rule.Targets {
			if target.Address != "" {
				seen[target.Address] = struct{}{}
			}
		}
	}
	if len(seen) == 0 {
		return nil, true
	}
	addrs := make([]string, 0, len(seen))
	for addr := range seen {
		addrs = append(addrs, addr)
	}
	slices.Sort(addrs)
	return addrs, false
}

// collectTargetAddresses extracts sorted unique non-empty IP addresses from a
// rule's resolved bus targets.
func collectTargetAddresses(targets []rpc.BusFilterTarget) []string {
	seen := make(map[string]struct{}, len(targets))
	for _, target := range targets {
		if target.Address != "" {
			seen[target.Address] = struct{}{}
		}
	}
	if len(seen) == 0 {
		return nil
	}
	addrs := make([]string, 0, len(seen))
	for addr := range seen {
		addrs = append(addrs, addr)
	}
	slices.Sort(addrs)
	return addrs
}

// deriveNegotiatedGrant determines which peers grant `bgp:negotiated` to this
// endpoint, controlling Event.ForGrant during Broadcast.
func deriveNegotiatedGrant(filter *rpc.BusParticipantFilter) (negotiatedScope, map[netip.Addr]bool) {
	if filter == nil {
		return negotiatedAll, nil
	}
	if filter.ReceiveNone {
		return negotiatedNone, nil
	}
	if filter.ReceiveFromNone {
		return negotiatedNone, nil
	}
	for _, rule := range filter.ReceiveRules {
		if rule.Message != busMsgNegotiated {
			continue
		}
		if !rule.Allowed {
			return negotiatedNone, nil
		}
		if rule.TargetNone {
			return negotiatedNone, nil
		}
		if rule.TargetAll {
			return negotiatedAll, nil
		}
		peers := make(map[netip.Addr]bool, len(rule.Targets))
		for _, target := range rule.Targets {
			if target.Address == "" {
				continue
			}
			if addr, err := netip.ParseAddr(target.Address); err == nil {
				peers[addr] = true
			}
		}
		if len(peers) == 0 {
			return negotiatedNone, nil
		}
		return negotiatedPeers, peers
	}
	if filter.ReceiveAll && filter.ReceiveFromAll {
		return negotiatedAll, nil
	}
	return negotiatedNone, nil
}
