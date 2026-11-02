package cli

import (
	"encoding/json"
	"os"
	"path/filepath"
	"slices"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/command"
	"github.com/ze-software/ze/internal/component/plugin/registry"
)

// privateCapabilityOpenHex is a whole OPEN carrying one capability no
// registered plugin decodes: code 250 (RFC 5492 Section 4 assigns 239-254 for
// private use), two value octets ABCD. Length 35: header 19, OPEN fields 10,
// optional parameters 6.
const privateCapabilityOpenHex = "FFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFF" + "0023" + "01" +
	"04" + "FFFD" + "00B4" + "0A000002" + "06" + "02" + "04" + "FA" + "02" + "ABCD"

// labPluginScript answers the decode protocol a third-party plugin speaks:
// one request line on standard input, one answer line. It decodes capability
// 250 by name and nothing else. It touches ran beside itself, so a test can
// tell whether it was started at all.
const labPluginScript = `#!/bin/sh
touch "$(dirname "$0")/ran"
read -r line
case "$line" in
"decode capability 250 "*) echo 'decoded json {"name":"lab-private","value":"'"${line##* }"'"}' ;;
*) echo 'decoded unknown' ;;
esac
`

// writeLabPlugin writes labPluginScript into a fresh directory and answers
// its path.
func writeLabPlugin(t *testing.T) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "lab-plugin")
	require.NoError(t, os.WriteFile(path, []byte(labPluginScript), 0o700))
	return path
}

// decodedCapability answers the capability of code in a decode answer's only
// message.
func decodedCapability(t *testing.T, answer command.LocalAnswer, code float64) map[string]any {
	t.Helper()
	result, ok := answer.Payload.(decodeAnswer)
	require.True(t, ok, "the decode handler answered %T", answer.Payload)
	require.Len(t, result.Decode.Messages, 1)
	var event map[string]any
	require.NoError(t, json.Unmarshal(result.Decode.Messages[0].Decoded, &event))
	bgp, ok := event["bgp"].(map[string]any)
	require.True(t, ok, "no bgp object in %v", event)
	open, ok := bgp["open"].(map[string]any)
	require.True(t, ok, "no open object in %v", bgp)
	capabilities, ok := open["capabilities"].([]any)
	require.True(t, ok, "no capabilities in %v", open)
	for _, entry := range capabilities {
		capability, ok := entry.(map[string]any)
		require.True(t, ok)
		if capability["code"] == code {
			return capability
		}
	}
	t.Fatalf("no capability %v in %v", code, capabilities)
	return nil
}

// TestDecodeNamedProgramDecodesACapability proves `--plugin <program>` is
// used, not parsed and dropped: the same OPEN decodes its private capability
// as unknown with no plugin named, and by the program's name with it, on the
// answer, its text drawing, and the plugins the answer lists.
func TestDecodeNamedProgramDecodesACapability(t *testing.T) {
	program := writeLabPlugin(t)

	without := answerDecode(command.ValidatedArgs{}, []string{"--open", privateCapabilityOpenHex}, command.RouteLocal)
	require.Equal(t, 0, without.Code, without.Diagnostic)
	require.Equal(t, capNameUnknown, decodedCapability(t, without, 250)["name"])

	with := answerDecode(command.ValidatedArgs{},
		[]string{"--plugin", program, "--open", privateCapabilityOpenHex}, command.RouteLocal)
	require.Equal(t, 0, with.Code, with.Diagnostic)
	capability := decodedCapability(t, with, 250)
	require.Equal(t, "lab-private", capability["name"])
	require.Equal(t, "ABCD", capability["value"])

	result, ok := with.Payload.(decodeAnswer)
	require.True(t, ok)
	require.Equal(t, []string{program}, result.Decode.Plugins)
	text, err := drawDecode(&result.Decode)
	require.NoError(t, err)
	require.Contains(t, text, "lab-private", "the text drawing decodes under the same plugins")
}

// TestDecodeNamedPluginRefusesAnUnknownName proves a name no registered plugin
// decodes under is refused, naming the plugins that do, rather than ignored.
func TestDecodeNamedPluginRefusesAnUnknownName(t *testing.T) {
	answer := answerDecode(command.ValidatedArgs{},
		[]string{"--plugin", "no-such-plugin", "--open", privateCapabilityOpenHex}, command.RouteLocal)
	require.Equal(t, 1, answer.Code)
	require.Nil(t, answer.Payload)
	require.Contains(t, answer.Diagnostic, `no plugin named "no-such-plugin" decodes`)
	names := slices.Sorted(func(yield func(string) bool) {
		for name := range registry.InProcessDecoders() {
			if !yield(name) {
				return
			}
		}
	})
	require.NotEmpty(t, names, "the test binary links no decoding plugin")
	require.Contains(t, answer.Diagnostic, names[0])
}

// TestDecodeNamedProgramRefusedOnARemoteRoute proves a program path never
// starts on a route serving another process's caller: the refusal names the
// local command, and the program did not run.
func TestDecodeNamedProgramRefusedOnARemoteRoute(t *testing.T) {
	program := writeLabPlugin(t)
	answer := answerDecode(command.ValidatedArgs{},
		[]string{"--plugin", program, "--open", privateCapabilityOpenHex}, command.RouteRemote)
	require.Equal(t, 1, answer.Code)
	require.Contains(t, answer.Diagnostic, "names a program")
	require.Contains(t, answer.Diagnostic, "ze bgp decode --plugin "+program)
	_, err := os.Stat(filepath.Join(filepath.Dir(program), "ran"))
	require.True(t, os.IsNotExist(err), "the program ran on a remote route")
}

// TestDecodePluginKeywordNamesARegisteredPlugin proves the plugin leaf binds
// a registered name, that the handler decodes under it and lists it, and that
// the model refuses a program path in the keyword before any handler runs.
func TestDecodePluginKeywordNamesARegisteredPlugin(t *testing.T) {
	validated := judged(t, pathDecode, "plugin", "ze-bgp-hostname", privateCapabilityOpenHex)
	value, found := validated.Keyed(leafDecodePlugin)
	require.True(t, found)
	require.Equal(t, "ze-bgp-hostname", value)

	answer := dataDecode(validated)
	require.Equal(t, 0, answer.Code, answer.Diagnostic)
	result, ok := answer.Payload.(decodeAnswer)
	require.True(t, ok)
	require.Equal(t, []string{"ze-bgp-hostname"}, result.Decode.Plugins)

	_, err := command.ValidateModelArgs(pathDecode, []string{"plugin", "/bin/sh", privateCapabilityOpenHex}, nil)
	require.Error(t, err, "the keyword takes a plugin name, never a program path")
	require.Contains(t, err.Error(), "/bin/sh")
}
