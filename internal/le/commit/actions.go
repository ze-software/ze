// Design: docs/architecture/core-design.md -- one structured commit workflow command
package commit

import (
	"errors"
	"strconv"
	"strings"

	"github.com/ze-software/ze/internal/core/textbuf"
	leaction "github.com/ze-software/ze/internal/le/le/action"
	lepath "github.com/ze-software/ze/internal/le/le/path"
	"github.com/ze-software/ze/internal/le/rfc"
	testweakened "github.com/ze-software/ze/internal/le/test/weakened"
	"github.com/ze-software/ze/internal/le/verify"
)

const area = "commit"

// The keywords the table declares and the bodies read, each spelled once.
const (
	keywordSubject    = "subject"
	keywordBody       = "body"
	keywordFile       = "file"
	keywordFileList   = "file-list"
	keywordRemove     = "remove"
	keywordRemoveList = "remove-list"
	keywordScript     = "script"
	keywordTag        = "tag"
	keywordAppend     = "append"
	keywordReplace    = "replace"
	keywordNoTest     = "no-test"
	keywordDryRun     = "dry-run"
	keywordSpec       = "spec"
	keywordCommit     = "commit"
	// valuePath names the value slot of every keyword that takes one path.
	valuePath = "path"
	// valueAuthorisation is push's one positional value: the owner's words,
	// one quoted word read as typed.
	valueAuthorisation = "owner authorisation"
)

// keywordPush is the push verb. It is the one push route: create carries no
// push keyword.
const keywordPush = "push"

// keywordDebtStatus is the verb that answers the derived verification debt.
const keywordDebtStatus = "debt-status"

// The create keywords that admit a commit the gates would otherwise refuse.
// Each one's reason is the operator's, and the admission it grants is
// unchanged by the derived debt.
const (
	keywordSession        = "session"
	actionReviewCheck     = "check"
	keywordReviewOverride = "review-override"
	keywordRFCChangeOK    = "rfc-change-ok"
	// keywordAttest prepares the owner's attestation: an empty commit whose
	// only content is its Owner-attested: trailer (D-7).
	keywordAttest = "attest"
	// keywordLedgerCarried names one person-act row the ledger-deletion
	// commit carries as a Ledger-carried: trailer (D-4).
	keywordLedgerCarried = "ledger-carried"
)

// The commit message trailers that record a person act or an admission. Each
// key is the line's prefix, its value follows. The derivation (owed.go) reads
// each one back.
const (
	// trailerReviewOwed is `Review-owed: <spec stem>: <reason>`.
	trailerReviewOwed = "Review-owed: "
	// trailerRFCApprovalOwed is `RFC-approval-owed: <package>.<Test>: <reason>`.
	trailerRFCApprovalOwed = "RFC-approval-owed: "
	// trailerLinuxLimitOwed is `Linux-limit-owed: <RID>: <LL-id>`.
	trailerLinuxLimitOwed = "Linux-limit-owed: "
	// trailerOwnerAttested is `Owner-attested: <stable patch-id>: <owner words>`.
	trailerOwnerAttested = "Owner-attested: "
	// trailerLedgerCarried is `Ledger-carried: <sha> [<stable patch-id>] <gate key>: <reason>`.
	trailerLedgerCarried = "Ledger-carried: "
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
	{trailerLinuxLimitOwed, "create automatically from added or reworded Linux-limit annotations"},
	{trailerOwnerAttested, `create ... ` + keywordAttest + ` <commit> "<owner words>"`},
	{trailerLedgerCarried, `the ledger-deletion commit: create ... remove <path> ` + keywordLedgerCarried + ` "<sha> <gate key>: <reason>"`},
	{rfc.ApprovalTrailer, "create from ./le rfc approve"},
	{testweakened.ApprovalTrailer, "create from ./le test weakened approve"},
}

// retiredVerbs maps each verb the derived debt removed to the command that
// replaced it. It feeds only the refusal: none of these verbs dispatches.
var retiredVerbs = map[string]string{
	"debt-list":      "./le commit " + keywordDebtStatus,
	"debt-clear":     scopedCommand,
	"debt-discharge": `./le commit create ... attest <commit> "<owner words>"`,
}

