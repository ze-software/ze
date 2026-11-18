// Design: docs/architecture/api/process-protocol.md -- ordinary local-origin offers.
// Related: origin_receipt.go -- current authority; reactor_api_batch.go -- direct/maintenance rails.
package reactor

import (
	"context"
	"errors"
	"fmt"

	bgptypes "github.com/ze-software/ze/internal/component/bgp/types"
	"github.com/ze-software/ze/internal/component/plugin"
	"github.com/ze-software/ze/internal/core/bgp/attribute"
	bgpctx "github.com/ze-software/ze/internal/core/bgp/context"
	"github.com/ze-software/ze/internal/core/bgp/nlri"
	"github.com/ze-software/ze/internal/core/opererr"
	"github.com/ze-software/ze/internal/core/redistevents"
	"github.com/ze-software/ze/internal/core/selector"
	"github.com/ze-software/ze/pkg/plugin/rpc"
)

// AnnounceNLRIBatch retains the error-only interface for existing internal
// maintenance producers. Command producers use the complete result below.
func (a *reactorAPIAdapter) AnnounceNLRIBatch(ctx context.Context, sel *selector.Selector, batch bgptypes.NLRIBatch, sender plugin.Sender) error {
	var output rpc.UpdateRouteOutput
	return a.announceNLRIBatch(ctx, sel, batch, sender, &output)
}

func (a *reactorAPIAdapter) AnnounceNLRIBatchResult(ctx context.Context, sel *selector.Selector, batch bgptypes.NLRIBatch, sender plugin.Sender) (*rpc.UpdateRouteOutput, error) {
	output := &rpc.UpdateRouteOutput{}
	err := a.announceNLRIBatch(ctx, sel, batch, sender, output)
	if err != nil {
		output.Failure = err.Error()
	}
	return output, err
}

func (a *reactorAPIAdapter) WithdrawNLRIBatch(ctx context.Context, sel *selector.Selector, batch bgptypes.NLRIBatch, sender plugin.Sender) error {
	var output rpc.UpdateRouteOutput
	return a.withdrawNLRIBatch(ctx, sel, batch, sender, &output)
}

func (a *reactorAPIAdapter) WithdrawNLRIBatchResult(ctx context.Context, sel *selector.Selector, batch bgptypes.NLRIBatch, sender plugin.Sender) (*rpc.UpdateRouteOutput, error) {
	output := &rpc.UpdateRouteOutput{}
	err := a.withdrawNLRIBatch(ctx, sel, batch, sender, output)
	if err != nil {
		output.Failure = err.Error()
	}
	return output, err
}

// These markers belong to already-owned received, recovery or SENT work. They
// never become new local candidates. Configured restoration stays on its own
// existing rail until the configured-origin producer adopts exact source keys.
func originBatchEligible(batch bgptypes.NLRIBatch) bool {
	if batch.SentOwnerMessage != 0 || batch.InitialReplay != 0 || batch.SourcePolicyEpoch != 0 {
		return false
	}
	if batch.RecoverySource.IsValid() || batch.RestoreConfigured {
		return false
	}
	if batch.Stale != 0 || batch.Replay {
		return false
	}
	return true
}

func originBatchProducer(ctx context.Context, batch bgptypes.NLRIBatch, sender plugin.Sender) (originProducer, error) {
	return originBatchProducerOperation(ctx, batch, sender, rpc.OriginIntakeAnnounce)
}

func originBatchProducerOperation(ctx context.Context, batch bgptypes.NLRIBatch, sender plugin.Sender, operation rpc.OriginIntakeOperation) (originProducer, error) {
	if sender.IsOperator() {
		return operatorOriginProducer(), nil
	}
	owner := batch.OriginOwner
	if !owner.Sender().IsSet() {

		var err error
		owner, err = plugin.OriginProducerFromContext(ctx)
		if err != nil {
			return originProducer{}, err
		}
	}
	if owner.Sender() != sender {
		return originProducer{}, errors.New("origin producer does not match the authenticated sender")
	}
	if operation == rpc.OriginIntakeWithdraw {
		if originRetirementOwner(ctx, owner) {
			return originProducer{kind: originProducerProcess, owner: owner}, nil
		}
	}
	return newProcessOriginProducer(owner)
}

