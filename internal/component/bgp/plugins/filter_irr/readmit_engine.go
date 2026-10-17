// Design: docs/architecture/bgp/filter-irr.md -- re-admitting a route the list did not yet authorize
// Overview: readmit.go -- the worker these commands serve
// RFC: rfc/short/rfc2918.md -- ROUTE-REFRESH sent to the peer

package filter_irr

import (
	"context"
	"encoding/json"
	"errors"
	"net/netip"

	"github.com/ze-software/ze/internal/core/textbuf"
	sdk "github.com/ze-software/ze/pkg/plugin/sdk"
)

var (
	errReadmitPeerAbsent = errors.New("the peer detail answer does not name the peer")
	errReadmitNotDone    = errors.New("the engine did not complete the command")
)

const (
	// sendNotPermittedText is the text of the reactor's errSendNotPermitted
	// (reactor/send_permission.go). An engine refusal reaches a plugin as an
	// RPC error string, so the worker recognizes it by this text.
	sendNotPermittedText = "send refused: no peer this selector names attaches this process"

	dispatchStatusDone = "done"
	peerStateUp        = "established"
)

// dispatchEngine reaches the engine through dispatched commands, the same
// commands an operator types. The ROUTE-REFRESH it asks for is gated by the
// peer's `attach process bgp-filter-irr { send [ refresh ] }`, like any other
// process's.
type dispatchEngine struct {
	plugin *sdk.Plugin
}

// peerDetailAnswer is the part of `show bgp peer <peer> detail` the worker reads.
type peerDetailAnswer struct {
	Peers map[string]struct {
		State        string `json:"state"`
		Capabilities struct {
			RouteRefresh bool `json:"route-refresh"`
		} `json:"capabilities"`
	} `json:"peers"`
}

func (e dispatchEngine) session(ctx context.Context, peer netip.Addr) (peerSession, error) {
	var tb textbuf.Buffer
	command := tb.Str("show bgp peer ").Str(peer.String()).Str(" detail").String()
	status, data, err := e.plugin.DispatchCommand(ctx, command)
	if err != nil {
		return peerSession{}, err
	}
	if status != dispatchStatusDone {
		return peerSession{}, errReadmitNotDone
	}
	var answer peerDetailAnswer
	if err := json.Unmarshal(data, &answer); err != nil {
		return peerSession{}, err
	}
	row, ok := answer.Peers[peer.String()]
	if !ok {
		return peerSession{}, errReadmitPeerAbsent
	}
	return peerSession{
		established:  row.State == peerStateUp,
		routeRefresh: row.Capabilities.RouteRefresh,
	}, nil
}

// refresh asks the peer to send the family again.
// RFC 2918 Section 4.
func (e dispatchEngine) refresh(ctx context.Context, peer netip.Addr, family string) error {
	var tb textbuf.Buffer
	command := tb.Str("request peer ").Str(peer.String()).Str(" refresh ").Str(family).String()
	status, _, err := e.plugin.DispatchCommand(ctx, command)
	if err != nil {
		return err
	}
	if status != dispatchStatusDone {
		return errReadmitNotDone
	}
	return nil
}
