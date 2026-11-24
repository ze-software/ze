package cli

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/config"
	configyang "github.com/ze-software/ze/internal/component/config/yang"
	"github.com/ze-software/ze/internal/core/diagnostic"
	"github.com/ze-software/ze/pkg/plugin/rpc"
)

// These tests exercise `ze config validate` with a config that declares an
// external plugin under S-3 / AC-18: offline validation queries external
// programs in declare mode (ZE_PLUGIN_MODE=declare) without starting their run
// loop and refuses closed when a program cannot declare; inside a daemon whose
// module set is fixed, no external program is started. None runs in parallel:
// each resets the module set the process published.

// monitorModule is the module of the test plugin: its own root and a leaf
// whose range is 1..10.
const monitorModule = `module acme-monitor-conf {
    namespace "urn:acme:monitor";
    prefix acme;
    container acme-monitor {
        leaf interval { type uint8 { range "1..10"; } }
    }
}`

// monitorSchemaDecl returns the Stage 1 SchemaDecl for monitorModule.
func monitorSchemaDecl() rpc.SchemaDecl {
	return rpc.SchemaDecl{
		Module:    "acme-monitor-conf",
		Namespace: "urn:acme:monitor",
		YANGText:  monitorModule,
	}
}

// markingPlugin writes a program that creates the file `started` beside it
// whenever it is started in any mode, and exits 0 without writing a Stage 1
// declaration. Used by daemon-mode tests where no program may be started at all.
func markingPlugin(t *testing.T) (run, marker string) {
	t.Helper()
	dir := t.TempDir()
	marker = filepath.Join(dir, "started")
	program := filepath.Join(dir, "plugin.sh")
	require.NoError(t, os.WriteFile(program, []byte("#!/bin/sh\necho run >> "+marker+"\n"), 0o600))
	return "sh " + program, marker
}

// silentPlugin writes a program that honors ZE_PLUGIN_MODE=declare by exiting 0
// without writing a Stage 1 declaration (so query mode fails closed) and
// without touching `started`. Any non-declare run writes `started` and exits 1.
func silentPlugin(t *testing.T) (run, marker string) {
	t.Helper()
	dir := t.TempDir()
	marker = filepath.Join(dir, "started")
	program := filepath.Join(dir, "silent.sh")
	script := "#!/bin/sh\nif [ \"$ZE_PLUGIN_MODE\" = \"declare\" ]; then\n  exit 0\nfi\necho run >> " + marker + "\nexit 1\n"
	require.NoError(t, os.WriteFile(program, []byte(script), 0o600))
	return "sh " + program, marker
}

// declaringPlugin writes a program that honors ZE_PLUGIN_MODE=declare by
// writing one valid Stage 1 declare-registration line carrying schemas and
// exiting 0 without touching `started`. Any non-declare run writes `started`
// and exits 1, proving offline query mode never starts the run loop.
func declaringPlugin(t *testing.T, schemas ...rpc.SchemaDecl) (run, marker string) {
	t.Helper()
	payload, err := json.Marshal(rpc.DeclareRegistrationInput{
		ProtocolVersion: 1,
		Schemas:         schemas,
	})
	require.NoError(t, err)
	dir := t.TempDir()
	marker = filepath.Join(dir, "started")
	declFile := filepath.Join(dir, "decl.line")
	line := append([]byte("#1 ze-plugin-engine:declare-registration "), payload...)
	line = append(line, '\n')
	require.NoError(t, os.WriteFile(declFile, line, 0o600))
	program := filepath.Join(dir, "declare.sh")
	script := "#!/bin/sh\nif [ \"$ZE_PLUGIN_MODE\" = \"declare\" ]; then\n  cat " + declFile + "\n  exit 0\nfi\necho run >> " + marker + "\nexit 1\n"
	require.NoError(t, os.WriteFile(program, []byte(script), 0o600))
	return "sh " + program, marker
}

// requireNotStarted fails when the program of markingPlugin was started.
func requireNotStarted(t *testing.T, marker string) {
	t.Helper()
	_, err := os.Stat(marker)
	require.True(t, errors.Is(err, os.ErrNotExist), "validate started the declared program (marker %s, stat error %v)", marker, err)
}

// monitorConfig answers a config that declares external plugin acme with run,
// followed by section.
func monitorConfig(run, section string) string {
	return "bus {\n\tendpoint acme {\n\t\texternal {\n\t\t\trun \"" + run + "\"\n\t\t}\n\t\tencoding json\n\t}\n}\n\n" + section
}

// messagesOf answers the messages of result's diagnostics of one severity,
// one per line.
func messagesOf(result *validationResult, severity diagnostic.Severity) string {
	var messages []string
	for i := range result.Diagnostics {
		if result.Diagnostics[i].Severity == severity {
			messages = append(messages, result.Diagnostics[i].Message)
		}
	}
	return strings.Join(messages, "\n")
}

