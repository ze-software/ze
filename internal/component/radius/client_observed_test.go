// Design: docs/research/l2tpv2-ze-integration.md -- actual observed RADIUS writes
// Related: client_observed.go -- first write outcome, cancellation and failover
package radius

import (
	"context"
	"encoding/binary"
	"errors"
	"net"
	"sync/atomic"
	"testing"
	"time"
)

// observedWriteServer owns one real UDP reader. Cleanup closes its socket and
// joins it, and the bounded received queue never blocks the request path.
func observedWriteServer(t *testing.T, answer bool) (string, <-chan *Packet) {
	t.Helper()
	conn, err := net.ListenUDP("udp4", &net.UDPAddr{IP: net.IPv4(127, 0, 0, 1)})
	if err != nil {
		t.Fatal(err)
	}
	seen := make(chan *Packet, 16)
	done := make(chan struct{})
	go func() {
		defer close(done)
		buf := make([]byte, MaxPacketLen)
		for {
			n, peer, readErr := conn.ReadFromUDP(buf)
			if readErr != nil {
				return
			}
			packet, decodeErr := Decode(buf[:n])
			if decodeErr != nil {
				continue
			}
			select {
			case seen <- packet:
			case <-t.Context().Done():
				return
			}
			if !answer {
				continue
			}
			response := make([]byte, HeaderLen)
			response[0], response[1] = CodeAccountingResp, packet.Identifier
			binary.BigEndian.PutUint16(response[2:4], HeaderLen)
			authenticator := ResponseAuthenticator(CodeAccountingResp, packet.Identifier, HeaderLen, packet.Authenticator, nil, []byte("observed-write"))
			copy(response[4:], authenticator[:])
			if _, err := conn.WriteToUDP(response, peer); err != nil {
				return
			}
		}
	}()
	t.Cleanup(func() {
		if err := conn.Close(); err != nil {
			t.Error(err)
		}
		select {
		case <-done:
		case <-time.After(5 * time.Second):
			t.Error("observed RADIUS reader did not join")
		}
	})
	return conn.LocalAddr().String(), seen
}

func observedWriteClient(t *testing.T, servers ...string) *Client {
	t.Helper()
	configured := make([]Server, len(servers))
	for i, address := range servers {
		configured[i] = Server{Address: address, SharedKey: []byte("observed-write")}
	}
	client, err := NewClient(ClientConfig{Servers: configured, Timeout: 30 * time.Millisecond, Retries: 1})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := client.Close(); err != nil {
			t.Error(err)
		}
	})
	return client
}

