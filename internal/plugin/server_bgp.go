package plugin

import (
	"context"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"time"

	"codeberg.org/thomas-mangin/ze/internal/ipc"
	"codeberg.org/thomas-mangin/ze/internal/plugin/registry"
	bgpctx "codeberg.org/thomas-mangin/ze/internal/plugins/bgp/context"
	bgpfilter "codeberg.org/thomas-mangin/ze/internal/plugins/bgp/filter"
	"codeberg.org/thomas-mangin/ze/internal/plugins/bgp/format"
	"codeberg.org/thomas-mangin/ze/internal/plugins/bgp/message"
	"codeberg.org/thomas-mangin/ze/internal/plugins/bgp/nlri"
	"codeberg.org/thomas-mangin/ze/internal/plugins/bgp/wireu"
	"codeberg.org/thomas-mangin/ze/pkg/plugin/rpc"
)

// BroadcastValidateOpen sends validate-open to all plugins that declared WantsValidateOpen.
// Returns nil if all accept, or an OpenValidationError on first rejection.
// Called by reactor Peer during OPEN processing.
func (s *Server) BroadcastValidateOpen(peerAddr string, local, remote *message.Open) error {
	return s.broadcastValidateOpenImpl(peerAddr, local, remote)
}

// handleDecodeNLRIRPC handles ze-plugin-engine:decode-nlri from a plugin.
// Routes through the compile-time registry to find the in-process decoder for the family.
func (s *Server) handleDecodeNLRIRPC(proc *Process, connA *PluginConn, req *ipc.Request) {
	s.handleCodecRPC(proc, connA, req, func(params json.RawMessage) (any, error) {
		var input rpc.DecodeNLRIInput
		if err := json.Unmarshal(params, &input); err != nil {
			return nil, fmt.Errorf("invalid decode-nlri params: %w", err)
		}
		result, err := registry.DecodeNLRIByFamily(input.Family, input.Hex)
		if err != nil {
			return nil, err
		}
		return &rpc.DecodeNLRIOutput{JSON: result}, nil
	})
}

// handleEncodeNLRIRPC handles ze-plugin-engine:encode-nlri from a plugin.
// Routes through the compile-time registry to find the in-process encoder for the family.
func (s *Server) handleEncodeNLRIRPC(proc *Process, connA *PluginConn, req *ipc.Request) {
	s.handleCodecRPC(proc, connA, req, func(params json.RawMessage) (any, error) {
		var input rpc.EncodeNLRIInput
		if err := json.Unmarshal(params, &input); err != nil {
			return nil, fmt.Errorf("invalid encode-nlri params: %w", err)
		}
		result, err := registry.EncodeNLRIByFamily(input.Family, input.Args)
		if err != nil {
			return nil, err
		}
		return &rpc.EncodeNLRIOutput{Hex: result}, nil
	})
}

// handleDecodeMPReachRPC handles ze-plugin-engine:decode-mp-reach from a plugin.
// RFC 4760 Section 3: Decodes MP_REACH_NLRI attribute value (AFI+SAFI+NH+Reserved+NLRI).
func (s *Server) handleDecodeMPReachRPC(proc *Process, connA *PluginConn, req *ipc.Request) {
	s.handleCodecRPC(proc, connA, req, func(params json.RawMessage) (any, error) {
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
	})
}

// handleDecodeMPUnreachRPC handles ze-plugin-engine:decode-mp-unreach from a plugin.
// RFC 4760 Section 4: Decodes MP_UNREACH_NLRI attribute value (AFI+SAFI+Withdrawn).
func (s *Server) handleDecodeMPUnreachRPC(proc *Process, connA *PluginConn, req *ipc.Request) {
	s.handleCodecRPC(proc, connA, req, func(params json.RawMessage) (any, error) {
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
	})
}

// handleDecodeUpdateRPC handles ze-plugin-engine:decode-update from a plugin.
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

// RFC 4271 Section 4.3: Decodes full UPDATE message body (after 19-byte BGP header).
func (s *Server) handleDecodeUpdateRPC(proc *Process, connA *PluginConn, req *ipc.Request) {
	s.handleCodecRPC(proc, connA, req, func(params json.RawMessage) (any, error) {
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

		jsonStr := formatDecodeUpdateJSON(result, input.AddPath)
		return &rpc.DecodeUpdateOutput{JSON: jsonStr}, nil
	})
}

