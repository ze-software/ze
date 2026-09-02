package reactor

import (
	"bufio"
	"bytes"
	"errors"
	"net"
	"net/netip"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/ze-software/ze/internal/component/bgp/wireu"
	bgpctx "github.com/ze-software/ze/internal/core/bgp/context"
	"github.com/ze-software/ze/internal/core/bgp/msgtype"
	"github.com/ze-software/ze/internal/core/capture"
	"github.com/ze-software/ze/internal/core/clock"
	"github.com/ze-software/ze/pkg/plugin/rpc"
)

// oneMiB is the smallest configurable capture cap, and the bound the rotation
// and stop tests assert against.
const oneMiB = 1 << 20

func testCaptureSettings(dir string) CaptureSettings {
	return CaptureSettings{
		Enabled:     true,
		Directory:   dir,
		MaximumSize: 1,
		OnLimit:     CaptureLimitRotate,
	}
}

// testCapturePeer builds a peer whose capture writes into dir. It goes through
// NewPeerSettings, so the AS numbers and the router-id the capture header
// records are the ones a real peer carries.
func testCapturePeer(dir string) *PeerSettings {
	ps := NewPeerSettings(netip.MustParseAddr("192.0.2.1"), 65000, 65001, 0x01020304)
	ps.Capture = testCaptureSettings(dir)
	return ps
}

func readEvents(t *testing.T, path string) (capture.Header, []*capture.Event) {
	t.Helper()
	f, err := os.Open(path)
	if err != nil {
		t.Fatalf("open capture %s: %v", path, err)
	}
	defer func() { _ = f.Close() }()
	r, hdr, err := capture.NewReader(f)
	if err != nil {
		t.Fatalf("read header of %s: %v", path, err)
	}
	var out []*capture.Event
	for {
		ev, err := r.Next()
		if errors.Is(err, capture.ErrEndOfStream) {
			return hdr, out
		}
		if err != nil {
			t.Fatalf("read event from %s: %v", path, err)
		}
		out = append(out, ev)
	}
}

// VALIDATES: AC-2 -- with capture enabled, each inbound message reaches the JSONL
// file as one event carrying the full wire bytes and its arrival metadata.
// PREVENTS: a tee that records metadata without the bytes, which is the gap this
// spec exists to close.
func TestSessionCaptureWritesEvents(t *testing.T) {
	dir := t.TempDir()
	c, err := newSessionCapture(testCapturePeer(dir), true, clock.RealClock{}, nil)
	if err != nil {
		t.Fatalf("new capture: %v", err)
	}

	c.Start()

	open := []byte{0xff, 0xff, 0xff, 0xff, 0x00, 0x1d, 0x01, 0x04}
	keepalive := []byte{0xff, 0x00, 0x13, 0x04}
	c.recordMessage(time.Unix(1, 0), 1, open, 3, 5)
	c.recordMessage(time.Unix(2, 0), 4, keepalive, 3, 5)
	c.Close()

	hdr, events := readEvents(t, c.Path())
	if hdr.Peer != "192.0.2.1" {
		t.Fatalf("header peer = %q", hdr.Peer)
	}
	if !hdr.Coalesce {
		t.Fatalf("header must record whether the coalesced read path was active")
	}

	var messages []*capture.Event
	for _, ev := range events {
		if ev.Type == capture.EventMessage {
			messages = append(messages, ev)
		}
	}
	if len(messages) != 2 {
		t.Fatalf("got %d message events, want 2", len(messages))
	}
	if !bytes.Equal(messages[0].Data, open) {
		t.Fatalf("first message bytes = %x, want %x", messages[0].Data, open)
	}
	if messages[0].MsgType != 1 || messages[1].MsgType != 4 {
		t.Fatalf("msg types = %d,%d want 1,4", messages[0].MsgType, messages[1].MsgType)
	}
	if messages[0].SourceID != 3 || messages[0].CtxID != 5 {
		t.Fatalf("source-id/ctx-id lost: %+v", messages[0])
	}
	if messages[0].Direction != capture.DirectionReceived {
		t.Fatalf("direction = %q", messages[0].Direction)
	}
	if events[0].Type != capture.EventSession || events[0].Event != capture.SessionCaptureStart {
		t.Fatalf("first event must be capture-start, got %+v", events[0])
	}
	last := events[len(events)-1]
	if last.Type != capture.EventSession || last.Event != capture.SessionCaptureStop {
		t.Fatalf("last event must be capture-stop, got %+v", last)
	}
}

