// Design: docs/guide/config-editor.md -- the commit subcommands and the force modifier
// Related: contract.go -- the commit result and conflict types every editor reads
//
// The commit grammar is declared here once, because three editors parse it:
// the SSH and file-mode Model (internal/component/cli) and the web terminal
// (internal/component/web), and web does not import cli.

package contract

import (
	"errors"
	"fmt"
	"slices"
	"strconv"
	"strings"
)

// CommitAction is what a commit subcommand does. The zero value is no action,
// so a CommitRequest nobody parsed never runs a commit.
type CommitAction uint8

const (
	// CommitActionUnspecified is the zero value: no subcommand was parsed.
	CommitActionUnspecified CommitAction = iota
	// CommitNow applies the candidate.
	CommitNow
	// CommitConfirmed applies the candidate and reverts it unless accepted in time.
	CommitConfirmed
	// CommitAccept keeps a pending confirmed commit.
	CommitAccept
	// CommitAbort reverts a pending confirmed commit at once.
	CommitAbort
	// CommitVerify validates the candidate and applies nothing.
	CommitVerify
)

// CommitForce is the trailing modifier that overrides validation warnings and
// conflicts on `commit now` and `commit confirmed <seconds>`. It never
// overrides an error, and it is an ordinary word on every other command.
const CommitForce = "force"

// The bounds of `commit confirmed <seconds>`, inclusive.
const (
	CommitConfirmedSecondsMin = 1
	CommitConfirmedSecondsMax = 3600
)

// CommitRequest is one parsed commit command. Seconds is set only for
// CommitConfirmed; Force only for CommitNow and CommitConfirmed.
type CommitRequest struct {
	Action  CommitAction
	Seconds int
	Force   bool
}

// CommitSubcommand describes one commit subcommand for parsing, help and
// completion.
type CommitSubcommand struct {
	Keyword      string
	Action       CommitAction
	Help         string
	TakesSeconds bool // the subcommand requires `<seconds>` after its keyword
	Forceable    bool // `force` may follow the subcommand
}

// Example returns the shortest argument list that runs the subcommand.
func (s CommitSubcommand) Example() []string {
	if s.TakesSeconds {
		return []string{s.Keyword, strconv.Itoa(CommitConfirmedSecondsMax)}
	}
	return []string{s.Keyword}
}

// usage is how the subcommand is typed, `<seconds>` included.
func (s CommitSubcommand) usage() string {
	if s.TakesSeconds {
		return "commit " + s.Keyword + " <seconds>"
	}
	return "commit " + s.Keyword
}

// commitSubcommands is the grammar, in the order help and completion show it.
var commitSubcommands = []CommitSubcommand{
	{Keyword: "now", Action: CommitNow, Help: "Apply the candidate", Forceable: true},
	{Keyword: "confirmed", Action: CommitConfirmed, Help: "Apply, and revert unless accepted within <seconds>", TakesSeconds: true, Forceable: true},
	{Keyword: "accept", Action: CommitAccept, Help: "Keep a pending confirmed commit"},
	{Keyword: "abort", Action: CommitAbort, Help: "Revert a pending confirmed commit now"},
	{Keyword: "verify", Action: CommitVerify, Help: "Validate the candidate, apply nothing"},
}

// CommitSubcommands returns the commit grammar in display order. The caller
// owns the returned slice.
func CommitSubcommands() []CommitSubcommand {
	return slices.Clone(commitSubcommands)
}

// commitUsage lists every subcommand and where force goes, derived from the
// table so a refusal never names a subcommand the parser lacks.
var commitUsage = buildCommitUsage()

func buildCommitUsage() string {
	all := make([]string, 0, len(commitSubcommands))
	forceable := make([]string, 0, len(commitSubcommands))
	for _, sub := range commitSubcommands {
		all = append(all, sub.usage())
		if sub.Forceable {
			forceable = append(forceable, sub.usage())
		}
	}
	var b strings.Builder
	b.WriteString(strings.Join(all[:len(all)-1], ", "))
	b.WriteString(" or ")
	b.WriteString(all[len(all)-1])
	b.WriteString("; force follows ")
	b.WriteString(strings.Join(forceable, " or "))
	b.WriteString(" to override warnings and conflicts")
	return b.String()
}

