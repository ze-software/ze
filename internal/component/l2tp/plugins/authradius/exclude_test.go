package l2tpauthradius

import (
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/ze-software/ze/internal/component/config"
	"github.com/ze-software/ze/internal/component/l2tp/plugins/authradius/yang"
	"github.com/ze-software/ze/internal/component/l2tp/ppp"
	"github.com/ze-software/ze/internal/component/radius"
)

// The tests in this file drive the real configuration pipeline rather than
// building an attributeExclusions by hand: the YANG module parses the text,
// ToPluginMap lowers it, and parseConfigFromTree reads it. A container that
// parses and never reaches a packet builder fails them.

// exclusionText wraps one `attributes exclude` body in a configuration Ze
// accepts. An empty body writes no attributes container at all, which is the
// deployment that excludes nothing.
func exclusionText(excludeBody string) string {
	attributes := ""
	if excludeBody != "" {
		attributes = "            attributes {\n                exclude {\n" +
			excludeBody + "\n                }\n            }\n"
	}
	return `l2tp {
    enabled true
    auth {
        radius {
            nas-identifier lns1
            server radius1 {
                address 127.0.0.1
                port 1812
                shared-key testing123
            }
` + attributes + `        }
    }
}`
}

// exclusionConfig loads one exclude body and fails the test when the load does.
func exclusionConfig(t *testing.T, excludeBody string) *radiusConfig {
	t.Helper()
	tree, err := config.ParseTreeWithYANG(exclusionText(excludeBody), map[string]string{
		"l2tp-auth-radius": yang.ZeL2TPAuthRadiusConfYANG,
	})
	if err != nil {
		t.Fatalf("parse tree: %v", err)
	}
	cfg, err := parseConfigFromTree(tree.ToPluginMap())
	if err != nil {
		t.Fatalf("parse config: %v", err)
	}
	return cfg
}

// exclusionLoadError loads one exclude body and returns whatever refused it.
func exclusionLoadError(excludeBody string) error {
	tree, err := config.ParseTreeWithYANG(exclusionText(excludeBody), map[string]string{
		"l2tp-auth-radius": yang.ZeL2TPAuthRadiusConfYANG,
	})
	if err != nil {
		return err
	}
	_, err = parseConfigFromTree(tree.ToPluginMap())
	return err
}

// acctExcluding returns an accounting instance carrying the exclusions the
// given body configures.
func acctExcluding(t *testing.T, excludeBody string) *radiusAcct {
	t.Helper()
	acct := newRADIUSAcct()
	acct.setExclusions(exclusionConfig(t, excludeBody).Exclusions)
	return acct
}

// excludedSession has a value for every excludable attribute, so a missing
// attribute in these tests is always the exclusion and never an absent value.
func excludedSession() *acctSession {
	return &acctSession{
		username:         "alice",
		acctSessID:       "1-2-1",
		callingStationID: "00:11:22:33:44:55",
		nasPortID:        "lns1:1.2",
		peerAddr:         "10.0.0.7",
	}
}

// attrTypes lists the attribute types of a packet in the order they were built.
func attrTypes(pkt *radius.Packet) []uint8 {
	types := make([]uint8, 0, len(pkt.Attrs))
	for _, attr := range pkt.Attrs {
		types = append(types, attr.Type)
	}
	return types
}

// wireAttrTypes encodes the packet and walks the octet stream, so what it
// reports is what a RADIUS server reads rather than what the builder held.
func wireAttrTypes(t *testing.T, pkt *radius.Packet) []uint8 {
	t.Helper()
	buf := make([]byte, radius.MaxPacketLen)
	size, err := pkt.EncodeTo(buf, 0)
	if err != nil {
		t.Fatalf("encode: %v", err)
	}
	types := []uint8{}
	for off := radius.HeaderLen; off < size; {
		if off+2 > size {
			t.Fatalf("attribute header runs past the packet at offset %d", off)
		}
		length := int(buf[off+1])
		if length < 2 {
			t.Fatalf("attribute at offset %d declares length %d", off, length)
		}
		types = append(types, buf[off])
		off += length
	}
	return types
}

