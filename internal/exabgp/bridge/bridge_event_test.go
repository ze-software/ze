package bridge

import (
	"encoding/json"
	"reflect"
	"strings"
	"testing"
)

// openEventZe is the OPEN ze's JSON encoder writes for the message whose hex is
// openWireHex (internal/component/bgp/format/json.go, Open, with the capability
// values formatCapability gives: a structured value for the six capabilities it
// knows, the payload in hex for every other one).
const openEventZe = `{"type":"bgp","bgp":{
 "message":{"type":"open","id":1,"direction":"received"},
 "peer":{"local":{"address":"127.0.0.1","as":65533},"remote":{"address":"127.0.0.1","as":65533}},
 "open":{"asn":65001,"router-id":"10.0.0.2","timer":{"hold-time":180},"capabilities":[
  {"code":1,"name":"multiprotocol","value":"ipv4/unicast"},
  {"code":1,"name":"multiprotocol","value":"ipv6/unicast"},
  {"code":2,"name":"unknown-2"},
  {"code":64,"name":"unknown-64","value":"807800010180"},
  {"code":65,"name":"asn4","value":"65001"},
  {"code":73,"name":"unknown-73","value":"01680164"},
  {"code":75,"name":"unknown-75","value":"7a65"},
  {"code":69,"name":"addpath","value":"ipv4/unicast send-receive"},
  {"code":200,"name":"unknown-200","value":"ab"},
  {"code":9,"name":"unknown-9","value":"03"},
  {"code":6,"name":"extended-message"},
  {"code":76,"name":"paths-limit","value":"ipv4/unicast 10"},
  {"code":5,"name":"extended-nexthop","value":"ipv4/unicast ipv6"},
  {"code":70,"name":"unknown-70"}
 ]}}}`

// openCapabilitiesV6 is what ExaBGP main (685f772e4) writes for that OPEN,
// copied from `exabgp decode -o` of
// FFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFF00640104FDE900B40A0000024702450104000100010104000200010200400680780001018041040000FDE94904016801644B027A65450400010103C801AB09010306004C05000101000A05060001000100024600
// with nothing changed but the whitespace.
const openCapabilitiesV6 = `{
 "multiprotocol": { "code": 1, "name": "multiprotocol", "families": [ "ipv4/unicast", "ipv6/unicast" ] },
 "route-refresh": { "code": 2, "name": "route-refresh", "variant": "RFC" },
 "graceful-restart": { "code": 64, "name": "graceful restart", "time": 120, "address-family-flags": { "ipv4/unicast": [ "forwarding" ] }, "restart-flags": [ "restart" ] },
 "asn4": { "code": 65, "name": "asn4", "asn4": 65001 },
 "hostname": { "code": 73, "host-name": "h", "domain-name": "d" },
 "software-version": { "code": 75, "software": "ze" },
 "add-path": { "code": 69, "name": "addpath", "ipv4/unicast": "send/receive" },
 "unknown capability 0xc8": { "code": 200, "name": "unknown", "iana": "reserved", "value": 200, "raw": "AB" },
 "role": { "code": 9, "name": "role", "role": "customer" },
 "extended-message": { "code": 6, "name": "extended-message", "size": 65535 },
 "paths-limit": { "code": 76, "name": "paths-limit", "ipv4/unicast": 10 },
 "nexthop": { "code": 5, "name": "nexthop", "conversion": [ "ipv4/unicast/ipv6" ] },
 "enhanced-route-refresh": { "code": 70, "name": "enhanced-route-refresh" } }`

