// Design: docs/architecture/api/commands.md -- bounded private utility inputs.
package cli

import (
	"encoding/base64"
	"encoding/json"
	"slices"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/command"
	configyang "github.com/ze-software/ze/internal/component/config/yang"
	"github.com/ze-software/ze/internal/component/plugin"
)

// TestSuppliedCompletionStrictEnvelope exercises the actual dispatcher with
// malformed pairs and a valid control, so an unconditional refusal cannot pass.
func TestSuppliedCompletionStrictEnvelope(t *testing.T) {
	server := utilityServer(t)
	response, err := utilityCall(t, server, t.Context(), "show config supplied completion"+
		utilityCompletionContent(t, validConfig, "set router")+" context bgp")
	require.NoError(t, err)
	require.Equal(t, plugin.StatusDone, response.Status, response.Error)
	for _, envelope := range []string{
		`{"configuration":"","input":"set ","input":"private-query"}`,
		`{"configuration":""}`,
		`{"configuration":"","input":null}`,
		`{"configuration":"","input":17}`,
		`{"configuration":"","input":"set ","private-field":"private-query"}`,
		`{"configuration":"","input":"set "} {}`,
		"{\"configuration\":\"\xff\",\"input\":\"set \"}",
	} {
		response, err := utilityCall(t, server, t.Context(), "show config supplied completion"+utilityContent(envelope))
		var refusal *plugin.RefusalError
		require.ErrorAs(t, err, &refusal)
		require.NotNil(t, response)
		require.Equal(t, plugin.StatusError, response.Status)
		require.Equal(t, response.Error, err.Error())
		require.Empty(t, response.Data)
		require.NotContains(t, response.Error, "private-query")
		require.NotContains(t, response.Error, "private-field")
		require.NotContains(t, err.Error(), "private-query")
		require.NotContains(t, err.Error(), "private-field")
	}
}

// TestSuppliedCompletionLiteralPlus compares native candidates without the
// standalone shell adapter, proving a supplied plus sign remains data.
func TestSuppliedCompletionLiteralPlus(t *testing.T) {
	server := utilityServer(t)
	completer, tree, code := completerFor(writeTestConfig(t, validConfig))
	require.Equal(t, exitOK, code)
	for _, input := range []string{"set router", "set+router"} {
		response, err := utilityCall(t, server, t.Context(), "show config supplied completion"+
			utilityCompletionContent(t, validConfig, input)+" context bgp")
		require.NoError(t, err)
		require.Equal(t, plugin.StatusDone, response.Status, response.Error)
		got, err := json.Marshal(response.Data)
		require.NoError(t, err)
		want, err := json.Marshal(struct {
			Completions []completionCandidate `json:"completions"`
		}{completionCandidates(completer.Complete(tree, input, []string{"bgp"}))})
		require.NoError(t, err)
		require.JSONEq(t, string(want), string(got))
	}
	require.NotEqual(t, completer.Complete(tree, "set router", []string{"bgp"}),
		completer.Complete(tree, "set+router", []string{"bgp"}), "the native control must distinguish the inputs")
}

// TestSuppliedUtilityModeledBounds checks native argument admission at both
// edges and reads the actual resolved sensitive leaves, including augment use.
func TestSuppliedUtilityModeledBounds(t *testing.T) {
	loader, err := configyang.DefaultLoader()
	require.NoError(t, err)
	definitions := configyang.PathToArgDefs(loader)
	root := loader.GetEntry("ze-config-cli-cmd")
	require.NotNil(t, root)
	for _, path := range []string{"show config supplied completion", "show secret encode", "show secret decode"} {
		entry := root
		for _, keyword := range strings.Fields(path + " content") {
			entry = entry.Dir[keyword]
			require.NotNil(t, entry, keyword)
		}
		require.True(t, configyang.HasSensitiveExtension(entry), path)
		minimum := uint64(0)
		if path == "show config supplied completion" {
			minimum = 4
		}
		var content *command.ArgDef
		for index := range definitions[path] {
			def := &definitions[path][index]
			if def.Name() == "content" {
				content = def
			}
		}
		require.NotNil(t, content, path)
		require.True(t, content.Mandatory())
		require.True(t, content.KeywordOnly())
		require.Equal(t, []command.UintRange{{Min: minimum, Max: command.ContentEncodedMax}}, slices.Collect(content.Lengths()))
		_, err := command.ValidateModelArgs(path, []string{"content", strings.Repeat("A", command.ContentEncodedMax)}, nil)
		require.NoError(t, err)
		_, err = command.ValidateModelArgs(path, []string{"content", base64.StdEncoding.EncodeToString(make([]byte, command.ContentOctetsMax+1))}, nil)
		require.Error(t, err)
		_, err = command.ValidateModelArgs(path, []string{"content", ""}, nil)
		if minimum == 0 {
			require.NoError(t, err)
		} else {
			require.Error(t, err)
		}
	}
}

