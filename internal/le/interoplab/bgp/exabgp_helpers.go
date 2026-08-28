package bgp

import (
	"bufio"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/syslog"
	"os"
	"strings"
	"time"
)

type exaProfile struct {
	startup, between, wait time.Duration
	commands               []string
}

func runExaBGPHelper(args []string, input io.Reader, output, diagnostic io.Writer) error {
	if len(args) == 0 {
		return errors.New("exabgp-api wants a profile")
	}
	switch args[0] {
	case "echo":
		return runExaBGPEcho(input, output, diagnostic)
	case "stderr":
		return runExaBGPLineLog(input, diagnostic, args[1:], false)
	case "syslog":
		return runExaBGPLineLog(input, diagnostic, args[1:], true)
	case "example-api-program":
		return runExampleAPIProgram(input, output)
	}
	profile, ok := exaProfiles[args[0]]
	if !ok {
		return fmt.Errorf("unknown ExaBGP API profile %q", args[0])
	}
	return runExaProfile(input, output, profile)
}

func runExaProfile(input io.Reader, output io.Writer, profile exaProfile) error {
	lines := exaLines(input)
	time.Sleep(profile.startup)
	for _, command := range profile.commands {
		if delayText, ok := strings.CutPrefix(command, "@sleep "); ok {
			delay, err := time.ParseDuration(delayText)
			if err != nil {
				return err
			}
			time.Sleep(delay)
			continue
		}
		if _, err := fmt.Fprintln(output, command); err != nil {
			return err
		}
		time.Sleep(profile.between)
	}
	wait := profile.wait
	if wait == 0 {
		wait = 5 * time.Second
	}
	timer := time.NewTimer(wait)
	defer timer.Stop()
	for {
		select {
		case line, open := <-lines:
			if !open || strings.Contains(line, "shutdown") {
				return nil
			}
		case <-timer.C:
			return nil
		}
	}
}

func exaLines(input io.Reader) <-chan string {
	lines := make(chan string, 32)
	go func() {
		defer close(lines)
		scanner := bufio.NewScanner(input)
		for scanner.Scan() {
			lines <- scanner.Text()
		}
	}()
	return lines
}

func runExaBGPEcho(input io.Reader, output, diagnostic io.Writer) error {
	mode := os.Getenv("TEST_MODE")
	if mode == "" {
		mode = "echo"
	}
	_, _ = fmt.Fprintf(diagnostic, "[exabgp_echo] starting, mode=%s\n", mode)
	scanner := bufio.NewScanner(input)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		_, _ = fmt.Fprintf(diagnostic, "[exabgp_echo] received: %.100s...\n", line)
		var event map[string]any
		if err := json.Unmarshal([]byte(line), &event); err != nil {
			_, _ = fmt.Fprintf(diagnostic, "[exabgp_echo] json parse error: %v\n", err)
			continue
		}
		if event["type"] == "shutdown" || mode == "noop" {
			break
		}
		if mode == "log" {
			pretty, _ := json.MarshalIndent(event, "", "  ")
			_, _ = fmt.Fprintf(diagnostic, "[exabgp_echo] full json: %s\n", pretty)
		}
		if mode == "echo" && event["type"] == "update" {
			if err := echoExaBGPUpdate(event, output, diagnostic); err != nil {
				return err
			}
		}
	}
	_, _ = fmt.Fprintln(diagnostic, "[exabgp_echo] exiting")
	return scanner.Err()
}

func echoExaBGPUpdate(event map[string]any, output, diagnostic io.Writer) error {
	neighbor, _ := event["neighbor"].(map[string]any)
	address, _ := neighbor["address"].(map[string]any)
	peer, _ := address["peer"].(string)
	if peer == "" {
		peer = "127.0.0.1"
	}
	message, _ := neighbor["message"].(map[string]any)
	update, _ := message["update"].(map[string]any)
	announce, _ := update["announce"].(map[string]any)
	for _, familyValue := range announce {
		nextHops, _ := familyValue.(map[string]any)
		for nextHop, entriesValue := range nextHops {
			if nextHop == "null" {
				nextHop = "192.0.2.1"
			}
			entries, _ := entriesValue.([]any)
			for _, entryValue := range entries {
				if prefix := exaPrefix(entryValue); prefix != "" {
					command := fmt.Sprintf("neighbor %s announce route %s next-hop %s", peer, prefix, nextHop)
					_, _ = fmt.Fprintf(diagnostic, "[exabgp_echo] sending: %s\n", command)
					if _, err := fmt.Fprintln(output, command); err != nil {
						return err
					}
				}
			}
		}
	}
	withdraw, _ := update["withdraw"].(map[string]any)
	for _, familyValue := range withdraw {
		entries, _ := familyValue.([]any)
		for _, entryValue := range entries {
			if prefix := exaPrefix(entryValue); prefix != "" {
				command := fmt.Sprintf("neighbor %s withdraw route %s", peer, prefix)
				_, _ = fmt.Fprintf(diagnostic, "[exabgp_echo] sending: %s\n", command)
				if _, err := fmt.Fprintln(output, command); err != nil {
					return err
				}
			}
		}
	}
	return nil
}

