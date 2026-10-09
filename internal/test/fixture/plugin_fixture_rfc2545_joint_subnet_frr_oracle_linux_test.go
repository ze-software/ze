//go:build linux

// Design: docs/functional-tests.md -- FRR schema failures are not RFC verdicts.
package fixture

import (
	"encoding/json"
	"net/netip"
	"strings"
	"testing"
)

const jointSubnetFRRRouteJSON = `{"prefix":"2001:db8:5701::/48","paths":[{"peer":{"peerId":"2001:db8:a::1"},"nexthops":[{"ip":"2001:db8:a::9","afi":"ipv6","scope":"global"}]}]}`

// TestJointSubnetFRRRouteRequiredFields distinguishes missing/null/incompatible
// fields from supported records, before a received-field semantic judgment.
func TestJointSubnetFRRRouteRequiredFields(t *testing.T) {
	for _, field := range []struct {
		level, name string
		wrong       any
	}{
		{"route", "prefix", 7},
		{"route", "paths", "not-an-array"},
		{"path", "peer", 7},
		{"peer", "peerId", 7},
		{"path", "nexthops", "not-an-array"},
		{"hop", "ip", 7},
		{"hop", "afi", 7},
		{"hop", "scope", 7},
	} {
		for _, mode := range []string{"missing", "null", "wrong-type"} {
			t.Run(field.level+"/"+field.name+"/"+mode, func(t *testing.T) {
				var object map[string]any
				if err := json.Unmarshal([]byte(jointSubnetFRRRouteJSON), &object); err != nil {
					t.Fatal(err)
				}
				path := object["paths"].([]any)[0].(map[string]any)
				target := object
				switch field.level {
				case "path":
					target = path
				case "peer":
					target = path["peer"].(map[string]any)
				case "hop":
					target = path["nexthops"].([]any)[0].(map[string]any)
				}
				switch mode {
				case "missing":
					delete(target, field.name)
				case "null":
					target[field.name] = nil
				case "wrong-type":
					target[field.name] = field.wrong
				}
				data, err := json.Marshal(object)
				if err != nil {
					t.Fatal(err)
				}
				_, err = parseJointSubnetFRRRoute(data)
				if err == nil {
					t.Fatal("incompatible schema accepted")
				}
			})
		}
	}
}

// TestJointSubnetFRRRouteAbsenceAndValues keeps genuine absence distinct from
// present empty lists and wrong values; the latter must reach the semantic gate.
func TestJointSubnetFRRRouteAbsenceAndValues(t *testing.T) {
	global := netip.MustParseAddr("2001:db8:a::9")
	const prefix = "2001:db8:5701::/48"
	absent, err := parseJointSubnetFRRRoute([]byte("{}"))
	if err != nil {
		t.Fatal(err)
	}
	if absent.Prefix != nil {
		t.Fatal("empty response classified as a present route")
	}
	for _, data := range []string{"null", "[]", `{"unknown":true}`, `{"prefix":null}`, `{"prefix":"x","paths":[null]}`} {
		if _, err := parseJointSubnetFRRRoute([]byte(data)); err == nil {
			t.Fatalf("incompatible response accepted: %s", data)
		}
	}
	route, err := parseJointSubnetFRRRoute([]byte(jointSubnetFRRRouteJSON))
	if err != nil {
		t.Fatal(err)
	}
	if err := assertJointSubnetFRRRoute(route, prefix, global, false); err != nil {
		t.Fatalf("valid global-only route rejected: %v", err)
	}
	for _, data := range []string{
		`{"prefix":"2001:db8:5701::/48","paths":[]}`,
		strings.Replace(jointSubnetFRRRouteJSON, `"2001:db8:a::1"`, `""`, 1),
		strings.Replace(jointSubnetFRRRouteJSON, `"2001:db8:a::9"`, `"2001:db8:b::9"`, 1),
		strings.Replace(jointSubnetFRRRouteJSON, `"ipv6"`, `"ipv4"`, 1),
		strings.Replace(jointSubnetFRRRouteJSON, `"global"`, `"link-local"`, 1),
		`{"prefix":"2001:db8:5701::/48","paths":[{"peer":{"peerId":"2001:db8:a::1"},"nexthops":[]}]}`,
	} {
		route, err := parseJointSubnetFRRRoute([]byte(data))
		if err != nil {
			t.Fatalf("present wrong value classified as schema failure: %v", err)
		}
		err = assertJointSubnetFRRRoute(route, prefix, global, false)
		if err == nil {
			t.Fatalf("present wrong value accepted: %s", data)
		}
	}
}

