// Design: docs/architecture/l2tp/cpe-1-pppoe-client.md -- client caller ownership.
// Related: client_protocol_fixture_test.go -- private logical, never PPP grants.

package pppoeclient

import (
	"errors"
	"io"
	"log/slog"
	"reflect"
	"syscall"
	"testing"
	"testing/synctest"

	"github.com/ze-software/ze/internal/component/iface"
	"github.com/ze-software/ze/internal/component/l2tp/ppp"
	pppkernel "github.com/ze-software/ze/internal/component/l2tp/ppp/kernel"
	"github.com/ze-software/ze/internal/component/l2tp/pppoe"
)

// TestClientPreflightRefusalPrecedesDiscovery observes the public dialer's
// actual preflight ordering with a private logical refusal. No public creator
// or Created owner is forged to make the discovery or transport path run.
func TestClientPreflightRefusalPrecedesDiscovery(t *testing.T) {
	previousResolve, previousOpen := resolveSourceInterface, openDiscoverySocket
	t.Cleanup(func() { resolveSourceInterface, openDiscoverySocket = previousResolve, previousOpen })
	var operations []string
	resolveSourceInterface = func(string) (pppoe.InterfaceBinding, error) {
		operations = append(operations, "resolve")
		return pppoe.InterfaceBinding{Device: "test0", Ifindex: 1, HWAddr: testHostMAC}, nil
	}
	openDiscoverySocket = func() (int, error) {
		operations = append(operations, "discovery")
		return -1, errors.New("unexpected discovery allocation")
	}
	cause := errors.New("original backend preflight refused")
	_, err := (&Dialer{}).dialPrepared(iface.PPPoEClientConfig{SourceInterface: "wan", Timing: testDialTiming},
		make(chan struct{}), slog.New(slog.NewTextHandler(io.Discard, nil)), func() (*clientDialPreparation, error) {
			operations = append(operations, "preflight")
			return nil, cause
		})
	if !errors.Is(err, cause) {
		t.Fatalf("preflight cause lost: %v", err)
	}
	if !reflect.DeepEqual(operations, []string{"resolve", "preflight"}) {
		t.Fatalf("allocation preceded preflight: %v", operations)
	}
}

// TestClientReadAttemptReleasesLifecycle proves an opportunity to stop or
// connect AFTER EACH EAGAIN and EINTR, with no clock/scheduler assumption.
// The private callback proves it runs under mu; the immediate TryLock after
// each return proves the owning operation released that lock before retry.
func TestClientReadAttemptReleasesLifecycle(t *testing.T) {
	client := testClientProtocol(&recordingRWC{})
	link := &sessionLink{client: client, stopped: make(chan struct{}), closeTransport: func() error { return nil }}
	sequence := []error{syscall.EAGAIN, syscall.EINTR, io.ErrUnexpectedEOF}
	attempts := 0
	client.read = func([]byte) (int, error) {
		if link.mu.TryLock() {
			link.mu.Unlock()
			t.Error("pair attempt ran outside lifecycle ownership")
		}
		if attempts >= len(sequence) {
			t.Fatal("unbounded retry consumed a fourth attempt")
		}
		result := sequence[attempts]
		attempts++
		return 0, result
	}
	var buffer [32]byte
	for index, want := range sequence {
		_, err := link.readAttempt(buffer[:])
		if !errors.Is(err, want) {
			t.Fatalf("attempt %d swallowed %v: %v", index, want, err)
		}
		if attempts != index+1 {
			t.Fatalf("attempt %d performed multiple pair reads: %d", index, attempts)
		}
		if !link.mu.TryLock() {
			t.Fatalf("attempt %d retained lifecycle lock before retry", index)
		}
		link.mu.Unlock()
	}
	if err := link.Close(); err != nil {
		t.Fatal(err)
	}
	if _, err := link.readAttempt(buffer[:]); !errors.Is(err, io.ErrClosedPipe) {
		t.Fatalf("stopped read: %v", err)
	}
	if attempts != len(sequence) {
		t.Fatal("stopped read reached pair")
	}
}

