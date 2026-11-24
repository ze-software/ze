// Design: docs/architecture/exabgp-bridge.md -- one ExaBGP script's process lifecycle.
// Overview: fleet.go -- the Fleet that owns every script.
//
// One script is one child process, a bounded queue of lines bound for its
// stdin, a stdout reader that translates its ExaBGP commands into Ze CLI
// dispatches, and a single-run watcher that reports the script's exit code to
// its Fleet so the wrapper exits with its script (E-6).

package bridgerun

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"net/netip"
	"os"
	"os/exec"
	"slices"
	"strings"
	"sync"
	"sync/atomic"

	"github.com/ze-software/ze/internal/core/textbuf"
	"github.com/ze-software/ze/internal/exabgp/bridge"
)

// queueDepth bounds the lines held for one script's stdin.
//
// ExaBGP holds the same number for the same reason and calls it
// WRITE_QUEUE_HIGH_WATER (src/exabgp/reactor/api/processes.py).
const queueDepth = 1000

// negotiatedScope describes which peers grant `bgp:negotiated` to a script.
type negotiatedScope uint8

const (
	negotiatedUnspecified negotiatedScope = iota
	negotiatedAll
	negotiatedNone
	negotiatedPeers
)

// script is one running ExaBGP API process.
//
// It owns two goroutines for as long as the Fleet runs: runOnce, which reads
// the child's stdout until EOF and reaps the child once, and writeLoop, which
// drains the queue into the child's stdin.
type script struct {
	name  string
	argv  []string
	log   *slog.Logger
	forms FormSpecs

	// filterMu guards translator, negScope, and negPeers across concurrent
	// ApplyFilter, Broadcast, and batch calls.
	filterMu   sync.RWMutex
	translator bridge.Translator
	negScope   negotiatedScope
	negPeers   map[netip.Addr]bool

	// api is the API version this script is held to and the format its events
	// are written in (bridge.ScriptVersion).
	api *bridge.ScriptVersion

	// ack answers this script after each dispatched command.
	ack bridge.AckMode

	// group is the script's `group start` buffer (bridge.Group).
	group bridge.Group

	// queue carries the lines bound for the child's stdin, from event delivery
	// and from this script's own acks. One writer goroutine drains it, so no two
	// writes into the pipe interleave.
	queue chan []byte

	// drained is closed by writeLoop once it has written its last queued line
	// and closed the child's stdin; reaped is closed by runOnce once the child
	// has been reaped.
	drained chan struct{}
	reaped  chan struct{}

	dropped atomic.Uint64

	// mu guards the child and its pipes across runOnce, writeLoop, and Stop.
	// No write into a pipe is performed while it is held.
	mu     sync.Mutex
	closed bool
	child  *exec.Cmd
	stdin  io.WriteCloser
	stdout io.ReadCloser
}

func newScript(log *slog.Logger, translator bridge.Translator, setting *atomic.Uint32, s Script) *script {
	return &script{
		api:        bridge.NewScriptVersion(s.Name, log, s.Encoder, setting),
		name:       s.Name,
		argv:       s.Argv,
		log:        log,
		forms:      s.Forms,
		translator: translator,
		negScope:   negotiatedAll,
		ack:        bridge.NewAckMode(),
		queue:      make(chan []byte, queueDepth),
	}
}

// setFilter updates the script's send scope and negotiated-block grant set
// atomically under filterMu.
func (s *script) setFilter(peers []string, noPeers bool, negScope negotiatedScope, negPeers map[netip.Addr]bool) {
	s.filterMu.Lock()
	s.translator.Peers = slices.Clone(peers)
	s.translator.NoPeers = noPeers
	s.negScope = negScope
	s.negPeers = negPeers
	s.filterMu.Unlock()
}

// snapshotTranslator returns a safe copy of the script's current Translator.
func (s *script) snapshotTranslator() bridge.Translator {
	s.filterMu.RLock()
	tr := s.translator
	s.filterMu.RUnlock()
	return tr
}

// grantsNegotiated reports whether peer grants `bgp:negotiated` to this script.
func (s *script) grantsNegotiated(peer string) bool {
	s.filterMu.RLock()
	scope := s.negScope
	peers := s.negPeers
	s.filterMu.RUnlock()

	switch scope {
	case negotiatedAll:
		return true
	case negotiatedNone:
		return false
	case negotiatedPeers:
		addr, err := netip.ParseAddr(peer)
		if err != nil {
			return false
		}
		return peers[addr]
	case negotiatedUnspecified:
		panic("BUG: bridgerun: negotiated scope unspecified")
	default:
		panic("BUG: bridgerun: invalid negotiated scope")
	}
}

