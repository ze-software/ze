// Design: docs/architecture/api/commands.md -- clear bgp rib advertised
// Related: peer.go -- the peer command registrations this one joins

package peer

import (
	"context"
	"errors"
	"time"

	"github.com/ze-software/ze/internal/component/bgp/route"
	bgptypes "github.com/ze-software/ze/internal/component/bgp/types"
	"github.com/ze-software/ze/internal/component/command"
	"github.com/ze-software/ze/internal/component/plugin"
	pluginserver "github.com/ze-software/ze/internal/component/plugin/server"
	"github.com/ze-software/ze/internal/core/family"
	"github.com/ze-software/ze/internal/core/selector"
	"github.com/ze-software/ze/internal/core/textbuf"
)

// cmdRibClearAdvertised is the command path, quoted in every refusal.
const cmdRibClearAdvertised = "clear bgp rib advertised"

// advertisedSelectorLeaf and advertisedFamilyLeaf are the leaves
// ze-peer-cmd.yang declares under clear bgp rib advertised.
const (
	advertisedSelectorLeaf = "selector"
	advertisedFamilyLeaf   = "family"
)

// advertisedTimeout bounds the withdrawal's admission and socket writes.
const advertisedTimeout = 30 * time.Second

var errAdvertisedNoBGPReactor = errors.New("clear bgp rib advertised: the reactor does not carry BGP")

// handleRibClearAdvertised withdraws the Adj-RIB-Out of the selected peers
// (WithdrawAdvertised). The selector is mandatory, * being the explicit form
// for every peer, and a selector that matches no peer is refused rather than
// answered as an empty table.
func handleRibClearAdvertised(ctx *pluginserver.CommandContext, validated command.ValidatedArgs) (*plugin.Response, error) {
	r, errResp, err := pluginserver.RequireReactor(ctx)
	if err != nil {
		return errResp, err
	}
	reactor, ok := r.(bgptypes.BGPReactor)
	if !ok {
		return &plugin.Response{Status: plugin.StatusError, Error: errAdvertisedNoBGPReactor.Error() +
			", so there is no Adj-RIB-Out to clear: start the daemon with a bgp block in its config"}, errAdvertisedNoBGPReactor
	}

	peers := ctx.Selector(advertisedSelectorLeaf)
	if peers == "" {
		peers = ctx.Peer
	}
	if peers == "" {
		var tb textbuf.Buffer
		return &plugin.Response{Status: plugin.StatusError, Error: tb.Str(cmdRibClearAdvertised).
			Str(" needs a peer selector: ").Str(cmdRibClearAdvertised).
			Str(" * for every peer, or an address, a peer name or an AS pattern for the peers it matches").String()}, nil
	}

	var only family.Family
	if name, given := validated.Value(advertisedFamilyLeaf); given {
		fam, known := family.LookupFamily(name)
		if !known {
			var tb textbuf.Buffer
			return &plugin.Response{Status: plugin.StatusError, Error: tb.Str(cmdRibClearAdvertised).
				Str(": unknown family ").Str(name).Str(": name one as afi/safi, such as ").
				Str(cmdRibClearAdvertised).Str(" * family ipv4/unicast").String()}, nil
		}
		only = fam
	}

	withdrawCtx, cancel := context.WithTimeout(context.Background(), advertisedTimeout)
	defer cancel()

	withdrawn, err := reactor.WithdrawAdvertised(withdrawCtx, selector.ParseDefault(peers), uint16(only.AFI), uint8(only.SAFI), ctx.Sender)
	if errors.Is(err, route.ErrNoPeersMatch) {
		var tb textbuf.Buffer
		return &plugin.Response{Status: plugin.StatusError, Error: tb.Str(cmdRibClearAdvertised).Str(": no peer matches ").
			Str(peers).Str(": show bgp peer lists the peers a selector can name").String()}, nil
	}
	if err != nil {
		var tb textbuf.Buffer
		return &plugin.Response{Status: plugin.StatusError, Error: tb.Str(cmdRibClearAdvertised).Str(": ").
			Str(err.Error()).String(), Data: plugin.Map{"withdrawn": withdrawn}}, err
	}
	return &plugin.Response{Status: plugin.StatusDone, Data: plugin.Map{"withdrawn": withdrawn}}, nil
}
