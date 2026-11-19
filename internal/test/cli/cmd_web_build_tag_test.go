// Design: docs/architecture/testing/runner-architecture.md -- web admission of a needs-build-tag case against the DUT.
package cli

import (
	"os"
	"path/filepath"
	"runtime/debug"
	"slices"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	lepath "github.com/ze-software/ze/internal/le/le/path"
	repofeaturetags "github.com/ze-software/ze/internal/le/repo/featuretags"
	"github.com/ze-software/ze/internal/test/runner"
)

// sampleTag is the development tag a review build adds through ze.tags
// (runner.developmentBuildTags): a .wb may require it, and a run without
// ze.tags builds no DUT with it.
const sampleTag = "ze_fleet_ui_sample"

// dutTagsOf answers the -tags build setting of the running test binary, the
// Go binary this test hands the suite as its DUT.
func dutTagsOf(t *testing.T) []string {
	t.Helper()

	info, ok := debug.ReadBuildInfo()
	require.True(t, ok, "the test binary carries Go build information")
	for _, setting := range info.Settings {
		if setting.Key == "-tags" && setting.Value != "" {
			return strings.Split(setting.Value, ",")
		}
	}
	t.Fatal("the test binary carries no -tags build setting, so it cannot stand in for a DUT whose tags are known; " +
		"run it through ./le test unit, which builds with the feature tags")
	return nil
}

// TestWebSuiteAdmitsNeedsBuildTagAgainstTheDUT runs the suite entry,
// runWebTests, over five cases of a scratch suite directory, with the test
// binary as the DUT: it was built with known tags, so its build information
// is a real DUT's. Three cases declare one needs-build-tag condition, one
// declares none and one an authored skip. Every case asks for the chaos
// dashboard, which this run has no binary for, so a case that runs fails at
// once naming that, and a case that is skipped does not.
//
// VALIDATES: runWebTests admits each case through the DUT's build tags
// (runner.NewDUTAdmission over bins.ze): a case whose condition the DUT
// meets runs, one requiring a tag the DUT lacks is skipped naming the tag
// and the env ze.tags command, one requiring the absence of a tag the DUT
// has is skipped naming the condition, and the skip reaches the case.
// PREVENTS: a wrongly skipped case reported as a pass, and a case that runs
// against a DUT that lacks the tag it needs.
func TestWebSuiteAdmitsNeedsBuildTagAgainstTheDUT(t *testing.T) {
	dutTags := dutTagsOf(t)
	// The web parse reads the manifest of the checkout the run starts in
	// (runner.ParseDUTOption), so the conditions name this checkout's tags.
	root, err := lepath.Root()
	require.NoError(t, err)
	vocabulary, err := repofeaturetags.DaemonTags(root)
	require.NoError(t, err)
	present := ""
	for _, tag := range vocabulary {
		if slices.Contains(dutTags, tag) {
			present = tag
			break
		}
	}
	require.NotEmpty(t, present, "the test binary was built with a tag of feature-gates.txt: %v", dutTags)
	require.False(t, slices.Contains(dutTags, sampleTag),
		"the test binary was built with %s; run this package without ze.tags, so one tag a case may require is absent from the DUT", sampleTag)
	dut, err := os.Executable()
	require.NoError(t, err)

	checkout := t.TempDir()
	suite := filepath.Join(checkout, "test", "web")
	require.NoError(t, os.MkdirAll(suite, 0o700))
	conditions := map[string]string{
		"runs.wb":           present + ":state=present",
		"lacks-tag.wb":      sampleTag + ":state=present",
		"has-unwanted.wb":   present + ":state=absent",
		"no-condition.wb":   "",
		"discovery-skip.wb": "",
	}
	for name, condition := range conditions {
		body := "option=server:kind=chaos\naction=open:path=/\n"
		if condition != "" {
			body = "option=needs-build-tag:value=" + condition + "\n" + body
		}
		if name == "discovery-skip.wb" {
			body = "option=skip:reason=authored skip\n" + body
		}
		require.NoError(t, os.WriteFile(filepath.Join(suite, name), []byte(body), 0o600))
	}

	tests := runner.NewTestSet[*zeTestWebTest]()
	require.NoError(t, discoverWebTests(tests, suite, checkout, webConcurrency))
	tests.EnableAll()
	selected := tests.Selected()
	require.Len(t, selected, len(conditions))
	for _, test := range selected {
		require.False(t, test.admission.ParseFailed, "%s parses: %v", test.Path, test.admission.Error)
	}

	err = runWebTests(t.Context(), selected, zeTestWebBinaries{ze: dut}, checkout, false, true)
	require.ErrorIs(t, err, ErrTestsFailed, "the cases that run fail on the missing chaos binary")

	byName := map[string]*zeTestWebTest{}
	for _, test := range selected {
		byName[filepath.Base(test.Path)] = test
	}
	const ran = "no le binary was built for this run's chaos dashboard"
	for _, name := range []string{"runs.wb", "no-condition.wb"} {
		test := byName[name]
		require.Error(t, test.GetError(), "%s ran", name)
		assert.Contains(t, test.GetError().Error(), ran, "%s reached its server start", name)
		assert.Empty(t, test.skipped, name)
	}

	lacks := byName["lacks-tag.wb"]
	assert.NoError(t, lacks.GetError(), "a case the DUT cannot serve is skipped, never run")
	assert.Contains(t, lacks.skipped, "option=needs-build-tag:value="+sampleTag+":state=present (tag is absent)")
	assert.Contains(t, lacks.skipped, "env ze.tags="+sampleTag, "the skip names how to build the DUT with the tag")
	assert.Contains(t, lacks.skipped, dut, "the skip names the DUT it read")

	unwanted := byName["has-unwanted.wb"]
	assert.NoError(t, unwanted.GetError())
	assert.Contains(t, unwanted.skipped, "option=needs-build-tag:value="+present+":state=absent (tag is present)")

	authored := byName["discovery-skip.wb"]
	assert.NoError(t, authored.GetError())
	assert.Contains(t, authored.skipped, "authored skip", "a discovery skip reaches the case too")
}
