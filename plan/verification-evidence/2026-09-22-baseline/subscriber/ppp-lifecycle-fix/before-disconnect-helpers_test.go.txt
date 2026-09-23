package ppp

import (
	"bytes"
	"io"
	"log/slog"
	"net"
	"sync"
	"testing"

	"github.com/ze-software/ze/internal/core/rtproto"
)

// fakeBackend records iface.Backend calls for assertions.
type fakeBackend struct {
	mu             sync.Mutex
	mtuCalls       []mtuCall
	upCalls        []string
	p2pCalls       []p2pCall
	routeAddCalls  []routeCall
	addrRemoves    []addrCall
	routeRemoves   []routeCall
	mtuErr         error
	upErr          error
	addAddrP2PErr  error
	addRouteErr    error
	removeAddrErr  error
	removeRouteErr error
}

type mtuCall struct {
	name string
	mtu  int
}

type p2pCall struct {
	name  string
	local string
	peer  string
}

type addrCall struct {
	name string
	cidr string
}

type routeCall struct {
	name    string
	dest    string
	gateway string
	metric  int
	proto   rtproto.Proto
}

func (f *fakeBackend) SetMTU(name string, mtu int) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.mtuCalls = append(f.mtuCalls, mtuCall{name, mtu})
	return f.mtuErr
}

func (f *fakeBackend) SetAdminUp(name string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.upCalls = append(f.upCalls, name)
	return f.upErr
}

func (f *fakeBackend) AddAddressP2P(name, localCIDR, peerCIDR string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.p2pCalls = append(f.p2pCalls, p2pCall{name, localCIDR, peerCIDR})
	return f.addAddrP2PErr
}

// setAddAddrP2PErr arms the AddAddressP2P failure path. It takes the mutex
// because the driver goroutine reads the field while the test goroutine writes
// it; a bare assignment is a data race the race detector will fail on.
func (f *fakeBackend) setAddAddrP2PErr(err error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.addAddrP2PErr = err
}

func (f *fakeBackend) AddRoute(name, destCIDR, gateway string, metric int, proto rtproto.Proto) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.routeAddCalls = append(f.routeAddCalls, routeCall{name, destCIDR, gateway, metric, proto})
	return f.addRouteErr
}

func (f *fakeBackend) RemoveAddress(name, cidr string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.addrRemoves = append(f.addrRemoves, addrCall{name, cidr})
	return f.removeAddrErr
}

func (f *fakeBackend) RemoveRoute(name, destCIDR, gateway string, metric int, proto rtproto.Proto) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.routeRemoves = append(f.routeRemoves, routeCall{name, destCIDR, gateway, metric, proto})
	return f.removeRouteErr
}

func (f *fakeBackend) MTUCalls() []mtuCall {
	f.mu.Lock()
	defer f.mu.Unlock()
	out := make([]mtuCall, len(f.mtuCalls))
	copy(out, f.mtuCalls)
	return out
}

func (f *fakeBackend) UpCalls() []string {
	f.mu.Lock()
	defer f.mu.Unlock()
	out := make([]string, len(f.upCalls))
	copy(out, f.upCalls)
	return out
}

func (f *fakeBackend) P2PCalls() []p2pCall {
	f.mu.Lock()
	defer f.mu.Unlock()
	out := make([]p2pCall, len(f.p2pCalls))
	copy(out, f.p2pCalls)
	return out
}

func (f *fakeBackend) RouteAddCalls() []routeCall {
	f.mu.Lock()
	defer f.mu.Unlock()
	out := make([]routeCall, len(f.routeAddCalls))
	copy(out, f.routeAddCalls)
	return out
}

func (f *fakeBackend) AddrRemoveCalls() []addrCall {
	f.mu.Lock()
	defer f.mu.Unlock()
	out := make([]addrCall, len(f.addrRemoves))
	copy(out, f.addrRemoves)
	return out
}

func (f *fakeBackend) RouteRemoveCalls() []routeCall {
	f.mu.Lock()
	defer f.mu.Unlock()
	out := make([]routeCall, len(f.routeRemoves))
	copy(out, f.routeRemoves)
	return out
}

// fakeOpsCall records a setMRU invocation.
type fakeOpsCall struct {
	fd  int
	mru uint16
}

func newFakeOps() (pppOps, *[]fakeOpsCall, *sync.Mutex) {
	var mu sync.Mutex
	var calls []fakeOpsCall
	ops := pppOps{
		setMRU: func(fd int, mru uint16) error {
			mu.Lock()
			defer mu.Unlock()
			calls = append(calls, fakeOpsCall{fd, mru})
			return nil
		},
		connect: func(chanFD, unitNum int) error { return nil },
	}
	return ops, &calls, &mu
}

// pipeRegistry maps StartSession.ChanFD ints to the driver-side end
// of a net.Pipe. The test installs a wrap function that returns the
// matching connection.
type pipeRegistry struct {
	mu sync.Mutex
	m  map[int]net.Conn
}

func newPipeRegistry() *pipeRegistry {
	return &pipeRegistry{m: make(map[int]net.Conn)}
}

func (r *pipeRegistry) register(fd int, c net.Conn) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.m[fd] = c
}

func (r *pipeRegistry) wrap(fd int, _ string) io.ReadWriteCloser {
	r.mu.Lock()
	defer r.mu.Unlock()
	c, ok := r.m[fd]
	if !ok {
		panic("BUG: test registered no pipe for fd")
	}
	return c
}

