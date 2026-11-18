// Design: docs/architecture/api/commands.md -- detached file transformations.
// Related: cmd_set.go -- local publication after the shared set operation.
// Related: cmd_deactivate.go -- local activation publication.

package cli

import (
	"bytes"
	"errors"
	"slices"
	"strings"

	editor "github.com/ze-software/ze/internal/component/cli"
	"github.com/ze-software/ze/internal/component/command"
	configyang "github.com/ze-software/ze/internal/component/config/yang"
	"github.com/ze-software/ze/internal/core/opererr"
	"github.com/ze-software/ze/internal/core/textbuf"
)

func dataSet(args command.ValidatedArgs) command.LocalAnswer {
	return fileToolAnswer(args, "show config set", fileSet)
}

func dataActivate(args command.ValidatedArgs) command.LocalAnswer {
	return fileToolAnswer(args, "show config activate", fileActivate)
}

func dataDeactivate(args command.ValidatedArgs) command.LocalAnswer {
	return fileToolAnswer(args, "show config deactivate", fileDeactivate)
}

func dataReplace(args command.ValidatedArgs) command.LocalAnswer {
	return fileToolAnswer(args, "show config replace", fileReplace)
}

func fileSet(args command.ValidatedArgs, before []byte, name string) command.LocalAnswer {
	return fileMutation(args, before, name, detachedSet)
}

func fileActivate(args command.ValidatedArgs, before []byte, name string) command.LocalAnswer {
	return fileMutation(args, before, name, detachedActivate)
}

func fileDeactivate(args command.ValidatedArgs, before []byte, name string) command.LocalAnswer {
	return fileMutation(args, before, name, detachedDeactivate)
}

func fileMutation(args command.ValidatedArgs, before []byte, name string, mutation detachedMutation) command.LocalAnswer {
	path := "show config " + mutation.verb()
	encoded, found := args.Keyed("content")
	if !found {
		return suppliedRefusal(path, "the editing command content is missing", nil)
	}
	statement, err := command.DecodeContent(encoded)
	if err != nil {
		return suppliedRefusal(path, "the editing command could not be decoded", err)
	}
	return detachedMutationAnswer(args, before, statement, name, mutation)
}

func fileReplace(args command.ValidatedArgs, before []byte, name string) command.LocalAnswer {
	encoded, found := args.Keyed("content")
	if !found {
		return suppliedRefusal("show config replace", "the replacement content is missing", nil)
	}
	after, err := command.DecodeContent(encoded)
	if err != nil {
		return suppliedRefusal("show config replace", "the replacement could not be decoded", err)
	}
	return detachedReplaceAnswer(args, before, after, name)
}

type detachedMutation uint8

const (
	detachedMutationUnspecified detachedMutation = iota
	detachedSet
	detachedActivate
	detachedDeactivate
)

func (mutation detachedMutation) verb() string {
	switch mutation {
	case detachedMutationUnspecified:
		panic("BUG: unspecified detached mutation")
	case detachedSet:
		return "set"
	case detachedActivate:
		return "activate"
	case detachedDeactivate:
		return "deactivate"
	}
	panic("BUG: unknown detached mutation")
}

func suppliedSet(args command.ValidatedArgs, data []byte, name string) command.LocalAnswer {
	return suppliedMutation(args, data, name, detachedSet)
}

func suppliedActivate(args command.ValidatedArgs, data []byte, name string) command.LocalAnswer {
	return suppliedMutation(args, data, name, detachedActivate)
}

func suppliedDeactivate(args command.ValidatedArgs, data []byte, name string) command.LocalAnswer {
	return suppliedMutation(args, data, name, detachedDeactivate)
}

func suppliedMutation(args command.ValidatedArgs, data []byte, name string, mutation detachedMutation) command.LocalAnswer {
	before, statement, err := suppliedPair(data)
	if err != nil {
		return suppliedRefusal("show config supplied "+mutation.verb(), "content must contain before configuration and after command strings", err)
	}
	return detachedMutationAnswer(args, before, statement, name, mutation)
}

func detachedStatement(statement []byte, mutation detachedMutation) ([]string, error) {
	return detachedEditingStatement(statement, mutation.verb(), "ze show config supplied "+mutation.verb()+" help")
}

func detachedEditingStatement(statement []byte, verb, help string) ([]string, error) {
	text := strings.TrimSpace(string(statement))
	if strings.ContainsAny(text, "\r\n") {
		return nil, opererr.New("the detached change contains more than one line",
			"Only one editing command is accepted, and no operational command is dispatched.",
			help)
	}
	words, err := editor.ParseCommandWords(text)
	if err != nil {
		return nil, err
	}
	if words.FirstPipe() >= 0 {
		return nil, opererr.New("the detached change contains a pipe",
			"The input is one editing command; output pipes belong after the inspection command.",
			help)
	}
	tokens := words.Tokens()
	if len(tokens) < 2 {
		return nil, opererr.New("the detached change has no complete path",
			"Give one editing command with its complete configuration path.",
			help)
	}
	if tokens[0] != verb {
		return nil, opererr.New("the detached change does not start with "+verb,
			"This operation accepts only its declared editing verb, never a different command.",
			help)
	}
	return tokens[1:], nil
}

