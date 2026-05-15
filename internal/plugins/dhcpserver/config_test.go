package dhcpserver

import (
	"net/netip"
	"testing"
)

func TestParseConfig(t *testing.T) {
	t.Parallel()

	data := `{
		"service": {
			"dhcp-server": {
				"enabled": "true",
				"listen-interface": ["br0", "eth1"],
				"shared-network": {
					"LAN": {
						"subnet": {
							"192.168.1.0/24": {
								"range": {
									"start": "192.168.1.100",
									"stop": "192.168.1.200"
								},
								"lease-time": "3600",
								"default-router": "192.168.1.1",
								"dns-server": ["8.8.8.8", "8.8.4.4"],
								"domain-name": "home.lan",
								"static-mapping": {
									"printer": {
										"mac-address": "aa:bb:cc:dd:ee:ff",
										"ip-address": "192.168.1.10"
									}
								}
							}
						}
					}
				}
			}
		}
	}`

	cfg, err := parseConfig(data)
	if err != nil {
		t.Fatalf("parseConfig: %v", err)
	}
	if !cfg.Enabled {
		t.Error("expected enabled=true")
	}
	if len(cfg.ListenInterfaces) != 2 {
		t.Fatalf("expected 2 listen-interfaces, got %d", len(cfg.ListenInterfaces))
	}
	if cfg.ListenInterfaces[0] != "br0" || cfg.ListenInterfaces[1] != "eth1" {
		t.Errorf("listen-interfaces = %v", cfg.ListenInterfaces)
	}
	if len(cfg.SharedNetworks) != 1 {
		t.Fatalf("expected 1 shared-network, got %d", len(cfg.SharedNetworks))
	}

	sn := cfg.SharedNetworks[0]
	if sn.Name != "LAN" {
		t.Errorf("shared-network name = %q", sn.Name)
	}
	if len(sn.Subnets) != 1 {
		t.Fatalf("expected 1 subnet, got %d", len(sn.Subnets))
	}

	sub := sn.Subnets[0]
	if sub.Prefix != netip.MustParsePrefix("192.168.1.0/24") {
		t.Errorf("prefix = %v", sub.Prefix)
	}
	if sub.RangeStart != netip.MustParseAddr("192.168.1.100") {
		t.Errorf("range start = %v", sub.RangeStart)
	}
	if sub.RangeStop != netip.MustParseAddr("192.168.1.200") {
		t.Errorf("range stop = %v", sub.RangeStop)
	}
	if sub.LeaseTimeSec != 3600 {
		t.Errorf("lease-time = %d", sub.LeaseTimeSec)
	}
	if sub.DefaultRouter != netip.MustParseAddr("192.168.1.1") {
		t.Errorf("default-router = %v", sub.DefaultRouter)
	}
	if len(sub.DNSServers) != 2 {
		t.Fatalf("expected 2 dns-servers, got %d", len(sub.DNSServers))
	}
	if sub.DomainName != "home.lan" {
		t.Errorf("domain-name = %q", sub.DomainName)
	}
	if len(sub.StaticMappings) != 1 {
		t.Fatalf("expected 1 static-mapping, got %d", len(sub.StaticMappings))
	}

	sm := sub.StaticMappings[0]
	if sm.Name != "printer" {
		t.Errorf("static-mapping name = %q", sm.Name)
	}
	if sm.MAC.String() != "aa:bb:cc:dd:ee:ff" {
		t.Errorf("static-mapping mac = %v", sm.MAC)
	}
	if sm.IP != netip.MustParseAddr("192.168.1.10") {
		t.Errorf("static-mapping ip = %v", sm.IP)
	}
}

func TestParseConfigDisabled(t *testing.T) {
	t.Parallel()

	data := `{"service": {"dhcp-server": {}}}`
	cfg, err := parseConfig(data)
	if err != nil {
		t.Fatalf("parseConfig: %v", err)
	}
	if cfg.Enabled {
		t.Error("expected enabled=false by default")
	}
}

