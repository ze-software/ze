// Design: docs/architecture/exabgp-bridge.md -- one write is one batch, and a group is applied at its end
// Related: bridge_batch.go -- Net, AnswerBatch, BatchSelectors
// Related: bridge_group.go -- Group, the buffer each script's lines go through

package bridge

import (
	"context"
	"io"
	"log/slog"
	"strings"

	"github.com/ze-software/ze/internal/core/textbuf"
)

// LineRunner is what a runner does with the commands one script's batch
// produced. Every runner reaches ze differently (the plugin dispatcher, or
// MuxConn requests on stdout), and that is all that differs: the reading of the
// lines, the group buffer, the netting and the answers are ScriptLines.Batch,
// once for every runner.
type LineRunner interface {
	// ExitRunner is Dispatch, which hands ze one command and answers its
	// refusal, and Flush, which drains the forward pool of the peers a
	// selector names, so what was dispatched for them is on the wire before
	// the lines are answered.
	ExitRunner
	// Status performs a `status` line: the reply ExaBGP writes and the outcome.
	Status(ctx context.Context, asJSON bool) ([]byte, BatchAnswer)
	// Dispatched records one route command ze accepted, with the selector its
	// line addressed (the owner ledger an exit withdraws from).
	Dispatched(selector string, command Command)
}

// ScriptLines is one script's command side for one batch: the translator for
// its peers, the API version it is held to, its ack state and its group
// buffer. API, Ack and Group are the script's own and outlive the batch.
type ScriptLines struct {
	Name       string
	Log        *slog.Logger
	Translator Translator
	API        *ScriptVersion
	Ack        *AckMode
	Group      *Group
}

// Batch translates, nets, dispatches, flushes and answers the lines of one
// read.
//
// The order is fixed and each step earns its place. The netting is ExaBGP's
// own (Net). The flush comes BEFORE the answers, because `done` means the
// command is done and a route is not done until it is on the wire: acking
// first let a script send its NEXT command while the previous route's flush
// was still running. The answers come last, one per line, in the order the
// script wrote them.
//
// A group that closes inside the batch splits it: the lines before it are
// dispatched and answered, then the group's commands are, as one set, and
// then the lines after it. ExaBGP applies a group when it reads its end, after
// the lines written before it and before the lines written after.
func (s ScriptLines) Batch(ctx context.Context, r LineRunner, w io.Writer, lines []string) {
	batch := make([]BatchLine, 0, len(lines))
	answers := make([]BatchAnswer, 0, len(lines))
	for _, text := range lines {
		if strings.TrimSpace(text) == "" {
			batch = append(batch, BatchLine{Text: text})
			answers = append(answers, BatchAnswer{})
			continue
		}
		// The version is settled BEFORE the line is read, and the line is
		// then held to it, as ExaBGP's API.process calls detect_api_version
		// and then dispatch_for. The group comes between the two: a line an
		// open group holds is held whatever the version (API.process).
		version := s.API.Detect(text)
		admitted := version.Admits(text)
		asJSON := answersInJSON(version, text)
		read := s.Group.read(text, asJSON, admitted == nil)
		switch read.step {
		case groupStepAnswer:
			batch = append(batch, BatchLine{Text: text, Version: version, Translation: Translation{Local: LocalGroup}, Reply: read.reply})
			answers = append(answers, BatchAnswer{Failed: read.failed})
			continue
		case groupStepApply:
			s.answer(ctx, r, w, batch, answers)
			batch, answers = batch[:0], answers[:0]
			s.applyGroup(ctx, r, w, text, version, asJSON, read.commands)
			continue
		case groupStepPass:
		case groupStepUnspecified:
			panic("BUG: a group read answered no step")
		}

		if admitted != nil {
			s.Log.Warn("exabgp-bridge line refused", "script", s.Name, "error", admitted)
			batch = append(batch, BatchLine{Text: text, Version: version, Err: admitted})
			answers = append(answers, BatchAnswer{})
			continue
		}
		translation, err := s.Translator.Line(text)
		if err != nil {
			// The bridge names the line it refused rather than handing an
			// untranslated line to ze's dispatcher, where it would die as an
			// unknown command with no mention of the bridge.
			s.Log.Warn("exabgp-bridge line refused", "script", s.Name, "error", err)
		}
		line := BatchLine{Text: text, Version: version, Translation: translation, Err: err}
		var answer BatchAnswer
		if translation.Local == LocalAPIVersion {
			line.Reply, answer.Failed = s.API.Answer(translation.APIVersion)
		}
		if translation.Local == LocalStatus {
			line.Reply, answer = r.Status(ctx, StatusInJSON(version, text))
		}
		batch = append(batch, line)
		answers = append(answers, answer)
	}
	s.answer(ctx, r, w, batch, answers)
}

// answer dispatches, flushes and answers one run of lines.
func (s ScriptLines) answer(ctx context.Context, r LineRunner, w io.Writer, batch []BatchLine, answers []BatchAnswer) {
	if len(batch) == 0 {
		return
	}
	s.dispatch(ctx, r, batch, answers, false)
	// The script is waiting for these. An ExaBGP API client sends one command,
	// blocks for `done`, and gives up after two seconds, so a runner that
	// dispatches without acking delivers exactly one command per script.
	AnswerBatch(w, s.Ack, batch, answers)
}

