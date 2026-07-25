package cli

import (
	"net"
	"sync"
	"testing"
	"time"

	"github.com/ze-software/ze/internal/component/radius"
)

func TestMockRADIUSConcurrent(t *testing.T) {
	rad, err := zeTestNewMockRADIUS("127.0.0.1:0", []byte("test-secret"), 0)
	if err != nil {
		t.Fatalf("zeTestNewMockRADIUS: %v", err)
	}
	ctx := t.Context()
	go rad.serve(ctx)
	defer rad.shutdown()

	addr := rad.addr()
	udpAddr, err := net.ResolveUDPAddr("udp4", addr)
	if err != nil {
		t.Fatalf("resolve: %v", err)
	}

	const workers = 50
	const requestsPerWorker = 4

	var wg sync.WaitGroup
	var mu sync.Mutex
	var errors []string

	for w := range workers {
		wg.Add(1)
		go func(workerID int) {
			defer wg.Done()
			conn, err := net.DialUDP("udp4", nil, udpAddr)
			if err != nil {
				mu.Lock()
				errors = append(errors, err.Error())
				mu.Unlock()
				return
			}
			defer func() {
				if err := conn.Close(); err != nil {
					t.Logf("close: %v", err)
				}
			}()

			for r := range requestsPerWorker {
				pkt := &radius.Packet{
					Code:       radius.CodeAccessRequest,
					Identifier: uint8(r),
					Attrs: []radius.Attr{
						{Type: radius.AttrUserName, Value: []byte("test-user")},
					},
				}
				auth, err := radius.RandomAuthenticator()
				if err != nil {
					mu.Lock()
					errors = append(errors, err.Error())
					mu.Unlock()
					return
				}
				pkt.Authenticator = auth

				var buf [radius.MaxPacketLen]byte
				n, err := pkt.EncodeTo(buf[:], 0)
				if err != nil {
					mu.Lock()
					errors = append(errors, err.Error())
					mu.Unlock()
					return
				}

				if _, err := conn.Write(buf[:n]); err != nil {
					mu.Lock()
					errors = append(errors, err.Error())
					mu.Unlock()
					return
				}

				if err := conn.SetReadDeadline(time.Now().Add(2 * time.Second)); err != nil {
					mu.Lock()
					errors = append(errors, err.Error())
					mu.Unlock()
					return
				}
				var resp [radius.MaxPacketLen]byte
				rn, err := conn.Read(resp[:])
				if err != nil {
					mu.Lock()
					errors = append(errors, err.Error())
					mu.Unlock()
					return
				}
				if rn < radius.HeaderLen {
					mu.Lock()
					errors = append(errors, "response too short")
					mu.Unlock()
					return
				}
				if resp[0] != radius.CodeAccessAccept {
					mu.Lock()
					errors = append(errors, "expected Access-Accept")
					mu.Unlock()
					return
				}
				_ = workerID
			}
		}(w)
	}
	wg.Wait()

	if len(errors) > 0 {
		t.Fatalf("errors: %v", errors)
	}

	expectedAuth := int64(workers * requestsPerWorker)
	if got := rad.authCount.Load(); got != expectedAuth {
		t.Errorf("auth count = %d, want %d", got, expectedAuth)
	}
}

