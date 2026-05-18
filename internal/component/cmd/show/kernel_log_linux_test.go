//go:build linux

package show

import "testing"

func TestParseKmsgLine(t *testing.T) {
	tests := []struct {
		name      string
		line      string
		wantNil   bool
		wantLevel string
		wantMsg   string
	}{
		{
			"normal",
			"6,1234,5678901,-;Hello from kernel",
			false, "info", "Hello from kernel",
		},
		{
			"err-level",
			"3,100,999999,-;Something bad happened",
			false, "err", "Something bad happened",
		},
		{
			"emerg",
			"0,1,100,-;Panic",
			false, "emerg", "Panic",
		},
		{
			"empty", "", true, "", "",
		},
		{
			"no-semicolon",
			"6,1234,5678901",
			true, "", "",
		},
		{
			"short-prefix",
			"6;message",
			true, "", "",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			entry := parseKmsgLine(tt.line)
			if tt.wantNil {
				if entry != nil {
					t.Errorf("expected nil, got %v", entry)
				}
				return
			}
			if entry == nil {
				t.Fatal("expected non-nil entry")
			}
			if entry["level"] != tt.wantLevel {
				t.Errorf("level = %v, want %v", entry["level"], tt.wantLevel)
			}
			if entry["message"] != tt.wantMsg {
				t.Errorf("message = %v, want %v", entry["message"], tt.wantMsg)
			}
		})
	}
}

func TestShowSystemKernelLog_Wiring(t *testing.T) {
	resp, err := handleShowSystemKernelLog(nil, []string{"count", "5"})
	if err != nil {
		t.Fatal(err)
	}
	if resp == nil {
		t.Fatal("expected non-nil response")
	}
}

func TestKernelLogCountBoundary(t *testing.T) {
	resp, err := handleShowSystemKernelLog(nil, []string{"count", "0"})
	if err != nil {
		t.Fatal(err)
	}
	if resp == nil {
		t.Fatal("expected non-nil response")
	}

	resp, err = handleShowSystemKernelLog(nil, []string{"count", "10001"})
	if err != nil {
		t.Fatal(err)
	}
	if resp == nil {
		t.Fatal("expected non-nil response")
	}
}
