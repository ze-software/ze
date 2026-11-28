// Design: docs/architecture/api/process-protocol.md -- enveloped UPDATE receipt quoting.
// Related: event_update_receipt.go -- updateEnvelopeCache under test.

package server

import (
	"encoding/json"
	"net/netip"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	bgptypes "github.com/ze-software/ze/internal/component/bgp/types"
	"github.com/ze-software/ze/internal/component/plugin"
	"github.com/ze-software/ze/internal/component/plugin/process"
	bgpevents "github.com/ze-software/ze/internal/core/bgp/events"
	"github.com/ze-software/ze/internal/core/bgp/msgtype"
	"github.com/ze-software/ze/pkg/plugin/rpc"
)

// TestUpdateEnvelopeWrap_Base64QuotesPayload proves that updateEnvelopeCache.wrap
// JSON-quotes positional `base64` UPDATE lines when SetEnvelope(true) is active
// under EncodingJSON, while preserving JSON-object embedding for FormatParsed
// and string quoting for EncodingText.
//
// VALIDATES: AC-1 -- SetEnvelope(true) + EncodingJSON + FormatBase64 produces a
// valid rpc.EventEnvelope carrying the exact upd line and receipt metadata,
// with FormatParsed JSON and EncodingText regression controls intact.
// PREVENTS: json.Marshal failing on bare `upd ...` payloads and silently
// dropping UPDATE events for enveloped base64 subscribers.
func TestUpdateEnvelopeWrap_Base64QuotesPayload(t *testing.T) {
	t.Parallel()

	peer := &plugin.PeerInfo{
		Address:   netip.MustParseAddr("10.0.0.1"),
		PeerOwner: 17,
	}
	msg := &bgptypes.RawMessage{
		Type:              msgtype.TypeUPDATE,
		MessageID:         77,
		SourcePolicyEpoch: 3,
		SourceReceipt:     9,
		ReactorForwarded:  true,
	}

	var cache updateEnvelopeCache

	// 1. EncodingJSON + FormatBase64: must JSON-quote the positional upd line.
	b64Proc := process.NewProcess(plugin.PluginConfig{Name: "base64-enveloped"})
	b64Proc.SetEnvelope(true)
	b64Proc.SetEncoding(plugin.EncodingJSON)
	b64Proc.SetFormat(plugin.FormatBase64)

	const b64Line = "upd 14 65000 65001 42 10.0.0.1 10.0.0.2 1.1.1.1 - 8:AAAAAA==\n"
	wrappedB64, err := cache.wrap(peer, msg, b64Proc, b64Line)
	require.NoError(t, err)

	var b64Env rpc.EventEnvelope
	require.NoError(t, json.Unmarshal([]byte(wrappedB64), &b64Env))
	assert.Equal(t, bgpevents.Namespace, b64Env.Namespace)
	assert.Equal(t, bgpevents.EventUpdate, b64Env.Event)

	var gotRawLine string
	require.NoError(t, json.Unmarshal(b64Env.Payload, &gotRawLine))
	assert.Equal(t, b64Line, gotRawLine)

	var receipt bgptypes.UpdateReceipt
	require.NoError(t, json.Unmarshal(b64Env.Metadata, &receipt))
	assert.Equal(t, peer.Address, receipt.Peer)
	assert.Equal(t, peer.PeerOwner, receipt.PeerOwner)
	assert.Equal(t, msg.MessageID, receipt.MessageID)
	assert.Equal(t, msg.SourcePolicyEpoch, receipt.SourcePolicyEpoch)
	assert.Equal(t, msg.SourceReceipt, receipt.SourceReceipt)
	assert.True(t, receipt.ReactorForwarded)

	// 2. Regression control: EncodingJSON + FormatParsed keeps Payload as JSON object.
	parsedProc := process.NewProcess(plugin.PluginConfig{Name: "parsed-json-enveloped"})
	parsedProc.SetEnvelope(true)
	parsedProc.SetEncoding(plugin.EncodingJSON)
	parsedProc.SetFormat(plugin.FormatParsed)

	const parsedJSON = `{"type":"bgp","bgp":{"update":{}}}`
	wrappedParsed, err := cache.wrap(peer, msg, parsedProc, parsedJSON)
	require.NoError(t, err)

	var parsedEnv rpc.EventEnvelope
	require.NoError(t, json.Unmarshal([]byte(wrappedParsed), &parsedEnv))
	var payloadObj map[string]any
	require.NoError(t, json.Unmarshal(parsedEnv.Payload, &payloadObj))
	assert.Equal(t, "bgp", payloadObj["type"])

	// 3. Regression control: EncodingText still JSON-quotes text payload.
	textProc := process.NewProcess(plugin.PluginConfig{Name: "parsed-text-enveloped"})
	textProc.SetEnvelope(true)
	textProc.SetEncoding(plugin.EncodingText)
	textProc.SetFormat(plugin.FormatParsed)

	const textLine = "peer 10.0.0.1 received update 77\n"
	wrappedText, err := cache.wrap(peer, msg, textProc, textLine)
	require.NoError(t, err)

	var textEnv rpc.EventEnvelope
	require.NoError(t, json.Unmarshal([]byte(wrappedText), &textEnv))
	var gotTextLine string
	require.NoError(t, json.Unmarshal(textEnv.Payload, &gotTextLine))
	assert.Equal(t, textLine, gotTextLine)
}
