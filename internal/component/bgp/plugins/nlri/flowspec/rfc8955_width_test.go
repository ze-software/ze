// Design: docs/architecture/wire/nlri-flowspec.md -- preferred numeric emission widths.
package flowspec

import (
	"bytes"
	"encoding/binary"
	"encoding/hex"
	"strconv"
	"strings"
	"testing"

	"github.com/ze-software/ze/internal/component/bgp/message"
	"github.com/ze-software/ze/internal/component/plugin/registry"
	"github.com/ze-software/ze/internal/core/bgp/attribute"
	"github.com/ze-software/ze/internal/core/bgp/msgtype"
	"github.com/ze-software/ze/internal/core/bgp/nlri/flowspec"
)

// TestFlowSpecPreferredNumericEmission compares independent literal operator/value
// bytes across typed, generic, registered config and registered text NLRI producers.
// Exact shortest-width selection is Ze policy: the RFC permits either one or
// two octets for small type 5/6/10 values, not only the one octet chosen here.
// RFC 8955 Section 4.2.2.5: "Type 5 component values SHOULD be encoded as 1- or
// 2-octet quantities (numeric_op len=00 or len=01)."
// RFC 8955 Section 4.2.2.6: "Type 6 component values SHOULD be encoded as 1- or
// 2-octet quantities (numeric_op len=00 or len=01)."
// RFC 8955 Section 4.2.2.8: "Type 8 component values SHOULD be encoded as single
// octet (numeric_op len=00)."
// RFC 8955 Section 4.2.2.10: "Type 10 component values SHOULD be encoded as 1- or
// 2-octet quantities (numeric_op len=00 or len=01)."
// RFC requirement: RFC8955-4.2.2.5-1 positive -- typed, generic, config and text producers emit exact one/two-octet destination-port operands at 0, 255, 256 and 65535.
// RFC requirement: RFC8955-4.2.2.6-1 positive -- typed, generic, config and text producers emit exact one/two-octet source-port operands at 0, 255, 256 and 65535.
// RFC requirement: RFC8955-4.2.2.8-1 positive -- typed, generic, config and text producers emit exact one-octet ICMP-code operands at 0 and 255.
// RFC requirement: RFC8955-4.2.2.10-1 positive -- typed, generic, config and text producers emit exact one/two-octet packet-length operands at 0, 255, 256 and 65535.
func TestFlowSpecPreferredNumericEmission(t *testing.T) {
	for _, component := range []struct {
		kind  flowspec.FlowComponentType
		name  string
		typed func(uint16) flowspec.FlowComponent
	}{
		{flowspec.FlowDestPort, "destination-port", func(v uint16) flowspec.FlowComponent { return flowspec.NewFlowDestPortComponent(v) }},
		{flowspec.FlowSourcePort, "source-port", func(v uint16) flowspec.FlowComponent { return flowspec.NewFlowSourcePortComponent(v) }},
		{flowspec.FlowICMPCode, "icmp-code", func(v uint16) flowspec.FlowComponent { return flowspec.NewFlowICMPCodeComponent(uint8(v)) }},
		{flowspec.FlowPacketLength, "packet-length", func(v uint16) flowspec.FlowComponent { return packetLengthComponent(v) }},
	} {
		for _, boundary := range []struct {
			value uint16
			wire  []byte
		}{
			{0, []byte{0x81, 0}},
			{255, []byte{0x81, 0xff}},
			{256, []byte{0x91, 1, 0}},
			{65535, []byte{0x91, 0xff, 0xff}},
		} {
			if component.kind == flowspec.FlowICMPCode && boundary.value > 255 {
				continue // The typed ICMP code domain ends at 255, not 65535.
			}
			t.Run(component.name+"/"+strconv.Itoa(int(boundary.value)), func(t *testing.T) {
				want := append([]byte{byte(1 + len(boundary.wire)), byte(component.kind)}, boundary.wire...)
				criterion := component.name + " =" + strconv.Itoa(int(boundary.value))
				// RFC 8955 Sections 4.2.2.5, 4.2.2.6, 4.2.2.8 and 4.2.2.10.
				for name, comp := range map[string]flowspec.FlowComponent{
					"typed":   component.typed(boundary.value),
					"generic": flowspec.NewFlowNumericComponent(component.kind, []flowspec.FlowMatch{{Op: flowspec.FlowOpEqual, Value: uint64(boundary.value)}}),
				} {
					fs := flowspec.NewFlowSpec(IPv4FlowSpec)
					if err := fs.AddComponent(comp); err != nil {
						t.Fatal(err)
					}
					if got := fs.Bytes(); !bytes.Equal(got, want) {
						t.Errorf("%s NLRI=%x, want %x", name, got, want)
					}
				}
				parser := registry.ConfigRouteParserByFamily("ipv4/flow")
				if parser == nil {
					t.Fatal("FlowSpec config producer is not registered")
				}
				configured, err := parser(registry.ConfigRouteRequest{Content: strings.Fields("add " + criterion)})
				if err != nil {
					t.Fatal(err)
				}
				if !bytes.Equal(configured.NLRI, want) {
					t.Errorf("config NLRI=%x, want %x", configured.NLRI, want)
				}
				encoded, err := registry.EncodeNLRIByFamily("ipv4/flow", strings.Fields(criterion))
				if err != nil {
					t.Fatal(err)
				}
				textNLRI, err := hex.DecodeString(encoded)
				if err != nil {
					t.Fatal(err)
				}
				if !bytes.Equal(textNLRI, want) {
					t.Errorf("registered text NLRI=%s, want %x", encoded, want)
				}
				// The separate match/then route-command grammar only supports
				// ports among these four components and takes bare port numbers.
				if component.kind != flowspec.FlowDestPort && component.kind != flowspec.FlowSourcePort {
					return
				}
				routeCriterion := component.name + " " + strconv.Itoa(int(boundary.value))
				frame, _, err := EncodeRoute("match "+routeCriterion+" then accept", "ipv4/flow", 65000, false, true, false)
				if err != nil {
					t.Fatal(err)
				}
				// EncodeRoute calls PackTo and returns a complete BGP frame.
				// RFC 4271 Section 4.1: validate its fixed header before
				// examining the UPDATE payload rather than guessing framing.
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
				body := frame[message.HeaderLen:]
				if len(body) < 4 {
					t.Fatalf("short UPDATE body: %x", body)
				}
				attrLen := int(binary.BigEndian.Uint16(body[2:4]))
				if 4+attrLen > len(body) {
					t.Fatalf("attributes exceed UPDATE: %x", body)
				}
				_, _, mp, found := attribute.AttrFind(body[4:4+attrLen], attribute.AttrMPReachNLRI)
				wantMP := append([]byte{0, 1, 133, 0, 0}, want...)
				if !found || !bytes.Equal(mp, wantMP) {
					t.Errorf("text MP_REACH=%x present=%v, want %x", mp, found, wantMP)
				}
			})
		}
	}
}
