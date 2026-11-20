// Design: docs/architecture/core-design.md -- le's composition, one import per tool
// Related: actions.go -- the table these answers are registered in
//
// answers.go adapts the documents, check and judge actions to the le action
// table: each resolves the checkout, reads the spec and answers one document.
package specdesign

import (
	"github.com/ze-software/ze/internal/core/textbuf"
	leaction "github.com/ze-software/ze/internal/le/le/action"
	lepath "github.com/ze-software/ze/internal/le/le/path"
)

// errorExit is the code of an action that could not answer: a checkout, a
// page or a record it could not read.
const errorExit = 2

// reportError prints why an action could not answer and answers its code.
func reportError(err error) int {
	var tb textbuf.Buffer
	tb.Str("error: le spec design: ").Err(err).Byte('\n').StdErr() //nolint:errcheck // CLI output
	return errorExit
}

// answerDocuments is `le spec design documents spec <path>`.
func answerDocuments(args leaction.Arguments) (any, int) {
	root, err := lepath.Root()
	if err != nil {
		return nil, reportError(err)
	}
	spec, err := loadSpec(root, args.One(specParameter.Keyword))
	if err != nil {
		return nil, reportError(err)
	}
	derived, err := derive(root, spec)
	if err != nil {
		return nil, reportError(err)
	}
	return derived, 0
}

// answerCheck is `le spec design check spec <path>`: exit 0 and "ready to
// present" when nothing is owed, exit 1 with one line per problem otherwise.
func answerCheck(args leaction.Arguments) (any, int) {
	root, err := lepath.Root()
	if err != nil {
		return nil, reportError(err)
	}
	spec, err := loadSpec(root, args.One(specParameter.Keyword))
	if err != nil {
		return nil, reportError(err)
	}
	derived, err := derive(root, spec)
	if err != nil {
		return nil, reportError(err)
	}
	problems, err := checkSpec(root, spec, derived, judgeStepOwed)
	if err != nil {
		return nil, reportError(err)
	}
	if len(problems) == 0 {
		return checkAnswer{Spec: spec.relative, Status: readyToPresent}, 0
	}
	problems = append(problems, listCommand(spec))
	return checkAnswer{Spec: spec.relative, Status: "owes", Problems: problems}, 1
}

// answerJudge is `le spec design judge spec <path> verdict <word> reason
// "<sentence>"`.
func answerJudge(args leaction.Arguments) (any, int) {
	root, err := lepath.Root()
	if err != nil {
		return nil, reportError(err)
	}
	return judge(root, args.One(specParameter.Keyword), args.One("verdict"), args.One("reason"))
}