// start forks the child process once.
func (s *script) start(ctx context.Context) error {
	if len(s.argv) == 0 {
		return errors.New("no run command")
	}

	//nolint:gosec // G204: the operator's own script command is what the wrapper exists to run.
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

// logStartFailure tells the operator that the script could not be forked.
func (s *script) logStartFailure(err error) {
	program := "the program"
	if len(s.argv) > 0 {
		program = s.argv[0]
	}
	s.log.Error("exabgp-bridge script could not start, so no route it announces or withdraws reaches any peer",
		"script", s.name,
		"run", strings.Join(s.argv, " "),
		"error", err,
		"path", os.Getenv("PATH"),
		"action", "give the full path of "+program+" in the endpoint external run command, or put the directory that holds "+program+" on ze's PATH, and make "+program+" executable")
}

// runOnce is the script's single-run watcher goroutine. It reads the child's
// stdout until EOF, reaps the child once without respawning it (E-6), and
// notifies the Fleet with the script's exit status when the exit was not
// caused by Shutdown/Stop or context cancellation.
func (s *script) runOnce(ctx context.Context, f *Fleet, d Dispatcher, reaped chan<- struct{}) {
	defer close(reaped)

	s.read(ctx, d)
	exitErr := s.reap()

	if ctx.Err() != nil {
		return
	}
	if f.stopped() {
		return
	}

	exitCode := 0
	if exitErr != nil {
		if ee, ok := errors.AsType[*exec.ExitError](exitErr); ok {
			exitCode = ee.ExitCode()
		} else {
			exitCode = 1
		}
	}
	s.log.Info("exabgp-bridge script exited", "script", s.name, "exit_code", exitCode)
	f.notifyScriptExit(exitCode)
}

// read consumes the child's stdout until the pipe closes, turning each read
// into one batch of commands.
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

// batch translates, nets, dispatches, flushes, and answers the lines of one
// read through bridge.ScriptLines.Batch.
func (s *script) batch(ctx context.Context, d Dispatcher, lines []string) {
	bridge.ScriptLines{
		Name:       s.name,
		Log:        s.log,
		Translator: s.snapshotTranslator(),
		API:        s.api,
		Ack:        &s.ack,
		Group:      &s.group,
	}.Batch(ctx, scriptRunner{s: s, d: d}, s.writer(), lines)
}

// scriptRunner is how bridgerun reaches ze for one script's batch.
type scriptRunner struct {
	s *script
	d Dispatcher
}

// Dispatch hands ze one command through the plugin dispatcher.
func (r scriptRunner) Dispatch(ctx context.Context, command string) error {
	_, _, err := r.d.DispatchCommand(ctx, command)
	return err
}

// Flush drains the forward pool of the peers selector names.
func (r scriptRunner) Flush(ctx context.Context, selector string) error {
	return r.s.flush(ctx, r.d, selector)
}

// Status performs a `status` line.
func (r scriptRunner) Status(ctx context.Context, asJSON bool) ([]byte, bridge.BatchAnswer) {
	return r.s.status(ctx, r.d, asJSON)
}

// flush drains the forward pool of the peers selector names.
func (s *script) flush(ctx context.Context, d Dispatcher, selector string) error {
	var tb textbuf.Buffer
	flush := tb.Str("request peer ").Str(selector).Str(" flush").String()
	_, _, err := d.DispatchCommand(ctx, flush)
	return err
}

// status performs a `status` line from ze's peer list.
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

// reap waits for the exited child, forgets its pipes, and returns Wait's error.
func (s *script) reap() error {
	s.mu.Lock()
	c := s.child
	s.mu.Unlock()

	var exitErr error
	if c != nil {
		exitErr = c.Wait()
	}

	s.mu.Lock()
	s.child = nil
	s.stdin = nil
	s.stdout = nil
	s.mu.Unlock()
	return exitErr
}

// writeLoop drains the queue into the child's stdin and closes drained after
// closing the child's stdin.
func (s *script) writeLoop(drained chan<- struct{}) {
	defer close(drained)

	for line := range s.queue {
		s.mu.Lock()
		w := s.stdin
		s.mu.Unlock()
		if w == nil {
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

// send queues one line for this script's stdin.
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

// ended answers the channels the writer and watcher close when each ends.
func (s *script) ended() (drained, reaped <-chan struct{}) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.drained, s.reaped
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

// reportDrops logs lines dropped when the script stopped reading stdin.
func (s *script) reportDrops() {
	if n := s.dropped.Load(); n > 0 {
		s.log.Warn("exabgp-bridge script missed lines", "script", s.name, "lines", n)
	}
}

// writer answers an io.Writer that queues what is written to it.
func (s *script) writer() io.Writer { return queueWriter{s: s} }

type queueWriter struct{ s *script }

func (w queueWriter) Write(p []byte) (int, error) {
	w.s.send(p)
	return len(p), nil
}
