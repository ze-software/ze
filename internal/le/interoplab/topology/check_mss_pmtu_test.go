package topology

import (
	"errors"
	"strings"
	"testing"
)

// VALIDATES: AC-8 of spec-home-cpe-9, the verdict half. mssPMTUVerdict passes
// a capture whose SYN to the probe port carries 1452 (offered 1460) or 1200
// (offered 1200), and refuses one that carries 1460, one with no MSS option,
// and one with no SYN to that port, which it reports as errNoProbeSYN so the
// checker keeps waiting.
// Method: tcpdump -n -r lines as fixtures, one per case.
// PREVENTS: a checker that passes an unclamped SYN, or reads another
// connection's SYN as the probe's.
func TestMSSPMTUVerdict(t *testing.T) {
	const (
		clamped   = "12:00:00.000001 IP 10.184.0.2.40001 > 10.186.1.5.9001: Flags [S], seq 1, win 64240, options [mss 1452,sackOK,TS val 1 ecr 0,nop,wscale 7], length 0"
		unclamped = "12:00:00.000001 IP 10.184.0.2.40001 > 10.186.1.5.9001: Flags [S], seq 1, win 64240, options [mss 1460,sackOK,TS val 1 ecr 0,nop,wscale 7], length 0"
		kept      = "12:00:01.000001 IP 10.184.0.2.40002 > 10.186.1.5.9002: Flags [S], seq 2, win 64240, options [mss 1200,sackOK,TS val 2 ecr 0,nop,wscale 7], length 0"
		noOption  = "12:00:00.000001 IP 10.184.0.2.40001 > 10.186.1.5.9001: Flags [S], seq 1, win 64240, length 0"
		otherPort = "12:00:00.000001 IP 10.184.0.2.40003 > 10.186.1.5.9003: Flags [S], seq 3, win 64240, options [mss 1452], length 0"
	)
	first := mssProbe{offered: 1460, port: 9001, want: 1452}
	second := mssProbe{offered: 1200, port: 9002, want: 1200}
	tests := []struct {
		name    string
		capture string
		probe   mssProbe
		wantErr string
		missing bool
	}{
		{"1452 passes", clamped, first, "", false},
		{"1200 passes", clamped + "\n" + kept + "\n", second, "", false},
		{"1460 refused", unclamped, first, "arrived with MSS 1460, want 1452", false},
		{"no MSS option refused", noOption, first, "arrived with no MSS option", false},
		{"missing SYN refused", otherPort, first, "no SYN to the probe port", true},
		{"empty capture refused", "", first, "no SYN to the probe port", true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := mssPMTUVerdict(tt.capture, tt.probe)
			if tt.wantErr == "" {
				if err != nil {
					t.Fatalf("expected pass, got %v", err)
				}
				return
			}
			if err == nil {
				t.Fatalf("expected a refusal containing %q, got nil", tt.wantErr)
			}
			if !strings.Contains(err.Error(), tt.wantErr) {
				t.Errorf("error %q does not contain %q", err, tt.wantErr)
			}
			if errors.Is(err, errNoProbeSYN) != tt.missing {
				t.Errorf("errors.Is(err, errNoProbeSYN) = %v, want %v", errors.Is(err, errNoProbeSYN), tt.missing)
			}
		})
	}
}
