// Design: docs/architecture/exabgp-bridge.md -- the scripts the bridge runs
// Detail: script.go -- one script's fork, stdout reader and respawn loop
// Related: ../bridgeplugin/internal.go, ../main_sdk.go -- the two runners that own a Fleet
//
// An ExaBGP config declares any number of `process <name> { run ...; }` blocks
// and a neighbor names the ones it wants in `api { processes [ a b ] }`. A
// Fleet runs every one of them: it forks one child per declared script, fans
// each ze event out to every child's stdin, and reads commands from every
// child's stdout. Running one of two declared scripts is the silently-wrong
// answer `ai/rules/principles.md` bans, and it is what the bridge did until
// 2026-09-06.
//
// Safe for concurrent use.

package bridgerun

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"slices"
	"sync"
	"sync/atomic"
	"time"

	"github.com/ze-software/ze/internal/component/plugin"
	"github.com/ze-software/ze/internal/exabgp/bridge"
)

// Dispatcher is the engine surface a Fleet needs: one command in, one answer
// out. *sdk.Plugin implements it, and a test implements it with a recorder.
type Dispatcher interface {
	DispatchCommand(ctx context.Context, command string) (string, json.RawMessage, error)
}

// Script is one operator script the bridge runs, named by the ExaBGP `process`
// block that declared it.
//
// Argv is the command already split into its executable and its arguments. The
// config parser splits the one `run` string it is given, and the CLI receives
// argv from its own flag parser, so neither has to split twice.
//
// Respawn is ExaBGP's `respawn` leaf: true starts the script again when it
// exits, false leaves it stopped. `script.go` states the rate limit that bounds
// the restart.
//
// Encoder is ExaBGP's `encoder` leaf: the format this script's events are
// written in. Every construction site states it, because the zero value names
// no format (bridge.Encoder).
//
// Feeds are the peers that feed this script and the events each of them feeds
// it. An EMPTY list is every peer and every event, which is what one process
// and one neighbor means and what a bridge with no feed list has always meant.
type Script struct {
	Name    string
	Argv    []string
	Feeds   []ScriptFeed
	Encoder bridge.Encoder
	Respawn bool
	// OnExit is ExaBGP's `on-exit` leaf; its zero, bridge.OnExitByVersion, is the
	// leaf left unset (bridge.OwnerLedger.Exited).
	OnExit bridge.OnExit
}

// ScriptFeed is one peer that feeds one script, and the events it feeds it.
//
// ExaBGP models the relation on the NEIGHBOR: `api { processes [ a b ]; receive
// { update; } }` names both the processes and the message kinds, so two
// neighbors can grant the same process different events. A per-script event
// list alone could not say that, and the union of two grants over-delivers to
// whichever neighbor granted less.
//
// An EMPTY Events list feeds no event from this peer, which is what an ExaBGP
// api block naming a process and no message kind does. It is not "every event":
// the absence of the whole feed list is what says that.
type ScriptFeed struct {
	Peer   string
	Events []string
}

// Fleet runs every script an ExaBGP config declares.
//
// The sequence is New, Start, then Broadcast for each event, then Shutdown
// when the daemon stops, then Stop. Shutdown MUST come before the caller
// cancels the context it passed to Start, and the caller MUST cancel that
// context before it calls Stop:
// Stop closes each script's stdin and waits, and a child that has stopped
// reading its stdin is ended by that cancellation rather than by the EOF.
type Fleet struct {
	log     *slog.Logger
	scripts []*script
	wg      sync.WaitGroup

	// stopping tells each script's supervisor that the EOF it is about to read
	// is the shutdown rather than a crash, so it does not respawn into it.
	// Shutdown sets it before it closes the scripts' stdin, and Stop sets it
	// for a fleet that was never told to shut down.
	stopping bool
	mu       sync.Mutex

	// apiSetting is `exabgp.api.version` as a bridge.APIVersion: what each
	// script starts held to. Start reads it from the env, and a script's
	// `api version <v>` line moves it for the scripts started afterwards.
	apiSetting atomic.Uint32

	// owners is which script announced each route last, shared by every
	// script because a route one script announces another can take over or
	// withdraw (bridge.OwnerLedger).
	owners *bridge.OwnerLedger

	// sessions holds each peer's negotiated block for its UPDATE lines.
	sessions bridge.Sessions
}

