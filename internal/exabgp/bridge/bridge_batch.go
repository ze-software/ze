// Design: docs/architecture/exabgp-bridge.md -- the batch is the unit a script writes
// Overview: bridge_command.go -- Translator.Line, which states the RouteKey this file nets on
// Related: bridge.go and internal/plugins/exabgp/bridgerun/script.go -- the two runners that read batches
//
// One write by an API script is one batch. ExaBGP reads its processes once per
// reactor cycle and holds what that read returned in its outgoing RIB, where the
// commands cancel each other before anything is encoded
// (src/exabgp/reactor/api/processes.py `received`, src/exabgp/rib/outgoing.py).
// A script that writes four lines in one write therefore reads back the frames
// of ONE netted set, not four.
//
// This file is that unit for ze, written once and read by both runners.

package bridge

import (
	"bytes"
	"errors"
	"io"
	"log/slog"
	"slices"
)

// batchReadSize is the byte count one read of a script's pipe takes.
//
// It is ze's `bufio` default. The number decides how many lines can share a
// batch, and a larger one only widens the window in which two writes a script
// made separately are read together.
const batchReadSize = 4096

// batchLineMax bounds one command line.
//
// A line is carried between reads until its newline arrives, so the carry is
// what has to be bounded rather than the read. The bound is far above any
// command an ExaBGP script writes: the longest in the ported qa/api corpus is a
// flow route of about 200 bytes. A line that passes it is REFUSED by name and
// its bytes are dropped up to the next newline, never truncated into a shorter
// line that would parse as a different command.
const batchLineMax = 64 * 1024

// BatchQueueDepth bounds the batches a reader may run ahead of its dispatcher.
//
// The depth is what buys the decoupling: while it has room, a read never waits
// for the wire, so two writes a script made separately stay two batches. A full
// queue BLOCKS the reader, which is the correct behavior when a script writes
// faster than ze can send -- the alternative is dropping commands, and a
// dropped announce is a prefix that never reaches the peer. A blocked reader
// falls back to ExaBGP's own shape, where the reactor reads once per cycle and
// does its wire work between reads.
const BatchQueueDepth = 64

// batchEmptyReadMax bounds a reader that answers zero bytes and no error.
//
// io.Reader discourages that answer and a pipe never gives it, but a caller
// looping on Next would spin against a reader that did. `bufio.Scanner` bounds
// the same case at the same count and answers io.ErrNoProgress, so this one
// does too.
const batchEmptyReadMax = 100

// ErrLineTooLong reports a command line longer than batchLineMax.
var ErrLineTooLong = errors.New("exabgp api line too long")

// BatchReader cuts a script's byte stream into batches: one read is one batch.
//
// It replaces a bufio.Scanner, which answers one line at a time and so cannot
// say which lines arrived together. What arrived together is the whole question
// the netting asks.
//
// A read that ends inside a line carries the partial line into the next batch.
// That carry is why a batch is "the complete lines of one read" rather than
// "the bytes of one read".
//
// NOT safe for concurrent use: one reader goroutine owns it.
type BatchReader struct {
	r    io.Reader
	buf  []byte
	tail []byte
	// dropping is set while a line past batchLineMax is being discarded, so its
	// remaining bytes never become a line of their own.
	dropping bool
}

// NewBatchReader reads batches from one script's stdout.
func NewBatchReader(r io.Reader) *BatchReader {
	return &BatchReader{r: r, buf: make([]byte, batchReadSize)}
}

