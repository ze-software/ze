package radius

import (
	"context"
	"encoding/binary"
	"net"
	"testing"
	"time"
)

type mockRADIUSServer struct {
	conn    *net.UDPConn
	addr    string
	handler func(req []byte) []byte
	done    chan struct{}
}

func newMockServer(t *testing.T, sharedKey []byte, code uint8) *mockRADIUSServer {
	t.Helper()
	conn, err := net.ListenUDP("udp4", &net.UDPAddr{IP: net.IPv4(127, 0, 0, 1)})
	if err != nil {
		t.Fatal(err)
	}

	m := &mockRADIUSServer{
		conn: conn,
		addr: conn.LocalAddr().String(),
		done: make(chan struct{}),
	}
	m.handler = func(req []byte) []byte {
		return buildResponse(code, req, sharedKey)
	}

	go m.serve()
	return m
}

func buildResponse(code uint8, req, sharedKey []byte) []byte {
	if len(req) < MinPacketLen {
		return nil
	}
	resp := make([]byte, HeaderLen)
	resp[0] = code
	resp[1] = req[1]
	binary.BigEndian.PutUint16(resp[2:4], HeaderLen)

	var reqAuth [AuthenticatorLen]byte
	copy(reqAuth[:], req[4:4+AuthenticatorLen])
	auth := ResponseAuthenticator(code, req[1], HeaderLen, reqAuth, nil, sharedKey)
	copy(resp[4:4+AuthenticatorLen], auth[:])
	return resp
}

func (m *mockRADIUSServer) serve() {
	defer close(m.done)
	buf := make([]byte, MaxPacketLen)
	for {
		n, from, err := m.conn.ReadFromUDP(buf)
		if err != nil {
			return
		}
		resp := m.handler(buf[:n])
		if resp != nil {
			m.conn.WriteToUDP(resp, from) //nolint:errcheck // test mock best-effort
		}
	}
}

func (m *mockRADIUSServer) close() {
	m.conn.Close() //nolint:errcheck // test cleanup
	<-m.done
}

func closeSilent(c interface{ Close() error }) {
	c.Close() //nolint:errcheck // test cleanup
}

func TestClientExchangeAccept(t *testing.T) {
	sharedKey := []byte("testing123")
	srv := newMockServer(t, sharedKey, CodeAccessAccept)
	defer srv.close()

	client, err := NewClient(ClientConfig{
		Timeout: 2 * time.Second,
		Retries: 2,
	})
	if err != nil {
		t.Fatal(err)
	}
	defer closeSilent(client)

	auth, _ := RandomAuthenticator()
	pkt := &Packet{
		Code:          CodeAccessRequest,
		Identifier:    client.NextID(),
		Authenticator: auth,
		Attrs: []Attr{
			{Type: AttrUserName, Value: AttrString("alice")},
		},
	}

	resp, err := client.Exchange(context.Background(), pkt, sharedKey, srv.addr)
	if err != nil {
		t.Fatal(err)
	}
	if resp.Code != CodeAccessAccept {
		t.Errorf("got code %d, want %d", resp.Code, CodeAccessAccept)
	}
}

func TestClientExchangeReject(t *testing.T) {
	sharedKey := []byte("testing123")
	srv := newMockServer(t, sharedKey, CodeAccessReject)
	defer srv.close()

	client, err := NewClient(ClientConfig{
		Timeout: 2 * time.Second,
		Retries: 2,
	})
	if err != nil {
		t.Fatal(err)
	}
	defer closeSilent(client)

	auth, _ := RandomAuthenticator()
	pkt := &Packet{
		Code:          CodeAccessRequest,
		Identifier:    client.NextID(),
		Authenticator: auth,
	}

	resp, err := client.Exchange(context.Background(), pkt, sharedKey, srv.addr)
	if err != nil {
		t.Fatal(err)
	}
	if resp.Code != CodeAccessReject {
		t.Errorf("got code %d, want %d", resp.Code, CodeAccessReject)
	}
}

