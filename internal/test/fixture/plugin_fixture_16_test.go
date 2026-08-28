package fixture

import (
	"context"
	"encoding/binary"
	"fmt"
	"net"
	"strconv"
	"testing"
	"time"
)

func TestPlugin16L2TPTunnelPeerHandshake(t *testing.T) {
	server, err := net.ListenUDP("udp4", &net.UDPAddr{IP: net.ParseIP("127.0.0.1")})
	if err != nil {
		t.Fatal(err)
	}
	defer server.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	serverErr := make(chan error, 1)
	go func() {
		buffer := make([]byte, 1500)
		if err := server.SetReadDeadline(time.Now().Add(2 * time.Second)); err != nil {
			serverErr <- err
			return
		}
		n, client, err := server.ReadFromUDP(buffer)
		if err != nil {
			serverErr <- err
			return
		}
		request := plugin16L2TPParse(buffer[:n])
		if string(request[7]) != "py-peer" || len(request[9]) != 2 || binary.BigEndian.Uint16(request[9]) != 0x0123 {
			serverErr <- fmt.Errorf("SCCRQ hostname=%q tunnel-id=%x", request[7], request[9])
			return
		}
		body := plugin16L2TPAVP(true, 0, plugin16L2TPU16(2))
		body = append(body, plugin16L2TPAVP(true, 9, plugin16L2TPU16(0x4321))...)
		if _, err := server.WriteToUDP(plugin16L2TPCtrl(0x0123, 0, 0, 1, body), client); err != nil {
			serverErr <- err
			return
		}
		n, client, err = server.ReadFromUDP(buffer)
		if err != nil {
			serverErr <- err
			return
		}
		completion := plugin16L2TPParse(buffer[:n])
		if len(completion[0]) != 2 || binary.BigEndian.Uint16(completion[0]) != 3 || binary.BigEndian.Uint16(buffer[4:6]) != 0x4321 {
			serverErr <- fmt.Errorf("SCCCN message=%x destination=%x", completion[0], buffer[4:6])
			return
		}
		_, err = server.WriteToUDP(plugin16L2TPCtrl(0x0123, 0, 1, 2, nil), client)
		serverErr <- err
		cancel()
	}()

	port := strconv.Itoa(server.LocalAddr().(*net.UDPAddr).Port)
	if err := plugin16L2TPTunnelPeer(ctx, []string{port, "0x0123"}); err != nil {
		t.Fatal(err)
	}
	if err := <-serverErr; err != nil {
		t.Fatal(err)
	}
}
