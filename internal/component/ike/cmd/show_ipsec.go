// Design: docs/architecture/ike/ipsec-10-cli-diag.md -- show vpn ipsec handlers.
// Related: monitor_ipsec.go -- streaming `monitor vpn ipsec` sibling of these
// one-shot show handlers.
// Owned by the ike component so that removing it removes the `show vpn ipsec ...`
// command, its schema, and these handlers together. See
// ai/rules/plugins.md.

package cmd

import (
	"sort"
	"time"

	"github.com/ze-software/ze/internal/component/ike/dataplane"
	"github.com/ze-software/ze/internal/component/ike/engine"
	"github.com/ze-software/ze/internal/component/plugin"
	pluginserver "github.com/ze-software/ze/internal/component/plugin/server"
)

func init() {
	pluginserver.RegisterRPCs(
		pluginserver.RPCRegistration{
			WireMethod: "ze-show:vpn-ipsec-sa",
			Handler:    handleShowVPNIPsecSA,
		},
		pluginserver.RPCRegistration{
			WireMethod: "ze-show:vpn-ipsec-status",
			Handler:    handleShowVPNIPsecStatus,
		},
		pluginserver.RPCRegistration{
			WireMethod: "ze-show:vpn-ipsec-peer",
			Handler:    handleShowVPNIPsecPeer,
		},
	)
}

func handleShowVPNIPsecSA(_ *pluginserver.CommandContext, _ []string) (*plugin.Response, error) {
	table := engine.ActiveTable()
	if table == nil {
		return &plugin.Response{Status: plugin.StatusDone, Data: plugin.Map{"peers": []map[string]any{}}}, nil
	}

	allSAs := table.All()
	sort.Slice(allSAs, func(i, j int) bool { return allSAs[i].PeerName < allSAs[j].PeerName })

	peerInfos := engine.PeerInfoMap()
	kernel := readSADCounters()
	now := time.Now()
	rows := make([]map[string]any, 0, len(allSAs))
	for _, sa := range allSAs {
		row := saToMap(sa, now, peerInfos, kernel)
		rows = append(rows, row)
	}

	return &plugin.Response{Status: plugin.StatusDone, Data: plugin.Map{"peers": rows}}, nil
}

func handleShowVPNIPsecStatus(_ *pluginserver.CommandContext, _ []string) (*plugin.Response, error) {
	table := engine.ActiveTable()
	peers := engine.ActivePeers()

	running := table != nil
	configuredPeers := 0
	activeSAs := 0
	establishedSAs := 0

	if peers != nil {
		configuredPeers = len(peers)
	}
	if table != nil {
		allSAs := table.All()
		activeSAs = len(allSAs)
		for _, sa := range allSAs {
			if sa.State == engine.StateEstablished {
				establishedSAs++
			}
		}
	}

	return &plugin.Response{
		Status: plugin.StatusDone,
		Data: plugin.Map{
			"engine-running":   running,
			"configured-peers": configuredPeers,
			"active-ike-sas":   activeSAs,
			"established-sas":  establishedSAs,
		},
	}, nil
}

func handleShowVPNIPsecPeer(ctx *pluginserver.CommandContext, args []string) (*plugin.Response, error) {
	// The peer name is the typed `name <name>` selector
	// (`show vpn ipsec peer name <name>`); a bare positional is accepted as a
	// fallback for programmatic callers.
	peerName := ""
	if ctx != nil {
		peerName = ctx.Selector("name")
	}
	if peerName == "" && len(args) > 0 {
		peerName = args[0]
	}
	if peerName == "" {
		return &plugin.Response{Status: plugin.StatusError, Error: "usage: show vpn ipsec peer name <name>"}, nil
	}
	if len(peerName) > 255 {
		return &plugin.Response{Status: plugin.StatusError, Error: "peer name must be 1-255 characters"}, nil
	}

	table := engine.ActiveTable()
	if table == nil {
		return &plugin.Response{Status: plugin.StatusError, Error: "ipsec engine not running"}, nil
	}

	allSAs := table.All()
	peerInfos := engine.PeerInfoMap()
	kernel := readSADCounters()
	now := time.Now()
	var matched []map[string]any
	for _, sa := range allSAs {
		if sa.PeerName == peerName {
			matched = append(matched, saToMap(sa, now, peerInfos, kernel))
		}
	}

	if len(matched) == 0 {
		return &plugin.Response{Status: plugin.StatusError, Error: "peer not found: " + peerName}, nil
	}

	return &plugin.Response{
		Status: plugin.StatusDone,
		Data: plugin.Map{
			"peer-name": peerName,
			"ike-sas":   matched,
		},
	}, nil
}

// sadCounters is the kernel SAD, indexed by SPI, for one command invocation.
//
// known says whether the SAD was READ AT ALL. It is not the same question as
// whether a given SPI is in it, and collapsing the two is how a tunnel that has
// carried gigabytes comes to report zero: the noop backend and an unprivileged
// process both leave the SAD unreadable, and a zero counter there is a wrong
// answer rather than a missing one (ai/rules/evidence.md).
type sadCounters struct {
	known bool
	bySPI map[uint32]dataplane.SAInfo
}

