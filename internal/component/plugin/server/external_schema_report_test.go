package server

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	plugin "github.com/ze-software/ze/internal/component/plugin"
	"github.com/ze-software/ze/internal/core/diagnostic"
	"github.com/ze-software/ze/internal/core/opererr"
	"github.com/ze-software/ze/internal/core/report"
)

// issueOf answers the issue of code among issues, and fails the test when
// there is none or more than one.
func issueOf(t *testing.T, issues []report.Issue, code string) report.Issue {
	t.Helper()
	var found []report.Issue
	for _, issue := range issues {
		if issue.Code == code {
			found = append(found, issue)
		}
	}
	require.Len(t, found, 1, "issues of %s", code)
	return found[0]
}

// TestReportUndeclaredSchemaRaisesTheWarning proves that a declared external
// plugin boot read no declaration from reaches doctor: one warning under
// doctor-plugin-schema-undeclared, about that plugin, with the reason and the
// fix of the refusal boot kept for it.
//
// VALIDATES: AC-14 of spec-plugin-external-yang-at-startup, the code of a
// plugin that did not declare, and AC-8's survivor (doctor reports it when
// the config holds no section of it and the daemon runs).
// PREVENTS: the outcome living only in a log line, under the code of a
// stopped plugin, which says nothing about the config schema ze lacks.
func TestReportUndeclaredSchemaRaisesTheWarning(t *testing.T) {
	report.ResetForTest()
	t.Cleanup(report.ResetForTest)
	cfg := plugin.PluginConfig{Name: "absent-plugin", Run: "./no-such-plugin-program"}

	ReportUndeclaredSchema(cfg.Name, notStartedRefusal(cfg, errors.New("fork/exec ./no-such-plugin-program: no such file or directory")))

	issue := issueOf(t, report.Warnings(), diagnostic.CodeDoctorPluginSchemaUndeclared)
	assert.Equal(t, serverEventSource, issue.Source)
	assert.Equal(t, "absent-plugin", issue.Subject)
	assert.Contains(t, issue.Message, `external plugin "absent-plugin" could not be started, so it declared nothing`)
	assert.Contains(t, issue.Message, "refused as an unknown keyword")
	assert.Equal(t, `ze show plugin conformance run "./no-such-plugin-program"`, issue.Fix)
}

// TestReportUndeclaredSchemaOfASchemalessPlugin proves the same code carries
// the third way ze can hold no schema of a plugin: it declared, with no YANG
// module.
//
// VALIDATES: the "declared none" case of doctor-plugin-schema-undeclared.
// PREVENTS: a section written for a plugin that declares no module being
// refused as a bare unknown keyword, with no word about the plugin.
func TestReportUndeclaredSchemaOfASchemalessPlugin(t *testing.T) {
	report.ResetForTest()
	t.Cleanup(report.ResetForTest)
	cfg := plugin.PluginConfig{Name: "no-schema", Run: "./no-schema-plugin"}

	refusal := SchemalessRefusal(cfg)
	ReportUndeclaredSchema(cfg.Name, refusal)

	assert.Equal(t, `external plugin "no-schema" declared no YANG module, so it defines no config section`, refusal.Problem())
	assert.Equal(t, `ze show plugin conformance run "./no-schema-plugin"`, refusal.Fix())
	issue := issueOf(t, report.Warnings(), diagnostic.CodeDoctorPluginSchemaUndeclared)
	assert.Equal(t, "no-schema", issue.Subject)
	assert.Contains(t, issue.Message, "declared no YANG module")
	assert.Equal(t, refusal.Fix(), issue.Fix)
}

// TestReportRefusedSchemaRaisesTheError proves a module the join refused is
// reported under doctor-plugin-schema-refused with the join's own problem and
// fix, and that a refusal with no operator error still names a fix.
//
// VALIDATES: AC-14, the code of a refused module (AC-4, AC-5 name the text).
// PREVENTS: a refused module reported only as "startup failed", with no code
// an operator can look up.
func TestReportRefusedSchemaRaisesTheError(t *testing.T) {
	report.ResetForTest()
	t.Cleanup(report.ResetForTest)
	refusal := opererr.New(
		`plugin "broken-module" declared YANG module "acme-broken-conf", which does not compile`,
		"ze cannot parse a config against a module with no schema",
		`ze show plugin conformance run "./broken"`)

	ReportRefusedSchema(refusal)

	issue := issueOf(t, report.Errors(0), diagnostic.CodeDoctorPluginSchemaRefused)
	assert.Equal(t, serverEventSource, issue.Source)
	assert.Equal(t, schemaJoinSubject, issue.Subject)
	assert.Contains(t, issue.Message, `plugin "broken-module" declared YANG module "acme-broken-conf"`)
	assert.Equal(t, `ze show plugin conformance run "./broken"`, issue.Fix)

	report.ResetForTest()
	ReportRefusedSchema(errors.New("resolve failed"))
	plain := issueOf(t, report.Errors(0), diagnostic.CodeDoctorPluginSchemaRefused)
	assert.Equal(t, "resolve failed", plain.Message)
	assert.Equal(t, fixShowErrors, plain.Fix)
}

// TestExternalConfigRootsFollowTheRunningPlugin proves the roots an external
// plugin declared are answered while it runs and forgotten when its process
// ends, and that a plugin compiled into ze is never recorded.
//
// VALIDATES: AC-14, the population the doctor claims check counts.
// PREVENTS: a root kept as claimed after the plugin that wanted it ended, so
// doctor would stay silent on config that reaches nobody.
func TestExternalConfigRootsFollowTheRunningPlugin(t *testing.T) {
	t.Cleanup(SetExternalConfigRootsForTest(nil))

	recordExternalConfigRoots(plugin.PluginConfig{Name: "acme-monitor", Run: "./acme"}, []string{"acme-monitor", "service/chrony"})
	recordExternalConfigRoots(plugin.PluginConfig{Name: "bgp-rib", Internal: true}, []string{"bgp"})
	recordExternalConfigRoots(plugin.PluginConfig{Name: "wants-nothing", Run: "./quiet"}, nil)

	assert.Equal(t, map[string][]string{"acme-monitor": {"acme-monitor", "service/chrony"}}, ExternalConfigRoots())

	forgetExternalConfigRoots("acme-monitor")
	assert.Empty(t, ExternalConfigRoots())
}
