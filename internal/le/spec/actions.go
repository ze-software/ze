// Design: docs/architecture/core-design.md -- native spec lifecycle commands
// Related: session.go -- spec ownership and state paths
// Related: review.go -- independent review artifacts

package spec

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"

	"github.com/ze-software/ze/internal/core/textbuf"
	leaction "github.com/ze-software/ze/internal/le/le/action"
	lepath "github.com/ze-software/ze/internal/le/le/path"
)

// The spec namespace holds one member per lifecycle verb, each registered from
// its own directory (internal/le/spec/<verb>/register.go). The bare `le spec`
// is the namespace token, so the dispatcher lists these members rather than
// answering one of them.
//
// Each member dispatches through its own action table, declared here beside
// the bodies it runs, so the table is the only parser of its line. A member
// whose bare line does one thing (current, claim, release, wip) still does it
// when bare: hooks and skills call it that way, and a bare listing would answer
// them 0 with nothing done. The spellings callers type are kept until the
// rename phase of plan/post-release/engineering/spec-le-cli-usability.md, so
// `spec claim spec <spec>` is the action `spec` with the spec file as its
// positional value.

// The value slots the review and state grammars name more than once.
const (
	// valueStem is the slot of a keyword that takes a spec's stem.
	valueStem = "stem"
	// valuePath is the slot of a keyword that takes one path.
	valuePath = "path"
)

// CurrentArea is `le spec current`: the spec this session claimed.
func CurrentArea() leaction.Area {
	return leaction.New("spec current", leaction.Action{
		Verb:    verbShow,
		Summary: "show the spec this session claimed",
		Why:     "the spec this session claimed",
		Answer:  answerCurrent,
	})
}

// ClaimArea is `le spec claim spec <spec>`: this session takes the spec.
func ClaimArea() leaction.Area {
	return leaction.New("spec claim", leaction.Action{
		Verb:    keywordSpec,
		Summary: "claim a spec for this session",
		Why:     "claim a spec for this session and move a ready spec to in-progress",
		Parameters: []leaction.Parameter{{
			Kind:        leaction.KindSpec,
			Value:       keywordSpec,
			Help:        "the spec file this session takes",
			Requirement: leaction.Required,
		}},
		AnswerArgs: answerClaim,
	})
}

// ReleaseArea is `le spec release`: this session gives its claim back.
func ReleaseArea() leaction.Area {
	return leaction.New("spec release", leaction.Action{
		Verb:    verbRun,
		Summary: "release this session's spec claim",
		Why:     "release this session's spec claim",
		Writes:  true,
		Answer:  answerRelease,
	})
}

// WIPArea is `le spec wip`: the in-progress specs against the WIP cap.
func WIPArea() leaction.Area {
	return leaction.New("spec wip", leaction.Action{
		Verb:    verbReport,
		Summary: "list in-progress specs against the WIP cap",
		Why:     "the in-progress specs, against the work-in-progress cap",
		Answer:  answerWIP,
	})
}

// StateArea is `le spec state current` and `le spec state latest spec <stem>`:
// the path of a per-spec state file.
func StateArea() leaction.Area {
	return leaction.New("spec state",
		leaction.Action{
			Verb:    verbCurrent,
			Summary: "print this session's per-spec state file path",
			Why:     "the path of this session's per-spec state file",
			Answer:  answerStateCurrent,
		},
		leaction.Action{
			Verb:    verbLatest,
			Summary: "print the newest state file for one spec",
			Why:     "the path of the newest state file any session wrote for one spec",
			Parameters: []leaction.Parameter{{
				Keyword:     keywordSpec,
				Kind:        leaction.KindLabel,
				Value:       "stem",
				Help:        "the spec's file name, without its directory or .md",
				Requirement: leaction.Required,
			}},
			AnswerArgs: answerStateLatest,
		},
	)
}

// ModelArea is `le spec model current [transcript <absolute-path>]`: the model a
// session transcript records.
func ModelArea() leaction.Area {
	return leaction.New("spec model", leaction.Action{
		Verb:    verbCurrent,
		Summary: "print the model a session transcript records",
		Why:     "the model a session transcript records",
		Parameters: []leaction.Parameter{{
			Keyword:     keywordTranscript,
			Kind:        leaction.KindPath,
			Value:       "absolute-path",
			Help:        "read this transcript rather than this session's own",
			Requirement: leaction.Optional,
		}},
		AnswerArgs: answerModelCurrent,
	})
}