func containsType(types []uint8, attrType uint8) bool {
	return slices.Contains(types, attrType)
}

// TestExcludedAttributeIsAbsentFromTheWire is the wiring test. It starts at the
// configuration text an operator writes and ends at the encoded octets, so
// every step between the two has to work for it to pass.
func TestExcludedAttributeIsAbsentFromTheWire(t *testing.T) {
	acct := acctExcluding(t, "                    calling-station-id;")
	pkt := acct.buildAcctPacket(excludedSession(), "lns1", nil, radius.AcctStatusStart, 0)

	onWire := wireAttrTypes(t, pkt)
	if containsType(onWire, radius.AttrCallingStationID) {
		t.Error("Calling-Station-Id reached the wire although the operator excluded it")
	}
	if !containsType(onWire, radius.AttrAcctSessionID) {
		t.Error("Acct-Session-Id left the wire, and no exclusion can name it")
	}
}

// TestNoExclusionsLeavesThePacketUnchanged covers AC-1: a deployment that
// writes no attributes container sends the same octets it sent before this
// feature existed.
func TestNoExclusionsLeavesThePacketUnchanged(t *testing.T) {
	saved := acctNow
	acctNow = func() time.Time { return time.Unix(1756900000, 0) }
	defer func() { acctNow = saved }()

	cfg := exclusionConfig(t, "")
	if cfg.Exclusions != nil {
		t.Fatalf("an absent attributes container configured exclusions: %v", cfg.Exclusions)
	}

	configured := newRADIUSAcct()
	configured.setExclusions(cfg.Exclusions)
	untouched := newRADIUSAcct()

	for _, statusType := range []uint8{radius.AcctStatusStart, radius.AcctStatusInterimUpdate, radius.AcctStatusStop} {
		want := wireAttrTypes(t, untouched.buildAcctPacket(excludedSession(), "lns1", nil, statusType, 60))
		got := wireAttrTypes(t, configured.buildAcctPacket(excludedSession(), "lns1", nil, statusType, 60))
		if len(got) != len(want) {
			t.Fatalf("status-type %d: %d attributes, want %d", statusType, len(got), len(want))
		}
		for index := range want {
			if got[index] != want[index] {
				t.Fatalf("status-type %d: attribute %d is type %d, want %d", statusType, index, got[index], want[index])
			}
		}
	}
}

// TestExcludeWithNoPacketTypeAppliesEverywhere covers AC-2.
func TestExcludeWithNoPacketTypeAppliesEverywhere(t *testing.T) {
	acct := acctExcluding(t, "                    calling-station-id;")

	for _, statusType := range []uint8{radius.AcctStatusStart, radius.AcctStatusInterimUpdate, radius.AcctStatusStop} {
		pkt := acct.buildAcctPacket(excludedSession(), "lns1", nil, statusType, 60)
		if pkt.FindAttr(radius.AttrCallingStationID) != nil {
			t.Errorf("status-type %d: Calling-Station-Id present", statusType)
		}
		if pkt.FindAttr(radius.AttrNASPortID) == nil {
			t.Errorf("status-type %d: NAS-Port-Id absent, and no exclusion named it", statusType)
		}
	}
}

// TestExcludePerPacketType covers AC-3: the record types the operator named
// lose the attribute and the others keep it.
func TestExcludePerPacketType(t *testing.T) {
	acct := acctExcluding(t, "                    calling-station-id {\n                        packet-type [ accounting-interim ]\n                    }")

	interim := acct.buildAcctPacket(excludedSession(), "lns1", nil, radius.AcctStatusInterimUpdate, 60)
	if interim.FindAttr(radius.AttrCallingStationID) != nil {
		t.Error("Interim record carries Calling-Station-Id")
	}
	for _, statusType := range []uint8{radius.AcctStatusStart, radius.AcctStatusStop} {
		pkt := acct.buildAcctPacket(excludedSession(), "lns1", nil, statusType, 60)
		if pkt.FindAttr(radius.AttrCallingStationID) == nil {
			t.Errorf("status-type %d: Calling-Station-Id absent although only the Interim record was named", statusType)
		}
	}
}

