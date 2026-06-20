package events

import (
	"encoding/json"
	"net/netip"
	"strings"
	"testing"

	bgptypes "codeberg.org/thomas-mangin/ze/internal/component/bgp/types"
)

func TestBestChangeEntryJSON_BackwardsCompatible(t *testing.T) {
	entry := BestChangeEntry{
		Action:   bgptypes.RouteActionAdd,
		Prefix:   netip.MustParsePrefix("10.0.0.0/24"),
		NextHop:  netip.MustParseAddr("192.168.1.1"),
		Protocol: "bgp",
	}
	data, err := json.Marshal(entry)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	s := string(data)
	if contains(s, "route-type") {
		t.Error("zero RouteType should be omitted from JSON")
	}
	if contains(s, "metric") {
		t.Error("zero Metric should be omitted from JSON")
	}
	if contains(s, "table-id") {
		t.Error("zero TableID should be omitted from JSON")
	}
	if contains(s, "ecmp-paths") {
		t.Error("nil ECMPPaths should be omitted from JSON")
	}
	if contains(s, "srv6-sid") {
		t.Error("zero SRv6SID should be omitted from JSON")
	}
}

func TestBestChangeEntryJSON_RichFields(t *testing.T) {
	entry := BestChangeEntry{
		Action:    bgptypes.RouteActionAdd,
		Prefix:    netip.MustParsePrefix("10.0.0.0/24"),
		NextHop:   netip.MustParseAddr("192.168.1.1"),
		Protocol:  "bgp",
		RouteType: RouteTypeUnicast,
		Metric:    100,
		TableID:   42,
		ECMPPaths: []ECMPPath{
			{NextHop: netip.MustParseAddr("192.168.1.1"), Weight: 1},
			{NextHop: netip.MustParseAddr("192.168.1.2"), Weight: 1},
		},
	}
	data, err := json.Marshal(entry)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	s := string(data)
	if !contains(s, `"route-type":1`) {
		t.Errorf("expected route-type:1 in %s", s)
	}
	if !contains(s, `"metric":100`) {
		t.Errorf("expected metric:100 in %s", s)
	}
	if !contains(s, `"table-id":42`) {
		t.Errorf("expected table-id:42 in %s", s)
	}
	if !contains(s, `"ecmp-paths"`) {
		t.Errorf("expected ecmp-paths in %s", s)
	}

	var decoded BestChangeEntry
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if decoded.RouteType != RouteTypeUnicast {
		t.Errorf("RouteType = %d, want %d", decoded.RouteType, RouteTypeUnicast)
	}
	if decoded.Metric != 100 {
		t.Errorf("Metric = %d, want 100", decoded.Metric)
	}
	if decoded.TableID != 42 {
		t.Errorf("TableID = %d, want 42", decoded.TableID)
	}
	if len(decoded.ECMPPaths) != 2 {
		t.Fatalf("ECMPPaths len = %d, want 2", len(decoded.ECMPPaths))
	}
	if decoded.ECMPPaths[0].Weight != 1 {
		t.Errorf("ECMPPaths[0].Weight = %d, want 1", decoded.ECMPPaths[0].Weight)
	}
}

func TestBestChangeEntryJSON_Blackhole(t *testing.T) {
	entry := BestChangeEntry{
		Action:    bgptypes.RouteActionAdd,
		Prefix:    netip.MustParsePrefix("192.0.2.0/24"),
		Protocol:  "static",
		RouteType: RouteTypeBlackhole,
	}
	data, err := json.Marshal(entry)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	var decoded BestChangeEntry
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if decoded.RouteType != RouteTypeBlackhole {
		t.Errorf("RouteType = %d, want %d (blackhole)", decoded.RouteType, RouteTypeBlackhole)
	}
	if decoded.NextHop.IsValid() {
		t.Error("blackhole route should have zero next-hop")
	}
}

func TestBestChangeEntryJSON_ECMPWithLabels(t *testing.T) {
	entry := BestChangeEntry{
		Action:   bgptypes.RouteActionAdd,
		Prefix:   netip.MustParsePrefix("10.1.0.0/16"),
		Protocol: "bgp",
		ECMPPaths: []ECMPPath{
			{NextHop: netip.MustParseAddr("10.0.0.1"), Weight: 2, Labels: []uint32{100, 200}},
			{NextHop: netip.MustParseAddr("10.0.0.2"), Weight: 1, Labels: []uint32{300}},
		},
	}
	data, err := json.Marshal(entry)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	var decoded BestChangeEntry
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if len(decoded.ECMPPaths) != 2 {
		t.Fatalf("ECMPPaths len = %d, want 2", len(decoded.ECMPPaths))
	}
	if len(decoded.ECMPPaths[0].Labels) != 2 {
		t.Errorf("ECMPPaths[0].Labels len = %d, want 2", len(decoded.ECMPPaths[0].Labels))
	}
	if decoded.ECMPPaths[0].Labels[0] != 100 || decoded.ECMPPaths[0].Labels[1] != 200 {
		t.Errorf("ECMPPaths[0].Labels = %v, want [100, 200]", decoded.ECMPPaths[0].Labels)
	}
}

func contains(s, substr string) bool {
	return strings.Contains(s, substr)
}