// captureOriginBuild owns the original API bytes once per admitted key. A wire
// block keeps its original ContextID, including unknown zero. A Builder is
// explicitly API ASN4; RawWire and every AppendAttributes contribution survive.
func captureOriginBuild(batch bgptypes.NLRIBatch, n nlri.NLRI) (*originBuildValue, error) {
	if n == nil {
		return nil, errors.New("origin native path is nil")
	}
	if n.Family() != batch.Family {
		return nil, errors.New("origin native path family differs from its batch")
	}
	input := originBuildInput{family: batch.Family, native: n.Bytes(), pathID: n.PathID(), hasPathID: nlri.HasPathID(n), nextHop: batch.NextHop, originAS: batch.OriginAS}
	switch {
	case batch.Wire != nil:
		input.attributes = batch.Wire.Packed()
		input.attributesExplicit = true
		input.attributeContext = batch.Wire.SourceContext()
	case batch.Attrs != nil:
		input.attributesExplicit = true
		input.attributeContext = bgpctx.APIContextID
		input.attributes = batch.Attrs.RawWire()
		if input.attributes == nil {
			var scratch [attribute.BuilderInlineAttrs]attribute.Attribute
			attrs := batch.Attrs.AppendAttributes(scratch[:0])
			size := 0
			for _, attr := range attrs {
				size += attribute.AttrWireLen(attr)
			}
			// Retained local build ownership, not a per-destination encoding buffer.
			input.attributes = make([]byte, size)
			off := 0
			for _, attr := range attrs {
				off += attribute.WriteAttrTo(attr, input.attributes, off)
			}
		}
	}
	return newOriginBuildValue(input)
}

// offerOriginBatch receives only peers already admitted by getMatchingPeersSel.
// It removes an ordinary destination from direct output ONLY when its exact
// configured owner opted into OriginIntake. Unknown configured capability
// refuses before direct output. Known live legacy owners, explicit roles and
// API-only/no-RIB peers preserve their rails. No daemon candidate election occurs.
func (a *reactorAPIAdapter) offerOriginBatch(ctx context.Context, batch bgptypes.NLRIBatch, sender plugin.Sender, peers []*Peer, operation rpc.OriginIntakeOperation, output *rpc.UpdateRouteOutput) ([]*Peer, error) {
	if operation == rpc.OriginIntakeWithdraw {
		if retirement, tracked := plugin.OriginRetirementFromContext(ctx); tracked {
			return a.retireTrackedOriginBatch(ctx, batch, sender, peers, retirement, output)
		}
	}
	if !originBatchEligible(batch) {
		return peers, nil
	}
	direct := peers
	partitioned := false
	var producer originProducer
	var source *originSource
	var builds []*originBuildValue
	prepared := false
	var counted []bool
	for peerIndex, peer := range peers {
		peer.mu.RLock()
		configured := peer.settings.DisseminationOwner
		peer.mu.RUnlock()
		if !configured.IsSet() || configured.Explicit() {
			if partitioned {
				direct = append(direct, peer)
			}
			continue
		}
		if a.r.api == nil {
			return nil, opererr.New("BGP route origination has no deciding-process connection", "A concrete destination has a configured ordinary owner; its unavailable capability cannot authorize direct output", "ze show doctor")
		}
		owner, supported, err := a.r.api.OriginIntakeOwner(configured.Name())
		if err != nil {
			return nil, err
		}
		if !supported {
			if partitioned {
				direct = append(direct, peer)
			}
			continue
		}
		if !partitioned {
			direct = append(make([]*Peer, 0, len(peers)), peers[:peerIndex]...)
			partitioned = true
		}
		if !prepared {
			producer, err = originBatchProducerOperation(ctx, batch, sender, operation)
			if err != nil {
				return nil, err
			}
			source, err = newOriginSource(originSourceInput{kind: originSourceLocalBGP})
			if err != nil {
				return nil, err
			}

			builds = make([]*originBuildValue, len(batch.NLRIs))
			if operation == rpc.OriginIntakeAnnounce {
				for i, n := range batch.NLRIs {
					builds[i], err = captureOriginBuild(batch, n)
					if err != nil {
						return nil, err
					}
				}
			}
			counted = make([]bool, len(batch.NLRIs))
			prepared = true
		}
		for i, n := range batch.NLRIs {
			if err := ctx.Err(); err != nil {
				return nil, err
			}
			var result *rpc.UpdateRouteOutput
			if operation == rpc.OriginIntakeAnnounce {
				result, err = a.offerOriginToPeer(ctx, producer, source, builds[i], peer, owner)
			} else {
				if n == nil {
					return nil, errors.New("origin withdrawal native path is nil")
				}
				if n.Family() != batch.Family {
					return nil, errors.New("origin withdrawal native family differs")
				}
				identity, identityErr := newOriginIdentity(batch.Family, n.Bytes(), n.PathID(), nlri.HasPathID(n))
				if identityErr != nil {
					return nil, identityErr
				}
				result, err = a.withdrawOriginFromPeer(ctx, producer, source, identity, peer, owner)
			}
			if result != nil {
				output.Admissions = append(output.Admissions, result.Admissions...)
				output.Warnings = append(output.Warnings, result.Warnings...)
				if !counted[i] {
					if operation == rpc.OriginIntakeAnnounce && result.Announced != 0 {
						output.Announced++
						counted[i] = true
					}
					if operation == rpc.OriginIntakeWithdraw && result.Withdrawn != 0 {
						output.Withdrawn++
						counted[i] = true
					}
				}
			}
			if err != nil {
				return nil, err
			}
		}
	}
	return direct, nil
}

