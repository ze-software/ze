package mcp

import (
	"encoding/base64"
	"net/http"
	"strings"
	"testing"
)

// TestHeaderMismatchRejected covers AC-6, AC-7 and AC-9.
//
// VALIDATES: every validation-failure condition the specification lists --
// a required standard header missing, a header value disagreeing with the
// body -- is answered with HTTP 400 and JSON-RPC -32020 (HeaderMismatch).
// PREVENTS: header/body confusion, where an intermediary routes on the header
// value while the server executes the body value.
func TestHeaderMismatchRejected(t *testing.T) {
	hs, cleanup := newTestStreamable(t, StreamableConfig{})
	defer cleanup()

	toolsCall := `{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"ze_reference","arguments":{},"_meta":` + metaBlock(ProtocolVersion, "{}") + `}}`
	toolsList := `{"jsonrpc":"2.0","id":1,"method":"tools/list","params":{"_meta":` + metaBlock(ProtocolVersion, "{}") + `}}`

	tests := []struct {
		name    string
		body    string
		headers map[string]string
	}{
		{
			name:    "MCP-Protocol-Version absent",
			body:    toolsList,
			headers: map[string]string{"Mcp-Method": "tools/list"},
		},
		{
			name: "MCP-Protocol-Version disagrees with _meta",
			body: toolsList,
			headers: map[string]string{
				"MCP-Protocol-Version": "2025-06-18",
				"Mcp-Method":           "tools/list",
			},
		},
		{
			name:    "Mcp-Method absent",
			body:    toolsList,
			headers: map[string]string{"MCP-Protocol-Version": ProtocolVersion},
		},
		{
			name: "Mcp-Method disagrees with body method",
			body: toolsList,
			headers: map[string]string{
				"MCP-Protocol-Version": ProtocolVersion,
				"Mcp-Method":           "tools/call",
			},
		},
		{
			name: "Mcp-Method differs only in case",
			body: toolsList,
			headers: map[string]string{
				"MCP-Protocol-Version": ProtocolVersion,
				"Mcp-Method":           "Tools/List",
			},
		},
		{
			name: "Mcp-Name absent on tools/call",
			body: toolsCall,
			headers: map[string]string{
				"MCP-Protocol-Version": ProtocolVersion,
				"Mcp-Method":           "tools/call",
			},
		},
		{
			name: "Mcp-Name disagrees with params.name",
			body: toolsCall,
			headers: map[string]string{
				"MCP-Protocol-Version": ProtocolVersion,
				"Mcp-Method":           "tools/call",
				"Mcp-Name":             "ze_execute",
			},
		},
		{
			name: "Mcp-Name sentinel payload is not valid Base64",
			body: toolsCall,
			headers: map[string]string{
				"MCP-Protocol-Version": ProtocolVersion,
				"Mcp-Method":           "tools/call",
				"Mcp-Name":             "=?base64?not-base64!!?=",
			},
		},
		{
			name: "Mcp-Name absent on resources/read",
			body: `{"jsonrpc":"2.0","id":1,"method":"resources/read","params":{"uri":"ui://x.html","_meta":` + metaBlock(ProtocolVersion, "{}") + `}}`,
			headers: map[string]string{
				"MCP-Protocol-Version": ProtocolVersion,
				"Mcp-Method":           "resources/read",
			},
		},
		{
			name: "Mcp-Name absent on prompts/get",
			body: `{"jsonrpc":"2.0","id":1,"method":"prompts/get","params":{"name":"review","_meta":` + metaBlock(ProtocolVersion, "{}") + `}}`,
			headers: map[string]string{
				"MCP-Protocol-Version": ProtocolVersion,
				"Mcp-Method":           "prompts/get",
			},
		},
		{
			// Header validation is a TRANSPORT guard and runs before dispatch,
			// so it wins even when the body is otherwise perfect: the tasks
			// capability is declared here, and the answer is still -32020 (which
			// carries no data object) rather than anything the handler would
			// have produced.
			name: "Mcp-Method disagrees on a tasks request that declared the capability",
			body: `{"jsonrpc":"2.0","id":1,"method":"tasks/get","params":{"taskId":"nonexistent","_meta":` + metaBlock(ProtocolVersion, capsTasks) + `}}`,
			headers: map[string]string{
				"MCP-Protocol-Version": ProtocolVersion,
				"Mcp-Method":           "tasks/cancel",
			},
		},
		{
			name: "Mcp-Name disagrees with params.name on prompts/get",
			body: `{"jsonrpc":"2.0","id":1,"method":"prompts/get","params":{"name":"review","_meta":` + metaBlock(ProtocolVersion, "{}") + `}}`,
			headers: map[string]string{
				"MCP-Protocol-Version": ProtocolVersion,
				"Mcp-Method":           "prompts/get",
				"Mcp-Name":             "summarize",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			status, parsed := postRaw(t, hs, tt.body, tt.headers)
			if status != http.StatusBadRequest {
				t.Fatalf("status = %d, want 400 (body %v)", status, parsed)
			}
			rpcErr, ok := parsed["error"].(map[string]any)
			if !ok {
				t.Fatalf("no error object in %v", parsed)
			}
			if rpcErr["code"] != float64(rpcHeaderMismatch) {
				t.Fatalf("code = %v, want %d", rpcErr["code"], rpcHeaderMismatch)
			}
			// The specification's HeaderMismatchError carries no data object;
			// the detail rides in message.
			if _, hasData := rpcErr["data"]; hasData {
				t.Fatalf("HeaderMismatchError must carry no data object, got %v", rpcErr["data"])
			}
			msg, _ := rpcErr["message"].(string)
			if msg == "" {
				t.Fatal("empty message on a header-mismatch rejection")
			}
			// A -32020 message must not reflect unvalidated header bytes back
			// to the client. Mcp-Name and MCP-Protocol-Version carry
			// attacker-controlled values; Mcp-Method is validated against the
			// body before any of them, and the method vocabulary appears in the
			// messages as documentation of the rule, not as an echo.
			for _, header := range []string{"Mcp-Name", "MCP-Protocol-Version"} {
				v := tt.headers[header]
				if v == "" || v == ProtocolVersion {
					continue
				}
				if strings.Contains(msg, v) {
					t.Fatalf("message %q echoes unvalidated %s value %q", msg, header, v)
				}
			}
		})
	}
}