func exaPrefix(value any) string {
	if entry, ok := value.(map[string]any); ok {
		prefix, _ := entry["nlri"].(string)
		return prefix
	}
	return fmt.Sprint(value)
}

func runExaBGPLineLog(input io.Reader, diagnostic io.Writer, args []string, system bool) error {
	level := "EXABGP PROCESS"
	if len(args) > 0 {
		level = args[0]
	}
	var logger *syslog.Writer
	if system {
		logger, _ = syslog.New(syslog.LOG_ALERT, "ExaBGP")
		if logger != nil {
			defer func() { _ = logger.Close() }()
		}
	}
	scanner := bufio.NewScanner(input)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		message := fmt.Sprintf("%s %-8s %-6d %s", time.Now().Format("Mon, 02 Jan 2006 15:04:05"), level, os.Getpid(), line)
		if logger != nil {
			_ = logger.Alert(message)
		} else {
			_, _ = fmt.Fprintln(diagnostic, message)
		}
	}
	return scanner.Err()
}

func runExampleAPIProgram(input io.Reader, output io.Writer) error {
	_ = exaLines(input)
	for prefix := 1; prefix < 255; prefix++ {
		if _, err := fmt.Fprintf(output, "update text nhop set 10.0.0.1 nlri ipv4/unicast add 192.0.2.%d\n", prefix); err != nil {
			return err
		}
	}
	return nil
}

func p(startup, between, wait time.Duration, commands ...string) exaProfile {
	return exaProfile{startup: startup, between: between, wait: wait, commands: commands}
}

