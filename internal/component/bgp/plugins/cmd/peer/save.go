// Design: docs/architecture/api/commands.md — BGP peer config persistence
// Overview: peer.go — BGP peer lifecycle and introspection handlers

package peer

import (
	"fmt"
	"net/netip"
	"slices"
	"time"

	"codeberg.org/thomas-mangin/ze/internal/component/cli"
	"codeberg.org/thomas-mangin/ze/internal/component/plugin"
	pluginserver "codeberg.org/thomas-mangin/ze/internal/component/plugin/server"
)

// defaultReceiveHoldTime is the default receive hold time per RFC 4271 Section 10 (90 seconds).
// Matches reactor/peersettings.go DefaultReceiveHoldTime. Duplicated here to avoid
// importing the reactor package from a command handler.
const defaultReceiveHoldTime = 90 * time.Second

// defaultConnectRetry is the default connect retry interval (5 seconds).
// Matches reactor/peersettings.go DefaultConnectRetry.
const defaultConnectRetry = 5 * time.Second

// HandleBgpPeerSave handles "set bgp peer <selector> save" command.
// Saves selected peer(s) to the config file, merging into existing config.
// Creates a backup before writing. Only writes optional fields that differ
// from reactor defaults (local as, router-id) or protocol defaults (hold-time, connection).
func HandleBgpPeerSave(ctx *pluginserver.CommandContext, _ []string) (*plugin.Response, error) {
	_, errResp, err := pluginserver.RequireReactor(ctx)
	if err != nil {
		return errResp, err
	}

	// Get config path from server
	configPath := ctx.Server.ConfigPath()
	if configPath == "" {
		return &plugin.Response{
			Status: plugin.StatusError,
			Data:   "config path not available",
		}, fmt.Errorf("config path not set")
	}

	// Get peers matching selector
	peers, errResp, err := filterPeersBySelector(ctx)
	if errResp != nil {
		return errResp, err
	}
	if len(peers) == 0 {
		return &plugin.Response{
			Status: plugin.StatusError,
			Data:   "no peers found matching selector",
		}, fmt.Errorf("no peers matched")
	}

	// Open config file via Editor (YANG-aware, creates backup on save)
	ed, err := cli.NewEditor(configPath)
	if err != nil {
		return &plugin.Response{
			Status: plugin.StatusError,
			Data:   fmt.Sprintf("cannot open config: %v", err),
		}, fmt.Errorf("open config: %w", err)
	}
	defer func() { _ = ed.Close() }()

	// Get the reactor stats for default comparison
	stats := ctx.Reactor().Stats()

	// Add/update each peer in the config tree.
	// SetValue navigates through list keys, creating entries as needed:
	// path ["bgp", "peer", "<name>"] creates the peer list entry keyed by peer name.
	var saved []string
	for i := range peers {
		p := &peers[i]
		// Peer key is the name; fall back to IP string if unnamed.
		peerKey := p.Name
		if peerKey == "" {
			peerKey = p.Address.String()
		}
		peerPath := []string{"bgp", "peer", peerKey}
		sessionASNPath := append(slices.Clone(peerPath), "session", "asn")
		sessionPath := append(slices.Clone(peerPath), "session")
		connRemotePath := append(slices.Clone(peerPath), "connection", "remote")
		connLocalPath := append(slices.Clone(peerPath), "connection", "local")

		// session > asn > remote is required
		if err := ed.SetValue(sessionASNPath, "remote", fmt.Sprintf("%d", p.PeerAS)); err != nil {
			return saveFieldError(p.Address, "session asn remote", err)
		}

		// connection > remote > ip is required
		if err := ed.SetValue(connRemotePath, "ip", p.Address.String()); err != nil {
			return saveFieldError(p.Address, "connection remote ip", err)
		}

		// Only write optional fields if they differ from defaults
		if p.LocalAS != 0 && p.LocalAS != stats.LocalAS {
			if err := ed.SetValue(sessionASNPath, "local", fmt.Sprintf("%d", p.LocalAS)); err != nil {
				return saveFieldError(p.Address, "session asn local", err)
			}
		}
		if p.LocalAddress.IsValid() {
			if err := ed.SetValue(connLocalPath, "ip", p.LocalAddress.String()); err != nil {
				return saveFieldError(p.Address, "connection local ip", err)
			}
		}
		if p.RouterID != 0 && p.RouterID != stats.RouterID {
			rid := netip.AddrFrom4([4]byte{
				byte(p.RouterID >> 24), byte(p.RouterID >> 16),
				byte(p.RouterID >> 8), byte(p.RouterID),
			})
			if err := ed.SetValue(sessionPath, "router-id", rid.String()); err != nil {
				return saveFieldError(p.Address, "session router-id", err)
			}
		}
		// Timer container: receive-hold-time, send-hold-time, and connect-retry (only if non-default).
		timerPath := append(slices.Clone(peerPath), "timer")
		if p.ReceiveHoldTime != defaultReceiveHoldTime {
			if err := ed.SetValue(timerPath, "receive-hold-time", fmt.Sprintf("%d", int(p.ReceiveHoldTime.Seconds()))); err != nil {
				return saveFieldError(p.Address, "receive-hold-time", err)
			}
		}
		if p.SendHoldTime != 0 {
			if err := ed.SetValue(timerPath, "send-hold-time", fmt.Sprintf("%d", int(p.SendHoldTime.Seconds()))); err != nil {
				return saveFieldError(p.Address, "send-hold-time", err)
			}
		}
		if p.ConnectRetry != 0 && p.ConnectRetry != defaultConnectRetry {
			if err := ed.SetValue(timerPath, "connect-retry", fmt.Sprintf("%d", int(p.ConnectRetry.Seconds()))); err != nil {
				return saveFieldError(p.Address, "connect-retry", err)
			}
		}
		if !p.Connect {
			if err := ed.SetValue(connRemotePath, "connect", "false"); err != nil {
				return saveFieldError(p.Address, "connection remote connect", err)
			}
		}
		if !p.Accept {
			if err := ed.SetValue(connLocalPath, "accept", "false"); err != nil {
				return saveFieldError(p.Address, "connection local accept", err)
			}
		}

		saved = append(saved, peerKey)
	}

	// Save config (creates backup, writes atomically)
	if err := ed.Save(); err != nil {
		return &plugin.Response{
			Status: plugin.StatusError,
			Data:   fmt.Sprintf("failed to save config: %v", err),
		}, fmt.Errorf("save config: %w", err)
	}

	return &plugin.Response{
		Status: plugin.StatusDone,
		Data: map[string]any{
			"saved":   saved,
			"config":  configPath,
			"message": fmt.Sprintf("saved %d peer(s) to config", len(saved)),
		},
	}, nil
}

// saveFieldError builds an error response for a failed config field write.
func saveFieldError(addr netip.Addr, key string, err error) (*plugin.Response, error) {
	return &plugin.Response{
		Status: plugin.StatusError,
		Data:   fmt.Sprintf("cannot set %s for %s: %v", key, addr, err),
	}, fmt.Errorf("set %s: %w", key, err)
}
