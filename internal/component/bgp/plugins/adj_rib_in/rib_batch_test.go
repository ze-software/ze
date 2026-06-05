package adj_rib_in

import (
	"strconv"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	bgp "codeberg.org/thomas-mangin/ze/internal/component/bgp"
	"codeberg.org/thomas-mangin/ze/internal/core/family"
	"codeberg.org/thomas-mangin/ze/internal/core/seqmap"
	"codeberg.org/thomas-mangin/ze/internal/core/textbuf"
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
		_, _, _ = r.handleCommand("request adj-rib-in accept-routes", individualArgs[idx], "")
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
		_, _, _ = r.handleCommand("request adj-rib-in batch-validate", batchArgs[batchIdx], "")
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
		ribIn:          make(map[string]*seqmap.Map[string, *RawRoute]),
		peerUp:         make(map[string]bool),
		pending:        make(map[string]*PendingRoute),
		earlyDecisions: make(map[string]*EarlyDecision),
	}
}

func benchPrefixTable() ([]string, []string) {
	prefixes := make([]string, benchN)
	rKeys := make([]string, benchN)
	for i := range benchN {
		var b textbuf.Buffer
		a := i / 256
		c := i % 256
		prefixes[i] = b.Str("10.").Int(int64(a)).Byte('.').Int(int64(c)).Str(".0/24").String()
		rKeys[i] = bgp.RouteKey("ipv4/unicast", prefixes[i], 0)
	}
	return prefixes, rKeys
}