func detachedMutationAnswer(args command.ValidatedArgs, before, statement []byte, name string, mutation detachedMutation) (answer command.LocalAnswer) {
	path := "show config supplied " + mutation.verb()
	tokens, err := detachedStatement(statement, mutation)
	if err != nil {
		return suppliedRefusal(path, "the editing command was refused", err)
	}
	ed, err := editor.NewEditorFromContent(before, name)
	if err != nil {
		return suppliedRefusal(path, "the source could not be opened", err)
	}
	defer func() {
		if err := ed.Close(); err != nil {
			answer = suppliedRefusal(path, "the detached editor could not be closed", err)
		}
	}()
	if err := applyDetachedMutation(ed, tokens, mutation); err != nil {
		return command.LocalRefusal(exitError, command.LocalErrorLine(err))
	}
	return detachedEditorAnswer(path, ed, args.Flag("dry-run"))
}

// applyDetachedMutation is the native statement producer shared by detached
// files, supplied content and privately owned backup configurations.
func applyDetachedMutation(ed *editor.Editor, tokens []string, mutation detachedMutation) *opererr.Error {
	switch mutation {
	case detachedMutationUnspecified:
		panic("BUG: unspecified detached mutation")
	case detachedSet:
		return applyConfigSet(ed, tokens)
	case detachedActivate:
		_, err := applyConfigActivation(ed, tokens, true)
		return err
	case detachedDeactivate:
		_, err := applyConfigActivation(ed, tokens, false)
		return err
	}
	panic("BUG: unknown detached mutation")
}

// applyConfigSet is the single local/supplied value and key judgment. A source
// that cannot parse is refused before a lenient editor could drop its content.
func applyConfigSet(ed *editor.Editor, tokens []string) *opererr.Error {
	if len(tokens) < 2 {
		return opererr.New("set needs a path and value", "No configuration was changed.", "ze config set help")
	}
	if _, err := ed.ParseLoad(ed.OriginalContent(), nil); err != nil {
		if native, known := err.(*opererr.Error); known { //nolint:errorlint // ParseLoad produces its complete native refusal directly.
			return native
		}
		return opererr.Wrap(err, "the source configuration could not be parsed",
			"No configuration was changed. Repair the source syntax before retrying.", "ze config validate help")
	}
	value := tokens[len(tokens)-1]
	path := tokens[:len(tokens)-1]
	completer, err := editor.NewCompleter()
	if err != nil {
		return opererr.Wrap(err, "the configuration schema could not be loaded",
			"No configuration was changed. Inspect the available schema before retrying.", "ze schema list")
	}
	if err := completer.ValidateValueAtPath(path, value); err != nil {
		if native, found := errors.AsType[*opererr.Error](err); found {
			// The validator already owns all three refusal parts. Mask each
			// without retaining an unmasked cause or framing a second fix.
			return opererr.New(
				ed.DisplayMessageAtPath(path, native.Problem(), value),
				ed.DisplayMessageAtPath(path, native.Why(), value),
				ed.DisplayMessageAtPath(path, native.Fix(), value))
		}
		return opererr.New(ed.DisplayMessageAtPath(path, err.Error(), value),
			"The value does not satisfy the configuration schema; the source was not changed.",
			"ze show config supplied set help")
	}
	if err := ed.SetValue(path[:len(path)-1], path[len(path)-1], value); err != nil {
		display := slices.Clone(tokens)
		display[len(display)-1] = ed.DisplayValueAtPath(path, value)
		var placeholders []int
		if display[len(display)-1] != value {
			placeholders = append(placeholders, len(display)-1)
		}
		refusal := editor.FramedKeyRefusal(err, "set", nil, display, nil, placeholders...)
		var native *opererr.Error
		var prefix textbuf.Buffer
		switch failure := refusal.(type) { //nolint:errorlint // Only direct native and schema-owned value refusals may supply their parts.
		case *opererr.Error:
			native = failure
		case *configyang.ValidationError:
			native, _ = failure.Unwrap().(*opererr.Error) //nolint:errorlint // This schema wrapper holds its direct native refusal, or none.
			if failure.LineNumber > 0 {
				prefix.Str("line ").Int(int64(failure.LineNumber)).Str(": ")
			}
		}
		if native != nil {
			why := native.Why()
			if cause := native.Unwrap(); cause != nil {
				why += ": " + cause.Error()
			}
			return opererr.New(
				ed.DisplayMessageAtPath(path, prefix.Str(native.Problem()).String(), value),
				ed.DisplayMessageAtPath(path, why, value),
				ed.DisplayMessageAtPath(path, native.Fix(), value))
		}
		return opererr.New(ed.DisplayMessageAtPath(path, refusal.Error(), value),
			"The setting was refused; no successful change was confirmed.", "ze show config supplied set help")
	}
	return nil
}

