// Design: docs/architecture/core-design.md -- the rfc area, as one command
// Overview: rfc.go -- the types, the paths and the closed sets every reader here shares
//
// The action table is the single source for dispatch, help, listings, write
// metadata, and the closed keyword grammar.
package rfc

import (
	"errors"

	leaction "github.com/ze-software/ze/internal/le/le/action"
	lepath "github.com/ze-software/ze/internal/le/le/path"
)

// The vocabulary verbs (leaction's vocabulary.go) that more than one action
// below declares in Does, named once so the table spells each the same way.
const (
	doesRecord = "record"
	doesReport = "report"
)

// actions is the `rfc` command surface; the extraction walk is its sub-area,
// extractionActions below.
var actions = leaction.New(area,
	leaction.Action{
		Verb:    "tagged-scope",
		Does:    "judge",
		Summary: "judge a proposed file against its RFC-tagged tests",
		Why: "judge one proposed file from stdin against its existing RFC-tagged test units, " +
			"and return the carrier predicate, widened edit scope, and owner-approval decision",
		Parameters: []leaction.Parameter{
			{Keyword: keyPath, Kind: leaction.KindPath, Help: "the proposed file to judge", Value: keyPath, Requirement: leaction.Required},
		},
		AnswerArgs: taggedScopeAnswer,
	},
	leaction.Action{
		Verb:    "discriminate",
		Summary: "report what an RFC stem has proven and still owes",
		Why: "report what one RFC stem or one requirement id has PROVEN and what it still owes: " +
			"the recorded breaks under which a tagged unit goes red, and the tags carrying no such " +
			"record. A tag's prose says what a test demonstrates and no gate can read a sentence, " +
			"so a record is what replaces reading it",
		Parameters: []leaction.Parameter{
			// Exactly one of stem and id selects the report, which a flat
			// keyword table cannot say: naming both, or neither, is refused
			// by discriminateAnswer.
			{Keyword: keyStem, Kind: leaction.KindLabel, Help: "the RFC or draft stem to report on", Value: keyStem, Requirement: leaction.Optional},
			{Keyword: keyID, Kind: leaction.KindLabel, Help: "one requirement id to report on", Value: keyID, Requirement: leaction.Optional},
			{Keyword: keyReport, Kind: leaction.KindPath, Help: "where the report is written", Value: keyPath, Requirement: leaction.Optional},
		},
		AnswerArgs: discriminateAnswer},
	leaction.Action{
		Verb:    "record",
		Summary: "record one observed red of a tagged unit",
		Why: "record ONE proof that a tagged unit was OBSERVED to fail under a named break of " +
			"the code its claim rests on, or one escape saying no break exists. It applies the " +
			"break through a Go overlay, runs the tagged unit, and requires a failure that NAMES " +
			"that unit: a red it did not observe is never written, and a green run refuses",
		Writes: true,
		Parameters: []leaction.Parameter{
			{Keyword: keyID, Kind: leaction.KindLabel, Help: "the requirement id the proof is for", Value: keyID, Requirement: leaction.Required},
			{Keyword: keyPolarity, Kind: leaction.KindString, Help: "the polarity of the break", Value: keyPolarity, Requirement: leaction.Required},
			{Keyword: keyUnit, Kind: leaction.KindString, Help: "the tagged test unit observed to fail", Value: keyUnit, Requirement: leaction.Required},
			{Keyword: keyRoute, Kind: leaction.KindString, Help: "the proof route", Value: keyRoute, Requirement: leaction.Required},
			// The five below are each required by one route and refused by
			// another: a proving route names its producer, the mutant route
			// names the report and the mutant it applies, and the no-break
			// route gives a reason instead. validateDiscrimination holds that
			// rule, which a flat keyword table cannot state.
			{Keyword: keyProducer, Kind: leaction.KindString, Help: "the function broken, as path::Func or path::Type.Method", Value: "path::Func", Requirement: leaction.Optional},
			{Keyword: keyReport, Kind: leaction.KindPath, Help: "the discriminate report the break comes from", Value: keyPath, Requirement: leaction.Optional},
			// Repeat: every mutant named is applied to the one broken run, for
			// a property two checks guard on the same path (PairedBreak).
			{Keyword: keyMutant, Kind: leaction.KindString, Help: "one mutant, as file:line:column#n", Value: "file:line:column#n", Requirement: leaction.Optional, Repeat: true},
			{Keyword: keyCitation, Kind: leaction.KindString, Help: "the citation the record carries", Value: keyCitation, Requirement: leaction.Optional},
			{Keyword: keyReason, Kind: leaction.KindString, Help: "why the record is written this way", Value: keyReason, Requirement: leaction.Optional},
			// Required only for a unit whose build constraints hold in the
			// Linux guest alone, and refused for every other unit
			// (requireGuestKernel).
			{Keyword: keyKernel, Kind: leaction.KindPath, Help: "the kernel image the unit runs under", Value: keyPath, Requirement: leaction.Optional},
			// These three only on a kernel-producer escape (validateKernelEscape).
			// Repeat: a claim can rest on more than one kernel function, and a
			// unit can reach more than one Ze producer.
			{Keyword: keyKernelFunction, Kind: leaction.KindString, Help: "one Linux function a kernel-producer escape names", Value: keyKernelFunction, Requirement: leaction.Optional, Repeat: true},
			{Keyword: keyZeStateExcluded, Kind: leaction.KindString, Help: "one Ze producer and why its state decides nothing", Value: "producer reason", Requirement: leaction.Optional, Repeat: true},
			{Keyword: keyLaterRoute, Kind: leaction.KindString, Help: "the unbuilt route that could prove a kernel-producer escape", Value: keyLaterRoute, Requirement: leaction.Optional},
		},
		AnswerArgs: discriminateRecordAnswer},
	leaction.Action{
		Verb:    "approve",
		Summary: "record a ze-test-judge approval of an RFC test change",
		Why: "record the approval an independent ze-test-judge agent gave a change to one RFC-tagged test unit, as one row in " +
			"this commit session's tmp/commit-rfc-approved-<session>.md. The edit hook and " +
			"`./le commit create` read that file; the commit carries each row it used as an " +
			"`RFC-approved:` trailer line and drops it from the file once the commit lands. A " +
			"second call for the same unit replaces the reason. The judge RUNS IT whenever it " +
			"approves, with its verdict sentence as the reason. What it MUST NOT do is manufacture " +
			"an approval, so the agent that made the change never runs it, and the reason is never " +
			"the author's own reasoning about why the change is fine",
		Writes: true,
		Parameters: []leaction.Parameter{
			{Keyword: keyUnit, Kind: leaction.KindString, Help: "the approved RFC-tagged unit, as <package>.<TestName>", Value: "<package>.<TestName>", Requirement: leaction.Required},
			{Keyword: keyReason, Kind: leaction.KindString, Help: "the judge's verdict approving the change", Value: keyReason, Requirement: leaction.Required},
		},
		AnswerArgs: approveAnswer},
	leaction.Action{Verb: "check", Summary: "check RFC coverage, evidence and sign-off", Why: "verify RFC requirement coverage, evidence strength, public status, audit " +
		"verdicts and extraction sign-off, judged from the summaries and the tags and " +
		"never from a generated page, without writing",
		Answer: checkAnswer},
	leaction.Action{Verb: "selftest", Summary: "run every RFC engine check against fixtures", Why: "exercise every RFC engine concern against in-process fixtures and report one " +
		"structured row per property",
		Answer: selftestAnswer},
	leaction.Action{Verb: "reseal", Summary: "refresh audit fingerprints a mechanical edit staled", Why: "rewrite the file-level fingerprints of the audit verdicts a mechanical edit " +
		"staled: the tagged unit is byte-identical and only the file around it moved, " +
		"so nothing was re-judged and no human should be asked to re-read. A verdict " +
		"whose unit, cited producer code, or requirement text MOVED is refused and " +
		"stays stale: that one needs /ze-rfc-audit <rfc>, then `./le rfc update`. It writes only " +
		"the stems named, so a shift another session's edit caused in another RFC stays untouched",
		Writes: true,
		Parameters: []leaction.Parameter{
			{Keyword: keyStem, Kind: leaction.KindLabel, Help: "an RFC or draft stem whose shifted verdicts are re-stamped", Value: keyStem,
				Requirement: leaction.Required, Repeat: true},
		},
		AnswerArgs: resealAnswer},
	leaction.Action{Verb: "evidence-split", Does: "migrate", Summary: "convert evidence files from the retired layout", Why: "convert each per-stem evidence file of the retired layout, " +
		"rfc/discrimination/<stem>.json and rfc/audit/<stem>.json, into one file per requirement under " +
		"<dir>/<stem>/<requirement-id>.json, so two sessions working different requirements never edit " +
		"one file. Every loader refuses a per-stem file and names this command. It writes only the files " +
		"whose bytes change and never deletes the per-stem file: the commit that carries the converted " +
		"files removes it. from head converts what HEAD holds instead of the working tree, for a file " +
		"that also carries another session's uncommitted edits",
		Writes: true,
		Parameters: []leaction.Parameter{
			{Keyword: keyFrom, Kind: leaction.KindEnum, Help: "the source of the conversion", Value: "source", Values: []string{splitSourceHead},
				ValueHelp: map[string]string{
					splitSourceHead: "convert what HEAD holds, not the working tree",
				},
				Requirement: leaction.Optional},
		},
		AnswerArgs: evidenceSplitAnswer},
	leaction.Action{Verb: "audit-stamp", Does: doesRecord, Summary: "add newly judged verdicts to an RFC audit file", Why: "add the verdicts an author has just judged to rfc/audit/<stem>/<requirement-id>.json. " +
		"The author writes verdict, note, code keys with empty values and no_code_path in a " +
		"pending file OUTSIDE rfc/audit/, an object of rfc and requirements keyed by id, because an unstamped verdict " +
		"there stops every load of the audits. This computes requirement_sha from the row's " +
		"text and declared partial scope, tests and units from the row's tags, and each cited code sha, then writes " +
		"each entry into its own requirement file. It refuses, naming the id and writing " +
		"nothing, an id that already has a verdict, an id that is no row, a word outside the " +
		"closed vocabulary, 'enforced' over no tag or a partial scope, partial without verified producer-break claims, and 'not-applicable' over a tagged row. " +
		"mode rejudge (default new) is for a verdict a judge has re-made after re-reading the " +
		"tests: it refuses an id with no verdict, replaces each recorded entry in place with " +
		"fresh fingerprints, and admits upgrade_reason, which it requires when a weak, wrong or partial " +
		"verdict becomes enforced over byte-identical units and refuses on any other move",
		Writes: true,
		Parameters: []leaction.Parameter{
			{Keyword: keyStem, Kind: leaction.KindLabel, Help: "the RFC or draft stem the verdicts belong to", Value: keyStem, Requirement: leaction.Required},
			{Keyword: keyFrom, Kind: leaction.KindPath, Help: "the file holding the judged verdicts", Value: keyPath, Requirement: leaction.Required},
			{Keyword: keyMode, Kind: leaction.KindEnum, Help: "whether the verdicts are new or judged again", Value: "mode", Values: []string{stampModeWordNew, stampModeWordRejudge},
				ValueHelp: map[string]string{
					stampModeWordNew:     "stamp ids that hold no verdict yet",
					stampModeWordRejudge: "replace the verdicts of ids already judged",
				},
				Requirement: leaction.Optional},
		},
		AnswerArgs: auditStampAnswer},
	leaction.Action{Verb: "move", Summary: "move tagged test files with their evidence", Why: "move tagged unit test files and carry their evidence with them: from <old> to " +
		"<new> moves one file, plan <file> moves every `<old> <new>` line of a file, and propose <file> " +
		"writes such a plan from the test file naming rule (under <dir> narrows it). The move is " +
		"byte-pure, then every path key in rfc/discrimination/ and rfc/audit/ and every backtick or " +
		"link citation is rewritten, and every plain mention is listed. It refuses, writing nothing, " +
		"a target that exists, leaves its directory or GOOS/GOARCH suffix, or fails the naming rule, " +
		"a source that is untracked or differs from HEAD, and a plan with one refused pair",
		Writes: true,
		Parameters: []leaction.Parameter{
			{Keyword: keyFrom, Kind: leaction.KindPath, Help: "the test file to move", Value: keyPath, Requirement: leaction.Optional},
			{Keyword: keyTo, Kind: leaction.KindPath, Help: "where the test file goes", Value: keyPath, Requirement: leaction.Optional},
			{Keyword: keyPlan, Kind: leaction.KindPath, Help: "a plan file listing several moves", Value: keyPath, Requirement: leaction.Optional},
			{Keyword: keyPropose, Kind: leaction.KindPath, Help: "where a proposed plan is written", Value: keyPath, Requirement: leaction.Optional},
			{Keyword: keyUnder, Kind: leaction.KindPath, Help: "the directory the proposal covers", Value: "dir", Requirement: leaction.Optional},
		},
		AnswerArgs: renameAnswer},
	leaction.Action{Verb: "tree", Does: doesReport, Summary: "print the RFC dependency tree with the section counts of both bars",
		Why: "print the dependency graph of the RFC texts roots to leaves: one row per RFC or draft with its " +
			"level, aggregate, builds-on and updated-by edges, and its section counts: the MUST bar (Ze proven " +
			"by a recorded break, OS verified, OS testing gap, Ze correctness gap, and their sum, the MUST rows " +
			"kept), the MUST rows not owed by annotation kind, and the SHOULD states (proven, OS verified, " +
			"OS testing gap, not ever, not yet, not decided, not owed by kind); `not walked` without a valid " +
			"extraction sign-off, `no summary` for a text no summary covers. Membership is declared by each summary in two tiers: its `Use cases` row names the " +
			"profiles that require it, its `Optional use cases` row those that list it without counting it. " +
			"Without profile the rows a release profile requires come first, each naming both tiers. " +
			"profile <name> limits it to one use-case profile's required members, with a rollup whose " +
			"section counts sum the required tier only, its walked members, then the optional members under their own rollup, " +
			"then the candidates: RFCs one builds-on or updates edge from a member that no tier lists, which " +
			"are never owed or counted. For an aggregate it lists the members naming it, then those whose " +
			"`Aggregate tier` row is `optional` under their own rollup",
		Parameters: []leaction.Parameter{
			{Keyword: keyProfile, Kind: leaction.KindLabel, Help: "the rfc/profiles/ profile or aggregate to limit the tree to", Value: "name", Requirement: leaction.Optional, Source: profileCandidates},
		},
		AnswerArgs: treeAnswer},
	leaction.Action{Verb: "next", Does: doesReport, Summary: "name the next RFC gap to close",
		Why: "name the next RFC gap the owner rule orders: the lowest-level RFC that a profile requires " +
			"(its summary's `Use cases` row) and that owes a mandatory row (uncovered or not walked), its " +
			"uncovered requirement ids, the profiles it serves and the summary that declares it. A row marked " +
			"{updated-by} is owed at the updater's level, and a citation cycle is named whole. The figure " +
			"counts the required tier only: members of `Optional use cases` that owe rows follow in their own " +
			"section with their own rollup, never as the answered gap. Once the mandatory figure is complete " +
			"it names the next optional-register gap instead. Without profile it judges every release profile at once",
		Parameters: []leaction.Parameter{
			{Keyword: keyProfile, Kind: leaction.KindLabel, Help: "the rfc/profiles/ profile or aggregate to judge alone", Value: "name", Requirement: leaction.Optional, Source: profileCandidates},
		},
		AnswerArgs: nextAnswer},
	leaction.Action{Verb: "update", Summary: "regenerate the RFC requirement index and tables", Why: "regenerate ai/RFC-REQUIREMENTS.md and one requirement table per RFC under " +
		"rfc/requirements/, from the summaries and the `RFC requirement:` tags the " +
		"tests themselves carry. It DELETES a table the render no longer produces, " +
		"so it refuses outright when a summary did not parse: that RFC's rows would " +
		"be absent from the render and its file removed as an orphan",
		Writes: true,
		Answer: indexUpdateAnswer},
	leaction.Action{Verb: "quote-backfill", Does: "update", Summary: "rewrite an RFC's rows to quote its own sentences", Why: "rewrite the requirement rows of one RFC whose text can be the RFC's own " +
		"sentence mechanically: a row mapped to exactly one site that is not a lead-in, and whose " +
		"paraphrase shares enough with the sentence. It keeps id, level, section and markers. " +
		"Every other row is listed for a human to quote. Without apply it writes nothing",
		Writes: true,
		Parameters: []leaction.Parameter{
			{Keyword: keyStem, Kind: leaction.KindLabel, Help: "the RFC or draft stem whose rows are rewritten", Value: keyStem, Requirement: leaction.Required},
			{Keyword: keyApply, Kind: leaction.KindSwitch, Help: "write the rows; without it the action only previews"},
		},
		AnswerArgs: quoteBackfillAnswer},
)

