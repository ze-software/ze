package fixture

import (
	"context"
	"encoding/binary"
	"errors"
	"fmt"
	"net"
	"strconv"
	"syscall"
	"time"

	"github.com/ze-software/ze/pkg/plugin/sdk"
)

func waitForDaemonFixture(ctx context.Context, _ []string) error {
	_, err := waitDaemon(ctx, 200)
	return err
}

func parsePort(args []string, index int) (int, error) {
	if len(args) <= index {
		return 0, fmt.Errorf("port argument %d missing", index+1)
	}
	port, err := strconv.Atoi(args[index])
	if err != nil || port < 1 || port > 65535 {
		return 0, fmt.Errorf("invalid port %q", args[index])
	}
	return port, nil
}

func listenUDP(port int) (*net.UDPConn, error) {
	listenConfig := net.ListenConfig{
		Control: func(_, _ string, raw syscall.RawConn) error {
			var optionErr error
			controlErr := raw.Control(func(fd uintptr) {
				optionErr = syscall.SetsockoptInt(int(fd), syscall.SOL_SOCKET, syscall.SO_REUSEADDR, 1)
			})
			return errors.Join(controlErr, optionErr)
		},
	}
	packetConn, err := listenConfig.ListenPacket(
		context.Background(),
		"udp4",
		net.JoinHostPort("127.0.0.1", strconv.Itoa(port)),
	)
	if err != nil {
		return nil, err
	}
	udpConn, ok := packetConn.(*net.UDPConn)
	if !ok {
		_ = packetConn.Close()
		return nil, fmt.Errorf("UDP listener has type %T", packetConn)
	}
	return udpConn, nil
}

func flowExportShow(ctx context.Context, _ []string) error {
	type collector struct {
		Name     string `json:"name"`
		Address  string `json:"address"`
		Protocol string `json:"protocol"`
	}
	return Observe(ctx, "flow-export-show-test", sdk.Registration{}, func(ctx context.Context, p *sdk.Plugin) error {
		var collectors []collector
		if !Poll(ctx, 100, 100*time.Millisecond, func() bool {
			collectors = nil
			status, err := Dispatch(ctx, p, "show flow export", &collectors)
			return err == nil && status == statusDone && len(collectors) >= 1
		}) {
			return fmt.Errorf("no collector ever appeared in `show flow export`")
		}
		got := collectors[0]
		if got.Name != "c1" {
			return fmt.Errorf("collector name=%q want %q", got.Name, "c1")
		}
		if got.Address != addrLoopback {
			return fmt.Errorf("collector address=%q want %q", got.Address, "127.0.0.1")
		}
		if got.Protocol != "sflow" {
			return fmt.Errorf("collector protocol=%q want %q", got.Protocol, "sflow")
		}
		return nil
	})
}

func receiveOne(ctx context.Context, name string, args []string, check func([]byte) error) error {
	port, err := parsePort(args, 0)
	if err != nil {
		return err
	}
	sock, err := listenUDP(port)
	if err != nil {
		return err
	}
	defer sock.Close() //nolint:errcheck // fixture teardown
	_ = sock.SetReadDeadline(time.Now().Add(8 * time.Second))
	return Observe(ctx, name, sdk.Registration{}, func(context.Context, *sdk.Plugin) error {
		data := make([]byte, 65535)
		n, _, readErr := sock.ReadFromUDP(data)
		if readErr != nil {
			return readErr
		}
		return check(data[:n])
	})
}

func ipfixReceiver(ctx context.Context, args []string) error {
	return receiveOne(ctx, "ipfix-receiver", args, func(data []byte) error {
		if len(data) < 20 {
			return fmt.Errorf("IPFIX datagram too short: %d bytes", len(data))
		}
		if version := binary.BigEndian.Uint16(data[:2]); version != 10 {
			return fmt.Errorf("IPFIX version=%#06x want 0x000a", version)
		}
		if setID := binary.BigEndian.Uint16(data[16:18]); setID != 2 {
			return fmt.Errorf("first Set id=%d want 2 (template)", setID)
		}
		return nil
	})
}

func netflow9Receiver(ctx context.Context, args []string) error {
	return receiveOne(ctx, "netflow9-receiver", args, func(data []byte) error {
		if len(data) < 24 {
			return fmt.Errorf("NetFlow v9 datagram too short: %d bytes", len(data))
		}
		if version := binary.BigEndian.Uint16(data[:2]); version != 9 {
			return fmt.Errorf("NetFlow v9 version=%d want 9", version)
		}
		if setID := binary.BigEndian.Uint16(data[20:22]); setID != 0 {
			return fmt.Errorf("first FlowSet id=%d want 0 (template)", setID)
		}
		return nil
	})
}

func sflowReceiver(ctx context.Context, args []string) error {
	return receiveOne(ctx, "sflow-receiver", args, func(data []byte) error {
		if len(data) < 4 {
			return fmt.Errorf("sFlow datagram too short: %d bytes", len(data))
		}
		if version := binary.BigEndian.Uint32(data[:4]); version != 5 {
			return fmt.Errorf("sFlow version=%d want 5", version)
		}
		return nil
	})
}

func multiCollectorReceiver(ctx context.Context, args []string) error {
	sflowPort, err := parsePort(args, 0)
	if err != nil {
		return err
	}
	ipfixPort, err := parsePort(args, 1)
	if err != nil {
		return err
	}
	sflowSock, err := listenUDP(sflowPort)
	if err != nil {
		return err
	}
	defer sflowSock.Close() //nolint:errcheck // fixture teardown
	ipfixSock, err := listenUDP(ipfixPort)
	if err != nil {
		return err
	}
	defer ipfixSock.Close() //nolint:errcheck // fixture teardown
	return Observe(ctx, "multi-receiver", sdk.Registration{}, func(context.Context, *sdk.Plugin) error {
		type result struct {
			kind string
			data []byte
			err  error
		}
		results := make(chan result, 2)
		read := func(kind string, sock *net.UDPConn) {
			_ = sock.SetReadDeadline(time.Now().Add(10 * time.Second))
			buf := make([]byte, 65535)
			n, _, readErr := sock.ReadFromUDP(buf)
			results <- result{kind: kind, data: buf[:n], err: readErr}
		}
		go read("sflow", sflowSock)
		go read("ipfix", ipfixSock)
		for range 2 {
			got := <-results
			if got.err != nil {
				return fmt.Errorf("no %s datagram received: %w", got.kind, got.err)
			}
			switch got.kind {
			case "sflow":
				if len(got.data) < 4 || binary.BigEndian.Uint32(got.data[:4]) != 5 {
					return fmt.Errorf("sFlow datagram had wrong version")
				}
			case "ipfix":
				if len(got.data) < 2 || binary.BigEndian.Uint16(got.data[:2]) != 10 {
					return fmt.Errorf("IPFIX datagram had wrong version")
				}
			}
		}
		return nil
	})
}