// lookup returns the counters for one SPI, and whether they are known.
func (c sadCounters) lookup(spi uint32) (bytes, packets uint64, ok bool) {
	if !c.known {
		return 0, 0, false
	}
	info, found := c.bySPI[spi]
	if !found {
		return 0, 0, false
	}
	return info.BytesCurrent, info.PacketsCurrent, true
}

// readSADCounters dumps the kernel SAD for the counter columns.
//
// A failure is NOT an error for the caller. `show vpn ipsec sa` reports engine
// belief and has done so since before this surface existed; losing the kernel
// columns must not lose the command. The failure is recorded as "not known" and
// every counter renders as null rather than as zero.
func readSADCounters() sadCounters {
	dp := dataplane.Get()
	if dp == nil {
		return sadCounters{}
	}
	sas, err := dp.ListSAs(0)
	if err != nil {
		return sadCounters{}
	}
	bySPI := make(map[uint32]dataplane.SAInfo, len(sas))
	for i := range sas {
		bySPI[sas[i].SPI] = sas[i]
	}
	return sadCounters{known: true, bySPI: bySPI}
}

func saToMap(sa *engine.SA, now time.Time, peerInfos map[string]engine.PeerInfo, kernel sadCounters) map[string]any {
	var uptimeSeconds float64
	if sa.State == engine.StateEstablished && !sa.EstablishedAt.IsZero() {
		uptimeSeconds = now.Sub(sa.EstablishedAt).Seconds()
	}

	m := map[string]any{
		"peer-name":      sa.PeerName,
		"state":          sa.State.String(),
		"initiator-spi":  engine.SPIHex(sa.InitiatorSPI),
		"responder-spi":  engine.SPIHex(sa.ResponderSPI),
		"is-initiator":   sa.IsInitiator,
		"encryption":     sa.Proposal.Encryption.ID.String(),
		"integrity":      sa.Proposal.Integrity.ID.String(),
		"dh-group":       sa.Proposal.DHGroup.ID.String(),
		"created-at":     sa.CreatedAt.UTC().Format(time.RFC3339),
		"established-at": sa.EstablishedAt.UTC().Format(time.RFC3339),
		"uptime-seconds": uptimeSeconds,
		"nat-detected":   sa.NATDetected,
		"rekey-count":    uint64(0),
		// RFC 7296 Section 2.3: the number of outstanding requests the peer promised to
		// keep in its SET_WINDOW_SIZE notify. Zero means the peer sent none, which the
		// same section reads as a window of one.
		"peer-window-size": sa.PeerWindowSize,
	}

	if info, ok := peerInfos[sa.PeerName]; ok {
		m["rekey-count"] = info.RekeyCount
		if info.HasChild {
			child := map[string]any{
				"inbound-spi":    info.ChildInSPI,
				"outbound-spi":   info.ChildOutSPI,
				jsonKeyIfID:      info.ChildIfID,
				"ts-local":       info.TSLocal,
				"ts-remote":      info.TSRemote,
				"esp-encryption": info.ESPEncryption,
				"esp-integrity":  info.ESPIntegrity,
				"lifetime":       info.Lifetime,
			}
			addChildCounters(child, info, kernel)
			m["child-sa"] = child
		}
	}

	return m
}

// addChildCounters fills the four counter keys the `sa` and `peer name` YANG
// descriptions have advertised as "byte counts" since 2026-06-03.
//
// They come from the kernel because the IKE engine never sees ESP payload: the
// kernel moves those bytes, so counting them in userspace would report zero for
// a working tunnel.
//
// EVERY KEY IS ALWAYS PRESENT, and an unknown counter is null rather than zero.
// The two are different answers: zero says the SA carried nothing, null says
// nobody could ask. A caller that renders null as 0 would reintroduce exactly the
// false-green this spec exists to remove (ai/rules/evidence.md).
func addChildCounters(child map[string]any, info engine.PeerInfo, kernel sadCounters) {
	inBytes, inPackets, inKnown := kernel.lookup(info.ChildInSPI)
	outBytes, outPackets, outKnown := kernel.lookup(info.ChildOutSPI)

	child["bytes-in"] = counterOrNil(inBytes, inKnown)
	child["packets-in"] = counterOrNil(inPackets, inKnown)
	child["bytes-out"] = counterOrNil(outBytes, outKnown)
	child["packets-out"] = counterOrNil(outPackets, outKnown)

	// counters-known reports whether the SAD WAS READ, not whether this SA was
	// found in it. The two are different answers and the difference is this
	// spec's whole subject:
	//
	//   false -- nobody could ask the kernel (no backend, noop or VPP backend,
	//            no CAP_NET_ADMIN). Nothing is known about this tunnel.
	//   true, with null counters -- the kernel WAS asked and does not hold this
	//            SPI. That is drift, and `show vpn ipsec dataplane drift` names it.
	//
	// Deriving this from whether the lookups hit would collapse the two and throw
	// the more interesting one away.
	child["counters-known"] = kernel.known
}

func counterOrNil(v uint64, known bool) any {
	if !known {
		return nil
	}
	return v
}