func originIntakeValue(producer originProducer, source *originSource, build *originBuildValue, peer *Peer, receipt uint64, operation rpc.OriginIntakeOperation) rpc.OriginIntakeInput {
	origin := rpc.OriginProducer{Kind: rpc.OriginProducerOperator}
	if name, named := producer.sender().Process(); named {
		origin = rpc.OriginProducer{Kind: rpc.OriginProducerProcess, Name: name}
	}
	value := rpc.OriginIntakeInput{Operation: operation, Destination: peer.addrString, PeerOwner: peer.sourceOwner, OriginReceipt: receipt,
		Producer: origin, Source: rpc.OriginSource{Kind: rpc.OriginSourceKind(source.identity.kind), Instance: source.identity.instance,
			Metric: source.metric, Distance: source.distance, HasDistance: source.hasDistance},
		Route: rpc.OriginBuild{Family: build.family.String(), KeyNLRI: build.native, SourcePathID: build.pathID, SourceAddPath: build.hasPathID,
			Attributes: build.attributes, AttributesExplicit: build.attributesExplicit, SourceContext: uint16(build.attributeContext),
			NextHopPolicy: rpc.OriginNextHopPolicy(build.nextHop.Policy), OriginAS: build.originAS}}
	if encoding := bgpctx.Registry.Get(build.attributeContext); encoding != nil {
		value.Route.ASPathKnown = true
		value.Route.ASPathASN4 = encoding.ASN4()
	}
	if source.identity.protocol != 0 {
		value.Source.Protocol = redistevents.ProtocolName(source.identity.protocol)
	}
	if build.nextHop.IsExplicit() || build.nextHop.Policy == bgptypes.NextHopDefault {
		value.Route.NextHop = build.nextHop.Addr.String()
	}
	return rpc.CloneOriginIntake(value)
}

func (a *reactorAPIAdapter) offerOriginToPeer(ctx context.Context, producer originProducer, source *originSource, build *originBuildValue, peer *Peer, owner plugin.SelectedOwner) (*rpc.UpdateRouteOutput, error) {
	store := a.r.originReceipts
	if store == nil {
		return nil, errors.New("origin intake was declared but its engine domain has not started")
	}
	receipt, err := store.issue(ctx, producer, source, build, peer, owner)
	if err != nil {
		return nil, err
	}
	return a.offerIssuedOrigin(ctx, receipt, peer, owner, false)
}

