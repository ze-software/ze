package adj_rib_in

import (
	"net/netip"
	"strconv"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/core/family"
	"github.com/ze-software/ze/internal/core/seqmap"
	"github.com/ze-software/ze/internal/core/textbuf"
	"github.com/ze-software/ze/pkg/plugin/rpc"
)

const benchN = 4096

// BenchmarkAcceptRoutesIndividual measures per-decision throughput of the
// single-command accept path (one lock acquisition per call).
// Pending routes are pre-populated before timing starts.
func BenchmarkAcceptRoutesIndividual(b *testing.B) {
	prefixes, rKeys := benchPrefixTable()
	individualArgs := make([][]string, benchN)
	for i := range benchN {
		individualArgs[i] = []string{"10.0.0.1", "ipv4/unicast", prefixes[i], "0", "1"}
	}

	r := benchManager(b)
	r.validationEnabled = true
	benchPopulatePending(r, prefixes, rKeys)

	b.ResetTimer()
	for i := range b.N {
		idx := i % benchN
		if idx == 0 && i > 0 {
			b.StopTimer()
			benchPopulatePending(r, prefixes, rKeys)
			b.StartTimer()
		}
		_, _, _ = r.handleCommand("request bgp adj-rib-in accept-routes", individualArgs[idx], "")
	}
}

// BenchmarkAcceptRoutesBatch measures throughput of the batch-validate path
// (one lock acquisition per batch of 128). Each b.N iteration dispatches one
// batch of 128 decisions; divide ns/op by 128 for per-decision cost.
// Pending routes are pre-populated before timing starts.
func BenchmarkAcceptRoutesBatch(b *testing.B) {
	const batchSize = 128
	prefixes, rKeys := benchPrefixTable()

	batchArgs := make([][]string, 0)
	for i := 0; i < benchN; i += batchSize {
		end := min(i+batchSize, benchN)
		args := make([]string, 0, (end-i)*batchValidateStride)
		for j := i; j < end; j++ {
			args = append(args, "a", "10.0.0.1", "ipv4/unicast", prefixes[j], "0", "1")
		}
		batchArgs = append(batchArgs, args)
	}

	r := benchManager(b)
	r.validationEnabled = true
	benchPopulatePending(r, prefixes, rKeys)

	b.ResetTimer()
	batchIdx := 0
	for range b.N {
		_, _, _ = r.handleCommand("request bgp adj-rib-in batch-validate", batchArgs[batchIdx], "")
		batchIdx++
		if batchIdx >= len(batchArgs) {
			batchIdx = 0
			b.StopTimer()
			benchPopulatePending(r, prefixes, rKeys)
			b.StartTimer()
		}
	}
}

func benchManager(b *testing.B) *AdjRIBInManager {
	b.Helper()
	return &AdjRIBInManager{
		ribIn:          make(map[netip.Addr]*seqmap.Map[compactRouteKey, *RawRoute]),
		peerUp:         make(map[netip.Addr]bool),
		pending:        make(map[compactPendingKey]*PendingRoute),
		earlyDecisions: make(map[compactPendingKey]*EarlyDecision),
	}
}

func benchPrefixTable() ([]string, []compactRouteKey) {
	prefixes := make([]string, benchN)
	rKeys := make([]compactRouteKey, benchN)
	for i := range benchN {
		var b textbuf.Buffer
		a := i / 256
		c := i % 256
		prefixes[i] = b.Str("10.").Int(int64(a)).Byte('.').Int(int64(c)).Str(".0/24").String()
		rKeys[i] = routeKeyFromStrings(family.IPv4Unicast, prefixes[i], 0)
	}
	return prefixes, rKeys
}

func benchPopulatePending(r *AdjRIBInManager, prefixes []string, rKeys []compactRouteKey) {
	r.mu.Lock()
	for i := range benchN {
		r.pending[pendingKey(netip.MustParseAddr("10.0.0.1"), rKeys[i])] = &PendingRoute{
			peerAddr: netip.MustParseAddr("10.0.0.1"), family: family.IPv4Unicast, prefix: prefixes[i],
			routeKey: rKeys[i], route: &RawRoute{Family: family.IPv4Unicast},
			state: ValidationPending,
		}
	}
	r.mu.Unlock()
}