// openCapabilitiesV4 is the same OPEN for an API 4 helper. ExaBGP's
// JSON._json_kv (src/exabgp/reactor/api/response/json.py) files each
// capability under int(code) and writes v.json() unchanged, so the bodies are
// those above without their "code".
const openCapabilitiesV4 = `{
 "1": { "name": "multiprotocol", "families": [ "ipv4/unicast", "ipv6/unicast" ] },
 "2": { "name": "route-refresh", "variant": "RFC" },
 "64": { "name": "graceful restart", "time": 120, "address-family-flags": { "ipv4/unicast": [ "forwarding" ] }, "restart-flags": [ "restart" ] },
 "65": { "name": "asn4", "asn4": 65001 },
 "73": { "host-name": "h", "domain-name": "d" },
 "75": { "software": "ze" },
 "69": { "name": "addpath", "ipv4/unicast": "send/receive" },
 "200": { "name": "unknown", "iana": "reserved", "value": 200, "raw": "AB" },
 "9": { "name": "role", "role": "customer" },
 "6": { "name": "extended-message", "size": 65535 },
 "76": { "name": "paths-limit", "ipv4/unicast": 10 },
 "5": { "name": "nexthop", "conversion": [ "ipv4/unicast/ipv6" ] },
 "70": { "name": "enhanced-route-refresh" } }`

// readZeEvent parses a ze JSON event the way the fan-out does.
func readZeEvent(t *testing.T, raw string) Event {
	t.Helper()
	var zebgp map[string]any
	if err := json.Unmarshal([]byte(raw), &zebgp); err != nil {
		t.Fatalf("ze event: %v", err)
	}
	return ReadEvent(zebgp)
}

// sameJSON compares two values by their JSON meaning: ExaBGP's lines are
// compared parsed by the compatibility tests too, so key order is not asserted.
func sameJSON(t *testing.T, what string, got any, want string) {
	t.Helper()
	encoded, err := json.Marshal(got)
	if err != nil {
		t.Fatalf("%s: marshal: %v", what, err)
	}
	var gotValue, wantValue any
	if err := json.Unmarshal(encoded, &gotValue); err != nil {
		t.Fatalf("%s: unmarshal got: %v", what, err)
	}
	if err := json.Unmarshal([]byte(want), &wantValue); err != nil {
		t.Fatalf("%s: unmarshal want: %v", what, err)
	}
	if !reflect.DeepEqual(gotValue, wantValue) {
		t.Errorf("%s:\n got  %s\n want %s", what, encoded, want)
	}
}

// openMember answers the neighbor.open object of a rendered OPEN.
func openMember(t *testing.T, rendered map[string]any) map[string]any {
	t.Helper()
	neighbor, ok := rendered["neighbor"].(map[string]any)
	if !ok {
		t.Fatalf("no neighbor in %v", rendered)
	}
	open, ok := neighbor["open"].(map[string]any)
	if !ok {
		t.Fatalf("no neighbor.open in %v", neighbor)
	}
	return open
}

// TestV4JSONEnvelopeAndCapabilitiesByCode proves X-6 and X-9 (AC-11): an API 4
// JSON helper reads envelope 4.0.1 and the OPEN's capabilities keyed by code,
// with no "code" inside. Method: render ze's OPEN event in FormatV4JSON and
// compare with ExaBGP's own output, whose v6 form was produced by the pinned
// ExaBGP from the same wire bytes.
func TestV4JSONEnvelopeAndCapabilitiesByCode(t *testing.T) {
	rendered := readZeEvent(t, openEventZe).ExabgpJSON(FormatV4JSON)
	if rendered["exabgp"] != JSONVersionV4 {
		t.Errorf("envelope %v, want %s", rendered["exabgp"], JSONVersionV4)
	}
	open := openMember(t, rendered)
	sameJSON(t, "capabilities", open["capabilities"], openCapabilitiesV4)
	open["capabilities"] = nil
	sameJSON(t, "open", open,
		`{"version": 4, "asn": 65001, "hold_time": 180, "router_id": "10.0.0.2", "capabilities": null}`)
}

// TestV6JSONCapabilitiesByName proves X-10 (AC-12): an API 6 helper reads the
// capabilities keyed by name, each with its "code" inside, as ExaBGP main
// wrote them for the same OPEN.
func TestV6JSONCapabilitiesByName(t *testing.T) {
	rendered := readZeEvent(t, openEventZe).ExabgpJSON(FormatV6JSON)
	open := openMember(t, rendered)
	sameJSON(t, "capabilities", open["capabilities"], openCapabilitiesV6)
}

