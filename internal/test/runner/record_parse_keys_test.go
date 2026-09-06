// VALIDATES: a .ci directive carrying a key the parser does not read fails to
//            parse, and the message names the key.
// PREVENTS: the vacuous-assertion class. `expect=stdout:not-contains=X` used to
//           be accepted in silence -- the key was never read, so the directive
//           recorded no assertion at all and the check passed whatever the
//           command printed. Ten such lines were live across seven .ci files.
//           A test that cannot fail is worse than no test, because it is
//           counted as coverage.

package runner

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// parseCILines writes lines to a .ci file and drives the real entry point.
// parseAndAdd is what discovery calls, so a test that goes through it proves
// the refusal reaches a suite rather than only the switch arm under it.
func parseCILines(t *testing.T, lines ...string) (*Record, error) {
	t.Helper()
	ResetNickCounter()

	dir := t.TempDir()
	ciFile := filepath.Join(dir, "test.ci")
	require.NoError(t, os.WriteFile(ciFile, []byte(strings.Join(lines, "\n")+"\n"), 0o600))

	return NewEncodingTests(dir).parseAndAdd(ciFile)
}

func TestParseCIUnknownKeyIsRefused(t *testing.T) {
	tests := []struct {
		name string
		line string
		// wantIn are substrings the message MUST carry. The offending key is
		// the one an author needs; a bare "parse error" sends them reading the
		// parser instead of their own line.
		wantIn []string
	}{
		{
			name:   "stdout not-contains, the expect=file spelling on a stream",
			line:   "expect=stdout:not-contains=doctor-platform-detect",
			wantIn: []string{"expect=stdout", `"not-contains"`, "reject=<stream>:contains="},
		},
		{
			name:   "stdout !contains, the spelling this vocabulary retired",
			line:   "expect=stdout:!contains=forbidden",
			wantIn: []string{"expect=stdout", `"!contains"`, "reject=<stream>:contains="},
		},
		{
			name:   "stderr has no negative key in either spelling",
			line:   "expect=stderr:not-contains=deprecated",
			wantIn: []string{"expect=stderr", `"not-contains"`, "reject=<stream>:contains="},
		},
		{
			name:   "a plausible wrong key names itself and the accepted keys",
			line:   "expect=stdout:substring=hello",
			wantIn: []string{"expect=stdout", `"substring"`, "accepts contains, pattern"},
		},
		{
			name:   "regex= is documented nowhere the parser reads",
			line:   "expect=stderr:regex=peer .*",
			wantIn: []string{"expect=stderr", `"regex"`},
		},
		{
			name:   "reject arms are checked too",
			line:   "reject=stdout:not-contains=forbidden",
			wantIn: []string{"reject=stdout", `"not-contains"`},
		},
		{
			// The retired key leads, because `!` is not a key-token lead byte:
			// after another key, `:!contains=` is not a boundary and the
			// splitter folds it into the PREVIOUS value (ciformat.go
			// splitOnKeyBoundary). It never reaches the map, so the key check
			// cannot see it. Recorded in
			// plan/journal/silent-drop-in-a-test-directive-parser.md.
			name:   "expect=file keeps not-contains and refuses the stream spelling",
			line:   "expect=file:!contains=forbidden:path=out.txt",
			wantIn: []string{"expect=file", `"!contains"`},
		},
		{
			name:   "expect=exit",
			line:   "expect=exit:code=0:contains=hello",
			wantIn: []string{"expect=exit", `"contains"`, "accepts code"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := parseCILines(t, tt.line)
			require.Error(t, err, "unknown key must fail the file, not be dropped")
			for _, want := range tt.wantIn {
				assert.Contains(t, err.Error(), want)
			}
			// The line number is what turns the message into a location.
			assert.Contains(t, err.Error(), "line 1:")
		})
	}
}

