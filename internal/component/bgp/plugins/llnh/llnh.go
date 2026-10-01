// Design: docs/architecture/core-design.md — link-local next-hop plugin
// RFC: rfc/short/rfc5549.md
//
// Package llnh implements a link-local next-hop capability plugin for ze.
// It declares capability code 77 (draft-ietf-idr-linklocal-capability) for peers
// that have link-local-nexthop enabled in their config.
//
// Capability 77 has no payload — it is a simple flag signaling willingness
// to receive IPv6 link-local addresses as BGP next-hops (RFC 2545 Section 3).
package llnh

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net"
	"strings"
	"sync/atomic"

	"github.com/ze-software/ze/internal/component/bgp/configjson"
	"github.com/ze-software/ze/internal/component/bgp/plugins/llnh/yang"
	"github.com/ze-software/ze/internal/core/slogutil"
	"github.com/ze-software/ze/internal/core/textbuf"
	sdk "github.com/ze-software/ze/pkg/plugin/sdk"
)

// llnhCapCode is the capability code for link-local next-hop.
// draft-ietf-idr-linklocal-capability: code 77, empty payload.
const llnhCapCode = 77

// modeDisable is the config value that suppresses capability advertisement.
const modeDisable = "disable"

// loggerPtr is the package-level logger, disabled by default.
// Stored as atomic.Pointer to avoid data races when tests start
// multiple in-process plugin instances concurrently.
var loggerPtr atomic.Pointer[slog.Logger]

func init() {
	d := slogutil.DiscardLogger()
	loggerPtr.Store(d)
}

func logger() *slog.Logger { return loggerPtr.Load() }

// setLLNHLogger sets the package-level logger.
func setLLNHLogger(l *slog.Logger) {
	if l != nil {
		loggerPtr.Store(l)
	}
}

// runLLNHPlugin runs the link-local-nexthop plugin using the SDK RPC protocol.
// It receives per-peer config during Stage 2 and registers capability 77
// for peers that have link-local-nexthop enabled during Stage 3.
func runLLNHPlugin(conn net.Conn) int {
	logger().Debug("llnh plugin starting (RPC)")

	p := sdk.NewWithConn("bgp-llnh", conn)
	defer func() { _ = p.Close() }()

	// The same refusal at the two points a configuration arrives. config-verify
	// runs inside the commit transaction, so the operator meets the refusal at
	// the prompt. Stage 2 runs at startup, where the refusal stops the daemon
	// (FatalOnConfigError, register.go).
	p.OnConfigVerify(refuseLinkLocalCapabilityWithoutAddressSections)

	// OnConfigure callback: refuse a peer that would advertise capability 77
	// with no Link-Local address to send, then set capabilities for Stage 3.
	p.OnConfigure(func(sections []sdk.ConfigSection) error {
		if err := refuseLinkLocalCapabilityWithoutAddressSections(sections); err != nil {
			return err
		}
		var caps []sdk.CapabilityDecl
		for _, section := range sections {
			if section.Root != configRootBGP {
				continue
			}
			caps = append(caps, extractLLNHCapabilities(section.Data)...)
		}
		p.SetCapabilities(caps)
		return nil
	})

	ctx, cancel := sdk.SignalContext()
	defer cancel()
	err := p.Run(ctx, sdk.Registration{
		WantsConfig: []string{configRootBGP},
	})
	if err != nil {
		logger().Error("llnh plugin failed", "error", err)
		return 1
	}

	return 0
}

// isLLNHEnabled checks whether a capability map has link-local-nexthop enabled.
// Returns (hasExplicit, enabled).
func isLLNHEnabled(capMap map[string]any) (bool, bool) {
	if capMap == nil {
		return false, false
	}
	llnhVal, exists := capMap["link-local-nexthop"]
	if !exists {
		return false, false
	}
	if s, isStr := llnhVal.(string); isStr && s == modeDisable {
		return true, false
	}
	return true, true
}

