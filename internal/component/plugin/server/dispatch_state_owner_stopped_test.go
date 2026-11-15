// Design: docs/architecture/irr-owner.md -- reads go to the daemon store, not to the owner
// Related: dispatch_state.go -- opStateGet, where a consumer's StateGet is answered
// Related: dispatch_state_bound_test.go -- the read grant, on test names
package server

import (
	"encoding/binary"
	"encoding/hex"
	"net/netip"
	"slices"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/resolve/irr/store"
	"github.com/ze-software/ze/internal/core/statestore"
)

// firewallIRRPlugin is the registered name of the firewall-irr consumer, whose
// grants internal/component/firewall/plugins/irr/register.go declares. The
// external test package of this directory links plugin/all
// (all_import_test.go), so that init has run in this test binary.
const firewallIRRPlugin = "firewall-irr"

// TestIRRConsumerReadsDatasetWithoutOwner proves spec assumption A-9
// (plan/release/fixes/spec-fetched-data-survives-restart.md): reading the IRR
// owner's keys needs no owner process. A consumer reads a stored dataset
// through store.ReadGeneration and store.ReadSet, over the SDK's StateGet and
// the daemon dispatcher, with its own read grant, while the owner is stopped
// or was never started, on both transports.
//
// VALIDATES: opStateGet answers from the daemon store and judges the caller by
// its own grants (statestore.PluginKeyReadable), never by asking or finding
// the owner, so a consumer started after the owner stopped, or before it ever
// started, reads the copy the owner left.
// PREVENTS: a read path that needs the owner running (a read routed to the
// owner, or a grant tied to the owner's liveness), which would leave a
// consumer with no list at a restart until the owner comes back and fetches.
//
// Method: the grant guard first requires the real registered grants, the
// owner's write grant and firewall-irr's read-only grant, on every key read.
// Then, per transport: "owner stopped" writes the dataset, the generation and
// the pointer as the owner does (writeDataset's order) through an SDK client
// named irr, and closes that client; "owner never started" writes the same
// values straight into the store, as a previous run left them. Only then is the
// consumer connected, to a dispatcher that holds no owner process at all, and
// its reads must return the generation, the pointer and every field of the
// dataset the owner wrote.
func TestIRRConsumerReadsDatasetWithoutOwner(t *testing.T) {
	asSet, err := store.ASSetName("AS-OWNERLESS")
	require.NoError(t, err)
	at := time.Unix(1_700_000_000, 0)
	ipv4 := []netip.Prefix{netip.MustParsePrefix("192.0.2.0/24")}
	ipv6 := []netip.Prefix{netip.MustParsePrefix("2001:db8::/32")}
	dataset, err := store.NewDataset(store.DatasetInput{
		ASSet: asSet, Server: "whois.radb.net", IPv4: ipv4, IPv6: ipv6, RefreshedAt: at, CheckedAt: at,
	})
	require.NoError(t, err)
	value := make([]byte, dataset.EncodedLen())
	require.Equal(t, len(value), dataset.WriteTo(value, 0))
	digest := store.DatasetDigest(value)
	datasetKey := store.Pointer{Server: dataset.Server(), Digest: hex.EncodeToString(digest[:])}.DatasetKey(asSet)
	pointerKey := store.PointerKey(asSet)
	generationKey := store.GenerationKey()

	// Red: remove the RegisterPluginReadKeys call from
	// firewall/plugins/irr/register.go. The guard fails here, and the read
	// below would be refused.
	for _, key := range []string{datasetKey, pointerKey, generationKey} {
		require.True(t, statestore.PluginKeyWritable(store.OwnerPlugin, key),
			"the irr plugin's write grant on %s is not registered in this binary", key)
		require.True(t, statestore.PluginKeyReadable(firewallIRRPlugin, key),
			"firewall-irr holds no read grant on %s", key)
		require.False(t, statestore.PluginKeyWritable(firewallIRRPlugin, key),
			"firewall-irr may write %s, so its read would not prove a read-only grant", key)
	}

	writers := []struct {
		name  string
		write func(t *testing.T, direct bool)
	}{
		{"owner stopped", func(t *testing.T, direct bool) {
			ctx, cancel := stateRPCContext(t)
			defer cancel()
			owner := stateRPCClientAs(t, direct, store.OwnerPlugin)
			require.NoError(t, owner.StatePut(ctx, datasetKey, value))
			generation, err := owner.StateIncrement(ctx, generationKey)
			require.NoError(t, err)
			pointer, err := store.EncodePointer(store.Pointer{
				Server: dataset.Server(), Generation: generation, Digest: hex.EncodeToString(digest[:]),
			})
			require.NoError(t, err)
			require.NoError(t, owner.StatePut(ctx, pointerKey, pointer))
			require.NoError(t, owner.Close(), "stopping the owner")
		}},
		{"owner never started", func(t *testing.T, _ bool) {
			require.NoError(t, statestore.Write(datasetKey, value))
			require.NoError(t, statestore.Write(generationKey, binary.BigEndian.AppendUint32(nil, 1)))
			pointer, err := store.EncodePointer(store.Pointer{
				Server: dataset.Server(), Generation: 1, Digest: hex.EncodeToString(digest[:]),
			})
			require.NoError(t, err)
			require.NoError(t, statestore.Write(pointerKey, pointer))
		}},
	}
	for _, writer := range writers {
		for _, direct := range []bool{false, true} {
			transport := "socket"
			if direct {
				transport = "direct"
			}
			t.Run(writer.name+"/"+transport, func(t *testing.T) {
				useStateTestStore(t)
				writer.write(t, direct)

				ctx, cancel := stateRPCContext(t)
				defer cancel()
				consumer := stateRPCClientAs(t, direct, firewallIRRPlugin)
				// Red: have opStateGet ask stateInput for stateAccessWrite, so a
				// read needs the write grant only the owner holds.
				generation, err := store.ReadGeneration(ctx, consumer)
				require.NoError(t, err, "the consumer could not read the generation without the owner")
				require.Equal(t, uint32(1), generation)
				stored, err := store.ReadSet(ctx, consumer, asSet)
				require.NoError(t, err, "the consumer could not read the stored dataset without the owner")
				require.Equal(t, uint32(1), stored.Pointer.Generation)
				require.Equal(t, asSet, stored.Dataset.ASSet())
				require.Equal(t, dataset.Server(), stored.Dataset.Server())
				require.True(t, slices.Equal(ipv4, stored.Dataset.IPv4()), "IPv4 %v, want %v", stored.Dataset.IPv4(), ipv4)
				require.True(t, slices.Equal(ipv6, stored.Dataset.IPv6()), "IPv6 %v, want %v", stored.Dataset.IPv6(), ipv6)
				require.True(t, stored.Dataset.CheckedAt().Equal(at), "checked-at %v, want %v", stored.Dataset.CheckedAt(), at)
			})
		}
	}
}
