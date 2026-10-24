// Design: docs/architecture/core-design.md -- one structured commit workflow command
package commit

import (
	"errors"
	"fmt"
	"strings"

	"github.com/ze-software/ze/internal/core/textbuf"
	leaction "github.com/ze-software/ze/internal/le/le/action"
	lepath "github.com/ze-software/ze/internal/le/le/path"
	"github.com/ze-software/ze/internal/le/rfc"
	testweakened "github.com/ze-software/ze/internal/le/test/weakened"
)

const area = "commit"

// keywordPush is the push verb. It is the one push route: create carries no
// push keyword.
const keywordPush = "push"

// keywordDebtStatus is the verb that answers the derived verification debt.
const keywordDebtStatus = "debt-status"

// The create keywords that admit a commit the gates would otherwise refuse.
// Each one's reason is the operator's, and the admission it grants is
// unchanged by the derived debt.
const (
	keywordSession         = "session"
	actionReviewCheck      = "review-check"
	keywordStructuralRedOK = "structural-red-ok"
	keywordReviewOverride  = "review-override"
	keywordBrokenHeadFix   = "broken-head-fix"
	keywordRFCChangeOK     = "rfc-change-ok"
	// keywordAttest prepares the owner's attestation: an empty commit whose
	// only content is its Owner-attested: trailer (D-7).
	keywordAttest = "attest"
	// keywordLedgerCarried names one person-act row the ledger-deletion
	// commit carries as a Ledger-carried: trailer (D-4).
	keywordLedgerCarried = "ledger-carried"
)

// The commit message trailers that record a person act or an admission. Each
// key is the line's prefix, its value follows. The derivation (owed.go) reads
// the first four back; the last two are an audit record only.
const (
	// trailerReviewOwed is `Review-owed: <spec stem>: <reason>`.
	trailerReviewOwed = "Review-owed: "
	// trailerRFCApprovalOwed is `RFC-approval-owed: <package>.<Test>: <reason>`.
	trailerRFCApprovalOwed = "RFC-approval-owed: "
	// trailerOwnerAttested is `Owner-attested: <stable patch-id>: <owner words>`.
	trailerOwnerAttested = "Owner-attested: "
	// trailerLedgerCarried is `Ledger-carried: <sha> [<stable patch-id>] <gate key>: <reason>`.
	trailerLedgerCarried = "Ledger-carried: "
	// trailerStructuralRedOK is `Structural-red-ok: <reason>`.
	trailerStructuralRedOK = "Structural-red-ok: "
	// trailerBrokenHeadFix is `Broken-head-fix: <reason>`.
	trailerBrokenHeadFix = "Broken-head-fix: "
)

// declaredTrailers is every trailer create writes itself, with the one route
// that writes it. A subject or body line that starts with a key is refused
// (refuseHandWrittenTrailer), because the derivation would read it as the
// record: a hand-written `Owner-attested:` would answer an obligation the
// owner never answered.
var declaredTrailers = []struct {
	Key       string
	WrittenBy string
}{
	{trailerReviewOwed, `create ... ` + keywordReviewOverride + ` "<reason>"`},
	{trailerRFCApprovalOwed, `create ... ` + keywordRFCChangeOK + ` "<reason>"`},
	{trailerOwnerAttested, `create ... ` + keywordAttest + ` <commit> "<owner words>"`},
	{trailerLedgerCarried, `the ledger-deletion commit: create ... remove <path> ` + keywordLedgerCarried + ` "<sha> <gate key>: <reason>"`},
	{trailerStructuralRedOK, `create ... ` + keywordStructuralRedOK + ` "<reason>"`},
	{trailerBrokenHeadFix, `create ... ` + keywordBrokenHeadFix + ` "<reason>"`},
	{rfc.ApprovalTrailer, "create from ./le rfc approve"},
	{testweakened.ApprovalTrailer, "create from ./le test weakened approve"},
}

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
	{keywordPush, true, "derive what the verified commit P owes and prepare a script that re-derives against P and pushes P to the upstream branch; takes the owner's authorisation"},
	{keywordDebtStatus, false, "derive what a push of the verified commit P, or of commit <rev>, would still owe: exit 0 when nothing, 1 when anything, 2 when it cannot answer"},
}