// decodeMPNLRIs decodes raw NLRI bytes for the given family, returning a JSON array.
// Plugin families route through the compile-time registry; core families parse via nlri package.
func decodeMPNLRIs(nlriBytes []byte, family nlri.Family, addPath bool) (json.RawMessage, error) {
	if len(nlriBytes) == 0 {
		return json.RawMessage("[]"), nil
	}

	// Plugin families: decode via registry (VPN, EVPN, FlowSpec, BGP-LS)
	familyStr := family.String()
	if registry.PluginForFamily(familyStr) != "" {
		nlriHex := hex.EncodeToString(nlriBytes)
		result, err := registry.DecodeNLRIByFamily(familyStr, nlriHex)
		if err != nil {
			return nil, err
		}
		return json.RawMessage(result), nil
	}

	// Core families: parse via nlri package (IPv4/IPv6 unicast/multicast)
	nlris, err := wireu.ParseNLRIs(nlriBytes, family, addPath)
	if err != nil {
		return nil, err
	}

	return formatNLRIsAsJSON(nlris), nil
}

// formatNLRIsAsJSON formats a slice of NLRIs as a JSON array.
// Uses formatNLRIJSONValue for consistent formatting of all NLRI types.
func formatNLRIsAsJSON(nlris []nlri.NLRI) json.RawMessage {
	var sb strings.Builder
	sb.WriteString("[")
	for i, n := range nlris {
		if i > 0 {
			sb.WriteString(",")
		}
		formatNLRIJSONValue(&sb, n)
	}
	sb.WriteString("]")
	return json.RawMessage(sb.String())
}

// formatDecodeUpdateJSON formats a FilterResult as ze-bgp JSON for the decode-update RPC.
// Produces {"update":{"attr":{...},"nlri":{...}}} without peer/message metadata.
func formatDecodeUpdateJSON(result bgpfilter.FilterResult, addPath bool) string {
	var sb strings.Builder
	sb.WriteString(`{"update":{`)

	// Attributes
	if len(result.Attributes) > 0 {
		sb.WriteString(`"attr":{`)
		formatAttributesJSON(&sb, result)
		sb.WriteString(`},`)
	}

	// Collect NLRI operations by family
	familyOps := make(map[string][]familyOperation)

	// MP-BGP announced routes
	for _, mp := range result.MPReach {
		nlris, err := mp.NLRIs(addPath)
		if err != nil || len(nlris) == 0 {
			continue
		}
		nhStr := mp.NextHop().String()
		familyOps[mp.Family().String()] = append(familyOps[mp.Family().String()], familyOperation{
			Action:  "add",
			NextHop: nhStr,
			NLRIs:   nlris,
		})
	}

	// MP-BGP withdrawn routes
	for _, mp := range result.MPUnreach {
		nlris, err := mp.NLRIs(addPath)
		if err != nil || len(nlris) == 0 {
			continue
		}
		familyOps[mp.Family().String()] = append(familyOps[mp.Family().String()], familyOperation{
			Action: "del",
			NLRIs:  nlris,
		})
	}

	// Legacy IPv4 announced
	if result.IPv4Announced != nil {
		nlris, err := result.IPv4Announced.NLRIs(addPath)
		if err == nil && len(nlris) > 0 {
			familyOps["ipv4/unicast"] = append(familyOps["ipv4/unicast"], familyOperation{
				Action:  "add",
				NextHop: result.IPv4Announced.NextHop().String(),
				NLRIs:   nlris,
			})
		}
	}

	// Legacy IPv4 withdrawn
	if result.IPv4Withdrawn != nil {
		nlris, err := result.IPv4Withdrawn.NLRIs(addPath)
		if err == nil && len(nlris) > 0 {
			familyOps["ipv4/unicast"] = append(familyOps["ipv4/unicast"], familyOperation{
				Action: "del",
				NLRIs:  nlris,
			})
		}
	}

	// Format NLRI operations
	sb.WriteString(`"nlri":{`)
	formatFamilyOpsJSON(&sb, familyOps)
	sb.WriteString(`}}}`)

	return sb.String()
}

// OnMessageReceived handles raw BGP messages from peers.
// Forwards to processes based on API subscriptions.
// Implements reactor.MessageReceiver interface.
//
// This is called for ALL message types (UPDATE, OPEN, NOTIFICATION, KEEPALIVE).
func (s *Server) OnMessageReceived(peer PeerInfo, msg RawMessage) {
	if s.procManager == nil || s.subscriptions == nil {
		logger().Debug("OnMessageReceived: no procManager or subscriptions")
		return
	}

	eventType := messageTypeToEventType(msg.Type)
	if eventType == "" {
		logger().Debug("OnMessageReceived: unknown event type", "msgType", msg.Type)
		return
	}

	logger().Debug("OnMessageReceived", "peer", peer.Address.String(), "event", eventType, "dir", msg.Direction)
	procs := s.subscriptions.GetMatching(NamespaceBGP, eventType, msg.Direction, peer.Address.String())
	logger().Debug("OnMessageReceived matched", "count", len(procs))
	for _, proc := range procs {
		output := s.formatMessageForSubscription(peer, msg, proc.Format())
		logger().Debug("OnMessageReceived writing", "proc", proc.Name(), "outputLen", len(output))
		connB := proc.ConnB()
		if connB == nil {
			continue
		}
		deliverCtx, cancel := context.WithTimeout(s.ctx, 5*time.Second)
		err := connB.SendDeliverEvent(deliverCtx, output)
		cancel()
		if err != nil {
			logger().Warn("OnMessageReceived write failed", "proc", proc.Name(), "err", err)
		}
	}
}