// extractionArea is the sub-area holding the three actions of an extraction
// walk (D-P4-positional, D-P4-depth3). Package rfcextraction registers it at
// the path its name predicts; the table and the answers stay here, beside the
// corpus they read.
const extractionArea = area + " extraction"

// extractionActions is the `rfc extraction` command surface.
var extractionActions = leaction.New(extractionArea,
	leaction.Action{Verb: "create", Summary: "derive one RFC's extraction skeleton", Why: "derive one RFC or draft's extraction skeleton, preserving authored classifications only " +
		"where the same locator still carries the same sentence. It reaches rfc/extraction/ only when every " +
		"site and section already carries a disposition; an unclassified skeleton goes to this session's " +
		"scratch, because one unclassified artifact in the corpus fails `./le rfc check` for all of it",
		Writes: true,
		Parameters: []leaction.Parameter{
			{Keyword: keyStem, Kind: leaction.KindLabel, Help: "the RFC or draft stem, such as rfc4271", Value: keyStem, Requirement: leaction.Required},
		},
		AnswerArgs: extractionCreateAnswer},
	leaction.Action{Verb: "classify", Does: doesRecord, Summary: "apply authored decisions to an extraction skeleton", Why: "apply ONE authored decision per site and per section to the skeleton, from a decisions " +
		"file naming the stem it walks. It writes no disposition the file does not name and " +
		"matches no locator pattern, so it transcribes a walk rather than performing one; a site " +
		"the file leaves out stays unclassified and is named in the report. binds-another-role " +
		"must cite a producer the reason carries, and feature-out-of-scope must quote the " +
		"sentence that makes the feature optional, checked against the RFC's own text",
		Writes: true,
		Parameters: []leaction.Parameter{
			{Keyword: keyDecisions, Kind: leaction.KindPath, Help: "the decisions file to apply", Value: keyPath, Requirement: leaction.Required},
		},
		AnswerArgs: extractionClassifyAnswer},
	leaction.Action{Verb: "report", Summary: "print the extraction counts the drain quota reads", Why: "the machine-readable extraction counts the umbrella's drain quota consumes: " +
		"signed and enrolled counts, the per-register split, and the unsigned backlog",
		Answer: extractionStatusAnswer},
)