func benchPopulatePending(r *AdjRIBInManager, prefixes, rKeys []string) {
	r.mu.Lock()
	for i := range benchN {
		r.pending[pendingKey("10.0.0.1", rKeys[i])] = &PendingRoute{
			peerAddr: "10.0.0.1", family: family.IPv4Unicast, prefix: prefixes[i],
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

	rKey1 := bgp.RouteKey("ipv4/unicast", "10.0.0.0/24", 0)
	rKey2 := bgp.RouteKey("ipv4/unicast", "10.0.1.0/24", 0)
	rKey3 := bgp.RouteKey("ipv6/unicast", "2001:db8::/32", 0)

	r.mu.Lock()
	r.pending[pendingKey("10.0.0.1", rKey1)] = &PendingRoute{
		peerAddr: "10.0.0.1", family: family.IPv4Unicast, prefix: "10.0.0.0/24",
		routeKey: rKey1, route: &RawRoute{Family: family.IPv4Unicast, AttrHex: "40010100", NHopHex: "0a000001", NLRIHex: "180a0000"},
		receivedAt: time.Now(), state: ValidationPending,
	}
	r.pending[pendingKey("10.0.0.1", rKey2)] = &PendingRoute{
		peerAddr: "10.0.0.1", family: family.IPv4Unicast, prefix: "10.0.1.0/24",
		routeKey: rKey2, route: &RawRoute{Family: family.IPv4Unicast, AttrHex: "40010100", NHopHex: "0a000001", NLRIHex: "180a0001"},
		receivedAt: time.Now(), state: ValidationPending,
	}
	r.pending[pendingKey("10.0.0.1", rKey3)] = &PendingRoute{
		peerAddr: "10.0.0.1", family: family.IPv6Unicast, prefix: "2001:db8::/32",
		routeKey: rKey3, route: &RawRoute{Family: family.IPv6Unicast, AttrHex: "40010100", NHopHex: "20010db8", NLRIHex: "2020010db8"},
		receivedAt: time.Now(), state: ValidationPending,
	}
	r.mu.Unlock()

	args := []string{
		"a", "10.0.0.1", "ipv4/unicast", "10.0.0.0/24", "0", "1",
		"r", "10.0.0.1", "ipv4/unicast", "10.0.1.0/24", "0", "0",
		"a", "10.0.0.1", "ipv6/unicast", "2001:db8::/32", "0", "2",
	}

	status, data, err := r.handleCommand("request adj-rib-in batch-validate", args, "")
	require.NoError(t, err)
	assert.Equal(t, statusDone, status)

	result, ok := data.(map[string]any)
	require.True(t, ok, "data should be map[string]any")
	assert.Equal(t, 2, result["accepted"])
	assert.Equal(t, 1, result["rejected"])

	r.mu.RLock()
	defer r.mu.RUnlock()

	assert.Empty(t, r.pending)

	require.Contains(t, r.ribIn, "10.0.0.1")
	rt1, ok := r.ribIn["10.0.0.1"].Get(rKey1)
	require.True(t, ok, "accepted route 1 should be installed")
	assert.Equal(t, ValidationValid, rt1.ValidationState)

	_, ok = r.ribIn["10.0.0.1"].Get(rKey2)
	assert.False(t, ok, "rejected route should not be installed")

	rt3, ok := r.ribIn["10.0.0.1"].Get(rKey3)
	require.True(t, ok, "accepted route 3 should be installed")
	assert.Equal(t, ValidationNotFound, rt3.ValidationState)
}

// TestBatchValidateOddPeerIdentifiers verifies batch handles peer keys
// containing spaces, quotes, and backslashes without tokenization.
func TestBatchValidateOddPeerIdentifiers(t *testing.T) {
	r := newTestManager(t)
	r.validationEnabled = true
	peer := `peer "odd\name" with spaces`
	rKey := bgp.RouteKey("ipv4/unicast", "203.0.113.0/24", 42)

	r.mu.Lock()
	r.pending[pendingKey(peer, rKey)] = &PendingRoute{
		peerAddr: peer, family: family.IPv4Unicast, prefix: "203.0.113.0/24",
		routeKey: rKey, route: &RawRoute{Family: family.IPv4Unicast, AttrHex: "40010100", NHopHex: "0a000001", NLRIHex: "18cb0071"},
		receivedAt: time.Now(), state: ValidationPending,
	}
	r.mu.Unlock()

	args := []string{"a", peer, "ipv4/unicast", "203.0.113.0/24", "42", "1"}
	status, _, err := r.handleCommand("request adj-rib-in batch-validate", args, "")
	require.NoError(t, err)
	assert.Equal(t, statusDone, status)

	r.mu.RLock()
	defer r.mu.RUnlock()

	assert.Empty(t, r.pending)
	require.Contains(t, r.ribIn, peer)
	rt, ok := r.ribIn[peer].Get(rKey)
	require.True(t, ok)
	assert.Equal(t, ValidationValid, rt.ValidationState)
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

	status, data, err := r.handleCommand("request adj-rib-in batch-validate", args, "")
	require.NoError(t, err)
	assert.Equal(t, statusDone, status)

	result, ok := data.(map[string]any)
	require.True(t, ok, "data should be map[string]any")
	assert.Equal(t, 2, result["early"])

	r.mu.RLock()
	defer r.mu.RUnlock()

	rKey1 := bgp.RouteKey("ipv4/unicast", "10.0.0.0/24", 0)
	ed1, ok := r.earlyDecisions[pendingKey("10.0.0.1", rKey1)]
	require.True(t, ok, "early accept should be stored")
	assert.Equal(t, earlyAccept, ed1.action)
	assert.Equal(t, ValidationValid, ed1.state)

	rKey2 := bgp.RouteKey("ipv4/unicast", "10.0.1.0/24", 0)
	ed2, ok := r.earlyDecisions[pendingKey("10.0.0.1", rKey2)]
	require.True(t, ok, "early reject should be stored")
	assert.Equal(t, earlyReject, ed2.action)
}

// TestBatchValidateEmptyBatch verifies empty batch returns zeros.
func TestBatchValidateEmptyBatch(t *testing.T) {
	r := newTestManager(t)

	status, data, err := r.handleCommand("request adj-rib-in batch-validate", nil, "")
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

	status, _, err := r.handleCommand("request adj-rib-in batch-validate",
		[]string{"a", "10.0.0.1", "ipv4/unicast", "10.0.0.0/24", "0"}, "")
	assert.Equal(t, statusError, status)
	assert.ErrorIs(t, err, errBatchValidateStride)
}

// TestBatchValidateInvalidAction verifies unknown action is rejected.
func TestBatchValidateInvalidAction(t *testing.T) {
	r := newTestManager(t)

	status, _, err := r.handleCommand("request adj-rib-in batch-validate",
		[]string{"x", "10.0.0.1", "ipv4/unicast", "10.0.0.0/24", "0", "1"}, "")
	assert.Equal(t, statusError, status)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "unknown action")
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

	status, _, err := r.handleCommand("request adj-rib-in batch-validate", args, "")
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
			rKey := bgp.RouteKey("ipv4/unicast", p, uint32(i))
			r.pending[pendingKey("10.0.0.1", rKey)] = &PendingRoute{
				peerAddr: "10.0.0.1", family: family.IPv4Unicast, prefix: p,
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
			_, _, _ = individual.handleCommand("request adj-rib-in accept-routes",
				[]string{"10.0.0.1", "ipv4/unicast", p, pathID, states[i]}, "")
		} else {
			_, _, _ = individual.handleCommand("request adj-rib-in reject-routes",
				[]string{"10.0.0.1", "ipv4/unicast", p, pathID}, "")
		}
	}

	batched := setup()
	var batchArgs []string
	for i, p := range prefixes {
		batchArgs = append(batchArgs, actions[i], "10.0.0.1", "ipv4/unicast", p, strconv.Itoa(i), states[i])
	}
	_, _, _ = batched.handleCommand("request adj-rib-in batch-validate", batchArgs, "")

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

		indRoutes.Range(func(key string, _ uint64, indRT *RawRoute) bool {
			batRT, ok := batRoutes.Get(key)
			require.True(t, ok, "batched missing route key %s", key)
			assert.Equal(t, indRT.ValidationState, batRT.ValidationState,
				"validation state mismatch for %s", key)
			return true
		})
	}
}