// TestOpenMyASIsTheTwoOctetField proves that `asn` is the OPEN's My
// Autonomous System field, as ExaBGP's Open.asn is: a four-octet AS reaches it
// as AS_TRANS, the value RFC 6793 makes a NEW speaker put there.
func TestOpenMyASIsTheTwoOctetField(t *testing.T) {
	event := readZeEvent(t, `{"type":"bgp","bgp":{"message":{"type":"open"},"peer":{"remote":{"address":"10.0.0.1"}},
	 "open":{"asn":4200000000,"router-id":"1.1.1.1","timer":{"hold-time":90},"capabilities":[{"code":65,"name":"asn4","value":"4200000000"}]}}}`)
	open := openMember(t, event.ExabgpJSON(FormatV6JSON))
	sameJSON(t, "asn", open["asn"], `23456`)
	sameJSON(t, "capabilities", open["capabilities"], `{"asn4": {"code": 65, "name": "asn4", "asn4": 4200000000}}`)
}

// notificationEventZe is a NOTIFICATION as ze's JSON encoder writes it: the
// Data field in lowercase hex, nothing else made of it.
func notificationEventZe(code, subcode int, dataHex string) string {
	return `{"type":"bgp","bgp":{"message":{"type":"notification","direction":"received"},` +
		`"peer":{"remote":{"address":"10.0.0.1","as":65001}},` +
		`"notification":{"code":` + itoa(code) + `,"subcode":` + itoa(subcode) + `,"data":"` + dataHex + `"}}}`
}

func itoa(n int) string {
	encoded, err := json.Marshal(n)
	if err != nil {
		panic("BUG: marshal int")
	}
	return string(encoded)
}

// TestV4NotificationDataReadable proves X-12 (AC-13): `data` is the field as on
// the wire for an API 6 helper, and for an API 4 one what 5.x hexed, the
// Shutdown Communication decoded (Notification.v4_data). `message` is the
// readable form for both (Notification.text). Expected values: ExaBGP main's
// JSON.notification, `hexstring` is "0x" and uppercase hex.
func TestV4NotificationDataReadable(t *testing.T) {
	cases := []struct {
		name            string
		code, subcode   int
		dataHex         string
		v4Data, v6Data  string
		message, v4Text string
	}{
		{
			// RFC 9003 Section 2: length 3, "bye".
			name: "shutdown communication", code: 6, subcode: 2, dataHex: "03627965",
			v4Data:  "0x" + upperHex(`Shutdown Communication: "bye"`),
			v6Data:  "0x03627965",
			message: `Shutdown Communication: "bye"`,
			v4Text:  "neighbor 10.0.0.1 receive notification code 6 subcode 2 data 0x" + upperHex(`Shutdown Communication: "bye"`) + "\n",
		},
		{
			name: "shutdown without communication", code: 6, subcode: 4, dataHex: "",
			v4Data: "0x", v6Data: "0x", message: "",
			v4Text: "neighbor 10.0.0.1 receive notification code 6 subcode 4 data 0x\n",
		},
		{
			name: "shutdown length zero", code: 6, subcode: 2, dataHex: "00",
			v4Data: "0x" + upperHex("empty Shutdown Communication."), v6Data: "0x00",
			message: "empty Shutdown Communication.",
			v4Text:  "neighbor 10.0.0.1 receive notification code 6 subcode 2 data 0x" + upperHex("empty Shutdown Communication.") + "\n",
		},
		{
			name: "shutdown underrun", code: 6, subcode: 2, dataHex: "0562",
			v4Data:  "0x" + upperHex("invalid Shutdown Communication (buffer underrun) length : 5 [0x62]"),
			v6Data:  "0x0562",
			message: "invalid Shutdown Communication (buffer underrun) length : 5 [0x62]",
			v4Text:  "neighbor 10.0.0.1 receive notification code 6 subcode 2 data 0x" + upperHex("invalid Shutdown Communication (buffer underrun) length : 5 [0x62]") + "\n",
		},
		{
			name: "shutdown with trailing data", code: 6, subcode: 2, dataHex: "0162ff",
			v4Data:  "0x" + upperHex(`Shutdown Communication: "b", trailing data: 0xFF`),
			v6Data:  "0x0162FF",
			message: `Shutdown Communication: "b", trailing data: 0xFF`,
			v4Text:  "neighbor 10.0.0.1 receive notification code 6 subcode 2 data 0x" + upperHex(`Shutdown Communication: "b", trailing data: 0xFF`) + "\n",
		},
		{
			// Not a shutdown: the wire for both, `message` printable ASCII as it is.
			name: "printable data", code: 2, subcode: 2, dataHex: "4142",
			v4Data: "0x4142", v6Data: "0x4142", message: "AB",
			v4Text: "neighbor 10.0.0.1 receive notification code 2 subcode 2 data 0x4142\n",
		},
		{
			// Not printable: `message` is the hex.
			name: "binary data", code: 3, subcode: 1, dataHex: "0001",
			v4Data: "0x0001", v6Data: "0x0001", message: "0x0001",
			v4Text: "neighbor 10.0.0.1 receive notification code 3 subcode 1 data 0x0001\n",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			event := readZeEvent(t, notificationEventZe(tc.code, tc.subcode, tc.dataHex))
			want := func(data string) string {
				encoded, err := json.Marshal(map[string]any{
					"code": tc.code, "subcode": tc.subcode, "data": data, "message": tc.message,
				})
				if err != nil {
					t.Fatalf("marshal: %v", err)
				}
				return string(encoded)
			}
			for _, format := range []struct {
				format Format
				data   string
			}{{FormatV4JSON, tc.v4Data}, {FormatV6JSON, tc.v6Data}} {
				neighbor, ok := event.ExabgpJSON(format.format)["neighbor"].(map[string]any)
				if !ok {
					t.Fatalf("no neighbor")
				}
				sameJSON(t, format.format.String(), neighbor["notification"], want(format.data))
			}
			text, form := event.AppendText(nil)
			if form != TextWritten {
				t.Fatalf("text form %v", form)
			}
			if string(text) != tc.v4Text {
				t.Errorf("v4 text\n got  %q\n want %q", text, tc.v4Text)
			}
		})
	}
}

