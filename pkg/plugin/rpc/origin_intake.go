// Design: docs/architecture/api/process-protocol.md -- scoped origin admission.
// Related: selected.go -- SelectedRouteOrigin consumes the same engine-issued receipt.
package rpc

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/netip"
	"slices"
	"time"
)

const MethodOriginIntake = "ze-plugin-callback:origin-intake"

type OriginIntakeOperation uint8

const (
	OriginIntakeUnspecified OriginIntakeOperation = iota
	OriginIntakeAnnounce
	OriginIntakeWithdraw
)

type OriginProducerKind uint8

const (
	OriginProducerUnspecified OriginProducerKind = iota
	OriginProducerProcess
	OriginProducerOperator
)

// OriginProducer is descriptive engine provenance, never client authority.
// A process name identifies a presentation; OriginReceipt binds its incarnation.
type OriginProducer struct {
	Kind OriginProducerKind `json:"kind"`
	Name string             `json:"name,omitempty"`
}

type OriginSourceKind uint8

const (
	OriginSourceUnspecified OriginSourceKind = iota
	OriginSourceLocalBGP
	OriginSourceConfiguredBGP
	OriginSourceRedistributed
)

// OriginSource preserves accepted source metadata. Protocol is the registered
// original protocol, not BGP ORIGIN. Distance presence is independent of zero.
// The engine validates it against its trusted accepted-contribution producer.
type OriginSource struct {
	Kind        OriginSourceKind `json:"kind"`
	Protocol    string           `json:"protocol,omitempty"`
	Instance    uint64           `json:"instance"`
	Metric      uint32           `json:"metric"`
	Distance    uint8            `json:"distance"`
	HasDistance bool             `json:"has-distance"`
}

type OriginNextHopPolicy uint8

const (
	OriginNextHopUnset OriginNextHopPolicy = iota
	OriginNextHopExplicit
	OriginNextHopSelf
	OriginNextHopDefault
)

// OriginBuild owns original local build choices. KeyNLRI excludes ADD-PATH
// framing; SourceAddPath preserves legal identifier zero. Attributes are the
// original block, not destination export output or received-source authority.
type OriginBuild struct {
	Family             string `json:"family"`
	KeyNLRI            []byte `json:"key-nlri"`
	SourcePathID       uint32 `json:"source-path-id"`
	SourceAddPath      bool   `json:"source-add-path"`
	Attributes         []byte `json:"attributes"`
	AttributesExplicit bool   `json:"attributes-explicit"`
	SourceContext      uint16 `json:"source-context"`
	// ASPathKnown/ASPathASN4 describe original attribute encoding across a fork.
	// SourceContext remains an opaque engine-local handle, never a plugin lookup.
	ASPathKnown   bool                `json:"as-path-known"`
	ASPathASN4    bool                `json:"as-path-asn4"`
	NextHopPolicy OriginNextHopPolicy `json:"next-hop-policy"`
	NextHop       string              `json:"next-hop,omitempty"`
	OriginAS      uint32              `json:"origin-as"`
}

// OriginIntakeInput names one engine-authorized concrete peer and one current
// origin contribution. The engine alone constructs this value after validating
// the original sender, selector, send grant and source identity. It conveys no
// authority to arbitrary plugin-to-engine callers. Receipt is the existing
// SelectedRoute.OriginReceipt, not a second source-reference namespace.
type OriginIntakeInput struct {
	Operation     OriginIntakeOperation `json:"operation"`
	Destination   string                `json:"destination"`
	PeerOwner     uint64                `json:"peer-owner"`
	OriginReceipt uint64                `json:"origin-receipt"`
	DeadlineMS    int64                 `json:"deadline-ms"`
	Producer      OriginProducer        `json:"producer"`
	Source        OriginSource          `json:"source"`
	Route         OriginBuild           `json:"route"`
}

func (input OriginIntakeInput) Validate(now time.Time) error {
	// The public operation enum is open; unrecognized values fail closed.
	switch input.Operation {
	case OriginIntakeAnnounce, OriginIntakeWithdraw:
	case OriginIntakeUnspecified:
		return errors.New("origin-intake operation is unspecified; use announce or withdraw")
	default:
		return errors.New("origin-intake operation is unspecified; use announce or withdraw")
	}
	if _, err := netip.ParseAddr(input.Destination); err != nil {
		return fmt.Errorf("origin-intake destination: %w", err)
	}
	if input.PeerOwner == 0 {
		return errors.New("origin-intake requires a concrete peer owner")
	}
	if input.OriginReceipt == 0 {
		return errors.New("origin-intake requires an engine-issued origin receipt")
	}
	if input.DeadlineMS <= now.UnixMilli() {
		return context.DeadlineExceeded
	}
	if input.DeadlineMS > now.Add(CancellableRequestTimeout).UnixMilli() {
		return errors.New("origin-intake deadline exceeds thirty seconds")
	}
	// The public producer enum is open; unrecognized values fail closed.
	switch input.Producer.Kind {
	case OriginProducerProcess:
		if input.Producer.Name == "" {
			return errors.New("origin-intake process producer is missing")
		}
	case OriginProducerOperator:
		if input.Producer.Name != "" {
			return errors.New("origin-intake operator cannot name a process")
		}
	case OriginProducerUnspecified:
		return errors.New("origin-intake producer is unspecified")
	default:
		return errors.New("origin-intake producer is unspecified")
	}
	// The public source enum is open; unrecognized values fail closed.
	switch input.Source.Kind {
	case OriginSourceLocalBGP, OriginSourceConfiguredBGP:
		if input.Source.Protocol != "" {
			return errors.New("origin-intake local BGP source cannot claim a redistribution protocol")
		}
	case OriginSourceRedistributed:
		if input.Source.Protocol == "" {
			return errors.New("origin-intake redistributed source protocol is missing")
		}
	case OriginSourceUnspecified:
		return errors.New("origin-intake source is unspecified")
	default:
		return errors.New("origin-intake source is unspecified")
	}
	if input.Route.Family == "" || len(input.Route.KeyNLRI) == 0 {
		return errors.New("origin-intake native identity is missing")
	}
	if !input.Route.SourceAddPath && input.Route.SourcePathID != 0 {
		return errors.New("origin-intake path ID has no presence declaration")
	}
	if !input.Route.AttributesExplicit && len(input.Route.Attributes) != 0 {
		return errors.New("origin-intake implicit attributes carry an explicit block")
	}
	if !input.Route.ASPathKnown && input.Route.ASPathASN4 {
		return errors.New("origin-intake unknown AS-path width cannot claim four-octet encoding")
	}
	switch input.Route.NextHopPolicy {
	case OriginNextHopUnset, OriginNextHopSelf:
		if input.Route.NextHop != "" {
			return errors.New("origin-intake next-hop policy cannot carry an address")
		}
	case OriginNextHopExplicit, OriginNextHopDefault:
		if _, err := netip.ParseAddr(input.Route.NextHop); err != nil {
			return fmt.Errorf("origin-intake next hop: %w", err)
		}
	default:
		return errors.New("origin-intake next-hop policy is unknown")
	}
	data, err := json.Marshal(input)
	if err != nil {
		return err
	}
	if idPrefixMax+len(MethodOriginIntake)+len(data)+1 > MaxMessageSize {
		return errors.New("origin-intake exceeds the RPC frame limit")
	}
	return nil
}