// TestJointSubnetFRRNeighborReadiness rejects incompatible schemas without
// mistaking FRR's documented not-yet-created instance for a protocol failure.
func TestJointSubnetFRRNeighborReadiness(t *testing.T) {
	for _, tc := range []struct {
		name, data     string
		ready, wantErr bool
	}{
		{"instance-absent", `{}`, false, false},
		{"idle", `{"2001:db8:a::1":{"bgpState":"Idle","messageStats":{"updatesRecv":0}}}`, false, false},
		{"established-without-update", `{"2001:db8:a::1":{"bgpState":"Established","messageStats":{"updatesRecv":0}}}`, false, false},
		{"update-without-established", `{"2001:db8:a::1":{"bgpState":"Active","messageStats":{"updatesRecv":1}}}`, false, false},
		{"ready", `{"2001:db8:a::1":{"bgpState":"Established","messageStats":{"updatesRecv":1}}}`, true, false},
		{"configured-neighbor-absent", `{"bgpNoSuchNeighbor":true}`, false, true},
		{"null", `null`, false, true},
		{"array", `[]`, false, true},
		{"unknown", `{"unknown":{}}`, false, true},
		{"neighbor-null", `{"2001:db8:a::1":null}`, false, true},
		{"state-absent", `{"2001:db8:a::1":{"messageStats":{"updatesRecv":1}}}`, false, true},
		{"state-null", `{"2001:db8:a::1":{"bgpState":null,"messageStats":{"updatesRecv":1}}}`, false, true},
		{"state-wrong-type", `{"2001:db8:a::1":{"bgpState":1,"messageStats":{"updatesRecv":1}}}`, false, true},
		{"updates-absent", `{"2001:db8:a::1":{"bgpState":"Established","messageStats":{}}}`, false, true},
		{"updates-null", `{"2001:db8:a::1":{"bgpState":"Established","messageStats":{"updatesRecv":null}}}`, false, true},
		{"updates-wrong-type", `{"2001:db8:a::1":{"bgpState":"Established","messageStats":{"updatesRecv":"1"}}}`, false, true},
		{"absence-null", `{"bgpNoSuchNeighbor":null}`, false, true},
		{"absence-wrong-type", `{"bgpNoSuchNeighbor":"true"}`, false, true},
		{"absence-false", `{"bgpNoSuchNeighbor":false}`, false, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			ready, err := jointSubnetFRRNeighborReady([]byte(tc.data))
			if ready != tc.ready {
				t.Fatalf("ready=%v, want %v", ready, tc.ready)
			}
			if (err != nil) != tc.wantErr {
				t.Fatalf("error=%v, want error=%v", err, tc.wantErr)
			}
		})
	}
}

// TestJointSubnetFRRControlCommunity checks the required received marker,
// including absence/null/type failures and present-but-wrong values.
func TestJointSubnetFRRControlCommunity(t *testing.T) {
	base := strings.Replace(jointSubnetFRRRouteJSON, "5701", "5702", 1)
	for _, tc := range []struct {
		name, field          string
		parseError, accepted bool
	}{
		{"present", `"community":{"list":["65001:7"]},`, false, true},
		{"absent", "", false, false},
		{"null-object", `"community":null,`, false, false},
		{"wrong-object-type", `"community":7,`, true, false},
		{"absent-list", `"community":{},`, false, false},
		{"null-list", `"community":{"list":null},`, false, false},
		{"wrong-list-type", `"community":{"list":"65001:7"},`, true, false},
		{"null-element", `"community":{"list":[null]},`, false, false},
		{"wrong-element-type", `"community":{"list":[7]},`, true, false},
		{"empty-list", `"community":{"list":[]},`, false, false},
		{"empty-value", `"community":{"list":[""]},`, false, false},
		{"wrong-as", `"community":{"list":["65004:7"]},`, false, false},
		{"wrong-value", `"community":{"list":["65001:0"]},`, false, false},
		{"extra-value", `"community":{"list":["65001:7","65001:8"]},`, false, false},
		{"duplicate", `"community":{"list":["65001:7","65001:7"]},`, false, false},
		{"med-is-not-marker", `"metric":7,`, false, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			data := strings.Replace(base, `"peer":`, tc.field+`"peer":`, 1)
			route, err := parseJointSubnetFRRRoute([]byte(data))
			if (err != nil) != tc.parseError {
				t.Fatalf("parse error=%v, want error=%v", err, tc.parseError)
			}
			if err != nil {
				return
			}
			err = assertJointSubnetFRRControl(route, netip.MustParseAddr("2001:db8:a::9"))
			if (err == nil) != tc.accepted {
				t.Fatalf("control error=%v, want accepted=%v", err, tc.accepted)
			}
		})
	}
}