// messageResult is a validated message preview.
type messageResult struct {
	Message string `json:"text"`
}

// sessionResult is the reusable commit session identity.
type sessionResult struct {
	Session string `json:"session"`
}

// Answer dispatches one commit-workflow line through the action table. A verb
// the derived debt removed is refused naming its replacement first, because the
// table holds only live verbs and would answer that it has no such action.
func Answer(args []string) (any, int) {
	if len(args) != 0 {
		if replacement, retired := retiredVerbs[args[0]]; retired {
			leaction.ReportError(errors.New("commit verb " + strconv.Quote(args[0]) +
				" was removed: verification debt is derived from git and the green record; run " +
				replacement + " instead"))
			return nil, 2
		}
	}
	return table().Answer(args)
}

// Actions answers the commit table's listing, which help, the manifest and
// completion render.
func Actions() leaction.List { return table().Actions() }

// Subs answers the verbs the table holds, for the registry's help line.
func Subs() string { return table().Subs() }

// table is commit's whole command surface. Each action's keywords are declared
// here and parsed by the table; the bodies read what the table parsed.
func table() leaction.Area {
	return leaction.New(area,
		leaction.Action{
			Verb: keywordSession, Writes: true,
			Does:       "ensure",
			Summary:    "create or reuse this session's commit namespace",
			Why:        "create or reuse this harness session's eight-hex commit namespace",
			Parameters: []leaction.Parameter{sessionParameter},
			AnswerArgs: withRoot(answerSession),
		},
		leaction.Action{
			Verb: "create", Writes: true,
			Summary:    "validate a file list and write its commit script",
			Why:        "validate an explicit file/remove population and generate its message and executable commit script",
			Parameters: createParameters,
			AnswerArgs: withRoot(answerCreate),
		},
		leaction.Action{
			Verb:       "message",
			Does:       "preview",
			Summary:    "render and validate a commit message only",
			Why:        "render and validate a commit message without preparing a script",
			Parameters: []leaction.Parameter{subjectParameter, bodyParameter},
			AnswerArgs: withRoot(answerMessage),
		},
		leaction.Action{
			Verb:    actionReviewCheck,
			Summary: "re-check a pinned review just before staging",
			Why:     "re-check a hash-pinned independent review immediately before staging",
			Parameters: []leaction.Parameter{
				{Keyword: keywordSpec, Kind: leaction.KindLabel, Value: "stem", Requirement: leaction.Required,
					Help: "the stem of the spec whose recorded review is checked"},
				{Keyword: keywordFile, Kind: leaction.KindPath, Value: valuePath, Requirement: leaction.Optional, Repeat: true,
					Help: "one file the review must cover, as the commit will stage it"},
			},
			AnswerArgs: withRoot(answerReviewCheck),
		},
		leaction.Action{
			Verb: keywordPush, Writes: true,
			Summary: "prepare the script that pushes a verified commit",
			Why: "derive what the verified commit P owes and prepare a script that re-derives against P and pushes P " +
				"to the upstream branch; takes the owner's authorisation",
			// The owner's words are the whole line, as typed: an argv, so words
			// that begin with a dash or read like a keyword are still his.
			// answerPush takes exactly one, the quoted form.
			Parameters: []leaction.Parameter{{Kind: leaction.KindArgv, Value: valueAuthorisation,
				Requirement: leaction.Required, Help: "the owner's words ordering this push, recorded in the script"}},
			AnswerArgs: withRoot(answerPush),
		},
		leaction.Action{
			Verb:    keywordDebtStatus,
			Does:    "report",
			Summary: "report what a push of a verified commit still owes",
			Why: "derive what a push of the verified commit P, or of commit <rev>, would still owe: " +
				"exit 0 when nothing, 1 when anything, 2 when it cannot answer",
			Parameters: []leaction.Parameter{{Keyword: keywordCommit, Kind: leaction.KindRevision, Value: "rev",
				Requirement: leaction.Optional, Help: "the commit to judge a push of; default: verified commit P"}},
			AnswerArgs: withRoot(answerDebtStatusArgs),
		},
	)
}

