package static

import (
	"net/netip"
	"testing"
)

func TestParseStaticConfig(t *testing.T) {
	input := `{"static":{"route":[{"prefix":"10.0.0.0/8","next-hop":[{"address":"192.168.1.1"}]}]}}`
	routes, err := parseStaticConfig(input)
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
	input := `{"static":{"route":[{"prefix":"0.0.0.0/0","next-hop":[{"address":"10.0.0.1","weight":3},{"address":"10.0.0.2","weight":1}]}]}}`
	routes, err := parseStaticConfig(input)
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
	if r.NextHops[0].Weight != 3 {
		t.Errorf("nh[0].weight = %d, want 3", r.NextHops[0].Weight)
	}
	if r.NextHops[1].Weight != 1 {
		t.Errorf("nh[1].weight = %d, want 1", r.NextHops[1].Weight)
	}
}

func TestParseStaticConfigWeight(t *testing.T) {
	input := `{"static":{"route":[{"prefix":"10.0.0.0/8","next-hop":[{"address":"10.0.0.1","weight":100}]}]}}`
	routes, err := parseStaticConfig(input)
	if err != nil {
		t.Fatal(err)
	}
	if routes[0].NextHops[0].Weight != 100 {
		t.Errorf("weight = %d, want 100", routes[0].NextHops[0].Weight)
	}
}

func TestParseStaticConfigBlackhole(t *testing.T) {
	input := `{"static":{"route":[{"prefix":"192.0.2.0/24","blackhole":{}}]}}`
	routes, err := parseStaticConfig(input)
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
	input := `{"static":{"route":[{"prefix":"198.51.100.0/24","reject":{}}]}}`
	routes, err := parseStaticConfig(input)
	if err != nil {
		t.Fatal(err)
	}
	if routes[0].Action != actionReject {
		t.Errorf("action = %s, want reject", routes[0].Action)
	}
}

func TestParseStaticConfigIPv6(t *testing.T) {
	input := `{"static":{"route":[{"prefix":"2001:db8::/32","next-hop":[{"address":"2001:db8::1"}]}]}}`
	routes, err := parseStaticConfig(input)
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
	input := `{"static":{"route":[{"prefix":"172.16.0.0/12","next-hop":[{"address":"10.0.0.1"}],"tag":100}]}}`
	routes, err := parseStaticConfig(input)
	if err != nil {
		t.Fatal(err)
	}
	if routes[0].Tag != 100 {
		t.Errorf("tag = %d, want 100", routes[0].Tag)
	}
}

func TestParseStaticConfigDescription(t *testing.T) {
	input := `{"static":{"route":[{"prefix":"10.0.0.0/8","next-hop":[{"address":"10.0.0.1"}],"description":"test route"}]}}`
	routes, err := parseStaticConfig(input)
	if err != nil {
		t.Fatal(err)
	}
	if routes[0].Description != "test route" {
		t.Errorf("description = %q, want %q", routes[0].Description, "test route")
	}
}

func TestParseStaticConfigEmpty(t *testing.T) {
	input := `{"static":{}}`
	routes, err := parseStaticConfig(input)
	if err != nil {
		t.Fatal(err)
	}
	if len(routes) != 0 {
		t.Errorf("got %d routes, want 0", len(routes))
	}
}

func TestParseStaticConfigInvalidPrefix(t *testing.T) {
	input := `{"static":{"route":[{"prefix":"not-a-prefix","next-hop":[{"address":"10.0.0.1"}]}]}}`
	_, err := parseStaticConfig(input)
	if err == nil {
		t.Fatal("expected error for invalid prefix")
	}
}

func TestParseStaticConfigBFDProfile(t *testing.T) {
	input := `{"static":{"route":[{"prefix":"10.0.0.0/8","next-hop":[{"address":"10.0.0.1","bfd-profile":"wan-fast"}]}]}}`
	routes, err := parseStaticConfig(input)
	if err != nil {
		t.Fatal(err)
	}
	if routes[0].NextHops[0].BFDProfile != "wan-fast" {
		t.Errorf("bfd-profile = %q, want %q", routes[0].NextHops[0].BFDProfile, "wan-fast")
	}
}

