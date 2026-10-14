// VALIDATES: the decode suite runs the exec= line a .ci names, word for word,
//            with the stdin= block it names piped to the process.
// PREVENTS: the 2026-09-07 journal row: exec= was parsed for its flags and a
//           different argv was built and run, so the "-" on every line was
//           decoration and the command text was a comment nobody ran.

package runner

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// echoZeScript stands in for ze: it prints its argv and its standard input as
// one JSON object, so the expectation pins exactly what the runner executed.
const echoZeScript = `#!/bin/sh
stdin=$(cat)
printf '{"args":"%s","stdin":"%s"}\n' "$*" "$stdin"
`

// TestDecodeRunnerExecutesExecLine drives runTest over a .ci whose expectation
// is the argv and stdin a literal execution of its exec= line produces.
func TestDecodeRunnerExecutesExecLine(t *testing.T) {
	dir := t.TempDir()
	zePath := filepath.Join(dir, "ze")
	require.NoError(t, os.WriteFile(zePath, []byte(echoZeScript), 0o700)) //nolint:gosec // test stand-in must be executable
	ci := "stdin=payload:hex=FFFF\n" +
		"cmd=foreground:seq=1:exec=ze bgp decode --json --family ipv4/unicast -:stdin=payload\n" +
		`expect=json:json={"args":"bgp decode --json --family ipv4/unicast -","stdin":"FFFF"}` + "\n"
	file := filepath.Join(dir, "literal.ci")
	require.NoError(t, os.WriteFile(file, []byte(ci), 0o600))

	dt := NewDecodingTests(dir)
	require.NoError(t, dt.Discover(dir))
	require.Equal(t, 1, dt.Count())
	test := dt.Registered()[0]
	require.NoError(t, test.ParseError)

	r := NewDecodingRunner(dt, dir, zePath)
	assert.True(t, r.runTest(context.Background(), test), "runner must execute exec= as written: %v", test.Error)
}

// TestDecodeRunnerFailsWhenCommandRefuses proves a command that exits non-zero,
// as ze does for a flag it does not know, fails the test rather than passing.
func TestDecodeRunnerFailsWhenCommandRefuses(t *testing.T) {
	dir := t.TempDir()
	zePath := filepath.Join(dir, "ze")
	refuse := "#!/bin/sh\necho 'flag provided but not defined: -famly' >&2\nexit 2\n"
	require.NoError(t, os.WriteFile(zePath, []byte(refuse), 0o700)) //nolint:gosec // test stand-in must be executable
	test, err := parseDecodeCI(t, decodeStdinLine,
		"cmd=foreground:seq=1:exec=ze bgp decode --json --famly ipv4/unicast -:stdin=payload", decodeExpectLine)
	require.NoError(t, err, "the reader no longer interprets exec= words; ze does")

	r := NewDecodingRunner(NewDecodingTests(dir), dir, zePath)
	assert.False(t, r.runTest(context.Background(), test))
	require.Error(t, test.Error)
	assert.Contains(t, test.Error.Error(), "-famly")
}