// CloneOriginIntake acquires ownership at the independent receiver boundary.
func CloneOriginIntake(input OriginIntakeInput) OriginIntakeInput {
	input.Route.KeyNLRI = slices.Clone(input.Route.KeyNLRI)
	input.Route.Attributes = slices.Clone(input.Route.Attributes)
	return input
}

// ValidateOriginIntakeOutput checks the P1 result against this exact offered
// scope and receipt. Bare zero is never retained admission or completed removal.
func ValidateOriginIntakeOutput(input OriginIntakeInput, output *UpdateRouteOutput) error {
	if err := output.Validate(); err != nil {
		return err
	}
	if output.Announced > 1 || output.Withdrawn > 1 {
		return errors.New("origin-intake result exceeds its single native contribution")
	}
	if input.Operation == OriginIntakeWithdraw {
		if len(output.Admissions) != 0 {
			return errors.New("origin-intake withdrawal cannot retain an announcement")
		}
		if output.Announced != 0 {
			return errors.New("origin-intake withdrawal reports an announcement")
		}
		if output.Withdrawn == 0 && output.Failure == "" {
			return errors.New("origin-intake withdrawal has no completion receipt")
		}
		return nil
	}
	if output.Withdrawn != 0 {
		return errors.New("origin-intake announcement reports a command withdrawal")
	}
	if len(output.Admissions) != 1 {
		return errors.New("origin-intake announcement requires one explicit admission receipt")
	}
	result := output.Admissions[0]
	if result.Family != input.Route.Family || !bytes.Equal(result.KeyNLRI, input.Route.KeyNLRI) {
		return errors.New("origin-intake receipt native identity differs")
	}
	if result.Destination != input.Destination || result.PeerOwner != input.PeerOwner {
		return errors.New("origin-intake receipt scope differs")
	}
	if result.Outcome == RouteAdmissionRetained {
		if result.Generation != input.OriginReceipt {
			return errors.New("origin-intake receipt generation differs")
		}
		if result.Disposition != RouteAdmissionSelected && output.Announced != 0 {
			return errors.New("origin-intake suppressed candidate increments Announced")
		}
	}
	return nil
}

// OriginIntakeCallContext bounds mutation on either transport.
func OriginIntakeCallContext(ctx context.Context, input *OriginIntakeInput) (context.Context, context.CancelFunc) {
	work, cancel := context.WithTimeout(ctx, CancellableRequestTimeout)
	if input.DeadlineMS > 0 {
		deadline, _ := work.Deadline()
		if time.UnixMilli(input.DeadlineMS).Before(deadline) {
			cancel()
			work, cancel = context.WithDeadline(ctx, time.UnixMilli(input.DeadlineMS))
		}
	}
	deadline, _ := work.Deadline()
	input.DeadlineMS = deadline.UnixMilli()
	return work, cancel
}

// OriginIntakeHandler mutates the deciding plugin's owned candidate domain.
// It MUST honor ctx, join every mutation/barrier before return, and retain no
// mutable input/output alias. It may call engine RPC; the independent worker is
// separate from ordinary callbacks and read-only CurrentSelection.
type OriginIntakeHandler func(context.Context, OriginIntakeInput) (*UpdateRouteOutput, error)

func (m *MuxConn) OriginIntake(ctx context.Context, input OriginIntakeInput) (*UpdateRouteOutput, error) {
	ctx, cancel := OriginIntakeCallContext(ctx, &input)
	defer cancel()
	if err := input.Validate(time.Now()); err != nil {
		return nil, err
	}
	data, err := m.CallRPCCancellable(ctx, MethodOriginIntake, input)
	if err != nil {
		var unsettled *CancellationUnsettledError
		if ctx.Err() != nil && !errors.As(err, &unsettled) {
			return nil, ctx.Err()
		}
		return nil, err
	}
	var output UpdateRouteOutput
	if err := json.Unmarshal(data, &output); err != nil {
		return nil, fmt.Errorf("origin-intake reply: %w", err)
	}
	if err := ValidateOriginIntakeOutput(input, &output); err != nil {
		return nil, err
	}
	resultErr := output.Err()
	return &output, resultErr
}
