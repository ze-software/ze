package peeringdb

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
)

// fakeHandler serves PeeringDB-compatible responses using the same
// deterministic formula as ze-test peeringdb: ipv4 = ASN, ipv6 = ASN/5.
func fakeHandler(w http.ResponseWriter, r *http.Request) {
	asn := r.URL.Query().Get("asn")
	if asn == "" {
		http.Error(w, "missing asn", http.StatusBadRequest)
		return
	}

	var asnNum uint32
	if _, err := fmt.Sscanf(asn, "%d", &asnNum); err != nil {
		http.Error(w, "bad asn", http.StatusBadRequest)
		return
	}

	w.Header().Set("Content-Type", "application/json")

	// ASN 0 means "not found in PeeringDB".
	if asnNum == 0 {
		if _, wErr := io.WriteString(w, `{"data":[]}`); wErr != nil {
			return
		}
		return
	}

	// Deterministic AS-SET: ASN 65001 → "AS-TEST", ASN 65002 → "AS-FOO AS-BAR",
	// ASN 65003 → empty (no AS-SET registered).
	var irrASSet string
	switch asnNum {
	case 65001:
		irrASSet = "AS-TEST"
	case 65002:
		irrASSet = "AS-FOO AS-BAR"
	}

	if _, wErr := fmt.Fprintf(w, `{"data":[{"asn":%d,"info_prefixes4":%d,"info_prefixes6":%d,"irr_as_set":"%s"}]}`,
		asnNum, asnNum, asnNum/5, irrASSet); wErr != nil {
		return
	}
}

// VALIDATES: AC-1 -- PeeringDB query returns prefix counts for known ASN.
// PREVENTS: client silently returning zero for valid ASN.
func TestLookupASN(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(fakeHandler))
	defer srv.Close()

	c := NewPeeringDB(srv.URL)

	tests := []struct {
		name    string
		asn     uint32
		wantV4  uint32
		wantV6  uint32
		wantErr bool
	}{
		{
			name:   "typical private ASN",
			asn:    65001,
			wantV4: 65001,
			wantV6: 13000,
		},
		{
			name:   "small ASN",
			asn:    100,
			wantV4: 100,
			wantV6: 20,
		},
		{
			name:   "large ASN",
			asn:    397213,
			wantV4: 397213,
			wantV6: 79442,
		},
		{
			name:    "unknown ASN returns not found",
			asn:     0,
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			counts, err := c.LookupASN(context.Background(), tt.asn)
			if tt.wantErr {
				if err == nil {
					t.Fatal("expected error for unknown ASN, got nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if counts.IPv4 != tt.wantV4 {
				t.Errorf("IPv4: got %d, want %d", counts.IPv4, tt.wantV4)
			}
			if counts.IPv6 != tt.wantV6 {
				t.Errorf("IPv6: got %d, want %d", counts.IPv6, tt.wantV6)
			}
		})
	}
}

// VALIDATES: AC-2 -- unreachable PeeringDB returns error.
// PREVENTS: client panicking or returning zero on network failure.
func TestLookupASNUnreachable(t *testing.T) {
	c := NewPeeringDB("http://127.0.0.1:1") // nothing listening

	_, err := c.LookupASN(context.Background(), 65001)
	if err == nil {
		t.Fatal("expected error for unreachable server, got nil")
	}
}

// VALIDATES: AC-4 -- client returns consistent results for same ASN.
// PREVENTS: non-deterministic responses from fake server.
func TestLookupASNDeterministic(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(fakeHandler))
	defer srv.Close()

	c := NewPeeringDB(srv.URL)

	first, err := c.LookupASN(context.Background(), 65001)
	if err != nil {
		t.Fatalf("first lookup: %v", err)
	}

	second, err := c.LookupASN(context.Background(), 65001)
	if err != nil {
		t.Fatalf("second lookup: %v", err)
	}

	if first != second {
		t.Errorf("non-deterministic: first=%+v, second=%+v", first, second)
	}
}

// VALIDATES: margin computation -- PeeringDB value + configured margin.
// PREVENTS: off-by-one or integer truncation in margin calculation.
func TestApplyMargin(t *testing.T) {
	tests := []struct {
		name   string
		count  uint32
		margin uint8
		want   uint32
	}{
		{"10% of 65001", 65001, 10, 71501},
		{"20% of 1000", 1000, 20, 1200},
		{"0% margin", 5000, 0, 5000},
		{"100% margin", 1000, 100, 2000},
		{"small count", 1, 10, 1},
		{"zero count", 0, 10, 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ApplyMargin(tt.count, tt.margin)
			if got != tt.want {
				t.Errorf("ApplyMargin(%d, %d) = %d, want %d", tt.count, tt.margin, got, tt.want)
			}
		})
	}
}

