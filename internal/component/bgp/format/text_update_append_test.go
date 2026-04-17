package format

import (
	"encoding/json"
	"net/netip"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"codeberg.org/thomas-mangin/ze/internal/component/bgp/message"
	bgptypes "codeberg.org/thomas-mangin/ze/internal/component/bgp/types"
	"codeberg.org/thomas-mangin/ze/internal/component/bgp/wireu"
	"codeberg.org/thomas-mangin/ze/internal/component/plugin"
)

// VALIDATES: AC-2 -- AppendSentMessage emits `"message":{"type":"sent"` in
// the JSON envelope for every UPDATE format (parsed/raw/full/summary). The
// legacy FormatSentMessage achieved this via strings.Replace on the output
// of FormatMessage; the Append form threads messageType through to the
// writers so the type is written at source.
// PREVENTS: regressions where the sent indicator reverts to "update" (the
// received envelope type) -- plugins subscribed to the sent event would
// receive payloads indistinguishable from received messages.
func TestAppendSentMessage_TypeIsSent(t *testing.T) {
	t.Parallel()

	ctxID := testEncodingContext()

	peer := plugin.PeerInfo{
		Address: netip.MustParseAddr("10.0.0.1"),
		PeerAS:  65001,
	}

	body := buildTestUpdateBodyWithAttrs(
		netip.MustParsePrefix("192.168.1.0/24"),
		netip.MustParseAddr("10.0.0.1"),
		0, 0, nil,
	)
	wireUpdate := wireu.NewWireUpdate(body, ctxID)
	attrsWire, err := wireUpdate.Attrs()
	require.NoError(t, err)

	msg := bgptypes.RawMessage{
		Type:       message.TypeUPDATE,
		RawBytes:   body,
		AttrsWire:  attrsWire,
		WireUpdate: wireUpdate,
		Direction:  "received", // deliberately wrong to prove AppendSentMessage overrides it
		MessageID:  42,
	}

	tests := []struct {
		name   string
		format string
	}{
		{"parsed", plugin.FormatParsed},
		{"raw", plugin.FormatRaw},
		{"full", plugin.FormatFull},
		{"summary", plugin.FormatSummary},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			content := bgptypes.ContentConfig{
				Encoding: plugin.EncodingJSON,
				Format:   tt.format,
			}
			output := string(AppendSentMessage(nil, &peer, msg, content))

			// Direct substring check -- the envelope MUST contain
			// `"message":{"type":"sent"` (immediately after the message
			// wrapper opens) and MUST NOT contain `"type":"update"` at the
			// message-type position.
			assert.Contains(t, output, `"message":{"type":"sent"`,
				"sent envelope must write type:sent at source, not update: %s", output)
			assert.NotContains(t, output, `"message":{"type":"update"`,
				"sent envelope must not carry the received envelope type: %s", output)

			// Parse to confirm structure is valid JSON + direction field
			// is threaded as "sent" too.
			var parsed map[string]any
			require.NoError(t, json.Unmarshal([]byte(output), &parsed),
				"output must be valid JSON: %s", output)
			payload := getBGPPayload(t, parsed)
			msgWrapper, ok := payload["message"].(map[string]any)
			require.True(t, ok, "message wrapper must be present")
			assert.Equal(t, "sent", msgWrapper["type"], "message.type must be 'sent'")
			if dir, ok := msgWrapper["direction"]; ok {
				assert.Equal(t, "sent", dir, "message.direction must be 'sent'")
			}
		})
	}
}

// VALIDATES: AC-2 follow-up -- the banned `strings.Replace` pattern does not
// reappear in text_update.go's call graph. The check is structural (grep at
// build time) rather than behavioral, but this test is the assertion that
// documents the intent.
// PREVENTS: silent reintroduction of the legacy "type":"update" -> "sent"
// string substitution as a seemingly-innocuous helper.
func TestAppendSentMessage_NoStringsReplaceSurgery(t *testing.T) {
	t.Parallel()
	// Building a sent-format envelope and checking the output does not
	// contain duplicated `"type":"update"` followed by `"type":"sent"`
	// (which would be the signature of a string replace pass that somehow
	// missed an occurrence).
	ctxID := testEncodingContext()
	peer := plugin.PeerInfo{
		Address: netip.MustParseAddr("10.0.0.1"),
		PeerAS:  65001,
	}
	body := buildTestUpdateBodyWithAttrs(
		netip.MustParsePrefix("192.168.1.0/24"),
		netip.MustParseAddr("10.0.0.1"),
		0, 0, nil,
	)
	wireUpdate := wireu.NewWireUpdate(body, ctxID)
	attrsWire, err := wireUpdate.Attrs()
	require.NoError(t, err)

	msg := bgptypes.RawMessage{
		Type:       message.TypeUPDATE,
		RawBytes:   body,
		AttrsWire:  attrsWire,
		WireUpdate: wireUpdate,
		Direction:  "received",
		MessageID:  7,
	}
	content := bgptypes.ContentConfig{
		Encoding: plugin.EncodingJSON,
		Format:   plugin.FormatParsed,
	}
	output := string(AppendSentMessage(nil, &peer, msg, content))
	assert.Equal(t, 0, strings.Count(output, `"message":{"type":"update"`),
		`"type":"update" must not appear anywhere in a sent envelope: %s`, output)
	assert.Equal(t, 1, strings.Count(output, `"message":{"type":"sent"`),
		`exactly one "type":"sent" must appear in the sent envelope: %s`, output)
}
