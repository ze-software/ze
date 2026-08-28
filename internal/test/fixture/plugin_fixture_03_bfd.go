package fixture

import (
	"bufio"
	"context"
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"

	"golang.org/x/sys/unix"

	"github.com/ze-software/ze/pkg/plugin/sdk"
)

func bfdEchoConfig03(ctx context.Context, p *sdk.Plugin) error {
	if !waitPeersEOR03(ctx, p, 1) {
		return fmt.Errorf("initial-sync EOR never reached the wire")
	}
	value, err := done03(ctx, p, "show bfd profile name fast-echo")
	if err != nil {
		return err
	}
	profile, ok := value.(map[string]any)
	if !ok || profile["name"] != "fast-echo" {
		return fmt.Errorf("profile name: %v", value)
	}
	value, err = done03(ctx, p, "show bfd session address 203.0.113.9")
	if err != nil {
		return err
	}
	session, ok := value.(map[string]any)
	if !ok || session["peer"] != "203.0.113.9" || session["mode"] != "single-hop" || session["profile"] != "fast-echo" {
		return fmt.Errorf("session payload: %v", value)
	}
	return nil
}

func bfdJSONShow03(ctx context.Context, p *sdk.Plugin) error {
	if !waitPeersEOR03(ctx, p, 1) {
		return fmt.Errorf("initial-sync EOR never reached the wire")
	}
	var status string
	var raw json.RawMessage
	var dispatchErr error
	if !Poll(ctx, 40, 250*time.Millisecond, func() bool {
		status, dispatchErr = Dispatch(ctx, p, "show bfd sessions", &raw)
		return dispatchErr == nil && status == "done"
	}) {
		if dispatchErr != nil {
			return dispatchErr
		}
		return fmt.Errorf("show bfd sessions: status=%s data=%s", status, raw)
	}
	if len(raw) == 0 {
		return fmt.Errorf("show bfd sessions: empty data")
	}
	var value any
	if err := json.Unmarshal(raw, &value); err != nil {
		return fmt.Errorf("show bfd sessions: invalid JSON: %w", err)
	}
	sessions, ok := value.([]any)
	if !ok {
		return fmt.Errorf("show bfd sessions: expected list, got %T", value)
	}
	if len(sessions) < 1 {
		return fmt.Errorf("show bfd sessions: expected at least 1 session")
	}
	for _, raw := range sessions {
		session, ok := raw.(map[string]any)
		if !ok {
			return fmt.Errorf("show bfd sessions: element is %T, not object", raw)
		}
		for _, key := range []string{"peer", "state", "mode", "local-discriminator"} {
			if _, exists := session[key]; !exists {
				return fmt.Errorf("show bfd sessions: missing field %s in %v", key, session["peer"])
			}
		}
	}
	return nil
}

func bfdMetrics03(ctx context.Context, p *sdk.Plugin) error {
	if _, err := donePoll03(ctx, p, "show bfd sessions"); err != nil {
		return fmt.Errorf("prime metrics: %w", err)
	}
	body, err := pollHTTP03(ctx, "http://127.0.0.1:19273/metrics", 20, func(body string) bool {
		return strings.Contains(body, "ze_bfd_sessions")
	})
	if err != nil {
		return err
	}
	if !strings.Contains(body, "ze_bfd_sessions") {
		return fmt.Errorf("missing ze_bfd_sessions in scrape body")
	}
	return nil
}

func bfdProfileShow03(ctx context.Context, p *sdk.Plugin) error {
	if !waitPeersEOR03(ctx, p, 1) {
		return fmt.Errorf("initial-sync EOR never reached the wire")
	}
	value, err := donePoll03(ctx, p, "show bfd profile")
	if err != nil {
		return err
	}
	profiles, ok := value.([]any)
	if !ok {
		return fmt.Errorf("profile list: expected array, got %T", value)
	}
	names := map[string]bool{}
	for _, raw := range profiles {
		if profile, ok := raw.(map[string]any); ok {
			if name, ok := profile["name"].(string); ok {
				names[name] = true
			}
		}
	}
	for _, want := range []string{"fast", "slow"} {
		if !names[want] {
			return fmt.Errorf("missing profile %s", want)
		}
	}
	value, err = done03(ctx, p, "show bfd profile name fast")
	if err != nil {
		return err
	}
	fast, ok := value.(map[string]any)
	if !ok || fast["name"] != "fast" || number03(fast["desired-min-tx-us"]) != 50000 {
		return fmt.Errorf("fast profile: %v", value)
	}
	status, _, err := dispatchAny03(ctx, p, "show bfd profile name nope")
	if err != nil {
		return err
	}
	if status != "error" {
		return fmt.Errorf("unknown profile status: %s", status)
	}
	return nil
}