func applyConfigActivation(ed *editor.Editor, path []string, activate bool) (string, *opererr.Error) {
	if _, err := ed.ParseLoad(ed.OriginalContent(), nil); err != nil {
		if native, known := err.(*opererr.Error); known { //nolint:errorlint // ParseLoad owns and directly returns its complete refusal.
			return "", native
		}
		return "", opererr.Wrap(err, "the source configuration could not be parsed",
			"No configuration was changed. Repair the source syntax before retrying.", "ze config validate help")
	}
	status, err := ed.ApplyActivation(path, activate)
	if err == nil {
		return status, nil
	}
	if native, known := err.(*opererr.Error); known { //nolint:errorlint // ApplyActivation owns its verb context and complete refusal.
		return "", native
	}
	if activation, known := err.(*editor.ActivationError); known { //nolint:errorlint // Project this exact producer while it retains the typed key for contextual editors.
		if native := activation.OperatorError(); native != nil {
			return "", native
		}
	}
	verb := "deactivate"
	if activate {
		verb = "activate"
	}
	return "", opererr.Wrap(err, "the activation change could not finish",
		"No successful change was confirmed. Inspect the target before retrying.", "ze show config supplied "+verb+" help")
}

func detachedEditorAnswer(path string, ed *editor.Editor, preview bool) command.LocalAnswer {
	if preview {
		return command.LocalAnswer{Payload: command.Blob(ed.Diff())}
	}
	content, warnings, err := serializeDetachedEditor(ed)
	if err != nil {
		return suppliedRefusal(path, "the transformed source could not be serialized", err)
	}
	var diag textbuf.Buffer
	for _, warning := range warnings {
		diag.Str("warning: ").Str(warning).Byte('\n')
	}
	return command.LocalAnswer{Payload: command.Blob(content), Diagnostic: diag.String()}
}

// serializeDetachedEditor returns complete native bytes for adapters that
// either publish them or render them. A masked display is never an input.
func serializeDetachedEditor(ed *editor.Editor) ([]byte, []string, error) {
	var output bytes.Buffer
	ed.SetStdoutSink(&output)
	warnings, err := ed.Save()
	return output.Bytes(), warnings, err
}

func suppliedReplace(args command.ValidatedArgs, data []byte, name string) command.LocalAnswer {
	before, after, err := suppliedPair(data)
	if err != nil {
		return suppliedRefusal("show config supplied replace", "content must contain before and after configuration strings", err)
	}
	return detachedReplaceAnswer(args, before, after, name)
}

func detachedReplaceAnswer(args command.ValidatedArgs, before, after []byte, name string) (answer command.LocalAnswer) {
	const path = "show config supplied replace"
	ed, warnings, err := prepareDetachedReplacement(before, after, name)
	if err != nil {
		return suppliedRefusal(path, "the replacement could not be prepared", err)
	}
	defer func() {
		if err := ed.Close(); err != nil {
			answer = suppliedRefusal(path, "the detached editor could not be closed", err)
		}
	}()
	answer = detachedEditorAnswer(path, ed, args.Flag("dry-run"))
	var diag textbuf.Buffer
	for _, warning := range warnings {
		diag.Str("warning: ").Str(warning).Byte('\n')
	}
	answer.Diagnostic = diag.Str(answer.Diagnostic).String()
	return answer
}

// prepareDetachedReplacement owns validation and tree replacement for both
// supplied bytes and an owner-held artifact. The caller MUST close the editor.
func prepareDetachedReplacement(before, after []byte, name string) (*editor.Editor, []string, error) {
	validator, err := editor.NewConfigValidator()
	if err != nil {
		return nil, nil, err
	}
	validation := validator.ValidateTransition(string(before), string(after))
	if validation.HasErrors() {
		var reason textbuf.Buffer
		for _, problem := range validation.Errors[:min(len(validation.Errors), 32)] {
			reason.Str(problem.Text(nil)).Byte('\n')
		}
		return nil, nil, errors.New(reason.String())
	}
	ed, err := editor.NewEditorFromContent(before, name)
	if err != nil {
		return nil, nil, err
	}
	loaded, err := ed.ParseLoad(string(after), nil)
	if err != nil {
		return nil, nil, errors.Join(err, ed.Close())
	}
	if err := ed.LoadReplace(nil, loaded); err != nil {
		return nil, nil, errors.Join(err, ed.Close())
	}
	warnings := make([]string, 0, len(validation.Warnings))
	for _, warning := range validation.Warnings {
		warnings = append(warnings, warning.Text(nil))
	}
	return ed, warnings, nil
}