// messageTypeToEventType converts BGP message type to event type string.
// Returns empty string for unsupported types (caller checks for empty).
func messageTypeToEventType(msgType message.MessageType) string {
	switch msgType { //nolint:exhaustive // Only supported types; caller checks empty return
	case message.TypeUPDATE:
		return EventUpdate
	case message.TypeOPEN:
		return EventOpen
	case message.TypeNOTIFICATION:
		return EventNotification
	case message.TypeKEEPALIVE:
		return EventKeepalive
	case message.TypeROUTEREFRESH:
		return EventRefresh
	default: // Unsupported type — caller checks for empty string
		return ""
	}
}

// formatMessageForSubscription formats a BGP message for subscription-based delivery.
// Uses JSON encoding with the specified format (from process settings).
func (s *Server) formatMessageForSubscription(peer PeerInfo, msg RawMessage, fmtMode string) string {
	switch msg.Type { //nolint:exhaustive // Only supported types; unsupported are filtered by caller
	case message.TypeUPDATE:
		content := ContentConfig{
			Encoding: EncodingJSON,
			Format:   fmtMode,
		}
		return FormatMessage(peer, msg, content, "")

	case message.TypeOPEN:
		decoded := format.DecodeOpen(msg.RawBytes)
		return s.encoder.Open(peer, decoded, msg.Direction, msg.MessageID)

	case message.TypeNOTIFICATION:
		decoded := format.DecodeNotification(msg.RawBytes)
		return s.encoder.Notification(peer, decoded, msg.Direction, msg.MessageID)

	case message.TypeKEEPALIVE:
		return s.encoder.Keepalive(peer, msg.Direction, msg.MessageID)

	case message.TypeROUTEREFRESH:
		decoded := format.DecodeRouteRefresh(msg.RawBytes)
		return s.encoder.RouteRefresh(peer, decoded, msg.Direction, msg.MessageID)

	default: // Unsupported type — filtered by messageTypeToEventType before reaching here
		return ""
	}
}

// OnPeerStateChange handles peer state transitions.
// Called by reactor when peer state changes (not a BGP message).
// State events are separate from BGP protocol messages.
func (s *Server) OnPeerStateChange(peer PeerInfo, state string) {
	logger().Debug("OnPeerStateChange", "peer", peer.Address.String(), "state", state)
	if s.procManager == nil || s.subscriptions == nil {
		logger().Debug("OnPeerStateChange: no procManager or subscriptions")
		return
	}

	procs := s.subscriptions.GetMatching(NamespaceBGP, EventState, "", peer.Address.String())
	logger().Debug("OnPeerStateChange matched", "count", len(procs))
	for _, proc := range procs {
		output := FormatStateChange(peer, state, EncodingJSON)
		logger().Debug("OnPeerStateChange writing", "proc", proc.Name())
		connB := proc.ConnB()
		if connB == nil {
			continue
		}
		deliverCtx, cancel := context.WithTimeout(s.ctx, 5*time.Second)
		err := connB.SendDeliverEvent(deliverCtx, output)
		cancel()
		if err != nil {
			logger().Warn("OnPeerStateChange write failed", "proc", proc.Name(), "err", err)
		}
	}
}

// OnPeerNegotiated handles capability negotiation completion.
// Called by reactor after OPEN exchange completes successfully.
// Informs plugins of negotiated capabilities so they can adjust behavior.
func (s *Server) OnPeerNegotiated(peer PeerInfo, neg format.DecodedNegotiated) {
	if s.procManager == nil || s.subscriptions == nil {
		return
	}

	procs := s.subscriptions.GetMatching(NamespaceBGP, EventNegotiated, "", peer.Address.String())
	for _, proc := range procs {
		output := FormatNegotiated(peer, neg, s.encoder)
		connB := proc.ConnB()
		if connB == nil {
			continue
		}
		deliverCtx, cancel := context.WithTimeout(s.ctx, 5*time.Second)
		err := connB.SendDeliverEvent(deliverCtx, output)
		cancel()
		if err != nil {
			logger().Warn("OnPeerNegotiated write failed", "proc", proc.Name(), "err", err)
		}
	}
}