// TestBatchValidateMixedAcceptReject verifies a mixed batch produces
// the same installed, pending, and early-decision state as individual commands.
func TestBatchValidateMixedAcceptReject(t *testing.T) {
	r := newTestManager(t)
	r.validationEnabled = true

	rKey1 := routeKeyFromStrings(family.IPv4Unicast, "10.0.0.0/24", 0)
	rKey2 := routeKeyFromStrings(family.IPv4Unicast, "10.0.1.0/24", 0)
	rKey3 := routeKeyFromStrings(family.IPv6Unicast, "2001:db8::/32", 0)

	r.mu.Lock()
	r.pending[pendingKey(netip.MustParseAddr("10.0.0.1"), rKey1)] = &PendingRoute{
		peerAddr: netip.MustParseAddr("10.0.0.1"), family: family.IPv4Unicast, prefix: "10.0.0.0/24",
		routeKey: rKey1, route: &RawRoute{Family: family.IPv4Unicast, AttrHex: "40010100", NHopHex: "0a000001", NLRIHex: "180a0000"},
		receivedAt: time.Now(), state: ValidationPending,
	}
	r.pending[pendingKey(netip.MustParseAddr("10.0.0.1"), rKey2)] = &PendingRoute{
		peerAddr: netip.MustParseAddr("10.0.0.1"), family: family.IPv4Unicast, prefix: "10.0.1.0/24",
		routeKey: rKey2, route: &RawRoute{Family: family.IPv4Unicast, AttrHex: "40010100", NHopHex: "0a000001", NLRIHex: "180a0001"},
		receivedAt: time.Now(), state: ValidationPending,
	}
	r.pending[pendingKey(netip.MustParseAddr("10.0.0.1"), rKey3)] = &PendingRoute{
		peerAddr: netip.MustParseAddr("10.0.0.1"), family: family.IPv6Unicast, prefix: "2001:db8::/32",
		routeKey: rKey3, route: &RawRoute{Family: family.IPv6Unicast, AttrHex: "40010100", NHopHex: "20010db8", NLRIHex: "2020010db8"},
		receivedAt: time.Now(), state: ValidationPending,
	}
	r.mu.Unlock()

	args := []string{
		"a", "10.0.0.1", "ipv4/unicast", "10.0.0.0/24", "0", "1",
		"r", "10.0.0.1", "ipv4/unicast", "10.0.1.0/24", "0", "0",
		"a", "10.0.0.1", "ipv6/unicast", "2001:db8::/32", "0", "2",
	}

	status, data, err := r.handleCommand("request bgp adj-rib-in batch-validate", args, "")
	require.NoError(t, err)
	assert.Equal(t, statusDone, status)

	result, ok := data.(map[string]any)
	require.True(t, ok, "data should be map[string]any")
	assert.Equal(t, 2, result["accepted"])
	assert.Equal(t, 1, result["rejected"])

	r.mu.RLock()
	defer r.mu.RUnlock()

	assert.Empty(t, r.pending)

	require.Contains(t, r.ribIn, netip.MustParseAddr("10.0.0.1"))
	rt1, ok := r.ribIn[netip.MustParseAddr("10.0.0.1")].Get(rKey1)
	require.True(t, ok, "accepted route 1 should be installed")
	assert.Equal(t, ValidationValid, rt1.ValidationState)

	_, ok = r.ribIn[netip.MustParseAddr("10.0.0.1")].Get(rKey2)
	assert.False(t, ok, "rejected route should not be installed")

	rt3, ok := r.ribIn[netip.MustParseAddr("10.0.0.1")].Get(rKey3)
	require.True(t, ok, "accepted route 3 should be installed")
	assert.Equal(t, ValidationNotFound, rt3.ValidationState)
}

// TestBatchValidateOddPeerIdentifiers verifies that peer keys which are not
// IP addresses (spaces, quotes, backslashes) are rejected at the command
// boundary: peer addresses are parsed once into netip.Addr, and a non-IP
// identifier fails closed with an error naming the offending value, leaving
// the pending map untouched.
func TestBatchValidateOddPeerIdentifiers(t *testing.T) {
	r := newTestManager(t)
	r.validationEnabled = true
	peer := `peer "odd\name" with spaces`
	validPeer := netip.MustParseAddr("10.0.0.9")
	rKey := routeKeyFromStrings(family.IPv4Unicast, "203.0.113.0/24", 42)

	r.mu.Lock()
	r.pending[pendingKey(validPeer, rKey)] = &PendingRoute{
		peerAddr: validPeer, family: family.IPv4Unicast, prefix: "203.0.113.0/24",
		routeKey: rKey, route: &RawRoute{Family: family.IPv4Unicast, AttrHex: "40010100", NHopHex: "0a000001", NLRIHex: "18cb0071"},
		receivedAt: time.Now(), state: ValidationPending,
	}
	r.mu.Unlock()

	args := []string{"a", peer, "ipv4/unicast", "203.0.113.0/24", "42", "1"}
	status, _, err := r.handleCommand("request bgp adj-rib-in batch-validate", args, "")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "invalid peer address")
	assert.Contains(t, err.Error(), strconv.Quote(peer), "error must name the offending value")
	assert.Equal(t, statusError, status)

	r.mu.RLock()
	defer r.mu.RUnlock()

	assert.Len(t, r.pending, 1, "rejected batch must not mutate pending state")
	assert.Empty(t, r.ribIn, "rejected batch must not install routes")
}

