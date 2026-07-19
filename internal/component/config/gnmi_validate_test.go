package config

import "testing"

func gnmiCfg(host, token string) GNMIListenConfig {
	return GNMIListenConfig{
		Servers: []ServerEndpoint{{Host: host, Port: "9339"}},
		Token:   token,
	}
}

func TestGNMIListenConfigValidate(t *testing.T) {
	// VALIDATES: gNMI Validate is fail-closed -- a non-loopback listener (the
	// 0.0.0.0:9339 default included) with an empty token is rejected, while
	// loopback-without-token and any-address-with-token are allowed. gNMI's
	// interceptors run only when a token is set and checkAuth allows on an empty
	// token, so a tokenless non-loopback bind is an unauthenticated read+write
	// surface.
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
		{"non-loopback WITH token allowed", gnmiCfg("0.0.0.0", "s3cret-token"), false},
		{"routable WITH token allowed", gnmiCfg("10.0.0.1", "s3cret-token"), false},
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