// ExtractionActions answers the `rfc extraction` command surface as data.
func ExtractionActions() leaction.List { return extractionActions.Actions() }

// ExtractionSubs is the one-line hint help renders under `rfc extraction`.
func ExtractionSubs() string { return extractionActions.Subs() }

// ExtractionAnswer is the `le rfc extraction` command.
func ExtractionAnswer(args []string) (any, int) { return extractionActions.Answer(args) }

// Actions answers the command surface as data, so the listing, the Subs line
// help renders, and the test that checks them all read one table.
func Actions() leaction.List { return actions.Actions() }

// Subs is the one-line hint help renders under the command.
func Subs() string { return actions.Subs() }

// Answer is the `le rfc` command.
func Answer(args []string) (any, int) { return actions.Answer(args) }

// extractionCreateAnswer writes one unsigned skeleton in this checkout.
func extractionCreateAnswer(args leaction.Arguments) (any, int) {
	if !args.Has(keyStem) {
		leaction.ReportError(errors.New("rfc extraction create requires stem <stem>"))
		return nil, 2
	}
	stem := args.One(keyStem)
	tree, err := lepath.Root()
	if err != nil {
		leaction.ReportError(err)
		return nil, 2
	}
	report, err := createExtraction(tree, stem)
	if err != nil {
		leaction.ReportError(err)
		return nil, 2
	}
	return report, 0
}

