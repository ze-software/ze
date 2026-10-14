// VALIDATES: the decode suite's .ci reader refuses a directive it has no arm
//            for, a key a line does not read, a second cmd=, and an exec= that
//            runs another program, naming the file, the line and what is
//            accepted. The exec= words themselves are run, not read, so ze's
//            own flag parser refuses a misspelled flag (decoding_exec_test.go).
// PREVENTS: the default-arm drop the 2026-09-07 decoding.go journal row
//           records: a second cmd= replaced the first with nothing said.

package runner

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	decodeStdinLine  = "stdin=payload:hex=FFFF"
	decodeCmdLine    = "cmd=foreground:seq=1:exec=ze bgp decode --json -:stdin=payload"
	decodeExpectLine = `expect=json:json={"a":1}`
)

func parseDecodeCI(t *testing.T, lines ...string) (*decodingTest, error) {
	t.Helper()
	file := filepath.Join(t.TempDir(), "decode.ci")
	require.NoError(t, os.WriteFile(file, []byte(strings.Join(lines, "\n")+"\n"), 0o600))
	return NewDecodingTests(filepath.Dir(file)).parseCIFile(file)
}

func TestDecodeCIRefusesWhatItCannotRead(t *testing.T) {
	tests := []struct {
		name   string
		lines  []string
		wantIn []string
	}{
		{"unknown directive", []string{decodeStdinLine, decodeCmdLine, decodeExpectLine, "option=env:var=X:value=1"},
			[]string{"line 4", `"option=env:var=X:value=1"`, "cmd=", "expect=json:", "stdin="}},
		{"unknown stdin key", []string{"stdin=payload:size=2:hex=FFFF", decodeCmdLine, decodeExpectLine},
			[]string{"line 1", "stdin=payload", `"size"`, "accepts hex"}},
		{"unknown expect key", []string{decodeStdinLine, decodeCmdLine, `expect=json:jsn={"a":1}`},
			[]string{"line 3", "expect=json", `"jsn"`, "accepts json"}},
		{"unknown cmd key", []string{decodeStdinLine, decodeCmdLine + ":timeout=5s", decodeExpectLine},
			[]string{"line 2", `"timeout"`, "accepts exec, seq, stdin"}},
		{"second cmd", []string{decodeStdinLine, decodeCmdLine, decodeCmdLine, decodeExpectLine},
			[]string{"line 3", "second cmd="}},
		{"wrong program", []string{decodeStdinLine,
			"cmd=foreground:seq=1:exec=ze bgp encode --json -:stdin=payload", decodeExpectLine},
			[]string{"line 2", "ze bgp decode"}},
		{"stdin name with no block", []string{decodeStdinLine,
			"cmd=foreground:seq=1:exec=ze bgp decode --json -:stdin=other", decodeExpectLine},
			[]string{"line 2", `"other"`}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := parseDecodeCI(t, tt.lines...)
			require.Error(t, err)
			for _, want := range tt.wantIn {
				assert.Contains(t, err.Error(), want)
			}
		})
	}
}

func TestDecodeCIStrictStillParses(t *testing.T) {
	test, err := parseDecodeCI(t, decodeStdinLine,
		"cmd=foreground:seq=1:exec=ze bgp decode --json --plugin bgp-ls --family ipv4/unicast --open -:stdin=payload",
		decodeExpectLine)
	require.NoError(t, err)
	assert.Equal(t, "FFFF", test.HexPayload)
	assert.Equal(t, []string{"bgp", "decode", "--json", "--plugin", "bgp-ls", "--family", "ipv4/unicast", "--open", "-"}, test.Args)
	assert.Equal(t, "FFFF\n", test.Stdin)
}

// TestParseCIEnvRefusesUnknownField covers the parse suite's option=env reader,
// which dropped every field but var= and value=.
func TestParseCIEnvRefusesUnknownField(t *testing.T) {
	for _, directive := range []string{
		"option=env:var=ZE_X:vaule=1",
		"option=env:var=ZE_X:value=1:var=ZE_Y",
		"option=env:1:var=ZE_X",
	} {
		t.Run(directive, func(t *testing.T) {
			test := discoverParseCI(t, dialectBaseCI+directive+"\n")
			require.Error(t, test.ParseError)
			assert.Contains(t, test.ParseError.Error(), "option=env")
			assert.Contains(t, test.ParseError.Error(), "dialect.ci")
			assert.Contains(t, test.ParseError.Error(), "accepts value, var")
		})
	}
}