// offerIssuedOrigin MUST enroll before reaching any provider mutation and MUST
// release only after postvalidation and any bounded compensation have joined.
func (a *reactorAPIAdapter) offerIssuedOrigin(ctx context.Context, receipt uint64, peer *Peer, owner plugin.SelectedOwner, keep bool) (*rpc.UpdateRouteOutput, error) {
	store := a.r.originReceipts
	store.mu.Lock()
	record := store.receipts[receipt]
	store.mu.Unlock()
	operation, release, err := store.beginOriginCall(ctx, record, owner, false)
	if err != nil {
		return nil, errors.Join(err, store.retireOriginReceipt(ctx, receipt))
	}
	defer release()
	if err := store.joinOriginPredecessors(operation.ctx, record.key, receipt); err != nil {
		return nil, errors.Join(err, store.retireOriginReceipt(ctx, receipt))
	}
	if err := store.validateOriginIntakeReceipt(receipt, peer, owner); err != nil {
		return nil, errors.Join(err, store.retireOriginReceipt(ctx, receipt))
	}
	input := originIntakeValue(record.key.producer, record.source, record.build, peer, receipt, rpc.OriginIntakeAnnounce)
	work, cancel := rpc.OriginIntakeCallContext(operation.ctx, &input)
	defer cancel()
	output, err := a.r.api.OriginIntake(work, owner, input)
	if err == nil {
		err = work.Err()
	}
	if err == nil {
		err = store.validateOriginIntakeReceipt(receipt, peer, owner)
	}
	if err == nil {
		err = rpc.ValidateOriginIntakeOutput(input, output)
	}
	if err == nil {
		return output, nil
	}
	if !keep {
		return output, errors.Join(err, a.compensateOriginIntake(ctx, owner, input))
	}
	// Preserve only the still-current immutable original during failed replay.
	// park cannot resurrect a concurrent withdrawal or a newer replacement.
	store.mu.Lock()
	store.park(record)
	idle := record.idle
	store.mu.Unlock()
	cleanup, stop := context.WithTimeout(context.WithoutCancel(ctx), rpc.CancellableRequestTimeout)
	defer stop()
	return output, errors.Join(err, waitSourceReceiptWrites(cleanup, idle), withdrawOriginIntake(cleanup, owner, input))
}

func (a *reactorAPIAdapter) withdrawOriginFromPeer(ctx context.Context, producer originProducer, source *originSource, identity *originIdentity, peer *Peer, owner plugin.SelectedOwner) (*rpc.UpdateRouteOutput, error) {
	store := a.r.originReceipts
	if store == nil {
		return nil, errors.New("origin intake was declared but its engine domain has not started")
	}
	record, err := store.originWithdrawal(ctx, producer, source, identity, peer, owner)
	if err != nil {
		return nil, err
	}
	if record == nil {
		return &rpc.UpdateRouteOutput{Withdrawn: 1}, nil
	}
	// Build/source values are immutable and retained by record. Registry lookup
	// for portable encoding facts must happen outside the leaf issuer mutex.
	input := originIntakeValue(producer, record.source, record.build, peer, record.id, rpc.OriginIntakeWithdraw)
	operation, release, err := store.beginOriginCall(ctx, record, owner, true)
	if err != nil {
		return nil, err
	}
	defer release()
	work, cancel := rpc.OriginIntakeCallContext(operation.ctx, &input)
	defer cancel()
	output, err := a.r.api.OriginIntake(work, owner, input)
	if err == nil {
		err = work.Err()
	}
	if err != nil {
		return output, err
	}
	if err := rpc.ValidateOriginIntakeOutput(input, output); err != nil {
		return nil, err
	}
	// A replacement admitted during the callback is a different obligation.

	err = store.retireOriginReceipt(work, record.id)
	if err != nil {
		return output, fmt.Errorf("origin withdrawal did not join its final writers: %w", err)
	}
	return output, nil
}
