// Design: plan/learned/745-ipsec-10-cli-diag.md -- monitor vpn ipsec streaming handler.
// Owned by the ike component (see ai/rules/plugins.md).
// Related: show_ipsec.go -- show vpn ipsec handlers

package cmd

import (
	"context"
	"encoding/json"
	"io"
	"time"

	"github.com/ze-software/ze/internal/component/ike/engine"
	"github.com/ze-software/ze/internal/component/plugin"
	pluginserver "github.com/ze-software/ze/internal/component/plugin/server"
)

func init() {
	pluginserver.RegisterStreamingHandler("monitor vpn ipsec", streamIPsecMonitor)
	pluginserver.RegisterRPCs(
		pluginserver.RPCRegistration{
			WireMethod: "ze-monitor:vpn-ipsec",
			Handler:    handleMonitorIPsec,
		},
	)
}

func handleMonitorIPsec(_ *pluginserver.CommandContext, _ []string) (*plugin.Response, error) {
	return &plugin.Response{
		Status: plugin.StatusDone,
		Data:   plugin.Map{"status": "monitor-configured"},
	}, nil
}

type ipsecMonitorEvent struct {
	Type      string `json:"type"`
	Timestamp string `json:"timestamp"`
	PeerName  string `json:"peer-name"`

	InitiatorSPI string `json:"initiator-spi,omitempty"`
	ResponderSPI string `json:"responder-spi,omitempty"`
	RemoteAddr   string `json:"remote-address,omitempty"`
	AuthMethod   string `json:"auth-method,omitempty"`

	InboundSPI  uint32 `json:"inbound-spi,omitempty"`
	OutboundSPI uint32 `json:"outbound-spi,omitempty"`
	IfID        uint32 `json:"if-id,omitempty"`
}

func streamIPsecMonitor(ctx context.Context, s *pluginserver.Server, w io.Writer, _ string, _ []string) error {
	ch := make(chan ipsecMonitorEvent, 64)

	unsub1 := engine.SAUp.Subscribe(s, func(ev *engine.SAEvent) {
		select {
		case ch <- ipsecMonitorEvent{
			Type: "sa-up", Timestamp: nowUTC(), PeerName: ev.PeerName,
			InitiatorSPI: ev.InitiatorSPI, ResponderSPI: ev.ResponderSPI,
			RemoteAddr: ev.RemoteAddress, AuthMethod: ev.AuthMethod,
		}:
		default:
		}
	})
	defer unsub1()

	unsub2 := engine.SADown.Subscribe(s, func(ev *engine.SAEvent) {
		select {
		case ch <- ipsecMonitorEvent{
			Type: "sa-down", Timestamp: nowUTC(), PeerName: ev.PeerName,
			InitiatorSPI: ev.InitiatorSPI, ResponderSPI: ev.ResponderSPI,
			RemoteAddr: ev.RemoteAddress,
		}:
		default:
		}
	})
	defer unsub2()

	unsub3 := engine.ChildUp.Subscribe(s, func(ev *engine.ChildSAEvent) {
		select {
		case ch <- ipsecMonitorEvent{
			Type: "child-up", Timestamp: nowUTC(), PeerName: ev.PeerName,
			InboundSPI: ev.InboundSPI, OutboundSPI: ev.OutboundSPI, IfID: ev.IfID,
		}:
		default:
		}
	})
	defer unsub3()

	unsub4 := engine.ChildDown.Subscribe(s, func(ev *engine.ChildSAEvent) {
		select {
		case ch <- ipsecMonitorEvent{
			Type: "child-down", Timestamp: nowUTC(), PeerName: ev.PeerName,
			InboundSPI: ev.InboundSPI, OutboundSPI: ev.OutboundSPI, IfID: ev.IfID,
		}:
		default:
		}
	})
	defer unsub4()

	unsub5 := engine.ChildRekey.Subscribe(s, func(ev *engine.ChildSAEvent) {
		select {
		case ch <- ipsecMonitorEvent{
			Type: "child-rekey", Timestamp: nowUTC(), PeerName: ev.PeerName,
			InboundSPI: ev.InboundSPI, OutboundSPI: ev.OutboundSPI, IfID: ev.IfID,
		}:
		default:
		}
	})
	defer unsub5()

	enc := json.NewEncoder(w)
	enc.SetEscapeHTML(false)

	for {
		select {
		case <-ctx.Done():
			return nil
		case ev := <-ch:
			if err := enc.Encode(ev); err != nil {
				return err
			}
		}
	}
}

func nowUTC() string {
	return time.Now().UTC().Format(time.RFC3339)
}
