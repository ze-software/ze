package static

import (
	"math"
	"net/netip"
	"strconv"
	"testing"

	"github.com/ze-software/ze/internal/core/routingtable"
)

func defReg() *routingtable.Registry { return routingtable.New(nil) }

func wrap(routeKey, routeJSON string) string {
	return `{"static":{"table":{"default":{"route":{"` + routeKey + `":` + routeJSON + `}}}}}`
}

func TestParseStaticConfig(t *testing.T) {
	input := wrap("10.0.0.0/8", `{"next":{"hop":{"192.168.1.1":{}}}}`)
	routes, err := parseStaticConfig(input, defReg())
	if err != nil {
		t.Fatal(err)
	}
	if len(routes) != 1 {
		t.Fatalf("got %d routes, want 1", len(routes))
	}
	r := routes[0]
	if r.Prefix != netip.MustParsePrefix("10.0.0.0/8") {
		t.Errorf("prefix = %s, want 10.0.0.0/8", r.Prefix)
	}
	if r.Action != actionForward {
		t.Errorf("action = %s, want forward", r.Action)
	}
	if len(r.NextHops) != 1 {
		t.Fatalf("got %d next-hops, want 1", len(r.NextHops))
	}
	if r.NextHops[0].Address != netip.MustParseAddr("192.168.1.1") {
		t.Errorf("next-hop = %s, want 192.168.1.1", r.NextHops[0].Address)
	}
	if r.NextHops[0].Weight != 1 {
		t.Errorf("weight = %d, want 1 (default)", r.NextHops[0].Weight)
	}
}

func TestParseStaticConfigMultiNextHop(t *testing.T) {
	input := wrap("0.0.0.0/0", `{"next":{"hop":{"10.0.0.1":{"weight":3},"10.0.0.2":{"weight":1}}}}`)
	routes, err := parseStaticConfig(input, defReg())
	if err != nil {
		t.Fatal(err)
	}
	if len(routes) != 1 {
		t.Fatalf("got %d routes, want 1", len(routes))
	}
	r := routes[0]
	if len(r.NextHops) != 2 {
		t.Fatalf("got %d next-hops, want 2", len(r.NextHops))
	}
}

func TestParseStaticConfigWeight(t *testing.T) {
	input := wrap("10.0.0.0/8", `{"next":{"hop":{"10.0.0.1":{"weight":100}}}}`)
	routes, err := parseStaticConfig(input, defReg())
	if err != nil {
		t.Fatal(err)
	}
	if routes[0].NextHops[0].Weight != 100 {
		t.Errorf("weight = %d, want 100", routes[0].NextHops[0].Weight)
	}
}

func TestParseStaticConfigBlackhole(t *testing.T) {
	input := wrap("192.0.2.0/24", `{"blackhole":{}}`)
	routes, err := parseStaticConfig(input, defReg())
	if err != nil {
		t.Fatal(err)
	}
	if len(routes) != 1 {
		t.Fatalf("got %d routes, want 1", len(routes))
	}
	if routes[0].Action != actionBlackhole {
		t.Errorf("action = %s, want blackhole", routes[0].Action)
	}
	if len(routes[0].NextHops) != 0 {
		t.Errorf("got %d next-hops for blackhole, want 0", len(routes[0].NextHops))
	}
}

func TestParseStaticConfigReject(t *testing.T) {
	input := wrap("198.51.100.0/24", `{"reject":{}}`)
	routes, err := parseStaticConfig(input, defReg())
	if err != nil {
		t.Fatal(err)
	}
	if routes[0].Action != actionReject {
		t.Errorf("action = %s, want reject", routes[0].Action)
	}
}

func TestParseStaticConfigIPv6(t *testing.T) {
	input := wrap("2001:db8::/32", `{"next":{"hop":{"2001:db8::1":{}}}}`)
	routes, err := parseStaticConfig(input, defReg())
	if err != nil {
		t.Fatal(err)
	}
	if routes[0].Prefix != netip.MustParsePrefix("2001:db8::/32") {
		t.Errorf("prefix = %s, want 2001:db8::/32", routes[0].Prefix)
	}
	if routes[0].NextHops[0].Address != netip.MustParseAddr("2001:db8::1") {
		t.Errorf("next-hop = %s, want 2001:db8::1", routes[0].NextHops[0].Address)
	}
}

