// Design: docs/architecture/core-design.md -- one structured commit workflow command
package commit

import (
	"context"
	"fmt"
	"slices"
	"strings"

	"github.com/ze-software/ze/internal/core/textbuf"
	"github.com/ze-software/ze/internal/le/leaction"
	"github.com/ze-software/ze/internal/le/lepath"
	"github.com/ze-software/ze/internal/le/testweakened"
	"github.com/ze-software/ze/internal/le/verify"
	verifydispatch "github.com/ze-software/ze/internal/le/verify/dispatch"
	verifyengine "github.com/ze-software/ze/internal/le/verify/engine"
)

const area = "commit"

var commandVerbs = []struct {
	Verb   string
	Writes bool
	Why    string
}{
	{keywordSession, true, "create or reuse this harness session's eight-hex commit namespace"},
	{"create", true, "validate an explicit file/remove population and generate its message and executable commit script"},
	{"message", false, "render and validate a commit message without preparing a script"},
	{"audit", false, "audit a branch range and worktree for unexplained test weakening"},
	{actionReviewCheck, false, "re-check a hash-pinned independent review immediately before staging"},
	{"debt-list", false, "list every verification-debt row"},
	{"debt-status", false, "summarize open and cleared verification debt"},
	{"debt-clear", true, "run owed native gates against HEAD and clear rows only after exit zero; part <n> of <m> runs one piece, all of <m> sweeps every piece in order, and nothing clears until every piece has passed at one commit"},
}

// CommandRow is one closed verb exposed by `le commit`.
type CommandRow struct {
	Verb   string `json:"verb"`
	Writes bool   `json:"writes"`
	Why    string `json:"why"`
}

// commandList is the structured bare-command answer.
type commandList struct {
	Area  string       `json:"area"`
	Verbs []CommandRow `json:"verbs"`
}

// debtStatus is the debt population summary.
type debtStatus struct {
	Open    int `json:"open"`
	Cleared int `json:"cleared"`
	Total   int `json:"total"`
}

// debtClearResult records the fixed commit judged and the rows changed. Part,
// Of and Proven are empty for an uncut pass, which proves the whole population
// in one run and has no progress to carry.
type debtClearResult struct {
	Commit      string   `json:"commit,omitempty"`
	Open        int      `json:"open"`
	Cleared     int      `json:"cleared"`
	Remaining   int      `json:"remaining"`
	Part        int      `json:"part,omitempty"`
	Of          int      `json:"of,omitempty"`
	Proven      []int    `json:"proven-parts,omitempty"`
	Skipped     []int    `json:"skipped-parts,omitempty"`
	Failed      []int    `json:"failed-parts,omitempty"`
	Runnable    []string `json:"runnable,omitempty"`
	Unrunnable  []string `json:"unrunnable,omitempty"`
	Diagnostics []string `json:"diagnostics,omitempty"`
}

// messageResult is a validated message preview.
type messageResult struct {
	Message string `json:"text"`
}

// sessionResult is the reusable commit session identity.
type sessionResult struct {
	Session string `json:"session"`
}

