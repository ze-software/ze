// Design: docs/exabgp/exabgp-migration.md -- migration parity across command surfaces.

package exabgp

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/command"
	configyang "github.com/ze-software/ze/internal/component/config/yang"
	"github.com/ze-software/ze/internal/component/plugin"
	pluginserver "github.com/ze-software/ze/internal/component/plugin/server"
)

const suppliedMigrationConfig = `process announce {
    run /bin/false;
    encoder json;
}
neighbor 192.0.2.1 {
    router-id 192.0.2.2;
    local-address 192.0.2.2;
    local-as 65000;
    peer-as 65001;
    capability { route-refresh; }
    api { processes [ announce ]; }
}`

// suppliedMigrationAnswer reaches the owner registration with the actual model
// arguments and daemon route; a missing handler is a wiring failure.
func suppliedMigrationAnswer(t *testing.T, ctx context.Context, operation string, tokens []string) command.LocalAnswer {
	t.Helper()
	path := "show exabgp migration " + operation
	handler, rest := command.LookupLocalData(strings.Fields(path))
	require.NotNil(t, handler, "%s has no owned handler", path)
	require.Empty(t, rest, "%s has no exact handler", path)
	validated, err := command.ValidateModelArgs(path, tokens, nil)
	if err != nil {
		return command.LocalRefusal(exitError, command.LocalErrorLine(err))
	}
	return command.AnswerRemote(handler, validated.WithContext(ctx))
}

func suppliedMigrationTokens(content string) []string {
	return []string{"content", base64.StdEncoding.EncodeToString([]byte(content))}
}

// captureMigrationCLI directs the bounded fixture's outputs to temporary files,
// so stdout and stderr remain separate without a pipe or a background reader.
func captureMigrationCLI(t *testing.T, run func() int) (int, string, string) {
	t.Helper()
	stdout, err := os.CreateTemp(t.TempDir(), "stdout")
	require.NoError(t, err)
	stderr, err := os.CreateTemp(t.TempDir(), "stderr")
	require.NoError(t, err)
	originalOut, originalErr := os.Stdout, os.Stderr
	os.Stdout, os.Stderr = stdout, stderr
	defer func() { os.Stdout, os.Stderr = originalOut, originalErr }()
	code := run()
	require.NoError(t, stdout.Close())
	require.NoError(t, stderr.Close())
	output, err := os.ReadFile(stdout.Name())
	require.NoError(t, err)
	diagnostic, err := os.ReadFile(stderr.Name())
	require.NoError(t, err)
	return code, string(output), string(diagnostic)
}

// TestSuppliedExaBGPMigrationMatchesStandalone proves both migration operations
// return the standalone bytes and diagnostics, including config dry-run's empty
// output and environment dry-run's warning plus converted output.
func TestSuppliedExaBGPMigrationMatchesStandalone(t *testing.T) {
	for _, operation := range []string{"config", "environment"} {
		for _, dryRun := range []bool{false, true} {
			name := operation
			if dryRun {
				name += "-dry-run"
			}
			t.Run(name, func(t *testing.T) {
				content := suppliedMigrationConfig
				if operation == "environment" {
					content = "[exabgp.log]\nlevel = INFO\n"
				}
				file := filepath.Join(t.TempDir(), "uploaded.conf")
				require.NoError(t, os.WriteFile(file, []byte(content), 0o600))
				localArgs := []string{}
				if dryRun {
					localArgs = append(localArgs, "--dry-run")
				}
				if operation == "environment" {
					localArgs = append(localArgs, "--env", file)
				} else {
					localArgs = append(localArgs, file)
				}
				code, output, diagnostic := captureMigrationCLI(t, func() int { return cmdMigrate(localArgs) })
				require.Equal(t, exitOK, code, diagnostic)
				if operation == "config" {
					require.Contains(t, diagnostic, "process:announce:/bin/false\n")
					require.Contains(t, diagnostic, "info: RIB plugin injected")
					if dryRun {
						require.Empty(t, output)
						require.Contains(t, diagnostic, "dry-run: would migrate "+file)
					} else {
						require.NotEmpty(t, output, "the positive config control converted no bytes")
					}
				} else {
					require.NotEmpty(t, output, "the positive environment control converted no bytes")
					if dryRun {
						require.Contains(t, diagnostic, "--dry-run is not supported with --env")
					}
				}
				tokens := append(suppliedMigrationTokens(content), "name", filepath.Base(file))
				if dryRun {
					tokens = append(tokens, "dry-run")
				}
				answer := suppliedMigrationAnswer(t, context.Background(), operation, tokens)
				require.Equal(t, code, answer.Code, answer.Diagnostic)
				blob, ok := answer.Payload.(command.Blob)
				require.True(t, ok, "payload %T is not a blob", answer.Payload)
				require.Equal(t, output, string(blob))
				require.Equal(t, strings.ReplaceAll(diagnostic, file, filepath.Base(file)), answer.Diagnostic)
				after, err := os.ReadFile(file)
				require.NoError(t, err)
				require.Equal(t, content, string(after), "migration rewrote its input")
			})
		}
	}
}