// TestClientJSONRPCResponseIsRefused covers the SECOND half of
// spec-mcp2026-2-mrtr AC-3: "no client JSON-RPC response is ever accepted".
//
// Nothing tested it. The five TestStreamable_JSONRPCResponse* tests that drove
// the old intake path went with reply_sink_test.go in Phase 1, and the property
// survived only as arithmetic: a response frame has no `method`, and the
// Mcp-Method header "must repeat the body's method", so no header value could
// match. Correct, but unnamed and unasserted -- a later change to header
// validation (deriving the body method from the header, defaulting a missing
// one) could have reopened the intake path with every test still green.
//
// The refusal is now explicit (errBodyCarriesNoMethod, headers.go) and this
// pins it from the outside.
//
// VALIDATES: a POST whose body is a JSON-RPC response -- result-shaped or
// error-shaped, with a numeric or a string id -- is answered HTTP 400 and
// -32020, whatever headers accompany it (conformant for a real method, absent
// Mcp-Method, or an Mcp-Method naming the frame's own correlation target); the
// answer carries no `result`, so nothing was dispatched; and it is never the
// 202 Accepted that would mean "accepted, nothing to say".
// PREVENTS: a client-to-server response frame becoming servable again. This
// server writes no JSON-RPC request to a client, so any response it accepted
// would be correlated against nothing -- the exact half-open channel MCP
// 2026-07-28 removed when MRTR replaced server-initiated elicitation.
func TestClientJSONRPCResponseIsRefused(t *testing.T) {
	hs, cleanup := newTestStreamable(t, StreamableConfig{})
	defer cleanup()

	// Every frame below is a well-formed JSON-RPC RESPONSE: `jsonrpc`, an `id`,
	// and a `result` or `error` member. None carries a `method`, which is what
	// makes it a response rather than a request.
	frames := []struct {
		name string
		body string
	}{
		{
			name: "elicitation accept, the shape the deleted intake path took",
			body: `{"jsonrpc":"2.0","id":1,"result":{"action":"accept","content":{"answer":"42"}}}`,
		},
		{
			name: "string id",
			body: `{"jsonrpc":"2.0","id":"bogus-id","result":{"action":"accept","content":{}}}`,
		},
		{
			name: "empty string id",
			body: `{"jsonrpc":"2.0","id":"","result":{"action":"accept"}}`,
		},
		{
			name: "error response",
			body: `{"jsonrpc":"2.0","id":7,"error":{"code":-32000,"message":"client refused"}}`,
		},
		{
			name: "result carrying a nested method, which is still not a request",
			body: `{"jsonrpc":"2.0","id":2,"result":{"method":"elicitation/create"}}`,
		},
	}

	// Three header shapes a client might pair with such a frame. None may
	// rescue it: the refusal is a property of the BODY.
	headerShapes := []struct {
		name    string
		headers map[string]string
	}{
		{
			name: "conformant headers for a real method",
			headers: map[string]string{
				"MCP-Protocol-Version": ProtocolVersion,
				"Mcp-Method":           methodToolsList,
			},
		},
		{
			name:    "no Mcp-Method header",
			headers: map[string]string{"MCP-Protocol-Version": ProtocolVersion},
		},
		{
			name:    "no standard headers at all",
			headers: nil,
		},
		{
			name: "Mcp-Method naming the request this frame would be answering",
			headers: map[string]string{
				"MCP-Protocol-Version": ProtocolVersion,
				"Mcp-Method":           "elicitation/create",
			},
		},
	}

	for _, frame := range frames {
		for _, shape := range headerShapes {
			t.Run(frame.name+"/"+shape.name, func(t *testing.T) {
				status, parsed := postRaw(t, hs, frame.body, shape.headers)

				// 202 is the acknowledgement a notification earns. A response
				// frame reaching it would mean the transport accepted the frame
				// and simply had nothing to say about it, which is the failure
				// this test exists to catch, not a pass.
				if status == http.StatusAccepted {
					t.Fatalf("a client JSON-RPC response was ACCEPTED with 202 (body %v)", parsed)
				}
				if status != http.StatusBadRequest {
					t.Fatalf("status = %d, want 400 (body %v)", status, parsed)
				}
				rpcErr := rpcErrorOf(t, parsed)
				if rpcErr["code"] != float64(rpcHeaderMismatch) {
					t.Fatalf("code = %v, want %d", rpcErr["code"], rpcHeaderMismatch)
				}
				// Nothing was dispatched: no handler ran, so there is no result
				// object and no serverInfo envelope on this answer.
				if got, present := parsed["result"]; present {
					t.Fatalf("a refused response frame produced a result: %v", got)
				}
				msg, _ := rpcErr["message"].(string)
				if !strings.Contains(msg, "carries no") {
					t.Errorf("message %q does not name the missing body method; a caller cannot tell this "+
						"from an ordinary header typo", msg)
				}
			})
		}
	}
}

