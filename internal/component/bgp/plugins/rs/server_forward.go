// Design: docs/architecture/plugin/rib-storage-design.md -- original RS output snapshots.
// Related: worker.go -- one source lane owns each batch through final flush.
package rs

import (
	"context"
	"errors"
	"slices"
	"strings"

	"github.com/ze-software/ze/internal/core/family"
	"github.com/ze-software/ze/internal/core/textbuf"
	"github.com/ze-software/ze/pkg/plugin/rpc"
)

type forwardBatch struct {
	ids       []uint64
	targetBuf []rpc.ForwardTarget
	targets   []rpc.ForwardTarget
	processed uint64
	parked    []parkedValidation
}

// Selection captures the complete original target tuple while mu is held.
// A named sibling sharing the source address remains a separate recipient.
func (rs *routeServer) selectForwardTargets(buf []rpc.ForwardTarget, source workerKey, msgID uint64, families map[family.Family]bool) []rpc.ForwardTarget {
	buf = buf[:0]
	if rs.peers[source].nonRSClient() {
		return buf
	}
	for key, peer := range rs.peers {
		if key.owner == source.owner {
			continue
		}
		if !peer.updateTarget() {
			continue
		}
		if msgID != 0 {
			if msgID <= peer.ForwardFrom {
				continue
			}
		}
		if peer.Families != nil {
			any := false
			for fam := range families {
				if peer.SupportsFamily(fam) {
					any = true
					break
				}
			}
			if !any {
				continue
			}
		}
		target, err := targetOf(key, peer)
		if err != nil {
			rs.warnLifecycle("cached forward target refused", "code", lifecycleDoctorCode, "fix", lifecycleDoctorFix, "error", err, "peer-owner", key.owner)
			continue
		}
		buf = append(buf, target)
	}
	slices.SortFunc(buf, compareForwardTarget)
	return buf
}

func compareForwardTarget(a, b rpc.ForwardTarget) int {
	if order := strings.Compare(a.Destination, b.Destination); order != 0 {
		return order
	}
	if a.DestinationOwner < b.DestinationOwner {
		return -1
	}
	if a.DestinationOwner > b.DestinationOwner {
		return 1
	}
	if a.DestinationSession < b.DestinationSession {
		return -1
	}
	if a.DestinationSession > b.DestinationSession {
		return 1
	}
	return 0
}

func (rs *routeServer) explainNoTarget(source workerKey, msgID uint64, families map[family.Family]bool) string {
	var message textbuf.Buffer
	for key, peer := range rs.peers {
		message.Str(key.address.String()).Str(" owner=").Uint(key.owner).Str(" session=").Uint(peer.DestinationSession).Str(" up=")
		if peer.Up {
			message.Str("true")
		} else {
			message.Str("false")
		}
		message.Byte(' ')
	}
	if message.Len() == 0 {
		return "no original peer actors recorded"
	}
	return message.String()
}

func (rs *routeServer) batchForwardUpdate(key workerKey, sourcePeer string, msgID uint64, families map[family.Family]bool) {
	batch := rs.workerBatch(key)
	rs.mu.RLock()
	batch.targetBuf = rs.selectForwardTargets(batch.targetBuf, key, msgID, families)
	var reason string
	if len(batch.targetBuf) == 0 {
		reason = rs.explainNoTarget(key, msgID, families)
	}
	rs.mu.RUnlock()
	if len(batch.targetBuf) == 0 {
		logger().Warn("forward matched no target", "source-peer", sourcePeer, "peer-owner", key.owner, "message-id", msgID, "why", reason)
	}
	rs.accumulateForward(batch, msgID, batch.targetBuf)
}

// A skip receipt already names the exact original UP. Its endpoint is display
// data; only owner plus destination session selects the actor under mu.
// This engine handback still needs its definitive response even if the
// operator did not grant this process ordinary UPDATE output to that peer.
func (rs *routeServer) fastPathOwedTarget(target rpc.ForwardTarget, source workerKey, msgID uint64, families map[family.Family]bool) bool {
	if target.DestinationOwner == source.owner {
		return false
	}
	address, err := target.Address()
	if err != nil {
		return false
	}
	for key, peer := range rs.peers {
		if key.owner != target.DestinationOwner {
			continue
		}
		if peer.DestinationSession != target.DestinationSession {
			continue
		}
		if key.address != address {
			return false
		}
		if !peer.Up {
			return false
		}
		if !peer.RSClient {
			return false
		}
		if msgID != 0 {
			if msgID <= peer.ForwardFrom {
				return false
			}
		}
		if peer.Families == nil {
			return true
		}
		for fam := range families {
			if peer.SupportsFamily(fam) {
				return true
			}
		}
		return false
	}
	return false
}