// TestExcludeBeatsAKnownValue covers AC-4, and it is the test that tells the
// two mechanisms apart.
//
// RFC 2865 Section 5: "Text of length zero (0) MUST NOT be sent; omit the
// entire attribute instead." That rule already drops an attribute whose value
// is empty, and it is NOT what this feature does. The session below has a
// Calling-Station-Id, so the third case can only be the exclusion.
func TestExcludeBeatsAKnownValue(t *testing.T) {
	sess := excludedSession()
	if sess.callingStationID == "" {
		t.Fatal("the fixture must carry a value, or this test proves nothing")
	}

	kept := newRADIUSAcct().buildAcctPacket(sess, "lns1", nil, radius.AcctStatusStart, 0)
	if kept.FindAttr(radius.AttrCallingStationID) == nil {
		t.Fatal("a known value must be sent when nothing excludes it")
	}

	empty := &acctSession{username: "alice", acctSessID: "1-2-1"}
	omitted := newRADIUSAcct().buildAcctPacket(empty, "lns1", nil, radius.AcctStatusStart, 0)
	if omitted.FindAttr(radius.AttrCallingStationID) != nil {
		t.Error("an empty value must send no attribute, which is the omit-when-empty rule")
	}

	acct := acctExcluding(t, "                    calling-station-id;")
	excluded := acct.buildAcctPacket(sess, "lns1", nil, radius.AcctStatusStart, 0)
	if excluded.FindAttr(radius.AttrCallingStationID) != nil {
		t.Error("an excluded attribute whose value is known must still be absent")
	}
}

// TestExcludeOnAccessRequestOnly covers AC-5.
func TestExcludeOnAccessRequestOnly(t *testing.T) {
	body := "                    nas-port-id {\n                        packet-type [ access-request ]\n                    }"
	cfg := exclusionConfig(t, body)

	req := ppp.EventAuthRequest{TunnelID: 1, SessionID: 2, Username: "alice", Method: ppp.AuthMethodPAP}
	attrs, ok := buildAccessRequestAttrs(req, "lns1", nil, attributePolicy{
		nasPortIDFormat: "{nas-id}:{tunnel-id}.{session-id}",
		exclusions:      cfg.Exclusions,
	})
	if !ok {
		t.Fatal("a PAP request MUST build an Access-Request")
	}
	access := &radius.Packet{Code: radius.CodeAccessRequest, Attrs: attrs}
	if access.FindAttr(radius.AttrNASPortID) != nil {
		t.Error("Access-Request carries NAS-Port-Id although the operator excluded it there")
	}
	if access.FindAttr(radius.AttrUserName) == nil {
		t.Error("User-Name left the Access-Request, and no exclusion named it")
	}

	acct := newRADIUSAcct()
	acct.setExclusions(cfg.Exclusions)
	for _, statusType := range []uint8{radius.AcctStatusStart, radius.AcctStatusInterimUpdate, radius.AcctStatusStop} {
		pkt := acct.buildAcctPacket(excludedSession(), "lns1", nil, statusType, 60)
		if pkt.FindAttr(radius.AttrNASPortID) == nil {
			t.Errorf("status-type %d: NAS-Port-Id absent although only the Access-Request was named", statusType)
		}
	}
}

