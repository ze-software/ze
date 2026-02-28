package rpc

import (
	"context"
	"io"
	"net"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// --- Phase 2: Framing tests ---

// TestTextLineFraming verifies multi-line text messages frame correctly over net.Pipe.
//
// VALIDATES: Stage messages write as newline-separated lines and read back with blank-line terminator.
// PREVENTS: Lost lines, broken framing, or scanner state corruption across sequential reads.
func TestTextLineFraming(t *testing.T) {
	t.Parallel()

	clientEnd, serverEnd := net.Pipe()
	defer closePipe(t, "clientEnd", clientEnd)
	defer closePipe(t, "serverEnd", serverEnd)

	writer := NewTextConn(serverEnd, serverEnd)
	reader := NewTextConn(clientEnd, clientEnd)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Format a registration message (multi-line)
	regInput := DeclareRegistrationInput{
		Families:     []FamilyDecl{{Name: "ipv4/unicast", Mode: "both"}},
		Dependencies: []string{"bgp-rib"},
	}
	regText, err := FormatRegistrationText(regInput)
	require.NoError(t, err)

	// Format a capabilities message (second sequential message)
	capInput := DeclareCapabilitiesInput{
		Capabilities: []CapabilityDecl{
			{Code: 65, Encoding: "hex", Payload: "0001"},
		},
	}
	capText, err := FormatCapabilitiesText(capInput)
	require.NoError(t, err)

	// Send both messages + responses from goroutine (net.Pipe is synchronous).
	// Errors are collected via channel rather than assert on t, because the
	// goroutine may outlive the test function on early failure.
	writerDone := make(chan error, 1)
	go func() {
		for _, writeErr := range []error{
			writer.WriteMessage(ctx, regText),
			writer.WriteLine(ctx, "ok"),
			writer.WriteMessage(ctx, capText),
			writer.WriteLine(ctx, "ok"),
		} {
			if writeErr != nil {
				writerDone <- writeErr
				return
			}
		}
		writerDone <- nil
	}()

	// Read first message → parse → verify
	got1, err := reader.ReadMessage(ctx)
	require.NoError(t, err)
	parsed1, err := ParseRegistrationText(got1)
	require.NoError(t, err)
	assert.Equal(t, regInput.Families, parsed1.Families)
	assert.Equal(t, regInput.Dependencies, parsed1.Dependencies)

	// Read first response
	resp1, err := reader.ReadLine(ctx)
	require.NoError(t, err)
	assert.Equal(t, "ok", resp1)

	// Read second message → parse → verify (scanner state preserved)
	got2, err := reader.ReadMessage(ctx)
	require.NoError(t, err)
	parsed2, err := ParseCapabilitiesText(got2)
	require.NoError(t, err)
	assert.Equal(t, capInput.Capabilities, parsed2.Capabilities)

	// Read second response
	resp2, err := reader.ReadLine(ctx)
	require.NoError(t, err)
	assert.Equal(t, "ok", resp2)
}

// TestTextLineFramingEdgeCases verifies edge cases for text line framing.
//
// VALIDATES: Minimal messages, long lines, and heredoc content all frame correctly.
// PREVENTS: Framing failures on minimal or unusual message shapes.
func TestTextLineFramingEdgeCases(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		send    string
		wantMsg string
	}{
		{
			name:    "minimal message (verb only)",
			send:    "register\n\n",
			wantMsg: "register\n",
		},
		{
			name:    "long capability payload",
			send:    "capabilities\ncode 65 encoding hex payload " + strings.Repeat("AB", 2000) + "\n\n",
			wantMsg: "capabilities\ncode 65 encoding hex payload " + strings.Repeat("AB", 2000) + "\n",
		},
		{
			name:    "heredoc config preserves internal structure",
			send:    "configure\nroot bgp json << END\n{\"asn\":65000,\"router-id\":\"1.1.1.1\"}\nEND\n\n",
			wantMsg: "configure\nroot bgp json << END\n{\"asn\":65000,\"router-id\":\"1.1.1.1\"}\nEND\n",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			clientEnd, serverEnd := net.Pipe()
			defer closePipe(t, "clientEnd", clientEnd)
			defer closePipe(t, "serverEnd", serverEnd)

			reader := NewTextConn(clientEnd, clientEnd)
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()

			go func() {
				_, writeErr := io.WriteString(serverEnd, tt.send)
				assert.NoError(t, writeErr)
			}()

			got, readErr := reader.ReadMessage(ctx)
			require.NoError(t, readErr)
			assert.Equal(t, tt.wantMsg, got)
		})
	}
}

