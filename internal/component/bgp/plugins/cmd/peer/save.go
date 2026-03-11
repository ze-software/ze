// Design: docs/architecture/api/commands.md — BGP peer config persistence
// Overview: peer.go — BGP peer lifecycle and introspection handlers

package peer

import (
	"fmt"
	"net/netip"
	"time"

	"codeberg.org/thomas-mangin/ze/internal/component/cli"
	"codeberg.org/thomas-mangin/ze/internal/component/plugin"
	pluginserver "codeberg.org/thomas-mangin/ze/internal/component/plugin/server"
)

// defaultHoldTime is the default hold time per RFC 4271 Section 10 (90 seconds).
// Matches reactor/peersettings.go DefaultHoldTime. Duplicated here to avoid
// importing the reactor package from a command handler.
const defaultHoldTime = 90 * time.Second

// handleBgpPeerSave handles "bgp peer <selector> save" command.
// Saves selected peer(s) to the config file, merging into existing config.
// Creates a backup before writing. Only writes optional fields that differ
// from reactor defaults (local-as, router-id) or protocol defaults (hold-time, connection).
func handleBgpPeerSave(ctx *pluginserver.CommandContext, _ []string) (*plugin.Response, error) {
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
	// path ["bgp", "peer", "10.0.0.1"] creates the peer list entry with key "10.0.0.1".
	var saved []string
	for i := range peers {
		p := &peers[i]
		peerPath := []string{"bgp", "peer", p.Address.String()}

		// peer-as is required
		if err := ed.SetValue(peerPath, "peer-as", fmt.Sprintf("%d", p.PeerAS)); err != nil {
			return saveFieldError(p.Address, "peer-as", err)
		}

		// Only write optional fields if they differ from defaults
		if p.LocalAS != 0 && p.LocalAS != stats.LocalAS {
			if err := ed.SetValue(peerPath, "local-as", fmt.Sprintf("%d", p.LocalAS)); err != nil {
				return saveFieldError(p.Address, "local-as", err)
			}
		}
		if p.LocalAddress.IsValid() {
			if err := ed.SetValue(peerPath, "local-address", p.LocalAddress.String()); err != nil {
				return saveFieldError(p.Address, "local-address", err)
			}
		}
		if p.RouterID != 0 && p.RouterID != stats.RouterID {
			rid := netip.AddrFrom4([4]byte{
				byte(p.RouterID >> 24), byte(p.RouterID >> 16),
				byte(p.RouterID >> 8), byte(p.RouterID),
			})
			if err := ed.SetValue(peerPath, "router-id", rid.String()); err != nil {
				return saveFieldError(p.Address, "router-id", err)
			}
		}
		// RFC 4271: hold-time 0 is valid (no keepalives). Save if different from default 90s.
		if p.HoldTime != defaultHoldTime {
			if err := ed.SetValue(peerPath, "hold-time", fmt.Sprintf("%d", int(p.HoldTime.Seconds()))); err != nil {
				return saveFieldError(p.Address, "hold-time", err)
			}
		}
		if p.Connection != "" && p.Connection != "both" {
			if err := ed.SetValue(peerPath, "connection", p.Connection); err != nil {
				return saveFieldError(p.Address, "connection", err)
			}
		}

		saved = append(saved, p.Address.String())
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