// withRoot resolves the checkout before an action's body runs, so every body
// reads the same root and a line outside a checkout is refused before any work.
func withRoot(body func(root string, args leaction.Arguments) (any, int)) func(leaction.Arguments) (any, int) {
	return func(args leaction.Arguments) (any, int) {
		root, err := lepath.Root()
		if err != nil {
			leaction.ReportError(err)
			return nil, 3
		}
		return body(root, args)
	}
}

func answerSession(root string, args leaction.Arguments) (any, int) {
	session, err := lepath.CommitSession(root, args.One(keywordSession))
	if errors.Is(err, lepath.ErrCommitSessionSpelling) {
		leaction.ReportError(err)
		return nil, 2
	}
	if err != nil {
		leaction.ReportError(err)
		return nil, 3
	}
	return sessionResult{Session: session}, 0
}

func answerCreate(root string, args leaction.Arguments) (any, int) {
	options, err := optionsOf(args)
	if err != nil {
		leaction.ReportError(err)
		return nil, 2
	}
	prepared, err := Create(root, &options)
	if err != nil {
		leaction.ReportError(err)
		return nil, exitCodeOf(err)
	}
	return &prepared, 0
}

// answerMessage renders a message alone. It reads no file of the checkout, and
// resolves the root anyway, as every commit verb does. Every refusal it makes
// is of the subject or body the line gave, so it answers 2.
func answerMessage(_ string, args leaction.Arguments) (any, int) {
	message, err := Message(args.One(keywordSubject), args.Values(keywordBody), nil)
	if err != nil {
		leaction.ReportError(err)
		return nil, 2
	}
	return messageResult{Message: message}, 0
}

func answerReviewCheck(root string, args leaction.Arguments) (any, int) {
	if args.One(keywordSpec) == "" {
		return nil, table().Refuse(actionReviewCheck, errors.New(actionReviewCheck+" requires spec <stem>"))
	}
	review := CheckReview(root, args.One(keywordSpec), args.Values(keywordFile))
	if !review.Clean {
		return review, 1
	}
	return review, 0
}

func answerPush(root string, args leaction.Arguments) (any, int) {
	// The authorisation is one quoted word, so `push yes go` is refused as it
	// always was rather than joined into words the owner may not have typed.
	if len(args.Values(valueAuthorisation)) != 1 {
		return nil, table().Refuse(keywordPush,
			errors.New(`push takes the owner's authorisation, as in: ./le commit push "<owner authorisation>"`))
	}
	prepared, code, err := preparePush(root, args.One(valueAuthorisation))
	if err != nil {
		leaction.ReportError(err)
		return nil, code
	}
	return &prepared, 0
}

// answerDebtStatusArgs answers the derived debt. The derivation answers or
// errors; an error is "cannot answer", exit 3, and never reads as nothing owed.
func answerDebtStatusArgs(root string, args leaction.Arguments) (any, int) {
	status, code, err := answerDebtStatus(root, args.One(keywordCommit))
	if err != nil {
		leaction.ReportError(err)
		return nil, code
	}
	return &status, code
}

// The parameters two actions share are declared once.
var (
	sessionParameter = leaction.Parameter{Keyword: keywordSession, Kind: leaction.KindLabel, Value: "id",
		Requirement: leaction.Optional,
		Help:        "the eight-hex commit namespace from ./le commit session"}
	subjectParameter = leaction.Parameter{Keyword: keywordSubject, Kind: leaction.KindString, Value: keywordSubject,
		Requirement: leaction.Required,
		Help:        "the one-line commit subject, at most 72 characters"}
	bodyParameter = leaction.Parameter{Keyword: keywordBody, Kind: leaction.KindString, Value: "text",
		Requirement: leaction.Optional, Repeat: true,
		Help: "one body chunk, wrapped at 72; empty means a new paragraph"}
)