// ParseCommit parses the arguments after the `commit` verb. Every refusal says
// what to type instead.
func ParseCommit(args []string) (CommitRequest, error) {
	if len(args) == 0 {
		return CommitRequest{}, errors.New("commit needs a subcommand: " + commitUsage)
	}
	if args[0] == CommitForce {
		return CommitRequest{}, errors.New("force is a modifier, not a subcommand: " + commitUsage)
	}
	idx := slices.IndexFunc(commitSubcommands, func(s CommitSubcommand) bool { return s.Keyword == args[0] })
	if idx < 0 {
		return CommitRequest{}, fmt.Errorf("unknown commit subcommand %q: use %s", args[0], commitUsage)
	}
	sub := commitSubcommands[idx]
	req := CommitRequest{Action: sub.Action}
	rest := args[1:]

	if sub.TakesSeconds {
		seconds, err := parseCommitSeconds(sub, rest)
		if err != nil {
			return CommitRequest{}, err
		}
		req.Seconds = seconds
		rest = rest[1:]
	}

	if len(rest) == 0 {
		return req, nil
	}
	if !sub.Forceable {
		if rest[0] == CommitForce {
			return CommitRequest{}, errors.New(sub.usage() + " takes no force: force overrides validation warnings or a conflict, and " + sub.usage() + " has neither")
		}
		return CommitRequest{}, fmt.Errorf("%s takes no arguments, got %q", sub.usage(), rest[0])
	}
	if rest[0] != CommitForce || len(rest) > 1 {
		extra := rest[0]
		if rest[0] == CommitForce {
			extra = rest[1]
		}
		return CommitRequest{}, fmt.Errorf("unexpected %q after %s: only force may follow it", extra, sub.usage())
	}
	req.Force = true
	return req, nil
}

// parseCommitSeconds reads the required `<seconds>` of a subcommand that takes
// one, and checks the bounds.
func parseCommitSeconds(sub CommitSubcommand, rest []string) (int, error) {
	if len(rest) == 0 {
		return 0, fmt.Errorf("%s needs the seconds, from %d to %d", sub.usage(), CommitConfirmedSecondsMin, CommitConfirmedSecondsMax)
	}
	seconds, err := strconv.Atoi(rest[0])
	if err != nil {
		return 0, fmt.Errorf("invalid seconds %q: %s takes a whole number from %d to %d", rest[0], sub.usage(), CommitConfirmedSecondsMin, CommitConfirmedSecondsMax)
	}
	if seconds < CommitConfirmedSecondsMin {
		return 0, fmt.Errorf("the time must be at least %d second", CommitConfirmedSecondsMin)
	}
	if seconds > CommitConfirmedSecondsMax {
		return 0, fmt.Errorf("the time must be at most %d seconds (1 hour)", CommitConfirmedSecondsMax)
	}
	return seconds, nil
}

// The answers every editor gives to a commit subcommand. They are declared
// here once, beside the grammar, and built from its keywords, so the SSH, the
// file-mode and the web editors cannot disagree (AC-29) and a renamed
// subcommand renames every message that names it.
const (
	// CommitAccepted answers `commit accept`.
	CommitAccepted = "Commit accepted: the confirmed configuration is saved permanently."
	// CommitAborted answers `commit abort`.
	CommitAborted = "Changes rolled back to previous configuration."
	// CommitTimedOut reports a window its deadline reverted.
	CommitTimedOut = "Timeout: configuration automatically rolled back."
	// CommitNothingPending answers `commit now` with nothing to apply.
	CommitNothingPending = "no changes to commit"
	// commitNothingPendingNoWindow answers `commit confirmed <seconds>` with
	// nothing to apply: no window opens, because it would revert nothing.
	commitNothingPendingNoWindow = CommitNothingPending + ": no confirmed commit was opened"
)

// CommitCommand is how the subcommand for action is typed, `<seconds>`
// included, with no force: `commit confirmed <seconds>`.
func CommitCommand(action CommitAction) string {
	return commitSubcommandFor(action).usage()
}

// CommitAcceptOrAbort names the two subcommands that end an open window.
func CommitAcceptOrAbort() string {
	return "Use '" + CommitCommand(CommitAccept) + "' or '" + CommitCommand(CommitAbort) + "'."
}

// ConfirmWithin is the window owner's countdown line.
func ConfirmWithin(seconds int64) string {
	return "Confirm within " + strconv.FormatInt(seconds, 10) + "s or auto-revert. " + CommitAcceptOrAbort()
}

// ForcedCommand is the command that commits req over validation warnings and
// conflicts: the form a refusal names, so `commit confirmed` never points at
// `commit now`.
func ForcedCommand(req CommitRequest) string {
	if req.Action == CommitConfirmed {
		return "commit " + commitSubcommandFor(CommitConfirmed).Keyword + " " + strconv.Itoa(req.Seconds) + " " + CommitForce
	}
	return CommitCommand(CommitNow) + " " + CommitForce
}

// NothingToCommit answers req when the user has nothing pending: the commit
// applies nothing and no window opens.
func NothingToCommit(req CommitRequest) string {
	if req.Action == CommitConfirmed {
		return commitNothingPendingNoWindow
	}
	return CommitNothingPending
}

// commitSubcommandFor is the subcommand for action. Every action but
// CommitActionUnspecified has one, so a miss is a defect in the table.
func commitSubcommandFor(action CommitAction) CommitSubcommand {
	idx := slices.IndexFunc(commitSubcommands, func(s CommitSubcommand) bool { return s.Action == action })
	if idx < 0 {
		panic("BUG: commit action with no subcommand")
	}
	return commitSubcommands[idx]
}
