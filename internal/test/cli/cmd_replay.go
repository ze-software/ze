// Design: plan/spec-improve-3-event-replay.md -- replay a captured BGP session
// Related: register.go -- ze-test root handler registration

package cli

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"net"
	"net/netip"
	"os"
	"time"

	"github.com/ze-software/ze/internal/component/bgp/reactor"
	"github.com/ze-software/ze/internal/component/bgp/wireu"
	bgpctx "github.com/ze-software/ze/internal/core/bgp/context"
	"github.com/ze-software/ze/internal/core/bgp/msgtype"
	"github.com/ze-software/ze/internal/core/bgp/nlri"
	"github.com/ze-software/ze/internal/core/capture"
	"github.com/ze-software/ze/internal/core/cliio"
	"github.com/ze-software/ze/internal/core/family"
	"github.com/ze-software/ze/internal/core/textbuf"
	"github.com/ze-software/ze/internal/test/sim"
	"github.com/ze-software/ze/pkg/plugin/rpc"
)

// replayEpoch is the fake clock's start. A replay is deterministic, so the time
// it reports must not depend on when the replay is run.
var replayEpoch = time.Date(2000, time.January, 1, 0, 0, 0, 0, time.UTC)

// replayStep is what one captured message did when it was fed back through the
// real read path.
type replayStep struct {
	MsgType    string   `json:"msg-type"`
	State      string   `json:"state"`
	Error      string   `json:"error,omitempty"`
	Announced  []string `json:"announced,omitempty"`
	Withdrawn  []string `json:"withdrawn,omitempty"`
	Seq        uint64   `json:"seq"`
	Bytes      int      `json:"bytes"`
	Terminated bool     `json:"terminated,omitempty"`
}

// replayCfg is one captured config operation, reported so a divergence that is
// really a config change is visible as one.
type replayCfg struct {
	Op   string `json:"op"`
	TxID string `json:"tx-id,omitempty"`
	Seq  uint64 `json:"seq"`
}

// replayReport is the whole outcome of a replay, and is what a developer
// compares between two builds.
type replayReport struct {
	File     string       `json:"file"`
	Peer     string       `json:"peer"`
	Steps    []replayStep `json:"steps"`
	Config   []replayCfg  `json:"config,omitempty"`
	States   []string     `json:"fsm-states"`
	Sent     []string     `json:"sent,omitempty"`
	Drops    uint64       `json:"dropped-events,omitempty"`
	LocalAS  uint32       `json:"local-as"`
	PeerAS   uint32       `json:"peer-as"`
	RouterID uint32       `json:"router-id"`
	Coalesce bool         `json:"coalesce"`
}

func cmdReplay(args []string) int {
	fs := flag.NewFlagSet("replay", flag.ContinueOnError)
	asJSON := fs.Bool("json", false, "emit the replay report as JSON")
	// 0 means "take it from the capture header". A header written before the
	// identity fields existed carries zeros, and replayFallback* covers that
	// case only.
	localAS := fs.Uint("local-as", 0, "local AS override; 0 reads it from the capture header")
	peerAS := fs.Uint("peer-as", 0, "peer AS override; 0 reads it from the capture header")
	routerID := fs.Uint("router-id", 0, "router-id override; 0 reads it from the capture header")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	if fs.NArg() != 1 {
		fmt.Fprintln(os.Stderr, "usage: ze-test replay [--json] [--local-as N] [--peer-as N] [--router-id N] <capture-file|->") //nolint:errcheck // usage
		return 2
	}
	path := fs.Arg(0)

	report, err := runReplay(path, replayIdentity{
		localAS:  uint32(*localAS),
		peerAS:   uint32(*peerAS),
		routerID: uint32(*routerID),
	})
	if err != nil {
		fmt.Fprintf(os.Stderr, "replay %s: %v\n", path, err) //nolint:errcheck // error output
		return 1
	}
	if *asJSON {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		if err := enc.Encode(report); err != nil {
			fmt.Fprintf(os.Stderr, "encode report: %v\n", err) //nolint:errcheck // error output
			return 1
		}
		return 0
	}
	printReplayReport(report)
	return 0
}

// replayIdentity overrides the session identity the capture header records. A
// zero field means "use the header".
type replayIdentity struct {
	localAS  uint32
	peerAS   uint32
	routerID uint32
}

// Fallbacks for a capture whose header carries no identity, which is every file
// written before the header recorded one. They are only ever reached when both
// the header field and the flag are zero.
const (
	replayFallbackLocalAS  = 65000
	replayFallbackPeerAS   = 65001
	replayFallbackRouterID = 0x01020304
)