// llnhEnabledFor reports whether a peer advertises capability 77: its own
// capability container decides, and the enclosing group's applies only when the
// peer states none. Both the advertisement and the refusal read this one answer,
// so they cannot disagree about which peer carries the capability.
func llnhEnabledFor(peerMap, groupMap map[string]any) bool {
	peerHasExplicit, peerEnabled := isLLNHEnabled(configjson.GetCapability(peerMap))
	if peerHasExplicit {
		return peerEnabled
	}
	if groupMap == nil {
		return false
	}
	_, groupEnabled := isLLNHEnabled(configjson.GetCapability(groupMap))
	return groupEnabled
}

// refuseLinkLocalCapabilityWithoutAddressSections runs
// refuseLinkLocalCapabilityWithoutAddress over every bgp section delivered.
func refuseLinkLocalCapabilityWithoutAddressSections(sections []sdk.ConfigSection) error {
	for _, section := range sections {
		if section.Root != configRootBGP {
			continue
		}
		if err := refuseLinkLocalCapabilityWithoutAddress(section.Data); err != nil {
			return err
		}
	}
	return nil
}

// refuseLinkLocalCapabilityWithoutAddress returns an error naming the first peer
// that enables capability 77 with no `session > link-local` address configured,
// on the peer or on its group.
//
// draft-ietf-idr-linklocal-capability Section 2: "A BGP speaker that is willing
// to use (send and receive) IPv6 Link-Local-only next hops SHOULD advertise the
// Link-Local Next Hop Capability to its peers only when: 1. It is capable of
// sending IPv6 Link-Local-only next hops for a route."
// Section 4: "If the route is directly connected to the speaker, ... the next
// hop MUST include its own Link-Local IPv6 address."
// The leaf is the only source of Ze's own Link-Local on every rail
// (linkScope.linkLocalNextHop, internal/component/bgp/reactor/link_scope.go), so
// a peer without it would negotiate the capability and then break Section 4.
// Deriving the address from the interface instead would make the next hop
// depend on runtime state the configuration does not show.
func refuseLinkLocalCapabilityWithoutAddress(jsonStr string) error {
	bgpSubtree, ok := configjson.ParseBGPSubtree(jsonStr)
	if !ok {
		return nil
	}

	var refusal error
	configjson.ForEachPeer(bgpSubtree, func(peerName string, peerMap, groupMap map[string]any, _ configjson.PeerOrigin) {
		if refusal != nil {
			return
		}
		if !llnhEnabledFor(peerMap, groupMap) {
			return
		}
		if sessionLinkLocal(peerMap) != "" {
			return
		}
		if sessionLinkLocal(groupMap) != "" {
			return
		}
		refusal = fmt.Errorf("peer %s: session capability link-local-nexthop requires session link-local, "+
			"the IPv6 link-local address Ze sends after its global next hop", peerName)
	})
	return refusal
}

// sessionLinkLocal returns the `session > link-local` value of a peer or group
// config map, or "" when the map states none.
func sessionLinkLocal(m map[string]any) string {
	if m == nil {
		return ""
	}
	session, ok := m["session"].(map[string]any)
	if !ok {
		return ""
	}
	linkLocal, _ := session["link-local"].(string)
	return linkLocal
}

