package cli

import (
	"strings"
	"testing"
)

// elicitRequest builds an inputRequests entry the way a server would, so the
// client-side tests below read a real request shape rather than a Ze-shaped one.
func elicitRequest(mode, field string) map[string]any {
	params := map[string]any{
		"message": "Which ze command should be run?",
		keyRequestedSchema: map[string]any{
			"type":        "object",
			keyProperties: map[string]any{field: map[string]any{"type": "string"}},
		},
	}
	if mode != "" {
		params[keyMode] = mode
	}
	return map[string]any{"method": "elicitation/create", keyParams: params}
}

// VALIDATES: --elicit turns into the capability object the specification
// defines, including the empty-object form that means form mode only.
// PREVENTS: a test declaring "elicitation" and silently getting a shape the
// server reads as url-only, which would make a form-mode assertion vacuous.
func TestParseElicitCapability(t *testing.T) {
	tests := []struct {
		spec       string
		wantAbsent bool
		wantModes  []string
		wantErr    bool
	}{
		{spec: "", wantAbsent: true},
		{spec: "empty", wantModes: nil},
		{spec: "form", wantModes: []string{mcpModeForm}},
		{spec: "url", wantModes: []string{mcpModeURL}},
		{spec: "form,url", wantModes: []string{mcpModeForm, mcpModeURL}},
		{spec: "telepathy", wantErr: true},
		{spec: ",", wantErr: true},
	}
	for _, tt := range tests {
		t.Run(tt.spec, func(t *testing.T) {
			caps, declared, err := parseElicitCapability(tt.spec)
			if tt.wantErr {
				if err == nil {
					t.Fatalf("parseElicitCapability(%q) accepted; want an error", tt.spec)
				}
				return
			}
			if err != nil {
				t.Fatalf("parseElicitCapability(%q): %v", tt.spec, err)
			}
			if tt.wantAbsent {
				if declared {
					t.Fatalf("want the capability undeclared, got %#v", caps)
				}
				return
			}
			if !declared {
				t.Fatal("want a declared capability, got none")
			}
			if len(caps) != len(tt.wantModes) {
				t.Fatalf("declared %#v, want modes %v", caps, tt.wantModes)
			}
			for _, mode := range tt.wantModes {
				if _, ok := caps[mode]; !ok {
					t.Errorf("mode %q not declared: %#v", mode, caps)
				}
			}
		})
	}
}

// VALIDATES: an empty declared object means form mode only, and a url-only
// client does not support form mode.
// PREVENTS: the client answering a mode it never declared, which would hide a
// server violating "Servers MUST NOT send elicitation requests with modes that
// are not supported by the client".
func TestSupportsElicitMode(t *testing.T) {
	tests := []struct {
		name    string
		spec    string
		form    bool
		urlMode bool
	}{
		{name: "undeclared", spec: "", form: false, urlMode: false},
		{name: "empty means form", spec: "empty", form: true, urlMode: false},
		{name: "form", spec: "form", form: true, urlMode: false},
		{name: "url only", spec: "url", form: false, urlMode: true},
		{name: "both", spec: "form,url", form: true, urlMode: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			caps, declared, err := parseElicitCapability(tt.spec)
			if err != nil {
				t.Fatalf("parseElicitCapability(%q): %v", tt.spec, err)
			}
			if !declared {
				caps = nil
			}
			c := &mcpClient{elicitCaps: caps}
			if got := c.supportsElicitMode(mcpModeForm); got != tt.form {
				t.Errorf("supportsElicitMode(form) = %v, want %v", got, tt.form)
			}
			if got := c.supportsElicitMode(mcpModeURL); got != tt.urlMode {
				t.Errorf("supportsElicitMode(url) = %v, want %v", got, tt.urlMode)
			}
		})
	}
}

