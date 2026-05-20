// Design: plan/spec-ipsec-8-ikev2-child-xfrm.md -- inbound message handling for established SAs
// RFC: rfc/short/rfc7296.md -- INFORMATIONAL (Section 1.4), CREATE_CHILD_SA (Section 1.3)

package engine

import (
	"log/slog"

	"codeberg.org/thomas-mangin/ze/internal/component/ike/wire"
)

// handleEstablishedInbound processes inbound messages on an established IKE SA.
func handleEstablishedInbound(sa *SA, msg *wire.Message, log *slog.Logger) {
	switch msg.Header.ExchangeType {
	case wire.ExchangeInformational:
		handleInformational(sa, msg, log)
	case wire.ExchangeCreateChildSA:
		handleCreateChildSA(sa, msg, log)
	default:
		log.Debug("ike: unexpected exchange on established SA",
			"peer", sa.PeerName, "exchange", msg.Header.ExchangeType)
	}
}

// handleInformational processes INFORMATIONAL exchanges (DPD, DELETE).
// RFC 7296 Section 1.4: empty INFORMATIONAL is a DPD probe/response.
func handleInformational(sa *SA, msg *wire.Message, log *slog.Logger) {
	isResponse := msg.Header.Flags&wire.FlagResponse != 0

	var hasDelete bool
	for _, pe := range msg.Payloads {
		switch p := pe.Payload.(type) {
		case *wire.PayloadDelete:
			hasDelete = true
			if p.ProtocolID == wire.ProtocolIKE {
				log.Info("ike: peer requested IKE SA delete", "peer", sa.PeerName)
				sa.State = StateDead
				return
			}
			log.Info("ike: peer deleted child SA",
				"peer", sa.PeerName, "proto", p.ProtocolID, "spis", len(p.SPIs))
		case *wire.PayloadNotify:
			log.Debug("ike: informational notify",
				"peer", sa.PeerName, "type", p.NotifyMsgType)
		}
	}

	if !hasDelete && isResponse {
		log.Debug("ike: DPD response received", "peer", sa.PeerName)
	}
	if !hasDelete && !isResponse {
		log.Debug("ike: DPD probe received", "peer", sa.PeerName)
	}
}

// handleCreateChildSA processes CREATE_CHILD_SA exchanges.
// RFC 7296 Section 1.3: new child SA, child rekey, or IKE rekey.
func handleCreateChildSA(sa *SA, msg *wire.Message, log *slog.Logger) {
	isResponse := msg.Header.Flags&wire.FlagResponse != 0

	var hasRekeySA bool
	for _, pe := range msg.Payloads {
		if n, ok := pe.Payload.(*wire.PayloadNotify); ok {
			if n.NotifyMsgType == wire.NotifyRekeySA {
				hasRekeySA = true
			}
		}
	}

	if isResponse {
		if hasRekeySA {
			log.Debug("ike: child SA rekey response", "peer", sa.PeerName)
		} else {
			log.Debug("ike: CREATE_CHILD_SA response", "peer", sa.PeerName)
		}
		return
	}

	if hasRekeySA {
		log.Info("ike: peer initiated child SA rekey", "peer", sa.PeerName)
	} else {
		hasSAPayload := false
		hasKE := false
		hasTS := false
		for _, pe := range msg.Payloads {
			switch pe.Payload.(type) {
			case *wire.PayloadSA:
				hasSAPayload = true
			case *wire.PayloadKE:
				hasKE = true
			case *wire.PayloadTS:
				hasTS = true
			}
		}
		if hasSAPayload && hasKE && !hasTS {
			log.Info("ike: peer initiated IKE SA rekey", "peer", sa.PeerName)
		} else {
			log.Info("ike: peer initiated new child SA", "peer", sa.PeerName)
		}
	}
}
