// Design: docs/architecture/api/process-protocol.md -- accepted original retirement.
// Related: internal/component/bgp/plugins/cmd/announce/registry.go -- accepted timer/tag owner.
package plugin

import (
	"bytes"
	"context"
	"errors"

	"github.com/ze-software/ze/pkg/plugin/rpc"
)

// OriginRetirement owns the accepted result of one genuine registry entry.
// It grants no new input or forwarding authority: the issuer must match its
// original source, concrete Peer and stable accepted lineage before removal.
// Both operator and Process entries use this value; a name or metadata cannot
// supply it. Fields and native bytes are immutable after successful capture.
// Safe for concurrent use.
type OriginRetirement struct {
	sender     Sender
	owner      SelectedOwner
	admissions []rpc.RouteAdmission
}

// CaptureOriginRetirement MUST receive the actual successful command result,
// not a caller's request metadata. Its registry owner MUST retain the returned
// value with that accepted entry and use it unchanged at expiry/tag retirement.
func CaptureOriginRetirement(sender Sender, owner SelectedOwner, output *rpc.UpdateRouteOutput) (*OriginRetirement, error) {
	if !sender.IsSet() {
		return nil, errors.New("origin registry entry has no authenticated sender")
	}
	if output == nil {
		return nil, errors.New("origin registry entry has no accepted command result")
	}
	if err := output.Validate(); err != nil {
		return nil, err
	}
	if err := output.Err(); err != nil {
		return nil, err
	}
	retirement := &OriginRetirement{sender: sender, owner: owner}
	for _, admission := range output.Admissions {
		if admission.Outcome != rpc.RouteAdmissionRetained {
			continue
		}
		if _, named := sender.Process(); named {
			if owner.Sender() != sender {
				return nil, errors.New("origin registry result differs from its captured producer")
			}
			if _, captured := owner.ProcessContext(); !captured {
				return nil, errors.New("origin registry producer has no captured Process lifetime")
			}
		}
		admission.KeyNLRI = bytes.Clone(admission.KeyNLRI)
		retirement.admissions = append(retirement.admissions, admission)
	}
	return retirement, nil
}

// OwnsProducer checks a captured Process even after it ends. An operator has no
// Process token and is matched explicitly by MatchesOrigin instead.
func (retirement *OriginRetirement) OwnsProducer(owner SelectedOwner) bool {
	if retirement == nil {
		return false
	}
	return retirement.owner.SameIncarnation(owner)
}

// HasPeer says this accepted entry actually admitted an ordinary contribution
// to this original Peer. An empty result leaves legacy direct output alone.
func (retirement *OriginRetirement) HasPeer(sender Sender, peerOwner uint64) bool {
	if retirement == nil {
		return false
	}
	if !retirement.sender.SameIssuer(sender) {
		return false
	}
	for _, admission := range retirement.admissions {
		if admission.PeerOwner == peerOwner {
			return true
		}
	}
	return false
}

// HasDestination recognizes an originally admitted ordinary destination even
// after its concrete Peer is replaced. That successor cannot become an old
// entry's direct fallback merely because its new Peer token does not match.
func (retirement *OriginRetirement) HasDestination(sender Sender, destination string) bool {
	if retirement == nil {
		return false
	}
	if !retirement.sender.SameIssuer(sender) {
		return false
	}
	for _, admission := range retirement.admissions {
		if admission.Destination == destination {
			return true
		}
	}
	return false
}

// MatchesOrigin is a fact comparison, never permission. The engine supplies
// the actual current record's captured producer, native key, Peer and lineage.
// Changing only a receipt during reoffer preserves that lineage; fresh ingress
// must have a different one, even when its attributes happen to be identical.
func (retirement *OriginRetirement) MatchesOrigin(sender Sender, owner SelectedOwner, family string, native []byte, peerOwner, lineage uint64) bool {
	if !retirement.HasPeer(sender, peerOwner) {
		return false
	}
	if _, named := sender.Process(); named {
		if !retirement.OwnsProducer(owner) {
			return false
		}
	}
	for _, admission := range retirement.admissions {
		if admission.PeerOwner != peerOwner {
			continue
		}
		if admission.Generation != lineage {
			continue
		}
		if admission.Family != family {
			continue
		}
		if bytes.Equal(admission.KeyNLRI, native) {
			return true
		}
	}
	return false
}

type originRetirementKey struct{}

// WithOriginRetirement is used only by the genuine accepted-entry registry.
// The issuer still matches the original and current scope; this context cannot
// authorize Announce or a new contribution from an ended producer.
func WithOriginRetirement(ctx context.Context, retirement *OriginRetirement) context.Context {
	if retirement == nil {
		return ctx
	}
	if !retirement.sender.IsSet() {
		return ctx
	}
	return context.WithValue(ctx, originRetirementKey{}, retirement)
}

func OriginRetirementFromContext(ctx context.Context) (*OriginRetirement, bool) {
	retirement, found := ctx.Value(originRetirementKey{}).(*OriginRetirement)
	if !found {
		return nil, false
	}
	if retirement == nil {
		return nil, false
	}
	return retirement, retirement.sender.IsSet()
}
