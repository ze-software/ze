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
	log     *slog.Logger

	// translator turns one ExaBGP line into ze commands. It carries the family
	// list a bare `announce eor` expands over, and the peers a line that names
	// no neighbor reaches.
	translator bridge.Translator

	// encoder is the format this script's events are written in, from the
	// `encoder` leaf of the process block that declared it. It is per script
	// because ExaBGP declares it per process, so one bridge can feed a text
	// script and a JSON script at once.
	encoder bridge.Encoder

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

func newScript(log *slog.Logger, translator bridge.Translator, s Script) *script {
	return &script{
		name:       s.Name,
		argv:       s.Argv,
		respawn:    s.Respawn,
		log:        log,
		translator: translator,
		encoder:    s.Encoder,
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
	return nil
}

// supervise is the script's own goroutine. It reads the child's stdout until
// the pipe closes, reaps the child, and starts it again when the config asks
// for a respawn and the rate limit allows one.
func (s *script) supervise(ctx context.Context, f *Fleet, d Dispatcher) {
	for {
		s.read(ctx, d)
		s.reap()

		if ctx.Err() != nil || f.stopped() {
			return
		}
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

// batch translates, nets, dispatches, flushes and answers the lines of one read.
//
// The order is fixed and each step earns its place. The netting is ExaBGP's own
// (bridge.Net). The flush comes BEFORE the answers, because `done` means the
// command is done and a route is not done until it is on the wire: acking first
// let a script send its NEXT command while the previous route's flush was still
// running, and api-ipv4, api-ipv6, api-mvpn and api-vpnv4 each caught it. The
// answers come last, one per line, in the order the script wrote them.
func (s *script) batch(ctx context.Context, d Dispatcher, lines []string) {
	batch := make([]bridge.BatchLine, 0, len(lines))
	for _, text := range lines {
		if text == "" {
			batch = append(batch, bridge.BatchLine{Text: text})
			continue
		}
		translation, err := s.translator.Line(text)
		if err != nil {
			// The bridge names the line it refused rather than handing an
			// untranslated line to ze's dispatcher, where it would die as an
			// unknown command with no mention of the bridge.
			s.log.Warn("exabgp-bridge line refused", "script", s.name, "error", err)
		}
		batch = append(batch, bridge.BatchLine{Text: text, Translation: translation, Err: err})
	}

	netted := bridge.Net(batch)
	answers := make([]bridge.BatchAnswer, len(batch))
	for _, dispatch := range netted {
		if answers[dispatch.Line].Failed {
			// This line already met a refusal. Its remaining commands describe
			// the same route, and the rest of the BATCH keeps going: one line's
			// failure never swallows another line's answer.
			continue
		}
		if _, _, derr := d.DispatchCommand(ctx, dispatch.Command.Text); derr != nil {
			s.log.Warn("exabgp-bridge dispatch failed",
				"script", s.name, "error", derr, "cmd", dispatch.Command.Text)
			answers[dispatch.Line] = bridge.BatchAnswer{Failed: true, Error: derr.Error()}
		}
	}

	// Route commands: one per-peer flush for each selector the batch reached, so
	// the forward pool drains before any line of it is answered. ExaBGP orders
	// the two the same way: announce_route awaits every peer's flush event and
	// calls answer_done after it (src/exabgp/reactor/api/command/announce.py).
	for _, selector := range bridge.BatchSelectors(batch, netted) {
		var tb textbuf.Buffer
		flush := tb.Str("request peer ").Str(selector).Str(" flush").String()
		if _, _, ferr := d.DispatchCommand(ctx, flush); ferr != nil {
			s.log.Warn("exabgp-bridge flush failed", "script", s.name, "error", ferr, "peer", selector)
		}
	}

	// The script is waiting for these. An ExaBGP API client sends one command,
	// blocks for `done`, and gives up after two seconds, so a runner that
	// dispatches without acking delivers exactly one command per script.
	bridge.AnswerBatch(s.writer(), &s.ack, batch, answers)
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