// VALIDATES: AC-6 -- config transaction events reach the capture with their txID.
// PREVENTS: a replay that cannot tell which transaction changed the peer.
func TestSessionCaptureRecordsConfigEvents(t *testing.T) {
	dir := t.TempDir()
	c, err := newSessionCapture(testCapturePeer(dir), false, clock.RealClock{}, nil)
	if err != nil {
		t.Fatalf("new capture: %v", err)
	}
	c.Start()
	c.recordConfig(time.Unix(3, 0), capture.OpModifyPeer, "tx-7", []byte(`{"md5":"s3cret","hold-time":90}`))
	c.Close()

	_, events := readEvents(t, c.Path())
	var cfg *capture.Event
	for _, ev := range events {
		if ev.Type == capture.EventConfig {
			cfg = ev
		}
	}
	if cfg == nil {
		t.Fatal("no config event recorded")
	}
	if cfg.Op != capture.OpModifyPeer || cfg.TxID != "tx-7" {
		t.Fatalf("config event = %+v", cfg)
	}
	if strings.Contains(string(cfg.Payload), "s3cret") {
		t.Fatalf("config payload leaked a secret: %s", cfg.Payload)
	}
	if !strings.Contains(string(cfg.Payload), "90") {
		t.Fatalf("config payload lost its non-secret content: %s", cfg.Payload)
	}
}

// VALIDATES: AC-6 end to end -- a config operation recorded through the REACTOR
// reaches every open capture with its operation name and transaction id.
// PREVENTS: the wiring being dead. TestSessionCaptureRecordsConfigEvents drives
// sc.recordConfig directly, so registerCapture, the live-capture fan-out and the
// txID hand-off could all be broken and it would stay green.
func TestReactorCaptureConfigEventReachesOpenCaptures(t *testing.T) {
	dir := t.TempDir()
	r := &Reactor{clock: clock.RealClock{}}

	if r.CapturesOpen() {
		t.Fatal("a reactor with no capture registered reports one open")
	}
	// A config event with nothing registered must be a no-op, not a panic.
	r.CaptureConfigEvent(capture.OpCommit, "tx-none", []byte(`{"a":1}`))

	first, err := newSessionCapture(testCapturePeer(dir), false, clock.RealClock{}, nil)
	if err != nil {
		t.Fatalf("new capture: %v", err)
	}
	first.Start()

	second := testCapturePeer(dir)
	second.Address = netip.MustParseAddr("192.0.2.2")
	other, err := newSessionCapture(second, false, clock.RealClock{}, nil)
	if err != nil {
		t.Fatalf("new second capture: %v", err)
	}
	other.Start()

	r.registerCapture(first)
	r.registerCapture(other)
	if !r.CapturesOpen() {
		t.Fatal("CapturesOpen is false while two captures are registered")
	}

	r.CaptureConfigEvent(capture.OpModifyPeer, "tx-42", []byte(`{"md5":"s3cret","hold-time":90}`))
	first.Close()
	other.Close()

	// Both peers' files carry it: a config operation is peer-scoped but its
	// effect on the reactor is global.
	for _, sc := range []*sessionCapture{first, other} {
		_, events := readEvents(t, sc.Path())
		var cfg *capture.Event
		for _, ev := range events {
			if ev.Type == capture.EventConfig {
				cfg = ev
			}
		}
		if cfg == nil {
			t.Fatalf("%s: no config event reached the capture", sc.Path())
		}
		if cfg.Op != capture.OpModifyPeer || cfg.TxID != "tx-42" {
			t.Fatalf("%s: op/tx = %q/%q, want %q/%q", sc.Path(), cfg.Op, cfg.TxID, capture.OpModifyPeer, "tx-42")
		}
		if strings.Contains(string(cfg.Payload), "s3cret") {
			t.Fatalf("%s: secret survived the reactor path: %s", sc.Path(), cfg.Payload)
		}
	}

	r.unregisterCapture(first)
	r.unregisterCapture(other)
	if r.CapturesOpen() {
		t.Fatal("CapturesOpen is true after both captures were unregistered")
	}
}

