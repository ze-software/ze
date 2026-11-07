package topology

import (
	"strings"
	"testing"
)

// TestUntaggedFramesCountsOnlyUntaggedDiscovery proves the observer's verdict
// separates the access link's own tagged discovery from a leak: a `tcpdump -e`
// line carrying the 802.1Q header is the link's frame and is not counted, a
// line without one is a leak and is answered, and an empty reading answers
// nothing. Method: the two line shapes tcpdump 4.99 prints with -e.
func TestUntaggedFramesCountsOnlyUntaggedDiscovery(t *testing.T) {
	tagged := "21:53:17.108814 02:42:ac:1f:b5:03 > ff:ff:ff:ff:ff:ff, ethertype 802.1Q (0x8100), length 40: " +
		"vlan 183, p 0, ethertype PPPoE D (0x8863), PPPoE PADI [Service-Name \"internet\"]"
	untagged := "21:53:18.000001 02:42:ac:1f:b5:03 > ff:ff:ff:ff:ff:ff, ethertype PPPoE D (0x8863), length 36: " +
		"PPPoE PADI [Service-Name \"internet\"]"
	if got := untaggedFrames(tagged + "\n"); got != "" {
		t.Errorf("the link's own tagged PADI counted as a leak: %q", got)
	}
	if got := untaggedFrames(""); got != "" {
		t.Errorf("an empty capture answered %q, want nothing", got)
	}
	got := untaggedFrames(tagged + "\n" + untagged + "\n")
	if got != untagged+"\n" {
		t.Errorf("untaggedFrames = %q, want only the untagged PADI", got)
	}
	if strings.Contains(got, "802.1Q") {
		t.Errorf("a tagged frame leaked into the answer: %q", got)
	}
}
