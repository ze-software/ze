// Design: plan/spec-diag-capture-interface.md -- Linux-only tests (BPF filter)

package show

import (
	"testing"
)

func TestCompileBPFFilter(t *testing.T) {
	tests := []struct {
		name    string
		expr    string
		wantErr bool
	}{
		{name: "tcp", expr: "tcp"},
		{name: "tcp port 179", expr: "tcp port 179"},
		{name: "udp port 53", expr: "udp port 53"},
		{name: "icmp", expr: "icmp"},
		{name: "host filter", expr: "host 10.0.0.1"},
		{name: "complex", expr: "tcp port 179 or udp port 53"},
		{name: "invalid expression", expr: "xyzzy foobar", wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			insns, err := compileBPF(tt.expr)
			if tt.wantErr {
				if err == nil {
					t.Fatalf("expected error, got nil (insns: %d)", len(insns))
				}
				return
			}
			if err != nil {
				t.Fatalf("compileBPF(%q) error: %v", tt.expr, err)
			}
			if len(insns) == 0 {
				t.Fatalf("compileBPF(%q) returned 0 instructions", tt.expr)
			}
		})
	}
}
