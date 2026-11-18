// Design: docs/architecture/api/process-protocol.md -- original local producer identity.
// Related: origin_receipt.go -- source-owned current records, never an election.
package reactor

import (
	"errors"

	"github.com/ze-software/ze/internal/component/plugin"
	"github.com/ze-software/ze/internal/core/redistevents"
)

type originProducerKind uint8

const (
	originProducerUnspecified originProducerKind = iota
	originProducerProcess
	originProducerOperator
)

// originProducer captures an authenticated engine-side process incarnation or
// an explicit operator. It grants no peer scope. Zero is invalid. The server
// MUST supply its actual Process.SelectedOwner token, never a name-derived
// replacement or a plugin-provided token. Immutable; safe for concurrent reads.
type originProducer struct {
	kind  originProducerKind
	owner plugin.SelectedOwner
}

func newProcessOriginProducer(owner plugin.SelectedOwner) (originProducer, error) {
	if err := owner.Current(); err != nil {
		return originProducer{}, err
	}
	if _, bound := owner.ProcessContext(); !bound {
		return originProducer{}, errors.New("origin producer has no captured Process connection lifetime")
	}
	return originProducer{kind: originProducerProcess, owner: owner}, nil
}

func operatorOriginProducer() originProducer { return originProducer{kind: originProducerOperator} }

func (producer originProducer) current() error {
	// Engine ingress supplies this value; an unset or unknown kind fails closed.
	switch producer.kind {
	case originProducerProcess:
		return producer.owner.Current()
	case originProducerOperator:
		return nil
	case originProducerUnspecified:
		return errors.New("origin producer is unrecorded")
	default:
		return errors.New("origin producer kind is unknown")
	}
}

func (producer originProducer) sender() plugin.Sender {
	if producer.kind == originProducerOperator {
		return plugin.OperatorSender()
	}
	return producer.owner.Sender()
}

type originSourceKind uint8

const (
	originSourceUnspecified originSourceKind = iota
	originSourceLocalBGP
	originSourceConfiguredBGP
	originSourceRedistributed
)

// originSourceInput is raw original provenance supplied by the engine's API,
// configured-origin or accepted redistribution producer. A policy acceptance
// and instance identity MUST be preserved before text formatting loses them.
type originSourceInput struct {
	kind        originSourceKind
	protocol    redistevents.ProtocolID
	instance    uint64
	metric      uint32
	distance    uint8
	hasDistance bool
}

type originSourceIdentity struct {
	kind     originSourceKind
	protocol redistevents.ProtocolID
	instance uint64
}

// originSource is validated identity and original preference input, not a rank
// or permission. Explicit local BGP is not mislabeled as a system static route.
// Nil/zero is invalid; immutable after construction and safe for concurrent reads.
type originSource struct {
	identity    originSourceIdentity
	accepted    *originAcceptedBinding
	metric      uint32
	distance    uint8
	hasDistance bool
}

func newOriginSource(input originSourceInput) (*originSource, error) {
	// Raw engine boundary: all known variants and safe unknown handling.
	switch input.kind {
	case originSourceLocalBGP, originSourceConfiguredBGP:
		if input.protocol != redistevents.ProtocolUnspecified {
			return nil, errors.New("explicit BGP origin carries a redistributed protocol")
		}
	case originSourceRedistributed:
		name := redistevents.ProtocolName(input.protocol)
		if name == "" {
			return nil, errors.New("redistributed origin has no registered original protocol")
		}
		if redistevents.WouldLoop(name, "bgp") {
			return nil, errors.New("a BGP source cannot be redistributed back into BGP")
		}
	case originSourceUnspecified:
		return nil, errors.New("origin source kind is unspecified")
	default:
		return nil, errors.New("origin source kind is unknown")
	}
	return &originSource{identity: originSourceIdentity{kind: input.kind, protocol: input.protocol, instance: input.instance},
		metric: input.metric, distance: input.distance, hasDistance: input.hasDistance}, nil
}
