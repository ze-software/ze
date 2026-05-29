package flowexport

import (
	"strings"
	"testing"
)

func TestParseConfigEmpty(t *testing.T) {
	cfg, err := ParseConfig("")
	if err != nil {
		t.Fatal(err)
	}
	if len(cfg.Collectors) != 0 {
		t.Fatalf("expected 0 collectors, got %d", len(cfg.Collectors))
	}
}

func TestParseConfigSingleCollector(t *testing.T) {
	data := `{"flow-export":{"collector":[{"name":"c1","address":"10.0.0.1","port":6343,"protocol":"sflow","polling-interval":30}]}}`
	cfg, err := ParseConfig(data)
	if err != nil {
		t.Fatal(err)
	}
	if len(cfg.Collectors) != 1 {
		t.Fatalf("expected 1 collector, got %d", len(cfg.Collectors))
	}
	c := cfg.Collectors[0]
	if c.Name != "c1" {
		t.Fatalf("name = %q, want %q", c.Name, "c1")
	}
	if c.Address != "10.0.0.1" {
		t.Fatalf("address = %q, want %q", c.Address, "10.0.0.1")
	}
	if c.Port != 6343 {
		t.Fatalf("port = %d, want %d", c.Port, 6343)
	}
	if c.Protocol != "sflow" {
		t.Fatalf("protocol = %q, want %q", c.Protocol, "sflow")
	}
	if c.PollingInterval != 30 {
		t.Fatalf("polling-interval = %d, want %d", c.PollingInterval, 30)
	}
	if c.TemplateRefresh != 600 {
		t.Fatalf("template-refresh = %d, want default %d", c.TemplateRefresh, 600)
	}
}

func TestParseConfigMultipleCollectors(t *testing.T) {
	data := `{"flow-export":{"collector":[
		{"name":"sflow1","address":"10.0.0.1","port":6343,"protocol":"sflow"},
		{"name":"ipfix1","address":"10.0.0.2","port":4739,"protocol":"ipfix","observation-domain":100}
	]}}`
	cfg, err := ParseConfig(data)
	if err != nil {
		t.Fatal(err)
	}
	if len(cfg.Collectors) != 2 {
		t.Fatalf("expected 2 collectors, got %d", len(cfg.Collectors))
	}
	if cfg.Collectors[1].ObservationDomain != 100 {
		t.Fatalf("observation-domain = %d, want 100", cfg.Collectors[1].ObservationDomain)
	}
}

func TestParseConfigYANGKeyedMap(t *testing.T) {
	data := `{"flow-export":{"collector":{"sflow1":{"address":"10.0.0.1","port":6343,"protocol":"sflow","agent-address":"192.168.1.1"},"ipfix1":{"address":"10.0.0.2","port":4739,"protocol":"ipfix"}}}}`
	cfg, err := ParseConfig(data)
	if err != nil {
		t.Fatal(err)
	}
	if len(cfg.Collectors) != 2 {
		t.Fatalf("expected 2 collectors, got %d", len(cfg.Collectors))
	}
	found := map[string]bool{}
	for _, c := range cfg.Collectors {
		found[c.Name] = true
		if c.Name == "sflow1" {
			if c.Address != "10.0.0.1" {
				t.Errorf("sflow1 address = %q, want 10.0.0.1", c.Address)
			}
			if c.AgentAddress != "192.168.1.1" {
				t.Errorf("sflow1 agent-address = %q, want 192.168.1.1", c.AgentAddress)
			}
		}
	}
	if !found["sflow1"] || !found["ipfix1"] {
		t.Errorf("missing collectors: found = %v", found)
	}
}