func bfdSessionShow03(ctx context.Context, p *sdk.Plugin) error {
	if !waitPeersEOR03(ctx, p, 1) {
		return fmt.Errorf("initial-sync EOR never reached the wire")
	}
	value, err := donePoll03(ctx, p, "show bfd session address 203.0.113.9")
	if err != nil {
		return err
	}
	detail, ok := value.(map[string]any)
	if !ok || detail["peer"] != "203.0.113.9" || detail["mode"] != "single-hop" {
		return fmt.Errorf("known peer payload: %v", value)
	}
	status, value, err := dispatchAny03(ctx, p, "show bfd session address 198.51.100.200")
	if err != nil {
		return err
	}
	if status != "error" || !strings.Contains(fmt.Sprint(value), "no session for peer") {
		return fmt.Errorf("unknown peer: status=%s data=%v", status, value)
	}
	return nil
}

func bfdSessionsShow03(ctx context.Context, p *sdk.Plugin) error {
	if !waitPeersEOR03(ctx, p, 1) {
		return fmt.Errorf("initial-sync EOR never reached the wire")
	}
	value, err := donePoll03(ctx, p, "show bfd sessions")
	if err != nil {
		return err
	}
	sessions, ok := value.([]any)
	if !ok {
		return fmt.Errorf("show bfd sessions: expected array, got %T", value)
	}
	peers := map[string]bool{}
	for _, raw := range sessions {
		session, ok := raw.(map[string]any)
		if !ok {
			continue
		}
		peer, _ := session["peer"].(string)
		peers[peer] = true
		for _, key := range []string{"mode", "state", "vrf", "local-discriminator"} {
			if _, exists := session[key]; !exists {
				return fmt.Errorf("show bfd sessions: missing field %s in %s", key, peer)
			}
		}
		if session["profile"] != "fast" {
			return fmt.Errorf("show bfd sessions: profile=%v want fast (%s)", session["profile"], peer)
		}
	}
	for _, peer := range []string{"203.0.113.9", "198.51.100.9"} {
		if !peers[peer] {
			return fmt.Errorf("show bfd sessions: missing peer %s", peer)
		}
	}
	return nil
}

const bfdConfigLoadConfig03 = `environment {
}

bfd {
	enabled true;
	profile fast {
		detect-multiplier 3
		desired-min-tx-us 50000
		required-min-rx-us 50000
	}
}
`

const bfdEchoMultiHopConfig03 = `environment {
}

bfd {
	enabled true;
	profile echo-profile {
		detect-multiplier 3
		desired-min-tx-us 100000
		required-min-rx-us 100000
		echo {
			desired-min-echo-tx-us 50000
		}
	}
	multi-hop-session 198.51.100.9 {
		local 203.0.113.8
		profile echo-profile
	}
}
`

const bfdIPv6Config03 = `environment {
}

bfd {
	enabled true;
	bind-v6 true;
	profile fast {
		detect-multiplier 3
		desired-min-tx-us 100000
		required-min-rx-us 100000
	}
	single-hop-session 2001:db8::9 {
		profile fast
	}
}
`

const bfdStage2Config03 = `environment {
}

bfd {
	enabled true;
	profile fast {
		detect-multiplier 3
		desired-min-tx-us 50000
		required-min-rx-us 50000
	}
	single-hop-session 203.0.113.9 {
		profile fast
	}
	multi-hop-session 198.51.100.9 {
		local 203.0.113.8
		min-ttl 250
		profile fast
	}
}
`

func bfdConfigLoad03(ctx context.Context, _ []string) error {
	return runZeUntilLogs03(ctx, bfdConfigLoadConfig03, []string{"bfd plugin starting", "bfd plugin configured", "bfd plugin running", "subsystem=bfd"}, map[string]string{"ze.log.bfd": "debug"})
}

func bfdEchoMultiHopReject03(ctx context.Context, _ []string) error {
	return runZeUntilLogsRejecting03(ctx, bfdEchoMultiHopConfig03,
		[]string{"prohibits multi-hop echo"}, nil, 8*time.Second,
		map[string]string{"ze.log.bfd": "debug", "ze.bfd.test-parallel": "true"})
}