// TestSuppliedUtilityPrivateRefusals checks local and remote failures without
// returning config values, supplied identifiers, or modular-crypt private text.
func TestSuppliedUtilityPrivateRefusals(t *testing.T) {
	server := utilityServer(t)
	const configuration = "bgp {\n router-id private-address-operand;\n}"
	code, _, local := captureBlobStdio(t, func() int {
		return cmdCompletion([]string{"--input", "set+", writeTestConfig(t, configuration)})
	})
	require.NotEqual(t, exitOK, code)
	require.Contains(t, local, "line 2")
	require.Contains(t, local, "ze config validate <file>")
	require.NotContains(t, local, "private-address-operand")
	response, err := utilityCall(t, server, t.Context(), "show config supplied completion"+
		utilityCompletionContent(t, configuration, "set private-query"))
	var refusal *plugin.RefusalError
	require.ErrorAs(t, err, &refusal)
	require.NotNil(t, response)
	require.Equal(t, plugin.StatusError, response.Status)
	require.Equal(t, response.Error, err.Error())
	require.Empty(t, response.Data)
	require.Contains(t, response.Error, "line 2")
	require.NotContains(t, response.Error, "private-address-operand")
	require.NotContains(t, response.Error, "private-query")
	require.NotContains(t, err.Error(), "private-address-operand")
	require.NotContains(t, err.Error(), "private-query")
	for _, operand := range []string{"$privatehashformat$private-operand", "$9$private-operand"} {
		action := "decode"
		if strings.HasPrefix(operand, "$9$") {
			action = "encode"
		}
		_, _, local := runSecretCapture(t, "", action, operand)
		require.NotContains(t, local, "private-operand")
		require.NotContains(t, local, "privatehashformat")
		response, err := utilityCall(t, server, t.Context(), "show secret "+action+utilityContent(operand))
		var refusal *plugin.RefusalError
		require.ErrorAs(t, err, &refusal)
		require.NotNil(t, response)
		require.Equal(t, plugin.StatusError, response.Status)
		require.Equal(t, response.Error, err.Error())
		require.Empty(t, response.Data)
		require.NotContains(t, response.Error, "private-operand")
		require.NotContains(t, response.Error, "privatehashformat")
		require.NotContains(t, err.Error(), "private-operand")
		require.NotContains(t, err.Error(), "privatehashformat")
	}
}

// TestSuppliedSecretBinaryRoundTrip proves arbitrary octets survive the codec
// and the command Blob envelope instead of JSON replacing invalid UTF-8 bytes.
func TestSuppliedSecretBinaryRoundTrip(t *testing.T) {
	server := utilityServer(t)
	plain := string([]byte{0, 0xff, '\r', '\n', 0x80})
	response, err := utilityCall(t, server, t.Context(), "show secret encode"+utilityContent(plain))
	require.NoError(t, err)
	encoded := strings.TrimSuffix(string(utilityBlob(t, response)), "\n")
	response, err = utilityCall(t, server, t.Context(), "show secret decode"+utilityContent(encoded))
	require.NoError(t, err)
	require.Equal(t, []byte(plain+"\n"), utilityBlob(t, response))
}
