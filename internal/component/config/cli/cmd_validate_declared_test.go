package cli

import (
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
)

// These tests give `ze config validate` a config that declares an external
// plugin, with a real program behind the run string: a shell script that
// leaves a marker file when it is started, in any mode. Validate must never
// start it. None runs in parallel: each resets the module set the process
// published.

// monitorModule is the module of the test plugin: its own root and a leaf
// whose range is 1..10.
const monitorModule = `module acme-monitor-conf {
    namespace "urn:acme:monitor";
    prefix acme;
    container acme-monitor {
        leaf interval { type uint8 { range "1..10"; } }
    }
}`

// markingPlugin writes a program that creates the file `started` beside it
// when it is started, and answers its run string and the marker's path.
func markingPlugin(t *testing.T) (run, marker string) {
	t.Helper()
	dir := t.TempDir()
	marker = filepath.Join(dir, "started")
	program := filepath.Join(dir, "plugin.sh")
	require.NoError(t, os.WriteFile(program, []byte("#!/bin/sh\necho run >> "+marker+"\n"), 0o600))
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
	return "plugin {\n\texternal acme {\n\t\trun \"" + run + "\"\n\t\tencoder json\n\t}\n}\n\n" + section
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

// TestValidateRefusesSectionOfExternalPluginWithoutStartingIt: with no daemon,
// `ze config validate` refuses a section only an external plugin's schema
// defines, says why, names the command that reaches a running ze, and starts
// no program. Method: a run string whose program leaves a marker.
//
// VALIDATES: the owner ruling that what needs an external plugin is not
// available with no daemon: the config is not valid, the diagnostics hold the
// unknown keyword, the declared plugin's name, that this command starts no
// plugin, the daemon form `show config validation ze.conf`, and `fix: ze cli`
// last; the marker is absent.
// PREVENTS: validate running the command a config names, validate passing a
// section it could not check, and a bare "unknown top-level keyword".
func TestValidateRefusesSectionOfExternalPluginWithoutStartingIt(t *testing.T) {
	t.Cleanup(config.ResetDeclaredSchemasForTest())
	run, marker := markingPlugin(t)

	result := runValidation(monitorConfig(run, "acme-monitor {\n\tinterval 5\n}\n"), "ze.conf")

	require.False(t, result.Valid, "a section ze could not check was accepted")
	errs := messagesOf(result, diagnostic.SeverityError)
	assert.Contains(t, errs, "unknown top-level keyword: acme-monitor")
	assert.Contains(t, errs, `the config declares external plugin "acme"`)
	assert.Contains(t, errs, "starts no plugin")
	assert.Contains(t, errs, "type `show config validation ze.conf`")
	assert.True(t, strings.HasSuffix(errs, "fix: ze cli"), "the command that reaches a running ze is not the last line:\n%s", errs)
	assert.Equal(t, 1, strings.Count(errs, "fix: "), "one fix line:\n%s", errs)
	requireNotStarted(t, marker)
}

// TestValidateAcceptsConfigWithNoSectionOfExternalPlugin: a declared external
// plugin refuses nothing when the config holds no name only its schema could
// define. Method: the same declaration and no section.
//
// VALIDATES: the config is valid, with no diagnostic about the plugin, and
// the program was not started.
// PREVENTS: validate refusing, or warning on, every config that declares an
// external plugin, which boot accepts.
func TestValidateAcceptsConfigWithNoSectionOfExternalPlugin(t *testing.T) {
	t.Cleanup(config.ResetDeclaredSchemasForTest())
	run, marker := markingPlugin(t)

	result := runValidation(monitorConfig(run, ""), "ze.conf")

	require.True(t, result.Valid, "a config with no section of the plugin is refused:\n%s", messagesOf(result, diagnostic.SeverityError))
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