// retiredVerbs maps each verb the derived debt removed to the command that
// replaced it. It feeds only the refusal: none of these verbs dispatches.
var retiredVerbs = map[string]string{
	"debt-list":      "./le commit " + keywordDebtStatus,
	"debt-clear":     verifyWorktreeCommand,
	"debt-discharge": `./le commit create ... attest <commit> "<owner words>"`,
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
		message, err := Message(values.one("subject"), values["body"], nil)
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
	case keywordPush:
		if len(args) != 2 {
			return commandError(errors.New(`push takes the owner's authorisation, as in: ./le commit push "<owner authorisation>"`), 2)
		}
		prepared, code, err := preparePush(root, args[1])
		if err != nil {
			return commandError(err, code)
		}
		return &prepared, 0
	case keywordDebtStatus:
		values, err := parseKeywords(args[1:], map[string]keywordRule{"commit": {Value: true}})
		if err != nil {
			return commandError(fmt.Errorf("%w; the form is ./le commit debt-status [commit <rev>]", err), 2)
		}
		// The derivation answers or errors; an error is "cannot answer", exit
		// 2, and never reads as nothing owed.
		status, code, err := answerDebtStatus(root, values.one("commit"))
		if err != nil {
			return commandError(err, code)
		}
		return &status, code
	default:
		if replacement, retired := retiredVerbs[args[0]]; retired {
			return commandError(fmt.Errorf("commit verb %q was removed: verification debt is derived from git and the green record; run %s instead", args[0], replacement), 2)
		}
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
	// Replacement is set for a keyword that was removed: it names the command
	// that replaced it, and the keyword is refused naming it.
	Replacement string
	// Pair is set for a keyword that takes two values; Usage is its form,
	// named when a value is missing.
	Pair  bool
	Usage string
	// Given reports whether Options carries the keyword, for a refusal that
	// judges Options rather than argv: Create is callable as Go. Every create
	// keyword but a removed one carries it (TestEveryCreateKeywordReadsOptions),
	// and refuseUndeclaredWithAttest refuses an attestation outright while a
	// live keyword lacks it.
	Given func(*Options) bool
	// WithAttest declares that an attestation may carry the keyword. Its zero
	// value refuses it, so a keyword added without a decision is refused by
	// refuseAttestAdmission rather than dropped in silence.
	WithAttest bool
}