// VALIDATES: the client answers with the field name the SERVER's requestedSchema
// declares, and refuses a mode it did not declare.
// PREVENTS: the client hardcoding Ze's field name, which would make it a
// restatement of the implementation instead of an independent reading.
func TestAnswerInputRequests(t *testing.T) {
	formOnly, _, err := parseElicitCapability("form")
	if err != nil {
		t.Fatalf("parseElicitCapability: %v", err)
	}
	interim := func(mode, field string) map[string]any {
		return map[string]any{keyInputRequests: map[string]any{"srv_key": elicitRequest(mode, field)}}
	}

	t.Run("accept uses the schema's field name", func(t *testing.T) {
		c := &mcpClient{elicitCaps: formOnly, elicit: elicitPlan{queued: true, action: elicitAccept, value: "show version"}}
		responses, err := c.answerInputRequests(interim(mcpModeForm, "whatever_the_server_called_it"))
		if err != nil {
			t.Fatalf("answerInputRequests: %v", err)
		}
		entry, ok := responses["srv_key"].(map[string]any)
		if !ok {
			t.Fatalf("no answer under the server's key: %#v", responses)
		}
		if entry[keyAction] != elicitAccept {
			t.Errorf("action = %v, want %q", entry[keyAction], elicitAccept)
		}
		content, _ := entry[keyContent].(map[string]any)
		if content["whatever_the_server_called_it"] != "show version" {
			t.Errorf("content = %#v, want the value under the schema's property name", content)
		}
	})

	t.Run("an absent mode is form", func(t *testing.T) {
		c := &mcpClient{elicitCaps: formOnly, elicit: elicitPlan{queued: true, action: elicitAccept, value: "x"}}
		if _, err := c.answerInputRequests(interim("", "command")); err != nil {
			t.Fatalf("answerInputRequests: %v", err)
		}
	})

	t.Run("a mode the client did not declare is refused", func(t *testing.T) {
		c := &mcpClient{elicitCaps: formOnly, elicit: elicitPlan{queued: true, action: elicitAccept, value: "x"}}
		_, err := c.answerInputRequests(interim(mcpModeURL, "command"))
		if err == nil {
			t.Fatal("client answered a url-mode request it never declared support for")
		}
		if !strings.Contains(err.Error(), mcpModeURL) {
			t.Errorf("error does not name the offending mode: %v", err)
		}
	})

	t.Run("omit answers nothing", func(t *testing.T) {
		c := &mcpClient{elicitCaps: formOnly, elicit: elicitPlan{queued: true}}
		responses, err := c.answerInputRequests(interim(mcpModeForm, "command"))
		if err != nil {
			t.Fatalf("answerInputRequests: %v", err)
		}
		if len(responses) != 0 {
			t.Fatalf("omit produced %#v, want an empty object", responses)
		}
	})

	t.Run("an extra key rides alongside the answer", func(t *testing.T) {
		c := &mcpClient{elicitCaps: formOnly, elicit: elicitPlan{queued: true, action: elicitAccept, value: "x", extra: "never_asked_for"}}
		responses, err := c.answerInputRequests(interim(mcpModeForm, "command"))
		if err != nil {
			t.Fatalf("answerInputRequests: %v", err)
		}
		if _, ok := responses["never_asked_for"]; !ok {
			t.Errorf("extra key absent: %#v", responses)
		}
		if _, ok := responses["srv_key"]; !ok {
			t.Errorf("the real answer went missing: %#v", responses)
		}
	})

	t.Run("decline and cancel carry no content", func(t *testing.T) {
		for _, action := range []string{elicitDecline, elicitCancel} {
			c := &mcpClient{elicitCaps: formOnly, elicit: elicitPlan{queued: true, action: action}}
			responses, err := c.answerInputRequests(interim(mcpModeForm, "command"))
			if err != nil {
				t.Fatalf("%s: %v", action, err)
			}
			entry, _ := responses["srv_key"].(map[string]any)
			if entry[keyAction] != action {
				t.Errorf("action = %v, want %q", entry[keyAction], action)
			}
			if _, has := entry[keyContent]; has {
				t.Errorf("%s carries content: %#v", action, entry)
			}
		}
	})
}

// VALIDATES: classifyResult treats an absent resultType as complete, refuses a
// value it does not recognize, and accepts the extension-contributed "task"
// value only when the client declared the tasks extension.
// PREVENTS: a client that silently accepts a future resultType as a finished
// answer, which basic/index forbids. It also covers the extension-gated half.
// A server that pushed a task handle at a client that never declared the
// extension would otherwise be accepted here. And task-no-extension.ci would
// prove nothing.
func TestClassifyResult(t *testing.T) {
	tests := []struct {
		name     string
		body     string
		declared bool
		want     string
		wantErr  bool
	}{
		{name: "absent means complete", body: `{"content":[]}`, want: resultTypeComplete},
		{name: "complete", body: `{"resultType":"complete"}`, want: resultTypeComplete},
		{name: "input required", body: `{"resultType":"input_required"}`, want: resultTypeInputRequired},
		{name: "unknown", body: `{"resultType":"partial"}`, wantErr: true},
		{name: "not a string", body: `{"resultType":7}`, wantErr: true},
		{name: "not an object", body: `[]`, wantErr: true},
		// The extension-contributed value, both ways round.
		{name: "task with the extension declared", body: `{"resultType":"task"}`, declared: true, want: resultTypeTask},
		{name: "task without the extension declared", body: `{"resultType":"task"}`, wantErr: true},
		// A declared extension widens the set by exactly one value. It does not
		// make the client permissive anywhere else.
		{name: "unknown stays invalid with the extension declared", body: `{"resultType":"partial"}`, declared: true, wantErr: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, _, err := classifyResult([]byte(tt.body), tt.declared)
			if tt.wantErr {
				if err == nil {
					t.Fatalf("classifyResult(%s, declared=%v) accepted; want an error", tt.body, tt.declared)
				}
				return
			}
			if err != nil {
				t.Fatalf("classifyResult(%s, declared=%v): %v", tt.body, tt.declared, err)
			}
			if got != tt.want {
				t.Fatalf("resultType = %q, want %q", got, tt.want)
			}
		})
	}
}
