// Tests for the route fields of the filter-update message (spec-filter-wire-0-umbrella D-1).

package reactor

import (
	"bytes"
	"context"
	"encoding/json"
	"testing"

	"github.com/ze-software/ze/internal/component/bgp/wireu"
	routepolicy "github.com/ze-software/ze/internal/component/policy"
	bgpctx "github.com/ze-software/ze/internal/core/bgp/context"
	"github.com/ze-software/ze/pkg/plugin/rpc"
)

// fieldsTransport answers each filter's declared reads by filter name and
// keeps every filter-update request it was sent, in order.
type fieldsTransport struct {
	reads map[string][]string
	sent  []rpc.FilterUpdateInput
}

func (f *fieldsTransport) FilterInfo(_, filterName string) ([]string, bool) {
	for _, read := range f.reads[filterName] {
		if read == rpc.FilterFieldRaw {
			return nil, true
		}
	}
	return nil, false
}

func (f *fieldsTransport) FilterDeclaration(_, _ string) ([]string, []string, rpc.OnErrorPolicy, bool) {
	return nil, nil, rpc.OnErrorReject, false
}

func (f *fieldsTransport) FilterReads(_, filterName string) []string { return f.reads[filterName] }

func (f *fieldsTransport) FilterOnError(_, _ string) rpc.OnErrorPolicy { return rpc.OnErrorReject }

func (f *fieldsTransport) CallFilterUpdate(_ context.Context, _ string, route *rpc.FilterRoute) (*rpc.FilterUpdateOutput, error) {
	inputValue, err := route.Input()
	if err != nil {
		return nil, err
	}
	input := &inputValue
	sent := *input
	// The fields buffer returns to its pool once the call answers, as it does
	// after the production transport marshalled the request, so the test
	// keeps a copy of what the plugin would have read.
	sent.Fields = append(json.RawMessage(nil), input.Fields...)
	f.sent = append(f.sent, sent)
	return &rpc.FilterUpdateOutput{Action: rpc.FilterAccept}, nil
}

// fieldsTestBody is an IPv4 unicast UPDATE for 10.0.0.0/24: ORIGIN IGP,
// AS_PATH [65001 65002] in the four-octet form, NEXT_HOP 192.0.2.1,
// LOCAL_PREF 100 and COMMUNITY 65001:100 65001:200.
func fieldsTestBody() []byte {
	attrs := []byte{
		0x40, 0x01, 0x01, 0x00,
		0x40, 0x02, 0x0a, 0x02, 0x02, 0x00, 0x00, 0xfd, 0xe9, 0x00, 0x00, 0xfd, 0xea,
		0x40, 0x03, 0x04, 192, 0, 2, 1,
		0x40, 0x05, 0x04, 0x00, 0x00, 0x00, 0x64,
		0xc0, 0x08, 0x08, 0xfd, 0xe9, 0x00, 0x64, 0xfd, 0xe9, 0x00, 0xc8,
	}
	return buildUpdateBody(attrs, []byte{24, 10, 0, 0})
}

// fieldsTestWire is fieldsTestBody under a registered four-octet AS context
// with no ADD-PATH.
func fieldsTestWire(t *testing.T) *wireu.WireUpdate {
	t.Helper()
	id, err := bgpctx.Registry.Register(bgpctx.EncodingContextForASN4(true))
	if err != nil {
		t.Fatalf("register context: %v", err)
	}
	return wireu.NewWireUpdate(fieldsTestBody(), id)
}