// VALIDATES: PeeringDB response with zero prefixes is treated as suspicious.
// PREVENTS: setting prefix maximum to 0 which would immediately tear down sessions.
func TestLookupASNZeroPrefixes(t *testing.T) {
	handler := func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if _, wErr := io.WriteString(w,
			`{"data":[{"asn":65999,"info_prefixes4":0,"info_prefixes6":0}]}`); wErr != nil {
			return
		}
	}

	srv := httptest.NewServer(http.HandlerFunc(handler))
	defer srv.Close()

	c := NewPeeringDB(srv.URL)

	counts, err := c.LookupASN(context.Background(), 65999)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if counts.IPv4 != 0 || counts.IPv6 != 0 {
		t.Errorf("expected zero counts, got %+v", counts)
	}
	if !counts.Suspicious() {
		t.Error("zero-prefix response should be marked suspicious")
	}
}

// VALIDATES: ApplyMargin does not overflow for large prefix counts.
// PREVENTS: uint32 wrap-around producing a smaller value than the input.
func TestApplyMarginOverflow(t *testing.T) {
	tests := []struct {
		name   string
		count  uint32
		margin uint8
		want   uint32
	}{
		{"near max with 10%", 4000000000, 10, 4294967295},
		{"max uint32 with 1%", 4294967295, 1, 4294967295},
		{"max uint32 with 0%", 4294967295, 0, 4294967295},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ApplyMargin(tt.count, tt.margin)
			if got != tt.want {
				t.Errorf("ApplyMargin(%d, %d) = %d, want %d", tt.count, tt.margin, got, tt.want)
			}
			if got < tt.count {
				t.Errorf("ApplyMargin(%d, %d) = %d, less than input (overflow)", tt.count, tt.margin, got)
			}
		})
	}
}

// VALIDATES: client handles malformed JSON gracefully.
// PREVENTS: panic on unexpected PeeringDB response format.
func TestLookupASNMalformedJSON(t *testing.T) {
	handler := func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if _, wErr := io.WriteString(w, `{not valid json`); wErr != nil {
			return
		}
	}

	srv := httptest.NewServer(http.HandlerFunc(handler))
	defer srv.Close()

	c := NewPeeringDB(srv.URL)

	_, err := c.LookupASN(context.Background(), 65001)
	if err == nil {
		t.Fatal("expected error for malformed JSON, got nil")
	}
}

// VALIDATES: client handles HTTP error status codes.
// PREVENTS: treating error responses as valid data.
func TestLookupASNHTTPError(t *testing.T) {
	handler := func(w http.ResponseWriter, _ *http.Request) {
		http.Error(w, "internal error", http.StatusInternalServerError)
	}

	srv := httptest.NewServer(http.HandlerFunc(handler))
	defer srv.Close()

	c := NewPeeringDB(srv.URL)

	_, err := c.LookupASN(context.Background(), 65001)
	if err == nil {
		t.Fatal("expected error for HTTP 500, got nil")
	}
}

// VALIDATES: negative values in PeeringDB response are treated as zero.
// PREVENTS: negative counts being cast to large uint32 values.
func TestLookupASNNegativeValues(t *testing.T) {
	handler := func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if _, wErr := io.WriteString(w,
			`{"data":[{"asn":65001,"info_prefixes4":-1,"info_prefixes6":-100}]}`); wErr != nil {
			return
		}
	}

	srv := httptest.NewServer(http.HandlerFunc(handler))
	defer srv.Close()

	c := NewPeeringDB(srv.URL)

	counts, err := c.LookupASN(context.Background(), 65001)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if counts.IPv4 != 0 {
		t.Errorf("IPv4: got %d, want 0 (negative should become 0)", counts.IPv4)
	}
	if counts.IPv6 != 0 {
		t.Errorf("IPv6: got %d, want 0 (negative should become 0)", counts.IPv6)
	}
}

// VALIDATES: context cancellation stops the request.
// PREVENTS: hung requests when caller cancels.
func TestLookupASNContextCancel(t *testing.T) {
	handler := func(w http.ResponseWriter, r *http.Request) {
		// Block until context is canceled.
		<-r.Context().Done()
	}

	srv := httptest.NewServer(http.HandlerFunc(handler))
	defer srv.Close()

	c := NewPeeringDB(srv.URL)

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // cancel immediately

	_, err := c.LookupASN(ctx, 65001)
	if err == nil {
		t.Fatal("expected error for canceled context, got nil")
	}
}