func TestValidateGoodConfig(t *testing.T) {
	cfg := &Config{
		Collectors: []CollectorConfig{
			{Name: "c1", Address: "10.0.0.1", Port: 6343, Protocol: "sflow", PollingInterval: 20, TemplateRefresh: 600},
		},
	}
	if err := cfg.Validate(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestValidateBadAddress(t *testing.T) {
	cfg := &Config{
		Collectors: []CollectorConfig{
			{Name: "c1", Address: "not-an-ip", Port: 6343, Protocol: "sflow", PollingInterval: 20, TemplateRefresh: 600},
		},
	}
	err := cfg.Validate()
	if err == nil {
		t.Fatal("expected error for bad address")
	}
	if !strings.Contains(err.Error(), "address") {
		t.Fatalf("error should mention address: %v", err)
	}
}

func TestValidatePortBoundary(t *testing.T) {
	tests := []struct {
		port    int
		wantErr bool
	}{
		{0, true},
		{1, false},
		{65535, false},
		{65536, true},
		{-1, true},
	}
	for _, tt := range tests {
		cfg := &Config{
			Collectors: []CollectorConfig{
				{Name: "c1", Address: "10.0.0.1", Port: tt.port, Protocol: "sflow", PollingInterval: 20, TemplateRefresh: 600},
			},
		}
		err := cfg.Validate()
		if (err != nil) != tt.wantErr {
			t.Errorf("port %d: err = %v, wantErr = %v", tt.port, err, tt.wantErr)
		}
	}
}

func TestValidatePollingIntervalBoundary(t *testing.T) {
	tests := []struct {
		interval int
		wantErr  bool
	}{
		{0, true},
		{1, false},
		{3600, false},
		{3601, true},
	}
	for _, tt := range tests {
		cfg := &Config{
			Collectors: []CollectorConfig{
				{Name: "c1", Address: "10.0.0.1", Port: 6343, Protocol: "sflow", PollingInterval: tt.interval, TemplateRefresh: 600},
			},
		}
		err := cfg.Validate()
		if (err != nil) != tt.wantErr {
			t.Errorf("polling-interval %d: err = %v, wantErr = %v", tt.interval, err, tt.wantErr)
		}
	}
}

func TestValidateTemplateRefreshBoundary(t *testing.T) {
	tests := []struct {
		refresh int
		wantErr bool
	}{
		{0, true},
		{1, false},
		{86400, false},
		{86401, true},
	}
	for _, tt := range tests {
		cfg := &Config{
			Collectors: []CollectorConfig{
				{Name: "c1", Address: "10.0.0.1", Port: 6343, Protocol: "sflow", PollingInterval: 20, TemplateRefresh: tt.refresh},
			},
		}
		err := cfg.Validate()
		if (err != nil) != tt.wantErr {
			t.Errorf("template-refresh %d: err = %v, wantErr = %v", tt.refresh, err, tt.wantErr)
		}
	}
}

func TestValidateBadProtocol(t *testing.T) {
	cfg := &Config{
		Collectors: []CollectorConfig{
			{Name: "c1", Address: "10.0.0.1", Port: 6343, Protocol: "snmp", PollingInterval: 20, TemplateRefresh: 600},
		},
	}
	err := cfg.Validate()
	if err == nil {
		t.Fatal("expected error for bad protocol")
	}
	if !strings.Contains(err.Error(), "unknown protocol") {
		t.Fatalf("error should mention unknown protocol: %v", err)
	}
}

func TestValidateMissingAddress(t *testing.T) {
	cfg := &Config{
		Collectors: []CollectorConfig{
			{Name: "c1", Port: 6343, Protocol: "sflow", PollingInterval: 20, TemplateRefresh: 600},
		},
	}
	err := cfg.Validate()
	if err == nil {
		t.Fatal("expected error for missing address")
	}
	if !strings.Contains(err.Error(), "address is required") {
		t.Fatalf("error should mention address required: %v", err)
	}
}

func TestValidateIPv6Address(t *testing.T) {
	cfg := &Config{
		Collectors: []CollectorConfig{
			{Name: "c1", Address: "2001:db8::1", Port: 6343, Protocol: "ipfix", PollingInterval: 20, TemplateRefresh: 600},
		},
	}
	if err := cfg.Validate(); err != nil {
		t.Fatalf("unexpected error for IPv6 address: %v", err)
	}
}

func TestParseSamplingConfig(t *testing.T) {
	data := `{"flow-export":{"sampling":{"interface":{"eth0":{"rate":2048,"trunc-size":256,"group":3}}}}}`
	cfg, err := ParseConfig(data)
	if err != nil {
		t.Fatal(err)
	}
	if len(cfg.Sampling) != 1 {
		t.Fatalf("expected 1 sampling interface, got %d", len(cfg.Sampling))
	}
	s := cfg.Sampling[0]
	if s.Interface != "eth0" {
		t.Errorf("interface = %q, want eth0", s.Interface)
	}
	if s.Rate != 2048 {
		t.Errorf("rate = %d, want 2048", s.Rate)
	}
	if s.TruncSize != 256 {
		t.Errorf("trunc-size = %d, want 256", s.TruncSize)
	}
	if s.Group != 3 {
		t.Errorf("group = %d, want 3", s.Group)
	}
}

func TestParseSamplingDefaults(t *testing.T) {
	data := `{"flow-export":{"sampling":{"interface":{"eth1":{"rate":1000}}}}}`
	cfg, err := ParseConfig(data)
	if err != nil {
		t.Fatal(err)
	}
	s := cfg.Sampling[0]
	if s.TruncSize != 128 {
		t.Errorf("default trunc-size = %d, want 128", s.TruncSize)
	}
	if s.Group != 1 {
		t.Errorf("default group = %d, want 1", s.Group)
	}
}

func TestParseConntrackAndEnrichment(t *testing.T) {
	data := `{"flow-export":{"conntrack":{"enabled":true,"active-timeout":30},"enrichment":{"bgp":true}}}`
	cfg, err := ParseConfig(data)
	if err != nil {
		t.Fatal(err)
	}
	if !cfg.Conntrack.Enabled {
		t.Error("conntrack not enabled")
	}
	if cfg.Conntrack.ActiveTimeout != 30 {
		t.Errorf("active-timeout = %d, want 30", cfg.Conntrack.ActiveTimeout)
	}
	if !cfg.Enrichment.BGP {
		t.Error("bgp enrichment not enabled")
	}
}

func TestConntrackDefaultActiveTimeout(t *testing.T) {
	data := `{"flow-export":{"conntrack":{"enabled":true}}}`
	cfg, err := ParseConfig(data)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Conntrack.ActiveTimeout != 60 {
		t.Errorf("default active-timeout = %d, want 60", cfg.Conntrack.ActiveTimeout)
	}
}

func TestSamplingValidateBoundaries(t *testing.T) {
	cases := []struct {
		name    string
		s       SamplingConfig
		wantErr bool
	}{
		{"valid", SamplingConfig{Interface: "eth0", Rate: 2048, TruncSize: 128, Group: 1}, false},
		{"rate-zero", SamplingConfig{Interface: "eth0", Rate: 0, TruncSize: 128, Group: 1}, true},
		{"rate-max", SamplingConfig{Interface: "eth0", Rate: 1000000, TruncSize: 128, Group: 1}, false},
		{"rate-over", SamplingConfig{Interface: "eth0", Rate: 1000001, TruncSize: 128, Group: 1}, true},
		{"trunc-low", SamplingConfig{Interface: "eth0", Rate: 1, TruncSize: 63, Group: 1}, true},
		{"trunc-high", SamplingConfig{Interface: "eth0", Rate: 1, TruncSize: 1501, Group: 1}, true},
		{"group-zero", SamplingConfig{Interface: "eth0", Rate: 1, TruncSize: 128, Group: 0}, true},
		{"no-interface", SamplingConfig{Interface: "", Rate: 1, TruncSize: 128, Group: 1}, true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := tc.s.validate()
			if tc.wantErr && err == nil {
				t.Error("expected error, got nil")
			}
			if !tc.wantErr && err != nil {
				t.Errorf("unexpected error: %v", err)
			}
		})
	}
}

func TestConntrackValidateActiveTimeout(t *testing.T) {
	c := &Config{Conntrack: ConntrackConfig{Enabled: true, ActiveTimeout: 0}}
	if err := c.Validate(); err == nil {
		t.Error("expected error for active-timeout=0")
	}
	c = &Config{Conntrack: ConntrackConfig{Enabled: true, ActiveTimeout: 3601}}
	if err := c.Validate(); err == nil {
		t.Error("expected error for active-timeout=3601")
	}
	c = &Config{Conntrack: ConntrackConfig{Enabled: true, ActiveTimeout: 60}}
	if err := c.Validate(); err != nil {
		t.Errorf("unexpected error: %v", err)
	}
}

// TestParseConfigStringValues feeds the string-valued JSON the live daemon
// delivers (config tree leaves are strings, not JSON numbers/bools). The
// original parser used float64/bool-only type assertions, which silently
// dropped every numeric field on this path: ports stayed at their default
// (so datagrams went to the wrong port and collectors saw nothing) and
// sampling rate parsed as 0 (failing validation). This guards that the
// daemon's representation parses identically to the array/number form.
func TestParseConfigStringValues(t *testing.T) {
	data := `{"flow-export":{` +
		`"collector":{"c1":{"address":"10.0.0.1","port":"4739","protocol":"ipfix","polling-interval":"5","template-refresh":"120","sub-agent-id":"7","observation-domain":"42"}},` +
		`"sampling":{"interface":{"eth0":{"rate":"2048","trunc-size":"256","group":"3"}}},` +
		`"conntrack":{"enabled":"true","active-timeout":"30"},` +
		`"enrichment":{"bgp":"true"}}}`
	cfg, err := ParseConfig(data)
	if err != nil {
		t.Fatal(err)
	}
	if len(cfg.Collectors) != 1 {
		t.Fatalf("collectors = %d, want 1", len(cfg.Collectors))
	}
	c := cfg.Collectors[0]
	if c.Port != 4739 {
		t.Errorf("port = %d, want 4739 (string value not coerced)", c.Port)
	}
	if c.PollingInterval != 5 {
		t.Errorf("polling-interval = %d, want 5", c.PollingInterval)
	}
	if c.TemplateRefresh != 120 {
		t.Errorf("template-refresh = %d, want 120", c.TemplateRefresh)
	}
	if c.SubAgentID != 7 {
		t.Errorf("sub-agent-id = %d, want 7", c.SubAgentID)
	}
	if c.ObservationDomain != 42 {
		t.Errorf("observation-domain = %d, want 42", c.ObservationDomain)
	}
	if len(cfg.Sampling) != 1 {
		t.Fatalf("sampling = %d, want 1", len(cfg.Sampling))
	}
	s := cfg.Sampling[0]
	if s.Rate != 2048 {
		t.Errorf("rate = %d, want 2048 (string value not coerced)", s.Rate)
	}
	if s.TruncSize != 256 {
		t.Errorf("trunc-size = %d, want 256", s.TruncSize)
	}
	if s.Group != 3 {
		t.Errorf("group = %d, want 3", s.Group)
	}
	if !cfg.Conntrack.Enabled {
		t.Error("conntrack.enabled = false, want true (string \"true\" not coerced)")
	}
	if cfg.Conntrack.ActiveTimeout != 30 {
		t.Errorf("active-timeout = %d, want 30", cfg.Conntrack.ActiveTimeout)
	}
	if !cfg.Enrichment.BGP {
		t.Error("enrichment.bgp = false, want true (string \"true\" not coerced)")
	}
	if err := cfg.Validate(); err != nil {
		t.Errorf("Validate: %v", err)
	}
}