func TestParseStaticConfigInterface(t *testing.T) {
	input := `{"static":{"route":[{"prefix":"10.0.0.0/8","next-hop":[{"address":"fe80::1","interface":"eth0"}]}]}}`
	routes, err := parseStaticConfig(input)
	if err != nil {
		t.Fatal(err)
	}
	if routes[0].NextHops[0].Interface != "eth0" {
		t.Errorf("interface = %q, want %q", routes[0].NextHops[0].Interface, "eth0")
	}
}

func TestParseStaticConfigMetric(t *testing.T) {
	input := `{"static":{"route":[{"prefix":"10.0.0.0/8","next-hop":[{"address":"10.0.0.1"}],"metric":200}]}}`
	routes, err := parseStaticConfig(input)
	if err != nil {
		t.Fatal(err)
	}
	if routes[0].Metric != 200 {
		t.Errorf("metric = %d, want 200", routes[0].Metric)
	}
}

func TestParseStaticConfigNoAction(t *testing.T) {
	input := `{"static":{"route":[{"prefix":"10.0.0.0/8"}]}}`
	_, err := parseStaticConfig(input)
	if err == nil {
		t.Fatal("expected error for route with no action")
	}
}

func TestParseStaticConfigNegativeMetric(t *testing.T) {
	input := `{"static":{"route":[{"prefix":"10.0.0.0/8","next-hop":[{"address":"10.0.0.1"}],"metric":-1}]}}`
	_, err := parseStaticConfig(input)
	if err == nil {
		t.Fatal("expected error for negative metric")
	}
}

func TestParseStaticConfigNegativeWeight(t *testing.T) {
	input := `{"static":{"route":[{"prefix":"10.0.0.0/8","next-hop":[{"address":"10.0.0.1","weight":-5}]}]}}`
	_, err := parseStaticConfig(input)
	if err == nil {
		t.Fatal("expected error for negative weight")
	}
}

func TestParseStaticConfigWeightExceeds65535(t *testing.T) {
	input := `{"static":{"route":[{"prefix":"10.0.0.0/8","next-hop":[{"address":"10.0.0.1","weight":70000}]}]}}`
	_, err := parseStaticConfig(input)
	if err == nil {
		t.Fatal("expected error for weight > 65535")
	}
}

func TestParseStaticConfigMalformedJSON(t *testing.T) {
	_, err := parseStaticConfig("{broken")
	if err == nil {
		t.Fatal("expected error for malformed JSON")
	}
}

func TestParseStaticConfigPrefixCanonicalized(t *testing.T) {
	input := `{"static":{"route":[{"prefix":"10.1.2.3/8","next-hop":[{"address":"10.0.0.1"}]}]}}`
	routes, err := parseStaticConfig(input)
	if err != nil {
		t.Fatal(err)
	}
	want := pfx("10.0.0.0/8")
	if routes[0].Prefix != want {
		t.Errorf("prefix = %s, want %s (canonicalized)", routes[0].Prefix, want)
	}
}

func TestParseStaticConfigOverflowMetric(t *testing.T) {
	input := `{"static":{"route":[{"prefix":"10.0.0.0/8","next-hop":[{"address":"10.0.0.1"}],"metric":5000000000}]}}`
	_, err := parseStaticConfig(input)
	if err == nil {
		t.Fatal("expected error for metric exceeding uint32 max")
	}
}

func TestParseStaticConfigDuplicatePrefix(t *testing.T) {
	input := `{"static":{"route":[
		{"prefix":"10.0.0.0/8","next-hop":[{"address":"1.1.1.1"}]},
		{"prefix":"10.0.0.0/8","next-hop":[{"address":"2.2.2.2"}]}
	]}}`
	_, err := parseStaticConfig(input)
	if err == nil {
		t.Fatal("expected error for duplicate prefix")
	}
}
