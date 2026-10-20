// Design: docs/architecture/exabgp-bridge.md -- the scripts the bridge runs
// Overview: fleet.go -- the Fleet that owns every script
//
// One script is one ExaBGP `process` block: a child process, a queue of lines
// bound for its stdin, a reader that turns its stdout into ze commands, and a
// supervisor that decides whether a child that exited is started again.

package bridgerun

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"os"
	"os/exec"
	"sync"
	"sync/atomic"
	"time"

	"github.com/ze-software/ze/internal/core/textbuf"
	"github.com/ze-software/ze/internal/exabgp/bridge"
)

// queueDepth bounds the lines held for one script's stdin.
//
// The bound is what keeps one script's failure to its own process. A child that
// stops reading fills its pipe, and a direct write into that pipe blocks the
// caller: the SDK event loop for an event, and with several scripts the fan-out
// to every sibling as well. The queue and its writer goroutine put that stall
// inside the one script, and the depth is where the stall becomes a drop.
//
// ExaBGP holds the same number for the same reason and calls it
// WRITE_QUEUE_HIGH_WATER (src/exabgp/reactor/api/processes.py).
const queueDepth = 1000

// errRespawnTooFast is answered by start when the rate limit refuses it.
var errRespawnTooFast = errors.New("respawned too fast")

// script is one running ExaBGP API process.
//
// It owns two goroutines for as long as the Fleet runs: a supervisor, which
// reads the child's stdout and restarts the child, and a writer, which drains
// the queue into the child's stdin. Both end when Fleet.Stop closes the queue.
type script struct {
	name    string
	argv    []string
	respawn bool
	onExit  bridge.OnExit
	log     *slog.Logger

	// owners is the Fleet's ledger of who announced each route last. batch
	// notes every route command it dispatched, and exited withdraws this
	// script's routes from it.
	owners *bridge.OwnerLedger

	// translator turns one ExaBGP line into ze commands. It carries the family
	// list a bare `announce eor` expands over, and the peers a line that names
	// no neighbor reaches.
	translator bridge.Translator

	// api is the API version this script is held to and the format its
	// events are written in (bridge.ScriptVersion). Its encoder is the
	// `encoder` leaf of the process block that declared the script, per
	// script because ExaBGP declares it per process, so one bridge can feed a
	// text script and a JSON script at once. Its setting is the Fleet's
	// `exabgp.api.version`, shared by every script.
	api *bridge.ScriptVersion

	// grants is the peer-and-event relation each neighbor's api block declared
	// for this script: the events one peer feeds it, by peer address. A NIL map
	// is every peer and every event, which is what a script no neighbor
	// singles out is fed by.
	grants map[string]map[string]struct{}

	// ack answers this script after each dispatched command. An ExaBGP API
	// client blocks on `done` before it sends the next line.
	//
	// It is per script, not per fleet, because the state moves: `disable-ack`
	// and `enable-ack` are lines a script writes about ITSELF (bridge.AckMode).
	// One AckMode shared by the fleet would let one script silence another's
	// acks, and the silenced script would then block for two seconds on every
	// line it sent.
	ack bridge.AckMode

	// group is the script's `group start` buffer (bridge.Group). It is per
	// script for the reason ack is, and it is emptied when a new child
	// starts: ExaBGP drops it when the process behind it is gone
	// (clear_group).
	group bridge.Group

	// queue carries the lines bound for the child's stdin, from the event
	// fan-out and from this script's own acks. One writer goroutine drains it,
	// so no two writes into the pipe interleave.
	queue chan []byte

	limiter respawnLimiter
	dropped atomic.Uint64

	// mu guards the child and its pipes across the supervisor, the writer and
	// Stop. No write into a pipe is performed while it is held.
	mu     sync.Mutex
	closed bool
	child  *exec.Cmd
	stdin  io.WriteCloser
	stdout io.ReadCloser
}