// extractionClassifyAnswer applies one decisions file in this checkout.
func extractionClassifyAnswer(args leaction.Arguments) (any, int) {
	if !args.Has(keyDecisions) {
		leaction.ReportError(errors.New("rfc extraction classify requires decisions <path>"))
		return nil, 2
	}
	path := args.One(keyDecisions)
	tree, err := lepath.Root()
	if err != nil {
		leaction.ReportError(err)
		return nil, 2
	}
	report, err := classifyExtraction(tree, path)
	if err != nil {
		leaction.ReportError(err)
		return nil, 2
	}
	return report, 0
}

// extractionStatusAnswer derives the envelope over this checkout.
//
// It answers 3, could not run (D-Q8, D-P5-extraction), for anything that
// stopped it: a checkout with no rfc/ tree, a malformed summary, an unreadable
// enrolled list, a malformed artifact and a tag in a carrier nothing runs are
// all input the count needs and does not have, and "clean" must never mean "I
// compared nothing".
func extractionStatusAnswer() (any, int) {
	tree, err := lepath.Root()
	if err != nil {
		leaction.ReportError(err)
		return nil, 3
	}
	collected, err := Collect(tree)
	if err != nil {
		leaction.ReportError(err)
		return nil, 3
	}
	status, err := extractionStatus(NewDeriver(tree), collected.Requirements, collected.Enrolled)
	if err != nil {
		leaction.ReportError(err)
		return nil, 3
	}
	return status, 0
}

