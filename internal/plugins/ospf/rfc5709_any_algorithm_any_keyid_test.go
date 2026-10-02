// VALIDATES: RFC 5709 Section 3 through operator configuration: every authentication
// algorithm Ze supports for AuType 2 can be configured under every Key ID, and the engine
// then verifies a packet signed with that Key ID and that algorithm.
// PREVENTS: a config parser, validator or key resolver that ties an algorithm to a Key ID
// (or refuses a supported algorithm at some Key ID), which the packet-level units cannot
// see because they build packet.AuthKey by hand.
package ospf

import (
	"strconv"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/plugins/ospf/packet"
	"github.com/ze-software/ze/internal/plugins/ospf/transport"
)

// rfc5709KeyChainConfig returns an OSPF config whose area key chain binds keyIDs[i] to
// algorithms[(i+shift)%len], every key with the same secret.
func rfc5709KeyChainConfig(keyIDs []uint32, algorithms []string, shift int) string {
	keys := ""
	for i, id := range keyIDs {
		if i > 0 {
			keys += ","
		}
		name := strconv.FormatUint(uint64(id), 10)
		keys += `"` + name + `":{"key-id":"` + name + `","algorithm":"` + algorithms[(i+shift)%len(algorithms)] + `","secret":"shared-key"}`
	}
	return `{"ospf":{"router-id":"10.0.0.1",` +
		`"areas":{"area":{"0":{"area-id":"0","authentication":{"key-chain":"kc1"}}}},` +
		`"interfaces":{"interface":{"eth0":{"area":"0","network-type":"point-to-point","authentication":{"mode":"inherit"}}}},` +
		`"key-chains":{"kc1":{"name":"kc1","key":{` + keys + `}}}}}`
}

// RFC requirement: RFC5709-3-5 positive -- the operator can configure ANY supported AuType 2
// algorithm (md5, hmac-sha-1, hmac-sha-256, hmac-sha-384, hmac-sha-512) under ANY Key ID:
// five configurations rotate the five algorithms over Key IDs 1, 77, 200, 254 and 255, so
// each algorithm sits under each Key ID once; every configuration passes parseOSPFConfig and
// validateConfig, and the engine's receive hook (verifyPacket) accepts a Hello signed with
// each configured (Key ID, algorithm) pair. {single-polarity: positive} is on the row.
//
// Goal: prove the operator path, not the packet codec, permits every pairing. Method: real
// config text through the parser and validator into a running engine, one signed packet per
// pair, with the auth failure counter unchanged.
func TestRFC5709AnyAlgorithmUnderAnyKeyID(t *testing.T) {
	keyIDs := []uint32{1, 77, 200, 254, 255}
	algorithms := []string{packet.AuthMD5, packet.AuthHMACSHA1, packet.AuthHMACSHA256, packet.AuthHMACSHA384, packet.AuthHMACSHA512}
	for shift := range algorithms {
		t.Run("shift-"+strconv.Itoa(shift), func(t *testing.T) {
			cfg, err := parseOSPFConfig(ospfSec(rfc5709KeyChainConfig(keyIDs, algorithms, shift)), nil)
			require.NoError(t, err)
			require.NoError(t, validateConfig(cfg))
			fb := &fakeBackend{}
			eng := newEngine(transport.New(fb))
			defer eng.shutdown()
			rec := &authFailRegistry{}
			eng.setMetrics(rec)
			eng.setConfig(cfg)
			require.NoError(t, eng.openInterfaces())
			fb.mu.Lock()
			handle := fb.handles["eth0"]
			fb.mu.Unlock()
			require.NotNil(t, handle)
			h := Header{RouterID: ridOf("2.2.2.2"), AreaID: cfg.Areas[0].AreaID}
			for i, id := range keyIDs {
				algorithm := algorithms[(i+shift)%len(algorithms)]
				key := packet.AuthKey{KeyID: id, Algorithm: algorithm, Secret: []byte("shared-key")}
				signed := signedHelloWith(t, key, uint64(10+i))
				if !eng.verifyPacket(transport.RawPacket{IfIndex: handle.ifindex, Payload: signed}, h) {
					t.Fatalf("key id %d with %s: configured pair refused by the engine", id, algorithm)
				}
			}
			if rec.authFailures != 0 {
				t.Fatalf("authentication failures = %d, want 0", rec.authFailures)
			}
		})
	}
}
