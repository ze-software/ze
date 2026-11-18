// Design: docs/architecture/web-interface.md -- standalone artifact operations through the native dispatcher.
package artifact_test

import (
	"bytes"
	"context"
	"encoding/json"
	"strconv"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/command"
	"github.com/ze-software/ze/internal/component/config/storage"
	storagecli "github.com/ze-software/ze/internal/component/config/storage/cli"
	"github.com/ze-software/ze/internal/component/plugin"
)

// TestArtifactFullRestoreUsesTheOuterOwner proves the uploaded source becomes
// a complete independent recovery source before its restart receipt is sent.
// Closing the original workspace cannot invalidate the accepted snapshot.
func TestArtifactFullRestoreUsesTheOuterOwner(t *testing.T) {
	fixture := newPolicyHTTP(t, nil, false)
	id := artifactOperationUpload(t, fixture)
	dir := t.TempDir()
	live, err := storage.Create(dir)
	require.NoError(t, err)
	require.NoError(t, live.WriteKey("unknown/live-original", []byte("keep until clean close")))
	bound := storage.BindConfigSource(live, "router.conf", storage.ConfigSourceStored)
	recovery, err := storage.NewDaemonRecovery(bound)
	require.NoError(t, err)
	t.Cleanup(func() {
		storagecli.InstallDataRPC(nil)
		if recovery.Borrow() != nil {
			closed, err := recovery.Close()
			require.NoError(t, err)
			require.NoError(t, closed.Discard())
		}
	})
	storagecli.InstallDataRPC(&storagecli.DataRPCTarget{Store: recovery.Borrow(), ConfigPath: "router.conf"})
	restarts := 0
	fixture.server.SetRestartFunc(func() { restarts++ })
	response := artifactOperationCall(t, fixture, "request config artifact restore full artifact "+id)
	require.Equal(t, plugin.StatusDone, response.Status, response.Error)
	data, ok := response.Data.(plugin.Map)
	require.True(t, ok)
	require.Equal(t, "accepted", data["state"])
	require.NotEmpty(t, data["digest"])
	require.Zero(t, restarts)
	current, err := recovery.Borrow().ReadKey("unknown/live-original")
	require.NoError(t, err)
	require.Equal(t, "keep until clean close", string(current))
	require.NoError(t, fixture.owner.Remove(t.Context(), "alice", id))
	response.TransportComplete()
	response.TransportComplete()
	require.Equal(t, 1, restarts)
	fixture.server.Stop()
	closed, err := recovery.Close()
	require.NoError(t, err)
	require.NoError(t, closed.Apply())
	restored, err := storage.Open(dir)
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, restored.Close()) })
	content, err := storage.ReadActiveConfig(restored, "router.conf")
	require.NoError(t, err)
	require.Equal(t, policyConfig, string(content))
	raw, err := restored.ReadKey("unknown/binary")
	require.NoError(t, err)
	require.Equal(t, []byte{0, 255, 128, 0, 1}, raw)
}

func artifactOperationCall(t *testing.T, fixture policyHTTP, line string) *plugin.Response {
	t.Helper()
	response, err := fixture.dispatch(t.Context(), plugin.CallerIdentity{Username: "alice"}, line)
	require.NoError(t, err)
	require.NotNil(t, response)
	return response
}

func artifactOperationData(t *testing.T, fixture policyHTTP, line string) map[string]any {
	t.Helper()
	response := artifactOperationCall(t, fixture, line)
	require.Equal(t, plugin.StatusDone, response.Status, response.Error)
	encoded, err := json.Marshal(response.Data)
	require.NoError(t, err)
	var result map[string]any
	require.NoError(t, json.Unmarshal(encoded, &result))
	return result
}

func artifactOperationUpload(t *testing.T, fixture policyHTTP) string {
	t.Helper()
	response := fixture.upload(t, fixture.source, "alice", "https://example.com")
	require.Equal(t, 303, response.Code, response.Body.String())
	return policyID(response.Header().Get("Location"))
}

func artifactOperationText(t *testing.T, fixture policyHTTP, text string) string {
	t.Helper()
	begin, err := fixture.owner.Begin(t.Context(), "alice", "change.txt", int64(len(text)), true)
	require.NoError(t, err)
	_, err = fixture.owner.Append(t.Context(), "alice", begin.ID, 0, []byte(text))
	require.NoError(t, err)
	_, finished, err := fixture.owner.Finish(t.Context(), "alice", begin.ID)
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, fixture.owner.Abort(context.Background(), "alice", finished.ID)) })
	return finished.ID
}