func TestParseStaticConfigTag(t *testing.T) {
	input := wrap("172.16.0.0/12", `{"next":{"hop":{"10.0.0.1":{}}},"tag":100}`)
	routes, err := parseStaticConfig(input, defReg())
	if err != nil {
		t.Fatal(err)
	}
	if routes[0].Tag != 100 {
		t.Errorf("tag = %d, want 100", routes[0].Tag)
	}
}

func TestParseStaticConfigDescription(t *testing.T) {
	input := wrap("10.0.0.0/8", `{"next":{"hop":{"10.0.0.1":{}}},"description":"test route"}`)
	routes, err := parseStaticConfig(input, defReg())
	if err != nil {
		t.Fatal(err)
	}
	if routes[0].Description != "test route" {
		t.Errorf("description = %q, want %q", routes[0].Description, "test route")
	}
}

func TestParseStaticConfigEmpty(t *testing.T) {
	input := `{"static":{}}`
	routes, err := parseStaticConfig(input, defReg())
	if err != nil {
		t.Fatal(err)
	}
	if len(routes) != 0 {
		t.Errorf("got %d routes, want 0", len(routes))
	}
}

func TestParseStaticConfigInvalidPrefix(t *testing.T) {
	input := wrap("not-a-prefix", `{"next":{"hop":{"10.0.0.1":{}}}}`)
	_, err := parseStaticConfig(input, defReg())
	if err == nil {
		t.Fatal("expected error for invalid prefix")
	}
}

func TestParseStaticConfigBFDProfile(t *testing.T) {
	input := wrap("10.0.0.0/8", `{"next":{"hop":{"10.0.0.1":{"bfd-profile":"wan-fast"}}}}`)
	routes, err := parseStaticConfig(input, defReg())
	if err != nil {
		t.Fatal(err)
	}
	if routes[0].NextHops[0].BFDProfile != "wan-fast" {
		t.Errorf("bfd-profile = %q, want %q", routes[0].NextHops[0].BFDProfile, "wan-fast")
	}
}

func TestParseStaticConfigInterface(t *testing.T) {
	input := wrap("10.0.0.0/8", `{"next":{"hop":{"fe80::1":{"interface":"eth0"}}}}`)
	routes, err := parseStaticConfig(input, defReg())
	if err != nil {
		t.Fatal(err)
	}
	if routes[0].NextHops[0].Interface != "eth0" {
		t.Errorf("interface = %q, want %q", routes[0].NextHops[0].Interface, "eth0")
	}
}

func TestParseStaticConfigMetric(t *testing.T) {
	input := wrap("10.0.0.0/8", `{"next":{"hop":{"10.0.0.1":{}}},"metric":200}`)
	routes, err := parseStaticConfig(input, defReg())
	if err != nil {
		t.Fatal(err)
	}
	if routes[0].Metric != 200 {
		t.Errorf("metric = %d, want 200", routes[0].Metric)
	}
}

// TestParseConfig_StringValuedDelivery pins the ACTUAL config format the plugin
// config framework delivers: numeric leaves (metric, tag, weight) arrive as JSON
// strings ("200"), not native numbers (Tree.values is map[string]string).
// mapUint32 previously asserted .(float64) only, so string-valued metric, tag,
// and weight silently fell back to zero.
func TestParseConfig_StringValuedDelivery(t *testing.T) {
	input := wrap("10.0.0.0/8", `{"next":{"hop":{"10.0.0.1":{"weight":"100"}}},"metric":"200","tag":"300"}`)
	routes, err := parseStaticConfig(input, defReg())
	if err != nil {
		t.Fatal(err)
	}
	if len(routes) != 1 {
		t.Fatalf("got %d routes, want 1", len(routes))
	}
	r := routes[0]
	if r.Metric != 200 {
		t.Errorf("metric = %d, want 200 (string-valued)", r.Metric)
	}
	if r.Tag != 300 {
		t.Errorf("tag = %d, want 300 (string-valued)", r.Tag)
	}
	if len(r.NextHops) != 1 {
		t.Fatalf("got %d next-hops, want 1", len(r.NextHops))
	}
	if r.NextHops[0].Weight != 100 {
		t.Errorf("weight = %d, want 100 (string-valued)", r.NextHops[0].Weight)
	}
}

