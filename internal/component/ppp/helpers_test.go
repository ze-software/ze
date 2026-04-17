package ppp

import (
	"io"
	"log/slog"
	"net"
	"sync"
	"testing"
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

func (f *fakeBackend) AddRoute(name, destCIDR, gateway string, metric int) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.routeAddCalls = append(f.routeAddCalls, routeCall{name, destCIDR, gateway, metric})
	return f.addRouteErr
}

func (f *fakeBackend) RemoveAddress(name, cidr string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.addrRemoves = append(f.addrRemoves, addrCall{name, cidr})
	return f.removeAddrErr
}

func (f *fakeBackend) RemoveRoute(name, destCIDR, gateway string, metric int) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.routeRemoves = append(f.routeRemoves, routeCall{name, destCIDR, gateway, metric})
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
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	d := NewDriver(DriverConfig{
		Logger:  logger,
		Backend: backend,
		Ops:     ops,
	})
	go autoAcceptAuth(d)
	return d
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