// TestTwoExclusionsApplyIndependently covers AC-6.
func TestTwoExclusionsApplyIndependently(t *testing.T) {
	acct := acctExcluding(t,
		"                    calling-station-id;\n"+
			"                    framed-ip-address {\n                        packet-type [ accounting-stop ]\n                    }")

	start := acct.buildAcctPacket(excludedSession(), "lns1", nil, radius.AcctStatusStart, 0)
	if start.FindAttr(radius.AttrCallingStationID) != nil {
		t.Error("Start record carries Calling-Station-Id")
	}
	if start.FindAttr(radius.AttrFramedIPAddress) == nil {
		t.Error("Start record lost Framed-IP-Address, which was excluded from the Stop record alone")
	}

	stop := acct.buildAcctPacket(excludedSession(), "lns1", nil, radius.AcctStatusStop, 60)
	if stop.FindAttr(radius.AttrCallingStationID) != nil {
		t.Error("Stop record carries Calling-Station-Id")
	}
	if stop.FindAttr(radius.AttrFramedIPAddress) != nil {
		t.Error("Stop record carries Framed-IP-Address")
	}
}

// TestExcludePreservesTheRemainingOrder covers AC-7: the surviving attributes
// keep the order the builder wrote them in.
//
// RFC 2866 Section 3: "The order of attributes of different types is not
// required to be preserved", so a server may not depend on it. Ze preserves it
// anyway, because a filter that reorders is a filter doing something it was not
// asked to do.
func TestExcludePreservesTheRemainingOrder(t *testing.T) {
	full := newRADIUSAcct().buildAcctPacket(excludedSession(), "lns1", nil, radius.AcctStatusStop, 60)
	acct := acctExcluding(t, "                    calling-station-id;")
	filtered := acct.buildAcctPacket(excludedSession(), "lns1", nil, radius.AcctStatusStop, 60)

	want := []uint8{}
	for _, attrType := range attrTypes(full) {
		if attrType == radius.AttrCallingStationID {
			continue
		}
		want = append(want, attrType)
	}

	got := attrTypes(filtered)
	if len(got) != len(want) {
		t.Fatalf("filtered record has %d attributes, want %d", len(got), len(want))
	}
	for index := range want {
		if got[index] != want[index] {
			t.Fatalf("attribute %d is type %d, want %d", index, got[index], want[index])
		}
	}
}

// TestRequiredAttributesAreNotInTheEnum covers AC-8.
//
// RFC 2866 Section 5.13 legend: "1  Exactly one instance of this attribute MUST
// be present", which the table gives to Acct-Status-Type and Acct-Session-Id.
// Note 1 of the same section: "An Accounting-Request MUST contain either a
// NAS-IP-Address or a NAS-Identifier (or both)". None of the four can be
// excluded, so the schema does not name them and neither does the Go map that
// reads it.
func TestRequiredAttributesAreNotInTheEnum(t *testing.T) {
	for _, name := range []string{"acct-status-type", "acct-session-id", "nas-identifier", "nas-ip-address"} {
		err := exclusionLoadError("                    " + name + ";")
		if err == nil {
			t.Errorf("%s was accepted as an excludable attribute", name)
			continue
		}
		if !strings.Contains(err.Error(), name) {
			t.Errorf("the refusal of %s does not name it: %v", name, err)
		}
	}

	for _, attrType := range []uint8{
		radius.AttrAcctStatusType,
		radius.AttrAcctSessionID,
		radius.AttrNASIdentifier,
		radius.AttrNASIPAddress,
	} {
		for name, mapped := range excludableAttributes {
			if mapped == attrType {
				t.Errorf("excludableAttributes maps %q to required attribute %d", name, attrType)
			}
		}
	}
}