// A real outer write is observed outside the client mutex: a distinct live
// Exchange must finish while the callback is still running. Under the lock
// mutation, the bounded callback returns first, so its worker can then join.
func TestObservedRequestReportsActualWriteOutsideClientLock(t *testing.T) {
	address, seen := observedWriteServer(t, true)
	client := observedWriteClient(t, address)
	client.config.Timeout = time.Second
	var writes atomic.Int32
	var innerDone chan struct{}
	var innerResponse *Packet
	var innerErr error
	var innerID uint8
	response, err := client.SendToServersObserved(t.Context(), &Packet{Code: CodeAccountingReq}, func(write RequestWrite) {
		writes.Add(1)
		if write.Server() != address {
			t.Errorf("write server=%q, want %q", write.Server(), address)
		}
		if write.Bytes() < HeaderLen {
			t.Errorf("actual accepted bytes=%d", write.Bytes())
		}
		if write.Err() != nil {
			t.Errorf("successful actual write=%v", write.Err())
		}

		live, cancelLive := context.WithTimeout(t.Context(), 5*time.Second)
		defer cancelLive()
		innerID = client.NextID()
		innerDone = make(chan struct{})
		go func() {
			defer close(innerDone)
			innerResponse, innerErr = client.Exchange(live, &Packet{Code: CodeAccountingReq, Identifier: innerID}, Server{Address: address, SharedKey: []byte("observed-write")})
		}()
		select {
		case <-innerDone:
		case <-time.After(time.Second):
			// Do not Fatal or join here: the deliberate under-mutex callback cut
			// must be allowed to return, release that mutex and let the worker exit.
			t.Error("observer held the client mutex: live reentrant Exchange could not finish")
		}

		// Preserve the original pre-canceled control as a separate no-write input.
		canceled, cancel := context.WithCancel(t.Context())
		cancel()
		_, canceledErr := client.Exchange(canceled, &Packet{Code: CodeAccountingReq}, Server{Address: address, SharedKey: []byte("observed-write")})
		if !errors.Is(canceledErr, context.Canceled) {
			t.Errorf("canceled observer reentry=%v", canceledErr)
		}
	})
	// Join only after the callback has returned, including its failure path.
	// cancelLive has now run, so a worker blocked by the mutation can unwind.
	if innerDone != nil {
		select {
		case <-innerDone:
		case <-time.After(5 * time.Second):
			t.Fatal("reentrant Exchange did not join after observer returned")
		}
	}
	if err != nil {
		t.Fatal(err)
	}
	if innerDone == nil {
		t.Fatal("write observer did not start its live reentrant Exchange")
	}
	if innerErr != nil {
		t.Fatalf("live observer reentry=%v", innerErr)
	}
	if response == nil || response.Code != CodeAccountingResp {
		t.Fatalf("outer response=%+v", response)
	}
	if innerResponse == nil || innerResponse.Code != CodeAccountingResp {
		t.Fatalf("inner response=%+v", innerResponse)
	}
	if response.Identifier == innerID {
		t.Fatal("live reentry reused the outer response waiter identifier")
	}
	if innerResponse.Identifier != innerID {
		t.Fatal("live reentry received another request's response")
	}
	if writes.Load() != 1 {
		t.Fatalf("write observations=%d", writes.Load())
	}
	received := make(map[uint8]bool, 2)
	for range 2 {
		select {
		case packet := <-seen:
			if received[packet.Identifier] {
				t.Fatalf("duplicate observed request identifier=%d", packet.Identifier)
			}
			received[packet.Identifier] = true
		case <-time.After(5 * time.Second):
			t.Fatal("outer or live reentrant Exchange had no actual datagram")
		}
	}
	if !received[response.Identifier] || !received[innerID] {
		t.Fatal("actual datagrams did not belong to both original requests")
	}
	select {
	case packet := <-seen:
		t.Fatalf("canceled reentry sent another datagram: %+v", packet)
	default:
	}
}

// A pre-canceled operation reaches neither the socket nor the observer. Its
// positive control uses the same live endpoint rather than accepting silence.
func TestObservedRequestCancellationHasNoWriteReceipt(t *testing.T) {
	address, seen := observedWriteServer(t, true)
	client := observedWriteClient(t, address)
	var writes int
	canceled, cancel := context.WithCancel(t.Context())
	cancel()
	_, err := client.SendToServersObserved(canceled, &Packet{Code: CodeAccountingReq}, func(RequestWrite) { writes++ })
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("canceled send=%v", err)
	}
	if writes != 0 {
		t.Fatal("unwritten request fabricated an outcome")
	}
	select {
	case <-seen:
		t.Fatal("pre-canceled request wrote")
	default:
	}
	if _, err := client.SendToServersObserved(t.Context(), &Packet{Code: CodeAccountingReq}, func(RequestWrite) { writes++ }); err != nil {
		t.Fatal(err)
	}
	if writes != 1 {
		t.Fatalf("live control observations=%d", writes)
	}
	select {
	case <-seen:
	case <-time.After(5 * time.Second):
		t.Fatal("live control did not write")
	}
}

// Failover preserves its actual two server writes but reports only the first
// write of the whole request. A timeout is not an absence or a second receipt.
func TestObservedRequestFirstWriteSurvivesFailover(t *testing.T) {
	silent, firstSeen := observedWriteServer(t, false)
	answering, secondSeen := observedWriteServer(t, true)
	client := observedWriteClient(t, silent, answering)
	var writes []RequestWrite
	response, err := client.SendToServersObserved(t.Context(), &Packet{Code: CodeAccountingReq}, func(write RequestWrite) { writes = append(writes, write) })
	if err != nil {
		t.Fatal(err)
	}
	if response.Code != CodeAccountingResp {
		t.Fatalf("response code=%d", response.Code)
	}
	if len(writes) != 1 {
		t.Fatalf("failover observations=%d", len(writes))
	}
	if writes[0].Server() != silent {
		t.Fatalf("first actual server=%q", writes[0].Server())
	}
	for _, seen := range []<-chan *Packet{firstSeen, secondSeen} {
		select {
		case <-seen:
		case <-time.After(5 * time.Second):
			t.Fatal("failover skipped a real endpoint")
		}
	}
}

