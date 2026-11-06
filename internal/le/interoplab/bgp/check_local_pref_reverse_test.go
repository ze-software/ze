// Design: docs/architecture/testing/interop.md -- both directions must reach the declared barrier.
package bgp

import (
	"context"
	"net"
	"net/netip"
	"strings"
	"testing"
	"time"

	"github.com/ze-software/ze/internal/le/interoplab"
)

func TestLocalPrefStripRequiresReverseBarrier(t *testing.T) {
	open, err := speakerOpen(65003, 90, net.ParseIP("172.30.0.5"), nil, false, false)
	if err != nil {
		t.Fatal(err)
	}
	initialOnly := localPrefTestRows(t, [][]byte{open, speakerKeepalive()},
		"172.30.0.5:179", "172.30.0.2:179", "keepalive-after-local-pref-fifo-fence")
	for name, reverse := range map[string]string{
		"missing": "",
		"initial KEEPALIVE is not subsequent barrier": initialOnly,
		"wrong speaker":     strings.ReplaceAll(localPrefTestReverse(t), "ac1e0005", "ac1e0003"),
		"wrong frame count": strings.ReplaceAll(localPrefTestReverse(t), `"frames":4`, `"frames":3`),
	} {
		t.Run(name, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(t.Context(), 250*time.Millisecond)
			defer cancel()
			lab := &localPrefWireLab{wire: localPrefTestCapture(t, 200), reverse: reverse}
			err := checkLocalPrefStrip(ctx, &interoplab.CheckContext{
				Network: interoplab.Network{IPv4: netip.MustParsePrefix("172.30.0.0/24")}, Lab: lab,
			})
			if err == nil {
				t.Fatal("normalized RIB certified wire stripping without a complete reverse barrier")
			}
			if strings.Contains(err.Error(), "LOCAL_PREF appeared") {
				t.Fatalf("semantic leakage was attributed before the opposite capture passed its controls: %v", err)
			}
		})
	}
}
