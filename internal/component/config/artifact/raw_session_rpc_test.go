// Design: docs/architecture/web-interface.md -- native artifact maintenance and explicit saved sessions.
package artifact_test

import (
	"bytes"
	"context"
	"encoding/base64"
	"strconv"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	editor "github.com/ze-software/ze/internal/component/cli"
	"github.com/ze-software/ze/internal/component/config/storage"
	"github.com/ze-software/ze/internal/component/plugin"
)

func artifactBinarySource(t *testing.T, fixture policyHTTP, content []byte) string {
	t.Helper()
	begin := artifactOperationData(t, fixture, "request config artifact bytes begin name input.bin total "+strconv.Itoa(len(content)))
	id, ok := begin["transfer"].(string)
	require.True(t, ok)
	for offset := 0; offset < len(content); {
		next := min(offset+16384, len(content))
		line := "request config artifact upload append transfer " + id + " offset " + strconv.Itoa(offset) + " content " + strconv.Quote(base64.StdEncoding.EncodeToString(content[offset:next]))
		artifactOperationData(t, fixture, line)
		offset = next
	}
	finished := artifactOperationData(t, fixture, "request config artifact upload finish transfer "+id)
	require.Equal(t, true, finished["complete"])
	t.Cleanup(func() { require.NoError(t, fixture.owner.Abort(context.Background(), "alice", id)) })
	return id
}

func artifactTransferBytes(t *testing.T, fixture policyHTTP, id string) []byte {
	t.Helper()
	var result []byte
	for offset := int64(0); ; {
		state, chunk, err := fixture.owner.Read(t.Context(), "alice", id, offset)
		require.NoError(t, err)
		result = append(result, chunk...)
		if state.Complete {
			break
		}
		require.Greater(t, state.Next, offset)
		offset = state.Next
	}
	return result
}

// TestArtifactRawKeysPreserveBinaryValues proves native key maintenance uses
// complete private bytes and affects only the explicit selected raw key.
func TestArtifactRawKeysPreserveBinaryValues(t *testing.T) {
	fixture := newPolicyHTTP(t, nil, false)
	id := artifactOperationUpload(t, fixture)
	content := []byte{0, 255, 128, '\n', 0, 1}
	input := artifactBinarySource(t, fixture, content)
	selection := " artifact " + id + " key unknown/selected"
	answer := artifactOperationData(t, fixture, "request config artifact key write"+selection+" transfer "+input)
	require.Equal(t, float64(len(content)), answer["bytes"])
	read := artifactOperationData(t, fixture, "request config artifact key read"+selection)
	output, ok := read["transfer"].(string)
	require.True(t, ok)
	require.Equal(t, content, artifactTransferBytes(t, fixture, output))
	require.NoError(t, fixture.owner.Abort(t.Context(), "alice", output))
	keys := artifactOperationData(t, fixture, "show config artifact key list artifact "+id+" prefix unknown/")
	require.Len(t, keys["keys"], 2)
	artifactOperationData(t, fixture, "request config artifact key delete"+selection)
	refused := artifactOperationCall(t, fixture, "request config artifact key read"+selection)
	require.Equal(t, plugin.StatusError, refused.Status)
	var exported bytes.Buffer
	require.NoError(t, fixture.owner.ExportTo(t.Context(), "alice", id, &exported))
	require.True(t, bytes.Contains(exported.Bytes(), []byte{0, 255, 128, 0, 1}))
	require.Contains(t, string(exported.Bytes()), "credential-to-preserve")
}

// TestArtifactRepairUsesAnIndependentSource reaches maintenance on bytes that
// cannot pass normal workspace admission, preserving the input and good keys.
func TestArtifactRepairUsesAnIndependentSource(t *testing.T) {
	fixture := newPolicyHTTP(t, nil, false)
	corrupt := bytes.Clone(fixture.source)
	offset := bytes.Index(corrupt, []byte("credential-to-preserve"))
	require.GreaterOrEqual(t, offset, 0)
	corrupt[offset] ^= 0xff
	input := artifactBinarySource(t, fixture, corrupt)
	checked := artifactOperationCall(t, fixture, "show config artifact check transfer "+input)
	require.Equal(t, plugin.StatusError, checked.Status)
	require.NotNil(t, checked.Data, "a failed check must retain its findings")
	repaired := artifactOperationCall(t, fixture, "request config artifact repair transfer "+input)
	require.Equal(t, plugin.StatusWarning, repaired.Status, repaired.Error)
	data, ok := repaired.Data.(plugin.Map)
	require.True(t, ok)
	id, ok := data["artifact"].(string)
	require.True(t, ok)
	require.NotEmpty(t, id)
	require.Greater(t, data["skipped"], 0)
	kept, err := fixture.owner.Config(t.Context(), "alice", id, "router.conf")
	require.NoError(t, err)
	require.Equal(t, policyConfig, string(kept))
	require.Equal(t, corrupt, artifactTransferBytes(t, fixture, input))
	require.Equal(t, fixture.source[offset]^0xff, corrupt[offset])
	var exported bytes.Buffer
	require.NoError(t, fixture.owner.ExportTo(t.Context(), "alice", id, &exported))
	require.True(t, bytes.Contains(exported.Bytes(), []byte{0, 255, 128, 0, 1}))
}