func TestParseConfigDefaultLeaseTime(t *testing.T) {
	t.Parallel()

	data := `{
		"service": {
			"dhcp-server": {
				"enabled": "true",
				"listen-interface": ["br0"],
				"shared-network": {
					"LAN": {
						"subnet": {
							"10.0.0.0/24": {
								"range": {
									"start": "10.0.0.10",
									"stop": "10.0.0.20"
								}
							}
						}
					}
				}
			}
		}
	}`

	cfg, err := parseConfig(data)
	if err != nil {
		t.Fatalf("parseConfig: %v", err)
	}
	if len(cfg.SharedNetworks) != 1 || len(cfg.SharedNetworks[0].Subnets) != 1 {
		t.Fatal("expected 1 subnet")
	}
	if cfg.SharedNetworks[0].Subnets[0].LeaseTimeSec != defaultLeaseTimeSec {
		t.Errorf("expected default lease time %d, got %d", defaultLeaseTimeSec, cfg.SharedNetworks[0].Subnets[0].LeaseTimeSec)
	}
}

func TestParseConfigValidationErrors(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		data string
	}{
		{
			name: "range start outside subnet",
			data: `{"service":{"dhcp-server":{"enabled":"true","listen-interface":["br0"],"shared-network":{"L":{"subnet":{"192.168.1.0/24":{"range":{"start":"10.0.0.1","stop":"192.168.1.200"}}}}}}}}`,
		},
		{
			name: "range stop outside subnet",
			data: `{"service":{"dhcp-server":{"enabled":"true","listen-interface":["br0"],"shared-network":{"L":{"subnet":{"192.168.1.0/24":{"range":{"start":"192.168.1.100","stop":"10.0.0.200"}}}}}}}}`,
		},
		{
			name: "range start after stop",
			data: `{"service":{"dhcp-server":{"enabled":"true","listen-interface":["br0"],"shared-network":{"L":{"subnet":{"192.168.1.0/24":{"range":{"start":"192.168.1.200","stop":"192.168.1.100"}}}}}}}}`,
		},
		{
			name: "static mapping IP outside subnet",
			data: `{"service":{"dhcp-server":{"enabled":"true","listen-interface":["br0"],"shared-network":{"L":{"subnet":{"192.168.1.0/24":{"range":{"start":"192.168.1.100","stop":"192.168.1.200"},"static-mapping":{"x":{"mac-address":"aa:bb:cc:dd:ee:ff","ip-address":"10.0.0.1"}}}}}}}}}`,
		},
		{
			name: "invalid MAC",
			data: `{"service":{"dhcp-server":{"enabled":"true","listen-interface":["br0"],"shared-network":{"L":{"subnet":{"192.168.1.0/24":{"range":{"start":"192.168.1.100","stop":"192.168.1.200"},"static-mapping":{"x":{"mac-address":"invalid","ip-address":"192.168.1.10"}}}}}}}}}`,
		},
		{
			name: "invalid subnet prefix",
			data: `{"service":{"dhcp-server":{"enabled":"true","listen-interface":["br0"],"shared-network":{"L":{"subnet":{"not-a-prefix":{"range":{"start":"192.168.1.100","stop":"192.168.1.200"}}}}}}}}`,
		},
		{
			name: "lease-time below minimum",
			data: `{"service":{"dhcp-server":{"enabled":"true","listen-interface":["br0"],"shared-network":{"L":{"subnet":{"192.168.1.0/24":{"range":{"start":"192.168.1.100","stop":"192.168.1.200"},"lease-time":"59"}}}}}}}`,
		},
		{
			name: "lease-time above maximum",
			data: `{"service":{"dhcp-server":{"enabled":"true","listen-interface":["br0"],"shared-network":{"L":{"subnet":{"192.168.1.0/24":{"range":{"start":"192.168.1.100","stop":"192.168.1.200"},"lease-time":"604801"}}}}}}}`,
		},
		{
			name: "domain-name too long",
			data: `{"service":{"dhcp-server":{"enabled":"true","listen-interface":["br0"],"shared-network":{"L":{"subnet":{"192.168.1.0/24":{"range":{"start":"192.168.1.100","stop":"192.168.1.200"},"domain-name":"` + string(make([]byte, 256)) + `"}}}}}}}`,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			_, err := parseConfig(tc.data)
			if err == nil {
				t.Error("expected validation error")
			}
		})
	}
}