func TestObservedRequestZeroReceiptIsUnknown(t *testing.T) {
	if (RequestWrite{}).Err() == nil {
		t.Fatal("zero write receipt became success")
	}
}

// observedWriteLockContext makes only the first real writeRequest Err check
// observable. The test cancels its actual context after that observation while
// holding the actual client mutex; no private client ownership is manufactured.
type observedWriteLockContext struct {
	context.Context
	firstErr chan struct{}
	calls    atomic.Int32
}

func (ctx *observedWriteLockContext) Err() error {
	if ctx.calls.Add(1) == 1 {
		close(ctx.firstErr)
		return nil
	}
	return ctx.Context.Err()
}

// A request canceled while waiting for the socket mutex must be refused by the
// post-lock check. A live write afterwards proves the original socket still works.
func TestObservedRequestCancellationAfterClientLockHasNoWriteReceipt(t *testing.T) {
	address, seen := observedWriteServer(t, true)
	client := observedWriteClient(t, address)
	destination, err := net.ResolveUDPAddr("udp4", address)
	if err != nil {
		t.Fatal(err)
	}
	packet := &Packet{Code: CodeAccountingReq, Identifier: client.NextID()}
	var buffer [MaxPacketLen]byte
	length, _, err := encodeRequest(packet, []byte("observed-write"), buffer[:])
	if err != nil {
		t.Fatal(err)
	}
	base, cancel := context.WithCancel(t.Context())
	defer cancel()
	ctx := &observedWriteLockContext{Context: base, firstErr: make(chan struct{})}
	var writes atomic.Int32
	client.mu.Lock()
	locked := true
	unlock := func() {
		if locked {
			locked = false
			client.mu.Unlock()
		}
	}
	defer unlock()
	finished := make(chan error, 1)
	go func() {
		finished <- client.writeRequest(ctx, buffer[:length], destination, func(RequestWrite) { writes.Add(1) })
	}()
	select {
	case <-ctx.firstErr:
	case <-time.After(5 * time.Second):
		// Always cancel/unlock below before joining, even when this gate fails.
		t.Error("writer did not check context before waiting for the client mutex")
	}
	cancel()
	unlock()
	select {
	case writeErr := <-finished:
		if !errors.Is(writeErr, context.Canceled) {
			t.Errorf("post-lock canceled write=%v", writeErr)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("post-lock canceled writer did not join")
	}
	if writes.Load() != 0 {
		t.Fatal("post-lock cancellation fabricated a write receipt")
	}
	select {
	case packet := <-seen:
		t.Fatalf("post-lock canceled request wrote: %+v", packet)
	default:
	}

	packet.Identifier = client.NextID()
	length, _, err = encodeRequest(packet, []byte("observed-write"), buffer[:])
	if err != nil {
		t.Fatal(err)
	}
	if err := client.writeRequest(t.Context(), buffer[:length], destination, func(write RequestWrite) {
		writes.Add(1)
		if write.Server() != address || write.Bytes() != length || write.Err() != nil {
			t.Errorf("live write receipt=%+v", write)
		}
	}); err != nil {
		t.Fatal(err)
	}
	if writes.Load() != 1 {
		t.Fatalf("live control write observations=%d", writes.Load())
	}
	select {
	case received := <-seen:
		if received.Identifier != packet.Identifier {
			t.Fatal("live control observed a canceled request instead")
		}
	case <-time.After(5 * time.Second):
		t.Fatal("live control did not reach its original endpoint")
	}
	select {
	case packet := <-seen:
		t.Fatalf("unexpected extra request: %+v", packet)
	default:
	}
}