// VALIDATES: an event too large for an EMPTY capture file stops the capture
// instead of rotating for ever.
// PREVENTS: the daemon dying. A reconcile records the whole BGP config tree as
// one line (reactor.go, ReconcilePeersWithJournal), so an event larger than the
// configured cap is reachable in production; before the retry was bounded, each
// turn renamed the live file over the rotated one and recursed until the stack
// ended the process, destroying both generations on the way.
func TestSessionCaptureStopsWhenAnEventCannotEverFit(t *testing.T) {
	dir := t.TempDir()
	c, err := newSessionCapture(testCapturePeer(dir), false, clock.RealClock{}, nil)
	if err != nil {
		t.Fatalf("new capture: %v", err)
	}
	c.Start()

	// One MESSAGE larger than the whole 1 MiB file. It is driven through
	// recordMessage on purpose: WriteConfig shortens an oversized config payload
	// to a marker, so a config event can no longer reach the cycle, and a test
	// built on one would pass with the bound removed. A message carries its
	// bytes by definition and cannot be shortened.
	huge := make([]byte, 2*oneMiB)
	c.recordMessage(time.Unix(1, 0), 2, huge, 0, 0)
	c.recordMessage(time.Unix(2, 0), 4, []byte{0xff, 0x00, 0x13, 0x04}, 0, 0)
	c.Close()

	// The capture stopped rather than looping, and the file it leaves is
	// readable: that is what proves the cycle was bounded, because an unbounded
	// one never returns from Close at all.
	_, events := readEvents(t, c.Path())
	if len(events) == 0 {
		t.Fatal("capture file carries no events at all")
	}
	// The file must be TERMINATED, not merely abandoned: a guard that set
	// sc.stopped without calling terminate would leave the capture without its
	// closing marker, and an operator cannot tell that file from a truncated one.
	var stopped bool
	for _, ev := range events {
		if ev.Type == capture.EventSession && ev.Event == capture.SessionCaptureStop {
			stopped = true
		}
	}
	if !stopped {
		t.Fatal("the stopped capture carries no capture-stop record")
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("read dir: %v", err)
	}
	if len(entries) > 2 {
		t.Fatalf("capture left %d files, want at most 2 (the file and one rotation)", len(entries))
	}
}

// VALIDATES: a new session moves the previous session's capture aside instead of
// truncating it.
// PREVENTS: losing the capture of the session that failed. The peer reconnects
// within seconds of the drop, and the file naming the drop is the one the
// operator was told to ship (user story 1).
func TestSessionCapturePreservesThePreviousSession(t *testing.T) {
	dir := t.TempDir()
	ps := testCapturePeer(dir)

	first, err := newSessionCapture(ps, false, clock.RealClock{}, nil)
	if err != nil {
		t.Fatalf("new capture: %v", err)
	}
	first.Start()
	first.recordMessage(time.Unix(1, 0), 1, []byte{0xde, 0xad, 0xbe, 0xef}, 0, 0)
	first.Close()

	second, err := newSessionCapture(ps, false, clock.RealClock{}, nil)
	if err != nil {
		t.Fatalf("new capture after reconnect: %v", err)
	}
	second.Start()
	second.recordMessage(time.Unix(2, 0), 4, []byte{0xca, 0xfe}, 0, 0)
	second.Close()

	_, rotated := readEvents(t, first.Path()+rotatedSuffix)
	var found bool
	for _, ev := range rotated {
		if ev.Type == capture.EventMessage && bytes.Equal(ev.Data, []byte{0xde, 0xad, 0xbe, 0xef}) {
			found = true
		}
	}
	if !found {
		t.Fatal("the first session's message is gone; the reconnect truncated its capture")
	}
}