// resolve picks the identity of the replayed session: the operator's override
// first, then what the capture recorded, then the fallback.
//
// The order matters and it is not cosmetic. Local AS versus peer AS decides
// whether the session is iBGP or eBGP, and that changes which branch OPEN
// validation and the forwarding rules take. A replay that invents the AS numbers
// stops reproducing the run it was fed.
func (ri replayIdentity) resolve(hdr capture.Header) (localAS, peerAS, routerID uint32) {
	pick := func(override, recorded, fallback uint32) uint32 {
		switch {
		case override != 0:
			return override
		case recorded != 0:
			return recorded
		default:
			return fallback
		}
	}
	return pick(ri.localAS, hdr.LocalAS, replayFallbackLocalAS),
		pick(ri.peerAS, hdr.PeerAS, replayFallbackPeerAS),
		pick(ri.routerID, hdr.RouterID, replayFallbackRouterID)
}

// runReplay feeds a capture file back through the real session read path.
//
// The point of the harness is that it drives Session.ReadAndProcess, the same
// function the daemon's read loop calls, over a stub connection and an injected
// clock. There is no parallel decoder: the announced and withdrawn prefixes it
// reports come off the WireUpdate the real path built, after RFC 7606
// enforcement, so a bug only the real path reaches is a bug this reproduces.
//
// The capture is opened through cliio, so "-" reads it from stdin
// (ai/rules/cli.md). A capture arrives from another machine, so piping it
// straight in is the normal case, not an edge one. The reader streams: a file
// at the 1024 MB cap is never buffered whole.
func runReplay(path string, ident replayIdentity) (*replayReport, error) {
	f, err := cliio.OpenReader(path)
	if err != nil {
		return nil, err
	}
	defer func() { _ = f.Close() }()

	r, hdr, err := capture.NewReader(f)
	if err != nil {
		return nil, err
	}
	peer, err := netip.ParseAddr(hdr.Peer)
	if err != nil {
		return nil, fmt.Errorf("capture header names peer %q, which is not an address: %w", hdr.Peer, err)
	}

	localAS, peerAS, routerID := ident.resolve(hdr)
	report := &replayReport{
		File: path, Peer: hdr.Peer, Coalesce: hdr.Coalesce,
		LocalAS: localAS, PeerAS: peerAS, RouterID: routerID,
	}

	settings := reactor.NewPeerSettings(peer, localAS, peerAS, routerID)
	session := reactor.NewSession(settings)
	session.SetClock(sim.NewFakeClock(replayEpoch))

	// Bring the session up in the order the daemon does (peer_run.go, runOnce):
	// the FSM leaves Idle only on RFC 4271 Event 1, so a harness that skips
	// Start would feed every captured message to a session still in Idle and
	// observe no transitions at all.
	if startErr := session.Start(); startErr != nil {
		return nil, fmt.Errorf("start replay session: %w", startErr)
	}
	conn := newReplayConn()
	// Accept installs the stub connection and sends our OPEN into it, exactly
	// as an inbound TCP connection would.
	if acceptErr := session.Accept(conn); acceptErr != nil {
		return nil, fmt.Errorf("install replay connection: %w", acceptErr)
	}
	defer func() { _ = session.Close() }()

	var announced, withdrawn []string
	session.SetMessageCallback(func(_ netip.Addr, mt msgtype.MessageType, _ []byte,
		wu *wireu.WireUpdate, _ bgpctx.ContextID, _ rpc.MessageDirection,
		_ reactor.BufHandle, _ map[string]any, _ string) bool {
		if mt == msgtype.TypeUPDATE && wu != nil {
			announced, withdrawn = replayUpdatePrefixes(wu)
		}
		return false
	})

	report.States = append(report.States, session.State().String())
	for {
		ev, readErr := r.Next()
		if errors.Is(readErr, capture.ErrEndOfStream) {
			break
		}
		if readErr != nil {
			return nil, readErr
		}
		switch ev.Type {
		case capture.EventConfig:
			report.Config = append(report.Config, replayCfg{Seq: ev.Seq, Op: ev.Op, TxID: ev.TxID})
			continue
		case capture.EventSession:
			if ev.Drops > report.Drops {
				report.Drops = ev.Drops
			}
			continue
		case capture.EventMessage:
		default:
			continue
		}

		announced, withdrawn = nil, nil
		conn.feed(ev.Data)
		procErr := session.ReadAndProcess()

		step := replayStep{
			Seq:       ev.Seq,
			MsgType:   msgtype.MessageType(ev.MsgType).String(),
			Bytes:     len(ev.Data),
			State:     session.State().String(),
			Announced: announced,
			Withdrawn: withdrawn,
		}
		if procErr != nil {
			step.Error = procErr.Error()
			step.Terminated = errors.Is(procErr, reactor.ErrConnectionClosed)
		}
		report.Steps = append(report.Steps, step)
		report.States = append(report.States, step.State)
		if step.Terminated {
			break
		}
	}
	report.Sent = conn.sentSummary()
	return report, nil
}