func bfdIPv6DualBind03(ctx context.Context, _ []string) error {
	return runZeUntilLogsRejecting03(ctx, bfdIPv6Config03,
		[]string{"bfd plugin starting", "bfd pinned session created", "ipv6=true", "bfd plugin running"},
		nil, 8*time.Second,
		map[string]string{"ze.log.bfd": "debug", "ze.bfd.test-parallel": "true"})
}

func bfdTransportStage203(ctx context.Context, _ []string) error {
	return runZeUntilLogs03(ctx, bfdStage2Config03, []string{"bfd plugin starting", "bfd plugin configured", "bfd loop started", "bfd pinned session created", "mode=single-hop", "mode=multi-hop"}, map[string]string{"ze.log.bfd": "debug"})
}

func runZeUntilLogs03(ctx context.Context, config string, required []string, extraEnv map[string]string) error {
	return runZeUntilLogsRejecting03(ctx, config, required, nil, 10*time.Second, extraEnv)
}

func runZeUntilLogsRejecting03(ctx context.Context, config string, required, forbidden []string, wait time.Duration, extraEnv map[string]string) error {
	cmd := exec.Command("ze", "-")
	cmd.Stdin = strings.NewReader(config)
	cmd.Stdout = os.Stdout
	stderr, err := cmd.StderrPipe()
	if err != nil {
		return err
	}
	cmd.Env = os.Environ()
	for key, value := range extraEnv {
		cmd.Env = append(cmd.Env, key+"="+value)
	}
	if err := cmd.Start(); err != nil {
		return err
	}
	seen := make(map[string]bool, len(required))
	rejected := make(map[string]bool, len(forbidden))
	matched := make(chan struct{})
	scanDone := make(chan error, 1)
	var matchedOnce sync.Once
	go func() {
		scanner := bufio.NewScanner(stderr)
		for scanner.Scan() {
			line := scanner.Text()
			fmt.Fprintln(os.Stderr, line)
			for _, needle := range required {
				if strings.Contains(line, needle) {
					seen[needle] = true
				}
			}
			for _, needle := range forbidden {
				if strings.Contains(line, needle) {
					rejected[needle] = true
				}
			}
			if len(seen) == len(required) {
				matchedOnce.Do(func() { close(matched) })
			}
		}
		scanDone <- scanner.Err()
	}()
	deadline := time.NewTimer(wait)
	scanned := false
	var scanErr error
	select {
	case <-matched:
	case scanErr = <-scanDone:
		scanned = true
	case <-ctx.Done():
	case <-deadline.C:
	}
	if !deadline.Stop() {
		select {
		case <-deadline.C:
		default:
		}
	}
	if cmd.Process != nil {
		_ = cmd.Process.Signal(syscall.SIGTERM)
	}
	waitDone := make(chan error, 1)
	go func() { waitDone <- cmd.Wait() }()
	select {
	case <-waitDone:
	case <-time.After(3 * time.Second):
		_ = cmd.Process.Kill()
		<-waitDone
	}
	if !scanned {
		scanErr = <-scanDone
	}
	if scanErr != nil {
		return fmt.Errorf("read ze stderr: %w", scanErr)
	}
	missing := make([]string, 0)
	for _, needle := range required {
		if !seen[needle] {
			missing = append(missing, needle)
		}
	}
	if len(missing) != 0 {
		return fmt.Errorf("missing required log patterns: %s", strings.Join(missing, ", "))
	}
	for _, needle := range forbidden {
		if rejected[needle] {
			return fmt.Errorf("configuration rejected: %s", needle)
		}
	}
	return nil
}

func bfdEchoHandshake03(ctx context.Context, p *sdk.Plugin, port string) error {
	if _, err := done03(ctx, p, "show bfd sessions"); err != nil {
		return err
	}
	body, err := pollHTTP03(ctx, "http://127.0.0.1:"+port+"/metrics", 40, func(body string) bool {
		return positiveMetric03(body, "ze_bfd_echo_tx_packets_total") && strings.Contains(body, "ze_bfd_echo_rtt_us_bucket")
	})
	if err != nil {
		return err
	}
	if !strings.Contains(body, "ze_bfd_echo_tx_packets_total") {
		return fmt.Errorf("missing ze_bfd_echo_tx_packets_total in scrape")
	}
	if !positiveMetric03(body, "ze_bfd_echo_tx_packets_total") {
		return fmt.Errorf("ze_bfd_echo_tx_packets_total is zero")
	}
	if !strings.Contains(body, "ze_bfd_echo_rtt_us_bucket") {
		return fmt.Errorf("ze_bfd_echo_rtt_us histogram has no buckets")
	}
	return nil
}