func newScript(log *slog.Logger, translator bridge.Translator, setting *atomic.Uint32, owners *bridge.OwnerLedger, s Script) *script {
	return &script{
		api:        bridge.NewScriptVersion(s.Name, log, s.Encoder, setting),
		name:       s.Name,
		argv:       s.Argv,
		respawn:    s.Respawn,
		onExit:     s.OnExit,
		owners:     owners,
		log:        log,
		translator: translator,
		grants:     grantSet(s.Feeds),
		ack:        bridge.NewAckMode(),
		queue:      make(chan []byte, queueDepth),
	}
}

// grantSet builds the membership test the fan-out runs for every event. It is
// built ONCE, at startup, because the fan-out runs for every UPDATE of every
// peer.
//
// An empty feed list answers nil, which receives() reads as every peer and
// every event.
func grantSet(feeds []ScriptFeed) map[string]map[string]struct{} {
	if len(feeds) == 0 {
		return nil
	}
	grants := make(map[string]map[string]struct{}, len(feeds))
	for _, feed := range feeds {
		events, ok := grants[feed.Peer]
		if !ok {
			events = make(map[string]struct{}, len(feed.Events))
			grants[feed.Peer] = events
		}
		for _, event := range feed.Events {
			events[event] = struct{}{}
		}
	}
	return grants
}

// receives reports whether one peer feeds this script one event, named by the
// key ExaBGP's api block grants it with (bridge.Event.APIKey).
//
// A script with no feed list receives every event of every peer. That is what a
// config with one script means, and it is what the bridge did for every script
// until 2026-09-06.
func (s *script) receives(peer, apiKey string) bool {
	if s.grants == nil {
		return true
	}
	events, ok := s.grants[peer]
	if !ok {
		return false
	}
	_, ok = events[apiKey]
	return ok
}

// feedPeers answers the addresses of the peers a script is fed by, which is the
// set an ExaBGP line that names no neighbor is sent to.
//
// ExaBGP scopes a process's COMMANDS by peer alone: Reactor.peers(service)
// reads `api['processes']` and never the per-message grants
// (src/exabgp/reactor/loop.py). So a peer that feeds a script no event at all
// still receives the routes that script announces.
func feedPeers(feeds []ScriptFeed) []string {
	if len(feeds) == 0 {
		return nil
	}
	peers := make([]string, 0, len(feeds))
	for _, feed := range feeds {
		peers = append(peers, feed.Peer)
	}
	return peers
}

// start forks the child and records the fork against the respawn limit.
func (s *script) start(ctx context.Context) error {
	if len(s.argv) == 0 {
		return errors.New("no run command")
	}
	if !s.limiter.count(time.Now()) {
		return errRespawnTooFast
	}

	//nolint:gosec // G204: the operator's own script command is what the bridge exists to run.
	c := exec.CommandContext(ctx, s.argv[0], s.argv[1:]...)
	sin, err := c.StdinPipe()
	if err != nil {
		return err
	}
	sout, err := c.StdoutPipe()
	if err != nil {
		return err
	}
	c.Stderr = os.Stderr

	if err := c.Start(); err != nil {
		return err
	}

	s.mu.Lock()
	s.child = c
	s.stdin = sin
	s.stdout = sout
	s.mu.Unlock()
	s.api.Reset()
	s.group = bridge.Group{}
	return nil
}

// supervise is the script's own goroutine. It reads the child's stdout until
// the pipe closes, reaps the child, and starts it again when the config asks
// for a respawn and the rate limit allows one.
func (s *script) supervise(ctx context.Context, f *Fleet, d Dispatcher) {
	for {
		s.read(ctx, d)
		s.reap()

		if ctx.Err() != nil {
			return
		}
		if f.stopped() {
			return
		}
		// The exit is settled BEFORE the next child starts, so the withdrawals
		// reach the wire ahead of anything the respawned child announces, as
		// ExaBGP queues its exit marker ahead of the new child's commands.
		s.exited(ctx, d)
		if !s.respawn {
			// api-no-respawn asserts exactly this: the one-shot script exits
			// after its ack and is not started again, while its sibling keeps
			// running.
			s.log.Info("exabgp-bridge script exited", "script", s.name, "respawn", false)
			return
		}
		if err := s.start(ctx); err != nil {
			if errors.Is(err, errRespawnTooFast) {
				s.log.Error("exabgp-bridge script restarted too often; it stays stopped",
					"script", s.name, "starts", respawnMax, "window", respawnWindow)
				return
			}
			s.log.Error("exabgp-bridge script restart failed", "script", s.name, "error", err)
			return
		}
		s.log.Info("exabgp-bridge script restarted", "script", s.name)
	}
}