// Answer dispatches one closed commit-workflow verb.
func Answer(args []string) (any, int) {
	if len(args) == 0 {
		return listCommands(), 0
	}
	root, err := lepath.Root()
	if err != nil {
		leaction.ReportError(err)
		return nil, 2
	}
	switch args[0] {
	case keywordSession:
		values, err := parseKeywords(args[1:], map[string]keywordRule{keywordSession: {Value: true}})
		if err != nil {
			return commandError(err, 2)
		}
		session, err := lepath.CommitSession(root, values.one(keywordSession))
		if err != nil {
			return commandError(err, 2)
		}
		return sessionResult{Session: session}, 0
	case "create":
		options, err := parseCreate(args[1:])
		if err != nil {
			return commandError(err, 2)
		}
		prepared, err := Create(root, &options)
		if err != nil {
			return commandError(err, 2)
		}
		return &prepared, 0
	case "message":
		values, err := parseKeywords(args[1:], map[string]keywordRule{
			"subject": {Value: true}, "body": {Value: true, Repeat: true},
		})
		if err != nil {
			return commandError(err, 2)
		}
		message, err := Message(values.one("subject"), values["body"])
		if err != nil {
			return commandError(err, 2)
		}
		return messageResult{Message: message}, 0
	case "audit":
		values, err := parseKeywords(args[1:], map[string]keywordRule{"base": {Value: true}})
		if err != nil {
			return commandError(err, 2)
		}
		report := testweakened.Audit(testweakened.AuditRequest{Root: root, Base: values.one("base")})
		return report, report.ExitCode()
	case actionReviewCheck:
		values, err := parseKeywords(args[1:], map[string]keywordRule{
			"spec": {Value: true}, "file": {Value: true, Repeat: true},
		})
		if err != nil {
			return commandError(err, 2)
		}
		if values.one("spec") == "" {
			return commandError(fmt.Errorf("review-check requires spec <stem>"), 2)
		}
		review := CheckReview(root, values.one("spec"), values["file"])
		if !review.Clean {
			return review, 3
		}
		return review, 0
	case "debt-list":
		if len(args) != 1 {
			return commandError(fmt.Errorf("debt-list takes no arguments, got %q", args[1]), 2)
		}
		rows, err := ListDebt(root)
		if err != nil {
			return commandError(err, 2)
		}
		return rows, 0
	case "debt-status":
		if len(args) != 1 {
			return commandError(fmt.Errorf("debt-status takes no arguments, got %q", args[1]), 2)
		}
		rows, err := ListDebt(root)
		if err != nil {
			return commandError(err, 2)
		}
		status := debtStatus{Total: len(rows)}
		for _, row := range rows {
			if row.Status == statusOpen {
				status.Open++
			} else {
				status.Cleared++
			}
		}
		return status, 0
	case "debt-clear":
		values, err := parseKeywords(args[1:], map[string]keywordRule{
			"part": {Value: true}, "of": {Value: true}, "all": {Value: false},
		})
		if err != nil {
			return commandError(err, 2)
		}
		part, err := debtPartFrom(values)
		if err != nil {
			return commandError(err, 2)
		}
		result, code := clearDebt(root, part)
		return result, code
	default:
		return commandError(fmt.Errorf("commit has no verb %q", args[0]), 2)
	}
}

func listCommands() commandList {
	list := commandList{Area: area, Verbs: make([]CommandRow, 0, len(commandVerbs))}
	for _, verb := range commandVerbs {
		list.Verbs = append(list.Verbs, CommandRow{Verb: verb.Verb, Writes: verb.Writes, Why: verb.Why})
	}
	return list
}

func Subs() string {
	verbs := make([]string, len(commandVerbs))
	for index, verb := range commandVerbs {
		verbs[index] = verb.Verb
	}
	return strings.Join(verbs, " | ")
}

type keywordRule struct {
	Value  bool
	Repeat bool
}

type keywordValues map[string][]string

func (v keywordValues) one(keyword string) string {
	if len(v[keyword]) == 0 {
		return ""
	}
	return v[keyword][0]
}

func (v keywordValues) has(keyword string) bool {
	_, exists := v[keyword]
	return exists
}

func parseKeywords(args []string, rules map[string]keywordRule) (keywordValues, error) {
	values := make(keywordValues)
	for index := 0; index < len(args); index++ {
		keyword := args[index]
		rule, exists := rules[keyword]
		if !exists {
			return nil, fmt.Errorf("unknown keyword %q; every value follows a closed keyword", keyword)
		}
		if !rule.Repeat && values.has(keyword) {
			return nil, fmt.Errorf("keyword %q appears more than once", keyword)
		}
		value := ""
		if rule.Value {
			if index+1 >= len(args) {
				return nil, fmt.Errorf("keyword %q requires a value", keyword)
			}
			index++
			value = args[index]
		}
		values[keyword] = append(values[keyword], value)
	}
	return values, nil
}