// EventFormat answers the format a runner subscribes to the engine's events in
// for these scripts: `full` when some feed grants a script a message's bytes
// (bridge.GrantsPacket), the only form that reads them, and `parsed`
// otherwise, so the engine does not hex-encode every UPDATE for a bridge whose
// scripts never see the bytes. A script with no feed list is written the
// decoded line alone (script.forms), so it asks for nothing more.
func EventFormat(scripts []Script) string {
	for _, s := range scripts {
		for _, feed := range s.Feeds {
			granted := func(apiKey string) bool { return slices.Contains(feed.Events, apiKey) }
			if bridge.GrantsPacket(granted) {
				return plugin.FormatFull
			}
		}
	}
	return plugin.FormatParsed
}

// New builds a Fleet for the given scripts. It forks nothing: Start does.
//
// families are the families this bridge declared, in ze spelling. No
// translation reads them since a bare `announce eor` became IPv4 unicast alone,
// as in ExaBGP; the parameter and bridge.Translator.Families are owed a removal.
//
// The translator is built PER SCRIPT, because the peers an unaddressed line
// reaches are the peers that script serves.
func New(log *slog.Logger, families []string, scripts []Script) *Fleet {
	f := &Fleet{log: log, owners: bridge.NewOwnerLedger()}
	for _, s := range scripts {
		translator := bridge.Translator{Families: families, Peers: feedPeers(s.Feeds)}
		f.scripts = append(f.scripts, newScript(log, translator, &f.apiSetting, f.owners, s))
	}
	return f
}

// Count answers how many scripts this Fleet holds. It is what a caller checks
// to see that every declared process reached the runner.
func (f *Fleet) Count() int { return len(f.scripts) }

// Start forks every script and starts the goroutines that serve it. It answers
// the first fork error and names the script that produced it, and logs that
// error at ERROR with the run command and the fix (script.logStartFailure), so
// an operator meets a bad `run` command at startup rather than as a missing
// script.
//
// A script that started before the failing one keeps running, and Stop ends it.
func (f *Fleet) Start(ctx context.Context, d Dispatcher) error {
	setting, err := bridge.APIVersionFromEnv()
	if err != nil {
		return fmt.Errorf("exabgp-bridge: %w", err)
	}
	f.apiSetting.Store(uint32(setting))
	for _, s := range f.scripts {
		// A script whose encoder nobody set is refused HERE rather than
		// written in whichever format the zero value happens to reach. Every
		// construction site is in this repository, so this is a defect in ze
		// and not in an operator's config, and it fails at startup where a
		// reader meets it.
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
			s.supervise(ctx, f, d, reaped)
		}()
		go func() {
			defer f.wg.Done()
			s.writeLoop(drained)
		}()
	}
	return nil
}