// reasonParameter declares one admission keyword: a reason the operator gives
// for a commit a gate would otherwise refuse.
func reasonParameter(keyword, help string) leaction.Parameter {
	return leaction.Parameter{Keyword: keyword, Kind: leaction.KindString, Value: "reason",
		Requirement: leaction.Optional, Help: help}
}

// createParameters is the closed keyword grammar of `le commit create`. What
// each keyword means to an attestation is createPolicy, keyed by the same
// keywords.
var createParameters = []leaction.Parameter{
	subjectParameter,
	bodyParameter,
	{Keyword: keywordFile, Kind: leaction.KindPath, Value: valuePath, Requirement: leaction.Optional, Repeat: true,
		Help: "one explicit file to stage, never a directory"},
	{Keyword: keywordFileList, Kind: leaction.KindPath, Value: "list", Requirement: leaction.Optional, Repeat: true,
		Help: "a file of paths to stage, one per line, # comments allowed"},
	{Keyword: keywordHunks, Kind: leaction.KindPath, Value: valuePath, Pair: "patch-file",
		Requirement: leaction.Optional, Repeat: true,
		Help: "commit HEAD's blob plus only the patch file's hunks"},
	{Keyword: keywordRemove, Kind: leaction.KindPath, Value: valuePath, Requirement: leaction.Optional, Repeat: true,
		Help: "one tracked file to delete; leave it in place for the script"},
	{Keyword: keywordRemoveList, Kind: leaction.KindPath, Value: "list", Requirement: leaction.Optional, Repeat: true,
		Help: "a file holding one tracked path to delete per line"},
	{Keyword: keywordReplace, Kind: leaction.KindSwitch, Help: "start a fresh script"},
	{Keyword: keywordAppend, Kind: leaction.KindSwitch, Help: "add another commit block to a script that already exists"},
	{Keyword: keywordScript, Kind: leaction.KindPath, Value: keywordScript, Requirement: leaction.Optional,
		Help: "the script to append to; else create picks a new path"},
	sessionParameter,
	{Keyword: keywordTag, Kind: leaction.KindLabel, Value: keywordTag, Requirement: leaction.Optional,
		Help: "the block tag inside the script"},
	{Keyword: keywordAttest, Kind: leaction.KindRevision, Value: keywordCommit, Pair: "owner words",
		Requirement: leaction.Optional,
		Help:        "prepare an empty commit attesting the named commit"},
	{Keyword: keywordLedgerCarried, Kind: leaction.KindString, Value: "row", Requirement: leaction.Optional, Repeat: true,
		Help: "a \"<sha> <gate key>: <reason>\" Ledger-carried: row"},
	reasonParameter(keywordNoTest, "the reason the commit carries no test evidence"),
	reasonParameter(keywordReviewOverride, "why a closure commits with no clean review (Review-owed:)"),
	reasonParameter(keywordRFCChangeOK, "why the commit changes an unapproved RFC-tagged test unit"),
	{Keyword: keywordDryRun, Kind: leaction.KindSwitch, Help: "print the message and the script and write neither"},
	{Keyword: keywordPush, Retired: `./le commit push "<owner authorisation>"`},
	{Keyword: "unverified", Retired: verify.ChangedCommand},
	{Keyword: "missing-full-verify-ok", Retired: verify.ChangedCommand},
	// A red prevents a release, never a commit (ai/META-DECISIONS.md, "Ze is
	// pre-release"), so no gate is left for these two to admit a commit past.
	{Keyword: "structural-red-ok", Retired: redsInTheBody},
	{Keyword: "broken-head-fix", Retired: redsInTheBody},
}

// redsInTheBody is what replaced the two admissions the structural red check
// took: the commit names each red its author knows of in its body.
const redsInTheBody = `body "<each red you know of and its cause>"`