// ReviewArea is `le spec review <hash|record|check> ...`: independent review
// artifacts.
func ReviewArea() leaction.Area {
	return leaction.New("spec review",
		leaction.Action{
			Verb:    verbHash,
			Summary: "print the review hash of each named file",
			Why:     "the review hash of each named file",
			Parameters: []leaction.Parameter{{
				Keyword: keywordFile, Kind: leaction.KindPath, Value: valuePath,
				Help: "a file to hash", Requirement: leaction.Required, Repeat: true,
			}},
			AnswerArgs: answerReviewHashLine,
		},
		leaction.Action{
			Verb:       verbRecord,
			Summary:    "record this session's review of a spec's files",
			Why:        "record this session's independent review of a spec's files",
			Writes:     true,
			Parameters: reviewRecordParameters,
			AnswerArgs: answerReviewRecordLine,
		},
		leaction.Action{
			Verb:    verbCheck,
			Summary: "check a recorded review still covers the spec",
			Why:     "whether a recorded review still covers the spec's files as they are now",
			Parameters: []leaction.Parameter{
				{
					Keyword: keywordSpec, Kind: leaction.KindLabel, Value: valueStem,
					Help: "the spec the review is for", Requirement: leaction.Required,
				},
				{
					Keyword: keywordFile, Kind: leaction.KindPath, Value: valuePath,
					Help: "a file the review must cover", Requirement: leaction.Optional, Repeat: true,
				},
			},
			AnswerArgs: answerReviewCheckLine,
		},
	)
}

// reviewRecordParameters is the grammar of `le spec review record`.
var reviewRecordParameters = []leaction.Parameter{
	{
		Keyword: keywordSpec, Kind: leaction.KindLabel, Value: valueStem,
		Help: "the spec the review is for", Requirement: leaction.Required,
	},
	{
		Keyword: keywordVerdict, Kind: leaction.KindLabel, Value: "verdict",
		Help: "the review's verdict", Requirement: leaction.Required,
	},
	{
		Keyword: keywordRounds, Kind: leaction.KindUint, Value: "n",
		Help: "how many review rounds ran, at least one", Requirement: leaction.Required,
	},
	{
		Keyword: keywordFile, Kind: leaction.KindPath, Value: valuePath,
		Help:        "a whole file the review covers (file or hunks required)",
		Requirement: leaction.Optional, Repeat: true,
	},
	{
		Keyword: keywordHunks, Kind: leaction.KindPath, Value: valuePath, Pair: "patch-file",
		Help:        "the hunks of one shared file the review covers, as a patch",
		Requirement: leaction.Optional, Repeat: true,
	},
	{
		Keyword: keywordReviewers, Kind: leaction.KindString, Value: "names",
		Help: "who reviewed", Requirement: leaction.Optional,
	},
	{
		Keyword: keywordFindingsFile, Kind: leaction.KindPath, Value: valuePath,
		Help: "the file that lists the review's findings", Requirement: leaction.Optional,
	},
	{
		Keyword: keywordRoundsReason, Kind: leaction.KindString, Value: "reason",
		Help: "why the review took more rounds than the cap", Requirement: leaction.Optional,
	},
	{
		Keyword: keywordOwnerAuthorised, Kind: leaction.KindString, Value: "words",
		Help: "the owner's words that allow rounds past the cap", Requirement: leaction.Optional,
	},
}

// Current is the `le spec current` command. A bare line shows the claim.
func Current(args []string) (any, int) {
	return CurrentArea().Answer(bareRuns(args, verbShow))
}

// Claim is the `le spec claim` command. A bare line reaches the `spec` action,
// whose body refuses it with claim's own usage, so the refusal names no other
// member of the namespace.
func Claim(args []string) (any, int) {
	return ClaimArea().Answer(bareRuns(args, keywordSpec))
}

// Release is the `le spec release` command. A bare line releases the claim.
func Release(args []string) (any, int) {
	return ReleaseArea().Answer(bareRuns(args, verbRun))
}

// WIP is the `le spec wip` command. A bare line reports the WIP cap.
func WIP(args []string) (any, int) {
	return WIPArea().Answer(bareRuns(args, verbReport))
}

// State is the `le spec state` command.
func State(args []string) (any, int) { return StateArea().Answer(args) }

// Model is the `le spec model` command.
func Model(args []string) (any, int) { return ModelArea().Answer(args) }

// Review is the `le spec review` command.
func Review(args []string) (any, int) { return ReviewArea().Answer(args) }

// bareRuns answers the line a member's table reads: the member's one verb when
// the line is bare, else the line as typed, so every word typed is the table's.
func bareRuns(args []string, verb string) []string {
	if len(args) == 0 {
		return []string{verb}
	}
	return args
}

func answerCurrent() (any, int) {
	owner, err := ownerAtRoot()
	if err != nil {
		return lifecycleError(err)
	}
	spec, err := owner.currentSpec()
	if err != nil {
		return lifecycleError(err)
	}
	return currentReport{Spec: spec}, 0
}