func TestMockRADIUSAccounting(t *testing.T) {
	rad, err := zeTestNewMockRADIUS("127.0.0.1:0", []byte("test-secret"), 0)
	if err != nil {
		t.Fatalf("zeTestNewMockRADIUS: %v", err)
	}
	ctx := t.Context()
	go rad.serve(ctx)
	defer rad.shutdown()

	udpAddr, err := net.ResolveUDPAddr("udp4", rad.addr())
	if err != nil {
		t.Fatalf("resolve: %v", err)
	}

	conn, err := net.DialUDP("udp4", nil, udpAddr)
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	defer func() {
		if err := conn.Close(); err != nil {
			t.Logf("close: %v", err)
		}
	}()

	sendAcct := func(statusType uint32, id uint8) {
		pkt := &radius.Packet{
			Code:       radius.CodeAccountingReq,
			Identifier: id,
			Attrs: []radius.Attr{
				{Type: radius.AttrAcctStatusType, Value: radius.AttrUint32(statusType)},
				{Type: radius.AttrAcctSessionID, Value: []byte("test-session")},
			},
		}

		var buf [radius.MaxPacketLen]byte
		n, err := pkt.EncodeTo(buf[:], 0)
		if err != nil {
			t.Fatalf("encode: %v", err)
		}
		auth := radius.AccountingRequestAuth(buf[:n], n, []byte("test-secret"))
		copy(buf[4:4+radius.AuthenticatorLen], auth[:])

		if _, err := conn.Write(buf[:n]); err != nil {
			t.Fatalf("write: %v", err)
		}

		if err := conn.SetReadDeadline(time.Now().Add(2 * time.Second)); err != nil {
			t.Fatalf("deadline: %v", err)
		}
		var resp [radius.MaxPacketLen]byte
		rn, readErr := conn.Read(resp[:])
		if readErr != nil {
			t.Fatalf("read: %v", readErr)
		}
		if rn < radius.HeaderLen || resp[0] != radius.CodeAccountingResp {
			t.Fatalf("unexpected response: code=%d len=%d", resp[0], rn)
		}
	}

	sendAcct(radius.AcctStatusStart, 1)
	sendAcct(radius.AcctStatusInterimUpdate, 2)
	sendAcct(radius.AcctStatusStop, 3)

	time.Sleep(50 * time.Millisecond)

	if got := rad.acctStarts.Load(); got != 1 {
		t.Errorf("acct starts = %d, want 1", got)
	}
	if got := rad.acctStops.Load(); got != 1 {
		t.Errorf("acct stops = %d, want 1", got)
	}
	if got := rad.acctInterims.Load(); got != 1 {
		t.Errorf("acct interims = %d, want 1", got)
	}
}

func TestMockRADIUSLatency(t *testing.T) {
	latency := 100 * time.Millisecond
	rad, err := zeTestNewMockRADIUS("127.0.0.1:0", []byte("test-secret"), latency)
	if err != nil {
		t.Fatalf("zeTestNewMockRADIUS: %v", err)
	}
	ctx := t.Context()
	go rad.serve(ctx)
	defer rad.shutdown()

	udpAddr, err := net.ResolveUDPAddr("udp4", rad.addr())
	if err != nil {
		t.Fatalf("resolve: %v", err)
	}

	conn, err := net.DialUDP("udp4", nil, udpAddr)
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	defer func() {
		if err := conn.Close(); err != nil {
			t.Logf("close: %v", err)
		}
	}()

	pkt := &radius.Packet{
		Code:       radius.CodeAccessRequest,
		Identifier: 1,
		Attrs:      []radius.Attr{{Type: radius.AttrUserName, Value: []byte("user")}},
	}
	auth, err := radius.RandomAuthenticator()
	if err != nil {
		t.Fatal(err)
	}
	pkt.Authenticator = auth

	var buf [radius.MaxPacketLen]byte
	n, err := pkt.EncodeTo(buf[:], 0)
	if err != nil {
		t.Fatal(err)
	}

	start := time.Now()
	if _, err := conn.Write(buf[:n]); err != nil {
		t.Fatal(err)
	}
	if err := conn.SetReadDeadline(time.Now().Add(2 * time.Second)); err != nil {
		t.Fatal(err)
	}
	var resp [radius.MaxPacketLen]byte
	if _, err := conn.Read(resp[:]); err != nil {
		t.Fatal(err)
	}
	elapsed := time.Since(start)

	if elapsed < latency {
		t.Errorf("response too fast: %v < %v", elapsed, latency)
	}
}
