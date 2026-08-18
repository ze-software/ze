//go:build ze_web

// Design: ai/rules/cli.md -- a response payload is structured data, never
// text a renderer already formatted.

package hub

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/command"
	pluginreg "github.com/ze-software/ze/internal/component/plugin/registry"
)

// keepaliveHex is a complete BGP KEEPALIVE: the 16-byte marker, length 19,
// type 4. The decoder needs no negotiated state to read it.
const keepaliveHex = "FFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFF001304"

// decodedMessageType reads bgp.message.type out of a decoder payload. It fails
// the test when the payload is not the JSON object the decoder must answer, so
// a payload that has gone back to pre-rendered text is a red test rather than a
// silently skipped assertion.
func decodedMessageType(t *testing.T, output string) string {
	t.Helper()
	var payload map[string]any
	require.NoError(t, json.Unmarshal([]byte(output), &payload), "payload must be structured JSON, got %q", output)
	bgp, ok := payload["bgp"].(map[string]any)
	require.True(t, ok, "payload must carry a bgp section, got %q", output)
	msg, ok := bgp["message"].(map[string]any)
	require.True(t, ok, "bgp section must carry a message, got %q", output)
	msgType, ok := msg["type"].(string)
	require.True(t, ok, "message must carry a type, got %q", output)
	return msgType
}

// TestBGPDecodePayloadRendersInEveryFormat pins the invariant that makes the
// pipe operators work: the decode command answers structured data, so each of
// "| json", "| yaml" and "| table" renders the SAME payload its own way.
//
// VALIDATES: ai/rules/cli.md -- a response payload MUST NOT be text a renderer
// already formatted.
// PREVENTS: the plugin.Text escape returning, where the decoder was asked for
// human text (outputJSON=false) and every format operator passed it through
// unchanged, so an operator asking for JSON got the text rendering.
func TestBGPDecodePayloadRendersInEveryFormat(t *testing.T) {
	dispatch := withBGPDecode(nil)
	rendered, err := dispatch.JSON(context.Background(), wodCaller, "show bgp decode "+keepaliveHex)
	defer rendered.TransportComplete()
	if !bgpDecodeLinked {
		// No decoder in this build, so there is no payload to render. The
		// honest answer is the daemon-required error, and asserting it keeps
		// this test from passing vacuously in a BGP-less binary.
		require.ErrorIs(t, err, errWebOnlyUnavailable)
		return
	}
	require.NoError(t, err)

	payload := rendered.Output
	require.Equal(t, "keepalive", decodedMessageType(t, payload))

	for _, tc := range []struct {
		format string
		want   string
	}{
		{"json", `"type": "keepalive"`},
		{"yaml", "type: keepalive"},
		{"table", "keepalive"},
	} {
		t.Run(tc.format, func(t *testing.T) {
			cmdStr, formatFn, pipeErr := command.ProcessPipesChecked("show bgp decode " + keepaliveHex + " | " + tc.format)
			require.Empty(t, pipeErr)
			require.Equal(t, "show bgp decode "+keepaliveHex, cmdStr)

			out := formatFn(payload)
			assert.Contains(t, out, tc.want, "%s must render the decoded message type", tc.format)
			assert.NotEqual(t, payload, out, "%s must transform the payload; an unchanged answer means the renderer could not read it", tc.format)
		})
	}

	assert.True(t, json.Valid([]byte(payload)), "the payload itself must be valid JSON")
}

// TestBGPDecodeTextPayloadRendersInNoFormat is the discrimination proof for the
// test above. It asks the same decoder for the human rendering the removed
// plugin.Text path carried, and runs the SAME three format operators over it.
// Every one of them returns its input unchanged, so each assertion in
// TestBGPDecodePayloadRendersInEveryFormat fails on that payload.
//
// This is why "the command answers structured data" is the invariant and not a
// preference: a payload a renderer already formatted leaves "| json", "| yaml"
// and "| table" nothing to render, and each one silently answers the text it
// was given (ai/rules/cli.md).
//
// VALIDATES: the test above discriminates.
// PREVENTS: a green format-pipe test over a payload no format operator reads.
func TestBGPDecodeTextPayloadRendersInNoFormat(t *testing.T) {
	decode := pluginreg.GetPacketDecoder()
	if !bgpDecodeLinked {
		require.Nil(t, decode, "a build without BGP must expose no decoder")
		return
	}
	require.NotNil(t, decode)

	text, err := decode(keepaliveHex, "", "", false)
	require.NoError(t, err)
	require.NotEmpty(t, text)

	for _, format := range []string{"json", "yaml", "table"} {
		t.Run(format, func(t *testing.T) {
			_, formatFn, pipeErr := command.ProcessPipesChecked("show bgp decode " + keepaliveHex + " | " + format)
			require.Empty(t, pipeErr)

			assert.Equal(t, text, formatFn(text), "a pre-rendered payload passes through %s unchanged, which is the escape this invariant closes", format)
		})
	}
}

// TestBGPDecodeReportsADecoderError proves the wrapper fails closed on bad
// input: the decoder's error reaches the caller instead of an empty or
// half-structured response.
//
// VALIDATES: ai/rules/evidence.md -- a guard fails closed or says something.
// PREVENTS: an unreadable hex string producing an empty success payload.
func TestBGPDecodeReportsADecoderError(t *testing.T) {
	dispatch := withBGPDecode(nil)
	rendered, err := dispatch.JSON(context.Background(), wodCaller, "show bgp decode ZZZZ")
	defer rendered.TransportComplete()
	require.Error(t, err)
	assert.Empty(t, rendered.Output)
	if bgpDecodeLinked {
		assert.Contains(t, err.Error(), "invalid hex")
	}
}