func TestClientRetransmit(t *testing.T) {
	sharedKey := []byte("testing123")
	attempts := 0

	conn, err := net.ListenUDP("udp4", &net.UDPAddr{IP: net.IPv4(127, 0, 0, 1)})
	if err != nil {
		t.Fatal(err)
	}
	addr := conn.LocalAddr().String()
	done := make(chan struct{})

	go func() {
		defer close(done)
		buf := make([]byte, MaxPacketLen)
		for {
			n, from, readErr := conn.ReadFromUDP(buf)
			if readErr != nil {
				return
			}
			attempts++
			if attempts < 2 {
				continue
			}
			resp := buildResponse(CodeAccessAccept, buf[:n], sharedKey)
			conn.WriteToUDP(resp, from) //nolint:errcheck // test mock
		}
	}()
	defer func() {
		closeSilent(conn)
		<-done
	}()

	client, err := NewClient(ClientConfig{
		Timeout: 200 * time.Millisecond,
		Retries: 3,
	})
	if err != nil {
		t.Fatal(err)
	}
	defer closeSilent(client)

	auth, _ := RandomAuthenticator()
	pkt := &Packet{
		Code:          CodeAccessRequest,
		Identifier:    client.NextID(),
		Authenticator: auth,
	}

	resp, err := client.Exchange(context.Background(), pkt, sharedKey, addr)
	if err != nil {
		t.Fatal(err)
	}
	if resp.Code != CodeAccessAccept {
		t.Errorf("got code %d, want %d", resp.Code, CodeAccessAccept)
	}
	if attempts < 2 {
		t.Errorf("expected at least 2 attempts, got %d", attempts)
	}
}

func TestClientFailover(t *testing.T) {
	sharedKey := []byte("testing123")

	dead, err := net.ListenUDP("udp4", &net.UDPAddr{IP: net.IPv4(127, 0, 0, 1)})
	if err != nil {
		t.Fatal(err)
	}
	defer closeSilent(dead)

	srv := newMockServer(t, sharedKey, CodeAccessAccept)
	defer srv.close()

	client, err := NewClient(ClientConfig{
		Servers: []Server{
			{Address: dead.LocalAddr().String(), SharedKey: sharedKey},
			{Address: srv.addr, SharedKey: sharedKey},
		},
		Timeout: 200 * time.Millisecond,
		Retries: 1,
	})
	if err != nil {
		t.Fatal(err)
	}
	defer closeSilent(client)

	auth, _ := RandomAuthenticator()
	pkt := &Packet{
		Code:          CodeAccessRequest,
		Authenticator: auth,
		Attrs: []Attr{
			{Type: AttrUserName, Value: AttrString("bob")},
		},
	}

	resp, err := client.SendToServers(context.Background(), pkt)
	if err != nil {
		t.Fatal(err)
	}
	if resp.Code != CodeAccessAccept {
		t.Errorf("got code %d, want %d", resp.Code, CodeAccessAccept)
	}
}

