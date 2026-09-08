package reactor

import (
	"encoding/binary"
	"net/netip"
	"testing"

	"github.com/ze-software/ze/internal/component/plugin"
	"github.com/ze-software/ze/internal/core/selector"

	"github.com/stretchr/testify/require"
)

func zzCountMessages(wire []byte) int {
	count := 0
	for len(wire) >= 19 {
		l := int(binary.BigEndian.Uint16(wire[16:18]))
		if l == 0 || l > len(wire) {
			break
		}
		count++
		wire = wire[l:]
	}
	return count
}

// Throwaway probe: the entry point, with groups on and off, small and large.
func TestZZProbeExtended(t *testing.T) {
	for _, groups := range []bool{true, false} {
		for _, large := range []bool{true, false} {
			dest := announceFactPeer{addr: "10.0.0.2", peerAS: announceFactPeerAS, extended: true}
			peer, conn := newAnnounceFactPeer(t, dest)
			r := &Reactor{
				config:          &Config{LocalAS: announceFactGlobalAS},
				peers:           map[netip.AddrPort]*Peer{peer.Settings().PeerKey(): peer},
				attrModHandlers: attrModHandlersWithDefaults(),
			}
			if groups {
				r.updateGroups = newUpdateGroupIndex(true)
			}
			batch := announceFactBatch()
			if large {
				batch = announceFactLargeBatch()
			}
			adapter := &reactorAPIAdapter{r: r}
			require.NoError(t, adapter.AnnounceNLRIBatch(selector.All(), batch, plugin.OperatorSender()))
			t.Logf("groups=%v large=%v nlris=%d bytes=%d messages=%d",
				groups, large, len(batch.NLRIs), len(conn.written()), zzCountMessages(conn.written()))
		}
	}
}
