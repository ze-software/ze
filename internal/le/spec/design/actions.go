// Design: docs/architecture/core-design.md -- le's composition, one import per tool
// Related: register.go -- registers this table under `le spec design`
//
// actions.go declares the area's whole command surface as data: help, the
// listing, dispatch and the grammar check all read this one table.
package specdesign

import (
	leaction "github.com/ze-software/ze/internal/le/le/action"
)

// area is the words this command is typed as.
const area = "spec design"

// areaDescription is the area's long help.
const areaDescription = "A spec lists every design document its files link and every decision those pages " +
	"carry, each with a verdict; an independent judge confirms the verdicts; Thomas approves the design " +
	"from his own terminal before the spec moves to ready, or records that he is away; his approval " +
	"carries over to a later text that only writes in decisions he made."

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
		AnswerArgs: answerDocuments,
	},
	leaction.Action{
		Verb:    "check",
		Summary: "check a spec's Design documents and Design presentation",
		Why: "every derived row present with a verdict, every quote a verbatim span of its section, " +
			"every presentation subsection written, and a current judge approval",
		Parameters: []leaction.Parameter{specParameter},
		AnswerArgs: answerCheck,
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
				Keyword: "verdict", Kind: leaction.KindEnum, Value: "verdict", Values: []string{judgeApprove, judgeRefuse},
				Requirement: leaction.Required, Help: "approve when every verdict holds, refuse otherwise",
				ValueHelp: map[string]string{
					judgeApprove: "every verdict holds against the section it names",
					judgeRefuse:  "a verdict does not hold; the reason names the row",
				},
			},
			{
				Keyword: "reason", Kind: leaction.KindString, Value: "text", Requirement: leaction.Required,
				Help: "one sentence: why the verdict holds",
			},
		},
		AnswerArgs: answerJudge,
	},
	leaction.Action{
		Verb:       "approve",
		Summary:    "record Thomas's approval of a design, from his terminal",
		Why:        "only Thomas approves a design; the record is bound to the design digest and the judge's verdict",
		Writes:     true,
		Parameters: []leaction.Parameter{specParameter, wordsParameter},
		AnswerArgs: answerApprove,
	},
	leaction.Action{
		Verb:    "carry",
		Does:    doesRecord,
		Summary: "carry Thomas's approval to a text that only writes in his own decisions",
		Why: "a change that only adds a verdict on a row of a page's Design decisions section, or follows a " +
			"rename on a page a row quotes, is not put back to him: the judge approves the new text, this " +
			"action proves nothing else changed against the text he approved, and anything else is refused " +
			"by name and goes back to him",
		Writes:     true,
		Parameters: []leaction.Parameter{specParameter},
		AnswerArgs: answerCarry,
	},
	leaction.Action{
		Verb:    "pending",
		Does:    "list",
		Summary: "list the ready specs whose design waits for Thomas",
		Why: "every ready spec with no matching approval, by priority, with what each lacks, then every " +
			"spec started under an away admission he has not approved since",
		Answer: answerPending,
	},
	leaction.Action{
		Verb:    "away",
		Does:    doesRecord,
		Summary: "record that Thomas is away until a date, from his terminal",
		Why: "while he is away a judged fix spec with no departure starts without his approval, " +
			"and pending lists each such start for his return",
		Writes: true,
		Parameters: []leaction.Parameter{
			{
				Keyword: untilKeyword, Kind: leaction.KindString, Value: "YYYY-MM-DD", Requirement: leaction.Required,
				Help: "the last day of the absence, today or later",
			},
			wordsParameter,
		},
		AnswerArgs: answerAway,
	},
	leaction.Action{
		Verb:    "return",
		Does:    doesRecord,
		Summary: "record that Thomas is back, from his terminal",
		Why:     "removes the away record, so every ready spec again needs his approval",
		Writes:  true,
		Answer:  answerReturn,
	},
)

// Actions returns the complete command surface as structured data.
func Actions() leaction.List { return actions.Actions() }

// Subs returns the action hint that help renders.
func Subs() string { return actions.Subs() }

// Answer is the `le spec design` command.
func Answer(args []string) (any, int) { return actions.Answer(args) }

// doesRecord is the Does word of an action that writes a design record.
const doesRecord = "record"