// installPipeRegistry swaps the chan-file wrapper for the test
// duration. Restores production on cleanup.
func installPipeRegistry(t *testing.T, reg *pipeRegistry) {
	t.Helper()
	prev := SetNewChanFileForTest(reg.wrap)
	t.Cleanup(func() { RestoreNewChanFile(prev) })
	prevUnit := SetNewUnitFileForTest(func(int) io.ReadCloser { return nil })
	t.Cleanup(func() { RestoreNewUnitFile(prevUnit) })
}

// pipePair holds the two ends of a net.Pipe.
type pipePair struct {
	driverEnd net.Conn // wrapped by the Driver
	peerEnd   net.Conn // the test peer reads/writes here
}

// newPipePair creates a net.Pipe and registers the driver end under
// the given synthetic fd.
func newPipePair(reg *pipeRegistry, fd int) pipePair {
	a, b := net.Pipe()
	reg.register(fd, a)
	return pipePair{driverEnd: a, peerEnd: b}
}

// scriptedPeer drives a minimal LCP exchange from the peer side.
// Acks the driver's CR; sends its own CR with MRU=1500; consumes
// the driver's CA; then idles until the connection closes.
func scriptedPeer(t *testing.T, conn net.Conn, done chan<- struct{}) {
	t.Helper()
	defer close(done)

	buf := make([]byte, MaxFrameLen)

	n, err := conn.Read(buf)
	if err != nil {
		t.Errorf("peer: read CR: %v", err)
		return
	}
	driverProto, driverPayload, _, err := ParseFrame(buf[:n])
	if err != nil || driverProto != ProtoLCP {
		t.Errorf("peer: bad first frame: proto=0x%04x err=%v", driverProto, err)
		return
	}
	driverCR, err := ParseLCPPacket(driverPayload)
	if err != nil || driverCR.Code != LCPConfigureRequest {
		t.Errorf("peer: expected CR, got code=%d err=%v", driverCR.Code, err)
		return
	}

	out := make([]byte, MaxFrameLen)
	off := WriteFrame(out, 0, ProtoLCP, nil)
	off += WriteLCPPacket(out, off, LCPConfigureAck, driverCR.Identifier, driverCR.Data)
	if _, err := conn.Write(out[:off]); err != nil {
		t.Errorf("peer: write CA: %v", err)
		return
	}

	mruOpt := []byte{LCPOptMRU, 0x04, 0x05, 0xDC}
	off = WriteFrame(out, 0, ProtoLCP, nil)
	off += WriteLCPPacket(out, off, LCPConfigureRequest, 1, mruOpt)
	if _, err := conn.Write(out[:off]); err != nil {
		t.Errorf("peer: write CR: %v", err)
		return
	}

	n, err = conn.Read(buf)
	if err != nil {
		t.Errorf("peer: read driver CA: %v", err)
		return
	}
	_, driverPayload, _, _ = ParseFrame(buf[:n])
	driverCA, err := ParseLCPPacket(driverPayload)
	if err != nil || driverCA.Code != LCPConfigureAck {
		t.Errorf("peer: expected driver CA, got code=%d err=%v", driverCA.Code, err)
		return
	}

	// Idle until the connection closes.
	for {
		_, err := conn.Read(buf)
		if err != nil {
			return
		}
	}
}

// makeTestDriver constructs a Driver with a discard logger, the
// supplied dependencies, and an always-accept auth responder
// goroutine reading d.AuthEventsOut() until the Driver stops.
//
// The responder replaces 6a's StubAuthHook. Tests that exercise
// auth-reject or auth-timeout paths MUST construct the Driver with
// NewDriver directly and drive the auth channel themselves rather
// than call this helper.
func makeTestDriver(backend IfaceBackend, ops pppOps) *Driver {
	return makeTestDriverWithLogger(backend, ops, slog.New(slog.NewTextHandler(io.Discard, nil)))
}

// makeTestDriverWithLogger is makeTestDriver with a caller-supplied logger, so
// a test can capture what the session goroutine logs and observe a gated code
// path whose only externally visible side effect is a log line.
func makeTestDriverWithLogger(backend IfaceBackend, ops pppOps, logger *slog.Logger) *Driver {
	d := newDriver(driverConfig{
		Logger:  logger,
		Backend: backend,
		Ops:     ops,
	})
	go autoAcceptAuth(d)
	return d
}

// captureWriter is a goroutine-safe io.Writer that accumulates everything
// written to it. A session goroutine writes its logs through it while the test
// goroutine snapshots them with String(); both take the same lock, so reads and
// the session's writes never race.
type captureWriter struct {
	mu  sync.Mutex
	buf bytes.Buffer
}

func (w *captureWriter) Write(p []byte) (int, error) {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.buf.Write(p)
}

func (w *captureWriter) String() string {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.buf.String()
}

// autoAcceptAuth reads every EventAuthRequest from d.AuthEventsOut()
// and replies with AuthResponse(accept=true). Exits when the channel
// closes (Driver.Stop). EventAuthSuccess / EventAuthFailure events
// from the session goroutine are consumed and discarded.
func autoAcceptAuth(d *Driver) {
	for ev := range d.AuthEventsOut() {
		req, ok := ev.(EventAuthRequest)
		if !ok {
			continue
		}
		_ = d.AuthResponse(req.TunnelID, req.SessionID, true, "", nil) //nolint:errcheck // ignore teardown race (ErrSessionNotFound) or duplicate-response (ErrAuthResponsePending)
	}
}
