// Design: docs/architecture/core-design.md — BGP codec RPC handlers
// Overview: event_dispatcher.go — event dispatch to plugins

package server

import (
	"encoding/hex"
	"encoding/json"
	"fmt"
	"sync"

	bgpctx "codeberg.org/thomas-mangin/ze/internal/component/bgp/context"
	bgpfilter "codeberg.org/thomas-mangin/ze/internal/component/bgp/filter"
	"codeberg.org/thomas-mangin/ze/internal/component/bgp/format"
	"codeberg.org/thomas-mangin/ze/internal/component/bgp/wireu"
	"codeberg.org/thomas-mangin/ze/internal/component/plugin/registry"
	"codeberg.org/thomas-mangin/ze/internal/core/family"
	"codeberg.org/thomas-mangin/ze/pkg/plugin/rpc"
)

// CodecRPCHandlers returns the map of BGP codec RPC method handlers.
// Registered with the plugin registry via bgp/server/register.go init().
func CodecRPCHandlers() map[string]func(json.RawMessage) (any, error) {
	return map[string]func(json.RawMessage) (any, error){
		"ze-plugin-engine:decode-nlri":       handleDecodeNLRI,
		"ze-plugin-engine:encode-nlri":       handleEncodeNLRI,
		"ze-plugin-engine:decode-mp-reach":   handleDecodeMPReach,
		"ze-plugin-engine:decode-mp-unreach": handleDecodeMPUnreach,
		"ze-plugin-engine:decode-update":     handleDecodeUpdate,
	}
}

// handleDecodeNLRI handles ze-plugin-engine:decode-nlri from a plugin.
// Routes through the compile-time registry to find the in-process decoder for the family.
func handleDecodeNLRI(params json.RawMessage) (any, error) {
	var input rpc.DecodeNLRIInput
	if err := json.Unmarshal(params, &input); err != nil {
		return nil, fmt.Errorf("invalid decode-nlri params: %w", err)
	}
	result, err := registry.DecodeNLRIByFamily(input.Family, input.Hex)
	if err != nil {
		return nil, err
	}
	return &rpc.DecodeNLRIOutput{JSON: result}, nil
}

// handleEncodeNLRI handles ze-plugin-engine:encode-nlri from a plugin.
// Routes through the compile-time registry to find the in-process encoder for the family.
func handleEncodeNLRI(params json.RawMessage) (any, error) {
	var input rpc.EncodeNLRIInput
	if err := json.Unmarshal(params, &input); err != nil {
		return nil, fmt.Errorf("invalid encode-nlri params: %w", err)
	}
	result, err := registry.EncodeNLRIByFamily(input.Family, input.Args)
	if err != nil {
		return nil, err
	}
	return &rpc.EncodeNLRIOutput{Hex: result}, nil
}

// handleDecodeMPReach handles ze-plugin-engine:decode-mp-reach from a plugin.
// RFC 4760 Section 3: Decodes MP_REACH_NLRI attribute value (AFI+SAFI+NH+Reserved+NLRI).
func handleDecodeMPReach(params json.RawMessage) (any, error) {
	var input rpc.DecodeMPReachInput
	if err := json.Unmarshal(params, &input); err != nil {
		return nil, fmt.Errorf("invalid decode-mp-reach params: %w", err)
	}

	data, err := hex.DecodeString(input.Hex)
	if err != nil {
		return nil, fmt.Errorf("invalid hex: %w", err)
	}

	// RFC 4760 Section 3: minimum is AFI(2)+SAFI(1)+NHLen(1)+Reserved(1) = 5 bytes
	if len(data) < 5 {
		return nil, fmt.Errorf("MP_REACH_NLRI too short: %d bytes", len(data))
	}

	mpw := wireu.MPReachWire(data)
	familyStr := mpw.Family().String()

	var nhStr string
	if nhAddr := mpw.NextHop(); nhAddr.IsValid() {
		nhStr = nhAddr.String()
	}

	nlriJSON, err := decodeMPNLRIs(mpw.NLRIBytes(), mpw.Family(), input.AddPath)
	if err != nil {
		return nil, err
	}

	return &rpc.DecodeMPReachOutput{
		Family:  familyStr,
		NextHop: nhStr,
		NLRI:    nlriJSON,
	}, nil
}