var exaProfiles = map[string]exaProfile{
	"api-ack-control":      p(200*time.Millisecond, 100*time.Millisecond, 5*time.Second, "announce route 1.1.0.0/24 next-hop 101.1.101.1", "disable-ack", "announce route 1.2.0.0/24 next-hop 101.1.101.1", "enable-ack", "announce route 1.3.0.0/24 next-hop 101.1.101.1"),
	"api-add-remove":       p(0, 0, 5*time.Second, "announce route 1.1.0.0/24 next-hop 101.1.101.1", "announce route 1.1.0.0/25 next-hop 101.1.101.1", "@sleep 200ms", "withdraw route 1.1.0.0/24", "withdraw route 1.1.0.0/25 next-hop 0.0.0.0"),
	"api-announce":         p(0, 0, 5*time.Second, "announce route 1.1.0.0/24 next-hop 101.1.101.1", "announce route 1.2.0.0/25 next-hop 101.1.101.1"),
	"api-announce-star":    p(0, 100*time.Millisecond, 5*time.Second, "neighbor * announce route 1.1.0.0/24 next-hop 101.1.101.1", "neighbor * announce route 1.2.0.0/25 next-hop 101.1.101.1"),
	"api-announcement":     p(200*time.Millisecond, 100*time.Millisecond, 5*time.Second, "announce route 1.0.0.0/21 next-hop 101.1.101.1 med 200 community [2:1] split /23", "neighbor 127.0.0.1 announce route 1.1.0.0/16 next-hop 101.1.101.1", "neighbor 127.0.0.1 local-as 1 announce route 1.2.0.0/22 next-hop 101.1.101.1 split /24", "neighbor 127.0.0.1 peer-as 1 announce route 1.3.0.0/24 next-hop 101.1.101.1 split /25", "neighbor 127.0.0.1 local-ip 127.0.0.1 announce route 1.4.0.0/16 next-hop 101.1.101.1 med 200", "neighbor 127.0.0.1 router-id 1.2.3.4 announce route 1.5.0.0/16 next-hop 101.1.101.1 community [2:1]", "neighbor absentone local-as 1 peer-as 1 local-ip 127.0.0.1 router-id 1.2.3.4 announce route 9.9.9.9/21 next-hop 101.1.101.1 med 200 community [2:1]", "neighbor 127.0.0.1 local-as 1 peer-as 1 local-ip 127.0.0.1 router-id 1.2.3.4 announce route 1.6.0.0/21 next-hop 101.1.101.1 med 200 community [2:1]", "neighbor 127.0.0.1 local-as 2 peer-as 3 local-ip 127.0.0.1 router-id 1.2.3.4 announce route 1.8.0.0/21 next-hop 101.1.101.1 med 200 large-community [2914:1:0]"),
	"api-attributes":       p(200*time.Millisecond, 200*time.Millisecond, 5*time.Second, "announce attributes med 100 next-hop 101.1.101.1 nlri 1.0.0.1/32 1.0.0.2/32", "announce attributes local-preference 200 as-path [ 1 2 3 4 ] next-hop 202.2.202.2 nlri 2.0.0.1/32 2.0.0.2/32"),
	"api-attributes-path":  p(0, 200*time.Millisecond, 5*time.Second, "neighbor 127.0.0.1 announce attributes path-information 1.2.3.4 next-hop 10.11.12.13 community [14:15] local-preference 16 nlri 16.17.18.19/32 20.21.22.23/32", "neighbor 127.0.0.1 announce attributes path-information 4.3.2.1 next-hop 10.11.12.13 community [14:15] local-preference 16 nlri 16.17.18.19/32 20.21.22.23/32"),
	"api-attributes-vpn":   p(200*time.Millisecond, 0, 5*time.Second, "announce attribute route-distinguisher 63333:100 label [ 110 ] next-hop 10.0.99.12 origin igp as-path [100, 500] local-preference 100 extended-community 0:0 originator-id 10.0.99.12 nlri 128.0.64.0/18 128.0.0.0/18"),
	"api-broken-flow":      p(0, 200*time.Millisecond, 5*time.Second, "announce flow route { match { source 170.170.170.170/32; destination 170.170.170.170/32; } then { rate-limit 1; } }", "withdraw flow route { match { source 170.170.170.170/32; destination 170.170.170.170/32; } then { rate-limit 1; } }", "@sleep 500ms", "announce flow route { match { source 187.187.187.187/32; destination 187.187.187.187/32; } then { rate-limit 2; } }", "@sleep 500ms", "announce flow route { match { source 204.204.204.204/32; destination 187.187.187.187/32; } then { rate-limit 3; } }"),
	"api-check":            p(0, 0, 5*time.Second, "neighbor 127.0.0.1 announce route 1.2.3.4 next-hop 5.6.7.8"),
	"api-eor":              p(2*time.Second, 100*time.Millisecond, 5*time.Second, "announce eor", "announce eor ipv6 unicast", "announce eor ipv4 unicast"),
	"api-fast":             p(200*time.Millisecond, 0, 5*time.Second, "announce route 1.1.0.0/24 next-hop 101.1.101.1", "announce route 1.1.0.0/25 next-hop 101.1.101.1", "withdraw route 1.1.0.0/24 next-hop 101.1.101.1", "announce route 2.2.0.0/25 next-hop 101.1.101.1", "announce route 2.2.0.0/24 next-hop 101.1.101.1", "withdraw route 2.2.0.0/25 next-hop 101.1.101.1", "announce route 0.0.0.0/0  next-hop 1.101.1.101"),
	"api-flow":             p(200*time.Millisecond, 0, time.Second, "withdraw flow route { match { source 0.0.0.0/32; destination 0.0.0.0/32; destination-port =3128; protocol tcp; } }", "announce flow route { match { source 0.0.0.0/32; destination 0.0.0.0/32; destination-port =3128; protocol tcp; } then { rate-limit 0; } }", "announce flow route { match { source 255.255.255.255/32; destination 255.255.255.255/32; } then { rate-limit 65535; } }"),
	"api-health":           p(2*time.Second, 100*time.Millisecond, 7*time.Second, "ping", "announce route 192.168.0.1/32 next-hop 10.0.0.1", "status", "announce route 192.168.0.2/32 next-hop 10.0.0.2"),
	"api-flow-merge":       p(0, 0, time.Second, "announce flow route { match { source 4.4.4.4/32; } then { rate-limit 0; } }", "announce flow route { match { source 5.5.5.5/32; } then { discard; } }", "announce flow route { match { source 6.6.6.6/32; } scope { interface-set [ non-transitive:input:3405770241:1 ]; } then { discard; } }", "announce flow route { match { source 7.7.7.7/32; } scope { interface-set [ non-transitive:input:3405770241:1 transitive:output:254:254 ]; } then { discard; } }", "announce flow route { match { source 8.8.8.8/32; } then { discard; } scope { interface-set [ non-transitive:input:3405770241:1 transitive:output:254:254 ]; } }", "announce flow route { match { source 9.9.9.9/32; } then { discard; } scope { interface-set [ non-transitive:input:3405770241:1 transitive:output:254:254 ]; } }", "announce flow route { match { source 10.10.10.10/32; } scope { interface-set [ transitive:input-output:1234:10 transitive:input:1234:10 transitive:output:0:0]; } then { discard; } }", "announce flow route destination 133.130.1.219/32 discard interface-set [ transitive:input:1234:10]", "announce flow route destination 133.130.1.19/32 interface-set [ transitive:input:1234:10 ] discard"),
	"api-ipv4":             p(2*time.Second, 200*time.Millisecond, 5*time.Second, "announce ipv4 unicast 10.0.1.0/24 next-hop 10.0.1.254 local-preference 200", "withdraw ipv4 unicast 10.0.1.0/24 next-hop 10.0.1.254 local-preference 200", "announce ipv4 mup mup-isd 10.0.1.0/24 rd 100:100 next-hop 2001::1 extended-community [ target:10:10 ] bgp-prefix-sid-srv6 ( l3-service 2001:db8:1:1:: 0x48 [64,24,16,0,0,0] )", "withdraw ipv4 mup mup-isd 10.0.1.0/24 rd 100:100 next-hop 2001::1 extended-community [ target:10:10 ] bgp-prefix-sid-srv6 ( l3-service 2001:db8:1:1:: 0x48 [64,24,16,0,0,0] )"),
	"api-ipv6":             p(200*time.Millisecond, 200*time.Millisecond, 5*time.Second, "announce ipv6 unicast fc00:1::/64 next-hop 2001::11 local-preference 200", "withdraw ipv6 unicast fc00:1::/64 next-hop 2001::11 local-preference 200", "announce ipv6 mup mup-isd 2001::/64 rd 100:100 next-hop 2001::2 extended-community [ target:10:10 ] bgp-prefix-sid-srv6 ( l3-service 2001:db8:1:1:: 0x47 [64,24,16,0,0,0] )", "withdraw ipv6 mup mup-isd 2001::/64 rd 100:100 next-hop 2001::2 extended-community [ target:10:10 ] bgp-prefix-sid-srv6 ( l3-service 2001:db8:1:1:: 0x47 [64,24,16,0,0,0] )"),
	"api-manual-eor":       p(time.Second, 100*time.Millisecond, 5*time.Second, "announce eor", "announce eor ipv6 unicast", "announce eor ipv4 unicast"),
	"api-multi-neighbor":   p(200*time.Millisecond, 0, 5*time.Second, "neighbor 127.0.0.1 announce route 1.1.0.0/24 next-hop 101.1.101.1", "neighbor 127.0.0.1 router-id 1.2.3.4, neighbor 127.0.0.1 announce route 1.1.0.0/25 next-hop 101.1.101.1", "withdraw route 1.1.0.0/24 next-hop 101.1.101.1"),
	"api-multiple-private": p(time.Second, 0, 5*time.Second, "announce route 2.2.2.2/32 next-hop 127.0.0.1", "shutdown"),
	"api-multiple-public":  p(200*time.Millisecond, 0, 5*time.Second, "announce route 1.1.1.1/32 next-hop 127.0.0.1", "announce route 3.3.3.3/32 next-hop 127.0.0.1"),
	"api-multisession":     p(200*time.Millisecond, 0, 5*time.Second, "announce route 1.0.0.0/24 next-hop 101.1.101.1 med 200 community [2:1]", "neighbor 127.0.0.1 announce route 1.1.0.0/24 next-hop 101.1.101.1", "neighbor 127.0.0.1 local-as 1 family-allowed ipv4-unicast announce route 1.2.0.0/24 next-hop 101.1.101.1", "neighbor 127.0.0.1 local-as 1 family-allowed in-open announce route 9.9.9.9/24 next-hop 101.1.101.1", "neighbor 127.0.0.1 local-as 1 peer-as 1 local-ip 127.0.0.1 router-id 1.2.3.4 announce route 1.3.0.0/24 next-hop 101.1.101.1"),
	"api-mvpn":             p(0, 100*time.Millisecond, 17*time.Second, mvpnCommands()...),
	"api-nexthop":          p(200*time.Millisecond, 200*time.Millisecond, 5*time.Second, "neighbor 127.0.0.1 announce route 2605::2/128 next-hop 2001::1 local-preference 500", "neighbor 127.0.0.1 announce route 2605::2/128 next-hop 2001::2 local-preference 500"),
	"api-nexthop-self":     p(200*time.Millisecond, 0, 5*time.Second, "neighbor 127.0.0.1 announce route 1.1.0.0/16 next-hop self", "announce route 2.2.0.0/16 next-hop self", "announce route 2.2.0.1/32 next-hop 127.0.0.1", "announce route 2.2.0.1/32 next-hop self", "announce route 3.3.0.3/32 next-hop self"),
	"api-no-neighbor":      p(time.Second, 0, time.Second, "system version", "request shutdown"),
	"api-no-respawn-1":     p(0, 0, time.Second, "announce route 1.1.1.1/32 next-hop 11.11.11.11"),
	"api-no-respawn-2":     p(200*time.Millisecond, 200*time.Millisecond, 5*time.Second, "announce route 2.2.2.2/32 next-hop 22.22.22.22", "announce route 3.3.3.3/32 next-hop 33.33.33.33"),
	"api-notification":     p(200*time.Millisecond, 0, 5*time.Second, "announce route 1.2.3.4 next-hop 5.6.7.8"),
	"api-open":             p(0, 0, 5*time.Second, "announce route 1.1.1.1/32 next-hop 101.1.101.1 med 200"),
	"api-peer-lifecycle":   p(time.Second, 200*time.Millisecond, time.Second, "create neighbor 127.0.0.1 local-address 127.0.0.1 local-as 1 peer-as 1 router-id 1.2.3.4 api peer-lifecycle", "announce route 1.1.0.0/24 next-hop 101.101.101.101", "announce route 999.999.999.999/32 next-hop 1.1.1.1", "shutdown"),
	"api-reload":           p(2*time.Second, 200*time.Millisecond, 5*time.Second, "announce route 3.0.0.0/24 next-hop 4.0.0.0"),
	"api-rib":              p(time.Second, 100*time.Millisecond, 7*time.Second, ribCommands()...),
	"api-rr-rib":           p(time.Second, 100*time.Millisecond, 7*time.Second, ribCommands()...),
	"api-rr":               p(200*time.Millisecond, 200*time.Millisecond, 15*time.Second, "announce route 192.168.0.0/32 next-hop 10.0.0.0", "announce route 192.168.0.1/32 next-hop 10.0.0.1", "announce route-refresh ipv4 unicast", "announce route 192.168.0.2/32 next-hop 10.0.0.1"),
	"api-silence-ack":      p(200*time.Millisecond, 100*time.Millisecond, 5*time.Second, "announce route 2.1.0.0/24 next-hop 102.1.102.1", "silence-ack", "announce route 2.2.0.0/24 next-hop 102.1.102.1", "enable-ack", "announce route 2.3.0.0/24 next-hop 102.1.102.1"),
	"api-simple":           p(time.Second, 0, time.Second, "version", "ping", "status", "shutdown"),
	"api-teardown":         p(200*time.Millisecond, 200*time.Millisecond, 5*time.Second, "announce route 1.1.0.0/16 next-hop 1.1.1.1", "neighbor 127.0.0.1 teardown 4", "announce route 2.2.0.0/16 next-hop 2.2.2.2", "announce route 3.3.0.0/16 next-hop 3.3.3.3"),
	"api-v6-comprehensive": p(500*time.Millisecond, 0, 2*time.Second, comprehensiveCommands()...),
	"api-vpls":             p(200*time.Millisecond, 200*time.Millisecond, 5*time.Second, "neighbor 127.0.0.1 announce vpls endpoint 5 base 10702 offset 1 size 8 rd 192.168.201.1:123 next-hop 192.168.201.1 origin igp as-path [ 30740 30740 30740 30740 30740 30740 30740 ] local-preference 100 med 2000 community [ 54591:123] extended-community [ target:54591:6 l2info:19:0:1500:111] originator-id 192.168.22.1 cluster-list [ 3.3.3.3 192.168.201.1 ]", "neighbor 127.0.0.1 withdraw vpls endpoint 5 base 10702 offset 1 size 8 rd 192.168.201.1:123 next-hop 192.168.201.1"),
	"api-vpnv4":            p(200*time.Millisecond, 200*time.Millisecond, 5*time.Second, "neighbor 127.0.0.1 announce route 1.4.0.0/16 rd 65000:1 next-hop 101.1.101.1 community 100:1 extended-community 0x0002FDE900000001 label 1000", "neighbor 127.0.0.1 withdraw route 1.4.0.0/16 rd 65000:1 next-hop 101.1.101.1 community 100:1 extended-community 0x0002FDE900000001 label 1000"),
	"api-api.receive":      p(0, 0, 8*time.Second, "announce route 6.6.6.0/24 next-hop 1.1.1.1"),
	"api-api.nothing":      p(0, 0, 5*time.Second, "announce route 6.6.6.0/24 next-hop 9.9.9.9"),
	"api-blocklist":        p(0, 0, 10*time.Second),
	"healthcheck":          p(0, 0, 10*time.Second),
	"watchdog":             p(200*time.Millisecond, 200*time.Millisecond, time.Second, "announce watchdog dnsr", "withdraw watchdog dnsr"),
}

