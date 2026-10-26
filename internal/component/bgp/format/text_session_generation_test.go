// Design: docs/architecture/api/json-format.md -- State Events, session-generation
// Related: text_human.go -- appendStateChangeText
// Related: text_json.go -- appendStateChangeJSON

package format

import (
	"net/netip"
	"strings"
	"testing"

	"github.com/ze-software/ze/internal/component/plugin"
	"github.com/ze-software/ze/pkg/plugin/rpc"
)

// TestStateDownCarriesSessionGeneration proves a down names the session it
// ended in both event encodings, as the decimal a forwarding plugin echoes as
// recovery-generation, and that an event with no generation carries none.
// Method: the real producer, AppendStateChange, over a down with generation
// 18446744073709551615 (past JSON's exact numeric range, so it must be a
// string) and over an up with none.
func TestStateDownCarriesSessionGeneration(t *testing.T) {
	down := plugin.PeerInfo{Address: netip.MustParseAddr("10.0.0.1"), PeerAS: rpc.RemoteASOf(65001),
		SessionGeneration: 18446744073709551615}
	text := string(AppendStateChange(nil, &down, rpc.SessionStateDown, "tcp-failure", nil, plugin.EncodingText))
	if want := "peer 10.0.0.1 remote as 65001 state down reason tcp-failure session-generation 18446744073709551615\n"; text != want {
		t.Fatalf("text down = %q, want %q", text, want)
	}
	json := string(AppendStateChange(nil, &down, rpc.SessionStateDown, "tcp-failure", nil, plugin.EncodingJSON))
	if !strings.Contains(json, `"session-generation":"18446744073709551615"`) {
		t.Fatalf("json down = %s, want session-generation as a decimal string", json)
	}

	up := plugin.PeerInfo{Address: netip.MustParseAddr("10.0.0.1"), PeerAS: rpc.RemoteASOf(65001)}
	for _, encoding := range []string{plugin.EncodingText, plugin.EncodingJSON} {
		got := string(AppendStateChange(nil, &up, rpc.SessionStateUp, "", nil, encoding))
		if strings.Contains(got, "session-generation") {
			t.Fatalf("%s up = %q, want no session-generation on an event that names no ended session", encoding, got)
		}
	}
}
