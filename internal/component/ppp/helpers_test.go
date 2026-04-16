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
	mu       sync.Mutex
	mtuCalls []mtuCall
	upCalls  []string
	mtuErr   error
	upErr    error
}

type mtuCall struct {
	name string
	mtu  int
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

// makeTestDriver constructs a Driver with a discard logger and the
// supplied dependencies.
func makeTestDriver(backend IfaceBackend, ops pppOps) *Driver {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	return NewDriver(DriverConfig{
		Logger:   logger,
		AuthHook: StubAuthHook{Logger: logger},
		Backend:  backend,
		Ops:      ops,
	})
}