// discriminateAnswer reports one selector's recorded proofs and unproven tags.
//
// Exactly one selector, because the two answer different questions and a call
// naming both would have to pick one silently. It answers 2 for anything that
// stopped it, which includes a malformed record: a corrupt artifact must never
// read as a stem with nothing proven.
func discriminateAnswer(args leaction.Arguments) (any, int) {
	stem := args.One(keyStem)
	rid := args.One(keyID)
	hasStem := args.Has(keyStem)
	hasID := args.Has(keyID)
	if hasStem == hasID {
		leaction.ReportError(errors.New("rfc discriminate requires exactly one of stem <stem> or id <ID>"))
		return nil, 2
	}
	selector := rid
	selected := func(one string) bool { return one == rid }
	if hasStem {
		selector = stem
		selected = func(one string) bool { return hasRIDStem(one, stem) }
	}
	if selector == "" {
		leaction.ReportError(errors.New("rfc discriminate was given an empty selector"))
		return nil, 2
	}
	tree, err := lepath.Root()
	if err != nil {
		leaction.ReportError(err)
		return nil, 2
	}
	status, err := discriminationStatusOf(tree, selector, args.One(keyReport), selected)
	if err != nil {
		leaction.ReportError(err)
		return nil, 2
	}
	return status, 0
}