// handleDecodeMPUnreach handles ze-plugin-engine:decode-mp-unreach from a plugin.
// RFC 4760 Section 4: Decodes MP_UNREACH_NLRI attribute value (AFI+SAFI+Withdrawn).
func handleDecodeMPUnreach(params json.RawMessage) (any, error) {
	var input rpc.DecodeMPUnreachInput
	if err := json.Unmarshal(params, &input); err != nil {
		return nil, fmt.Errorf("invalid decode-mp-unreach params: %w", err)
	}

	data, err := hex.DecodeString(input.Hex)
	if err != nil {
		return nil, fmt.Errorf("invalid hex: %w", err)
	}

	// RFC 4760 Section 4: minimum is AFI(2)+SAFI(1) = 3 bytes
	if len(data) < 3 {
		return nil, fmt.Errorf("MP_UNREACH_NLRI too short: %d bytes", len(data))
	}

	mpw := wireu.MPUnreachWire(data)
	familyStr := mpw.Family().String()

	nlriJSON, err := decodeMPNLRIs(mpw.WithdrawnBytes(), mpw.Family(), input.AddPath)
	if err != nil {
		return nil, err
	}

	return &rpc.DecodeMPUnreachOutput{
		Family: familyStr,
		NLRI:   nlriJSON,
	}, nil
}

// statelessDecodeCtxID is a lazily-registered encoding context for stateless decode RPCs.
// Uses ASN4=true (modern default) since the caller has no negotiated capabilities.
var (
	statelessDecodeOnce sync.Once
	statelessDecodeCtx  bgpctx.ContextID
)

func getStatelessDecodeCtxID() bgpctx.ContextID {
	statelessDecodeOnce.Do(func() {
		ctx := bgpctx.EncodingContextForASN4(true)
		statelessDecodeCtx = bgpctx.Registry.Register(ctx)
	})
	return statelessDecodeCtx
}

// handleDecodeUpdate handles ze-plugin-engine:decode-update from a plugin.
// RFC 4271 Section 4.3: Decodes full UPDATE message body (after 19-byte BGP header).
func handleDecodeUpdate(params json.RawMessage) (any, error) {
	var input rpc.DecodeUpdateInput
	if err := json.Unmarshal(params, &input); err != nil {
		return nil, fmt.Errorf("invalid decode-update params: %w", err)
	}

	body, err := hex.DecodeString(input.Hex)
	if err != nil {
		return nil, fmt.Errorf("invalid hex: %w", err)
	}

	// RFC 4271 Section 4.3: minimum is withdrawn_len(2) + attr_len(2) = 4 bytes
	if len(body) < 4 {
		return nil, fmt.Errorf("UPDATE body too short: %d bytes", len(body))
	}

	ctxID := getStatelessDecodeCtxID()
	wu := wireu.NewWireUpdate(body, ctxID)
	wire, err := wu.Attrs()
	if err != nil {
		return nil, fmt.Errorf("parsing attributes: %w", err)
	}

	filter := bgpfilter.NewFilterAll()
	result, err := filter.ApplyToUpdate(wire, body, bgpfilter.NewNLRIFilterAll())
	if err != nil {
		return nil, fmt.Errorf("parsing UPDATE: %w", err)
	}

	jsonStr := format.FormatDecodeUpdateJSON(result, input.AddPath)
	return &rpc.DecodeUpdateOutput{JSON: jsonStr}, nil
}

// decodeMPNLRIs decodes raw NLRI bytes for the given family, returning a JSON array.
// Plugin families route through the compile-time registry; core families parse via nlri package.
func decodeMPNLRIs(nlriBytes []byte, fam family.Family, addPath bool) (json.RawMessage, error) {
	if len(nlriBytes) == 0 {
		return json.RawMessage("[]"), nil
	}

	// Plugin families: decode via registry (VPN, EVPN, FlowSpec, BGP-LS)
	familyStr := fam.String()
	if registry.PluginForFamily(familyStr) != "" {
		nlriHex := hex.EncodeToString(nlriBytes)
		result, err := registry.DecodeNLRIByFamily(familyStr, nlriHex)
		if err != nil {
			return nil, err
		}
		return json.RawMessage(result), nil
	}

	// Core families: parse via nlri package (IPv4/IPv6 unicast/multicast)
	nlris, err := wireu.ParseNLRIs(nlriBytes, fam, addPath)
	if err != nil {
		return nil, err
	}

	return format.FormatNLRIsAsJSON(nlris), nil
}