// VALIDATES: one capture per file. A second capture is refused while the first
// still holds the path, and the reactor releases the path when a peer is removed.
// PREVENTS: two live captures destroying each other. The path comes from the peer
// ADDRESS, and Peer.Stop only cancels a context (peer.go), so a RemovePeer plus
// AddPeer on one address overlaps them: each one's rotation renames the other's
// live inode into the single `.1` slot, and the session still running ends up
// writing to an unlinked file.
func TestReactorRefusesASecondCaptureOnOnePath(t *testing.T) {
	dir := t.TempDir()
	ps := testCapturePeer(dir)
	r := &Reactor{clock: clock.RealClock{}}

	held := capturePath(ps.Capture, ps.Address)
	if r.captureHeldForPath(held) {
		t.Fatal("an empty reactor reports the path as held")
	}

	first, err := newSessionCapture(ps, false, clock.RealClock{}, nil)
	if err != nil {
		t.Fatalf("new capture: %v", err)
	}
	first.Start()
	r.registerCapture(first)

	if !r.captureHeldForPath(held) {
		t.Fatal("the path is not reported as held while a capture owns it")
	}
	// A different peer's path is unaffected: the guard is per file, not global.
	other := testCapturePeer(dir)
	other.Address = netip.MustParseAddr("192.0.2.9")
	if r.captureHeldForPath(capturePath(other.Capture, other.Address)) {
		t.Fatal("another peer's path is reported as held")
	}

	// Removing the peer releases the path synchronously, which is what stops the
	// next AddPeer from overlapping.
	r.closeCapturesForPeer(ps.Address)
	if r.captureHeldForPath(held) {
		t.Fatal("the path is still held after the peer was removed")
	}
	if _, events := readEvents(t, first.Path()); len(events) == 0 {
		t.Fatal("the closed capture was not terminated cleanly")
	}
}

// VALIDATES: AC-4 -- reaching the size cap rotates once and the daemon keeps
// running; the bound is two files, never an unbounded set.
// PREVENTS: a replay log that can fill a disk.
func TestSessionCaptureRotatesAtLimit(t *testing.T) {
	dir := t.TempDir()
	c, err := newSessionCapture(testCapturePeer(dir), false, clock.RealClock{}, nil)
	if err != nil {
		t.Fatalf("new capture: %v", err)
	}
	c.Start()

	// The smallest configurable cap is 1 MiB, and the test drives the real
	// bound rather than a test-only seam: 4000 kilobyte messages base64 to
	// well past two full files.
	big := make([]byte, 1024)
	for i := range 4000 {
		c.recordMessage(time.Unix(int64(i), 0), 2, big, 0, 0)
	}
	c.Close()

	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("read dir: %v", err)
	}
	if len(entries) != 2 {
		names := make([]string, 0, len(entries))
		for _, e := range entries {
			names = append(names, e.Name())
		}
		t.Fatalf("rotation must leave exactly two files, got %v", names)
	}
	for _, e := range entries {
		info, err := e.Info()
		if err != nil {
			t.Fatalf("stat %s: %v", e.Name(), err)
		}
		if info.Size() > oneMiB {
			t.Fatalf("%s is %d bytes, past the %d-byte bound", e.Name(), info.Size(), oneMiB)
		}
	}
	// The rotated-away file is still a readable capture.
	readEvents(t, c.Path()+rotatedSuffix)
}