// OnMessageSent handles BGP messages sent to peers.
// Forwards to processes that subscribed to sent events.
// Called by reactor after successfully sending UPDATE to peer.
func (s *Server) OnMessageSent(peer PeerInfo, msg RawMessage) {
	eventType := messageTypeToEventType(msg.Type)
	logger().Debug("OnMessageSent", "peer", peer.Address.String(), "type", eventType)
	if s.procManager == nil || s.subscriptions == nil {
		logger().Debug("OnMessageSent: no procManager or subscriptions")
		return
	}

	if eventType == "" {
		return
	}

	procs := s.subscriptions.GetMatching(NamespaceBGP, eventType, DirectionSent, peer.Address.String())
	logger().Debug("OnMessageSent matched", "count", len(procs))
	for _, proc := range procs {
		output := s.formatSentMessageForSubscription(peer, msg, proc.Format())
		logger().Debug("OnMessageSent writing", "proc", proc.Name())
		connB := proc.ConnB()
		if connB == nil {
			continue
		}
		deliverCtx, cancel := context.WithTimeout(s.ctx, 5*time.Second)
		err := connB.SendDeliverEvent(deliverCtx, output)
		cancel()
		if err != nil {
			logger().Warn("OnMessageSent write failed", "proc", proc.Name(), "err", err)
		}
	}
}

// formatSentMessageForSubscription formats a sent BGP message for subscription delivery.
// Uses FormatSentMessage which sets "type":"sent" to distinguish from received messages.
// The format parameter is the process's configured format (hex, base64, parsed, full).
func (s *Server) formatSentMessageForSubscription(peer PeerInfo, msg RawMessage, fmtMode string) string {
	content := ContentConfig{
		Encoding: EncodingJSON,
		Format:   fmtMode,
	}
	return FormatSentMessage(peer, msg, content)
}

// EncodeNLRI encodes NLRI by routing to the appropriate family plugin via RPC.
// This is the public API for external callers (CLI tools, external plugins, tests).
// Internal code paths use direct function calls for performance (e.g., update_text.go
// calls flowspec.Encode directly). This method exists for callers that don't know
// which plugin handles a family at compile time.
// Returns error if no plugin registered or plugin not running.
func (s *Server) EncodeNLRI(family nlri.Family, args []string) ([]byte, error) {
	if s.registry == nil || s.procManager == nil {
		return nil, fmt.Errorf("server not configured for plugins")
	}

	familyStr := family.String()
	pluginName := s.registry.LookupFamily(familyStr)
	if pluginName == "" {
		return nil, fmt.Errorf("no plugin registered for family %s", familyStr)
	}

	// Get the process
	proc := s.procManager.GetProcess(pluginName)
	if proc == nil {
		return nil, fmt.Errorf("plugin %s not running", pluginName)
	}

	// Send RPC request and wait for response
	connB := proc.ConnB()
	if connB == nil {
		return nil, fmt.Errorf("plugin %s connection closed", pluginName)
	}
	ctx, cancel := context.WithTimeout(s.ctx, 5*time.Second)
	defer cancel()

	hexStr, err := connB.SendEncodeNLRI(ctx, familyStr, args)
	if err != nil {
		return nil, fmt.Errorf("plugin request failed: %w", err)
	}

	data, err := hex.DecodeString(hexStr)
	if err != nil {
		return nil, fmt.Errorf("decode plugin hex response: %w", err)
	}
	return data, nil
}

// DecodeNLRI decodes NLRI by routing to the appropriate family plugin via RPC.
// This is the public API for external callers (CLI tools, external plugins, tests).
// Internal code paths use direct function calls for performance. This method exists
// for callers that don't know which plugin handles a family at compile time.
// Returns the JSON representation of the decoded NLRI.
// Returns error if no plugin registered or plugin not running.
func (s *Server) DecodeNLRI(family nlri.Family, hexData string) (string, error) {
	if s.registry == nil || s.procManager == nil {
		return "", fmt.Errorf("server not configured for plugins")
	}

	familyStr := family.String()
	pluginName := s.registry.LookupFamily(familyStr)
	if pluginName == "" {
		return "", fmt.Errorf("no plugin registered for family %s", familyStr)
	}

	// Get the process
	proc := s.procManager.GetProcess(pluginName)
	if proc == nil {
		return "", fmt.Errorf("plugin %s not running", pluginName)
	}

	// Send RPC request and wait for response
	connB := proc.ConnB()
	if connB == nil {
		return "", fmt.Errorf("plugin %s connection closed", pluginName)
	}
	ctx, cancel := context.WithTimeout(s.ctx, 5*time.Second)
	defer cancel()

	jsonResult, err := connB.SendDecodeNLRI(ctx, familyStr, hexData)
	if err != nil {
		return "", fmt.Errorf("plugin request failed: %w", err)
	}

	return jsonResult, nil
}
