package fixture

import (
	"context"
	"fmt"
	"net"
	"os"
	"strconv"
	"strings"
	"time"
)

func extra1AS112UnreachableProbe(ctx context.Context, args []string) error {
	if len(args) != 1 {
		return fmt.Errorf("AS112 health command probe expects the SSH port: %q", args)
	}
	configDir, err := os.MkdirTemp("", "as112-health-command-")
	if err != nil {
		return err
	}
	defer os.RemoveAll(configDir) //nolint:errcheck // fixture cleanup
	output, err := extra1CLI(ctx, configDir, args[0], "admin", "testpass", "request as112 healthcheck target 192.0.2.1")
	if err != nil {
		return fmt.Errorf("request as112 healthcheck: %w\n%s", err, output)
	}
	return nil
}

func extra1AS112ProbeAnycast(ctx context.Context, args []string) error {
	if len(args) != 0 {
		return fmt.Errorf("as112-probe-anycast-not-loopback takes no arguments: %q", args)
	}
	if err := extra1TouchReady(); err != nil {
		return err
	}
	listener, err := (&net.ListenConfig{}).Listen(ctx, "tcp", "127.0.0.1:0")
	if err != nil {
		return err
	}
	address, ok := listener.Addr().(*net.TCPAddr)
	if !ok {
		return fmt.Errorf("a tcp listener answered %T, want *net.TCPAddr", listener.Addr())
	}
	sshPort := strconv.Itoa(address.Port)
	if err := listener.Close(); err != nil {
		return err
	}
	config := fmt.Sprintf(`service { as112 { enabled true } }
bgp {
	healthcheck {
		probe as112check {
			command "ze-test fixture plugin/as112-probe-anycast-not-loopback-probe %s"
			group as112-hc
			interval 1
			rise 1
			fall 1
			withdraw-on-down true
		}
	}
	peer peer1 {
		connection { remote { ip 127.0.0.1; } local { ip 127.0.0.1; accept false; } }
		session {
			asn { local 65533; remote 65533; }
			router-id 10.0.0.10;
			family { ipv4/unicast { prefix { maximum 10000; } } }
		}
		update {
			attribute { origin igp; local-preference 100; next-hop 1.2.3.4; }
			nlri { ipv4/unicast add 192.175.48.0/24; }
			watchdog { name as112-hc; withdraw true; }
		}
	}
}
system { authentication { user admin { password %q; } } }
environment { ssh { enabled true; server main { ip 127.0.0.1; port %s; } } }
`, sshPort, extra1AdminHash, sshPort)
	daemon, port, err := extra1RunDaemon(ctx, "as112-probe-anycast.conf", config, nil)
	if err != nil {
		return err
	}
	defer daemon.stop()
	fmt.Fprintf(os.Stderr, "SSH port: %s\n", port)

	sawDown := false
	lastStatus := ""
	for range 24 {
		status, commandErr := extra1Command(port, "admin", "testpass", "show bgp healthcheck as112check | yaml")
		if commandErr == nil {
			lastStatus = status
			if strings.Contains(status, "state: UP") {
				return fmt.Errorf("as112check probe reached UP even though 192.0.2.1 never answers: %s", status)
			}
			if strings.Contains(status, "state: DOWN") {
				sawDown = true
			}
		}
		if err := extra1Wait(ctx, 500*time.Millisecond); err != nil {
			return err
		}
	}
	if !sawDown {
		return fmt.Errorf("as112check probe never reached DOWN within the poll window (last status: %s)", lastStatus)
	}
	fmt.Fprintf(os.Stderr, "OK: as112check probe reached DOWN and never UP over ~12s of failing probes (last status: %s)\n", lastStatus)
	fmt.Fprintln(os.Stderr, "OK: as112-probe-anycast-not-loopback test passed")
	return nil
}