// createKeywords is the closed keyword table of `le commit create`: what each
// keyword takes, how Options holds it, and whether an attestation admits it.
var createKeywords = map[string]keywordRule{
	keywordSession: {Value: true, Given: givenText(func(o *Options) string { return o.Session }), WithAttest: true},
	"tag":          {Value: true, Given: givenText(func(o *Options) string { return o.Tag }), WithAttest: true},
	"subject":      {Value: true, Given: givenText(func(o *Options) string { return o.Subject }), WithAttest: true},
	"body":         {Value: true, Repeat: true, Given: func(o *Options) bool { return len(o.Body) != 0 }, WithAttest: true},
	"file":         {Value: true, Repeat: true, Given: givenFiles},
	"file-list":    {Value: true, Repeat: true, Given: givenFiles},
	keywordHunks: {Pair: true, Repeat: true, Usage: keywordHunks + " <path> <patch-file>",
		Given: func(o *Options) bool { return len(o.Hunks) != 0 }},
	"remove":               {Value: true, Repeat: true, Given: givenRemove},
	"remove-list":          {Value: true, Repeat: true, Given: givenRemove},
	"script":               {Value: true, Given: givenText(func(o *Options) string { return o.Script }), WithAttest: true},
	"append":               {Given: func(o *Options) bool { return o.Append }, WithAttest: true},
	"replace":              {Given: func(o *Options) bool { return o.Replace }, WithAttest: true},
	"no-test":              {Value: true, Given: givenText(func(o *Options) string { return o.NoTest })},
	keywordStructuralRedOK: {Value: true, Given: givenText(func(o *Options) string { return o.StructuralRedOK })},
	keywordReviewOverride:  {Value: true, Given: givenText(func(o *Options) string { return o.ReviewOverride })},
	keywordBrokenHeadFix:   {Value: true, Given: givenText(func(o *Options) string { return o.BrokenHeadFix })},
	keywordRFCChangeOK:     {Value: true, Given: givenText(func(o *Options) string { return o.RFCChangeOK })},
	"dry-run":              {Given: func(o *Options) bool { return o.DryRun }, WithAttest: true},
	keywordAttest: {Pair: true, Usage: keywordAttest + ` <commit> "<owner words>"`,
		Given: givenText(func(o *Options) string { return o.Attest }), WithAttest: true},
	keywordLedgerCarried: {Value: true, Repeat: true,
		Given: func(o *Options) bool { return len(o.LedgerCarried) != 0 }},
	keywordPush:              {Replacement: `./le commit push "<owner authorisation>"`},
	"unverified":             {Replacement: "./le verify worktree"},
	"missing-full-verify-ok": {Replacement: "./le verify worktree"},
}

// givenText answers a Given for a keyword whose value is one string: blank is
// not given, the reading Create applies to a reason.
func givenText(value func(*Options) string) func(*Options) bool {
	return func(o *Options) bool { return strings.TrimSpace(value(o)) != "" }
}

// givenFiles and givenRemove read the two path keywords; a list expands into
// the same field as the keyword it lists.
func givenFiles(o *Options) bool  { return len(o.Files) != 0 }
func givenRemove(o *Options) bool { return len(o.Remove) != 0 }

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
		if rule.Replacement != "" {
			return nil, fmt.Errorf("keyword %q was removed: verification debt is derived from git and the green record; run %s instead", keyword, rule.Replacement)
		}
		if !rule.Repeat && values.has(keyword) {
			return nil, fmt.Errorf("keyword %q appears more than once", keyword)
		}
		if rule.Pair {
			if index+2 >= len(args) {
				return nil, fmt.Errorf("keyword %q requires two values: %s", keyword, rule.Usage)
			}
			values[keyword] = append(values[keyword], args[index+1], args[index+2])
			index += 2
			continue
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
	values, err := parseKeywords(args, createKeywords)
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
	attest := pairValues(values[keywordAttest])
	return Options{
		Session: values.one(keywordSession), Tag: values.one("tag"), Subject: values.one("subject"),
		Body: values["body"], Files: files, Hunks: hunkSelections(values[keywordHunks]), Remove: removed, Script: values.one("script"),
		Append: values.has("append"), Replace: values.has("replace"),
		NoTest:          values.one("no-test"),
		StructuralRedOK: values.one(keywordStructuralRedOK),
		ReviewOverride:  values.one(keywordReviewOverride), BrokenHeadFix: values.one(keywordBrokenHeadFix),
		RFCChangeOK: values.one(keywordRFCChangeOK), DryRun: values.has("dry-run"),
		Attest: attest.one(0), AttestWords: attest.one(1), LedgerCarried: values[keywordLedgerCarried],
	}, nil
}

// pairValues is the two values of a Pair keyword, or none when it was not given.
type pairValues []string

func (p pairValues) one(index int) string {
	if index >= len(p) {
		return ""
	}
	return p[index]
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
	if p.NoTest != "" {
		text.Str("no-test=").Str(p.NoTest).Byte('\n')
	}
	if p.Debt != "" {
		text.Str("debt=").Str(p.Debt).Byte('\n')
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
