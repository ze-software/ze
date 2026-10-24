package bridge

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"
)

// TestMarshalExabgpJSONUsesExaBGPSpacing checks the bytes a script reads carry
// ExaBGP's spacing, while string contents and empty containers are untouched.
//
// VALIDATES: `{ "k": v, "k": v }` and `[ a, b ]`, as ExaBGP's hand-built JSON
// writes them (src/exabgp/reactor/api/response/json.py at f9bfb853b), and the
// output still parses to the same document.
// PREVENTS: a ported script's substring test, such as api-rr's
// `'"nlri": "192.168.0.0/32"' in line`, failing on a line carrying the route.
func TestMarshalExabgpJSONUsesExaBGPSpacing(t *testing.T) {
	document := map[string]any{
		"announce": map[string]any{"ipv4 unicast": map[string]any{
			"10.0.0.0": []any{map[string]any{"nlri": "192.168.0.0/32"}},
		}},
		"empty":  map[string]any{},
		"none":   []any{},
		"quoted": `a:b, {c} [d] "e"`,
	}
	got, err := marshalExabgpJSON(document)
	if err != nil {
		t.Fatalf("marshalExabgpJSON: %v", err)
	}
	want := `{ "announce": { "ipv4 unicast": { "10.0.0.0": [ { "nlri": "192.168.0.0/32" } ] } }, ` +
		`"empty": {}, "none": [], "quoted": "a:b, {c} [d] \"e\"" }`
	if string(got) != want {
		t.Errorf("marshalExabgpJSON =\n%s\nwant\n%s", got, want)
	}
	if !json.Valid(got) {
		t.Errorf("the spaced form is not valid JSON: %s", got)
	}
}

// TestTranslateAndForwardWritesExaBGPSpacing checks the bridge's own event path
// writes the spaced form a script reads.
//
// VALIDATES: the line written to the script carries `"exabgp": "6.0.0"`.
// PREVENTS: the forwarding path keeping Go's compact form beside the fleet's.
func TestTranslateAndForwardWritesExaBGPSpacing(t *testing.T) {
	var out bytes.Buffer
	b := NewBridge([]string{"echo"})
	b.translateAndForward(`{"type":"state","peer":{"address":"127.0.0.1"},"state":"up"}`, &out)
	if !strings.Contains(out.String(), `"exabgp": "6.0.0"`) {
		t.Errorf("forwarded line = %q, want ExaBGP's spacing", out.String())
	}
}