func upperHex(text string) string {
	const digits = "0123456789ABCDEF"
	out := make([]byte, 0, 2*len(text))
	for i := range len(text) {
		out = append(out, digits[text[i]>>4], digits[text[i]&0x0f])
	}
	return string(out)
}

// TestOpenTextHoldTimeFromTimer proves the text OPEN line reads the hold time
// where ze writes it, under `timer`: it was read flat, and every line said 0.
func TestOpenTextHoldTimeFromTimer(t *testing.T) {
	text, form := readZeEvent(t, openEventZe).AppendText(nil)
	if form != TextWritten {
		t.Fatalf("text form %v", form)
	}
	if !strings.Contains(string(text), " hold_time 180 ") {
		t.Errorf("no hold_time 180 in %q", text)
	}
}

// updateEventZe is a received UPDATE as ze writes it, with one announce group.
func updateEventZe(family, nextHop, nlri, attr string) string {
	return `{"type":"bgp","bgp":{"message":{"type":"update","direction":"received"},` +
		`"peer":{"remote":{"address":"10.0.0.1","as":65001}},` +
		`"update":{"attr":` + attr + `,"nlri":{"` + family + `":[{"action":"add","next-hop":"` + nextHop + `","nlri":[` + nlri + `]}]}}}}`
}

// announcedNextHop is the next hop every UPDATE announcedNLRI reads carries.
const announcedNextHop = "1.2.3.4"

// announcedNLRI answers the one NLRI an UPDATE rendered in JSON announces
// through announcedNextHop.
func announcedNLRI(t *testing.T, rendered map[string]any, family string) map[string]any {
	t.Helper()
	nextHop := announcedNextHop
	neighbor, _ := rendered["neighbor"].(map[string]any)
	message, _ := neighbor["message"].(map[string]any)
	update, _ := message["update"].(map[string]any)
	announce, _ := update["announce"].(map[string]map[string][]any)
	byNextHop := announce[family]
	if len(byNextHop[nextHop]) != 1 {
		t.Fatalf("no %s %s announce in %v", family, nextHop, update)
	}
	nlri, ok := byNextHop[nextHop][0].(map[string]any)
	if !ok {
		t.Fatalf("nlri %v", byNextHop[nextHop][0])
	}
	return nlri
}