// TestBatchValidateEarlyDecisions verifies batch stores early decisions
// when routes are not yet pending.
func TestBatchValidateEarlyDecisions(t *testing.T) {
	r := newTestManager(t)
	r.validationEnabled = true

	args := []string{
		"a", "10.0.0.1", "ipv4/unicast", "10.0.0.0/24", "0", "1",
		"r", "10.0.0.1", "ipv4/unicast", "10.0.1.0/24", "0", "0",
	}

	status, data, err := r.handleCommand("request bgp adj-rib-in batch-validate", args, "")
	require.NoError(t, err)
	assert.Equal(t, statusDone, status)

	result, ok := data.(map[string]any)
	require.True(t, ok, "data should be map[string]any")
	assert.Equal(t, 2, result["early"])

	r.mu.RLock()
	defer r.mu.RUnlock()

	rKey1 := routeKeyFromStrings(family.IPv4Unicast, "10.0.0.0/24", 0)
	ed1, ok := r.earlyDecisions[pendingKey(netip.MustParseAddr("10.0.0.1"), rKey1)]
	require.True(t, ok, "early accept should be stored")
	assert.Equal(t, earlyAccept, ed1.action)
	assert.Equal(t, ValidationValid, ed1.state)

	rKey2 := routeKeyFromStrings(family.IPv4Unicast, "10.0.1.0/24", 0)
	ed2, ok := r.earlyDecisions[pendingKey(netip.MustParseAddr("10.0.0.1"), rKey2)]
	require.True(t, ok, "early reject should be stored")
	assert.Equal(t, earlyReject, ed2.action)
}

// TestBatchValidateEmptyBatch verifies empty batch returns zeros.
func TestBatchValidateEmptyBatch(t *testing.T) {
	r := newTestManager(t)

	status, data, err := r.handleCommand("request bgp adj-rib-in batch-validate", nil, "")
	require.NoError(t, err)
	assert.Equal(t, statusDone, status)

	result, ok := data.(map[string]any)
	require.True(t, ok, "data should be map[string]any")
	assert.Equal(t, 0, result["accepted"])
	assert.Equal(t, 0, result["rejected"])
	assert.Equal(t, 0, result["early"])
}

// TestBatchValidateInvalidStride verifies non-multiple-of-6 args are rejected.
func TestBatchValidateInvalidStride(t *testing.T) {
	r := newTestManager(t)

	status, _, err := r.handleCommand("request bgp adj-rib-in batch-validate",
		[]string{"a", "10.0.0.1", "ipv4/unicast", "10.0.0.0/24", "0"}, "")
	assert.Equal(t, statusError, status)
	assert.ErrorIs(t, err, errBatchValidateStride)
}

// TestBatchValidateInvalidAction verifies unknown action is rejected.
func TestBatchValidateInvalidAction(t *testing.T) {
	r := newTestManager(t)

	status, _, err := r.handleCommand("request bgp adj-rib-in batch-validate",
		[]string{"x", "10.0.0.1", "ipv4/unicast", "10.0.0.0/24", "0", "1"}, "")
	assert.Equal(t, statusError, status)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "unknown action")
}

// TestBatchValidateAtMaxCount verifies a batch of exactly maxBatchValidateCount
// decisions is accepted. The internal rpki sender caps at 128 but external
// plugins may send up to 256 via the command protocol.
func TestBatchValidateAtMaxCount(t *testing.T) {
	r := newTestManager(t)
	r.validationEnabled = true

	n := maxBatchValidateCount
	args := make([]string, 0, n*batchValidateStride)
	for i := range n {
		prefix := "10.0." + strconv.Itoa(i/256) + "." + strconv.Itoa(i%256) + "/32"
		args = append(args, "a", "10.0.0.1", "ipv4/unicast", prefix, "0", "1")
	}

	status, data, err := r.handleCommand("request bgp adj-rib-in batch-validate", args, "")
	require.NoError(t, err)
	assert.Equal(t, statusDone, status)

	result, ok := data.(map[string]any)
	require.True(t, ok)
	assert.Equal(t, n, result["accepted"])
	assert.Equal(t, n, result["early"])
}