// --- Phase 1: Serialization tests ---

// TestTextRegistrationRoundTrip verifies text format/parse round-trip for DeclareRegistrationInput.
//
// VALIDATES: Registration with families, commands, deps, config roots serializes to text and parses back identically.
// PREVENTS: Data loss during text handshake stage 1.
func TestTextRegistrationRoundTrip(t *testing.T) {
	t.Parallel()

	input := DeclareRegistrationInput{
		Families: []FamilyDecl{
			{Name: "ipv4/unicast", Mode: "both"},
			{Name: "ipv6/unicast", Mode: "encode"},
		},
		Commands: []CommandDecl{
			{Name: "rib-show", Description: "Show RIB entries", Args: []string{"peer", "type"}, Completable: true},
			{Name: "rib-clear", Description: "Clear RIB"},
		},
		Dependencies: []string{"bgp-rib"},
		WantsConfig:  []string{"bgp", "bgp/peer"},
		Schema: &SchemaDecl{
			Module:    "bgp-rs",
			Namespace: "urn:bgp-rs",
			Handlers:  []string{"bgp:route-reflection"},
		},
		WantsValidateOpen:      true,
		CacheConsumer:          true,
		CacheConsumerUnordered: false,
	}

	text, err := FormatRegistrationText(input)
	require.NoError(t, err)

	parsed, err := ParseRegistrationText(text)
	require.NoError(t, err)

	assert.Equal(t, input.Families, parsed.Families)
	assert.Equal(t, input.Commands, parsed.Commands)
	assert.Equal(t, input.Dependencies, parsed.Dependencies)
	assert.Equal(t, input.WantsConfig, parsed.WantsConfig)
	assert.Equal(t, input.Schema.Module, parsed.Schema.Module)
	assert.Equal(t, input.Schema.Namespace, parsed.Schema.Namespace)
	assert.Equal(t, input.Schema.Handlers, parsed.Schema.Handlers)
	assert.Equal(t, input.WantsValidateOpen, parsed.WantsValidateOpen)
	assert.Equal(t, input.CacheConsumer, parsed.CacheConsumer)
	assert.Equal(t, input.CacheConsumerUnordered, parsed.CacheConsumerUnordered)
}

// TestTextRegistrationEdgeCases verifies edge cases for registration text format.
//
// VALIDATES: Empty fields, quoted descriptions, zero-length lists all round-trip correctly.
// PREVENTS: Parsing failures on minimal or unusual registration data.
func TestTextRegistrationEdgeCases(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		input DeclareRegistrationInput
	}{
		{
			name:  "empty registration",
			input: DeclareRegistrationInput{},
		},
		{
			name: "command with spaces in description",
			input: DeclareRegistrationInput{
				Commands: []CommandDecl{
					{Name: "show", Description: "Show all the routing entries in detail"},
				},
			},
		},
		{
			name: "single family no extras",
			input: DeclareRegistrationInput{
				Families: []FamilyDecl{{Name: "l2vpn/evpn", Mode: "decode"}},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			text, err := FormatRegistrationText(tt.input)
			require.NoError(t, err)

			parsed, err := ParseRegistrationText(text)
			require.NoError(t, err)

			assert.Equal(t, tt.input, parsed)
		})
	}
}

// TestTextConfigHeredocRoundTrip verifies text format/parse round-trip for ConfigureInput with heredoc.
//
// VALIDATES: Config sections use heredoc-delimited JSON that round-trips correctly.
// PREVENTS: Data corruption in stage 2 config delivery.
func TestTextConfigHeredocRoundTrip(t *testing.T) {
	t.Parallel()

	input := ConfigureInput{
		Sections: []ConfigSection{
			{Root: "bgp", Data: `{"asn":65000,"router-id":"1.1.1.1"}`},
			{Root: "bgp/peer", Data: `{"address":"192.168.1.1","remote-as":65001}`},
		},
	}

	text, err := FormatConfigureText(input)
	require.NoError(t, err)

	parsed, err := ParseConfigureText(text)
	require.NoError(t, err)

	require.Len(t, parsed.Sections, 2)
	assert.Equal(t, "bgp", parsed.Sections[0].Root)
	assert.Equal(t, input.Sections[0].Data, parsed.Sections[0].Data)
	assert.Equal(t, "bgp/peer", parsed.Sections[1].Root)
	assert.Equal(t, input.Sections[1].Data, parsed.Sections[1].Data)
}

