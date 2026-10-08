// Design: docs/architecture/wire/nlri-flowspec.md -- source emission through cached forwarding.
package reactor

import (
	"encoding/hex"
	"strings"
	"testing"

	"github.com/ze-software/ze/internal/component/bgp/message"
	"github.com/ze-software/ze/internal/component/bgp/plugins/nlri/flowspec"
	"github.com/ze-software/ze/internal/component/plugin/registry"
	"github.com/ze-software/ze/internal/core/bgp/msgtype"
	"github.com/ze-software/ze/internal/core/bgp/ribevents"
	"github.com/ze-software/ze/internal/core/family"
)

// TestFlowSpecPreferredNumericForwarding reads the destination TCP socket after
// registered text/config producers, receive cache and forward pipeline. Literal
// NLRIs distinguish incorrect source widths from forwarding a supplied fixture.
// A separately supplied wide operand remains valid and is preserved, because
// these emission SHOULDs do not impose a receive-side width MUST.
// RFC 8955 Sections 4.2.2.5, 4.2.2.6 and 4.2.2.10 specify 1- or 2-octet values.
// RFC 8955 Section 4.2.2.8: "Type 8 component values SHOULD be encoded as single
// octet (numeric_op len=00)."
// RFC requirement: RFC8955-4.2.2.5-1 positive -- registered text/config emission and supported route-command emission reach the recipient TCP socket with exact preferred destination-port widths at 0, 255, 256 and 65535.
// RFC requirement: RFC8955-4.2.2.6-1 positive -- registered text/config emission and supported route-command emission reach the recipient TCP socket with exact preferred source-port widths at 0, 255, 256 and 65535.
// RFC requirement: RFC8955-4.2.2.8-1 positive -- registered text/config emission reaches the recipient TCP socket with exact one-octet ICMP-code operands at 0 and 255.
// RFC requirement: RFC8955-4.2.2.10-1 positive -- registered text/config emission reaches the recipient TCP socket with exact preferred packet-length widths at 0, 255, 256 and 65535.
// The wide value-255 cases are compatibility controls, not negative evidence
// for the emission SHOULDs or coverage of arbitrary high-bit operand values.
func TestFlowSpecPreferredNumericForwarding(t *testing.T) {
	for _, tc := range []struct {
		criterion string
		nlri      string
	}{
		{"destination-port =0", "080118c00002058100"},
		{"destination-port =255", "080118c000020581ff"},
		{"destination-port =256", "090118c0000205910100"},
		{"destination-port =65535", "090118c000020591ffff"},
		{"source-port =0", "080118c00002068100"},
		{"source-port =255", "080118c000020681ff"},
		{"source-port =256", "090118c0000206910100"},
		{"source-port =65535", "090118c000020691ffff"},
		{"icmp-code =0", "080118c00002088100"},
		{"icmp-code =255", "080118c000020881ff"},
		{"packet-length =0", "080118c000020a8100"},
		{"packet-length =255", "080118c000020a81ff"},
		{"packet-length =256", "090118c000020a910100"},
		{"packet-length =65535", "090118c000020a91ffff"},
	} {
		for _, producer := range []string{"registered-text", "config", "route-command"} {
			if producer == "route-command" {
				if strings.HasPrefix(tc.criterion, "icmp-code") {
					continue // The separate match/then route grammar lacks ICMP code.
				}
				if strings.HasPrefix(tc.criterion, "packet-length") {
					continue // The separate match/then route grammar lacks packet length.
				}
			}
			t.Run(producer+"/"+tc.criterion, func(t *testing.T) {
				// RFC 8955 Sections 4.2.2.5/6/8/10: originate, rather than
				// pre-fill, the operand whose bytes the recipient must observe.
				frame := preferredWidthSource(t, producer, tc.criterion)
				// Both source producers return complete BGP frames. The receive
				// fixture accepts only an UPDATE payload; raw controls below
				// already supply that payload and must not be stripped.
				// RFC 4271 Section 4.1: validate the declared framing explicitly.
				header, err := message.ParseHeader(frame)
				if err != nil {
					t.Fatal(err)
				}
				if int(header.Length) != len(frame) {
					t.Fatalf("frame length=%d, header declares %d", len(frame), header.Length)
				}
				if header.Type != msgtype.TypeUPDATE {
					t.Fatalf("frame type=%v, want UPDATE", header.Type)
				}
				forwardPreferredWidth(t, frame[message.HeaderLen:], mustHex(t, tc.nlri))
			})
		}
	}
	for _, kind := range []byte{5, 6, 8, 10} {
		for _, operand := range [][]byte{
			{0x91, 0, 0xff},
			{0xa1, 0, 0, 0, 0xff},
			{0xb1, 0, 0, 0, 0, 0, 0, 0, 0xff},
		} {
			raw := append([]byte{byte(6 + len(operand)), 1, 24, 192, 0, 2, kind}, operand...)
			t.Run("received-wide-"+hex.EncodeToString(raw), func(t *testing.T) {
				fam := family.Family{AFI: family.AFIIPv4, SAFI: family.SAFIFlowSpec}
				forwardPreferredWidth(t, flowForwardPayload(t, fam, raw, nil), raw)
			})
		}
	}
}

