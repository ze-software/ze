// Design: docs/architecture/wire/nlri-flowspec.md — FlowSpec NLRI plugin
// RFC: rfc/short/rfc8955.md — traffic filtering actions (Section 7)
// Related: ../../../../../core/bgp/attribute/flowspec_encode.go — the shared action encoders

package flowspec

import (
	"errors"
	"fmt"
	"strings"

	"github.com/ze-software/ze/internal/component/bgp/message"
	"github.com/ze-software/ze/internal/component/bgp/route"
	bgptypes "github.com/ze-software/ze/internal/component/bgp/types"
	"github.com/ze-software/ze/internal/core/bgp/attribute"
)

var errMissingFlowspecCommand = errors.New("missing FlowSpec command")

// EncodeRoute encodes a FlowSpec route command into UPDATE body bytes and NLRI bytes.
// This implements the InProcessRouteEncoder signature for the plugin registry.
func EncodeRoute(routeCmd, family string, localAS uint32, isIBGP, asn4, addPath bool) ([]byte, []byte, error) {
	isIPv6 := strings.HasPrefix(family, "ipv6/")
	ub := message.GetUpdateBuilder(localAS, isIBGP, asn4, addPath)
	defer message.PutUpdateBuilder(ub)

	// Parse route command - expects "match <spec> then <action>"
	args := strings.Fields(routeCmd)
	if len(args) < 1 {
		return nil, nil, errMissingFlowspecCommand
	}

	// Parse using API parser
	parsed, err := route.ParseFlowSpecArgs(args)
	if err != nil {
		return nil, nil, fmt.Errorf("parse error: %w", err)
	}

	// Build FlowSpec NLRI
	var fam Family
	if isIPv6 {
		fam = Family{AFI: AFIIPv6, SAFI: SAFIFlowSpec}
	} else {
		fam = Family{AFI: AFIIPv4, SAFI: SAFIFlowSpec}
	}

	fs := NewFlowSpec(fam)

	// Add components based on parsed route
	if parsed.DestPrefix != nil {
		if err := fs.AddComponent(NewFlowDestPrefixComponent(*parsed.DestPrefix)); err != nil {
			return nil, nil, err
		}
	}
	if parsed.SourcePrefix != nil {
		if err := fs.AddComponent(NewFlowSourcePrefixComponent(*parsed.SourcePrefix)); err != nil {
			return nil, nil, err
		}
	}
	if len(parsed.Protocols) > 0 {
		if err := fs.AddComponent(NewFlowIPProtocolComponent(parsed.Protocols...)); err != nil {
			return nil, nil, err
		}
	}
	if len(parsed.Ports) > 0 {
		if err := fs.AddComponent(NewFlowPortComponent(parsed.Ports...)); err != nil {
			return nil, nil, err
		}
	}
	if len(parsed.DestPorts) > 0 {
		if err := fs.AddComponent(NewFlowDestPortComponent(parsed.DestPorts...)); err != nil {
			return nil, nil, err
		}
	}
	if len(parsed.SourcePorts) > 0 {
		if err := fs.AddComponent(NewFlowSourcePortComponent(parsed.SourcePorts...)); err != nil {
			return nil, nil, err
		}
	}

	// Get NLRI bytes
	nlriBytes := fs.Bytes()

	// Build the Traffic Filtering Action extended-communities (RFC 8955 Section 7).
	extComms, err := flowSpecActionExtComm(parsed)
	if err != nil {
		return nil, nil, err
	}

	// Build UPDATE via the generic plugin builder (SAFI 133, FlowSpec unicast).
	afi := uint16(1)
	if isIPv6 {
		afi = 2
	}
	var rawAttrs [][]byte
	if len(extComms) > 0 {
		raw := make([]byte, 0, 4+len(extComms))
		if len(extComms) > 255 {
			// Extended-length form (flag bit 0x10) when the value exceeds 255 bytes.
			raw = append(raw, 0xC0|0x10, 16, byte(len(extComms)>>8), byte(len(extComms)))
		} else {
			raw = append(raw, 0xC0, 16, byte(len(extComms)))
		}
		raw = append(raw, extComms...)
		rawAttrs = append(rawAttrs, raw)
	}
	params := message.PluginParams{AFI: afi, SAFI: 133, IsIPv6: isIPv6, NLRI: nlriBytes, RawAttrs: rawAttrs}
	update := ub.BuildPlugin(params)
	updateBody := message.PackTo(update, nil)

	return updateBody, nlriBytes, nil
}

// flowSpecActionExtComm builds the Traffic Filtering Action extended-community
// wire bytes (RFC 8955 Section 7) from a parsed FlowSpec route's actions.
//
// Every community comes from the shared encoders in the attribute package, so
// this path and the two config/API parsers cannot drift on what an action means.
//
// An action is emitted when the operator asked for it, which is what the `*Set`
// flags record. Reading the value alone dropped `then rate-limit 0` and
// `then mark 0`: a zero rate is RFC 8955 Section 7.1's discard and DSCP 0 is CS0,
// so both are values rather than absences.
func flowSpecActionExtComm(r bgptypes.FlowSpecRoute) ([]byte, error) {
	var extComms []byte

	// Discard action: RFC 8955 Section 7.1 gives a traffic-rate of 0 that meaning.
	if r.Actions.Discard {
		ec, err := attribute.FlowSpecTrafficRate(attribute.FlowSpecRateBytes, 0, 0)
		if err != nil {
			return nil, err
		}
		extComms = append(extComms, ec[:]...)
	}

	// Rate-limit action (RFC 8955 Section 7.1).
	if r.Actions.RateLimitSet || r.Actions.RateLimit > 0 {
		ec, err := attribute.FlowSpecTrafficRate(attribute.FlowSpecRateBytes, 0, float64(r.Actions.RateLimit))
		if err != nil {
			return nil, err
		}
		extComms = append(extComms, ec[:]...)
	}

	// Packet rate-limit action (RFC 8955 Section 7.2).
	if r.Actions.RateLimitPacketsSet || r.Actions.RateLimitPackets > 0 {
		ec, err := attribute.FlowSpecTrafficRate(attribute.FlowSpecRatePackets, 0, float64(r.Actions.RateLimitPackets))
		if err != nil {
			return nil, err
		}
		extComms = append(extComms, ec[:]...)
	}

	// DSCP marking (RFC 8955 Section 7.5).
	if r.Actions.MarkDSCPSet || r.Actions.MarkDSCP > 0 {
		ec, err := attribute.FlowSpecTrafficMarking(uint64(r.Actions.MarkDSCP))
		if err != nil {
			return nil, err
		}
		extComms = append(extComms, ec[:]...)
	}

	// rt-redirect action (RFC 8955 Section 7.4).
	if r.Actions.Redirect != "" {
		admin, local, ok := strings.Cut(r.Actions.Redirect, ":")
		if !ok {
			return nil, fmt.Errorf("invalid redirect format: %s", r.Actions.Redirect)
		}
		ec, err := attribute.FlowSpecRedirect(admin, local)
		if err != nil {
			return nil, fmt.Errorf("invalid redirect: %w", err)
		}
		extComms = append(extComms, ec[:]...)
	}

	return extComms, nil
}