// TestBatchValidateExceedsMaxCount verifies batches larger than
// maxBatchValidateCount are rejected before mutating state.
func TestBatchValidateExceedsMaxCount(t *testing.T) {
	r := newTestManager(t)
	r.validationEnabled = true

	n := maxBatchValidateCount + 1
	args := make([]string, 0, n*batchValidateStride)
	for i := range n {
		prefix := "10.0." + strconv.Itoa(i/256) + "." + strconv.Itoa(i%256) + "/32"
		args = append(args, "a", "10.0.0.1", "ipv4/unicast", prefix, "0", "1")
	}

	status, _, err := r.handleCommand("request bgp adj-rib-in batch-validate", args, "")
	assert.Equal(t, statusError, status)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "exceeds maximum")

	r.mu.RLock()
	defer r.mu.RUnlock()
	assert.Empty(t, r.pending, "no state should be mutated on oversized batch")
	assert.Empty(t, r.ribIn, "no routes should be installed on oversized batch")
}

// TestBatchValidateMatchesIndividual verifies batch produces identical state
// to issuing the same decisions individually.
func TestBatchValidateMatchesIndividual(t *testing.T) {
	prefixes := []string{"10.0.0.0/24", "10.0.1.0/24", "10.0.2.0/24", "10.0.3.0/24"}
	actions := []string{"a", "r", "a", "r"}
	states := []string{"1", "0", "2", "0"}

	setup := func() *AdjRIBInManager {
		r := newTestManager(t)
		r.validationEnabled = true
		r.mu.Lock()
		for i, p := range prefixes {
			rKey := routeKeyFromStrings(family.IPv4Unicast, p, uint32(i))
			r.pending[pendingKey(netip.MustParseAddr("10.0.0.1"), rKey)] = &PendingRoute{
				peerAddr: netip.MustParseAddr("10.0.0.1"), family: family.IPv4Unicast, prefix: p,
				routeKey: rKey, route: &RawRoute{Family: family.IPv4Unicast, AttrHex: "40010100", NHopHex: "0a000001"},
				receivedAt: time.Now(), state: ValidationPending,
			}
		}
		r.mu.Unlock()
		return r
	}

	individual := setup()
	for i, p := range prefixes {
		pathID := strconv.Itoa(i)
		if actions[i] == "a" {
			_, _, _ = individual.handleCommand("request bgp adj-rib-in accept-routes",
				[]string{"10.0.0.1", "ipv4/unicast", p, pathID, states[i]}, "")
		} else {
			_, _, _ = individual.handleCommand("request bgp adj-rib-in reject-routes",
				[]string{"10.0.0.1", "ipv4/unicast", p, pathID}, "")
		}
	}

	batched := setup()
	var batchArgs []string
	for i, p := range prefixes {
		batchArgs = append(batchArgs, actions[i], "10.0.0.1", "ipv4/unicast", p, strconv.Itoa(i), states[i])
	}
	_, _, _ = batched.handleCommand("request bgp adj-rib-in batch-validate", batchArgs, "")

	individual.mu.RLock()
	batched.mu.RLock()
	defer individual.mu.RUnlock()
	defer batched.mu.RUnlock()

	assert.Equal(t, len(individual.pending), len(batched.pending), "pending count mismatch")
	assert.Equal(t, len(individual.ribIn), len(batched.ribIn), "ribIn peer count mismatch")

	for peer, indRoutes := range individual.ribIn {
		batRoutes, ok := batched.ribIn[peer]
		require.True(t, ok, "batched missing peer %s", peer)
		assert.Equal(t, indRoutes.Len(), batRoutes.Len(), "route count mismatch for peer %s", peer)

		indRoutes.Range(func(key compactRouteKey, _ uint64, indRT *RawRoute) bool {
			batRT, ok := batRoutes.Get(key)
			require.True(t, ok, "batched missing route key %v", key)
			assert.Equal(t, indRT.ValidationState, batRT.ValidationState,
				"validation state mismatch for %v", key)
			return true
		})
	}
}

