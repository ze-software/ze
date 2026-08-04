package capture

import (
	"errors"
	"strings"
	"testing"
)

// VALIDATES: R-2 -- replay rejects a capture whose schema version it does not know.
// PREVENTS: silently replaying a stream written by an incompatible format.
func TestReaderRejectsUnknownVersion(t *testing.T) {
	line := `{"format":"ze-capture","version":99,"peer":"192.0.2.1","started":"x","daemon-version":"t","coalesce":false}` + "\n"
	_, _, err := NewReader(strings.NewReader(line))
	if err == nil {
		t.Fatal("expected an error for version 99")
	}
	if !errors.Is(err, ErrUnsupportedVersion) {
		t.Fatalf("err = %v, want ErrUnsupportedVersion", err)
	}
	if !strings.Contains(err.Error(), "99") {
		t.Fatalf("error must name the offending version: %v", err)
	}
}

// VALIDATES: AC-5 -- a truncated or corrupt capture produces a clear error naming
// the offending line, and never a panic.
// PREVENTS: replay crashing or silently accepting a damaged stream.
func TestReaderCorruptInput(t *testing.T) {
	hdr := `{"format":"ze-capture","version":1,"peer":"192.0.2.1","started":"x","daemon-version":"t","coalesce":false}`
	tests := []struct {
		name     string
		input    string
		wantLine string
		wantErr  error
	}{
		{
			name:     "empty file",
			input:    "",
			wantLine: "line 1",
			wantErr:  ErrNoHeader,
		},
		{
			name:     "header is not json",
			input:    "not json\n",
			wantLine: "line 1",
			wantErr:  ErrBadHeader,
		},
		{
			name:     "header is json but not a capture header",
			input:    `{"format":"pcap","version":1}` + "\n",
			wantLine: "line 1",
			wantErr:  ErrBadHeader,
		},
		{
			name:     "event line truncated mid-json",
			input:    hdr + "\n" + `{"seq":1,"ts":"x","type":"mes`,
			wantLine: "line 2",
			wantErr:  ErrBadEvent,
		},
		{
			name:     "event line has unknown type",
			input:    hdr + "\n" + `{"seq":1,"ts":"x","type":"weather"}` + "\n",
			wantLine: "line 2",
			wantErr:  ErrBadEvent,
		},
		{
			name:     "sequence goes backwards",
			input:    hdr + "\n" + `{"seq":2,"ts":"x","type":"session","event":"connect"}` + "\n" + `{"seq":1,"ts":"x","type":"session","event":"connect"}` + "\n",
			wantLine: "line 3",
			wantErr:  ErrSequence,
		},
		{
			name:     "message event with no data",
			input:    hdr + "\n" + `{"seq":1,"ts":"x","type":"message","direction":"recv","msg-type":4,"len":19}` + "\n",
			wantLine: "line 2",
			wantErr:  ErrBadEvent,
		},
		{
			name:     "message event whose len disagrees with its data",
			input:    hdr + "\n" + `{"seq":1,"ts":"x","type":"message","direction":"recv","msg-type":4,"len":99,"data":"AAEC"}` + "\n",
			wantLine: "line 2",
			wantErr:  ErrBadEvent,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r, _, err := NewReader(strings.NewReader(tt.input))
			if err == nil {
				// The header parsed; the failure must come from the event stream.
				for {
					_, err = r.Next()
					if err != nil {
						break
					}
				}
			}
			if !errors.Is(err, tt.wantErr) {
				t.Fatalf("err = %v, want %v", err, tt.wantErr)
			}
			if !strings.Contains(err.Error(), tt.wantLine) {
				t.Fatalf("error must name %q: %v", tt.wantLine, err)
			}
		})
	}
}
