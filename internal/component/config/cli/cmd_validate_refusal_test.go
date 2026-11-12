// Design: docs/architecture/config/yang-config-design.md -- validation CLI
// Related: cmd_validate.go -- runValidation, ValidateContent, refusalDiagnostic

package cli

import (
	"errors"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	configyang "github.com/ze-software/ze/internal/component/config/yang"
	"github.com/ze-software/ze/internal/core/diagnostic"
	"github.com/ze-software/ze/internal/core/opererr"
)

const (
	refusedRouteRefreshProblem = `bgp peer peer1 session capability route-refresh: "requre" is not an accepted value (did you mean require?)`
	refusedRouteRefreshValues  = "route-refresh accepts only these values:"
)

// refusedRouteRefreshFile is validConfig with route-refresh requre on line 17.
var refusedRouteRefreshFile = strings.Replace(validConfig, "\t\tsession {\n",
	"\t\tsession {\n\t\t\tcapability {\n\t\t\t\troute-refresh requre;\n\t\t\t}\n", 1)

// refusedRouteRefreshSet is the same refusal in the set form, on line 2.
const refusedRouteRefreshSet = "set bgp router-id 203.0.113.253\n" +
	"set bgp peer peer1 session capability route-refresh requre\n"

// onlyError answers the one error diagnostic of result.
func onlyError(t *testing.T, result *validationResult) diagnostic.Diagnostic {
	t.Helper()
	var found []diagnostic.Diagnostic
	for i := range result.Diagnostics {
		if result.Diagnostics[i].Severity == diagnostic.SeverityError {
			found = append(found, result.Diagnostics[i])
		}
	}
	require.Len(t, found, 1, "diagnostics: %+v", result.Diagnostics)
	return found[0]
}

// TestRunValidationSetFormatRefusalFollowsTheLine proves a set-format file's
// value refusal names the path right after the line, as a hierarchical file's
// does: the loader's "parse config" segment is daemon start's and reload's.
//
// VALIDATES: AC-24 -- `line N: <path>: ...`, the set-form fix, no `parse
// config:`; the same problem line as the hierarchical file.
// PREVENTS: `line 2: parse config: bgp peer ...`, the loader's context leaking
// into `ze config validate` through ParseTreeForValidation.
func TestRunValidationSetFormatRefusalFollowsTheLine(t *testing.T) {
	set := onlyError(t, runValidation(refusedRouteRefreshSet, "set.conf"))
	assert.Equal(t, 2, set.Line)
	assert.True(t, strings.HasPrefix(set.Message, refusedRouteRefreshProblem+"\n"), "message %q", set.Message)
	assert.NotContains(t, set.Message, "parse config")
	assert.True(t, strings.HasSuffix(set.Message, "\nfix: set bgp peer peer1 session capability route-refresh require"),
		"message %q", set.Message)

	file := onlyError(t, runValidation(refusedRouteRefreshFile, "file.conf"))
	assert.True(t, strings.HasPrefix(file.Message, refusedRouteRefreshProblem+"\n"), "message %q", file.Message)
	assert.NotContains(t, file.Message, "parse config")
}

// TestRunValidationParserEnumDiagnostic proves a parser refusal gives the JSON
// diagnostic the walk gives the same leaf.
//
// VALIDATES: AC-11 -- code config-yang-enum, expected the accepted values in
// declared order, actual the value, the line, the walk-form path, the message
// the operator error with the line left out; the same on the set form.
// PREVENTS: a parser refusal reported as config-parse with no typed fields.
func TestRunValidationParserEnumDiagnostic(t *testing.T) {
	for name, input := range map[string]string{"hierarchical": refusedRouteRefreshFile, "set": refusedRouteRefreshSet} {
		t.Run(name, func(t *testing.T) {
			d := onlyError(t, runValidation(input, "test.conf"))
			assert.Equal(t, diagnostic.CodeConfigYANGEnum, d.Code)
			assert.Equal(t, "enable, disable, require, refuse", d.Expected)
			assert.Equal(t, "requre", d.Actual)
			assert.Positive(t, d.Line)
			assert.Equal(t, "bgp.bgp/peer[peer1]/session/capability/route-refresh", d.Path)
			assert.True(t, strings.HasPrefix(d.Message, refusedRouteRefreshProblem+"\n"+refusedRouteRefreshValues+"\n"),
				"message %q", d.Message)
			assert.NotContains(t, d.Message, "line ")
		})
	}
}

// TestValidateContentFramesEnumRefusal proves ValidateContent writes a
// refusal for a session: each line indented, no line number, the set-form
// fix last, and errors.As reaching the typed refusal.
//
// VALIDATES: AC-8, AC-21 -- `config validation failed:`, the problem, why and
// value lines indented two (values four), `  fix: set bgp peer peer1 session
// capability route-refresh require` last; the ValidationError and the
// operator error recovered from the answer.
// PREVENTS: a session reading the file-form fix `route-refresh require;` or a
// `line N:` that names no line it has.
func TestValidateContentFramesEnumRefusal(t *testing.T) {
	err := ValidateContent(refusedRouteRefreshFile, "session")
	require.Error(t, err)
	text := err.Error()
	assert.True(t, strings.HasPrefix(text, "config validation failed:\n  "+refusedRouteRefreshProblem+"\n  "+
		refusedRouteRefreshValues+"\n    enable"), "text %q", text)
	assert.True(t, strings.HasSuffix(text, "\n  fix: set bgp peer peer1 session capability route-refresh require"),
		"text %q", text)
	assert.NotContains(t, text, "line ")
	assert.NotContains(t, text, "route-refresh require;")

	refusal, ok := errors.AsType[*configyang.ValidationError](err)
	require.True(t, ok, "errors.As finds the ValidationError")
	assert.Equal(t, configyang.ErrTypeEnum, refusal.Type)
	assert.Equal(t, "requre", refusal.Got)
	operator, ok := errors.AsType[*opererr.Error](err)
	require.True(t, ok, "errors.As finds the operator error")
	assert.Equal(t, "set bgp peer peer1 session capability route-refresh require", operator.Fix())
}

// TestValidateContentIndentsEveryLine proves an error that is not a refusal
// keeps its line number and has every line of its block indented.
//
// VALIDATES: AC-8 -- one block per error, every line indented two spaces.
// PREVENTS: a multi-line error's continuation lines starting at column 0,
// where they read as a new error.
func TestValidateContentIndentsEveryLine(t *testing.T) {
	err := ValidateContent("bgp { router-id invalid; }", "bad.conf")
	require.Error(t, err)
	body := strings.TrimPrefix(err.Error(), "config validation failed:\n")
	assert.Contains(t, body, "  line 1: ", "a parse refusal that is no value refusal keeps its line")
	for line := range strings.SplitSeq(body, "\n") {
		assert.True(t, strings.HasPrefix(line, "  "), "line %q", line)
	}
}
