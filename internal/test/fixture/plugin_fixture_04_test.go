package fixture

import (
	"context"
	"encoding/binary"
	"net"
	"strconv"
	"testing"
	"time"
)

func TestPluginFixture04BMPClientFrames(t *testing.T) {
	initiation := bmpInitiation04(true)
	if initiation[0] != 3 || initiation[5] != 4 || int(binary.BigEndian.Uint32(initiation[1:5])) != len(initiation) {
		t.Fatalf("invalid initiation header: %x", initiation[:6])
	}
	peerUp := bmpPeerUp04()
	if peerUp[0] != 3 || peerUp[5] != 3 || len(peerUp) != 6+42+16+4+29+29 {
		t.Fatalf("invalid peer-up frame: type=%d len=%d", peerUp[5], len(peerUp))
	}
	route := bmpRouteMonitoring04()
	if route[5] != 0 || !validateMonitoring04(route[6:], false) {
		t.Fatalf("invalid route-monitoring frame: %x", route)
	}
	peerDown := bmpPeerDown04()
	if peerDown[5] != 2 || len(peerDown) != 6+42+1 || peerDown[len(peerDown)-1] != 5 {
		t.Fatalf("invalid peer-down frame: %x", peerDown)
	}
}

func TestPluginFixture04CollectorValidators(t *testing.T) {
	update := bgpUpdate04()
	mirror := make([]byte, 42+4+len(update))
	binary.BigEndian.PutUint16(mirror[42:44], 0)
	binary.BigEndian.PutUint16(mirror[44:46], uint16(len(update)))
	copy(mirror[46:], update)
	if !validateMirroring04(mirror) {
		t.Fatal("valid route-mirroring payload rejected")
	}

	realOpen := func(asn uint16) []byte {
		open := append(bgpOpen04(asn), 0)
		binary.BigEndian.PutUint16(open[16:18], uint16(len(open)))
		return open
	}
	peerUp := make([]byte, 42+16+4)
	peerUp = append(peerUp, realOpen(65000)...)
	peerUp = append(peerUp, realOpen(65001)...)
	if !validatePeerUp04(peerUp) {
		t.Fatal("real peer-up OPEN payloads rejected")
	}
	peerUp[len(peerUp)-1] = 1
	binary.BigEndian.PutUint16(peerUp[42+16+4+30+16:42+16+4+30+18], 29)
	if validatePeerUp04(peerUp) {
		t.Fatal("malformed received OPEN accepted")
	}
}

func TestPluginFixture04CollectorDriver(t *testing.T) {
	t.Chdir(t.TempDir())
	reservation, err := (&net.ListenConfig{}).Listen(t.Context(), "tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	address, ok := reservation.Addr().(*net.TCPAddr)
	if !ok {
		t.Fatalf("a tcp listener answered %T, want *net.TCPAddr", reservation.Addr())
	}
	port := address.Port
	if err := reservation.Close(); err != nil {
		t.Fatal(err)
	}
	done := make(chan error, 1)
	go func() {
		done <- bmpCollector04("monitoring")(context.Background(), []string{strconv.Itoa(port)})
	}()
	var conn net.Conn
	deadline := time.Now().Add(5 * time.Second)
	for conn == nil && time.Now().Before(deadline) {
		dialer := net.Dialer{Timeout: 100 * time.Millisecond}
		conn, _ = dialer.DialContext(t.Context(), "tcp", "127.0.0.1:"+strconv.Itoa(port))
	}
	if conn == nil {
		t.Fatal("collector did not listen")
	}
	if _, err := conn.Write(bmpRouteMonitoring04()); err != nil {
		t.Fatal(err)
	}
	if err := conn.Close(); err != nil {
		t.Fatal(err)
	}
	select {
	case err := <-done:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("collector did not finish")
	}
}
