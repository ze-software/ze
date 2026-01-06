package source

import (
	"net/netip"
	"testing"
)

func TestSourceIDType(t *testing.T) {
	// VALIDATES: SourceID.Type() returns correct type for ID ranges
	// PREVENTS: Wrong type derivation from ID

	tests := []struct {
		id   SourceID
		want SourceType
	}{
		{SourceIDConfig, SourceConfig},
		{SourceIDPeerMin, SourcePeer},
		{SourceIDPeerMax, SourcePeer},
		{SourceID(50000), SourcePeer},
		{SourceID(100000), SourceUnknown}, // Gap between peer and api
		{SourceIDAPIMin, SourceAPI},
		{SourceID(200000), SourceAPI},
		{InvalidSourceID, SourceUnknown},
	}

	for _, tt := range tests {
		got := tt.id.Type()
		if got != tt.want {
			t.Errorf("SourceID(%d).Type() = %v, want %v", tt.id, got, tt.want)
		}
	}
}

func TestSourceIDConvenienceMethods(t *testing.T) {
	// VALIDATES: IsValid, IsPeer, IsAPI, IsConfig return correct values
	// PREVENTS: Wrong type checks

	tests := []struct {
		id       SourceID
		valid    bool
		isPeer   bool
		isAPI    bool
		isConfig bool
	}{
		{InvalidSourceID, false, false, false, false},
		{SourceIDConfig, true, false, false, true},
		{SourceIDPeerMin, true, true, false, false},
		{SourceIDPeerMax, true, true, false, false},
		{SourceID(50000), true, true, false, false},
		{SourceID(100000), true, false, false, false}, // Gap - valid but no type
		{SourceIDAPIMin, true, false, true, false},
		{SourceID(200000), true, false, true, false},
	}

	for _, tt := range tests {
		if got := tt.id.IsValid(); got != tt.valid {
			t.Errorf("SourceID(%d).IsValid() = %v, want %v", tt.id, got, tt.valid)
		}
		if got := tt.id.IsPeer(); got != tt.isPeer {
			t.Errorf("SourceID(%d).IsPeer() = %v, want %v", tt.id, got, tt.isPeer)
		}
		if got := tt.id.IsAPI(); got != tt.isAPI {
			t.Errorf("SourceID(%d).IsAPI() = %v, want %v", tt.id, got, tt.isAPI)
		}
		if got := tt.id.IsConfig(); got != tt.isConfig {
			t.Errorf("SourceID(%d).IsConfig() = %v, want %v", tt.id, got, tt.isConfig)
		}
	}
}

func TestSourceIDString(t *testing.T) {
	// VALIDATES: SourceID.String() returns type:n with 1-based numbering
	// PREVENTS: Unclear ID representation in logs

	tests := []struct {
		id   SourceID
		want string
	}{
		{SourceIDConfig, "config:1"},
		{SourceIDPeerMin, "peer:1"},
		{SourceID(42), "peer:42"},
		{SourceIDAPIMin, "api:1"},
		{SourceID(100002), "api:2"},
		{SourceID(100006), "api:6"},
		{InvalidSourceID, "unknown"},
	}

	for _, tt := range tests {
		got := tt.id.String()
		if got != tt.want {
			t.Errorf("SourceID(%d).String() = %q, want %q", tt.id, got, tt.want)
		}
	}
}

func TestParseSourceID(t *testing.T) {
	// VALIDATES: ParseSourceID correctly converts string to SourceID
	// PREVENTS: Wrong ID from string input

	tests := []struct {
		input string
		want  SourceID
	}{
		{"config:1", SourceIDConfig},
		{"peer:1", SourceIDPeerMin},
		{"peer:42", SourceID(42)},
		{"peer:99999", SourceIDPeerMax},
		{"api:1", SourceIDAPIMin},
		{"api:2", SourceID(100002)},
		{"api:6", SourceID(100006)},
		// Invalid cases
		{"", InvalidSourceID},
		{"peer", InvalidSourceID},
		{":1", InvalidSourceID},
		{"peer:", InvalidSourceID},
		{"config:0", InvalidSourceID},
		{"config:2", InvalidSourceID},
		{"peer:0", InvalidSourceID},
		{"peer:100000", InvalidSourceID}, // exceeds peer max
		{"unknown:1", InvalidSourceID},
		{"foo:1", InvalidSourceID},
		// Overflow cases
		{"peer:9999999999", InvalidSourceID},       // overflow
		{"peer:99999999999999", InvalidSourceID},   // way overflow
		{"api:4294967295", InvalidSourceID},        // MaxUint32
		{"api:99999999999999999", InvalidSourceID}, // huge overflow
		// Negative/invalid chars
		{"peer:-1", InvalidSourceID},
		{"api:-100", InvalidSourceID},
		{"peer:1a", InvalidSourceID},
		{"peer:1 ", InvalidSourceID},
	}

	for _, tt := range tests {
		got := ParseSourceID(tt.input)
		if got != tt.want {
			t.Errorf("ParseSourceID(%q) = %d, want %d", tt.input, got, tt.want)
		}
	}
}

func TestSourceIDRoundTrip(t *testing.T) {
	// VALIDATES: String() and ParseSourceID() are inverses
	// PREVENTS: Data loss in serialization

	ids := []SourceID{
		SourceIDConfig,
		SourceIDPeerMin,
		SourceID(42),
		SourceID(99999),
		SourceIDAPIMin,
		SourceID(100010),
	}

	for _, id := range ids {
		str := id.String()
		parsed := ParseSourceID(str)
		if parsed != id {
			t.Errorf("Round trip failed: %d -> %q -> %d", id, str, parsed)
		}
	}
}

func TestSourceTypeString(t *testing.T) {
	// VALIDATES: SourceType.String() returns human-readable names
	// PREVENTS: Unclear source types in logs/errors

	tests := []struct {
		st   SourceType
		want string
	}{
		{SourceUnknown, "unknown"},
		{SourcePeer, "peer"},
		{SourceAPI, "api"},
		{SourceConfig, "config"},
	}

	for _, tt := range tests {
		got := tt.st.String()
		if got != tt.want {
			t.Errorf("SourceType(%d).String() = %q, want %q", tt.st, got, tt.want)
		}
	}
}

func TestSourceString(t *testing.T) {
	// VALIDATES: Source.String() formats correctly per type
	// PREVENTS: Incorrect source identification in JSON/text output

	tests := []struct {
		name   string
		source Source
		want   string
	}{
		{
			name: "peer source",
			source: Source{
				ID:     SourceIDPeerMin,
				PeerIP: netip.MustParseAddr("10.0.0.1"),
				PeerAS: 65001,
			},
			want: "peer:10.0.0.1",
		},
		{
			name: "api source",
			source: Source{
				ID:   SourceIDAPIMin,
				Name: "rr-plugin",
			},
			want: "api:rr-plugin",
		},
		{
			name: "config source",
			source: Source{
				ID: SourceIDConfig,
			},
			want: "config:1",
		},
		{
			name:   "unknown source",
			source: Source{ID: InvalidSourceID},
			want:   "unknown",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.source.String()
			if got != tt.want {
				t.Errorf("Source.String() = %q, want %q", got, tt.want)
			}
		})
	}
}
