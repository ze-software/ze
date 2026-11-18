// Design: docs/architecture/api/commands.md -- private file answers require edit authorization.
package cli

import (
	"encoding/base64"
	"encoding/json"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/command"
	"github.com/ze-software/ze/internal/component/config"
	"github.com/ze-software/ze/internal/component/plugin"
	pluginserver "github.com/ze-software/ze/internal/component/plugin/server"
)

type privateConfigPolicy struct{ edit bool }

func (policy privateConfigPolicy) Authorize(_, _, _ string, readOnly bool) bool {
	if readOnly {
		return true
	}
	return policy.edit
}

type privateConfigFileCase struct {
	operation string
	content   string
	statement string
}

func privateConfigFileCases() []privateConfigFileCase {
	return []privateConfigFileCase{
		{operation: "migrate", content: testConfigSecretBase},
		{operation: "set", content: testConfigSecretBase, statement: "set bgp router-id 192.0.2.9"},
		{operation: "activate", content: strings.Replace(testConfigSecretBase, "bgp {", "inactive: bgp {", 1), statement: "activate bgp"},
		{operation: "deactivate", content: testConfigSecretBase, statement: "deactivate bgp"},
		{operation: "replace", content: testConfigSecretBase, statement: strings.Replace(testConfigSecretBase,
			"connection {", "connection {\n                local { ip auto; }", 1)},
		{operation: "fmt", content: testConfigSecretBase},
		{operation: "dump", content: testConfigSecretBase},
	}
}

func (test privateConfigFileCase) invocation(file string) string {
	line := "show config " + test.operation + " "
	if test.operation != "fmt" {
		if test.operation != "dump" {
			line += "file "
		}
	}
	line += strconv.Quote(file)
	if test.operation == "migrate" {
		line += " format hierarchical"
	}
	if test.statement != "" {
		line += " content " + base64.StdEncoding.EncodeToString([]byte(test.statement))
	}
	return line
}

func privateConfigServer(t *testing.T) *pluginserver.Server {
	t.Helper()
	server, err := pluginserver.NewServer(&pluginserver.ServerConfig{}, nil)
	require.NoError(t, err)
	t.Cleanup(server.Stop)
	return server
}

func privateConfigDispatch(t *testing.T, server *pluginserver.Server, line string, edit bool) (*plugin.Response, error) {
	t.Helper()
	return server.Dispatcher().Dispatch(&pluginserver.CommandContext{
		Server: server, Username: "reader", Authorizer: privateConfigPolicy{edit: edit},
		RequestContext: t.Context(), Sender: plugin.OperatorSender(),
	}, line)
}

// TestPrivateConfigFileAnswersRequireEditAuthorization reaches each actual
// file registration through the daemon dispatcher. A reader is refused before
// both an existing private file and a nonexistent file can be read.
func TestPrivateConfigFileAnswersRequireEditAuthorization(t *testing.T) {
	for _, test := range privateConfigFileCases() {
		t.Run(test.operation, func(t *testing.T) {
			file := confinedConfigDir(t, test.content)
			server := privateConfigServer(t)
			for _, path := range []string{file, filepath.Join(filepath.Dir(file), "missing.conf")} {
				response, err := privateConfigDispatch(t, server, test.invocation(path), false)
				require.ErrorIs(t, err, pluginserver.ErrUnauthorized, test.operation)
				require.NotNil(t, response)
				require.Equal(t, plugin.StatusError, response.Status)
				require.Nil(t, response.Data, "a refused read must return no source bytes")
			}
		})
	}
}

// TestPrivateConfigFileAnswersRetainAuthorizedOutput proves the authorization
// change preserves useful complete native answers, including their reversible
// secret encoding, while detached operations leave the original file intact.
func TestPrivateConfigFileAnswersRetainAuthorizedOutput(t *testing.T) {
	for _, test := range privateConfigFileCases() {
		t.Run(test.operation, func(t *testing.T) {
			file := confinedConfigDir(t, test.content)
			server := privateConfigServer(t)
			response, err := privateConfigDispatch(t, server, test.invocation(file), true)
			require.NoError(t, err)
			require.Equal(t, plugin.StatusDone, response.Status, response.Error)
			encoded, err := json.Marshal(response.Data)
			require.NoError(t, err)
			require.Contains(t, string(encoded), "$9$", "authorized source output must retain usable encoded credentials")
			require.NotContains(t, string(encoded), config.SecretDataPlaceholder)
			original, err := os.ReadFile(file)
			require.NoError(t, err)
			require.Equal(t, test.content, string(original))
		})
	}
}

// TestPrivateConfigFilePolicyKeepsCallerBytesAndMaskedSourceReadable proves
// that private host-file authorization does not hide caller-owned conversion
// or an already masked view of a permitted host file from a read-only caller.
func TestPrivateConfigFilePolicyKeepsCallerBytesAndMaskedSourceReadable(t *testing.T) {
	file := confinedConfigDir(t, testConfigSecretBase)
	server := privateConfigServer(t)
	content := base64.StdEncoding.EncodeToString([]byte(testConfigSecretBase))
	response, err := privateConfigDispatch(t, server,
		"show config supplied migrate content "+content+" format hierarchical", false)
	require.NoError(t, err)
	require.Equal(t, plugin.StatusDone, response.Status, response.Error)
	encoded, err := json.Marshal(response.Data)
	require.NoError(t, err)
	require.Contains(t, string(encoded), "$9$")

	response, err = privateConfigDispatch(t, server, "show config source file "+strconv.Quote(file), false)
	require.NoError(t, err)
	require.Equal(t, plugin.StatusDone, response.Status, response.Error)
	encoded, err = json.Marshal(response.Data)
	require.NoError(t, err)
	var envelope struct {
		Blob json.RawMessage `json:"blob"`
	}
	require.NoError(t, json.Unmarshal(encoded, &envelope))
	masked, err := command.DecodeBlob(envelope.Blob)
	require.NoError(t, err)
	require.Contains(t, string(masked), config.SecretDataPlaceholder)
	require.NotContains(t, string(masked), diffStoredPassword)
	require.NotContains(t, string(masked), "$9$")
}