// TestBatchValidateTypedMatchesString verifies the typed bridge path produces
// identical state to the string-based batchValidateCommand path.
func TestBatchValidateTypedMatchesString(t *testing.T) {
	prefixes := []string{"10.0.0.0/24", "10.0.1.0/24", "10.0.2.0/24", "10.0.3.0/24"}

	setup := func() *AdjRIBInManager {
		r := newTestManager(t)
		r.validationEnabled = true
		r.mu.Lock()
		for i, p := range prefixes {
			rKey := routeKeyFromStrings(family.IPv4Unicast, p, uint32(i))
			r.pending[pendingKey(netip.MustParseAddr("10.0.0.1"), rKey)] = &PendingRoute{
				peerAddr: netip.MustParseAddr("10.0.0.1"), family: family.IPv4Unicast, prefix: p,
				routeKey: rKey, route: &RawRoute{Family: family.IPv4Unicast, AttrHex: "40010100", NHopHex: "0a000001"},
				receivedAt: time.Now(), state: ValidationPending,
			}
		}
		r.mu.Unlock()
		return r
	}

	stringPath := setup()
	stringArgs := []string{
		"a", "10.0.0.1", "ipv4/unicast", "10.0.0.0/24", "0", "1",
		"r", "10.0.0.1", "ipv4/unicast", "10.0.1.0/24", "1", "0",
		"a", "10.0.0.1", "ipv4/unicast", "10.0.2.0/24", "2", "2",
		"r", "10.0.0.1", "ipv4/unicast", "10.0.3.0/24", "3", "0",
	}
	_, stringData, err := stringPath.handleCommand("request bgp adj-rib-in batch-validate", stringArgs, "")
	require.NoError(t, err)

	typedPath := setup()
	typedDecisions := []rpc.ValidationDecision{
		{Accept: true, PeerAddr: "10.0.0.1", Family: "ipv4/unicast", Prefix: "10.0.0.0/24", PathID: 0, ValState: 1},
		{Accept: false, PeerAddr: "10.0.0.1", Family: "ipv4/unicast", Prefix: "10.0.1.0/24", PathID: 1},
		{Accept: true, PeerAddr: "10.0.0.1", Family: "ipv4/unicast", Prefix: "10.0.2.0/24", PathID: 2, ValState: 2},
		{Accept: false, PeerAddr: "10.0.0.1", Family: "ipv4/unicast", Prefix: "10.0.3.0/24", PathID: 3},
	}
	typedResult, err := typedPath.handleBatchValidateTyped(typedDecisions)
	require.NoError(t, err)

	stringResult, ok := stringData.(map[string]any)
	require.True(t, ok, "string path should return map[string]any")
	assert.Equal(t, stringResult["accepted"], typedResult.Accepted)
	assert.Equal(t, stringResult["rejected"], typedResult.Rejected)
	assert.Equal(t, stringResult["early"], typedResult.Early)

	stringPath.mu.RLock()
	typedPath.mu.RLock()
	defer stringPath.mu.RUnlock()
	defer typedPath.mu.RUnlock()

	assert.Equal(t, len(stringPath.pending), len(typedPath.pending), "pending count mismatch")
	for peer, strRoutes := range stringPath.ribIn {
		typRoutes, ok := typedPath.ribIn[peer]
		require.True(t, ok, "typed path missing peer %s", peer)
		assert.Equal(t, strRoutes.Len(), typRoutes.Len(), "route count mismatch for peer %s", peer)

		strRoutes.Range(func(key compactRouteKey, _ uint64, strRT *RawRoute) bool {
			typRT, ok := typRoutes.Get(key)
			require.True(t, ok, "typed path missing route key %v", key)
			assert.Equal(t, strRT.ValidationState, typRT.ValidationState,
				"validation state mismatch for %v", key)
			return true
		})
	}
}

// TestBatchValidateTypedRejectsInvalidState verifies the typed handler
// rejects accepts with invalid validation state (parity with parseValidationState).
func TestBatchValidateTypedRejectsInvalidState(t *testing.T) {
	r := newTestManager(t)
	r.validationEnabled = true

	decisions := []rpc.ValidationDecision{
		{Accept: true, PeerAddr: "10.0.0.1", Family: "ipv4/unicast", Prefix: "10.0.0.0/24", PathID: 0, ValState: 3},
	}
	_, err := r.handleBatchValidateTyped(decisions)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "invalid validation state")

	r.mu.RLock()
	defer r.mu.RUnlock()
	assert.Empty(t, r.ribIn, "no routes should be installed on invalid state")
}