// TestSuppliedExaBGPMigrationDescriptorsReachTheModel proves generic web inputs
// can discover both operations from owner metadata and their real YANG leaves.
func TestSuppliedExaBGPMigrationDescriptorsReachTheModel(t *testing.T) {
	for _, path := range []string{"show exabgp migration config", "show exabgp migration environment"} {
		t.Run(path, func(t *testing.T) {
			found := false
			for _, entry := range command.ListLocal() {
				if entry.Path != path {
					continue
				}
				found = true
				require.NotNil(t, entry.Meta.SuppliedInput)
				require.Equal(t, "content", entry.Meta.SuppliedInput.ContentArg)
				require.Equal(t, "name", entry.Meta.SuppliedInput.NameArg)
				require.Empty(t, entry.Meta.SuppliedInput.Fields)
			}
			require.True(t, found, "%s is absent from the local-data inventory", path)
			_, err := command.ValidateModelArgs(path, append(suppliedMigrationTokens("x"), "dry-run"), nil)
			require.NoError(t, err)
			for _, tokens := range [][]string{
				nil,
				{"content", "!bad!"},
				{"content", strings.Repeat("A", command.ContentEncodedMax+4)},
				append(suppliedMigrationTokens("x"), "name", "../router.conf"),
			} {
				_, err := command.ValidateModelArgs(path, tokens, nil)
				require.Error(t, err, "invalid arguments were admitted: %v", tokens)
			}
		})
	}
}

// TestSuppliedExaBGPMigrationInvalidSourcesRetainFailure proves parsing refusals
// do not return converted bytes or turn into a successful display result.
func TestSuppliedExaBGPMigrationInvalidSourcesRetainFailure(t *testing.T) {
	for _, operation := range []string{"config", "environment"} {
		t.Run(operation, func(t *testing.T) {
			answer := suppliedMigrationAnswer(t, context.Background(), operation, suppliedMigrationTokens("{{invalid"))
			require.NotZero(t, answer.Code)
			require.Nil(t, answer.Payload)
			require.Contains(t, answer.Diagnostic, "error")
		})
	}
}

// TestSuppliedExaBGPMigrationCanceledRequestReturnsNoBytes proves a canceled
// caller cannot begin conversion or receive a successful payload.
func TestSuppliedExaBGPMigrationCanceledRequestReturnsNoBytes(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	answer := suppliedMigrationAnswer(t, ctx, "config", suppliedMigrationTokens(suppliedMigrationConfig))
	require.NotZero(t, answer.Code)
	require.Nil(t, answer.Payload)
	require.Contains(t, answer.Diagnostic, "canceled")
}

// TestSuppliedExaBGPMigrationDaemonDispatch proves the daemon's actual builtin
// loader discovers each owner-held YANG command and reaches its byte producer,
// retaining conversion diagnostics and parsing failures in the real response.
func TestSuppliedExaBGPMigrationDaemonDispatch(t *testing.T) {
	resolved, err := configyang.DefaultLoader()
	require.NoError(t, err)
	dispatcher := pluginserver.NewDispatcher()
	require.NoError(t, pluginserver.LoadBuiltins(dispatcher, configyang.WireMethodToPath(resolved),
		configyang.PathToDescription(resolved), configyang.PathToHelp(resolved), configyang.PathToArgDefs(resolved)))
	for _, operation := range []string{"config", "environment"} {
		t.Run(operation, func(t *testing.T) {
			content := suppliedMigrationConfig
			if operation == "environment" {
				content = "[exabgp.log]\nlevel = INFO\n"
			}
			path := "show exabgp migration " + operation
			tokens := suppliedMigrationTokens(content)
			want := suppliedMigrationAnswer(t, context.Background(), operation, tokens)
			require.Zero(t, want.Code, want.Diagnostic)
			response, err := dispatcher.Dispatch(&pluginserver.CommandContext{
				Username: "migration-operator", Sender: plugin.OperatorSender(), RequestContext: context.Background(),
			}, path+" "+strings.Join(tokens, " "))
			require.NoError(t, err)
			encoded, err := json.Marshal(response.Data)
			require.NoError(t, err)
			var envelope map[string]json.RawMessage
			require.NoError(t, json.Unmarshal(encoded, &envelope))
			require.Contains(t, envelope, "blob")
			blob, err := command.DecodeBlob(envelope["blob"])
			require.NoError(t, err)
			require.Equal(t, want.Payload, blob)
			if want.Diagnostic == "" {
				require.Equal(t, plugin.StatusDone, response.Status)
				require.Empty(t, response.Warnings)
			} else {
				require.Equal(t, plugin.StatusWarning, response.Status)
				require.Equal(t, want.Diagnostic, strings.Join(response.Warnings, "\n")+"\n")
			}
			response, err = dispatcher.Dispatch(&pluginserver.CommandContext{
				Username: "migration-operator", Sender: plugin.OperatorSender(), RequestContext: context.Background(),
			}, path+" "+strings.Join(suppliedMigrationTokens("{{invalid"), " "))
			require.Error(t, err)
			require.Equal(t, plugin.StatusError, response.Status)
			require.Nil(t, response.Data)
			require.NotEmpty(t, response.Error)
		})
	}
}