func answerClaim(args leaction.Arguments) (any, int) {
	if !args.Has(keywordSpec) {
		return nil, ClaimArea().Refuse(keywordSpec, errors.New("spec claim needs the spec file to take"))
	}
	owner, err := ownerAtRoot()
	if err != nil {
		return lifecycleError(err)
	}
	report, err := owner.Claim(args.One(keywordSpec))
	if err != nil {
		return lifecycleError(err)
	}
	if report.Refused {
		return report, 3
	}
	return report, 0
}

func answerRelease() (any, int) {
	owner, err := ownerAtRoot()
	if err != nil {
		return lifecycleError(err)
	}
	if err := owner.Release(); err != nil {
		return lifecycleError(err)
	}
	return releaseReport{Released: true}, 0
}

func answerWIP() (any, int) {
	root, err := lepath.Root()
	if err != nil {
		return lifecycleError(err)
	}
	report, err := wip(root, configuredWIPCap())
	if err != nil {
		return lifecycleError(err)
	}
	return report, 0
}

func answerStateCurrent() (any, int) {
	owner, err := ownerAtRoot()
	if err != nil {
		return lifecycleError(err)
	}
	path, err := owner.StateFile()
	if err != nil {
		return lifecycleError(err)
	}
	return statePathReport{Path: path}, 0
}

func answerStateLatest(args leaction.Arguments) (any, int) {
	if !args.Has(keywordSpec) {
		return nil, StateArea().Refuse(verbLatest, errors.New("spec state latest needs spec <stem>"))
	}
	owner, err := ownerAtRoot()
	if err != nil {
		return lifecycleError(err)
	}
	path, err := LatestStateForSpec(owner.Root, args.One(keywordSpec))
	if err != nil {
		return lifecycleError(err)
	}
	return statePathReport{Path: path}, 0
}

func answerModelCurrent(args leaction.Arguments) (any, int) {
	root, err := lepath.Root()
	if err != nil {
		return lifecycleError(err)
	}
	return answerModel(root, args)
}

// answerModel answers the model of the transcript the line names, or of this
// session's own transcript under root when it names none.
func answerModel(root string, args leaction.Arguments) (any, int) {
	if !args.Has(keywordTranscript) {
		return answerRunningModel(TranscriptPath(root))
	}
	path, err := readableTranscriptPath(args.One(keywordTranscript))
	if err != nil {
		return lifecycleError(err)
	}
	return answerRunningModel(path)
}

func answerRunningModel(path string) (any, int) {
	model := RunningModel(path)
	report := modelReport{Transcript: path, Model: model, Readable: model != ""}
	if model == "" {
		return report, 1
	}
	return report, 0
}

func readableTranscriptPath(path string) (string, error) {
	if path == "" {
		return "", errors.New("model transcript needs a non-empty path")
	}
	if !filepath.IsAbs(path) {
		return "", fmt.Errorf("model transcript path must be absolute: %q", path)
	}
	info, err := os.Stat(path)
	if err != nil {
		return "", fmt.Errorf("read model transcript %s: %w", path, err)
	}
	if !info.Mode().IsRegular() {
		return "", fmt.Errorf("model transcript is not a regular file: %s", path)
	}
	return path, nil
}

func answerReviewHashLine(args leaction.Arguments) (any, int) {
	root, err := lepath.Root()
	if err != nil {
		return lifecycleError(err)
	}
	return answerReviewHash(root, args)
}

// answerReviewHash answers the review hash of each file the line names, under
// root.
func answerReviewHash(root string, args leaction.Arguments) (any, int) {
	files := args.Values(keywordFile)
	if len(files) == 0 {
		return nil, ReviewArea().Refuse(verbHash, errors.New("review hash needs file <path>"))
	}
	report := reviewHashes{Files: make([]ReviewedFile, 0, len(files))}
	for _, path := range uniqueSorted(files) {
		hash, err := reviewFileHash(root, path)
		if err != nil {
			return lifecycleError(err)
		}
		report.Files = append(report.Files, ReviewedFile{Path: path, Hash: hash})
	}
	return report, 0
}

func answerReviewRecordLine(args leaction.Arguments) (any, int) {
	owner, err := ownerAtRoot()
	if err != nil {
		return lifecycleError(err)
	}
	return answerReviewRecord(owner.Root, owner.SessionID, args)
}