// extractLLNHCapabilities parses bgp config JSON and returns per-peer capabilities.
// Handles both standalone peers (bgp.peer) and grouped peers (bgp.group.<name>.peer).
// draft-ietf-idr-linklocal-capability: capability code 77, empty payload.
//
// Caller MUST run refuseLinkLocalCapabilityWithoutAddress over the same section
// first: this function declares the capability for every peer that enables it.
func extractLLNHCapabilities(jsonStr string) []sdk.CapabilityDecl {
	bgpSubtree, ok := configjson.ParseBGPSubtree(jsonStr)
	if !ok {
		logger().Warn("invalid JSON in bgp config")
		return nil
	}

	var caps []sdk.CapabilityDecl

	configjson.ForEachPeer(bgpSubtree, func(peerAddr string, peerMap, groupMap map[string]any, origin configjson.PeerOrigin) {
		if !llnhEnabledFor(peerMap, groupMap) {
			return
		}

		// Capability 77 has empty payload -- just the code signals support
		caps = append(caps, sdk.CapabilityDecl{
			Code:  llnhCapCode,
			Peers: []string{configjson.CapabilitySelector(peerAddr, origin)},
		})
		logger().Debug("link-local-nexthop capability", "peer", peerAddr)
	})

	return caps
}

// getLLNHYANG returns the embedded YANG for the llnh plugin.
func getLLNHYANG() string {
	return yang.ZeLinkLocalNexthopYANG
}

// lLNHDecodableCapabilities returns the capability codes this plugin can decode.
func lLNHDecodableCapabilities() []uint8 {
	return []uint8{llnhCapCode}
}

// runLLNHDecodeMode runs the plugin in decode mode for ze bgp decode.
// Reads decode requests from stdin, writes responses to stdout.
//
// Capability 77 has no payload, so decoding always succeeds with the same output.
func runLLNHDecodeMode(input io.Reader, output io.Writer) int {
	writeResponse := func(s string) {
		_, err := io.WriteString(output, s)
		_ = err // Protocol writes - pipe failure causes exit
	}
	writeUnknown := func() { writeResponse("decoded unknown\n") }

	scanner := bufio.NewScanner(input)
	for scanner.Scan() {
		line := scanner.Text()
		if line == "" {
			continue
		}

		// Parse request: "decode [json|text] capability <code> <hex>"
		parts := strings.Fields(line)
		if len(parts) < 3 || parts[0] != "decode" {
			writeUnknown()
			continue
		}

		// Determine format and adjust parts index
		format := "json"
		capIdx := 1
		if parts[1] == "json" || parts[1] == "text" {
			format = parts[1]
			capIdx = 2
			if len(parts) < 4 {
				writeUnknown()
				continue
			}
		}

		if parts[capIdx] != "capability" {
			writeUnknown()
			continue
		}

		codeIdx := capIdx + 1
		if parts[codeIdx] != "77" {
			writeUnknown()
			continue
		}

		// Capability 77 has empty payload — no hex to decode
		if format == "text" {
			writeResponse("decoded text link-local-nexthop\n")
		} else {
			result := map[string]any{
				"name": "link-local-nexthop",
			}
			jsonBytes, err := json.Marshal(result)
			if err != nil {
				writeUnknown()
				continue
			}
			writeResponse("decoded json " + string(jsonBytes) + "\n")
		}
	}
	// bufio.Scanner reports a read failure and an over-long line through Err(),
	// never through Scan(). Without this the caller sees a clean, complete decode.
	if err := scanner.Err(); err != nil {
		var eb textbuf.Buffer
		writeResponse(eb.Str("decoded error ").Err(err).Byte('\n').String())
		return 1
	}
	return 0
}

// runLLNHCLIDecode decodes hex capability data directly from CLI arguments.
// For capability 77, the payload is always empty — this just confirms the capability.
func runLLNHCLIDecode(hexData string, textOutput bool, stdout, stderr io.Writer) int {
	write := func(w io.Writer, s string) {
		_, err := io.WriteString(w, s)
		_ = err // CLI output - pipe failure causes exit
	}

	if textOutput {
		write(stdout, "link-local-nexthop\n")
	} else {
		result := map[string]any{
			"name": "link-local-nexthop",
		}
		jsonBytes, err := json.Marshal(result)
		if err != nil {
			write(stderr, "error: JSON encoding: "+err.Error()+"\n")
			return 1
		}
		write(stdout, string(jsonBytes)+"\n")
	}
	return 0
}