// replayUpdatePrefixes reads the IPv4 unicast prefixes off the WireUpdate the
// real read path produced. This is the RIB effect a single-session replay can
// observe: what the peer told this session to install and to remove.
func replayUpdatePrefixes(wu *wireu.WireUpdate) (announced, withdrawn []string) {
	fam := family.IPv4Unicast
	if it, err := wu.WithdrawnIterator(false); err == nil && it != nil {
		withdrawn = collectPrefixes(it, fam)
	}
	if it, err := wu.NLRIIterator(false); err == nil && it != nil {
		announced = collectPrefixes(it, fam)
	}
	return announced, withdrawn
}

func collectPrefixes(it *nlri.NLRIIterator, fam family.Family) []string {
	var out []string
	for {
		wire, _, ok := it.Next()
		if !ok {
			return out
		}
		if p, keyed := nlri.WirePrefixToKey(wire, fam); keyed {
			out = append(out, p.String())
		}
	}
}

func printReplayReport(report *replayReport) {
	fmt.Fprintf(os.Stdout, "capture   %s\n", report.File) //nolint:errcheck // CLI output
	//nolint:errcheck // CLI output
	fmt.Fprintf(os.Stdout, "peer      %s local-as=%d peer-as=%d router-id=%d (coalesce=%v)\n",
		report.Peer, report.LocalAS, report.PeerAS, report.RouterID, report.Coalesce)
	if report.Drops > 0 {
		fmt.Fprintf(os.Stdout, "WARNING   %d events were dropped when this capture was written; the stream has gaps\n", report.Drops) //nolint:errcheck // CLI output
	}
	for _, c := range report.Config {
		fmt.Fprintf(os.Stdout, "config    seq=%d %s tx=%s\n", c.Seq, c.Op, c.TxID) //nolint:errcheck // CLI output
	}
	for _, s := range report.Steps {
		fmt.Fprintf(os.Stdout, "message   seq=%d %s %dB -> %s", s.Seq, s.MsgType, s.Bytes, s.State) //nolint:errcheck // CLI output
		if len(s.Announced) > 0 {
			fmt.Fprintf(os.Stdout, " announce=%v", s.Announced) //nolint:errcheck // CLI output
		}
		if len(s.Withdrawn) > 0 {
			fmt.Fprintf(os.Stdout, " withdraw=%v", s.Withdrawn) //nolint:errcheck // CLI output
		}
		if s.Error != "" {
			fmt.Fprintf(os.Stdout, " error=%q", s.Error) //nolint:errcheck // CLI output
		}
		fmt.Fprintln(os.Stdout) //nolint:errcheck // CLI output
	}
	fmt.Fprintf(os.Stdout, "states    %v\n", report.States) //nolint:errcheck // CLI output
	for _, s := range report.Sent {
		fmt.Fprintf(os.Stdout, "sent      %s\n", s) //nolint:errcheck // CLI output
	}
}

// replayConn is the stub net.Conn a replay drives the session over. Reads return
// the bytes the capture recorded; writes are kept so the report can say what the
// session sent back, which is how a replayed NOTIFICATION becomes visible.
type replayConn struct {
	pending []byte
	sent    []byte
	closed  bool
}

func newReplayConn() *replayConn { return &replayConn{} }

// feed queues one captured message for the session to read.
func (c *replayConn) feed(b []byte) { c.pending = append(c.pending, b...) }

func (c *replayConn) Read(p []byte) (int, error) {
	if len(c.pending) == 0 {
		return 0, io.EOF
	}
	n := copy(p, c.pending)
	c.pending = c.pending[n:]
	return n, nil
}

func (c *replayConn) Write(p []byte) (int, error) {
	if c.closed {
		return 0, net.ErrClosed
	}
	c.sent = append(c.sent, p...)
	return len(p), nil
}

func (c *replayConn) Close() error                     { c.closed = true; return nil }
func (c *replayConn) LocalAddr() net.Addr              { return replayAddr{} }
func (c *replayConn) RemoteAddr() net.Addr             { return replayAddr{} }
func (c *replayConn) SetDeadline(time.Time) error      { return nil }
func (c *replayConn) SetReadDeadline(time.Time) error  { return nil }
func (c *replayConn) SetWriteDeadline(time.Time) error { return nil }

// sentSummary names the messages the session wrote back, one line each.
func (c *replayConn) sentSummary() []string {
	const headerLen = 19
	var out []string
	var b textbuf.Buffer
	buf := c.sent
	for len(buf) >= headerLen {
		length := int(buf[16])<<8 | int(buf[17])
		if length < headerLen || length > len(buf) {
			return out
		}
		mt := msgtype.MessageType(buf[18])
		b.Reset().Str(mt.String())
		if mt == msgtype.TypeNOTIFICATION && length >= headerLen+2 {
			b.Str(" code=").Uint8(buf[headerLen]).Str(" subcode=").Uint8(buf[headerLen+1])
		}
		out = append(out, b.String())
		buf = buf[length:]
	}
	return out
}

type replayAddr struct{}

func (replayAddr) Network() string { return "replay" }
func (replayAddr) String() string  { return "capture" }