func TestParseStaticConfigNoAction(t *testing.T) {
	input := wrap("10.0.0.0/8", `{}`)
	_, err := parseStaticConfig(input, defReg())
	if err == nil {
		t.Fatal("expected error for route with no action")
	}
}

func TestParseStaticConfigMalformedJSON(t *testing.T) {
	_, err := parseStaticConfig("{broken", defReg())
	if err == nil {
		t.Fatal("expected error for malformed JSON")
	}
}

func TestParseStaticConfigPrefixCanonicalized(t *testing.T) {
	input := wrap("10.1.2.3/8", `{"next":{"hop":{"10.0.0.1":{}}}}`)
	routes, err := parseStaticConfig(input, defReg())
	if err != nil {
		t.Fatal(err)
	}
	want := pfx("10.0.0.0/8")
	if routes[0].Prefix != want {
		t.Errorf("prefix = %s, want %s (canonicalized)", routes[0].Prefix, want)
	}
}

// VALIDATES: validateRouteMetric rejects a metric that does not survive the
// conversion to Go int that netlink.Route.Priority performs, while accepting
// every value this build can program.
// PREVENTS: a `metric` above MaxInt32 on a 32-bit build silently becoming the
// kernel's default metric. buildRoute converts to int (backend_linux.go) and
// the encoder emits RTA_PRIORITY only when Priority > 0
// (vendor/github.com/vishvananda/netlink/route_linux.go:1069), so a negative
// Priority drops the attribute and the route is installed at the wrong metric
// with no error, silently reordering route preference.
func TestValidateRouteMetricRejectsUnencodable(t *testing.T) {
	// The bound is passed explicitly so this exercises the 32-bit rejection on
	// a 64-bit host, where maxNetlinkInt is above every uint32.
	const maxInt32 = uint64(math.MaxInt32)

	tests := []struct {
		name       string
		metric     uint32
		maxEncode  uint64
		wantReject bool
	}{
		{"zero", 0, maxInt32, false},
		{"below bound", 200, maxInt32, false},
		{"at bound", math.MaxInt32, maxInt32, false},
		{"one above bound", math.MaxInt32 + 1, maxInt32, true},
		{"max uint32 on 32-bit", math.MaxUint32, maxInt32, true},
		{"max uint32 on 64-bit", math.MaxUint32, uint64(math.MaxInt), false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateRouteMetric(tt.metric, tt.maxEncode)
			if tt.wantReject && err == nil {
				t.Fatalf("validateRouteMetric(%d, %d): accepted, want rejection", tt.metric, tt.maxEncode)
			}
			if !tt.wantReject && err != nil {
				t.Fatalf("validateRouteMetric(%d, %d): unexpected error: %v", tt.metric, tt.maxEncode, err)
			}
		})
	}
}

// VALIDATES: the bound parseRoute applies is this build's own int limit, so a
// metric is accepted exactly when it survives the int conversion.
// PREVENTS: the parser being wired to a hardcoded 32-bit bound, which would
// reject usable metrics on the 64-bit targets Ze ships.
func TestParseRouteMetricBoundMatchesBuild(t *testing.T) {
	for _, metric := range []uint64{200, math.MaxInt32, math.MaxInt32 + 1, math.MaxUint32} {
		input := wrap("10.0.0.0/8", `{"next":{"hop":{"10.0.0.1":{}}},"metric":`+strconv.FormatUint(metric, 10)+`}`)
		_, err := parseStaticConfig(input, defReg())
		fits := metric <= uint64(math.MaxInt)
		if fits != (err == nil) {
			t.Errorf("metric %d: fits in int = %v, accepted = %v (err=%v)", metric, fits, err == nil, err)
		}
	}
}