func (rs *routeServer) batchForwardUpdateSkipped(key workerKey, sourcePeer string, msgID uint64, families map[family.Family]bool, skipped []rpc.ForwardTarget) {
	batch := rs.workerBatch(key)
	rs.mu.RLock()
	if rs.peers[key].nonRSClient() {
		batch.targetBuf = batch.targetBuf[:0]
		rs.mu.RUnlock()
		rs.accumulateForward(batch, msgID, nil)
		return
	}
	targets := batch.targetBuf[:0]
	for _, target := range skipped {
		if rs.fastPathOwedTarget(target, key, msgID, families) {
			if !slices.Contains(targets, target) {
				targets = append(targets, target)
			}
		}
	}
	for targetKey, peer := range rs.peers {
		if !peer.updateTarget() {
			continue
		}
		if msgID >= peer.InitialReplay {
			continue
		}
		target, err := targetOf(targetKey, peer)
		if err != nil {
			continue
		}
		if slices.Contains(targets, target) {
			continue
		}
		if rs.fastPathOwedTarget(target, key, msgID, families) {
			targets = append(targets, target)
		}
	}
	batch.targetBuf = targets
	rs.mu.RUnlock()
	slices.SortFunc(targets, compareForwardTarget)
	rs.accumulateForward(batch, msgID, targets)
}

func (rs *routeServer) accumulateForward(batch *forwardBatch, msgID uint64, targets []rpc.ForwardTarget) {
	if len(targets) == 0 {
		rs.releaseCache(msgID)
		return
	}
	if !slices.Equal(batch.targets, targets) {
		rs.flushBatch(batch)
		batch.ids = batch.ids[:0]
		batch.targets = append(batch.targets[:0], targets...)
	}
	batch.ids = append(batch.ids, msgID)
	if len(batch.ids) >= maxBatchSize {
		rs.flushBatch(batch)
		batch.ids = batch.ids[:0]
		batch.targets = batch.targets[:0]
	}
}

// ONE request carries all captured recipients. The engine performs one cache
// acknowledgement after their work joins; repeating IDs per target is invalid.
func (rs *routeServer) flushBatch(batch *forwardBatch) {
	if len(batch.ids) == 0 {
		return
	}
	ctx, cancel := context.WithTimeout(rs.replayParent(), forwardCachedTimeout)
	defer cancel()
	done := make(chan struct{})
	work, admitted := rs.outputs.begin(ctx, done)
	if !admitted {
		rs.rememberUnknownOutput(batch.targets, errors.New("actual cached output was not admitted before process stop"))
		return
	}
	rs.outputMu.Lock()
	if rs.outputTargets == nil {
		rs.outputTargets = make(map[chan struct{}][]rpc.ForwardTarget)
	}
	rs.outputTargets[done] = slices.Clone(batch.targets)
	rs.outputMu.Unlock()
	defer func() {
		rs.outputMu.Lock()
		delete(rs.outputTargets, done)
		rs.outputMu.Unlock()
		rs.outputs.finish(done)
	}()
	if rs.forwardCachedHook != nil {
		addresses := make([]string, 0, len(batch.targets))
		for _, target := range batch.targets {
			addresses = append(addresses, target.Destination)
		}
		rs.forwardCachedHook(batch.ids, addresses)
		return
	}
	if err := rs.plugin.ForwardCached(work, rpc.ForwardCachedInput{IDs: batch.ids, Targets: batch.targets}); err != nil {
		rs.rememberUnknownOutput(batch.targets, err)
		rs.logEngineCallFailure("captured cached forward failed", err, "ids", len(batch.ids))
	}
}

func (rs *routeServer) flushWorkerBatch(key workerKey) {
	value, present := rs.batches.Load(key)
	if !present {
		return
	}
	batch, ok := value.(*forwardBatch)
	if !ok {
		return
	}
	rs.flushBatch(batch)
	batch.ids = batch.ids[:0]
	batch.targets = batch.targets[:0]
	if err := rs.releaseEmptySource(rs.replayParent(), key); err != nil {
		rs.warnLifecycle("empty original source release remains owed", "code", lifecycleDoctorCode, "fix", lifecycleDoctorFix, "error", err)
	}
}