// read consumes the child's stdout until the pipe closes, turning each read
// into one batch of commands.
//
// One read is one batch, and this goroutine does NO wire work: it hands each
// batch to the dispatcher below and returns to the pipe at once. A reader that
// waited for the flush would let the script's NEXT write join the batch it was
// still dispatching, and the netting would then cancel two commands the script
// wrote in two separate writes (internal/exabgp/bridge/bridge_batch.go).
func (s *script) read(ctx context.Context, d Dispatcher) {
	s.mu.Lock()
	sout := s.stdout
	s.mu.Unlock()
	if sout == nil {
		return
	}

	batches := make(chan []string, bridge.BatchQueueDepth)

	var dispatching sync.WaitGroup
	dispatching.Go(func() {
		for lines := range batches {
			s.batch(ctx, d, lines)
		}
	})

	defer func() {
		close(batches)
		dispatching.Wait()
	}()

	reader := bridge.NewBatchReader(sout)
	for {
		if ctx.Err() != nil {
			return
		}
		lines, err := reader.Next()
		if len(lines) > 0 {
			select {
			case batches <- lines:
			case <-ctx.Done():
				return
			}
		}
		if err == nil {
			continue
		}
		if !errors.Is(err, io.EOF) {
			s.log.Warn("exabgp-bridge script stdout read error", "script", s.name, "error", err)
		}
		return
	}
}

// batch translates, nets, dispatches, flushes and answers the lines of one
// read, through the steps every runner shares (bridge.ScriptLines.Batch).
func (s *script) batch(ctx context.Context, d Dispatcher, lines []string) {
	bridge.ScriptLines{
		Name:       s.name,
		Log:        s.log,
		Translator: s.translator,
		API:        s.api,
		Ack:        &s.ack,
		Group:      &s.group,
	}.Batch(ctx, scriptRunner{s: s, d: d}, s.writer(), lines)
}

// scriptRunner is how this runner reaches ze for one script's batch: the
// plugin dispatcher, and the fleet's owner ledger.
type scriptRunner struct {
	s *script
	d Dispatcher
}

// Dispatch hands ze one command through the plugin dispatcher. A route ze
// accepted and sent to no peer is a refusal (bridge.RefuseUnsentRoute, X-23).
func (r scriptRunner) Dispatch(ctx context.Context, command string) error {
	status, document, err := r.d.DispatchCommand(ctx, command)
	if err != nil {
		return err
	}
	return bridge.RefuseUnsentRoute(command, status, document)
}

// Flush drains the forward pool of the peers selector names.
func (r scriptRunner) Flush(ctx context.Context, selector string) { r.s.flush(ctx, r.d, selector) }

// Status performs a `status` line.
func (r scriptRunner) Status(ctx context.Context, asJSON bool) ([]byte, bridge.BatchAnswer) {
	return r.s.status(ctx, r.d, asJSON)
}

// Dispatched makes the script the owner of the route command ze accepted.
func (r scriptRunner) Dispatched(selector string, command bridge.Command) {
	r.s.owners.Note(r.s.name, selector, command)
}

// flush drains the forward pool of the peers selector names, so what was
// dispatched for them is on the wire before the caller goes on.
func (s *script) flush(ctx context.Context, d Dispatcher, selector string) {
	var tb textbuf.Buffer
	flush := tb.Str("request peer ").Str(selector).Str(" flush").String()
	if _, _, err := d.DispatchCommand(ctx, flush); err != nil {
		s.log.Warn("exabgp-bridge flush failed", "script", s.name, "error", err, "peer", selector)
	}
}