// checkAnswer runs the read-only RFC requirement gate over this checkout.
//
// The answer is a POINTER, and it MUST stay one. `CheckReport.Text` carries a
// pointer receiver because the struct is past gocritic's hugeParam threshold,
// and `leroot.Prose` is matched by a type assertion on the answer. A
// `CheckReport` value fails that assertion, so the dispatcher falls back to the
// generic table renderer and the violation page a person reads disappears with
// no error, no log line and no change of exit code.
func checkAnswer() (any, int) {
	tree, err := lepath.Root()
	if err != nil {
		return &CheckReport{CannotRun: err.Error()}, 3
	}
	approvals, err := sessionApprovals(tree)
	if err != nil {
		return &CheckReport{CannotRun: err.Error()}, 3
	}
	report, code := Check(tree, approvals)
	return &report, code
}

// resealAnswer re-stamps the shifted verdicts of the named stems in this
// checkout.
//
// It answers 0 whether or not anything was re-stamped, and whether or not
// anything was refused, which is what the script answers. A refusal is a
// verdict a human must re-read, not a broken tree: reporting it as a failure
// would make the ledger generator that follows look like the remedy. A tree
// the writer could not READ, no stem, or a stem that is not enrolled answers 2.
func resealAnswer(args leaction.Arguments) (any, int) {
	tree, err := lepath.Root()
	if err != nil {
		leaction.ReportError(err)
		return nil, 2
	}
	report, err := resealStems(tree, args.Values(keyStem))
	if err != nil {
		leaction.ReportError(err)
		return nil, 2
	}
	return report, 0
}

// evidenceSplitAnswer converts the per-stem evidence files in this checkout.
//
// It answers 0 when every file converted, and 2 for a file it could not read
// or that does not validate: nothing is converted from a file the loader
// would refuse.
func evidenceSplitAnswer(args leaction.Arguments) (any, int) {
	source := ""
	if args.Has(keyFrom) {
		source = args.One(keyFrom)
		if source != splitSourceHead {
			leaction.ReportError(errors.New("rfc evidence-split: from takes only `head`, the content HEAD holds"))
			return nil, 2
		}
	}
	tree, err := lepath.Root()
	if err != nil {
		leaction.ReportError(err)
		return nil, 2
	}
	report, err := splitEvidence(tree, source)
	if err != nil {
		leaction.ReportError(err)
		return nil, 2
	}
	return report, 0
}

