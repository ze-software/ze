package dhcpserver

import (
	"net/netip"
	"strings"
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
	if len(sub.Ranges) != 1 {
		t.Fatalf("expected 1 range, got %d", len(sub.Ranges))
	}
	if sub.Ranges[0].Start != netip.MustParseAddr("192.168.1.100") {
		t.Errorf("range start = %v", sub.Ranges[0].Start)
	}
	if sub.Ranges[0].Stop != netip.MustParseAddr("192.168.1.200") {
		t.Errorf("range stop = %v", sub.Ranges[0].Stop)
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

func TestParseConfigMultipleRanges(t *testing.T) {
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
									"low": {
										"start": "10.0.0.10",
										"stop": "10.0.0.20"
									},
									"high": {
										"start": "10.0.0.100",
										"stop": "10.0.0.200"
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
	sub := cfg.SharedNetworks[0].Subnets[0]
	if len(sub.Ranges) != 2 {
		t.Fatalf("expected 2 ranges, got %d", len(sub.Ranges))
	}
	if sub.Ranges[0].Start != netip.MustParseAddr("10.0.0.10") {
		t.Errorf("first range start = %v", sub.Ranges[0].Start)
	}
	if sub.Ranges[1].Start != netip.MustParseAddr("10.0.0.100") {
		t.Errorf("second range start = %v", sub.Ranges[1].Start)
	}
}

func TestParseConfigSingleNamedRange(t *testing.T) {
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
									"pool1": {
										"start": "10.0.0.10",
										"stop": "10.0.0.50"
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
	sub := cfg.SharedNetworks[0].Subnets[0]
	if len(sub.Ranges) != 1 {
		t.Fatalf("expected 1 range, got %d", len(sub.Ranges))
	}
	if sub.Ranges[0].Name != "pool1" {
		t.Errorf("range name = %q, want pool1", sub.Ranges[0].Name)
	}
	if sub.Ranges[0].Start != netip.MustParseAddr("10.0.0.10") {
		t.Errorf("range start = %v", sub.Ranges[0].Start)
	}
	if sub.Ranges[0].Stop != netip.MustParseAddr("10.0.0.50") {
		t.Errorf("range stop = %v", sub.Ranges[0].Stop)
	}
}

func TestParseConfigOldRangeFormat(t *testing.T) {
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
									"stop": "10.0.0.50"
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
	sub := cfg.SharedNetworks[0].Subnets[0]
	if len(sub.Ranges) != 1 {
		t.Fatalf("expected 1 range from old format, got %d", len(sub.Ranges))
	}
	if sub.Ranges[0].Name != "default" {
		t.Errorf("range name = %q, want default", sub.Ranges[0].Name)
	}
	if sub.Ranges[0].Start != netip.MustParseAddr("10.0.0.10") {
		t.Errorf("range start = %v", sub.Ranges[0].Start)
	}
}

func TestParseConfigRangeNamedStart(t *testing.T) {
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
									"start": {
										"start": "10.0.0.10",
										"stop": "10.0.0.50"
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
	sub := cfg.SharedNetworks[0].Subnets[0]
	if len(sub.Ranges) != 1 {
		t.Fatalf("expected 1 range, got %d", len(sub.Ranges))
	}
	if sub.Ranges[0].Name != "start" {
		t.Errorf("range name = %q, want start", sub.Ranges[0].Name)
	}
}

func TestParseConfigOverlappingRanges(t *testing.T) {
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
									"a": {
										"start": "10.0.0.10",
										"stop": "10.0.0.50"
									},
									"b": {
										"start": "10.0.0.40",
										"stop": "10.0.0.80"
									}
								}
							}
						}
					}
				}
			}
		}
	}`

	_, err := parseConfig(data)
	if err == nil {
		t.Fatal("expected overlap error")
	}
}

func TestParsePXEConfig(t *testing.T) {
	t.Parallel()

	data := `{
		"service": {
			"dhcp-server": {
				"enabled": "true",
				"listen-interface": ["eth0"],
				"pxe": {
					"enabled": "true",
					"tftp-server": "192.168.1.1",
					"bootfile-bios": "ipxe.pxe",
					"bootfile-uefi": "ipxe.efi"
				},
				"shared-network": {
					"LAN": {
						"subnet": {
							"192.168.1.0/24": {
								"range": {
									"start": "192.168.1.100",
									"stop": "192.168.1.200"
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
	if !cfg.PXE.Enabled {
		t.Error("expected PXE enabled=true")
	}
	if cfg.PXE.TFTPServer != netip.MustParseAddr("192.168.1.1") {
		t.Errorf("PXE tftp-server = %v, want 192.168.1.1", cfg.PXE.TFTPServer)
	}
	if cfg.PXE.BootfileBIOS != "ipxe.pxe" {
		t.Errorf("PXE bootfile-bios = %q, want ipxe.pxe", cfg.PXE.BootfileBIOS)
	}
	if cfg.PXE.BootfileUEFI != "ipxe.efi" {
		t.Errorf("PXE bootfile-uefi = %q, want ipxe.efi", cfg.PXE.BootfileUEFI)
	}
}

func TestParsePXEConfigBootScriptURL(t *testing.T) {
	t.Parallel()

	data := `{
		"service": {
			"dhcp-server": {
				"enabled": "true",
				"listen-interface": ["eth0"],
				"pxe": {
					"enabled": "true",
					"tftp-server": "192.168.1.1",
					"bootfile-bios": "ipxe.pxe",
					"bootfile-uefi": "ipxe.efi",
					"boot-script-url": "http://192.168.1.1/install/boot/boot.ipxe"
				},
				"shared-network": {
					"LAN": {
						"subnet": {
							"192.168.1.0/24": {
								"range": {
									"start": "192.168.1.100",
									"stop": "192.168.1.200"
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
	if cfg.PXE.BootScriptURL != "http://192.168.1.1/install/boot/boot.ipxe" {
		t.Errorf("PXE boot-script-url = %q, want http://192.168.1.1/install/boot/boot.ipxe", cfg.PXE.BootScriptURL)
	}
}

func TestParsePXEConfigMissing(t *testing.T) {
	t.Parallel()

	data := `{"service": {"dhcp-server": {"enabled": "true"}}}`
	cfg, err := parseConfig(data)
	if err != nil {
		t.Fatalf("parseConfig: %v", err)
	}
	if cfg.PXE.Enabled {
		t.Error("expected PXE disabled when no pxe block")
	}
}

func TestParsePXEConfigInvalid(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		data string
	}{
		{
			name: "invalid tftp-server IP",
			data: `{"service":{"dhcp-server":{"pxe":{"enabled":"true","tftp-server":"not-an-ip"}}}}`,
		},
		{
			name: "IPv6 tftp-server",
			data: `{"service":{"dhcp-server":{"pxe":{"enabled":"true","tftp-server":"::1"}}}}`,
		},
		{
			name: "boot-script-url with ftp scheme",
			data: `{"service":{"dhcp-server":{"pxe":{"enabled":"true","tftp-server":"192.168.1.1","bootfile-bios":"ipxe.pxe","bootfile-uefi":"ipxe.efi","boot-script-url":"ftp://192.168.1.1/boot.ipxe"}}}}`,
		},
		{
			name: "boot-script-url bare path",
			data: `{"service":{"dhcp-server":{"pxe":{"enabled":"true","tftp-server":"192.168.1.1","bootfile-bios":"ipxe.pxe","bootfile-uefi":"ipxe.efi","boot-script-url":"/install/boot/boot.ipxe"}}}}`,
		},
		{
			name: "boot-script-url exceeds DHCP option length",
			data: `{"service":{"dhcp-server":{"pxe":{"enabled":"true","tftp-server":"192.168.1.1","bootfile-bios":"ipxe.pxe","bootfile-uefi":"ipxe.efi","boot-script-url":"http://192.168.1.1/` + strings.Repeat("a", 240) + `"}}}}`,
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

// TestParseConfigRejectsNonIPv4 pins the family guards on every address leaf a
// DHCPv4 subnet carries. The goal is that no IPv6 spelling survives parsing:
// the reply builder narrows each of these to four bytes with netip.Addr.As4,
// which panics on an IPv6 address, and the panic would land on the DISCOVER
// path where a client packet triggers it. The method is one case per leaf,
// asserting the message names the leaf so a guard cannot drift onto the wrong
// field. An IPv4-mapped address is refused with the rest, matching what
// config.IPv4AddressValidator already accepts for every other module.
func TestParseConfigRejectsNonIPv4(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		data    string
		wantErr string
	}{
		{
			name:    "IPv6 subnet prefix",
			data:    `{"service":{"dhcp-server":{"enabled":"true","listen-interface":["br0"],"shared-network":{"L":{"subnet":{"2001:db8::/64":{"range":{"start":"2001:db8::10","stop":"2001:db8::20"}}}}}}}}`,
			wantErr: "subnet prefix",
		},
		{
			name:    "IPv6 default-router",
			data:    `{"service":{"dhcp-server":{"enabled":"true","listen-interface":["br0"],"shared-network":{"L":{"subnet":{"192.168.1.0/24":{"range":{"start":"192.168.1.100","stop":"192.168.1.200"},"default-router":"2001:db8::1"}}}}}}}`,
			wantErr: "default-router",
		},
		{
			name:    "IPv6 dns-server",
			data:    `{"service":{"dhcp-server":{"enabled":"true","listen-interface":["br0"],"shared-network":{"L":{"subnet":{"192.168.1.0/24":{"range":{"start":"192.168.1.100","stop":"192.168.1.200"},"dns-server":["8.8.8.8","2001:4860:4860::8888"]}}}}}}}`,
			wantErr: "dns-server",
		},
		{
			name:    "IPv4-mapped default-router",
			data:    `{"service":{"dhcp-server":{"enabled":"true","listen-interface":["br0"],"shared-network":{"L":{"subnet":{"192.168.1.0/24":{"range":{"start":"192.168.1.100","stop":"192.168.1.200"},"default-router":"::ffff:192.168.1.1"}}}}}}}`,
			wantErr: "default-router",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			_, err := parseConfig(tc.data)
			if err == nil {
				t.Fatal("expected the config to be refused")
			}
			if !strings.Contains(err.Error(), tc.wantErr) {
				t.Errorf("error %q does not name %q", err, tc.wantErr)
			}
		})
	}
}
