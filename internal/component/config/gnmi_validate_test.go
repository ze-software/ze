package config

import (
	"strings"
	"testing"
)

func gnmiCfg(host, token string) GNMIListenConfig {
	return GNMIListenConfig{
		Servers: []ServerEndpoint{{Host: host, Port: "9339"}},
		Token:   token,
	}
}

// gnmiCfgTLS is gnmiCfg with both TLS files named, the only form a
// non-loopback gNMI listener boots with.
func gnmiCfgTLS(host, token string) GNMIListenConfig {
	cfg := gnmiCfg(host, token)
	cfg.TLS.Cert = "/etc/ze/gnmi.crt"
	cfg.TLS.Key = "/etc/ze/gnmi.key"
	return cfg
}

func TestGNMIListenConfigValidate(t *testing.T) {
	// VALIDATES: gNMI Validate is fail-closed -- a non-loopback listener (the
	// 0.0.0.0:9339 default included) with an empty token is rejected, while
	// loopback-without-token and any-address-with-token-and-TLS are allowed.
	// gNMI's interceptors run only when a token is set, so a tokenless
	// non-loopback bind is an unauthenticated read surface.
	// PREVENTS: `ze config validate` / `ze doctor` / boot passing an exposed
	// gNMI config silently.
	tests := []struct {
		name    string
		cfg     GNMIListenConfig
		wantErr bool
	}{
		{"default 0.0.0.0 no token rejected", gnmiCfg("0.0.0.0", ""), true},
		{"routable no token rejected", gnmiCfg("10.0.0.1", ""), true},
		{"hostname no token rejected (unparseable=non-loopback)", gnmiCfg("localhost", ""), true},
		{"loopback v4 no token allowed", gnmiCfg("127.0.0.1", ""), false},
		{"loopback v4 high no token allowed", gnmiCfg("127.255.255.254", ""), false},
		{"loopback v6 no token allowed", gnmiCfg("::1", ""), false},
		{"non-loopback WITH token and TLS allowed", gnmiCfgTLS("0.0.0.0", "s3cret-token"), false},
		{"routable WITH token and TLS allowed", gnmiCfgTLS("10.0.0.1", "s3cret-token"), false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.cfg.Validate()
			if tt.wantErr && err == nil {
				t.Fatalf("expected error, got nil")
			}
			if !tt.wantErr && err != nil {
				t.Fatalf("expected no error, got %v", err)
			}
		})
	}
}

func TestGNMIListenConfigValidateMultiListener(t *testing.T) {
	// One non-loopback entry among loopback entries taints the whole config.
	cfg := GNMIListenConfig{
		Servers: []ServerEndpoint{
			{Host: "127.0.0.1", Port: "9339"},
			{Host: "0.0.0.0", Port: "9340"},
		},
	}
	if err := cfg.Validate(); err == nil {
		t.Fatal("expected error when any listener is non-loopback without a token")
	}
}

// TestGNMIListenConfigValidateRequiresTLS: a non-loopback gNMI listener with a
// token but no TLS (or half a TLS pair) is refused, with the same problem, why
// and fix the daemon's boot refusal prints; loopback with a token and no TLS
// passes.
//
// VALIDATES: `ze config validate` and `ze doctor` apply the same rule the
// daemon's start-time listener filter applies (cmd/ze/hub filterMgmtListener
// over the same GNMIExposure rule).
// PREVENTS: a config that validates clean and then leaves gNMI down at boot
// because gNMI would send its token and config in clear text.
func TestGNMIListenConfigValidateRequiresTLS(t *testing.T) {
	certOnly := gnmiCfg("10.0.0.1", "s3cret-token")
	certOnly.TLS.Cert = "/etc/ze/gnmi.crt"
	keyOnly := gnmiCfg("10.0.0.1", "s3cret-token")
	keyOnly.TLS.Key = "/etc/ze/gnmi.key"
	refused := []struct {
		name string
		cfg  GNMIListenConfig
	}{
		{"token without TLS", gnmiCfg("0.0.0.0", "s3cret-token")},
		{"token with cert only", certOnly},
		{"token with key only", keyOnly},
	}
	for _, tt := range refused {
		t.Run(tt.name, func(t *testing.T) {
			refusals := tt.cfg.Refusals()
			if len(refusals) != 1 {
				t.Fatalf("refusals = %d, want exactly the TLS refusal: %v", len(refusals), tt.cfg.Validate())
			}
			r := refusals[0]
			if want := "without TLS"; !strings.Contains(r.Problem(), want) {
				t.Errorf("problem = %q, want it to contain %q", r.Problem(), want)
			}
			if want := "requires TLS"; !strings.Contains(r.Why(), want) {
				t.Errorf("why = %q, want it to contain %q", r.Why(), want)
			}
			if r.Fix() != GNMITLSFix {
				t.Errorf("fix = %q, want %q", r.Fix(), GNMITLSFix)
			}
			if tt.cfg.Validate() == nil {
				t.Fatal("Validate returned nil for a refused config")
			}
		})
	}

	if err := gnmiCfg("127.0.0.1", "s3cret-token").Validate(); err != nil {
		t.Fatalf("loopback with a token and no TLS must pass, got %v", err)
	}
}

// TestServerEndpointListenBracketsIPv6: Listen joins an IPv6 host in the form
// net.Listen and net.SplitHostPort read, and a non-loopback IPv6 gNMI refusal
// prints it bracketed.
//
// VALIDATES: "::1" + "9339" is "[::1]:9339", a zoned host keeps its zone inside
// the brackets, an already bracketed host is not bracketed twice, IPv4 is
// unchanged.
// PREVENTS: "::1:9339", which every reader took for a non-loopback address.
func TestServerEndpointListenBracketsIPv6(t *testing.T) {
	cases := []struct{ host, want string }{
		{"::1", "[::1]:9339"},
		{"fe80::1%eth0", "[fe80::1%eth0]:9339"},
		{"[::1]", "[::1]:9339"},
		{"127.0.0.1", "127.0.0.1:9339"},
		{"0.0.0.0", "0.0.0.0:9339"},
	}
	for _, tt := range cases {
		if got := (ServerEndpoint{Host: tt.host, Port: "9339"}).Listen(); got != tt.want {
			t.Errorf("Listen(%q) = %q, want %q", tt.host, got, tt.want)
		}
	}

	refusals := gnmiCfg("2001:db8::1", "").Refusals()
	if len(refusals) == 0 {
		t.Fatal("a routable IPv6 gNMI listener without a token must be refused")
	}
	if want := `"[2001:db8::1]:9339"`; !strings.Contains(refusals[0].Problem(), want) {
		t.Errorf("problem = %q, want the bracketed address %s", refusals[0].Problem(), want)
	}
}

// TestGNMIListenConfigValidateReportsEveryBreach: a non-loopback listener with
// neither a token nor TLS gets both refusals, so the operator fixes both at
// once instead of meeting the second at the next boot.
func TestGNMIListenConfigValidateReportsEveryBreach(t *testing.T) {
	refusals := gnmiCfg("0.0.0.0", "").Refusals()
	if len(refusals) != 2 {
		t.Fatalf("refusals = %d, want 2 (authentication and TLS)", len(refusals))
	}
	if refusals[0].Fix() != GNMIAuthFix {
		t.Errorf("first fix = %q, want %q", refusals[0].Fix(), GNMIAuthFix)
	}
	if refusals[1].Fix() != GNMITLSFix {
		t.Errorf("second fix = %q, want %q", refusals[1].Fix(), GNMITLSFix)
	}
}