// Next answers the complete lines of one read.
//
// It answers io.EOF once the stream ends, after answering any final line that
// carried no newline. An EMPTY batch is answered for a read that completed no
// line, so a caller loops on the error rather than on the line count.
func (b *BatchReader) Next() ([]string, error) {
	for range batchEmptyReadMax {
		n, err := b.r.Read(b.buf)
		if n > 0 {
			return b.cut(b.buf[:n]), nil
		}
		if err == nil {
			continue
		}
		if !errors.Is(err, io.EOF) {
			return nil, err
		}
		// The stream ended. A trailing line with no newline is still a command
		// the script wrote, so it is answered before the EOF is.
		if len(b.tail) > 0 && !b.dropping {
			last := string(b.tail)
			b.tail = b.tail[:0]
			return []string{last}, nil
		}
		return nil, io.EOF
	}
	return nil, io.ErrNoProgress
}

// cut splits one read into complete lines, carrying the rest.
//
// The loop is bounded by len(chunk), which is at most batchReadSize.
func (b *BatchReader) cut(chunk []byte) []string {
	var lines []string
	for len(chunk) > 0 {
		nl := bytes.IndexByte(chunk, '\n')
		if nl < 0 {
			b.carry(chunk)
			return lines
		}

		b.carry(chunk[:nl])
		if b.dropping {
			// The refused line ends at this newline. Its bytes are gone and the
			// next line starts clean. The reset belongs HERE rather than at the
			// top of the loop, because carry can refuse the line in this very
			// iteration and this newline is then already the one that ends it.
			b.dropping = false
		} else {
			lines = append(lines, string(b.tail))
			b.tail = b.tail[:0]
		}
		chunk = chunk[nl+1:]
	}
	return lines
}

// carry appends to the partial line held between reads, and refuses one that
// passes batchLineMax.
func (b *BatchReader) carry(part []byte) {
	if b.dropping {
		return
	}
	if len(b.tail)+len(part) > batchLineMax {
		b.dropping = true
		b.tail = b.tail[:0]
		slog.Warn("exabgp-bridge line refused", "error", ErrLineTooLong, "bytes", batchLineMax)
		return
	}
	b.tail = append(b.tail, part...)
}

// BatchLine is one line a script wrote and what the translator made of it.
type BatchLine struct {
	// Text is the line as the script wrote it.
	Text string
	// Translation is what the translator answered for Text.
	Translation Translation
	// Err is the translator's refusal. Translation is zero when it is set.
	Err error
}

// BatchDispatch is one command of a netted batch, with the index of the line
// that produced it so the runner answers that line.
type BatchDispatch struct {
	// Line indexes the BatchLine slice Net was given.
	Line int
	// Command is the ze command to dispatch.
	Command Command
}

// Net cancels the commands one batch takes back, and orders what survives.
//
// Two rules, and each is ExaBGP's own.
//
// A withdrawal cancels an announce of the same route EARLIER in the batch, and
// an announce cancels nothing. Upstream states both directions where it queues:
// _del_from_rib_impl pops a queued announce of the same route index, and
// _update_rib says an announce does not cancel a pending withdrawal, so a
// withdraw followed by an announce is sent as both
// (src/exabgp/rib/outgoing.py, upstream exa-networks/exabgp). A batch of
// `withdraw X` then `announce X` therefore puts both on the wire, and a batch of
// `announce X` then `withdraw X` puts neither.
//
// Withdrawals dispatch before announces, whatever order the script wrote them
// in: "Generate Updates for pending withdraws before announces (preserves
// semantic ordering)" (same file). api-fast batch 2 is the recording of it.
//
// Every command that carries no route keeps its write order and dispatches
// after the routes. An End-of-RIB is one of these: it names no route, so it
// cancels nothing and nothing cancels it.
//
// The walk is linear in the number of commands, which one read bounds.
func Net(lines []BatchLine) []BatchDispatch {
	all := make([]BatchDispatch, 0, len(lines))
	for i := range lines {
		if !lines[i].dispatchable() {
			continue
		}
		for _, command := range lines[i].Translation.Commands {
			all = append(all, BatchDispatch{Line: i, Command: command})
		}
	}

	canceled := make([]bool, len(all))
	// announced holds, for each route this batch has announced and not yet had
	// withdrawn, the entries that announced it. A withdrawal empties its own
	// list, so a later announce of the same route survives an earlier
	// withdrawal rather than being canceled by it.
	announced := make(map[routeIdentity][]int)
	for i := range all {
		key := all[i].Command.Key
		if !key.Route() {
			continue
		}
		id := key.identity()
		if !key.Withdraw {
			announced[id] = append(announced[id], i)
			continue
		}
		for _, earlier := range announced[id] {
			canceled[earlier] = true
		}
		delete(announced, id)
	}

	netted := make([]BatchDispatch, 0, len(all))
	for _, wanted := range [3]func(RouteKey) bool{isWithdraw, isAnnounce, isNotRoute} {
		for i := range all {
			if canceled[i] || !wanted(all[i].Command.Key) {
				continue
			}
			netted = append(netted, all[i])
		}
	}
	return netted
}