// VALIDATES: AC-4 -- on-limit stop leaves exactly one file, closes it cleanly,
// and the daemon keeps running.
// PREVENTS: a "stop" policy that silently keeps writing.
func TestSessionCaptureStopsAtLimit(t *testing.T) {
	dir := t.TempDir()
	ps := testCapturePeer(dir)
	ps.Capture.OnLimit = CaptureLimitStop
	c, err := newSessionCapture(ps, false, clock.RealClock{}, nil)
	if err != nil {
		t.Fatalf("new capture: %v", err)
	}
	c.Start()

	big := make([]byte, 1024)
	for i := range 4000 {
		c.recordMessage(time.Unix(int64(i), 0), 2, big, 0, 0)
	}
	c.Close()

	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("read dir: %v", err)
	}
	if len(entries) != 1 {
		t.Fatalf("stop policy must leave exactly one file, got %d", len(entries))
	}
	info, err := os.Stat(c.Path())
	if err != nil {
		t.Fatalf("stat: %v", err)
	}
	if info.Size() > oneMiB {
		t.Fatalf("file is %d bytes, past the %d-byte bound", info.Size(), oneMiB)
	}
	_, events := readEvents(t, c.Path())
	last := events[len(events)-1]
	if last.Type != capture.EventSession || last.Event != capture.SessionCaptureStop {
		t.Fatalf("a stopped capture must end with capture-stop, got %+v", last)
	}
}

// VALIDATES: writer backpressure sheds events instead of blocking the read
// goroutine, counts what it shed, and records the gap in the stream.
// PREVENTS: a capture that stalls the BGP read loop under load (A-3).
func TestSessionCaptureDropsUnderBackpressure(t *testing.T) {
	dir := t.TempDir()
	dropped := 0
	c, err := newSessionCapture(testCapturePeer(dir), false, clock.RealClock{},
		func() { dropped++ })
	if err != nil {
		t.Fatalf("new capture: %v", err)
	}
	// The writer goroutine is deliberately not started, so the queue fills and
	// the producer must shed rather than block.
	msg := make([]byte, 16)
	total := captureQueueDepth + 64
	for i := range total {
		c.recordMessage(time.Unix(int64(i), 0), 2, msg, 0, 0)
	}
	if c.Drops() == 0 {
		t.Fatal("a full queue must shed events, not block")
	}
	if dropped == 0 {
		t.Fatal("the drop counter hook must fire so the metric moves")
	}
	if int(c.Drops()) != dropped {
		t.Fatalf("drop count %d disagrees with hook count %d", c.Drops(), dropped)
	}
	c.Close()

	_, events := readEvents(t, c.Path())
	var sawDrops bool
	for _, ev := range events {
		if ev.Type == capture.EventSession && ev.Event == capture.SessionDrops {
			sawDrops = true
			if ev.Drops == 0 {
				t.Fatalf("drops event must carry the count: %+v", ev)
			}
		}
	}
	if !sawDrops {
		t.Fatal("a capture that shed events must say so in the stream")
	}
}

// VALIDATES: a capture directory that cannot be created fails at open time with a
// clear error, rather than silently capturing nothing.
// PREVENTS: an operator enabling capture and finding no file and no reason.
func TestSessionCaptureFailsClosedOnBadDirectory(t *testing.T) {
	dir := t.TempDir()
	blocker := filepath.Join(dir, "notadir")
	if err := os.WriteFile(blocker, []byte("x"), 0o600); err != nil {
		t.Fatalf("write blocker: %v", err)
	}
	bad := testCapturePeer(filepath.Join(blocker, "sub"))
	_, err := newSessionCapture(bad, false, clock.RealClock{}, nil)
	if err == nil {
		t.Fatal("expected an error when the capture directory cannot be created")
	}
}

