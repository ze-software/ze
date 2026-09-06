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
type Script struct {
	Name    string
	Argv    []string
	Respawn bool
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
func New(log *slog.Logger, families []string, scripts []Script) *Fleet {
	f := &Fleet{log: log}
	translator := bridge.Translator{Families: families}
	for _, s := range scripts {
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

// Broadcast translates one ze JSON event into ExaBGP JSON and queues it for
// every script. The translation happens once for the whole fleet.
func (f *Fleet) Broadcast(event string) {
	var zebgp map[string]any
	if err := json.Unmarshal([]byte(event), &zebgp); err != nil {
		f.log.Warn("exabgp-bridge: invalid JSON event", "error", err)
		return
	}
	out, err := json.Marshal(bridge.ZebgpToExabgpJSON(zebgp))
	if err != nil {
		f.log.Warn("exabgp-bridge: marshal external JSON failed", "error", err)
		return
	}
	out = append(out, '\n')
	for _, s := range f.scripts {
		s.send(out)
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