func isWithdraw(k RouteKey) bool { return k.Route() && k.Withdraw }
func isAnnounce(k RouteKey) bool { return k.Route() && !k.Withdraw }
func isNotRoute(k RouteKey) bool { return !k.Route() }

// dispatchable reports whether this line carries commands for ze's dispatcher.
//
// A refused line, a line the bridge answers itself, and a line whose selector
// names no session each carry none. They are still ANSWERED, in write order, by
// the runner's own second pass.
func (l BatchLine) dispatchable() bool {
	if l.Err != nil {
		return false
	}
	if l.Translation.Local != LocalNone || l.Translation.Unmatched {
		return false
	}
	return !l.Translation.Nothing()
}

// BatchAnswer is the outcome of one line's dispatch.
//
// The zero value is the success a line reaches by dispatching without a
// refusal, which is what every line of an ordinary batch reaches.
type BatchAnswer struct {
	// Failed says ze's dispatcher refused a command of this line. Error carries
	// the reason it gave.
	Failed bool
	Error  string
}

// AnswerBatch writes one answer for each line of a batch, in the order the
// script wrote them.
//
// The order matters twice. A script blocks on `done` for each line it wrote, so
// a line answered out of turn answers the wrong write. And the ack-control
// words move the AckMode as they are read, so `disable-ack` in the middle of a
// batch must silence the lines that FOLLOW it and no others.
//
// Every answer here comes after the batch's flush, because `done` means the
// command is done and a route is not done until it is on the wire
// (docs/architecture/exabgp-bridge.md).
//
// answers MUST hold one entry for each line.
func AnswerBatch(w io.Writer, ack *AckMode, lines []BatchLine, answers []BatchAnswer) {
	for i := range lines {
		switch {
		case lines[i].Err != nil:
			// A line the translator refused never reached ze, and the bridge
			// has already named it. ExaBGP answers nothing for a line its own
			// parser refused, so neither does this.
		case lines[i].Translation.Local != LocalNone:
			ack.AnswerLocal(w, lines[i].Translation.Local)
		case !lines[i].Translation.Unmatched && lines[i].Translation.Nothing():
			// A blank line and a comment carry no command and owe no answer.
		case answers[i].Failed:
			ack.WriteError(w, answers[i].Error)
		default:
			ack.WriteAck(w)
		}
	}
}

// BatchSelectors answers the peer selectors a batch's route lines addressed,
// each once, in the order the batch first reached them.
//
// One flush per selector is what a batch owes: the flush drains a peer's
// forward pool, so a batch that announced to two neighbors and flushed one
// would ack the second before its routes reached the wire.
func BatchSelectors(lines []BatchLine, netted []BatchDispatch) []string {
	selectors := make([]string, 0, 2)
	for _, dispatch := range netted {
		line := lines[dispatch.Line]
		if !line.Translation.Route {
			continue
		}
		if slices.Contains(selectors, line.Translation.Selector) {
			continue
		}
		selectors = append(selectors, line.Translation.Selector)
	}
	return selectors
}
