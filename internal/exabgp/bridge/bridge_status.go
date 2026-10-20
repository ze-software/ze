// Design: docs/architecture/exabgp-bridge.md — the bridge answers `status` itself
// Related: bridge_ping.go — the daemon identity status shares with ping
// Related: bridge_batch.go — AnswerBatch writes the reply before the done

package bridge

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/netip"
	"os"
	"slices"
	"strings"
	"time"

	"github.com/ze-software/ze/internal/core/bgp/asn"
	"github.com/ze-software/ze/internal/core/textbuf"
)

// StatusPeerCommand is the ze command whose answer names every peer and its
// FSM state. ExaBGP's status lists reactor._peers, every peer the daemon
// holds, so the selector is every peer rather than the script's own.
const StatusPeerCommand = "show bgp peer * detail"

// daemonStart is when this daemon started, as ExaBGP's
// reactor.daemon_start_time is (src/exabgp/reactor/loop.py, `time.time()` in
// Reactor.__init__). It is read once, with daemonUUID, when the package
// initializes.
var daemonStart = time.Now()

// StatusPeer is one line of ExaBGP's peer list: the neighbor's ExaBGP name and
// its FSM state.
type StatusPeer struct {
	// Name is ExaBGP's Neighbor.name(), the key of reactor._peers.
	Name string
	// State is the RFC 4271 Section 8.2.2 state in upper case, FSM.name().
	State string
}

// statusInfo is what one status answer reports. The process values are
// fields, rather than read inside write, so a test can hold the bytes.
type statusInfo struct {
	uuid  string
	pid   int
	start time.Time
	now   time.Time
	peers []StatusPeer
}

// StatusInJSON answers whether a status line is answered in JSON, as
// API._answers_in_json decides at the pinned ExaBGP
// (src/exabgp/reactor/api/__init__.py): a script held to API 6 always reads
// JSON, any other script only when the line's last word is `json`.
func StatusInJSON(version APIVersion, line string) bool {
	if version == APIV6 {
		return true
	}
	words := strings.Fields(line)
	if len(words) == 0 {
		return false
	}
	return words[len(words)-1] == "json"
}

// StatusReply is the answer a status line owes before its done: ExaBGP's
// reactor.py `status`, over the peers StatusPeers read from ze.
func StatusReply(peers []StatusPeer, asJSON bool) []byte {
	info := statusInfo{uuid: daemonUUID, pid: os.Getpid(), start: daemonStart, now: time.Now(), peers: peers}
	var out bytes.Buffer
	info.write(&out, asJSON)
	return out.Bytes()
}

// statusDetailRow is the part of one `show bgp peer * detail` row the status
// line reads. Local-ip is absent when ze holds no local address.
type statusDetailRow struct {
	LocalIP  string      `json:"local-ip"`
	LocalAS  *asn.Number `json:"local-as"`
	RemoteAS *asn.Number `json:"remote-as"`
	RouterID string      `json:"router-id"`
	FSMState string      `json:"fsm-state"`
}

// errStatusStandalone is the refusal of `status` by the standalone runner
// (`ze exabgp plugin`), whose dispatch answer carries no payload. The plugin
// runner (the exabgp-bridge plugin, bridgerun) answers it.
var errStatusStandalone = errors.New("`status` needs the peer list, which only the exabgp-bridge plugin reads from ze; run the script under `exabgp { bridge { process ... } }` instead of `ze exabgp plugin`")

// errStatusDetail names the answer StatusPeers could not read.
var errStatusDetail = errors.New("the bridge cannot list the peers for `status`")

// StatusPeers reads the answer of StatusPeerCommand into ExaBGP's peer list.
//
// Each name is ExaBGP's Neighbor.name() (src/exabgp/bgp/neighbor/neighbor.py):
//
//	neighbor <ip> local-ip <local|auto> local-as <as|auto> peer-as <as|auto>
//	router-id <id> family-allowed in-open
//
// `in-open` because ze negotiates no multi-session. ExaBGP lists its peers in
// configuration order, which ze's answer, a map keyed by address, does not
// carry, so the peers are ordered by address. A row without its FSM state is
// an error: a peer printed with a state ze never reported would be a guess.
func StatusPeers(detail json.RawMessage) ([]StatusPeer, error) {
	// The engine may carry the document as one JSON string; it is unwrapped
	// once, as every other reader of a dispatch answer does.
	var encoded string
	if json.Unmarshal(detail, &encoded) == nil {
		detail = json.RawMessage(encoded)
	}
	var answer struct {
		Peers map[string]statusDetailRow `json:"peers"`
	}
	if err := json.Unmarshal(detail, &answer); err != nil {
		return nil, errors.Join(errStatusDetail, err)
	}
	addresses := make([]netip.Addr, 0, len(answer.Peers))
	for key := range answer.Peers {
		address, err := netip.ParseAddr(key)
		if err != nil {
			return nil, errors.Join(errStatusDetail, err)
		}
		addresses = append(addresses, address)
	}
	slices.SortFunc(addresses, netip.Addr.Compare)

	peers := make([]StatusPeer, 0, len(addresses))
	for _, address := range addresses {
		row := answer.Peers[address.String()]
		if row.FSMState == "" {
			var tb textbuf.Buffer
			return nil, errors.Join(errStatusDetail,
				errors.New(tb.Str("peer ").Addr(address).Str(" carries no fsm-state").String()))
		}
		peers = append(peers, StatusPeer{Name: statusPeerName(address, &row), State: row.FSMState})
	}
	return peers, nil
}