// exited settles the routes of a child that has exited
// (bridge.OwnerLedger.Exited): the `on-exit` leaf, or the API version the
// child was held to, decides whether the routes this script owns are
// withdrawn.
//
// It MUST run after the child's last batch is dispatched (script.read has
// returned) and before the next child starts, while the version the child was
// held to is still the script's.
func (s *script) exited(ctx context.Context, d Dispatcher) {
	s.owners.Exited(ctx, s.log, s.name, s.onExit, s.api.Current(), scriptRunner{s: s, d: d})
}

// status performs a `status` line: it reads every peer's state from ze and
// answers ExaBGP's daemon status. A peer list ze did not give is never
// written, so a failed read answers the line with an error.
func (s *script) status(ctx context.Context, d Dispatcher, asJSON bool) ([]byte, bridge.BatchAnswer) {
	_, detail, err := d.DispatchCommand(ctx, bridge.StatusPeerCommand)
	if err != nil {
		s.log.Warn("exabgp-bridge status: peer list refused", "script", s.name, "error", err, "cmd", bridge.StatusPeerCommand)
		return nil, bridge.BatchAnswer{Failed: true, Error: err.Error()}
	}
	peers, err := bridge.StatusPeers(detail)
	if err != nil {
		s.log.Warn("exabgp-bridge status: peer list unreadable", "script", s.name, "error", err)
		return nil, bridge.BatchAnswer{Failed: true, Error: err.Error()}
	}
	return bridge.StatusReply(peers, asJSON), bridge.BatchAnswer{}
}

// reap waits for the exited child and forgets its pipes. It runs after read
// has returned, because Wait closes the stdout pipe under any reader still on
// it (os/exec, Cmd.StdoutPipe).
func (s *script) reap() {
	s.mu.Lock()
	c := s.child
	s.child = nil
	s.stdin = nil
	s.stdout = nil
	s.mu.Unlock()

	if c == nil {
		return
	}
	if err := c.Wait(); err != nil {
		s.log.Debug("exabgp-bridge script exited", "script", s.name, "error", err)
	}
}

// writeLoop drains the queue into the child's stdin. It is the one writer the
// pipe has, so an event and an ack never interleave inside one line.
func (s *script) writeLoop() {
	for line := range s.queue {
		s.mu.Lock()
		w := s.stdin
		s.mu.Unlock()
		if w == nil {
			// The child is down. Its events are lost, as they are for ExaBGP:
			// there is no process to read them.
			s.dropped.Add(1)
			continue
		}
		if _, err := w.Write(line); err != nil {
			s.log.Debug("exabgp-bridge write to script failed", "script", s.name, "error", err)
		}
	}

	s.mu.Lock()
	w := s.stdin
	s.mu.Unlock()
	if w != nil {
		if err := w.Close(); err != nil {
			s.log.Debug("exabgp-bridge close script stdin", "script", s.name, "error", err)
		}
	}
}

// send queues one line for this script's stdin. The line is copied, because
// every caller hands over a buffer it reuses.
func (s *script) send(line []byte) {
	held := make([]byte, len(line))
	copy(held, line)

	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		return
	}
	select {
	case s.queue <- held:
	default:
		if s.dropped.Add(1) == 1 {
			s.log.Warn("exabgp-bridge script is not reading its stdin; lines are dropped",
				"script", s.name, "depth", queueDepth)
		}
	}
}

// closeQueue ends the writer goroutine, which closes the child's stdin.
func (s *script) closeQueue() {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		return
	}
	s.closed = true
	close(s.queue)
}

// reportDrops states the lines this script never received, so a silent loss is
// never the whole record of it.
func (s *script) reportDrops() {
	if n := s.dropped.Load(); n > 0 {
		s.log.Warn("exabgp-bridge script missed lines", "script", s.name, "lines", n)
	}
}

// writer answers an io.Writer that queues what is written to it, for the ack
// path, which formats into a buffer it reuses and writes it once.
func (s *script) writer() io.Writer { return queueWriter{s: s} }

type queueWriter struct{ s *script }

func (w queueWriter) Write(p []byte) (int, error) {
	w.s.send(p)
	return len(p), nil
}