// runFieldsExport sends the test UPDATE through the export chain of refs and
// answers what each filter was sent.
func runFieldsExport(t *testing.T, reads map[string][]string, refs ...string) []rpc.FilterUpdateInput {
	t.Helper()
	transport := &fieldsTransport{reads: reads}
	r := &Reactor{filterTransportSeam: transport}
	chain := make([]routepolicy.Ref, 0, len(refs))
	for _, ref := range refs {
		chain = append(chain, routepolicy.Ref{Name: "fields-plugin:" + ref})
	}
	got := r.runEgressPolicyChainASN4(mustPolicy(t, chain, routepolicy.FinalAccept), "10.0.0.2", 65002, 65000, false, fieldsTestWire(t), true)
	if !got.accept {
		t.Fatalf("export = %+v, want accept", got)
	}
	if len(transport.sent) != len(refs) {
		t.Fatalf("sent %d requests, want %d", len(transport.sent), len(refs))
	}
	return transport.sent
}

// TestFilterMessageFieldShapes proves the fields object keys each declared
// field by its vocabulary name, never by the event's attr key, and writes each
// value in the attr shape, or in the shape the protocol page gives a field the
// attr table lacks.
//
// VALIDATES: AC-29 (keys and shapes; the refusal of a field no writer is
// linked for is TestFieldWriterNotLinked in policyview, where no plugin links).
// PREVENTS: the event key `communities` reaching a filter, or a value shape
// that differs from the one a plugin reads in events.
func TestFilterMessageFieldShapes(t *testing.T) {
	cases := []struct {
		name  string
		reads []string
		want  string
	}{
		{
			name:  "AC-29 attribute fields",
			reads: []string{"community", "as-path", "local-preference"},
			want:  `{"community":["65001:100","65001:200"],"as-path":[65001,65002],"local-preference":100}`,
		},
		{
			name:  "route fields the attr table lacks",
			reads: []string{"prefix", "family", "next-hop", "protocol", "origin-as", "origin"},
			want:  `{"prefix":{"ipv4/unicast":["10.0.0.0/24"]},"family":["ipv4/unicast"],"next-hop":{"ipv4/unicast":"192.0.2.1"},"protocol":"ebgp","origin-as":65002,"origin":"igp"}`,
		},
		{
			name:  "a declared field the route does not carry is left out",
			reads: []string{"med", "prefix"},
			want:  `{"prefix":{"ipv4/unicast":["10.0.0.0/24"]}}`,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			sent := runFieldsExport(t, map[string][]string{"f": tc.reads}, "f")
			if got := string(sent[0].Fields); got != tc.want {
				t.Fatalf("fields = %s\nwant     %s", got, tc.want)
			}
		})
	}
}

// TestFilterMessageDeclaredFieldsOnly proves Ze sends a filter the fields it
// declared and nothing else, and the raw UPDATE only to a filter that
// declared the raw read.
//
// VALIDATES: AC-2 (a prefix-only filter gets no as-path, no community and no
// raw key) and AC-3 (two filters in one chain, only the raw reader gets raw).
// PREVENTS: a field leaking to a filter that never asked for it.
func TestFilterMessageDeclaredFieldsOnly(t *testing.T) {
	sent := runFieldsExport(t, map[string][]string{
		"raw-reader":    {rpc.FilterFieldRaw},
		"prefix-reader": {"prefix"},
	}, "raw-reader", "prefix-reader")

	if !bytes.Equal(sent[0].Raw, fieldsTestBody()) {
		t.Fatalf("raw reader got raw %x, want the UPDATE body", sent[0].Raw)
	}
	if len(sent[0].Fields) != 0 {
		t.Fatalf("raw reader got fields %s, want none: it declared no route field", sent[0].Fields)
	}

	msg, err := json.Marshal(&sent[1])
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	var top map[string]json.RawMessage
	if err := json.Unmarshal(msg, &top); err != nil {
		t.Fatalf("unmarshal %s: %v", msg, err)
	}
	if _, ok := top["raw"]; ok {
		t.Fatalf("prefix reader was sent raw: %s", msg)
	}
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(top["fields"], &fields); err != nil {
		t.Fatalf("fields of %s: %v", msg, err)
	}
	if len(fields) != 1 || fields["prefix"] == nil {
		t.Fatalf("prefix reader got fields %s, want the prefix key alone", top["fields"])
	}
}