// TestTextConfigHeredocEdgeCases verifies edge cases for heredoc config delivery.
//
// VALIDATES: Empty config, nested JSON, and config with END substring in values all work.
// PREVENTS: Heredoc terminator collision or empty-config failures.
func TestTextConfigHeredocEdgeCases(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		input ConfigureInput
	}{
		{
			name:  "empty sections",
			input: ConfigureInput{},
		},
		{
			name: "nested JSON arrays and objects",
			input: ConfigureInput{
				Sections: []ConfigSection{
					{Root: "bgp", Data: `{"peers":[{"address":"10.0.0.1"},{"address":"10.0.0.2"}],"options":{"extended":true}}`},
				},
			},
		},
		{
			name: "value containing END substring",
			input: ConfigureInput{
				Sections: []ConfigSection{
					{Root: "bgp", Data: `{"description":"ENDPOINT for backend","vendor":"ENDURA"}`},
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			text, err := FormatConfigureText(tt.input)
			require.NoError(t, err)

			parsed, err := ParseConfigureText(text)
			require.NoError(t, err)

			assert.Equal(t, tt.input, parsed)
		})
	}
}

// TestTextCapabilitiesRoundTrip verifies text format/parse round-trip for DeclareCapabilitiesInput.
//
// VALIDATES: Capabilities with codes, encodings, payloads, and peer filters round-trip correctly.
// PREVENTS: Data loss during text handshake stage 3.
func TestTextCapabilitiesRoundTrip(t *testing.T) {
	t.Parallel()

	input := DeclareCapabilitiesInput{
		Capabilities: []CapabilityDecl{
			{Code: 9, Encoding: "hex", Payload: "0180"},
			{Code: 128, Encoding: "text", Payload: "hostname=router1.example.com", Peers: []string{"192.168.1.1", "10.0.0.1"}},
			{Code: 65, Encoding: "hex", Payload: "0001000100040000FDE8"},
		},
	}

	text, err := FormatCapabilitiesText(input)
	require.NoError(t, err)

	parsed, err := ParseCapabilitiesText(text)
	require.NoError(t, err)

	require.Len(t, parsed.Capabilities, 3)
	assert.Equal(t, input.Capabilities[0], parsed.Capabilities[0])
	assert.Equal(t, input.Capabilities[1], parsed.Capabilities[1])
	assert.Equal(t, input.Capabilities[2], parsed.Capabilities[2])
}

// TestTextRegistryRoundTrip verifies text format/parse round-trip for ShareRegistryInput.
//
// VALIDATES: Registry commands round-trip through text format.
// PREVENTS: Data loss during text handshake stage 4.
func TestTextRegistryRoundTrip(t *testing.T) {
	t.Parallel()

	input := ShareRegistryInput{
		Commands: []RegistryCommand{
			{Name: "rib-show", Plugin: "bgp-rib", Encoding: "json"},
			{Name: "announce", Plugin: "bgp-plugin", Encoding: "text"},
			{Name: "withdraw", Plugin: "bgp-plugin"},
		},
	}

	text, err := FormatRegistryText(input)
	require.NoError(t, err)

	parsed, err := ParseRegistryText(text)
	require.NoError(t, err)

	require.Len(t, parsed.Commands, 3)
	assert.Equal(t, input.Commands, parsed.Commands)
}

// TestTextReadyRoundTrip verifies text format/parse round-trip for ReadyInput.
//
// VALIDATES: Ready with and without subscriptions round-trips correctly.
// PREVENTS: Lost subscription params during text handshake stage 5.
func TestTextReadyRoundTrip(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		input ReadyInput
	}{
		{
			name:  "bare ready",
			input: ReadyInput{},
		},
		{
			name: "ready with subscriptions",
			input: ReadyInput{
				Subscribe: &SubscribeEventsInput{
					Events:   []string{"bgp:update", "bgp:open"},
					Encoding: "text",
					Peers:    []string{"192.168.1.1"},
				},
			},
		},
		{
			name: "ready with events only",
			input: ReadyInput{
				Subscribe: &SubscribeEventsInput{
					Events: []string{"bgp:update"},
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			text, err := FormatReadyText(tt.input)
			require.NoError(t, err)

			parsed, err := ParseReadyText(text)
			require.NoError(t, err)

			assert.Equal(t, tt.input, parsed)
		})
	}
}
