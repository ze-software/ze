// Design: docs/contributing/committing.md -- native approval checking and exact-row cleanup
// Related: actions.go -- parent forwards to this same registered table
// Related: approval/register.go -- command registration at its namespace path
package commit

import (
	"errors"
	"strconv"

	"github.com/ze-software/ze/internal/core/opererr"
	leaction "github.com/ze-software/ze/internal/le/le/action"
	lepath "github.com/ze-software/ze/internal/le/le/path"
	"github.com/ze-software/ze/internal/le/rfc"
	testweakened "github.com/ze-software/ze/internal/le/test/weakened"
)

const approvalArea = area + " " + actionApprovalCheck

const (
	approvalClean         = "clean"
	approvalKindKeyword   = "kind"
	approvalRowKeyword    = "row"
	approvalRFCName       = "rfc"
	approvalWeakeningName = "weakening"
)

type approvalKind uint8

const (
	approvalKindUnspecified approvalKind = iota
	approvalKindRFC
	approvalKindWeakening
)

// ApprovalAnswer serves the registered sub-area and the parent's direct Go
// dispatcher through the same grammar. Bare parent/verdict selects check.
func ApprovalAnswer(args []string) (any, int) { return approvalTable().Answer(args) }

// ApprovalActions publishes this sub-area's actual accepted forms.
func ApprovalActions() leaction.List { return approvalTable().Actions() }

// ApprovalSubs is the registration's derived action listing.
func ApprovalSubs() string { return approvalTable().Subs() }

func approvalTable() leaction.Area {
	return leaction.New(approvalArea,
		leaction.Action{
			Verb: actionReviewCheck, Default: true,
			Summary: "refuse already-spent ordinary approval verdicts",
			Why:     "check captured verdicts against the actual publication parent without changing pending approvals",
			Parameters: []leaction.Parameter{
				{Keyword: keywordParent, Kind: leaction.KindRevision, Value: "commit", Requirement: leaction.Required,
					Help: "the actual full parent commit of this CAS attempt"},
				{Keyword: keywordVerdict, Kind: leaction.KindString, Value: "line", Requirement: leaction.Required, Repeat: true,
					Help: "one captured ordinary approval trailer line"},
			},
			AnswerArgs: withRoot(answerApprovalCheck),
		},
		leaction.Action{
			Verb: approvalClean, Writes: true,
			Summary: "remove exact approval rows after publication",
			Why:     "remove only captured spent rows from the prepared source kind and namespace under the shared process lock",
			Parameters: []leaction.Parameter{
				{Keyword: approvalKindKeyword, Kind: leaction.KindEnum, Value: "source", Requirement: leaction.Required,
					Values: []string{approvalRFCName, approvalWeakeningName}, Help: "the approval source captured during preparation",
					ValueHelp: map[string]string{
						approvalRFCName:       "RFC-tagged test approvals",
						approvalWeakeningName: "test and fixture approvals",
					}},
				{Keyword: keywordSession, Kind: leaction.KindLabel, Value: "id", Requirement: leaction.Required,
					Help: "the captured eight-hex approval namespace"},
				{Keyword: approvalRowKeyword, Kind: leaction.KindString, Value: "line", Requirement: leaction.Required, Repeat: true,
					Help: "one exact whole ledger row consumed by publication"},
			},
			AnswerArgs: withRoot(answerApprovalClean),
		},
	)
}

func answerApprovalClean(root string, args leaction.Arguments) (any, int) {
	session := args.One(keywordSession)
	if !lepath.ValidCommitSession(session) {
		return nil, approvalTable().Refuse(approvalClean, lepath.ErrCommitSessionSpelling)
	}
	kind, err := parseApprovalKind(args.One(approvalKindKeyword))
	if err != nil {
		return nil, approvalTable().Refuse(approvalClean, err)
	}
	rows := args.Values(approvalRowKeyword)
	if err := rfc.PruneApprovals(root, kind.file(session), rows); err != nil {
		leaction.ReportError(opererr.Wrap(err, "approval cleanup failed",
			"the consumed rows could not be safely updated",
			approvalCleanCommand(kind, session, rows)))
		return nil, 1
	}
	return nil, 0
}

func parseApprovalKind(name string) (approvalKind, error) {
	switch name {
	case approvalRFCName:
		return approvalKindRFC, nil
	case approvalWeakeningName:
		return approvalKindWeakening, nil
	default:
		// Open boundary: a direct Go caller may supply a value grammar refused.
		return approvalKindUnspecified, errors.New("approval source must be rfc or weakening, got " + strconv.Quote(name))
	}
}

func (kind approvalKind) name() string {
	switch kind {
	case approvalKindRFC:
		return approvalRFCName
	case approvalKindWeakening:
		return approvalWeakeningName
	case approvalKindUnspecified:
		panic("BUG: unspecified approval kind reached a prepared cleanup")
	default:
		panic("BUG: unknown approval kind reached a prepared cleanup")
	}
}

func (kind approvalKind) file(session string) rfc.ApprovalFile {
	switch kind {
	case approvalKindRFC:
		return rfc.ApprovalFile{Path: rfc.ApprovalPath(session)}
	case approvalKindWeakening:
		return rfc.ApprovalFile{Path: testweakened.ApprovalPath(session)}
	case approvalKindUnspecified:
		panic("BUG: unspecified approval kind reached its ledger selection")
	default:
		panic("BUG: unknown approval kind reached its ledger selection")
	}
}

func approvalCleanCommand(kind approvalKind, session string, rows []string) string {
	// Cleanup can be introduced by the commit that just published. A shared
	// HEAD launcher can still serve its previous binary while rebuilding in
	// the background, so this edge MUST select a synchronous working-tree build.
	// The prepared eight-hex namespace is a safe, stable explicit build name.
	prefix := [...]string{leLauncherPath, "--name", "commit-approval-" + session, area, actionApprovalCheck, approvalClean,
		approvalKindKeyword, kind.name(), keywordSession, session}
	words := make([]string, 0, len(prefix)+2*len(rows))
	words = append(words, prefix[:]...)
	for _, row := range rows {
		words = append(words, approvalRowKeyword, row)
	}
	return quotePaths(words)
}
