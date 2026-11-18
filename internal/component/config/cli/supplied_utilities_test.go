// Design: docs/architecture/api/commands.md -- supplied completion and private codec inputs.
package cli

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"strconv"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/command"
	"github.com/ze-software/ze/internal/component/config/secret"
	"github.com/ze-software/ze/internal/component/plugin"
	pluginserver "github.com/ze-software/ze/internal/component/plugin/server"
	"github.com/ze-software/ze/internal/core/redact"
)

func utilityServer(t *testing.T) *pluginserver.Server {
	t.Helper()
	server, err := pluginserver.NewServer(&pluginserver.ServerConfig{}, nil)
	require.NoError(t, err)
	t.Cleanup(server.Stop)
	return server
}

func utilityCall(t *testing.T, server *pluginserver.Server, ctx context.Context, line string) (*plugin.Response, error) {
	t.Helper()
	return server.Dispatcher().Dispatch(&pluginserver.CommandContext{Server: server,
		Username: "reader", Authorizer: privateConfigPolicy{}, RequestContext: ctx,
		Sender: plugin.OperatorSender()}, line)
}

func utilityBlob(t *testing.T, response *plugin.Response) []byte {
	t.Helper()
	require.NotNil(t, response)
	require.Equal(t, plugin.StatusDone, response.Status, response.Error)
	encoded, err := json.Marshal(response.Data)
	require.NoError(t, err)
	var envelope map[string]json.RawMessage
	require.NoError(t, json.Unmarshal(encoded, &envelope))
	content, err := command.DecodeBlob(envelope["blob"])
	require.NoError(t, err)
	return content
}

func utilityContent(content string) string {
	return " content " + strconv.Quote(base64.StdEncoding.EncodeToString([]byte(content)))
}

func utilityCompletionContent(t *testing.T, configuration, input string) string {
	t.Helper()
	encoded, err := json.Marshal(map[string]string{"configuration": configuration, "input": input})
	require.NoError(t, err)
	return utilityContent(string(encoded))
}

// TestSuppliedCompletionMatchesStandalone queries the same caller-held source,
// context and editing text through the real daemon registration and local CLI.
func TestSuppliedCompletionMatchesStandalone(t *testing.T) {
	server := utilityServer(t)
	file := writeTestConfig(t, validConfig)
	completer, tree, code := completerFor(file)
	require.Equal(t, exitOK, code)
	input := "set router"
	contextPath := []string{"bgp"}
	want := completer.Complete(tree, input, contextPath)
	require.NotEmpty(t, want, "the positive control must actually complete this path")
	line := "show config supplied completion" + utilityCompletionContent(t, validConfig, input) + " context bgp"
	response, err := utilityCall(t, server, t.Context(), line)
	require.NoError(t, err)
	require.Equal(t, plugin.StatusDone, response.Status, response.Error)
	encoded, err := json.Marshal(response.Data)
	require.NoError(t, err)
	var answer struct {
		Completions []struct {
			Text        string `json:"text"`
			Kind        string `json:"type"`
			ShortHelp   string `json:"short-help"`
			Description string `json:"description"`
		} `json:"completions"`
	}
	require.NoError(t, json.Unmarshal(encoded, &answer))
	require.Len(t, answer.Completions, len(want))
	for index, expected := range want {
		got := answer.Completions[index]
		require.Equal(t, expected.Text, got.Text)
		require.Equal(t, expected.Type, got.Kind)
		require.Equal(t, expected.ShortHelp, got.ShortHelp)
		require.Equal(t, expected.Description, got.Description)
	}
	code, local, diagnostic := captureBlobStdio(t, func() int {
		return cmdCompletion([]string{"--ghost", "--context", "bgp", "--input", "set+router", file})
	})
	require.Equal(t, exitOK, code, diagnostic)
	response, err = utilityCall(t, server, t.Context(), line+" ghost")
	require.NoError(t, err)
	require.Equal(t, local, string(utilityBlob(t, response)))
}

// TestSuppliedSecretMatchesStandalone preserves literal bytes and the explicit
// one-line-ending stdin adapter without reading any daemon-side secret or file.
func TestSuppliedSecretMatchesStandalone(t *testing.T) {
	server := utilityServer(t)
	for _, plain := range []string{"p@ss w0rd!$ ", "clé", "x\x00y\n"} {
		response, err := utilityCall(t, server, t.Context(), "show secret encode"+utilityContent(plain))
		require.NoError(t, err)
		encoded := strings.TrimSuffix(string(utilityBlob(t, response)), "\n")
		decoded, err := secret.Decode(encoded)
		require.NoError(t, err)
		require.Equal(t, plain, decoded)
		code, local, diagnostic := runSecretCapture(t, "", "decode", encoded)
		require.Equal(t, exitOK, code, diagnostic)
		response, err = utilityCall(t, server, t.Context(), "show secret decode"+utilityContent(encoded))
		require.NoError(t, err)
		require.Equal(t, local, string(utilityBlob(t, response)))
		response, err = utilityCall(t, server, t.Context(), "show secret decode"+utilityContent(encoded+"\r\n")+" trim-line-ending")
		require.NoError(t, err)
		require.Equal(t, local, string(utilityBlob(t, response)))
	}
}

// TestSuppliedUtilitiesRefuseAndProtectInput covers bounded/canceled/refused
// sources while retaining the real shared recorder's private-input masking.
func TestSuppliedUtilitiesRefuseAndProtectInput(t *testing.T) {
	server := utilityServer(t)
	for _, line := range []string{
		"show secret decode" + utilityContent("$2a$10$irreversible-private-hash"),
		"show secret decode" + utilityContent("$9$~~~~"),
		"show secret encode" + utilityContent("$9$private-already-encoded"),
		"show secret encode content \"\"",
		"show config supplied completion" + utilityCompletionContent(t, "invalid configuration syntax", "set "),
		"show config supplied completion content !!!",
	} {
		response, err := utilityCall(t, server, t.Context(), line)
		if err == nil {
			require.NotNil(t, response)
			require.Equal(t, plugin.StatusError, response.Status, line)
			require.Empty(t, response.Data)
		} else {
			require.NotContains(t, err.Error(), "private-already-encoded")
		}
		if response != nil {
			require.NotContains(t, response.Error, "private-already-encoded")
		}
	}
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	response, err := utilityCall(t, server, ctx, "show secret encode"+utilityContent("canceled-private"))
	if err == nil {
		require.Equal(t, plugin.StatusError, response.Status)
		require.Empty(t, response.Data)
	}
	for _, path := range []string{"show secret encode", "show secret decode", "show config supplied completion"} {
		var entryFound bool
		for _, entry := range command.ListLocal() {
			if entry.Path != path {
				continue
			}
			entryFound = true
			require.NotNil(t, entry.Meta.SuppliedInput)
			require.Equal(t, "content", entry.Meta.SuppliedInput.ContentArg)
			if path == "show config supplied completion" {
				require.Equal(t, []string{"configuration", "input"}, entry.Meta.SuppliedInput.Fields)
			}
		}
		require.True(t, entryFound, path)
	}
	line := "show config supplied completion" + utilityCompletionContent(t, "environment {}", "set description private-operand")
	encoded := strings.TrimPrefix(line, "show config supplied completion content ")
	masked := redact.Command(line)
	require.NotContains(t, masked, encoded)
	require.Contains(t, masked, redact.Placeholder)
}