// VALIDATES: missing prefix fields in response return zero, not error.
// PREVENTS: crash when PeeringDB returns a record without prefix fields
// (e.g. IXP entry with no routing data).
func TestLookupASNMissingFields(t *testing.T) {
	handler := func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		// Record exists but has no prefix count fields.
		if _, wErr := io.WriteString(w,
			`{"data":[{"asn":65001,"name":"Test Network"}]}`); wErr != nil {
			return
		}
	}

	srv := httptest.NewServer(http.HandlerFunc(handler))
	defer srv.Close()

	c := NewPeeringDB(srv.URL)

	counts, err := c.LookupASN(context.Background(), 65001)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// Missing fields should be zero, and flagged suspicious.
	if counts.IPv4 != 0 || counts.IPv6 != 0 {
		t.Errorf("expected zero for missing fields, got %+v", counts)
	}
	if !counts.Suspicious() {
		t.Error("missing prefix fields should be flagged suspicious")
	}
}

// VALIDATES: LookupASSet returns single AS-SET for ASN with one registered.
// PREVENTS: AS-SET lookup silently returning empty for valid ASN.
func TestLookupASSet(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(fakeHandler))
	defer srv.Close()

	c := NewPeeringDB(srv.URL)
	sets, err := c.LookupASSet(context.Background(), 65001)
	if err != nil {
		t.Fatalf("LookupASSet: %v", err)
	}

	if len(sets) != 1 || sets[0] != "AS-TEST" {
		t.Errorf("got %v, want [AS-TEST]", sets)
	}
}

// VALIDATES: LookupASSet returns multiple AS-SETs when space-separated.
// PREVENTS: only first AS-SET returned when multiple registered.
func TestLookupASSetMultiple(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(fakeHandler))
	defer srv.Close()

	c := NewPeeringDB(srv.URL)
	sets, err := c.LookupASSet(context.Background(), 65002)
	if err != nil {
		t.Fatalf("LookupASSet: %v", err)
	}

	if len(sets) != 2 || sets[0] != "AS-FOO" || sets[1] != "AS-BAR" {
		t.Errorf("got %v, want [AS-FOO AS-BAR]", sets)
	}
}

// VALIDATES: LookupASSet returns nil when ASN has no AS-SET registered.
// PREVENTS: error on ASN with empty irr_as_set field.
func TestLookupASSetEmpty(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(fakeHandler))
	defer srv.Close()

	c := NewPeeringDB(srv.URL)
	sets, err := c.LookupASSet(context.Background(), 65003)
	if err != nil {
		t.Fatalf("LookupASSet: %v", err)
	}

	if sets != nil {
		t.Errorf("got %v, want nil", sets)
	}
}

// VALIDATES: LookupASSet returns error for unknown ASN.
// PREVENTS: silent empty result for non-existent ASN.
func TestLookupASSetNotFound(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(fakeHandler))
	defer srv.Close()

	c := NewPeeringDB(srv.URL)
	_, err := c.LookupASSet(context.Background(), 0)
	if err == nil {
		t.Fatal("expected error for unknown ASN, got nil")
	}
}

// VALIDATES: parseASSetField handles various separator formats.
// PREVENTS: broken parsing on comma/newline separated AS-SET strings.
func TestParseASSetField(t *testing.T) {
	tests := []struct {
		input string
		want  []string
	}{
		{"AS-FOO", []string{"AS-FOO"}},
		{"AS-FOO AS-BAR", []string{"AS-FOO", "AS-BAR"}},
		{"AS-FOO,AS-BAR", []string{"AS-FOO", "AS-BAR"}},
		{"AS-FOO\nAS-BAR", []string{"AS-FOO", "AS-BAR"}},
		{"RIPE::AS-FOO RADB::AS-BAR", []string{"RIPE::AS-FOO", "RADB::AS-BAR"}},
		{"  AS-FOO  ", []string{"AS-FOO"}},
		{"", nil},
		// Security: names with invalid characters are filtered out.
		{"AS-FOO AS-BAR;DROP", []string{"AS-FOO"}},
		{"AS-OK\tAS-ALSO-OK", []string{"AS-OK", "AS-ALSO-OK"}}, // tab is a valid separator
		{"AS-GOOD AS-B\x00D", []string{"AS-GOOD"}},             // null byte filtered
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := parseASSetField(tt.input)
			if len(got) != len(tt.want) {
				t.Fatalf("got %v, want %v", got, tt.want)
			}
			for i := range got {
				if got[i] != tt.want[i] {
					t.Errorf("got[%d] = %q, want %q", i, got[i], tt.want[i])
				}
			}
		})
	}
}

// VALIDATES: Suspicious returns false when at least one family has data.
// PREVENTS: false positive suspicious flag on ASNs with only IPv4.
func TestSuspiciousPartialData(t *testing.T) {
	p := PrefixCounts{IPv4: 1000, IPv6: 0}
	if p.Suspicious() {
		t.Error("should not be suspicious when IPv4 > 0")
	}

	p = PrefixCounts{IPv4: 0, IPv6: 500}
	if p.Suspicious() {
		t.Error("should not be suspicious when IPv6 > 0")
	}
}