// TestMcpNameSourceField pins WHICH body field each Mcp-Name-carrying method
// mirrors, and proves a conformant header reaches dispatch.
//
// VALIDATES: tools/call and prompts/get mirror `params.name`, resources/read
// mirrors `params.uri`, and every other method requires no Mcp-Name at all. A
// conformant prompts/get therefore reaches the 404 + -32601 the specification
// mandates for a method the server does not implement.
// PREVENTS: the regression this test was written for -- prompts/get sourced
// from `params.uri`, a field GetPromptRequest never carries, so the header
// never matched, every prompts/get died at -32020, and the mandated 404 was
// unreachable.
func TestMcpNameSourceField(t *testing.T) {
	t.Run("source field per method", func(t *testing.T) {
		params := map[string]any{"name": "review", "uri": "ui://x.html"}
		cases := []struct {
			method   string
			want     string
			required bool
		}{
			{methodToolsCall, "review", true},
			{methodPromptsGet, "review", true},
			{methodResourcesRead, "ui://x.html", true},
			{methodToolsList, "", false},
			{methodServerDiscover, "", false},
			{methodTasksUpdate, "", false},
		}
		for _, tc := range cases {
			t.Run(tc.method, func(t *testing.T) {
				got, required := mcpNameSource(tc.method, params)
				if required != tc.required {
					t.Fatalf("mcpNameSource(%q) required = %v, want %v", tc.method, required, tc.required)
				}
				if got != tc.want {
					t.Fatalf("mcpNameSource(%q) = %q, want %q", tc.method, got, tc.want)
				}
			})
		}
	})

	t.Run("conformant prompts/get reaches the mandated 404", func(t *testing.T) {
		hs, cleanup := newTestStreamable(t, StreamableConfig{})
		defer cleanup()

		body := `{"jsonrpc":"2.0","id":1,"method":"prompts/get","params":{"name":"review","_meta":` +
			metaBlock(ProtocolVersion, "{}") + `}}`
		status, parsed := postRaw(t, hs, body, map[string]string{
			"MCP-Protocol-Version": ProtocolVersion,
			"Mcp-Method":           "prompts/get",
			"Mcp-Name":             "review",
		})
		if status != http.StatusNotFound {
			t.Fatalf("status = %d, want 404 (body %v)", status, parsed)
		}
		rpcErr, ok := parsed["error"].(map[string]any)
		if !ok {
			t.Fatalf("no error object in %v", parsed)
		}
		if rpcErr["code"] != float64(rpcMethodNotFound) {
			t.Fatalf("code = %v, want %d -- header validation swallowed the request", rpcErr["code"], rpcMethodNotFound)
		}
	})
}