func parseCreate(args []string) (Options, error) {
	rules := map[string]keywordRule{
		keywordSession: {Value: true}, "tag": {Value: true}, "subject": {Value: true},
		"body": {Value: true, Repeat: true}, "file": {Value: true, Repeat: true},
		"file-list": {Value: true, Repeat: true}, "remove-list": {Value: true, Repeat: true},
		"remove": {Value: true, Repeat: true}, "script": {Value: true},
		"append": {}, "replace": {}, "push": {Value: true}, "no-test": {Value: true},
		gateUnverified:      {Value: true},
		gateStructuralRedOK: {Value: true}, gateMissingFullVerifyOK: {Value: true},
		gateStaleIndexOK: {Value: true}, gateReviewOverride: {Value: true},
		gateBrokenHeadFix: {Value: true}, gateRFCChangeOK: {Value: true}, "dry-run": {},
	}
	values, err := parseKeywords(args, rules)
	if err != nil {
		return Options{}, err
	}
	// A list expands where the keywords are read, so Options stays one contract:
	// an explicit population, however the caller wrote it down.
	files, err := expandLists(values["file"], values["file-list"])
	if err != nil {
		return Options{}, err
	}
	removed, err := expandLists(values["remove"], values["remove-list"])
	if err != nil {
		return Options{}, err
	}
	return Options{
		Session: values.one(keywordSession), Tag: values.one("tag"), Subject: values.one("subject"),
		Body: values["body"], Files: files, Remove: removed, Script: values.one("script"),
		Append: values.has("append"), Replace: values.has("replace"), Push: values.one("push"),
		NoTest: values.one("no-test"), Unverified: values.one(gateUnverified),
		StructuralRedOK:     values.one(gateStructuralRedOK),
		MissingFullVerifyOK: values.one(gateMissingFullVerifyOK), StaleIndexOK: values.one(gateStaleIndexOK),
		ReviewOverride: values.one(gateReviewOverride), BrokenHeadFix: values.one(gateBrokenHeadFix),
		RFCChangeOK: values.one(gateRFCChangeOK), DryRun: values.has("dry-run"),
	}, nil
}

// clearDebt re-judges every open debt row against HEAD, through the native
// verification the rows name, running the piece of it the caller named.
func clearDebt(root string, part debtPart) (debtClearResult, int) {
	return clearDebtWith(root, part, verifydispatch.RunAction)
}

// clearDebtWith is clearDebt with the verification's action dispatcher named.
//
// The dispatcher is a parameter because the DECISION this function makes is
// "clear only what the gate passed", and proving that decision means running it
// once against a green verdict and once against a red one. Through the real
// dispatcher each of those runs is the hour-long verification, so the decision
// would be the one part of debt clearing no test ever reaches, in a function
// whose whole job is to refuse to trust an unverified claim.
func clearDebtWith(root string, part debtPart, runner verifyengine.ActionRunner) (debtClearResult, int) {
	rows, err := openDebt(root)
	if err != nil {
		leaction.ReportError(err)
		return debtClearResult{}, 2
	}
	result := debtClearResult{Open: len(rows)}
	if len(rows) == 0 {
		return result, 0
	}
	runnableSet := make(map[string]bool)
	unrunnableSet := make(map[string]bool)
	for _, row := range rows {
		if row.Gate == "independent critical review" ||
			row.Gate == "owner approval for an RFC-tagged test change" {
			unrunnableSet[row.Gate] = true
			continue
		}
		runnableSet[row.Gate] = true
	}
	for gate := range runnableSet {
		result.Runnable = append(result.Runnable, gate)
	}
	for gate := range unrunnableSet {
		result.Unrunnable = append(result.Unrunnable, gate)
	}
	slices.Sort(result.Runnable)
	slices.Sort(result.Unrunnable)
	passed := make(map[string]bool)
	proven := false
	if len(result.Runnable) != 0 {
		// A sweep skips what an earlier sweep already proved at this commit, so
		// a run killed at its fattest stage resumes rather than starting again.
		already := provenDebtParts(root, debtSweepCommit(root), part.Of)
		for _, piece := range part.sweep() {
			if part.All && slices.Contains(already, piece.Index) {
				result.Skipped = append(result.Skipped, piece.Index)
				result.Proven = already
				result.Of = piece.Of
				proven = debtPartsComplete(already, piece.Of)
				continue
			}
			report := verify.Run(context.Background(), root,
				verify.Options{Commit: "HEAD", Part: piece.Index, Parts: piece.Of}, runner)
			result.Commit = report.Commit
			result.Diagnostics = append(result.Diagnostics, report.Diagnostics...)
			if piece.cut() {
				result.Of = piece.Of
				if !part.All {
					result.Part = piece.Index
				}
			}
			if report.Code != 0 {
				result.Remaining = result.Open
				if !part.All {
					if report.Verify == nil {
						return result, 1
					}
					break
				}
				// A sweep carries on: the pieces are independent, so one red
				// says nothing about the ones behind it.
				result.Failed = append(result.Failed, piece.Index)
				continue
			}
			if !piece.cut() {
				proven = true
				break
			}
			// The piece exited 0, so what it proved is recorded before anything
			// else can end this process. The whole point of cutting the run is
			// that a piece killed mid-flight costs that piece and not the run.
			recorded, err := recordDebtPart(root, report.Commit, piece)
			if err != nil {
				leaction.ReportError(err)
				result.Remaining = result.Open
				return result, 2
			}
			result.Proven = recorded
			proven = debtPartsComplete(recorded, piece.Of)
		}
	}
	if proven {
		for _, gate := range result.Runnable {
			passed[gate] = true
		}
	}
	if !proven {
		result.Remaining = result.Open
		return result, 0
	}
	cleared, err := clearDebtRows(root, passed)
	if err != nil {
		leaction.ReportError(err)
		result.Remaining = result.Open
		return result, 2
	}
	if part.cut() {
		if err := forgetDebtParts(root); err != nil {
			leaction.ReportError(err)
		}
	}
	result.Cleared = cleared
	result.Remaining = result.Open - cleared
	return result, 0
}