// TestArtifactSessionResumesOnlyItsNativeAccount keeps committed reads separate
// from explicit saved session edits, and preserves another account's intent.
func TestArtifactSessionResumesOnlyItsNativeAccount(t *testing.T) {
	fixture := newPolicyHTTP(t, nil, false)
	id := artifactOperationUpload(t, fixture)
	var bobBefore []byte
	err := fixture.owner.ChangeStore(t.Context(), "alice", id, func(target storage.Storage) error {
		if err := target.Remove(editor.DraftPath("router.conf")); err != nil {
			return err
		}
		if err := target.Remove(editor.ChangePath("router.conf", "alice")); err != nil {
			return err
		}
		for _, user := range []string{"alice", "bob"} {
			ed, err := editor.NewOfflineSessionEditor(target, "router.conf", user)
			if err != nil {
				return err
			}
			if user == "alice" {
				err = ed.SetValue([]string{"bgp"}, "router-id", "192.0.2.70")
			} else {
				err = ed.SetValue([]string{"bgp", "session", "asn"}, "local", "65042")
			}
			if err == nil && user == "alice" {
				err = ed.SaveDraft()
			}
			closeErr := ed.Close()
			if err != nil {
				return err
			}
			if closeErr != nil {
				return closeErr
			}
		}
		var err error
		bobBefore, err = target.ReadFile(editor.ChangePath("router.conf", "bob"))
		return err
	})
	require.NoError(t, err)
	selection := " artifact " + id + " name router.conf"
	resumed := artifactOperationData(t, fixture, "request config artifact session resume"+selection)
	require.Contains(t, resumed["diff"], "192.0.2.70")
	require.NotContains(t, resumed["diff"], "65042")
	committed, err := fixture.owner.Config(t.Context(), "alice", id, "router.conf")
	require.NoError(t, err)
	require.Equal(t, policyConfig, string(committed))
	text := artifactOperationText(t, fixture, "set bgp router-id 192.0.2.71")
	artifactOperationData(t, fixture, "request config artifact session set"+selection+" text "+text)
	artifactOperationData(t, fixture, "request config artifact session save"+selection)
	changes := artifactOperationData(t, fixture, "request config artifact session changes"+selection)
	require.NotEmpty(t, changes["changes"])
	result := artifactOperationData(t, fixture, "request config artifact session commit"+selection)
	require.Equal(t, true, result["committed"])
	current, err := fixture.owner.Config(t.Context(), "alice", id, "router.conf")
	require.NoError(t, err)
	require.Contains(t, string(current), "192.0.2.71")
	require.NotContains(t, string(current), "65042")
	require.NoError(t, fixture.owner.ViewStore(t.Context(), "alice", id, func(source storage.Storage) error {
		kept, err := source.ReadFile(editor.ChangePath("router.conf", "bob"))
		if err != nil {
			return err
		}
		require.Equal(t, bobBefore, kept)
		return nil
	}))
}

// TestArtifactSessionNativeEditingForms drives the remaining native editing
// forms through their modeled entry points, including reversal and discard.
func TestArtifactSessionNativeEditingForms(t *testing.T) {
	fixture := newPolicyHTTP(t, nil, false)
	id := artifactOperationUpload(t, fixture)
	require.NoError(t, fixture.owner.ChangeStore(t.Context(), "alice", id, func(target storage.Storage) error {
		if err := target.Remove(editor.DraftPath("router.conf")); err != nil { return err }
		if err := target.Remove(editor.ChangePath("router.conf", "alice")); err != nil { return err }
		_, err := storage.RestoreConfig(target, "router.conf", []byte(policyConfig))
		return err
	}))
	selection := " artifact "+id+" name router.conf"
	difference := func() string {
		data := artifactOperationData(t, fixture, "request config artifact session diff"+selection)
		value, ok := data["diff"].(string)
		require.True(t, ok)
		return value
	}
	history := artifactOperationData(t, fixture, "request config artifact session history"+selection)
	require.NotEmpty(t, history["revisions"])
	for _, verb := range []string{"deactivate", "activate"} {
		text := artifactOperationText(t, fixture, verb+" bgp")
		artifactOperationData(t, fixture, "request config artifact session "+verb+selection+" text "+text)
		if verb == "deactivate" { require.Contains(t, difference(), "inactive:")
		} else { require.Empty(t, difference(), "activation must reverse the pending deactivation") }
	}
	for index, verb := range []string{"merge", "replace"} {
		address := "192.0.2.72"
		if index == 1 { address = "192.0.2.73" }
		text := artifactOperationText(t, fixture, strings.ReplaceAll(policyConfig, "1.2.3.4", address))
		artifactOperationData(t, fixture, "request config artifact session "+verb+selection+" text "+text)
		require.Contains(t, difference(), address)
	}
	deletion := artifactOperationText(t, fixture, "delete bgp session")
	artifactOperationData(t, fixture, "request config artifact session delete"+selection+" text "+deletion)
	require.Contains(t, difference(), "65000", "the deleted committed session must appear in the native difference")
	artifactOperationData(t, fixture, "request config artifact session discard"+selection)
	require.Empty(t, difference())
	committed, err := fixture.owner.Config(t.Context(), "alice", id, "router.conf")
	require.NoError(t, err)
	require.Equal(t, policyConfig, string(committed))
}
