// Design: plan/learned/745-ipsec-10-cli-diag.md -- show vpn ipsec handlers.
// Related: monitor_ipsec.go -- streaming `monitor vpn ipsec` sibling of these
// one-shot show handlers.
// Owned by the ike component so that removing it removes the `show vpn ipsec ...`
// command, its schema, and these handlers together. See
// ai/rules/plugin-self-containment.md.

package cmd

import (
	"sort"
	"time"

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
	now := time.Now()
	rows := make([]map[string]any, 0, len(allSAs))
	for _, sa := range allSAs {
		row := saToMap(sa, now, peerInfos)
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
	now := time.Now()
	var matched []map[string]any
	for _, sa := range allSAs {
		if sa.PeerName == peerName {
			matched = append(matched, saToMap(sa, now, peerInfos))
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

func saToMap(sa *engine.SA, now time.Time, peerInfos map[string]engine.PeerInfo) map[string]any {
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
	}

	if info, ok := peerInfos[sa.PeerName]; ok {
		m["rekey-count"] = info.RekeyCount
		if info.HasChild {
			m["child-sa"] = map[string]any{
				"inbound-spi":    info.ChildInSPI,
				"outbound-spi":   info.ChildOutSPI,
				"if-id":          info.ChildIfID,
				"ts-local":       info.TSLocal,
				"ts-remote":      info.TSRemote,
				"esp-encryption": info.ESPEncryption,
				"esp-integrity":  info.ESPIntegrity,
				"lifetime":       info.Lifetime,
			}
		}
	}

	return m
}