// TestArtifactConfigOperationsShareNativeProducers drives the native stored
// inventory, resolved comparison, statement mutation and version selector on
// the same uploaded artifact, preserving unrelated keys and saved drafts.
func TestArtifactConfigOperationsShareNativeProducers(t *testing.T) {
	fixture := newPolicyHTTP(t, nil, false)
	id := artifactOperationUpload(t, fixture)
	selectConfig := " artifact " + id + " name router.conf"
	inventory := artifactOperationData(t, fixture, "show config artifact config list artifact "+id)
	encoded, err := json.Marshal(inventory)
	require.NoError(t, err)
	require.Contains(t, string(encoded), "router.conf")
	require.Contains(t, string(encoded), "other.conf")
	before, err := fixture.owner.Config(t.Context(), "alice", id, "router.conf")
	require.NoError(t, err)
	statement := artifactOperationText(t, fixture, "set bgp router-id 192.0.2.10")
	preview := artifactOperationData(t, fixture, "request config artifact set"+selectConfig+" text "+statement+" dry-run")
	require.Contains(t, preview["diff"], "192.0.2.10")
	still, err := fixture.owner.Config(t.Context(), "alice", id, "router.conf")
	require.NoError(t, err)
	require.Equal(t, before, still)
	changed := artifactOperationData(t, fixture, "request config artifact set"+selectConfig+" text "+statement)
	require.Equal(t, true, changed["saved"])
	current, err := fixture.owner.Config(t.Context(), "alice", id, "router.conf")
	require.NoError(t, err)
	require.Contains(t, string(current), "192.0.2.10")
	comparison := artifactOperationData(t, fixture, "show config artifact diff"+selectConfig+" other other.conf")
	encoded, err = json.Marshal(comparison)
	require.NoError(t, err)
	require.Contains(t, string(encoded), "192.0.2.10")
	history := artifactOperationData(t, fixture, "show config artifact history"+selectConfig)
	encoded, err = json.Marshal(history)
	require.NoError(t, err)
	require.Contains(t, string(encoded), "revision")
	require.Contains(t, string(encoded), "draft", "the independent saved draft remains represented")
	artifactOperationData(t, fixture, "show config artifact diff"+selectConfig+" revision 2")
	artifactOperationData(t, fixture, "request config artifact rollback"+selectConfig+" revision 2")
	rolledBack, err := fixture.owner.Config(t.Context(), "alice", id, "router.conf")
	require.NoError(t, err)
	require.Contains(t, string(rolledBack), "1.2.3.4")
	require.NotContains(t, string(rolledBack), "192.0.2.10")
	var download bytes.Buffer
	require.NoError(t, fixture.owner.ExportTo(t.Context(), "alice", id, &download))
	require.Contains(t, string(download.Bytes()), "credential-to-preserve")
	require.True(t, bytes.Contains(download.Bytes(), []byte{0, 255, 128, 0, 1}))
}

// TestArtifactConfigMutationRefusesOtherCommands proves statement decoding
// never dispatches an operational command and a failed edit retains all bytes.
func TestArtifactConfigMutationRefusesOtherCommands(t *testing.T) {
	fixture := newPolicyHTTP(t, nil, false)
	id := artifactOperationUpload(t, fixture)
	before, err := fixture.owner.Inspect(t.Context(), "alice", id)
	require.NoError(t, err)
	for _, input := range []string{"request shutdown", "set bgp router-id 192.0.2.12\nrequest shutdown", "set bgp router-id 192.0.2.12 | json"} {
		text := artifactOperationText(t, fixture, input)
		response := artifactOperationCall(t, fixture, "request config artifact set artifact "+id+" name router.conf text "+text)
		require.Equal(t, plugin.StatusError, response.Status)
		after, err := fixture.owner.Inspect(t.Context(), "alice", id)
		require.NoError(t, err)
		require.Equal(t, before.Digest, after.Digest)
	}
	response := artifactOperationCall(t, fixture, "show config artifact diff artifact "+id+" name router.conf other other.conf revision 1")
	require.Equal(t, plugin.StatusError, response.Status, "the comparison source must be unambiguous")
}

// TestArtifactConfigActivationAndReplacement use the existing editing
// statement grammar and full-text serializer rather than text substitution.
func TestArtifactConfigActivationAndReplacement(t *testing.T) {
	fixture := newPolicyHTTP(t, nil, false)
	id := artifactOperationUpload(t, fixture)
	selectConfig := " artifact " + id + " name router.conf"
	for _, verb := range []string{"deactivate", "activate"} {
		text := artifactOperationText(t, fixture, verb+" bgp")
		answer := artifactOperationData(t, fixture, "request config artifact "+verb+selectConfig+" text "+text)
		require.Equal(t, true, answer["saved"])
		content, err := fixture.owner.Config(t.Context(), "alice", id, "router.conf")
		require.NoError(t, err)
		require.Equal(t, verb == "deactivate", strings.Contains(string(content), "inactive:"))
	}
	replacement := strings.ReplaceAll(policyConfig, "1.2.3.4", "192.0.2.44")
	text := artifactOperationText(t, fixture, replacement)
	answer := artifactOperationData(t, fixture, "request config artifact replace"+selectConfig+" text "+text)
	require.Equal(t, true, answer["saved"])
	content, err := fixture.owner.Config(t.Context(), "alice", id, "router.conf")
	require.NoError(t, err)
	require.Contains(t, string(content), "192.0.2.44")
	response := artifactOperationCall(t, fixture, "show config artifact source"+selectConfig+" path "+strconv.Quote("bgp"))
	require.Equal(t, plugin.StatusDone, response.Status, response.Error)
	blob, ok := response.Data.(plugin.RawJSON)
	require.True(t, ok)
	var envelope map[string]json.RawMessage
	require.NoError(t, json.Unmarshal([]byte(blob), &envelope))
	source, err := command.DecodeBlob(envelope["blob"])
	require.NoError(t, err)
	require.Contains(t, string(source), "192.0.2.44")
}