func positiveMetric03(body, name string) bool {
	for _, line := range strings.Split(body, "\n") {
		if strings.HasPrefix(line, "#") || !strings.HasPrefix(line, name) {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) >= 2 {
			value, err := strconv.ParseFloat(fields[1], 64)
			if err == nil && value > 0 {
				return true
			}
		}
	}
	return false
}

func bfdEchoPeer03(ctx context.Context, args []string) error {
	if len(args) != 0 {
		return fmt.Errorf("unexpected arguments %q", args)
	}
	control, err := listenBFDUDP03("127.0.0.2:3784")
	if err != nil {
		return fmt.Errorf("listen control: %w", err)
	}
	defer control.Close()
	echo, err := listenBFDUDP03("127.0.0.2:3785")
	if err != nil {
		return fmt.Errorf("listen echo: %w", err)
	}
	defer echo.Close()
	var wg sync.WaitGroup
	wg.Add(2)
	go func() {
		defer wg.Done()
		bfdControlLoop03(ctx, control)
	}()
	go func() {
		defer wg.Done()
		bfdEchoLoop03(ctx, echo)
	}()
	<-ctx.Done()
	_ = control.Close()
	_ = echo.Close()
	wg.Wait()
	return nil
}

func listenBFDUDP03(address string) (*net.UDPConn, error) {
	listen := net.ListenConfig{Control: func(_, _ string, raw syscall.RawConn) error {
		var controlErr error
		if err := raw.Control(func(fd uintptr) {
			if err := unix.SetsockoptInt(int(fd), unix.SOL_SOCKET, unix.SO_REUSEPORT, 1); err != nil {
				controlErr = err
				return
			}
			controlErr = unix.SetsockoptInt(int(fd), unix.IPPROTO_IP, unix.IP_TTL, 255)
		}); err != nil {
			return err
		}
		return controlErr
	}}
	packet, err := listen.ListenPacket(context.Background(), "udp4", address)
	if err != nil {
		return nil, err
	}
	conn, ok := packet.(*net.UDPConn)
	if !ok {
		packet.Close()
		return nil, fmt.Errorf("unexpected packet connection %T", packet)
	}
	return conn, nil
}

func bfdControlLoop03(ctx context.Context, conn *net.UDPConn) {
	buffer := make([]byte, 256)
	state := byte(1)
	for ctx.Err() == nil {
		_ = conn.SetReadDeadline(time.Now().Add(200 * time.Millisecond))
		n, address, err := conn.ReadFromUDP(buffer)
		if err != nil {
			var timeout net.Error
			if errors.As(err, &timeout) && timeout.Timeout() {
				continue
			}
			return
		}
		if n < 24 {
			continue
		}
		remoteState := (buffer[1] >> 6) & 3
		remoteDiscriminator := binary.BigEndian.Uint32(buffer[4:8])
		switch remoteState {
		case 1:
			if state == 1 {
				state = 2
			}
		case 2:
			if state == 2 || state == 3 {
				state = 3
			}
		case 3:
			if state == 2 {
				state = 3
			}
		}
		packet := make([]byte, 24)
		packet[0] = 1 << 5
		packet[1] = state << 6
		packet[2] = 3
		packet[3] = 24
		binary.BigEndian.PutUint32(packet[4:8], 0xdead0001)
		binary.BigEndian.PutUint32(packet[8:12], remoteDiscriminator)
		binary.BigEndian.PutUint32(packet[12:16], 500000)
		binary.BigEndian.PutUint32(packet[16:20], 500000)
		binary.BigEndian.PutUint32(packet[20:24], 500000)
		_, _ = conn.WriteToUDP(packet, address)
	}
}

func bfdEchoLoop03(ctx context.Context, conn *net.UDPConn) {
	buffer := make([]byte, 256)
	for ctx.Err() == nil {
		_ = conn.SetReadDeadline(time.Now().Add(200 * time.Millisecond))
		n, address, err := conn.ReadFromUDP(buffer)
		if err != nil {
			var timeout net.Error
			if errors.As(err, &timeout) && timeout.Timeout() {
				continue
			}
			return
		}
		_, _ = conn.WriteToUDP(buffer[:n], address)
	}
}