// keywordPolicy is what one live create keyword means to an attestation.
type keywordPolicy struct {
	// Given reports whether Options carries the keyword, for a refusal that
	// judges Options rather than argv: Create is callable as Go. Every live
	// create keyword carries one (TestEveryCreateKeywordReadsOptions), and
	// refuseUndeclaredWithAttest refuses an attestation outright while a live
	// keyword lacks it.
	Given func(*Options) bool
	// WithAttest declares that an attestation may carry the keyword. Its zero
	// value refuses it, so a keyword added without a decision is refused by
	// refuseAttestAdmission rather than dropped in silence.
	WithAttest bool
}

// createPolicy holds one entry per live keyword of createParameters: how
// Options holds it, and whether an attestation admits it.
var createPolicy = map[string]keywordPolicy{
	keywordSession:        {Given: givenText(func(o *Options) string { return o.Session }), WithAttest: true},
	keywordTag:            {Given: givenText(func(o *Options) string { return o.Tag }), WithAttest: true},
	keywordSubject:        {Given: givenText(func(o *Options) string { return o.Subject }), WithAttest: true},
	keywordBody:           {Given: func(o *Options) bool { return len(o.Body) != 0 }, WithAttest: true},
	keywordFile:           {Given: givenFiles},
	keywordFileList:       {Given: givenFiles},
	keywordHunks:          {Given: func(o *Options) bool { return len(o.Hunks) != 0 }},
	keywordRemove:         {Given: givenRemove},
	keywordRemoveList:     {Given: givenRemove},
	keywordScript:         {Given: givenText(func(o *Options) string { return o.Script }), WithAttest: true},
	keywordAppend:         {Given: func(o *Options) bool { return o.Append }, WithAttest: true},
	keywordReplace:        {Given: func(o *Options) bool { return o.Replace }, WithAttest: true},
	keywordNoTest:         {Given: givenText(func(o *Options) string { return o.NoTest })},
	keywordReviewOverride: {Given: givenText(func(o *Options) string { return o.ReviewOverride })},
	keywordRFCChangeOK:    {Given: givenText(func(o *Options) string { return o.RFCChangeOK })},
	keywordDryRun:         {Given: func(o *Options) bool { return o.DryRun }, WithAttest: true},
	keywordAttest:         {Given: givenText(func(o *Options) string { return o.Attest }), WithAttest: true},
	keywordLedgerCarried:  {Given: func(o *Options) bool { return len(o.LedgerCarried) != 0 }},
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

// parseCreate reads a create line through the table, as Answer does, and
// answers the Options it names. It runs nothing.
func parseCreate(args []string) (Options, error) {
	parsed, err := table().Parse("create", args)
	if err != nil {
		return Options{}, err
	}
	return optionsOf(parsed)
}

// optionsOf answers the Options a parsed create line names. A list expands
// where the keywords are read, so Options stays one contract: an explicit
// population, however the caller wrote it down.
func optionsOf(args leaction.Arguments) (Options, error) {
	files, err := expandLists(args.Values(keywordFile), args.Values(keywordFileList))
	if err != nil {
		return Options{}, err
	}
	removed, err := expandLists(args.Values(keywordRemove), args.Values(keywordRemoveList))
	if err != nil {
		return Options{}, err
	}
	var attest [2]string
	if pairs := args.Pairs(keywordAttest); len(pairs) != 0 {
		attest = pairs[0]
	}
	return Options{
		Session: args.One(keywordSession), Tag: args.One(keywordTag), Subject: args.One(keywordSubject),
		Body: args.Values(keywordBody), Files: files, Hunks: hunkSelections(args.Values(keywordHunks)), Remove: removed,
		Script: args.One(keywordScript), Append: args.Has(keywordAppend), Replace: args.Has(keywordReplace),
		NoTest:         args.One(keywordNoTest),
		ReviewOverride: args.One(keywordReviewOverride),
		RFCChangeOK:    args.One(keywordRFCChangeOK), DryRun: args.Has(keywordDryRun),
		Attest: attest[0], AttestWords: attest[1], LedgerCarried: args.Values(keywordLedgerCarried),
	}, nil
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
