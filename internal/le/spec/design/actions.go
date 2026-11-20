// Design: docs/architecture/core-design.md -- le's composition, one import per tool
// Related: register.go -- registers this table under `le spec design`
//
// actions.go declares the area's whole command surface as data: help, the
// listing, dispatch and the grammar check all read this one table.
package specdesign

import (
	"github.com/ze-software/ze/internal/core/textbuf"
	leaction "github.com/ze-software/ze/internal/le/le/action"
)

// area is the words this command is typed as.
const area = "spec design"

// areaDescription is the area's long help.
const areaDescription = "A spec lists every design document its files link and every decision those pages " +
	"carry, each with a verdict; an independent judge confirms the verdicts; Thomas approves the design " +
	"from his own terminal before the spec moves to ready, or records that he is away."

// The parameters the actions share, declared once so two actions cannot spell
// one keyword two ways.
var (
	specParameter = leaction.Parameter{
		Keyword: "spec", Kind: leaction.KindSpec, Value: "path", Requirement: leaction.Required,
		Help: "the spec file whose design is judged",
	}
	wordsParameter = leaction.Parameter{
		Keyword: "words", Kind: leaction.KindString, Value: "text", Requirement: leaction.Required,
		Help: "what Thomas said, one line",
	}
)

var actions = leaction.New(area,
	leaction.Action{
		Verb:    "documents",
		Does:    "report",
		Summary: "list the design pages and decisions a spec owes a verdict",
		Why: "every // Design: page of the spec's Files to Modify and Files to Create, the architecture " +
			"pages the doc index anchors to them, the covering doors, the design pages the spec modifies, " +
			"and every row of their Design decisions sections, each with what derived it",
		Parameters: []leaction.Parameter{specParameter},
		AnswerArgs: answerNotBuilt("documents"),
	},
	leaction.Action{
		Verb:    "check",
		Summary: "check a spec's Design documents and Design presentation",
		Why: "every derived row present with a verdict, every quote a verbatim span of its section, " +
			"every presentation subsection written, and a current judge approval",
		Parameters: []leaction.Parameter{specParameter},
		AnswerArgs: answerNotBuilt("check"),
	},
	leaction.Action{
		Verb:    "judge",
		Summary: "record the independent judge's verdict on a spec's design",
		Why: "the judge did not write the spec and has read the section behind every row of its " +
			"Design documents table; the verdict is bound to the design digest",
		Writes: true,
		Parameters: []leaction.Parameter{
			specParameter,
			{
				Keyword: "verdict", Kind: leaction.KindEnum, Value: "verdict", Values: []string{"approve", "refuse"},
				Requirement: leaction.Required, Help: "approve when every verdict holds, refuse otherwise",
				ValueHelp: map[string]string{
					"approve": "every verdict holds against the section it names",
					"refuse":  "a verdict does not hold; the reason names the row",
				},
			},
			{
				Keyword: "reason", Kind: leaction.KindString, Value: "text", Requirement: leaction.Required,
				Help: "one sentence: why the verdict holds",
			},
		},
		AnswerArgs: answerNotBuilt("judge"),
	},
	leaction.Action{
		Verb:       "approve",
		Summary:    "record Thomas's approval of a design, from his terminal",
		Why:        "only Thomas approves a design; the record is bound to the design digest and the judge's verdict",
		Writes:     true,
		Parameters: []leaction.Parameter{specParameter, wordsParameter},
		AnswerArgs: answerNotBuilt("approve"),
	},
	leaction.Action{
		Verb:    "pending",
		Does:    "list",
		Summary: "list the ready specs whose design waits for Thomas",
		Why: "every ready spec with no matching approval, by priority, with what each lacks, then every " +
			"spec started under an away admission he has not approved since",
		Answer: func() (any, int) { return notBuilt("pending") },
	},
	leaction.Action{
		Verb:    "away",
		Does:    "record",
		Summary: "record that Thomas is away until a date, from his terminal",
		Why: "while he is away a judged fix spec with no departure starts without his approval, " +
			"and pending lists each such start for his return",
		Writes: true,
		Parameters: []leaction.Parameter{
			{
				Keyword: "until", Kind: leaction.KindString, Value: "YYYY-MM-DD", Requirement: leaction.Required,
				Help: "the last day of the absence, today or later",
			},
			wordsParameter,
		},
		AnswerArgs: answerNotBuilt("away"),
	},
	leaction.Action{
		Verb:    "return",
		Does:    "record",
		Summary: "record that Thomas is back, from his terminal",
		Why:     "removes the away record, so every ready spec again needs his approval",
		Writes:  true,
		Answer:  func() (any, int) { return notBuilt("return") },
	},
)

// Actions returns the complete command surface as structured data.
func Actions() leaction.List { return actions.Actions() }

// Subs returns the action hint that help renders.
func Subs() string { return actions.Subs() }

// Answer is the `le spec design` command.
func Answer(args []string) (any, int) { return actions.Answer(args) }

// notBuiltAnswer is what an action answers before its phase of
// plan/release/validation/spec-design-needs-owner-approval.md lands. It is
// never read as a result: the exit code says the action could not answer.
type notBuiltAnswer struct {
	Action string `json:"action"`
	Status string `json:"status"`
	Spec   string `json:"spec"`
}

// notBuiltExit is the code an action answers while it is not built: it ran
// and could not answer, which a caller tells apart from a refusal (2) and from
// an answer (0).
const notBuiltExit = 1

// Text renders the answer for a person.
func (a notBuiltAnswer) Text() string {
	var tb textbuf.Buffer
	return tb.Str("le spec design ").Str(a.Action).Str(": not built\n").
		Str("  why: the action is registered and its behavior is not implemented yet\n").
		Str("  fix: implement it under ").Str(a.Spec).Str("\n").String()
}

// notBuilt answers the not-built document for one verb.
func notBuilt(verb string) (any, int) {
	return notBuiltAnswer{
		Action: verb,
		Status: "not built",
		Spec:   "plan/release/validation/spec-design-needs-owner-approval.md",
	}, notBuiltExit
}

// answerNotBuilt adapts notBuilt to an action that takes arguments.
func answerNotBuilt(verb string) func(leaction.Arguments) (any, int) {
	return func(leaction.Arguments) (any, int) { return notBuilt(verb) }
}
