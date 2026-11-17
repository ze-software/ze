// Design: docs/architecture/config/yang-config-design.md — `ze config validate` asks the running ze first
// Overview: daemon_route.go — the route to the running ze every local config command shares
// Overview: cmd_validate.go — cmdValidate, which prints the verdict this file answers
// Related: config_supplied.go — suppliedValidation, the daemon command this file sends
// Related: config_data.go — dataValidate, the daemon command a config too large to send is named to
//
// `ze config validate` asks the running ze about a config that declares an
// external plugin (daemon_route.go says why), with the file's bytes in the
// `content` leaf of `show config supplied validation`, and draws the daemon's
// verdict with its own renderer and exit codes. The daemon answers a config it
// judges invalid with a failed status and the verdict document beside it, so
// the document is the verdict, and a failed status with no document is no
// answer.

package cli

import (
	"encoding/json"

	"github.com/ze-software/ze/internal/component/config"
	"github.com/ze-software/ze/internal/core/diagnostic"
)

// The two daemon commands `ze config validate` sends, as
// ze-config-cli-cmd.yang declares them. pathSuppliedValidation takes the
// config's bytes (its keywords are suppliedContent and suppliedNameArg, which
// suppliedHandler reads); pathShowConfigValidation takes a file the daemon
// reads itself, in its `file` leaf (dataValidate). suppliedQuestion.line
// judges the supplied form by its model before it is sent, so a spelling here
// that drifted from the model is refused on this side.
const (
	pathSuppliedValidation   = "show config supplied validation"
	pathShowConfigValidation = "show config validation"
	leafFile                 = "file"
	localValidate            = "ze config validate"
	placeholderConfigFile    = "<config-file>"
)

// validationQuestion is what `ze config validate` asks the running ze about
// input, the config read from path.
func validationQuestion(input, path string) suppliedQuestion {
	question := suppliedQuestion{
		supplied:     pathSuppliedValidation,
		content:      []byte(input),
		labelPath:    path,
		byPath:       pathShowConfigValidation,
		fileLeaf:     true,
		local:        localValidate,
		operands:     placeholderConfigFile,
		subject:      configSubject(path),
		placeholders: []string{placeholderConfigFile},
	}
	if namesConfigFile(path) {
		question.files = []string{path}
		question.operands = path
	}
	return question
}

// judgeConfig answers the verdict `ze config validate` prints for input, the
// config read from path.
//
// A config that declares an external plugin this process holds no schema of
// goes to the running ze first, and its answer is the verdict. When no ze
// answers, or the config declares no such plugin, the verdict is this
// process's own (runValidation), which says why no ze answered beside the
// refusal of a name no compiled-in schema defines.
func judgeConfig(input, path, user string, ask daemonAsk) *validationResult {
	question := validationQuestion(input, path)
	// A schema that does not build asks nobody: runValidation reports it.
	if !question.needsDaemon([]config.ConfigSource{{Text: input, Path: path}}) {
		return runValidation(input, path)
	}
	result, err := askDaemonValidation(question, path, user, ask)
	if err == nil {
		return result
	}
	unanswered := question.unanswered(err)
	return runValidationUnanswered(input, path, &unanswered)
}

// askDaemonValidation sends question to the running ze and answers its
// verdict, named for path as the local verdict is.
func askDaemonValidation(question suppliedQuestion, path, user string, ask daemonAsk) (*validationResult, error) {
	line, err := question.line()
	if err != nil {
		return nil, err
	}
	verdict, err := ask(user, line)
	if err != nil {
		return nil, err
	}
	return decodeDaemonValidation(verdict, path, question.asked(line))
}

// decodeDaemonValidation reads what the daemon sent back for the command
// asked as a verdict named for path. Anything that is not a validation result
// this ze reads (diagnostic.DecodeValidateResult) is no answer, and neither is
// a document that disagrees with the status beside it (a valid verdict under a
// failed status, an invalid one under none): drawing it would print a verdict
// the daemon never gave. A daemon that refused the command itself (a denied
// account, an argument it does not know, a daemon that predates the command)
// sent no document, so it lands here too and is no answer.
func decodeDaemonValidation(verdict daemonVerdict, path, asked string) (*validationResult, error) {
	document, err := diagnostic.DecodeValidateResult(verdict.body)
	if err != nil {
		return nil, foreignAnswer(verdict.address, asked, err.Error())
	}
	if document.Valid == verdict.refused {
		return nil, foreignAnswer(verdict.address, asked, "a verdict that disagrees with the status beside it")
	}
	result := &validationResult{
		Valid:       document.Valid,
		Path:        path,
		Diagnostics: document.Diagnostics,
	}
	// A verdict of a config with no bgp block carries no summary, and the
	// renderer draws none.
	if document.Config == nil {
		return result, nil
	}
	summary, err := daemonSummary(document.Config)
	if err != nil {
		return nil, foreignAnswer(verdict.address, asked, "a config summary this ze does not read ("+err.Error()+")")
	}
	result.Config = summary
	return result, nil
}

// daemonSummary reads the summary of a daemon's verdict, which arrives as
// decoded JSON, into the type the local renderer draws.
func daemonSummary(decoded any) (*validationSummary, error) {
	encoded, err := json.Marshal(decoded)
	if err != nil {
		return nil, err
	}
	var summary validationSummary
	if err := json.Unmarshal(encoded, &summary); err != nil {
		return nil, err
	}
	return &summary, nil
}
