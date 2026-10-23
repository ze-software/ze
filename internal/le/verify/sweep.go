// Design: docs/architecture/testing/verify-freshness-scope.md -- every piece of one cut in one invocation
// Related: green.go -- the parts record the sweep consults and each piece's run writes
// Related: lifecycle.go -- the run each piece is
package verify

import (
	"context"
	"errors"
	"slices"
	"strings"

	"github.com/ze-software/ze/internal/core/textbuf"
	verifyengine "github.com/ze-software/ze/internal/le/verify/engine"
)

// SweepReport is the verdict of `le verify worktree all of <m>`: which pieces
// it ran, which it skipped as already proven, which were red, and whether the
// green record now covers the commit.
type SweepReport struct {
	Action      string   `json:"action"`
	Commit      string   `json:"commit"`
	Of          int      `json:"of"`
	Code        int      `json:"code"`
	Skipped     []int    `json:"skipped"`
	Ran         []int    `json:"ran"`
	Failed      []int    `json:"failed"`
	Proven      []int    `json:"proven"`
	Green       bool     `json:"green"`
	Diagnostics []string `json:"diagnostics"`
}

// Sweep runs every piece of the cut options.Parts names over one commit, each
// piece a Run of its own worktree, so a sweep killed mid-flight costs the piece
// it was running and keeps the ones behind it.
//
// The commit is resolved ONCE (options.Commit, default HEAD), so every piece
// judges the same SHA even when HEAD moves during the sweep: pieces of two
// commits never add up. A piece the parts record already holds for that commit
// and cut is skipped, and a commit the green record already covers runs
// nothing. hostReady runs for every piece the sweep will run before the first
// starts, so a host that cannot run the last piece starts none. A red piece
// does not end the sweep, because the pieces are independent; the exit is then
// the first red piece's.
//
// options.Part MUST be zero and options.Parts at least one; first-red is
// refused, because a piece counts only when every stage of it ran.
func Sweep(ctx context.Context, root string, options Options, actions verifyengine.ActionRunner,
	hostReady func(Options) error) (report SweepReport) {
	report = SweepReport{Action: actionName, Of: options.Parts, Code: 1, Skipped: []int{}, Ran: []int{},
		Failed: []int{}, Proven: []int{}, Diagnostics: []string{}}
	var text textbuf.Buffer
	// The verdict line is written last, after every branch that moves Code.
	defer func() {
		report.Diagnostics = append(report.Diagnostics, text.Reset().Str("verify-worktree: all of ").
			Int(int64(report.Of)).Str(" over ").Str(shortSHA(report.Commit)).
			Str(": ran ").Str(pieceList(report.Ran)).Str(", skipped ").Str(pieceList(report.Skipped)).
			Str(", red ").Str(pieceList(report.Failed)).Str(" exit=").Int(int64(report.Code)).String())
	}()

	if err := sweepValid(options); err != nil {
		report.Code = 2
		report.Diagnostics = append(report.Diagnostics, text.Str("verify-worktree: ").Err(err).String())
		return report
	}
	revision := strings.TrimSpace(options.Commit)
	if revision == "" {
		revision = "HEAD"
	}
	sha, err := resolveCommit(ctx, root, revision, runGit)
	if err != nil {
		report.Diagnostics = append(report.Diagnostics, text.Reset().Str("verify-worktree: ").Err(err).String())
		return report
	}
	report.Commit = sha

	every := make([]int, 0, options.Parts)
	for index := 1; index <= options.Parts; index++ {
		every = append(every, index)
	}
	if record, readErr := ReadGreenRecord(root); readErr == nil && record.Commit() == sha {
		report.Skipped, report.Proven, report.Green, report.Code = every, every, true, 0
		report.Diagnostics = append(report.Diagnostics, text.Reset().Str("verify-worktree: ").
			Str(shortSHA(sha)).Str(" already holds the green record; no piece runs").String())
		return report
	}
	already := provenParts(root, sha, options.Parts)
	pending := make([]int, 0, options.Parts)
	for _, index := range every {
		if slices.Contains(already, index) {
			report.Skipped = append(report.Skipped, index)
			continue
		}
		pending = append(pending, index)
	}
	for _, index := range pending {
		if err := hostReady(pieceOptions(options, sha, index)); err != nil {
			report.Code = 2
			report.Diagnostics = append(report.Diagnostics, text.Reset().Str("verify-worktree: piece ").
				Int(int64(index)).Str(" of ").Int(int64(options.Parts)).Str(" cannot run here, so no piece starts: ").
				Err(err).String())
			return report
		}
	}

	report.Proven = append(report.Proven, already...)
	report.Code = 0
	for _, index := range pending {
		if ctx.Err() != nil {
			report.Code = verifyengine.Unjudged
			report.Diagnostics = append(report.Diagnostics, text.Reset().
				Str("verify-worktree: sweep interrupted before piece ").Int(int64(index)).String())
			break
		}
		piece := Run(ctx, root, pieceOptions(options, sha, index), actions)
		report.Ran = append(report.Ran, index)
		report.Diagnostics = append(report.Diagnostics, piece.Diagnostics...)
		if piece.Code != 0 {
			report.Failed = append(report.Failed, index)
			if report.Code == 0 {
				report.Code = piece.Code
			}
			continue
		}
		if len(piece.Proven) != 0 {
			report.Proven = piece.Proven
		}
		if piece.Green {
			report.Green = true
			report.Proven = every
		}
	}
	return report
}

// sweepValid refuses a sweep the grammar cannot have produced.
func sweepValid(options Options) error {
	if options.Part != 0 {
		return errors.New(sweepWithPart)
	}
	if options.Parts < 1 {
		return errors.New(sweepWithoutCount)
	}
	if options.FirstRed {
		return errors.New(firstRedWithPart)
	}
	return nil
}

// sweepWithPart and sweepWithoutCount are the refusals of a malformed sweep.
const (
	sweepWithPart = "all and part name different runs: le verify worktree all of <m> runs every piece, " +
		"le verify worktree part <i> of <m> runs one"
	sweepWithoutCount = "all needs the piece count: le verify worktree all of <m>"
)

// pieceOptions answers the run of one piece of the sweep, pinned to sha.
func pieceOptions(options Options, sha string, index int) Options {
	return Options{Commit: sha, Keep: options.Keep, Part: index, Parts: options.Parts}
}

// pieceList renders piece indices as "[1 2 3]".
func pieceList(pieces []int) string {
	var text textbuf.Buffer
	text.Byte('[')
	for position, piece := range pieces {
		if position != 0 {
			text.Byte(' ')
		}
		text.Int(int64(piece))
	}
	return text.Byte(']').String()
}

// Text renders the sweep's diagnostics in the order the pieces produced them,
// ending with the verdict line.
func (r SweepReport) Text() string {
	if len(r.Diagnostics) == 0 {
		return ""
	}
	var text textbuf.Buffer
	return text.Join(r.Diagnostics, "\n").Byte('\n').String()
}