// Broadcast translates one ze event and queues it for the scripts that serve
// the peer it names.
//
// The event is READ once for the whole fleet and RENDERED once per format, so
// the cost of a fan-out grows with the number of formats in use rather than
// with the number of scripts. Both matter on this path: it runs for every
// UPDATE of every peer.
//
// A script the event's peer did not grant this event is skipped, which is what
// ExaBGP's own `_notify` does: it walks the processes THAT NEIGHBOR named for
// THAT event rather than every process it runs
// (src/exabgp/reactor/api/processes.py, Processes._notify).
func (f *Fleet) Broadcast(event string) {
	var zebgp map[string]any
	if err := json.Unmarshal([]byte(event), &zebgp); err != nil {
		f.log.Warn("exabgp-bridge: invalid JSON event", "error", err)
		return
	}
	read := bridge.ReadEvent(zebgp)
	if read.Kind == "" {
		// An envelope that carries no BGP message. The bridge subscribes to
		// every event ze publishes, so it meets these, and neither ExaBGP
		// format has anything to write for one. It is logged rather than
		// dropped in silence, at debug because it is routine.
		f.log.Debug("exabgp-bridge: event carries no BGP message", "event", event)
		return
	}
	f.sessions.Note(&read)
	apiKey := read.APIKey()

	// rendered[0] holds the lines for a script the peer grants `negotiated`,
	// which carry the session's block, rendered[1] those for a script it does
	// not, which carry none, whatever the API version (Event.ForGrant). Each
	// is cached per set of forms the scripts asked a message in (bridge.Forms).
	var rendered [2][bridge.FormsCount][bridge.FormatCount][]byte
	variants := [2]bridge.Event{read.ForGrant(true), read.ForGrant(false)}
	for _, s := range f.scripts {
		if !s.receives(read.Peer, apiKey) {
			continue
		}
		forms := s.forms(read.Peer, read.Direction)
		grant := 0
		if !s.receives(read.Peer, bridge.APIKeyNegotiated) {
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
// asks for it and reusing the answer for every script after.
//
// It answers false for an event that format writes nothing for, which is an
// answer rather than a failure: ExaBGP's text encoder writes no line for
// `negotiated`, `fsm` or `signal`.
//
// The format is the script's API version and encoder together (bridge.FormatFor),
// so a fan-out renders at most bridge.FormatCount lines for one event.
func (f *Fleet) render(cache *[bridge.FormatCount][]byte, format bridge.Format, forms bridge.Forms, event bridge.Event) ([]byte, bool) {
	slot := format.Slot()
	if cache[slot] != nil {
		return cache[slot], len(cache[slot]) > 0
	}

	line := event.Lines(format, forms, f.log)

	// An empty slice is not nil, so a format that wrote nothing is cached as
	// "asked and answered" rather than rendered again for the next script.
	if line == nil {
		line = []byte{}
	}
	cache[slot] = line
	return line, len(line) > 0
}

// Shutdown writes every script ExaBGP's shutdown line (bridge.ShutdownLine),
// in its own format, whatever its api block grants: ExaBGP writes it to every
// process before it terminates them (Processes.terminate). Each runner calls it
// once its SDK Run returns, which is how the engine stops a plugin (it closes
// the connection and sends no `bye`, process.ProcessManager.Stop), while the
// scripts still run. It MUST be called before Stop, and before the caller
// cancels the context it passed to Start: that cancellation kills each child
// (exec.CommandContext), and a line still queued then is never read.
//
// So it does not only queue the line. It marks the fleet stopping, so no
// supervisor respawns a child into the EOF, queues the line behind whatever is
// already queued, and closes each started script's queue. It then waits for
// each writer to write the line and close the child's stdin, and for each
// child to read up to that EOF and exit: a written line a child has not yet
// read is lost to the kill as surely as a queued one. ExaBGP gives a child the
// same chance: it writes the line, pauses, then sends SIGTERM and waits up to
// two seconds (Processes.terminate, _terminate_run).
//
// The wait is bounded by shutdownWait for the whole fleet, not per script, so
// a child that stopped reading or ignores its EOF cannot hold the daemon's
// stop, and each script still running when the bound is hit is logged. Stop
// then finds every queue already closed.
func (f *Fleet) Shutdown() {
	f.mu.Lock()
	f.stopping = true
	f.mu.Unlock()

	var started []*script
	for _, s := range f.scripts {
		if drained, _ := s.ended(); drained == nil {
			// Start never reached this script: no child runs to be told.
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
// the scripts to receive the shutdown line and exit on their EOF. A script
// that reads its stdin does both in milliseconds; the bound is what a script
// that does neither costs the daemon's stop.
const shutdownWait = time.Second

// untilEnded answers true once ended is closed, and false if the bound fires
// first. A channel already closed wins over a bound that fired at the same
// moment, so a script that did end is never logged as late.
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

// lateAtShutdown logs each script Shutdown stopped waiting for. The context
// cancellation that follows kills it, so it may not have read the shutdown
// line.
func (f *Fleet) lateAtShutdown(late []*script) {
	for _, s := range late {
		f.log.Warn("exabgp-bridge: script still running when the shutdown wait ended; it is killed and may not have read the shutdown line",
			"script", s.name, "bound", shutdownWait)
	}
}

// Stop closes every script's stdin, which gives each child an EOF, and waits
// for every goroutine this Fleet started. It MUST be called after the context
// passed to Start is canceled, and it MAY be called when Start failed.
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

// stopped answers whether Stop has been called.
func (f *Fleet) stopped() bool {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.stopping
}
