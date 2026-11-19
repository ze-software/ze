// Design: docs/architecture/plugin/rib-storage-design.md -- original source policy terminal admission.
package rs

import (
	"context"
	"github.com/stretchr/testify/require"
	"github.com/ze-software/ze/pkg/plugin/rpc"
	"strings"
	"sync"
	"testing"
	"time"
)

// An Apply already admitted before DOWN must join its real worker barrier,
// then refuse its deleted original transaction instead of publishing success
// through a pointer captured before the slow join. The immediate DOWN refusal
// remains separately covered by TestRFC8212SourcePolicyRSDownDropsTransaction.
func TestRSSourcePolicyHeldApplyCannotPublishAfterOriginalDown(t *testing.T) {
	rs, _ := newPolicyRouteServer(t)
	require.NoError(t, policyPhase(t, rs, rpc.SourcePolicyPrepare, 0))
	rs.workers.Stop()
	key := rsFixtureKey(policySource, 1, 101)
	entered, release := make(chan struct{}), make(chan struct{})
	var once sync.Once
	unblock := func() { once.Do(func() { close(release) }) }
	rs.workers = newWorkerPool(func(key workerKey, item workItem) {
		if item.barrier != nil {
			rs.processForward(key, item)
			return
		}
		close(entered)
		<-release
	}, poolConfig{chanSize: 4, idleTimeout: time.Hour})
	ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
	joined := make(chan struct{})
	started := false
	t.Cleanup(func() {
		unblock()
		cancel()
		rs.workers.Stop()
		if started {
			<-joined
		}
	})
	require.True(t, rs.workers.Dispatch(key, workItem{sourcePeer: policySource}))
	select {
	case <-entered:
	case <-ctx.Done():
		t.Fatal(ctx.Err())
	}
	input := rpc.SourcePolicyInput{TransactionID: "tx1", Operation: rpc.SourcePolicyApply, Source: rpc.SourcePolicyIdentity{Peer: policySource, PeerOwner: 1, SessionGeneration: 3, PolicyEpoch: 5, ReceiveFrontier: 9, TemplateGeneration: 1}, NextPolicyEpoch: 6, DeadlineMS: time.Now().Add(5 * time.Second).UnixMilli()}
	result := make(chan error, 1)
	started = true
	go func() { defer close(joined); result <- rs.handleSourcePolicy(ctx, input) }()
	require.Eventually(t, func() bool {
		rs.workers.mu.Lock()
		defer rs.workers.mu.Unlock()
		worker := rs.workers.workers[key]
		return worker != nil && len(worker.ch) != 0
	}, time.Second, time.Millisecond, "Apply never admitted its actual original worker barrier")
	rs.handleStateDown(key, 3)
	unblock()
	select {
	case err := <-result:
		if err == nil || !strings.Contains(err.Error(), "bgp-rs: source "+policySource+" has no prepared source-policy transaction") {
			t.Fatalf("held Apply published or changed its original DOWN refusal: %v", err)
		}
	case <-ctx.Done():
		t.Fatal(ctx.Err())
	}
	rs.withdrawalMu.Lock()
	transaction := rs.sourcePolicy[1]
	rs.withdrawalMu.Unlock()
	if transaction != nil {
		t.Fatal("ended original transaction was reinstalled after the joined Apply")
	}
}
