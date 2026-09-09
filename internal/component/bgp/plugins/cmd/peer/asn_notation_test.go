package peer

import (
	"encoding/json"
	"net/netip"
	"testing"

	"github.com/ze-software/ze/internal/component/plugin"
	"github.com/ze-software/ze/internal/core/bgp/asn"
)

// TestPeerRowsFollowTheConfiguredNotation proves the AS numbers an operator
// reads on `show bgp peer detail` are written in the notation
// bgp/as-notation selected. The method builds the rows twice, once per
// notation, and compares the JSON each answer marshals to.
//
// VALIDATES: handleBgpPeerDetail writes remote-as and local-as as asn.Number.
// PREVENTS: the leaf reaching the RIB rows and not the peer rows, which is the
// operator setting a notation and reading two spellings of one AS number.
func TestPeerRowsFollowTheConfiguredNotation(t *testing.T) {
	restore := asn.Configured()
	t.Cleanup(func() { configureNotation(t, restore) })

	reactor := &mockReactor{
		peers: []plugin.PeerInfo{{
			Address: netip.MustParseAddr("192.0.2.1"),
			PeerAS:  65546,
			LocalAS: 100,
			State:   plugin.PeerStateEstablished,
		}},
	}

	configureNotation(t, asn.NotationPlain)
	plain := marshalPeerDetail(t, reactor)
	if !contains(plain, `"remote-as":65546`) || !contains(plain, `"local-as":100`) {
		t.Fatalf("asplain rows = %s, want the AS numbers as JSON numbers", plain)
	}

	configureNotation(t, asn.NotationDot)
	dotted := marshalPeerDetail(t, reactor)
	if !contains(dotted, `"remote-as":"1.10"`) {
		t.Errorf("asdot rows = %s, want remote-as \"1.10\"", dotted)
	}
	if !contains(dotted, `"local-as":"100"`) {
		t.Errorf("asdot rows = %s, want local-as \"100\": asdot leaves a 2-byte AS number plain", dotted)
	}
	if contains(dotted, "65546") {
		t.Errorf("asdot rows = %s, want no decimal spelling of AS 65546", dotted)
	}
}

// TestCreatedPeerEchoFollowsTheConfiguredNotation proves the AS number
// `create bgp peer` echoes back carries the configured notation. The method
// reads the echo out of a peer tree holding the stored decimal form.
//
// VALIDATES: remoteASOf renders through asn.Text.
// PREVENTS: the one command that ACCEPTS an asdot AS number answering with the
// decimal form, which reads as a refusal of what the operator typed.
func TestCreatedPeerEchoFollowsTheConfiguredNotation(t *testing.T) {
	restore := asn.Configured()
	t.Cleanup(func() { configureNotation(t, restore) })

	// The tree holds the decimal form whatever the operator typed, because
	// setASN normalizes on the way in.
	tree := map[string]any{"session": map[string]any{"asn": map[string]any{"remote": "65546"}}}

	configureNotation(t, asn.NotationPlain)
	if got := remoteASOf(tree); got != "65546" {
		t.Errorf("asplain echo = %q, want %q", got, "65546")
	}

	configureNotation(t, asn.NotationDot)
	if got := remoteASOf(tree); got != "1.10" {
		t.Errorf("asdot echo = %q, want %q", got, "1.10")
	}
}

// marshalPeerDetail runs `show bgp peer detail` and returns its payload JSON.
func marshalPeerDetail(t *testing.T, reactor *mockReactor) string {
	t.Helper()
	resp, err := handleBgpPeerDetail(newTestContext(reactor), nil)
	if err != nil {
		t.Fatalf("handleBgpPeerDetail: %v", err)
	}
	encoded, err := json.Marshal(resp.Data)
	if err != nil {
		t.Fatalf("marshal peer detail: %v", err)
	}
	return string(encoded)
}

// configureNotation records one notation for the rest of a test.
func configureNotation(t *testing.T, notation asn.Notation) {
	t.Helper()
	if err := asn.Configure(notation.String()); err != nil {
		t.Fatalf("asn.Configure(%s): %v", notation, err)
	}
}

// contains reports whether the payload holds the exact text.
func contains(payload, want string) bool {
	return len(payload) >= len(want) && indexOf(payload, want) >= 0
}

func indexOf(haystack, needle string) int {
	for i := 0; i+len(needle) <= len(haystack); i++ {
		if haystack[i:i+len(needle)] == needle {
			return i
		}
	}
	return -1
}