// VALIDATES: an IPv6 peer produces a usable filename.
// PREVENTS: colons in the address breaking the path on any filesystem.
func TestCaptureFileNameIsPathSafe(t *testing.T) {
	name := captureFileName(netip.MustParseAddr("2001:db8::1"))
	if strings.ContainsAny(name, ":/\\") {
		t.Fatalf("filename %q is not path-safe", name)
	}
	if !strings.Contains(name, "2001") {
		t.Fatalf("filename %q must still identify the peer", name)
	}
}

// VALIDATES: AC-1 -- the tee costs one nil check and zero allocations when
// capture is off.
// PREVENTS: an always-on cost on the BGP read path.
// VALIDATES: AC-1 -- with capture disabled, the tee allocates nothing at all on
// the receive path.
// PREVENTS: the disabled-capture cost claim rotting. The two benchmarks beside
// this test REPORT allocations; nothing reads their number, so a per-message
// allocation added to the tee would fail no gate. This asserts it.
func TestSessionTeeCaptureDisabledDoesNotAllocate(t *testing.T) {
	s := &Session{settings: NewPeerSettings(netip.MustParseAddr("192.0.2.1"), 65000, 65001, 1)}
	s.clock = clock.RealClock{}
	wire := make([]byte, 4096)

	if s.captureWriter != nil {
		t.Fatal("a session with no capture configured has a capture writer")
	}
	if got := testing.AllocsPerRun(100, func() { s.teeCapture(2, wire) }); got != 0 {
		t.Fatalf("the disabled tee allocated %v times per message, want 0", got)
	}
}

func BenchmarkSessionTeeCaptureDisabled(b *testing.B) {
	s := &Session{settings: NewPeerSettings(netip.MustParseAddr("192.0.2.1"), 65000, 65001, 1)}
	s.clock = clock.RealClock{}
	wire := make([]byte, 4096)
	b.ReportAllocs()
	for b.Loop() {
		s.teeCapture(2, wire)
	}
}

// VALIDATES: the enabled tee does not allocate per message once the record pool
// is warm.
// PREVENTS: GC pressure proportional to captured message rate.
func BenchmarkSessionTeeCaptureEnabled(b *testing.B) {
	dir := b.TempDir()
	benchPeer := testCapturePeer(dir)
	benchPeer.Capture.MaximumSize = 1024
	c, err := newSessionCapture(benchPeer, false, clock.RealClock{}, nil)
	if err != nil {
		b.Fatalf("new capture: %v", err)
	}
	defer c.Close()

	s := &Session{settings: NewPeerSettings(netip.MustParseAddr("192.0.2.1"), 65000, 65001, 1)}
	s.clock = clock.RealClock{}
	s.captureWriter = c
	wire := make([]byte, 512)
	b.ReportAllocs()
	for b.Loop() {
		s.teeCapture(2, wire)
	}
}

// VALIDATES: the capture maximum-size leaf enforces its 1..1024 megabyte range at
// both ends, so a value YANG would reject is rejected here too.
// PREVENTS: a settings path that bypasses the schema producing a zero-byte cap,
// which is a capture the operator asked for that records nothing.
func TestParseCaptureSettingsBoundaries(t *testing.T) {
	tests := []struct {
		name    string
		size    any
		wantErr bool
		want    uint32
	}{
		{name: "invalid below", size: "0", wantErr: true},
		{name: "last valid low", size: "1", want: 1},
		{name: "last valid high", size: "1024", want: 1024},
		{name: "invalid above", size: "1025", wantErr: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			out := CaptureSettings{Directory: DefaultCaptureDirectory, MaximumSize: DefaultCaptureMaximumSize}
			err := parseCaptureSettings("p1", map[string]any{"maximum-size": tt.size}, &out)
			if tt.wantErr {
				if err == nil {
					t.Fatalf("size %v must be rejected", tt.size)
				}
				return
			}
			if err != nil {
				t.Fatalf("size %v must be accepted: %v", tt.size, err)
			}
			if out.MaximumSize != tt.want {
				t.Fatalf("maximum-size = %d, want %d", out.MaximumSize, tt.want)
			}
		})
	}
}