// VALIDATES: a header name matching case-insensitively is accepted, since
// RFC 9110 field names are case-insensitive.
// PREVENTS: rejecting a conformant client that spells `mcp-method` lowercase.
func TestHeaderNamesAreCaseInsensitive(t *testing.T) {
	hs, cleanup := newTestStreamable(t, StreamableConfig{})
	defer cleanup()

	body := `{"jsonrpc":"2.0","id":1,"method":"tools/list","params":{"_meta":` + metaBlock(ProtocolVersion, "{}") + `}}`
	status, parsed := postRaw(t, hs, body, map[string]string{
		"mcp-protocol-version": ProtocolVersion,
		"mcp-method":           "tools/list",
	})
	if status != http.StatusOK {
		t.Fatalf("status = %d, want 200 (body %v)", status, parsed)
	}
}

// TestMcpNameBase64Sentinel covers AC-8.
//
// VALIDATES: an Mcp-Name carrying the =?base64?...?= sentinel is decoded
// before it is compared with the body value, and the markers are matched
// case-sensitively in their lowercase form.
// PREVENTS: comparing the encoded header against the plain body value, which
// would reject every conformant client whose tool name is not header-safe.
func TestMcpNameBase64Sentinel(t *testing.T) {
	t.Run("decodeSentinel", func(t *testing.T) {
		tests := []struct {
			name   string
			in     string
			want   string
			wantOK bool
		}{
			{name: "plain value passes through", in: "ze_reference", want: "ze_reference", wantOK: true},
			{name: "empty value passes through", in: "", want: "", wantOK: true},
			{name: "padded standard base64", in: "=?base64?SGVsbG8sIOS4lueVjA==?=", want: "Hello, 世界", wantOK: true},
			{name: "leading and trailing space", in: "=?base64?IHBhZGRlZCA=?=", want: " padded ", wantOK: true},
			{name: "alphabet includes slash", in: "=?base64?PT9iYXNlNjQ/bGl0ZXJhbD89?=", want: "=?base64?literal?=", wantOK: true},
			{name: "empty payload", in: "=?base64??=", want: "", wantOK: true},
			{name: "uppercase marker is not the sentinel", in: "=?BASE64?SGk=?=", want: "=?BASE64?SGk=?=", wantOK: true},
			{name: "prefix without suffix is plain", in: "=?base64?SGk=", want: "=?base64?SGk=", wantOK: true},
			{name: "overlapping prefix and suffix", in: "=?base64?=", wantOK: false},
			{name: "payload not base64", in: "=?base64?not base64!!?=", wantOK: false},
			{name: "unpadded payload is rejected", in: "=?base64?SGVsbG8sIOS4lueVjA?=", wantOK: false},
		}
		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				got, ok := decodeSentinel(tt.in)
				if ok != tt.wantOK {
					t.Fatalf("decodeSentinel(%q) ok = %v, want %v", tt.in, ok, tt.wantOK)
				}
				if ok && got != tt.want {
					t.Fatalf("decodeSentinel(%q) = %q, want %q", tt.in, got, tt.want)
				}
			})
		}
	})

	t.Run("accepted end to end", func(t *testing.T) {
		hs, cleanup := newTestStreamable(t, StreamableConfig{})
		defer cleanup()
		body := `{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"ze_reference","arguments":{},"_meta":` + metaBlock(ProtocolVersion, "{}") + `}}`
		encoded := "=?base64?" + base64.StdEncoding.EncodeToString([]byte("ze_reference")) + "?="
		status, parsed := postRaw(t, hs, body, map[string]string{
			"MCP-Protocol-Version": ProtocolVersion,
			"Mcp-Method":           "tools/call",
			"Mcp-Name":             encoded,
		})
		if status != http.StatusOK {
			t.Fatalf("status = %d, want 200 (body %v)", status, parsed)
		}
		if _, isErr := parsed["error"]; isErr {
			t.Fatalf("unexpected error: %v", parsed)
		}
	})
}

