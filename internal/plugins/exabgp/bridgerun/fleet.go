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
	"sync"

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
// The sequence is New, Start, then Broadcast for each event, then Stop. The
// caller MUST cancel the context it passed to Start before it calls Stop:
// Stop closes each script's stdin and waits, and a child that has stopped
// reading its stdin is ended by that cancellation rather than by the EOF.
type Fleet struct {
	log     *slog.Logger
	scripts []*script
	wg      sync.WaitGroup

	// stopping tells each script's supervisor that the EOF it is about to read
	// is the shutdown rather than a crash, so it does not respawn into it.
	stopping bool
	mu       sync.Mutex
}

// New builds a Fleet for the given scripts. It forks nothing: Start does.
//
// families are the families this bridge declared, in ze spelling. A bare
// `announce eor` is every one of them, and the line itself names none, so the
// translator carries the set (bridge.Translator).
//
// The translator is built PER SCRIPT, because the peers an unaddressed line
// reaches are the peers that script serves.
func New(log *slog.Logger, families []string, scripts []Script) *Fleet {
	f := &Fleet{log: log}
	for _, s := range scripts {
		translator := bridge.Translator{Families: families, Peers: feedPeers(s.Feeds)}
		f.scripts = append(f.scripts, newScript(log, translator, s))
	}
	return f
}

// Count answers how many scripts this Fleet holds. It is what a caller checks
// to see that every declared process reached the runner.
func (f *Fleet) Count() int { return len(f.scripts) }

// Start forks every script and starts the goroutines that serve it. It answers
// the first fork error and names the script that produced it, so an operator
// meets a bad `run` command at startup rather than as a missing script.
//
// A script that started before the failing one keeps running, and Stop ends it.
func (f *Fleet) Start(ctx context.Context, d Dispatcher) error {
	for _, s := range f.scripts {
		// A script whose encoder nobody set is refused HERE rather than
		// written in whichever format the zero value happens to reach. Every
		// construction site is in this repository, so this is a defect in ze
		// and not in an operator's config, and it fails at startup where a
		// reader meets it.
		if s.encoder == bridge.EncoderUnspecified {
			return fmt.Errorf("exabgp-bridge: script %s: no encoder was set for it", s.name)
		}
		if err := s.start(ctx); err != nil {
			return fmt.Errorf("exabgp-bridge: script %s: %w", s.name, err)
		}
		f.wg.Add(2)
		go func() {
			defer f.wg.Done()
			s.supervise(ctx, f, d)
		}()
		go func() {
			defer f.wg.Done()
			s.writeLoop()
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
	apiKey := read.APIKey()

	var rendered [2][]byte
	for _, s := range f.scripts {
		if !s.receives(read.Peer, apiKey) {
			continue
		}
		line, ok := f.render(&rendered, s.encoder, read)
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
func (f *Fleet) render(cache *[2][]byte, encoder bridge.Encoder, event bridge.Event) ([]byte, bool) {
	slot := encoderSlot(encoder)
	if cache[slot] != nil {
		return cache[slot], len(cache[slot]) > 0
	}

	var line []byte
	switch encoder {
	case bridge.EncoderText:
		var scratch [512]byte
		out, form := event.AppendText(scratch[:0])
		switch form {
		case bridge.TextWritten:
			line = out
		case bridge.TextUnknown:
			f.log.Warn("exabgp-bridge: no text form for this event; the script is not told about it",
				"event", event.Kind)
		case bridge.TextNone:
			// ExaBGP writes no text line for this kind either.
		}
	default:
		// EncoderJSON. Start refuses a fleet holding any other value, so this
		// branch is the JSON format rather than a fallback for an unset one.
		encoded, err := json.Marshal(event.ExabgpJSON())
		if err != nil {
			f.log.Warn("exabgp-bridge: marshal external JSON failed", "error", err)
			break
		}
		encoded = append(encoded, '\n')
		line = encoded
	}

	// An empty slice is not nil, so a format that wrote nothing is cached as
	// "asked and answered" rather than rendered again for the next script.
	if line == nil {
		line = []byte{}
	}
	cache[slot] = line
	return line, len(line) > 0
}

// encoderSlot indexes the per-format render cache. The two formats are the
// whole set (bridge.Encoder), and Start refuses a script that names neither.
func encoderSlot(encoder bridge.Encoder) int {
	if encoder == bridge.EncoderText {
		return 1
	}
	return 0
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