// preferredWidthSource uses existing registered encoders and the production
// UPDATE builder, returning a complete BGP frame. Only the separate route-command
// grammar has a smaller domain.
func preferredWidthSource(t *testing.T, producer, criterion string) []byte {
	t.Helper()
	content := "destination-ipv4 192.0.2.0/24 " + criterion
	// RFC 8955 Sections 4.2.2.5/6: the route-command API supports bare ports.
	if producer == "route-command" {
		frame, _, err := flowspec.EncodeRoute("match "+strings.ReplaceAll(content, " =", " ")+" then rate-limit 9600", "ipv4/flow", 65002, false, true, false)
		if err != nil {
			t.Fatal(err)
		}
		return frame
	}
	var raw []byte
	// RFC 8955 Sections 4.2.2.5/6/8/10: both registered producers encode values.
	if producer == "config" {
		parser := registry.ConfigRouteParserByFamily("ipv4/flow")
		if parser == nil {
			t.Fatal("FlowSpec config producer is not registered")
		}
		configured, err := parser(registry.ConfigRouteRequest{Content: strings.Fields("add " + content)})
		if err != nil {
			t.Fatal(err)
		}
		raw = configured.NLRI
	} else {
		encoded, err := registry.EncodeNLRIByFamily("ipv4/flow", strings.Fields(content))
		if err != nil {
			t.Fatal(err)
		}
		raw = mustHex(t, encoded)
	}
	builder := message.GetUpdateBuilder(65002, false, true, false)
	defer message.PutUpdateBuilder(builder)
	update := builder.BuildPlugin(message.PluginParams{
		AFI: 1, SAFI: 133, NLRI: raw,
		RawAttrs: [][]byte{mustHex(t, "c010088006000046160000")},
	})
	return message.PackTo(update, nil)
}

func forwardPreferredWidth(t *testing.T, body, want []byte) {
	t.Helper()
	fam := family.Family{AFI: family.AFIIPv4, SAFI: family.SAFIFlowSpec}
	f := newAIGPReplayFixture(t, nil)
	for _, peer := range []*Peer{f.source, f.destination} {
		peer.negotiated.Store(&NegotiatedCapabilities{ASN4: true, families: map[family.Family]bool{fam: true}})
	}
	f.destination.settings.PeerAS = 65000
	f.destination.refreshForwardFacts()
	// Authorization is not this root's claim. Admit only the independent
	// expected route, so incorrect emission cannot authorize itself.
	key := ribevents.ValidationRoute{Peer: f.source.Settings().Address, Family: fam, NLRI: string(want)}
	ribevents.RegisterFlowSpecLookup(func(got ribevents.ValidationRoute, _ uint64) bool { return got == key }, nil, nil, nil)
	t.Cleanup(func() { ribevents.RegisterFlowSpecLookup(nil, nil, nil, nil) })
	// RFC 8955 Sections 4.2.2.5/6/8/10: observe the actual forwarding sink.
	id := f.receive(t, body)
	f.forward(t, id)
	forwardSocketBarrier(t, f.r)
	bodies := aigpSocketBodies(t, f.conn)
	if len(bodies) != 1 {
		t.Fatalf("recipient UPDATE count=%d, want one FlowSpec", len(bodies))
	}
	assertFlowForwardWire(t, bodies[0], fam, want)
}
