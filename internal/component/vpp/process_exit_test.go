// Design: docs/research/vpp-deployment-reference.md -- actual process-exit proof
package vpp

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"sync"
	"testing"

	"go.fd.io/govpp/api"
	"go.fd.io/govpp/binapi/memclnt"
)

func observeProcessExits(t *testing.T) func() []ProcessExit {
	t.Helper()
	name := fmt.Sprintf("test-exit-%d", nextConnectorIdentity())
	var mu sync.Mutex
	var proofs []ProcessExit
	if err := RegisterProcessExitObserver(name, func(proof ProcessExit) { mu.Lock(); proofs = append(proofs, proof); mu.Unlock() }); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { processObservers.Lock(); delete(processObservers.byName, name); processObservers.Unlock() })
	return func() []ProcessExit { mu.Lock(); defer mu.Unlock(); return append([]ProcessExit(nil), proofs...) }
}

// Logical Covers remains exact. Empty, foreign and future epochs refuse;
// closing a transport or completing a logical wait cannot create physical exit.
func TestVPPProcessExitProofIsExact(t *testing.T) {
	first, second := NewConnector("first"), NewConnector("second")
	proof := exitedProcess{connector: first.Identity(), generation: 7}
	if !proof.Covers(first.Identity(), 7) {
		t.Fatal("exact owner refused")
	}
	for _, item := range []struct{ identity, generation uint64 }{{0, 7}, {first.Identity(), 0}, {second.Identity(), 7}, {first.Identity(), 6}, {first.Identity(), 8}} {
		if proof.Covers(item.identity, item.generation) {
			t.Fatalf("foreign coverage: %+v", item)
		}
	}
	if (exitedProcess{}).Covers(first.Identity(), 7) {
		t.Fatal("empty proof accepted")
	}
	proofs := observeProcessExits(t)
	first.generation = 7
	first.Close()
	if len(proofs()) != 0 {
		t.Fatal("ordinary Close retired a process")
	}
	first.retireManagedConnection(managedConnection{connector: first.Identity(), generation: 7})
	if len(proofs()) != 0 {
		t.Fatal("logical child wait manufactured physical peer death")
	}
}

// API loss must kill and reap the exact supervised child before returning.
// Its independent physical observer proves death. The child blocks on an owned
// pipe after readiness.
func TestVPPManagedConnectionLossReapsChild(t *testing.T) {
	executable, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	cmd := exec.CommandContext(t.Context(), executable, "-test.run=^TestVPPManagedWaitChild$", "--", "vpp-owned-wait-child")
	out, err := cmd.StdoutPipe()
	if err != nil {
		t.Fatal(err)
	}
	input, err := cmd.StdinPipe()
	if err != nil {
		t.Fatal(err)
	}
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := input.Close(); err != nil {
			t.Log(err)
		}
		if cmd.ProcessState == nil {
			if err := cmd.Process.Kill(); err != nil {
				t.Log(err)
			}
			if err := cmd.Wait(); err != nil {
				t.Log(err)
			}
		}
	})
	line, err := bufio.NewReader(out).ReadString('\n')
	if err != nil || line != "READY\n" {
		t.Fatalf("child readiness = %q, %v", line, err)
	}
	want := errors.New("API connection lost")
	loss := make(chan error, 1)
	loss <- want
	if err := waitVPPConnection(t.Context(), cmd, loss); !errors.Is(err, want) {
		t.Fatalf("wait = %v", err)
	}
	if cmd.ProcessState == nil {
		t.Fatal("managed wait returned before reaping the child")
	}
}

// The helper is a bounded test-owned child lifetime. It has no external state
// and remains blocked only while its parent owns the input pipe.
func TestVPPManagedWaitChild(t *testing.T) {
	if os.Args[len(os.Args)-1] != "vpp-owned-wait-child" {
		return
	}
	if _, err := os.Stdout.WriteString("READY\n"); err != nil {
		t.Fatal(err)
	}
	var value [1]byte
	if _, err := os.Stdin.Read(value[:]); err != nil {
		t.Fatal(err)
	}
}

// External VPP has no child owned here. API failure ends that connection's run
// without claiming that the external process died or waiting for shutdown.
func TestVPPExternalConnectionLossHasNoExitProof(t *testing.T) {
	proofs := observeProcessExits(t)
	want := errors.New("external API lost")
	loss := make(chan error, 1)
	loss <- want
	if err := waitVPPConnection(context.Background(), nil, loss); !errors.Is(err, want) {
		t.Fatalf("external wait = %v", err)
	}
	c := NewConnector("external")
	c.generation = 7
	c.Close()
	if len(proofs()) != 0 {
		t.Fatal("external loss manufactured process death")
	}
}

type processIdentityStream struct {
	ctx    context.Context
	pid    uint32
	closed bool
}

func (s *processIdentityStream) Context() context.Context  { return s.ctx }
func (s *processIdentityStream) SendMsg(api.Message) error { return nil }
func (s *processIdentityStream) RecvMsg() (api.Message, error) {
	return &memclnt.ControlPingReply{VpePID: s.pid}, nil
}
func (s *processIdentityStream) Close() error { s.closed = true; return nil }

// Neither formerly matching nor foreign claimed PIDs can repair a transport
// with no retained physical witness. The exact-server positive lives in
// process_managed_test.go and actual child/socket proof lives in the Linux test.
func TestVPPManagedProcessRejectsUnwitnessedClaimedPID(t *testing.T) {
	for _, pid := range []uint32{41, 42} {
		t.Run(fmt.Sprint(pid), func(t *testing.T) {
			c := NewConnector("unused")
			stream := &processIdentityStream{ctx: t.Context(), pid: pid}
			conn := &recordingConnection{open: func(context.Context) (api.Stream, error) { return stream, nil }}
			c.conn, c.connected, c.generation = conn, true, 7
			child, _ := newPeerTestWitness(t, 41)
			owned, err := c.bindManagedProcess(t.Context(), child)
			if err == nil || owned != (managedConnection{}) {
				t.Fatalf("unwitnessed claimed PID acquired receipt: %+v,%v", owned, err)
			}
			if conn.streams.Load() != 0 {
				t.Fatal("untrusted claimed PID was consulted")
			}
			if stream.closed {
				t.Fatal("binding opened and closed an identity-claim stream")
			}
		})
	}
}