// TestValidateRefusesSectionOfExternalPluginWithoutStartingIt verifies S-3 /
// AC-18 at runValidation: offline validation queries declared external programs
// in declare mode (ZE_PLUGIN_MODE=declare) without starting their run loop,
// validates their declared YANG module when they declare, and refuses closed
// with the conformance command when a program cannot declare.
//
// VALIDATES: S-3 / AC-18 / S16 — declare-mode schema validation accepts valid
// values and refuses out-of-range values without touching the run-loop marker,
// while a non-declaring program is refused naming endpoint, run line, cause,
// and `ze show plugin conformance run "<run>"`.
// PREVENTS: offline validation starting an external program's run loop or
// accepting a program that cannot declare.
func TestValidateRefusesSectionOfExternalPluginWithoutStartingIt(t *testing.T) {
	t.Run("non_declaring_program_refused", func(t *testing.T) {
		t.Cleanup(config.ResetDeclaredSchemasForTest())
		run, marker := silentPlugin(t)

		result := runValidation(monitorConfig(run, "acme-monitor {\n\tinterval 5\n}\n"), "ze.conf")

		require.False(t, result.Valid, "a program that cannot declare must be refused")
		require.NotEmpty(t, result.Diagnostics)
		assert.Contains(t, result.Diagnostics[0].Message, `bus endpoint "acme"`)
		assert.Contains(t, result.Diagnostics[0].Message, run)
		assert.Contains(t, result.Diagnostics[0].Message, "without writing a Stage 1 declaration")
		assert.Equal(t, `ze show plugin conformance run "`+run+`"`, result.Diagnostics[0].Fix)
		requireNotStarted(t, marker)
	})

	t.Run("declaring_program_validated_without_run_loop", func(t *testing.T) {
		t.Cleanup(config.ResetDeclaredSchemasForTest())
		run, marker := declaringPlugin(t, monitorSchemaDecl())

		valid := runValidation(monitorConfig(run, "acme-monitor {\n\tinterval 5\n}\n"), "ze.conf")
		require.True(t, valid.Valid, "valid section of declare-mode plugin refused:\n%s", messagesOf(valid, diagnostic.SeverityError))
		assert.Equal(t, []string{"acme"}, valid.Queried)

		refused := runValidation(monitorConfig(run, "acme-monitor {\n\tinterval 11\n}\n"), "ze.conf")
		require.False(t, refused.Valid, "out-of-range interval must be refused")
		assert.Contains(t, messagesOf(refused, diagnostic.SeverityError), "interval")
		requireNotStarted(t, marker)
	})
}

// TestValidateAcceptsConfigWithNoSectionOfExternalPlugin: a declared external
// program that honors declare mode validates cleanly when the config holds no
// section of it, without starting its run loop.
//
// VALIDATES: S-3 / AC-18 — declare-mode query succeeds with Queried=["acme"],
// no warning about acme, and run-loop marker untouched.
// PREVENTS: validate refusing, or warning on, a valid declare-mode external
// endpoint with no extra config section.
func TestValidateAcceptsConfigWithNoSectionOfExternalPlugin(t *testing.T) {
	t.Cleanup(config.ResetDeclaredSchemasForTest())
	run, marker := declaringPlugin(t)

	result := runValidation(monitorConfig(run, ""), "ze.conf")

	require.True(t, result.Valid, "a config with a declaring plugin is refused:\n%s", messagesOf(result, diagnostic.SeverityError))
	assert.Equal(t, []string{"acme"}, result.Queried)
	assert.NotContains(t, messagesOf(result, diagnostic.SeverityWarning), "acme")
	requireNotStarted(t, marker)
}

// TestValidateJudgesSectionWithThePublishedModule: in a process that holds the
// plugin's module, as a daemon does after boot read it from the plugin's live
// Stage 1, validate judges the section with that module. This is the answer
// `show config validation` gives when a running ze serves it. Method: publish
// the module for plugin acme, then the last valid value of range 1..10 and the
// first invalid one on each side.
//
// VALIDATES: AC-2 and AC-10 at the validate entry for a served command:
// interval 10 is valid, 0 and 11 are refused naming the leaf, none is an
// unknown keyword, and no program is started.
// PREVENTS: a daemon refusing a section of a plugin it started with, or
// accepting a value the plugin's module forbids.
func TestValidateJudgesSectionWithThePublishedModule(t *testing.T) {
	t.Cleanup(config.ResetDeclaredSchemasForTest())
	run, marker := markingPlugin(t)
	require.NoError(t, config.PublishExternalModules([]configyang.ExternalModule{{
		Plugin: "acme", Module: "acme-monitor-conf", Namespace: "urn:acme:monitor", Text: monitorModule,
	}}))

	valid := runValidation(monitorConfig(run, "acme-monitor {\n\tinterval 10\n}\n"), "ze.conf")
	require.True(t, valid.Valid, "interval 10 is refused:\n%s", messagesOf(valid, diagnostic.SeverityError))

	for _, value := range []string{"0", "11"} {
		refused := runValidation(monitorConfig(run, "acme-monitor {\n\tinterval "+value+"\n}\n"), "ze.conf")
		require.False(t, refused.Valid, "interval %s is outside range 1..10 and was accepted", value)
		errs := messagesOf(refused, diagnostic.SeverityError)
		assert.Contains(t, errs, "interval")
		assert.NotContains(t, errs, "unknown top-level keyword")
		assert.NotContains(t, errs, "fix: ze cli")
	}
	requireNotStarted(t, marker)
}