// TestExcludeRefusesUnknownWords covers AC-9: an unknown attribute, an unknown
// record type, and a record type the named attribute cannot appear in are each
// refused when the configuration loads.
func TestExcludeRefusesUnknownWords(t *testing.T) {
	cases := []struct {
		name string
		body string
		says []string
	}{
		{
			name: "unknown attribute",
			body: "                    calling-number;",
			says: []string{"calling-number"},
		},
		{
			name: "unknown packet type",
			body: "                    calling-station-id {\n                        packet-type [ accounting-off ]\n                    }",
			says: []string{"packet-type", "accounting-off", "accounting-start"},
		},
		{
			// RFC 2866 Section 5.10: Acct-Terminate-Cause "can only be present
			// in Accounting-Request records where the Acct-Status-Type is set
			// to Stop", so the schema gives this attribute one legal value.
			name: "packet type the attribute cannot reach",
			body: "                    acct-terminate-cause {\n                        packet-type [ accounting-start ]\n                    }",
			says: []string{"packet-type", "accounting-start", "accounting-stop"},
		},
		{
			name: "a value where the name stands alone",
			body: "                    calling-station-id nonsense;",
			says: []string{"nonsense"},
		},
	}

	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			err := exclusionLoadError(test.body)
			if err == nil {
				t.Fatal("the configuration loaded")
			}
			for _, word := range test.says {
				if !strings.Contains(err.Error(), word) {
					t.Errorf("the refusal does not name %q: %v", word, err)
				}
			}
		})
	}
}

// TestFullyExcludedRecordIsStillConformant covers AC-10: an operator who
// excludes everything the schema offers still sends a conformant record.
//
// RFC 2866 Section 5.13 legend: "1  Exactly one instance of this attribute MUST
// be present", and Note 1: "An Accounting-Request MUST contain either a
// NAS-IP-Address or a NAS-Identifier (or both)".
func TestFullyExcludedRecordIsStillConformant(t *testing.T) {
	acct := acctExcluding(t,
		"                    calling-station-id;\n"+
			"                    event-timestamp;\n"+
			"                    acct-delay-time;\n"+
			"                    acct-terminate-cause;\n"+
			"                    nas-port-id;\n"+
			"                    framed-ip-address;")

	pkt := acct.buildAcctPacket(excludedSession(), "lns1", nil, radius.AcctStatusStop, 60)

	for _, attrType := range []uint8{
		radius.AttrCallingStationID,
		radius.AttrEventTimestamp,
		radius.AttrAcctTerminateCause,
		radius.AttrNASPortID,
		radius.AttrFramedIPAddress,
	} {
		if pkt.FindAttr(attrType) != nil {
			t.Errorf("attribute %d survived its exclusion", attrType)
		}
	}
	if !pkt.OmitAcctDelayTime {
		t.Error("the packet does not tell the client to hold Acct-Delay-Time back")
	}

	if pkt.FindAttr(radius.AttrAcctStatusType) == nil {
		t.Error("Acct-Status-Type is absent, and RFC 2866 Section 5.13 counts it 1")
	}
	if pkt.FindAttr(radius.AttrAcctSessionID) == nil {
		t.Error("Acct-Session-Id is absent, and RFC 2866 Section 5.13 counts it 1")
	}
	if pkt.FindAttr(radius.AttrNASIdentifier) == nil && pkt.FindAttr(radius.AttrNASIPAddress) == nil {
		t.Error("neither NAS-Identifier nor NAS-IP-Address is present, and Note 1 requires one")
	}
}

// TestExcludeAcctDelayTimePerPacketType covers the one excludable attribute the
// accounting builder does not append. RFC 2866 Section 5.2 measures
// Acct-Delay-Time from the moment the client starts trying to send, so the
// client writes it; the record therefore carries the operator's decision as a
// packet field rather than as a missing attribute.
func TestExcludeAcctDelayTimePerPacketType(t *testing.T) {
	acct := acctExcluding(t, "                    acct-delay-time {\n                        packet-type [ accounting-interim ]\n                    }")

	interim := acct.buildAcctPacket(excludedSession(), "lns1", nil, radius.AcctStatusInterimUpdate, 60)
	if !interim.OmitAcctDelayTime {
		t.Error("the Interim record does not carry the exclusion the operator wrote")
	}
	for _, statusType := range []uint8{radius.AcctStatusStart, radius.AcctStatusStop} {
		pkt := acct.buildAcctPacket(excludedSession(), "lns1", nil, statusType, 60)
		if pkt.OmitAcctDelayTime {
			t.Errorf("status-type %d carries the exclusion although only the Interim record was named", statusType)
		}
	}
}
