package show

import "testing"

func TestTCPCheck_Wiring(t *testing.T) {
	resp, err := handleTCPCheck(nil, []string{"127.0.0.1", "1"})
	if err != nil {
		t.Fatal(err)
	}
	data, ok := resp.Data.(map[string]any)
	if !ok {
		t.Fatal("expected map response")
	}
	result, ok := data["result"].(string)
	if !ok {
		t.Fatal("missing result field")
	}
	if result != tcpCheckResultRefused && result != tcpCheckResultConnected && result != tcpCheckResultTimeout {
		t.Errorf("unexpected result: %q", result)
	}
}

func TestParseTCPCheckArgs(t *testing.T) {
	tests := []struct {
		name    string
		args    []string
		wantErr bool
	}{
		{"valid", []string{"10.0.0.1", "179"}, false},
		{"with-source", []string{"10.0.0.1", "179", "source", "10.0.0.2"}, false},
		{"with-timeout", []string{"10.0.0.1", "179", "timeout", "3s"}, false},
		{"missing-host", []string{}, true},
		{"missing-port", []string{"10.0.0.1"}, true},
		{"port-zero", []string{"10.0.0.1", "0"}, true},
		{"port-too-high", []string{"10.0.0.1", "65536"}, true},
		{"timeout-too-high", []string{"10.0.0.1", "80", "timeout", "60s"}, true},
		{"timeout-too-low", []string{"10.0.0.1", "80", "timeout", "100ms"}, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, _, _, _, err := parseTCPCheckArgs(tt.args)
			if (err != nil) != tt.wantErr {
				t.Errorf("parseTCPCheckArgs(%v) error = %v, wantErr %v", tt.args, err, tt.wantErr)
			}
		})
	}
}