// VALIDATES: an Mcp-Param-* header whose value carries octets HTTP field
// values do not permit is rejected with -32020.
// PREVENTS: forwarding a control character from a mirrored tool parameter into
// a downstream consumer that trusts the header.
func TestMcpParamHeaderInvalidCharacters(t *testing.T) {
	if validFieldValue("us-west1") != true {
		t.Error("plain ASCII value rejected")
	}
	if validFieldValue("with\ttab") != true {
		t.Error("horizontal tab rejected; RFC 9110 permits it")
	}
	if validFieldValue("line1\nline2") {
		t.Error("newline accepted in a field value")
	}
	if validFieldValue("bell\x07") {
		t.Error("control character accepted in a field value")
	}
	if validFieldValue("café") {
		t.Error("non-ASCII accepted in a field value; it needs the Base64 sentinel")
	}
}

// TestLegacyInitializeNamesSupportedVersion covers AC-18.
//
// VALIDATES: a legacy client's header-less `initialize` POST is rejected with
// HTTP 400 + -32020 whose message names the protocol version this server
// supports.
// PREVENTS: answering a legacy client with a bare "missing header" that gives
// it nothing to act on -- it has no fall-forward mechanism, so this message
// may be the only diagnostic it can surface.
func TestLegacyInitializeNamesSupportedVersion(t *testing.T) {
	hs, cleanup := newTestStreamable(t, StreamableConfig{})
	defer cleanup()

	body := `{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2025-06-18","capabilities":{}}}`
	status, parsed := postRaw(t, hs, body, nil)
	if status != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400 (body %v)", status, parsed)
	}
	rpcErr, ok := parsed["error"].(map[string]any)
	if !ok {
		t.Fatalf("no error object in %v", parsed)
	}
	if rpcErr["code"] != float64(rpcHeaderMismatch) {
		t.Fatalf("code = %v, want %d", rpcErr["code"], rpcHeaderMismatch)
	}
	msg, _ := rpcErr["message"].(string)
	if !strings.Contains(msg, ProtocolVersion) {
		t.Fatalf("message %q does not name the supported protocol version %q", msg, ProtocolVersion)
	}
	// No session id is minted for a legacy handshake attempt.
	if got := parsed["result"]; got != nil {
		t.Fatalf("initialize produced a result: %v", got)
	}
}

// VALIDATES: an `initialize` POST that DOES carry conformant headers still
// reaches the unknown-method path, and that error also names the supported
// version.
// PREVENTS: a modern-transport client probing `initialize` getting a
// diagnostic it cannot act on.
func TestInitializeWithHeadersIsUnknownMethod(t *testing.T) {
	hs, cleanup := newTestStreamable(t, StreamableConfig{})
	defer cleanup()

	body := `{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"_meta":` + metaBlock(ProtocolVersion, "{}") + `}}`
	status, parsed := postRaw(t, hs, body, map[string]string{
		"MCP-Protocol-Version": ProtocolVersion,
		"Mcp-Method":           "initialize",
	})
	if status != http.StatusNotFound {
		t.Fatalf("status = %d, want 404 (body %v)", status, parsed)
	}
	rpcErr, ok := parsed["error"].(map[string]any)
	if !ok {
		t.Fatalf("no error object in %v", parsed)
	}
	if rpcErr["code"] != float64(rpcMethodNotFound) {
		t.Fatalf("code = %v, want %d", rpcErr["code"], rpcMethodNotFound)
	}
	msg, _ := rpcErr["message"].(string)
	if !strings.Contains(msg, ProtocolVersion) {
		t.Fatalf("message %q does not name the supported protocol version", msg)
	}
}