// answerReviewRecord records the review the line describes as session sid's,
// under root. The session is passed in rather than read from the environment,
// so a caller that resolved it once records under that session.
func answerReviewRecord(root, sid string, args leaction.Arguments) (any, int) {
	request, err := reviewRecordOf(args)
	if err != nil {
		return nil, ReviewArea().Refuse(verbRecord, err)
	}
	request.SessionID = sid
	artifact, err := recordReview(root, request)
	if err != nil {
		return lifecycleError(err)
	}
	return artifact, 0
}

func answerReviewCheckLine(args leaction.Arguments) (any, int) {
	owner, err := ownerAtRoot()
	if err != nil {
		return lifecycleError(err)
	}
	return answerReviewCheck(owner.Root, owner.SessionID, args)
}

// answerReviewCheck answers whether session sid's recorded review still covers
// the files the line names, under root. A blocked review answers 3.
func answerReviewCheck(root, sid string, args leaction.Arguments) (any, int) {
	if !args.Has(keywordSpec) {
		return nil, ReviewArea().Refuse(verbCheck, errors.New("review check needs spec <stem>"))
	}
	check, err := CheckReview(root, args.One(keywordSpec), sid, args.Values(keywordFile))
	if err != nil {
		return lifecycleError(err)
	}
	if check.Blocked {
		return check, 3
	}
	return check, 0
}

// reviewRecordOf turns the parsed line into the record request, refusing one
// that lacks what every review records.
func reviewRecordOf(args leaction.Arguments) (reviewRecord, error) {
	request := reviewRecord{
		Spec:            args.One(keywordSpec),
		Verdict:         args.One(keywordVerdict),
		Files:           args.Values(keywordFile),
		Reviewers:       args.One(keywordReviewers),
		FindingsFile:    args.One(keywordFindingsFile),
		RoundsReason:    args.One(keywordRoundsReason),
		OwnerAuthorised: args.One(keywordOwnerAuthorised),
	}
	for _, pair := range args.Pairs(keywordHunks) {
		request.Hunks = append(request.Hunks, ReviewHunk{Path: pair[0], Patch: pair[1]})
	}
	if args.Has(keywordRounds) {
		value := args.One(keywordRounds)
		rounds, err := strconv.Atoi(value)
		if err != nil {
			return reviewRecord{}, fmt.Errorf("review rounds %q is not an integer", value)
		}
		request.Rounds = rounds
	}
	if request.Spec == "" {
		return reviewRecord{}, errors.New("review record needs spec, verdict, rounds, and at least one file")
	}
	if request.Verdict == "" {
		return reviewRecord{}, errors.New("review record needs spec, verdict, rounds, and at least one file")
	}
	if request.Rounds == 0 {
		return reviewRecord{}, errors.New("review record needs spec, verdict, rounds, and at least one file")
	}
	if len(request.Files) == 0 && len(request.Hunks) == 0 {
		return reviewRecord{}, errors.New("review record needs spec, verdict, rounds, and at least one file")
	}
	return request, nil
}

// ownerAtRoot answers the spec owner of this session in the checkout le runs in.
func ownerAtRoot() (specOwner, error) {
	root, err := lepath.Root()
	if err != nil {
		return specOwner{}, err
	}
	return newSpecOwner(root)
}

// lifecycleError reports one refusal on stderr. Every lifecycle refusal exits 2:
// the caller is a session that asked for something the gate does not allow.
func lifecycleError(err error) (any, int) {
	var tb textbuf.Buffer
	tb.Str("error: spec-session: ").Err(err).Byte('\n').StdErr() //nolint:errcheck // CLI output
	return nil, 2
}

// The command verbs and keywords this lifecycle accepts, the spec status it
// transitions from, and the `commit check` refusal reason.
const (
	verbCurrent         = "current"
	verbLatest          = "latest"
	verbShow            = "show"
	verbRun             = "run"
	verbReport          = "report"
	verbHash            = "hash"
	verbRecord          = "record"
	verbCheck           = "check"
	keywordSpec         = "spec"
	keywordFile         = "file"
	keywordHunks        = "hunks"
	keywordRounds       = "rounds"
	keywordReviewers    = "reviewers"
	keywordFindingsFile = "findings-file"
	keywordRoundsReason = "rounds-reason"
	keywordTranscript   = "transcript"
	keywordVerdict      = "verdict"
	reasonMissing       = "missing"
	statusReady         = "ready"
	// keywordOwnerAuthorised is a CLI keyword and a JSON key, not prose. It is
	// named in ai/rules/planning.md, ai/skills/ze-close.md, ai/skills/ze-review.md
	// and plan/templates/TEMPLATE-CLOSURE.md, so the spelling is a contract and
	// renaming it is the owner's decision.
	keywordOwnerAuthorised = "owner-authorised" //nolint:misspell // a CLI keyword whose spelling is a contract; see the comment above
)