// TestV4ShapesRestoredFrom5x proves X-11 for JSON: an API 4 helper reads each
// label as its value alone (Labels.json(raw=False), `[[100]]`) and a flow's
// `string` ending with the next hop (NLRI.v4_text); an API 6 helper keeps the
// raw label entry and the bare flow string (ExaBGP 0e8dc3bd2).
func TestV4ShapesRestoredFrom5x(t *testing.T) {
	labeled := readZeEvent(t, updateEventZe("ipv4/mpls-label", announcedNextHop,
		`{"prefix":"10.0.0.0/24","labels":[[100,1601]]}`, `{"origin":"igp"}`))
	sameJSON(t, "v4 label", announcedNLRI(t, labeled.ExabgpJSON(FormatV4JSON), "ipv4 nlri-mpls")["label"], `[[100]]`)
	sameJSON(t, "v6 label", announcedNLRI(t, labeled.ExabgpJSON(FormatV6JSON), "ipv4 nlri-mpls")["label"], `[[100, 1601]]`)

	flow := readZeEvent(t, updateEventZe("ipv4/flow", announcedNextHop,
		`{"source-ipv4":[["10.0.1.0/24"]]}`, `{"origin":"igp"}`))
	v4String, _ := announcedNLRI(t, flow.ExabgpJSON(FormatV4JSON), "ipv4 flow")["string"].(string)
	v6String, _ := announcedNLRI(t, flow.ExabgpJSON(FormatV6JSON), "ipv4 flow")["string"].(string)
	if v6String == "" {
		t.Fatalf("no flow string")
	}
	if v4String != v6String+" next-hop 1.2.3.4" {
		t.Errorf("v4 flow string %q, want %q", v4String, v6String+" next-hop 1.2.3.4")
	}
}

// TestV4TextLabeledRouteAndClusterList proves X-11 for text, as v4/text.py
// writes a route (LabelBase.v4_text): the labels without their raw value, the
// next hop, then the route distinguisher; and a cluster list of one cluster in
// brackets (ClusterList.__repr__).
func TestV4TextLabeledRouteAndClusterList(t *testing.T) {
	cases := []struct{ name, family, nlri, attr, want string }{
		{
			name: "vpn", family: "ipv4/mpls-vpn",
			nlri: `{"prefix":"10.0.0.0/24","labels":[[100,1601]],"rd":"0:65000:1"}`, attr: `{"origin":"igp"}`,
			want: "neighbor 10.0.0.1 receive update announced 10.0.0.0/24 label 100 next-hop 1.2.3.4 rd 65000:1 origin igp\n",
		},
		{
			name: "label stack", family: "ipv4/mpls-label",
			nlri: `{"prefix":"10.0.0.0/24","labels":[[100,1600],[200,3201]]}`, attr: `{"origin":"igp"}`,
			want: "neighbor 10.0.0.1 receive update announced 10.0.0.0/24 label [ 100 200 ] next-hop 1.2.3.4 origin igp\n",
		},
		{
			name: "one cluster", family: "ipv4/unicast",
			nlri: `"10.0.0.0/24"`, attr: `{"origin":"igp","cluster-list":["1.1.1.1"]}`,
			want: "neighbor 10.0.0.1 receive update announced 10.0.0.0/24 next-hop 1.2.3.4 origin igp cluster-list [ 1.1.1.1 ]\n",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			text, _ := readZeEvent(t, updateEventZe(tc.family, "1.2.3.4", tc.nlri, tc.attr)).AppendText(nil)
			if !strings.Contains(string(text), tc.want) {
				t.Errorf("text\n got  %q\n want %q", text, tc.want)
			}
		})
	}
}