// VALIDATES: the capture container's leaves reach PeerSettings, including the
// on-limit enum, and an unknown enum value is rejected rather than defaulted.
// PREVENTS: a configured "stop" policy silently behaving as "rotate".
func TestParseCaptureSettingsLeaves(t *testing.T) {
	out := CaptureSettings{Directory: DefaultCaptureDirectory, MaximumSize: DefaultCaptureMaximumSize}
	err := parseCaptureSettings("p1", map[string]any{
		"enabled":      "true",
		"directory":    "/var/tmp/zecap",
		"maximum-size": "7",
		"on-limit":     "stop",
	}, &out)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if !out.Enabled || out.Directory != "/var/tmp/zecap" || out.MaximumSize != 7 || out.OnLimit != CaptureLimitStop {
		t.Fatalf("settings = %+v", out)
	}

	bad := CaptureSettings{Directory: DefaultCaptureDirectory, MaximumSize: DefaultCaptureMaximumSize}
	if err := parseCaptureSettings("p1", map[string]any{"on-limit": "truncate"}, &bad); err == nil {
		t.Fatal("an unknown on-limit value must be rejected")
	}
}

// VALIDATES: capture defaults in NewPeerSettings match the YANG defaults, and a
// peer that configures nothing has capture off.
// PREVENTS: the two default sites drifting apart.
func TestPeerSettingsCaptureDefaults(t *testing.T) {
	ps := NewPeerSettings(netip.MustParseAddr("192.0.2.1"), 65000, 65001, 1)
	if ps.Capture.Enabled {
		t.Fatal("capture must be off by default")
	}
	if ps.Capture.Directory != DefaultCaptureDirectory {
		t.Fatalf("default directory = %q", ps.Capture.Directory)
	}
	if ps.Capture.MaximumSize != DefaultCaptureMaximumSize {
		t.Fatalf("default maximum-size = %d", ps.Capture.MaximumSize)
	}
	if ps.Capture.OnLimit != CaptureLimitRotate {
		t.Fatalf("default on-limit = %d", ps.Capture.OnLimit)
	}
}

// VALIDATES: an enabled capture with no directory or a zero cap is refused, so
// the feature fails closed rather than appearing to run.
// PREVENTS: an operator enabling capture and finding no file and no reason.
func TestParseCaptureSettingsFailsClosed(t *testing.T) {
	empty := CaptureSettings{}
	if err := parseCaptureSettings("p1", map[string]any{"enabled": "true"}, &empty); err == nil {
		t.Fatal("an enabled capture with no directory and no cap must be refused")
	}
}

// captureBothPaths feeds the same wire bytes through one read path and returns
// the message events the capture recorded. coalesce selects which of the two
// real read functions drives the session, so the two calls differ in nothing but
// the path under test.
func captureBothPaths(t *testing.T, dir string, coalesce bool, msgs ...[]byte) []*capture.Event {
	t.Helper()
	settings := NewPeerSettings(netip.MustParseAddr("192.0.2.1"), 65001, 65002, 0x01020301)
	settings.Capture = testCaptureSettings(dir)
	session := NewSession(settings)
	session.coalesceEnabled = coalesce
	session.onMessageReceived = func(
		_ netip.Addr, _ msgtype.MessageType, _ []byte,
		_ *wireu.WireUpdate, _ bgpctx.ContextID, _ rpc.MessageDirection,
		_ BufHandle, _ map[string]any, _ string,
	) bool {
		return false
	}

	sc, err := newSessionCapture(settings, coalesce, clock.RealClock{}, nil)
	if err != nil {
		t.Fatalf("new capture: %v", err)
	}
	sc.Start()
	session.captureWriter = sc

	server, client := net.Pipe()
	defer func() { _ = server.Close() }()
	defer func() { _ = client.Close() }()
	go writeAllAndClose(client, msgs...)

	reader := bufio.NewReaderSize(server, 65536)
	for range len(msgs) + 1 {
		var readErr error
		if coalesce {
			readErr = session.readAndProcessCoalesced(server, reader)
		} else {
			readErr = session.readAndProcessMessage(server, reader)
		}
		if readErr != nil {
			break
		}
	}
	sc.Close()

	_, events := readEvents(t, sc.Path())
	var messages []*capture.Event
	for _, ev := range events {
		if ev.Type == capture.EventMessage {
			messages = append(messages, ev)
		}
	}
	return messages
}