// auditStampAnswer stamps the new verdicts of one stem in this checkout.
//
// It answers 0 when every pending verdict was stamped and merged, and 2 for a
// refusal as well as for a tree it could not read: a refused verdict is a claim
// the author must correct, and nothing was written.
func auditStampAnswer(args leaction.Arguments) (any, int) {
	if !args.Has(keyStem) {
		leaction.ReportError(errors.New("rfc audit-stamp requires stem <stem>"))
		return nil, 2
	}
	if !args.Has(keyFrom) {
		leaction.ReportError(errors.New("rfc audit-stamp requires from <path>, the pending verdicts file"))
		return nil, 2
	}
	mode, err := parseStampMode(args.One(keyMode), args.Has(keyMode))
	if err != nil {
		leaction.ReportError(err)
		return nil, 2
	}
	tree, err := lepath.Root()
	if err != nil {
		leaction.ReportError(err)
		return nil, 2
	}
	report, err := auditStamp(tree, args.One(keyStem), args.One(keyFrom), mode)
	if err != nil {
		leaction.ReportError(err)
		return nil, 2
	}
	return report, 0
}

// indexUpdateAnswer regenerates the ledger and its per-RFC tables.
//
// It answers 2 for everything that stopped it, and the two REFUSALS are among
// them: a summary that did not parse and a render with no rows are both states
// where the prune would delete a tracked file the generator still owns, so
// neither may report success. Nothing has been written when either fires.
func indexUpdateAnswer() (any, int) {
	tree, err := lepath.Root()
	if err != nil {
		leaction.ReportError(err)
		return nil, 2
	}
	report, err := IndexUpdate(tree)
	if err != nil {
		leaction.ReportError(err)
		return nil, 2
	}
	return report, 0
}

// treeAnswer prints the dependency tree of this checkout.
//
// It answers 2 for anything that stopped it, an unknown profile included: a
// tree over a corpus that did not load would show figures nobody counted.
func treeAnswer(args leaction.Arguments) (any, int) {
	tree, err := lepath.Root()
	if err != nil {
		leaction.ReportError(err)
		return nil, 2
	}
	report, err := treeReport(tree, args.One(keyProfile))
	if err != nil {
		leaction.ReportError(err)
		return nil, 2
	}
	return report, 0
}

// nextAnswer names the next gap of this checkout. It answers 2 for anything
// that stopped it, as treeAnswer does.
func nextAnswer(args leaction.Arguments) (any, int) {
	tree, err := lepath.Root()
	if err != nil {
		leaction.ReportError(err)
		return nil, 2
	}
	report, err := nextReport(tree, args.One(keyProfile))
	if err != nil {
		leaction.ReportError(err)
		return nil, 2
	}
	return report, 0
}

// profileCandidates answers the names rfc/profiles/ holds, use cases and
// aggregates, for the `profile` value of `./le rfc tree` and `./le rfc next`.
// It reads the profiles the action itself reads, so completion offers
// exactly the names scope accepts. A keystroke calls
// it, so it reports no error: a profile file that does not parse is left out,
// as scope refuses it, and a checkout that cannot be read offers nothing.
func profileCandidates() []leaction.Candidate {
	tree, err := lepath.Root()
	if err != nil {
		// refuse-by-default: guards leaction.ValueSource -- a completion has no error channel, and no candidate is its refusal
		return nil
	}
	profiles, _, err := LoadProfiles(tree)
	if err != nil {
		// refuse-by-default: guards leaction.ValueSource -- a completion has no error channel, and no candidate is its refusal
		return nil
	}
	out := make([]leaction.Candidate, 0, len(profiles.UseCases)+len(profiles.Aggregates))
	for _, name := range sortedKeysOf(profiles.UseCases) {
		out = append(out, leaction.Candidate{Value: name, Summary: "use case: " + profiles.UseCases[name].Title})
	}
	for _, name := range sortedKeysOf(profiles.Aggregates) {
		out = append(out, leaction.Candidate{Value: name, Summary: "aggregate: " + profiles.Aggregates[name].Title})
	}
	return out
}
