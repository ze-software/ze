package format

import (
	"encoding/json"
	"errors"
	"net"
	"strings"
	"testing"

	"github.com/ze-software/ze/internal/component/plugin/registry"
	"github.com/ze-software/ze/internal/core/bgp/nlri"
	"github.com/ze-software/ze/internal/core/family"
)

// TestUndecodableNLRIIsNeverWrittenAsARoute registers a plugin decoder that
// refuses the octets and renders one NLRI of its family, with and without a
// carried Path Identifier. The element must use the event's error convention,
// so a consumer reading `prefix` finds none.
//
// VALIDATES: a plugin decode failure writes {"error":...,"parsed":false,"raw":HEX}
// naming the decoder's reason, and keeps the carried path-id (nlri.HasPathID).
// PREVENTS: {"prefix":"wire[<family>](N bytes)"}, a route nobody sent.
func TestUndecodableNLRIIsNeverWrittenAsARoute(t *testing.T) {
	claimable := unclaimedFamilyForFormat(t)

	snapshot := registry.Snapshot()
	t.Cleanup(func() { registry.Restore(snapshot) })

	err := registry.Register(registry.Registration{
		Name:        "test-format-refusing-decoder",
		Description: "synthetic NLRI plugin whose decoder refuses every NLRI",
		Families:    []string{claimable},
		RunEngine:   func(net.Conn) int { return 0 },
		CLIHandler:  func([]string) int { return 0 },
		InProcessNLRIDecoder: func(_, _ string, _, _ bool) (any, error) {
			return nil, errors.New("synthetic: truncated route")
		},
	})
	if err != nil {
		t.Fatalf("register synthetic plugin: %v", err)
	}
	fam, held := family.LookupFamily(claimable)
	if !held {
		t.Fatalf("family registry does not hold %s", claimable)
	}

	cases := []struct {
		name     string
		data     []byte
		addPath  bool
		wantRaw  string
		wantPath bool
	}{
		{name: "plain", data: []byte{0x18, 0x0a, 0x00}, wantRaw: "180A00"},
		{name: "add-path identifier zero", data: []byte{0, 0, 0, 0, 0x18, 0x0a, 0x00}, addPath: true, wantRaw: "00000000180A00", wantPath: true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			wire, wireErr := nlri.NewWireNLRI(fam, tc.data, tc.addPath)
			if wireErr != nil {
				t.Fatalf("wrap NLRI: %v", wireErr)
			}
			got := appendNLRIJSONValue(nil, wire, fam, false)
			if strings.Contains(string(got), "wire[") {
				t.Fatalf("undecodable NLRI written as a route: %s", got)
			}
			var element map[string]any
			if jsonErr := json.Unmarshal(got, &element); jsonErr != nil {
				t.Fatalf("element is not a JSON object: %v: %s", jsonErr, got)
			}
			if _, hasPrefix := element["prefix"]; hasPrefix {
				t.Errorf("undecodable NLRI carries a prefix member: %s", got)
			}
			if element["parsed"] != false {
				t.Errorf(`want "parsed":false, got %s`, got)
			}
			if element["raw"] != tc.wantRaw {
				t.Errorf("raw = %v, want %s", element["raw"], tc.wantRaw)
			}
			reason, _ := element["error"].(string)
			if !strings.Contains(reason, "synthetic: truncated route") {
				t.Errorf("error %q does not carry the decoder's reason", reason)
			}
			if _, hasPath := element["path-id"]; hasPath != tc.wantPath {
				t.Errorf("path-id present = %v, want %v: %s", hasPath, tc.wantPath, got)
			}
		})
	}
}

// TestUnclaimedOpaqueNLRIIsNeverWrittenAsARoute renders an NLRI of a family no
// plugin decodes and whose routes are not keyed by a prefix. With nothing to
// decode it, the element must say so rather than name the carrier.
//
// VALIDATES: an opaque NLRI with no decoder writes the error convention.
// PREVENTS: {"prefix":"wire[<family>](N bytes)"} when no plugin claims the family.
func TestUnclaimedOpaqueNLRIIsNeverWrittenAsARoute(t *testing.T) {
	fam := unclaimedOpaqueFamily(t)
	wire, err := nlri.NewWireNLRI(fam, []byte{0x01, 0x02, 0x03}, false)
	if err != nil {
		t.Fatalf("wrap NLRI: %v", err)
	}
	got := appendNLRIJSONValue(nil, wire, fam, false)
	if strings.Contains(string(got), "wire[") {
		t.Fatalf("undecodable NLRI written as a route: %s", got)
	}
	if !strings.Contains(string(got), `"parsed":false,"raw":"010203"`) {
		t.Errorf("want the error convention with the raw octets, got %s", got)
	}
}

// unclaimedOpaqueFamily answers a registered family no plugin claims whose
// routes the WireNLRI carrier cannot name by a prefix.
func unclaimedOpaqueFamily(t *testing.T) family.Family {
	t.Helper()
	claimed := registry.FamilyMap()
	for _, name := range family.RegisteredFamilyNames() {
		if claimed[name] != "" {
			continue
		}
		fam, held := family.LookupFamily(name)
		if !held {
			continue
		}
		wire, err := nlri.NewWireNLRI(fam, []byte{0x01, 0x02, 0x03}, false)
		if err != nil {
			continue
		}
		if _, named := wire.RoutePrefix(); !named {
			return fam
		}
	}
	t.Skip("every unclaimed family in this test binary keys its routes by a prefix")
	return family.Family{}
}