// VALIDATES: AC-8 -- capture with coalescing on and off produces identical
// streams for identical input.
// PREVENTS: a tee placed after coalescing, which would record one synthetic
// UPDATE the peer never sent instead of the several it did.
func TestSessionCaptureIdenticalAcrossReadPaths(t *testing.T) {
	attrs := sampleAttrs()
	msgs := [][]byte{
		buildUpdateMsg(buildUpdateBody(attrs, []byte{24, 10, 1, 1})),
		buildUpdateMsg(buildUpdateBody(attrs, []byte{24, 10, 1, 2})),
		buildUpdateMsg(buildUpdateBody(attrs, []byte{24, 10, 1, 3})),
	}

	coalesced := captureBothPaths(t, t.TempDir(), true, msgs...)
	standard := captureBothPaths(t, t.TempDir(), false, msgs...)

	if len(coalesced) != len(msgs) || len(standard) != len(msgs) {
		t.Fatalf("message counts differ: coalesced=%d standard=%d want %d",
			len(coalesced), len(standard), len(msgs))
	}
	for i := range msgs {
		if !bytes.Equal(coalesced[i].Data, msgs[i]) {
			t.Fatalf("coalesced path captured message %d as %x, want %x", i, coalesced[i].Data, msgs[i])
		}
		if !bytes.Equal(standard[i].Data, coalesced[i].Data) {
			t.Fatalf("the two read paths captured different bytes at message %d", i)
		}
		if standard[i].MsgType != coalesced[i].MsgType || standard[i].Len != coalesced[i].Len {
			t.Fatalf("the two read paths captured different metadata at message %d", i)
		}
	}
}

// VALIDATES: AC-7 -- a malformed UPDATE that RFC 7606 handling rewrites or
// short-circuits is captured as the peer sent it, byte for byte.
// PREVENTS: capturing the post-enforcement form, which would make the capture
// useless for exactly the bug it exists to reproduce.
func TestSessionCaptureRecordsPreEnforcementBytes(t *testing.T) {
	// ORIGIN with length 2 is malformed: RFC 7606 Section 7.1 makes it
	// treat-as-withdraw, which rewrites the UPDATE before any observer sees it.
	badAttrs := []byte{
		0x40, 0x01, 0x02, 0x00, 0x00, // ORIGIN, length 2 (must be 1)
		0x40, 0x02, 0x00,
		0x40, 0x03, 0x04, 0x0a, 0x00, 0x00, 0x01,
	}
	msg := buildUpdateMsg(buildUpdateBody(badAttrs, []byte{24, 10, 1, 1}))

	for _, coalesce := range []bool{false, true} {
		name := "standard-path"
		if coalesce {
			name = "coalesced-path"
		}
		t.Run(name, func(t *testing.T) {
			messages := captureBothPaths(t, t.TempDir(), coalesce, msg)
			if len(messages) != 1 {
				t.Fatalf("got %d captured messages, want 1", len(messages))
			}
			if !bytes.Equal(messages[0].Data, msg) {
				t.Fatalf("captured %x, want the unmodified wire bytes %x", messages[0].Data, msg)
			}
		})
	}
}