// TestParseCIKnownKeysStillParse is the other polarity: the keys each arm reads
// must survive the check. A guard that refuses everything is as useless as one
// that refuses nothing, and it fails in a direction nobody notices until the
// whole suite is red.
func TestParseCIKnownKeysStillParse(t *testing.T) {
	rec, err := parseCILines(t,
		"expect=stdout:contains=hello",
		`expect=stdout:pattern=version=\d+`,
		"expect=stderr:contains=warning",
		"expect=stderr:pattern=subsystem=server",
		"reject=stdout:contains=forbidden",
		`reject=stdout:pattern=error.*fatal`,
		"reject=stderr:contains=deprecated",
		"reject=stderr:pattern=level=ERROR",
		"expect=syslog:pattern=daemon",
		"expect=file:path=out.txt:not-contains=old:contains=new",
		"expect=exit:code=0",
	)
	require.NoError(t, err)
	require.NotNil(t, rec)

	assert.Equal(t, []string{"hello"}, rec.ExpectStdoutMatch)
	assert.Equal(t, []string{`version=\d+`}, rec.ExpectStdoutRegex)
	assert.Equal(t, []string{"warning"}, rec.ExpectStderrMatch)
	assert.Equal(t, []string{"subsystem=server"}, rec.ExpectStderr)
	assert.Equal(t, []string{"forbidden"}, rec.ExpectStdoutNotMatch)
	assert.Equal(t, []string{`error.*fatal`}, rec.RejectStdoutRegex)
	assert.Equal(t, []string{"deprecated"}, rec.RejectStderrMatch)
	assert.Equal(t, []string{"level=ERROR"}, rec.RejectStderr)
	assert.Equal(t, []string{"daemon"}, rec.ExpectSyslog)
	require.Len(t, rec.FileChecks, 1)
	assert.Equal(t, "old", rec.FileChecks[0].NotContains)
	assert.Equal(t, "new", rec.FileChecks[0].Contains)
}

// TestRejectStderrNeedsAnOperand refuses `reject=stderr` with nothing after it.
// Silence there would be the same defect one level up: a directive that names a
// stream and asserts nothing about it.
func TestRejectStderrNeedsAnOperand(t *testing.T) {
	_, err := parseCILines(t, "reject=stderr")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "reject=stderr needs pattern=")
}

// TestNonContainmentBothPolarities drives the corrected assertion the whole way
// from the .ci line to the verdict, in both directions.
//
// The parse half alone would not have caught the defect this file exists to
// prevent: `expect=stdout:not-contains=` parsed cleanly, produced an empty
// ExpectStdoutNotMatch, and the check below then passed over any output at all.
// So the green case here MUST be green because the needle is absent, never
// because no assertion was recorded -- which is why each case asserts the step
// trace as well as the verdict.
func TestNonContainmentBothPolarities(t *testing.T) {
	tests := []struct {
		name       string
		line       string
		output     string
		wantPassed bool
		wantStep   string
	}{
		{
			name:       "stdout reject holds when the needle is absent",
			line:       "reject=stdout:contains=doctor-vpp-hugepages",
			output:     `{"diagnostics":[]}`,
			wantPassed: true,
			wantStep:   "stdout-not-contains",
		},
		{
			name:       "stdout reject fails when the needle is present",
			line:       "reject=stdout:contains=doctor-vpp-hugepages",
			output:     `{"diagnostics":[{"code":"doctor-vpp-hugepages"}]}`,
			wantPassed: false,
			wantStep:   "stdout-not-contains",
		},
		{
			name:       "stderr reject holds when the needle is absent",
			line:       "reject=stderr:contains=deprecated",
			output:     "usage: ze appliance init|build\n",
			wantPassed: true,
			wantStep:   "stderr-reject-contains",
		},
		{
			name:       "stderr reject fails when the needle is present",
			line:       "reject=stderr:contains=deprecated",
			output:     "warning: ze install appliance is deprecated\n",
			wantPassed: false,
			wantStep:   "stderr-reject-contains",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rec, err := parseCILines(t, tt.line)
			require.NoError(t, err)
			rec.ClientOutput = tt.output

			assert.Equal(t, tt.wantPassed, checkOutputAssertions(rec))

			// One step, and it is the assertion the line declared. An empty
			// trace is the signature of the vacuous parse: no assertion was
			// recorded, so nothing could have failed.
			require.Len(t, rec.StepTrace, 1, "the assertion must reach the step trace")
			assert.Equal(t, tt.wantStep, rec.StepTrace[0].Assert)
			assert.Equal(t, tt.wantPassed, rec.StepTrace[0].Passed)
		})
	}
}
