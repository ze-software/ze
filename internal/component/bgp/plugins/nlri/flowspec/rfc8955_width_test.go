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
		kind  FlowComponentType
		name  string
		typed func(uint16) FlowComponent
	}{
		{FlowDestPort, "destination-port", func(v uint16) FlowComponent { return NewFlowDestPortComponent(v) }},
		{FlowSourcePort, "source-port", func(v uint16) FlowComponent { return NewFlowSourcePortComponent(v) }},
		{FlowICMPCode, "icmp-code", func(v uint16) FlowComponent { return newFlowICMPCodeComponent(uint8(v)) }},
		{FlowPacketLength, "packet-length", func(v uint16) FlowComponent { return NewFlowPacketLengthComponent(v) }},
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
			if component.kind == FlowICMPCode && boundary.value > 255 {
				continue // The typed ICMP code domain ends at 255, not 65535.
			}
			t.Run(component.name+"/"+strconv.Itoa(int(boundary.value)), func(t *testing.T) {
				want := append([]byte{byte(1 + len(boundary.wire)), byte(component.kind)}, boundary.wire...)
				criterion := component.name + " =" + strconv.Itoa(int(boundary.value))
				// RFC 8955 Sections 4.2.2.5, 4.2.2.6, 4.2.2.8 and 4.2.2.10.
				for name, comp := range map[string]FlowComponent{
					"typed":   component.typed(boundary.value),
					"generic": newFlowNumericComponent(component.kind, []FlowMatch{{Op: FlowOpEqual, Value: uint64(boundary.value)}}),
				} {
					fs := NewFlowSpec(IPv4FlowSpec)
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
				if component.kind != FlowDestPort && component.kind != FlowSourcePort {
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

// TestFlowSpecPreferredNumericWideReception prevents an emission SHOULD from
// becoming a receive-side width MUST. Legal wide operands retain their value;
// reconstruction emits the preferred width when that value fits the component.
// RFC 8955 Section 4.2.1.1: "This encodes 1 (len=00), 2 (len=01), 4
// (len=10), and 8 (len=11) octets."
// These are compatibility controls for value 255, not negative inputs to the
// emission SHOULDs and not proof of arbitrary high-bit operand preservation.
func TestFlowSpecPreferredNumericWideReception(t *testing.T) {
	for _, kind := range []FlowComponentType{FlowDestPort, FlowSourcePort, FlowICMPCode, FlowPacketLength} {
		for _, operand := range [][]byte{
			{0x91, 0, 0xff},
			{0xa1, 0, 0, 0, 0xff},
			{0xb1, 0, 0, 0, 0, 0, 0, 0, 0xff},
		} {
			wire := append([]byte{byte(1 + len(operand)), byte(kind)}, operand...)
			// RFC 8955 Sections 4.2.1.1 and 4.2.2.5/6/8/10.
			fs, err := ParseFlowSpec(IPv4FlowSpec, wire)
			if err != nil {
				t.Fatalf("valid wider NLRI %x rejected: %v", wire, err)
			}
			if len(fs.Components()) != 1 {
				t.Fatalf("%x decoded %d components", wire, len(fs.Components()))
			}
			numeric, ok := fs.Components()[0].(*numericComponent)
			if !ok {
				t.Fatalf("%x decoded as %T", wire, fs.Components()[0])
			}
			matches := numeric.Matches()
			if len(matches) != 1 {
				t.Fatalf("%x decoded %d matches", wire, len(matches))
			}
			if matches[0].Value != 255 || matches[0].Op != FlowOpEqual || matches[0].And {
				t.Fatalf("%x decoded as %+v", wire, matches[0])
			}
			if got, want := fs.Bytes(), []byte{3, byte(kind), 0x81, 0xff}; !bytes.Equal(got, want) {
				t.Errorf("reconstructed %x as %x, want %x", wire, got, want)
			}
		}
	}
}