// dispatch nets one run of lines, hands ze what survives, and flushes every
// selector the run reached. A refusal is recorded against its line.
//
// grouped is set for a closed group's commands. Route commands that differ in
// their one prefix alone are then sent as ONE command, so they leave in one
// UPDATE: that is what a group is for, and ExaBGP's RIB does it by grouping a
// group's routes by their attributes. A command whose NLRI is several words (a
// flow rule, an RD and label) is never merged, because two of them run
// together would read as one route.
func (s ScriptLines) dispatch(ctx context.Context, r LineRunner, batch []BatchLine, answers []BatchAnswer, grouped bool) {
	netted := Net(batch)
	sets := make([]*dispatchSet, 0, len(netted))
	heads := make(map[string]*dispatchSet)
	for _, dispatch := range netted {
		if answers[dispatch.Line].Failed {
			// This line already met a refusal. Its remaining commands describe
			// the same route, and the rest of the batch keeps going: one line's
			// failure never swallows another line's answer.
			continue
		}
		head, mergeable := mergeHead(dispatch.Command)
		if grouped && mergeable {
			if set, ok := heads[head]; ok {
				set.parts = append(set.parts, dispatch)
				continue
			}
		}
		set := &dispatchSet{head: head, parts: []BatchDispatch{dispatch}}
		sets = append(sets, set)
		if grouped && mergeable {
			heads[head] = set
		}
	}
	for _, set := range sets {
		text := set.text()
		if err := r.Dispatch(ctx, text); err != nil {
			s.Log.Warn("exabgp-bridge dispatch failed", "script", s.Name, "error", err, "cmd", text)
			for _, part := range set.parts {
				answers[part.Line] = BatchAnswer{Failed: true, Error: err.Error()}
			}
			continue
		}
		// Each route keeps its own owner entry, merged or not, so a later
		// withdrawal of one of them clears that one.
		for _, part := range set.parts {
			r.Dispatched(batch[part.Line].Translation.Selector, part.Command)
		}
	}
	// One per-peer flush for each selector the run reached, so the forward
	// pool drains before any line of it is answered. ExaBGP orders the two the
	// same way: announce_route awaits every peer's flush event and calls
	// answer_done after it (src/exabgp/reactor/api/command/announce.py).
	// A refused flush fails every route line of its selector, because the
	// script would otherwise read `done` for a barrier that did not hold.
	for _, selector := range BatchSelectors(batch, netted) {
		err := r.Flush(ctx, selector)
		if err == nil {
			continue
		}
		s.Log.Warn("exabgp-bridge flush failed", "script", s.Name, "error", err, "selector", selector)
		failSelectorLines(batch, netted, answers, selector, err)
	}
}

// failSelectorLines records err against every route line of the run that
// addressed selector and has no failure yet.
func failSelectorLines(batch []BatchLine, netted []BatchDispatch, answers []BatchAnswer, selector string, err error) {
	for _, dispatch := range netted {
		line := batch[dispatch.Line]
		if !line.Translation.Route {
			continue
		}
		if line.Translation.Selector != selector {
			continue
		}
		if answers[dispatch.Line].Failed {
			continue
		}
		answers[dispatch.Line] = BatchAnswer{Failed: true, Error: err.Error()}
	}
}

// dispatchSet is the commands sent to ze as one: a single command, or route
// commands that share everything but their one prefix.
type dispatchSet struct {
	head  string
	parts []BatchDispatch
}

// text is the command ze is handed: the one command as written, or the shared
// head followed by every part's prefix in the order they were written.
func (d *dispatchSet) text() string {
	if len(d.parts) == 1 {
		return d.parts[0].Command.Text
	}
	var tb textbuf.Buffer
	tb.Str(d.head)
	for _, part := range d.parts {
		tb.Byte(' ').Str(part.Command.Key.NLRI)
	}
	return tb.String()
}

// mergeHead answers a route command's text without its NLRI, when the NLRI is
// one word written last (oneCommandPerNLRI), and false for every other
// command.
func mergeHead(command Command) (string, bool) {
	if !command.Key.Route() {
		return "", false
	}
	if strings.Contains(command.Key.NLRI, " ") {
		return "", false
	}
	head, cut := strings.CutSuffix(command.Text, " "+command.Key.NLRI)
	return head, cut
}

// applyGroup applies a closed group's commands as one set, then answers the
// line that closed it with what the group applied (_process_group,
// _answer_group). The routes that could be applied are, whatever the others
// did.
func (s ScriptLines) applyGroup(ctx context.Context, r LineRunner, w io.Writer, text string, version APIVersion, asJSON bool, commands []groupCommand) {
	applied := s.Translator.group(commands, version)
	answers := make([]BatchAnswer, len(applied.lines))
	s.dispatch(ctx, r, applied.lines, answers, true)
	reply, failed := applied.answer(answers, asJSON)
	line := BatchLine{Text: text, Version: version, Translation: Translation{Local: LocalGroup}, Reply: reply}
	AnswerBatch(w, s.Ack, []BatchLine{line}, []BatchAnswer{{Failed: failed}})
}
