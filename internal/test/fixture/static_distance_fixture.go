// Design: docs/architecture/static-routes.md -- static routes reach the FIB through the Loc-RIB
// Related: ../../component/sysrib/sysrib.go -- showRIB, the arbitration result this reads
// Related: ../../plugins/static/locrib.go -- staticPath, which stamps the declared distance
// Related: netfilter_fixture.go -- registers the two drivers below

package fixture

import (
	"context"
	"errors"
	"time"

	"github.com/ze-software/ze/internal/core/textbuf"
	sdk "github.com/ze-software/ze/pkg/plugin/sdk"
)

// arbitratedPrefix is the one prefix both sources offer. It is the whole point of
// the scenario: a prefix only one source holds proves nothing about ranking.
const arbitratedPrefix = "10.0.0.0/8"

// staticDistanceWinner02 builds the scenario that reads which protocol won the
// contested prefix in the system RIB.
//
// The route arrives through `request bgp rib inject`, so the daemon needs no live
// peer announcing it, and the winner is read from `show rib`, which is the answer
// the FIB plugin is handed. The two configurations that use this differ only in
// `rib { distance { static } }`, so the winner is the one thing the operator
// changed.
func staticDistanceWinner02(wantProtocol string) ObserverScenario {
	return func(ctx context.Context, plugin *sdk.Plugin) error {
		if err := apiRIBReady02(ctx, plugin); err != nil {
			return err
		}
		var inject textbuf.Buffer
		inject.Str("request bgp rib inject 10.0.0.99 ipv4/unicast ").Str(arbitratedPrefix)
		inject.Str(" origin igp aspath 64500 nexthop 198.51.100.1")
		if _, err := requireDone02(ctx, plugin, inject.String()); err != nil {
			return err
		}

		var winner, seen string
		if !Poll(ctx, 100, 100*time.Millisecond, func() bool {
			winner, seen = ribWinner02(ctx, plugin, arbitratedPrefix)
			return winner == wantProtocol
		}) {
			var tb textbuf.Buffer
			tb.Str(arbitratedPrefix).Str(": system RIB winner is ").Str(winner)
			tb.Str(", want ").Str(wantProtocol).Str("; show rib: ").Str(seen)
			return errors.New(tb.String())
		}
		var ok textbuf.Buffer
		ok.Str("OK: ").Str(arbitratedPrefix).Str(" is won by ").Str(wantProtocol).Byte('\n')
		return ok.StdErr()
	}
}

// ribWinner02 returns the protocol that holds prefix in the system RIB, plus the
// raw answer for a failure message. An absent prefix returns an empty protocol,
// which no expectation matches.
func ribWinner02(ctx context.Context, plugin *sdk.Plugin, prefix string) (protocol, raw string) {
	data, err := requireDone02(ctx, plugin, "show rib")
	if err != nil {
		return "", err.Error()
	}
	raw = text02(data)
	var rows []struct {
		Prefix   string `json:"prefix"`
		Protocol string `json:"protocol"`
	}
	if decodeErr := decode02(data, &rows); decodeErr != nil {
		return "", raw
	}
	for _, row := range rows {
		if row.Prefix == prefix {
			return row.Protocol, raw
		}
	}
	return "", raw
}