func ribCommands() []string {
	return []string{"announce route 192.168.0.0/32 next-hop 10.0.0.0", "clear adj-rib out", "announce route 192.168.0.1/32 next-hop 10.0.0.1", "clear adj-rib out", "announce route 192.168.0.2/32 next-hop 10.0.0.2", "announce route 192.168.0.3/32 next-hop 10.0.0.3", "flush adj-rib out", "announce route 192.168.0.4/32 next-hop 10.0.0.4", "flush adj-rib out", "clear adj-rib out", "announce route 192.168.0.5/32 next-hop 10.0.0.5"}
}

func mvpnCommands() []string {
	routes := []string{"ipv4 mcast-vpn shared-join rp 10.99.199.1 group 239.251.255.228 rd 65000:99999 source-as 65000 next-hop 10.10.6.3 extended-community [ target:192.168.94.12:5 ]", "ipv4 mcast-vpn source-join source 10.99.12.2 group 239.251.255.228 rd 65000:99999 source-as 65000 next-hop 10.10.6.3 extended-community [ target:192.168.94.12:5 ]", "ipv6 mcast-vpn shared-join rp fd00::1 group ff0e::1 rd 65000:99999 source-as 65000 next-hop 10.10.6.3 extended-community [ target:192.168.94.12:5 ]", "ipv6 mcast-vpn source-join source fd12::2 group ff0e::1 rd 65000:99999 source-as 65000 next-hop 10.10.6.3 extended-community [ target:192.168.94.12:5 ]", "ipv6 mcast-vpn source-ad source fd12::4 group ff0e::1 rd 65000:99999 next-hop 10.10.6.4 extended-community [ target:65000:99999 ]", "ipv4 mcast-vpn source-ad source 10.99.12.4 group 239.251.255.228 rd 65000:99999 next-hop 10.10.6.4 extended-community [ target:65000:99999 ]"}
	result := make([]string, 0, len(routes)*2)
	for _, route := range routes {
		result = append(result, "announce "+route)
	}
	for _, route := range routes {
		result = append(result, "withdraw "+route)
	}
	return result
}

func comprehensiveCommands() []string {
	return []string{"system version", "system help", "system api version", "system queue-status", "session ping", "session ack enable", "session sync enable", "session sync disable", "session reset", "show status", "show bgp peer list", "peer 127.0.0.1 show", "peer 127.0.0.1 show summary", "peer 127.0.0.1 announce route 10.0.0.0/24 next-hop 1.2.3.4", "peer 127.0.0.1 announce ipv4 unicast 10.0.1.0/24 next-hop 1.2.3.4", "peer 127.0.0.1 announce eor ipv4 unicast", "peer 127.0.0.1 announce route-refresh ipv4 unicast", "peer 127.0.0.1 withdraw route 10.0.0.0/24", "peer 127.0.0.1 withdraw ipv4 unicast 10.0.1.0/24", "rib show in", "rib show out", "rib clear in", "rib clear out *", "invalid_command_xyz_12345", "request invalid_action_xyz", "peer 999.999.999.999 show", "# this is a comment", "peer 127.0.0.1 announce route 255.255.255.255/32 next-hop 255.255.255.255", "request shutdown"}
}