func commandError(err error, code int) (any, int) {
	leaction.ReportError(err)
	return nil, code
}

// Text preserves the helper's copyable key=value output for a bare create.
func (p *Prepared) Text() string {
	var text textbuf.Buffer
	text.Str("session=").Str(p.Session).Str("\nmessage=").Str(p.Message).
		Str("\nscript=").Str(p.Script).Str("\nverify=").Str(strings.ToUpper(p.Verify.State)).
		Str(" (").Str(p.Verify.Detail).Str(")\n")
	if len(p.Debt) != 0 {
		text.Str("debt=").Int(int64(len(p.Debt))).Str(" row(s) -> ").Str(debtPath(p.Session)).Byte('\n')
	}
	if p.NoTest != "" {
		text.Str("no-test=").Str(p.NoTest).Byte('\n')
	}
	if p.Push {
		text.Str("push=AUTHORIZED\n")
	}
	// The staged paths and their size against HEAD, last so it sits directly
	// above the prompt. A file that reads bigger than the author expects is
	// another session's work riding along; see FileStat for why this prints.
	text.Str(StatText(p.Stats))
	if p.DryRun {
		text.Str("--- message ---\n").Str(p.MessageText).Str("--- script ---\n").Str(p.ScriptText)
	}
	return text.String()
}

func (s sessionResult) Text() string { return s.Session + "\n" }
func (m messageResult) Text() string { return m.Message }

func (l commandList) Text() string {
	var text textbuf.Buffer
	text.Str("commit:\n")
	for _, row := range l.Verbs {
		mark := "checks"
		if row.Writes {
			mark = "writes"
		}
		text.Str("  ").Str(row.Verb).Str("  ").Str(mark).Str("  ").Str(row.Why).Byte('\n')
	}
	return text.String()
}

func (s debtStatus) Text() string {
	var text textbuf.Buffer
	return text.Str("verification debt: ").Int(int64(s.Open)).Str(" open, ").
		Int(int64(s.Cleared)).Str(" cleared\n").String()
}

func (r debtClearResult) Text() string {
	var text textbuf.Buffer
	if r.Open == 0 {
		return "No open verification-debt rows.\n"
	}
	for _, gate := range r.Unrunnable {
		text.Str("UNRUNNABLE  ").Str(gate).Byte('\n')
	}
	for _, line := range r.Diagnostics {
		text.Str(line).Byte('\n')
	}
	if r.Of != 0 {
		if r.Part == 0 {
			text.Str("sweep of ").Int(int64(r.Of)).Str(" piece(s)")
		} else {
			text.Str("part ").Int(int64(r.Part)).Str(" of ").Int(int64(r.Of))
		}
		text.Str(" over ").Str(r.Commit).Str("; proven so far:")
		for _, piece := range r.Proven {
			text.Byte(' ').Int(int64(piece))
		}
		text.Str(" of ").Int(int64(r.Of)).Byte('\n')
		if len(r.Skipped) != 0 {
			text.Str("already proven, not re-run:")
			for _, piece := range r.Skipped {
				text.Byte(' ').Int(int64(piece))
			}
			text.Byte('\n')
		}
		if len(r.Failed) != 0 {
			text.Str("red this sweep, re-run them:")
			for _, piece := range r.Failed {
				text.Byte(' ').Int(int64(piece))
			}
			text.Byte('\n')
		}
	}
	return text.Str("cleared ").Int(int64(r.Cleared)).Str(" row(s), ").
		Int(int64(r.Remaining)).Str(" still open\n").String()
}