func TestClientAuthenticatorVerify(t *testing.T) {
	sharedKey := []byte("testing123")

	conn, err := net.ListenUDP("udp4", &net.UDPAddr{IP: net.IPv4(127, 0, 0, 1)})
	if err != nil {
		t.Fatal(err)
	}
	addr := conn.LocalAddr().String()
	done := make(chan struct{})
	attempts := 0

	go func() {
		defer close(done)
		buf := make([]byte, MaxPacketLen)
		for {
			n, from, readErr := conn.ReadFromUDP(buf)
			if readErr != nil {
				return
			}
			attempts++
			var resp []byte
			if attempts == 1 {
				resp = buildResponse(CodeAccessAccept, buf[:n], []byte("wrong-key"))
			} else {
				resp = buildResponse(CodeAccessAccept, buf[:n], sharedKey)
			}
			conn.WriteToUDP(resp, from) //nolint:errcheck // test mock
		}
	}()
	defer func() {
		closeSilent(conn)
		<-done
	}()

	client, err := NewClient(ClientConfig{
		Timeout: 500 * time.Millisecond,
		Retries: 3,
	})
	if err != nil {
		t.Fatal(err)
	}
	defer closeSilent(client)

	auth, _ := RandomAuthenticator()
	pkt := &Packet{
		Code:          CodeAccessRequest,
		Identifier:    client.NextID(),
		Authenticator: auth,
	}

	resp, err := client.Exchange(context.Background(), pkt, sharedKey, addr)
	if err != nil {
		t.Fatal(err)
	}
	if resp.Code != CodeAccessAccept {
		t.Errorf("got code %d, want %d", resp.Code, CodeAccessAccept)
	}
	if attempts < 2 {
		t.Errorf("expected at least 2 attempts (first had bad auth), got %d", attempts)
	}
}

func TestClientSourceAddress(t *testing.T) {
	sharedKey := []byte("testing123")

	var receivedFrom *net.UDPAddr
	conn, err := net.ListenUDP("udp4", &net.UDPAddr{IP: net.IPv4(127, 0, 0, 1)})
	if err != nil {
		t.Fatal(err)
	}
	addr := conn.LocalAddr().String()
	done := make(chan struct{})

	go func() {
		defer close(done)
		buf := make([]byte, MaxPacketLen)
		n, from, readErr := conn.ReadFromUDP(buf)
		if readErr != nil {
			return
		}
		receivedFrom = from
		resp := buildResponse(CodeAccessAccept, buf[:n], sharedKey)
		conn.WriteToUDP(resp, from) //nolint:errcheck // test mock
	}()
	defer func() {
		closeSilent(conn)
		<-done
	}()

	client, err := NewClient(ClientConfig{
		Timeout:       2 * time.Second,
		Retries:       1,
		SourceAddress: net.IPv4(127, 0, 0, 1),
	})
	if err != nil {
		t.Fatal(err)
	}
	defer closeSilent(client)

	auth, _ := RandomAuthenticator()
	pkt := &Packet{
		Code:          CodeAccessRequest,
		Identifier:    client.NextID(),
		Authenticator: auth,
	}

	_, err = client.Exchange(context.Background(), pkt, sharedKey, addr)
	if err != nil {
		t.Fatal(err)
	}
	if receivedFrom == nil {
		t.Fatal("no request received")
	}
	if !receivedFrom.IP.Equal(net.IPv4(127, 0, 0, 1)) {
		t.Errorf("source IP: got %v, want 127.0.0.1", receivedFrom.IP)
	}
}

func TestClientSourceAddressBadBind(t *testing.T) {
	_, err := NewClient(ClientConfig{
		Timeout:       time.Second,
		Retries:       1,
		SourceAddress: net.IPv4(198, 51, 100, 1),
	})
	if err == nil {
		t.Fatal("expected bind error for non-local source address")
	}
}

func TestClientTimeout(t *testing.T) {
	dead, err := net.ListenUDP("udp4", &net.UDPAddr{IP: net.IPv4(127, 0, 0, 1)})
	if err != nil {
		t.Fatal(err)
	}
	defer closeSilent(dead)

	client, err := NewClient(ClientConfig{
		Timeout: 100 * time.Millisecond,
		Retries: 2,
	})
	if err != nil {
		t.Fatal(err)
	}
	defer closeSilent(client)

	auth, _ := RandomAuthenticator()
	pkt := &Packet{
		Code:          CodeAccessRequest,
		Identifier:    client.NextID(),
		Authenticator: auth,
	}

	_, err = client.Exchange(context.Background(), pkt, []byte("key"), dead.LocalAddr().String())
	if err == nil {
		t.Fatal("expected timeout error")
	}
}