// statusPeerName writes Neighbor.name() for one peer row.
func statusPeerName(address netip.Addr, row *statusDetailRow) string {
	local := row.LocalIP
	if local == "" {
		local = "auto"
	}
	var tb textbuf.Buffer
	tb.Str("neighbor ").Addr(address).Str(" local-ip ").Str(local)
	tb.Str(" local-as ")
	writeStatusASN(&tb, row.LocalAS)
	tb.Str(" peer-as ")
	writeStatusASN(&tb, row.RemoteAS)
	tb.Str(" router-id ").Str(row.RouterID).Str(" family-allowed in-open")
	return tb.String()
}

// writeStatusASN prints an AS number as ExaBGP does, in decimal, and `auto`
// for one ze does not hold, as ExaBGP prints an unset one. ze's own answer may
// be in the operator's asdot notation, so the value is read, never the spelling.
func writeStatusASN(tb *textbuf.Buffer, number *asn.Number) {
	if number == nil {
		tb.Str("auto")
		return
	}
	if number.Value() == 0 {
		tb.Str("auto")
		return
	}
	tb.Uint32(number.Value())
}

// write writes reactor.py `status`. Text is one processes.write per line;
// JSON is json.dumps with its default separators, start_time a Python float.
func (s *statusInfo) write(w io.Writer, asJSON bool) {
	uptime := int64(s.now.Sub(s.start) / time.Second)
	var tb textbuf.Buffer
	if asJSON {
		tb.Str(`{"version": "`).Str(Version).Str(`", "uuid": "`).Str(s.uuid).Str(`", "pid": `).Int(int64(s.pid))
		tb.Str(`, "uptime": `).Int(uptime).Str(`, "start_time": `)
		writePythonFloat(&tb, s.start)
		tb.Str(`, "peers": {`)
		for i, peer := range s.peers {
			if i > 0 {
				tb.Str(", ")
			}
			tb.Str(statusJSONString(peer.Name)).Str(": ").Str(statusJSONString(peer.State))
		}
		tb.Str("}}\n")
	} else {
		tb.Str("ExaBGP Daemon Status\n====================\n")
		tb.Str("Version: ").Str(Version).Byte('\n')
		tb.Str("UUID: ").Str(s.uuid).Byte('\n')
		tb.Str("PID: ").Int(int64(s.pid)).Byte('\n')
		tb.Str("Uptime: ").Int(uptime / 3600).Str("h ").Int(uptime % 3600 / 60).Str("m ").Int(uptime % 60).Str("s\n")
		tb.Str("Peers: ").Int(int64(len(s.peers))).Byte('\n')
		for _, peer := range s.peers {
			tb.Str("  - ").Str(peer.Name).Str(": ").Str(peer.State).Byte('\n')
		}
	}
	if _, err := w.Write(tb.Bytes()); err != nil {
		slog.Debug("bridge status: write failed", "error", err)
	}
}

// writePythonFloat prints a time as Python's repr of time.time(): the
// shortest digits that read back to the same float, with `.0` when whole.
func writePythonFloat(tb *textbuf.Buffer, t time.Time) {
	before := tb.Len()
	tb.Float(float64(t.UnixNano())/float64(time.Second), -1)
	if bytes.IndexByte(tb.Bytes()[before:], '.') < 0 {
		tb.Str(".0")
	}
}

// statusJSONString quotes a name or a state as json.dumps does. Both are
// built from addresses, numbers and fixed words, so the escapes where Go and
// Python differ (HTML characters, non-ASCII) never arise.
func statusJSONString(s string) string {
	quoted, err := json.Marshal(s)
	if err != nil {
		panic("BUG: a Go string always marshals: " + err.Error())
	}
	return string(quoted)
}