// TestClientInitialAttachmentFollowsIPCP drives the actual initial phase owner.
// It observes no Connect before both IPCP directions open, then Connect followed
// by the full receive MRU. Each refusal retains its cause and suppresses later
// native operations and result publication. This is logical ordering, not a
// claim that a private fixture attached any real kernel unit.
func TestClientInitialAttachmentFollowsIPCP(t *testing.T) {
	for _, refusal := range []string{"none", "connect", "mru"} {
		t.Run(refusal, func(t *testing.T) {
			synctest.Test(t, func(t *testing.T) {
				w := newConnectedExchange()
				w.unit = w.frames
				client := testClientProtocol(w)
				var operations []string
				cause := errors.New("original native operation refused")
				client.connect = func() error {
					operations = append(operations, "connect")
					if refusal == "connect" {
						return cause
					}
					return nil
				}
				client.mru = func(mru uint16) error {
					operations = append(operations, "mru")
					if mru != ppp.MaxFrameLen {
						t.Errorf("receive MRU = %d", mru)
					}
					if refusal == "mru" {
						return cause
					}
					return nil
				}
				link := &sessionLink{client: client}
				reader := testFrameReader(t, w.frames, nil)
				var result sessionResult
				finished := make(chan error, 1)
				go func() {
					var err error
					result, err = negotiateSession(link, reader, sessionConfig{
						mtu: 1492, lcpRestartTimer: lcpRestartInterval,
						lcpMaxConfigure: lcpConfigureMax, lcpMaxFailure: 5,
					}, make(chan struct{}), slog.New(slog.NewTextHandler(io.Discard, nil)), nil)
					finished <- err
				}()
				request := w.take(t, ppp.ProtoLCP, ppp.LCPConfigureRequest)
				options := exchangePeerOptions(0)
				w.send(ppp.ProtoLCP, ppp.LCPConfigureRequest, 91, options)
				ack := w.take(t, ppp.ProtoLCP, ppp.LCPConfigureAck)
				if !reflect.DeepEqual(ack, exchangeFrame(ppp.ProtoLCP, ppp.LCPConfigureAck, 91, options)) {
					t.Fatalf("peer LCP Ack changed: %x", ack)
				}
				w.send(ppp.ProtoLCP, ppp.LCPConfigureAck, request[3], request[6:])
				if len(operations) != 0 {
					t.Fatalf("attached before IPCP: %v", operations)
				}
				w.openIPCP(t, 0, 0)
				err := <-finished
				want := []string{"connect", "mru"}
				if refusal == "connect" {
					want = []string{"connect"}
				}
				if !reflect.DeepEqual(operations, want) {
					t.Fatalf("operations %v, want %v", operations, want)
				}
				if refusal == "none" {
					if err != nil {
						t.Fatal(err)
					}
					if result.reader != reader {
						t.Fatal("attachment replaced receive owner")
					}
					if result.negMTU != 1400 {
						t.Fatalf("negotiated MTU = %d", result.negMTU)
					}
					return
				}
				if !errors.Is(err, cause) {
					t.Fatalf("original refusal lost: %v", err)
				}
				if result != (sessionResult{}) {
					t.Fatal("refused attachment published a result")
				}
			})
		})
	}
}

// TestClientSplitCleanupRetainsEveryCause proves a channel panic still closes
// the transport and publishes Done, while the unit survives until explicit
// post-join release. A unit-close panic cannot suppress backend cleanup.
func TestClientSplitCleanupRetainsEveryCause(t *testing.T) {
	channelCause := errors.New("channel close")
	transportCause := errors.New("transport close")
	unitCause := errors.New("unit close")
	backendCause := errors.New("backend close")
	var operations []string
	client := testClientProtocol(&recordingRWC{})
	client.channelClose = func() error { operations = append(operations, "channel"); panic(channelCause) }
	client.unitClose = func() error { operations = append(operations, "unit"); panic(unitCause) }
	link := &sessionLink{client: client, stopped: make(chan struct{}), closeTransport: func() error {
		operations = append(operations, "transport")
		panic(transportCause)
	}}
	if err := link.Close(); !errors.Is(err, channelCause) {
		t.Fatalf("channel cause lost: %v", err)
	}
	select {
	case <-link.stopped:
	default:
		t.Fatal("close failure suppressed Done")
	}
	if !reflect.DeepEqual(operations, []string{"channel", "transport"}) {
		t.Fatalf("early unit release: %v", operations)
	}
	link.releaseOwners(client.CloseUnit, func() error {
		operations = append(operations, "backend")
		return backendCause
	})
	err := link.Close()
	for _, cause := range []error{channelCause, transportCause, unitCause, backendCause} {
		if !errors.Is(err, cause) {
			t.Errorf("cleanup lost %v: %v", cause, err)
		}
	}
	if !reflect.DeepEqual(operations, []string{"channel", "transport", "unit", "backend"}) {
		t.Fatalf("close order/once: %v", operations)
	}
}

// TestClientStoppedFamilyOperationsNeverReachThePair supplies both invalid and
// legal modes after stop. Neither family can validate or touch its logical unit.
// The live opposite controls also keep the two independent family gates apart.
func TestClientStoppedFamilyOperationsNeverReachThePair(t *testing.T) {
	client := testClientProtocol(&recordingRWC{})
	var ipv4, ipv6 []pppkernel.NPMode
	client.ipv4 = func(mode pppkernel.NPMode) error { ipv4 = append(ipv4, mode); return nil }
	client.ipv6 = func(mode pppkernel.NPMode) error { ipv6 = append(ipv6, mode); return nil }
	link := &sessionLink{client: client, stopped: make(chan struct{}), closeTransport: func() error { return nil }}
	if err := link.setIPv4Mode(pppkernel.NPModePass); err != nil {
		t.Fatal(err)
	}
	if err := link.setIPv6Mode(pppkernel.NPModeDrop); err != nil {
		t.Fatal(err)
	}
	if err := link.Close(); err != nil {
		t.Fatal(err)
	}
	for _, mode := range []pppkernel.NPMode{pppkernel.NPModeUnspecified, pppkernel.NPModePass, pppkernel.NPModeDrop, 255} {
		if err := link.setIPv4Mode(mode); !errors.Is(err, io.ErrClosedPipe) {
			t.Fatalf("IPv4 after stop: %v", err)
		}
		if err := link.setIPv6Mode(mode); !errors.Is(err, io.ErrClosedPipe) {
			t.Fatalf("IPv6 after stop: %v", err)
		}
	}
	if !reflect.DeepEqual(ipv4, []pppkernel.NPMode{pppkernel.NPModePass}) {
		t.Fatalf("IPv4 reached pair: %v", ipv4)
	}
	if !reflect.DeepEqual(ipv6, []pppkernel.NPMode{pppkernel.NPModeDrop}) {
		t.Fatalf("IPv6 reached pair: %v", ipv6)
	}
}
