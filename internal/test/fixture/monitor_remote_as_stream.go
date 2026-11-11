// Design: docs/architecture/api/commands.md -- compact monitor event output
// Related: plugin_fixture_10_process.go -- native daemon and SSH PTY helpers
// Related: register_bgp_delay_open.go -- validated BGP frame reads

package fixture

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/ze-software/ze/internal/component/bgp/message"
)

func init() {
	Register("plugin/monitor-remote-as-stream", monitorRemoteASStream)
}

// monitorRemoteASStream observes the same dynamic session before and after its
// remote OPEN. The PTY's active-monitor marker is the subscription barrier.
func monitorRemoteASStream(ctx context.Context, args []string) error {
	if len(args) != 1 {
		return errors.New("monitor-remote-as-stream requires the runner-assigned BGP port")
	}
	port, err := strconv.Atoi(args[0])
	if err != nil {
		return fmt.Errorf("read runner-assigned BGP port %q: %w", args[0], err)
	}
	if port < 1 {
		return fmt.Errorf("runner-assigned BGP port %d must be positive", port)
	}
	if port > 65535 {
		return fmt.Errorf("runner-assigned BGP port %d exceeds 65535", port)
	}
	work, err := os.MkdirTemp(".", "monitor-remote-as-")
	if err != nil {
		return err
	}
	defer os.RemoveAll(work) //nolint:errcheck // fixture-owned scratch only
	stateDir := filepath.Join(work, "state")
	if err := os.Mkdir(stateDir, 0o700); err != nil {
		return err
	}
	configPath := filepath.Join(work, "monitor.conf")
	if err := os.WriteFile(configPath, []byte(fmt.Sprintf(monitorRemoteASConfig, port)), 0o600); err != nil {
		return err
	}
	daemonCtx, cancel := context.WithCancel(ctx)
	defer cancel()
	logPath := filepath.Join(work, "daemon.log")
	daemon, logFile, err := fixture10StartDaemon(daemonCtx, configPath, logPath, map[string]string{envConfigDir: stateDir})
	if err != nil {
		return err
	}
	defer func() {
		fixture10StopProcess(daemon)
		logFile.Close() //nolint:errcheck // fixture teardown follows the observed result
	}()
	sshPattern := regexp.MustCompile(`SSH server listening.*?address=127\.0\.0\.1:(\d+)`)
	sshPort := 0
	if !Poll(ctx, 60, 250*time.Millisecond, func() bool {
		sshPort = fixture10PortFromLog(fixture10ReadLog(logFile, logPath), sshPattern)
		return sshPort != 0
	}) {
		return fmt.Errorf("monitor daemon did not report its SSH listener: %s", fixture10ReadLog(logFile, logPath))
	}
	terminal, err := fixture10StartPTY(ctx, sshPort)
	if err != nil {
		return err
	}
	defer terminal.close()
	var transcript bytes.Buffer
	if err := monitorRemoteASAwait(ctx, terminal, &transcript, "password:"); err != nil {
		return err
	}
	if _, err := terminal.file.WriteString("testpass\r"); err != nil {
		return err
	}
	if err := monitorRemoteASAwait(ctx, terminal, &transcript, "Welcome to Ze"); err != nil {
		return err
	}
	transcript.Reset()
	if _, err := terminal.file.WriteString("exit\r"); err != nil {
		return err
	}
	if err := monitorRemoteASAwait(ctx, terminal, &transcript, "ze> "); err != nil {
		return err
	}
	transcript.Reset()
	if _, err := terminal.file.WriteString("monitor event include open,state\r"); err != nil {
		return err
	}
	if err := monitorRemoteASAwait(ctx, terminal, &transcript, "monitor active"); err != nil {
		return err
	}
	var dialer net.Dialer
	conn, err := dialer.DialContext(ctx, "tcp", net.JoinHostPort(addrLoopback, args[0]))
	if err != nil {
		return fmt.Errorf("dial dynamic peer after monitor subscription: %w", err)
	}
	defer conn.Close() //nolint:errcheck // the terminal assertions carry the result
	if err := conn.SetDeadline(time.Now().Add(fixture10ProcessTimeout)); err != nil {
		return err
	}
	if err := delayOpenFixtureRead(conn, 1); err != nil {
		return fmt.Errorf("dynamic peer's initial OPEN: %w", err)
	}
	if err := monitorRemoteASAwait(ctx, terminal, &transcript, "sent OPEN   127.0.0.1 AS unknown"); err != nil {
		return fmt.Errorf("before peer OPEN: %w", err)
	}
	open := &message.Open{Version: 4, MyAS: 65001, HoldTime: 90, BGPIdentifier: 0x02020202}
	if _, err := conn.Write(message.PackTo(open, nil)); err != nil {
		return err
	}
	if err := delayOpenFixtureRead(conn, 4); err != nil {
		return fmt.Errorf("dynamic peer's handshake KEEPALIVE: %w", err)
	}
	if _, err := conn.Write(message.PackTo(message.NewKeepalive(), nil)); err != nil {
		return err
	}
	if err := monitorRemoteASAwait(ctx, terminal, &transcript, "STATE  127.0.0.1 AS65001 up"); err != nil {
		return fmt.Errorf("after peer OPEN: %w", err)
	}
	if strings.Contains(transcript.String(), " AS0") {
		return fmt.Errorf("monitor fabricated AS0: %q", transcript.String())
	}
	fmt.Fprintln(os.Stderr, "OK: daemon monitor PTY showed AS unknown before OPEN and AS65001 after negotiation")
	return nil
}

func monitorRemoteASAwait(ctx context.Context, terminal *fixture10PTYSession, transcript *bytes.Buffer, marker string) error {
	timer := time.NewTimer(fixture10ProcessTimeout)
	defer timer.Stop()
	for !strings.Contains(strings.ToLower(transcript.String()), strings.ToLower(marker)) {
		select {
		case chunk, open := <-terminal.chunks:
			if !open {
				return fmt.Errorf("PTY ended before %q: %q", marker, transcript.String())
			}
			transcript.Write(chunk)
		case <-timer.C:
			return fmt.Errorf("PTY did not show %q: %q", marker, transcript.String())
		case <-ctx.Done():
			return fmt.Errorf("PTY waiting for %q: %w", marker, ctx.Err())
		}
	}
	return nil
}

const monitorRemoteASConfig = `system {
    authentication {
        user operator {
            password "$2a$04$UlwuiuH82Unfsq.XEMPGJeDkXwbm3KW.nvVaVXOd/JeFK8VjMjrQO"
            profile [ monitor-test ]
        }
    }
    authorization {
        profile monitor-test {
            run {
                default-action allow
            }
            edit {
                default-action deny
            }
        }
    }
}
environment {
    ssh {
        enabled true
        server main {
            ip 127.0.0.1
            port 0
        }
    }
}
bgp {
    group dynamic-monitor {
        connection {
            remote {
                ip dynamic
                connect false
                range 127.0.0.1/32
            }
            local {
                ip 127.0.0.1
                port %d
                accept true
            }
        }
        session {
            asn {
                local 65000
            }
            router-id 1.1.1.1
            family {
                ipv4/unicast {
                    prefix {
                        maximum 10000
                    }
                }
            }
            capability {
                graceful-restart disable
            }
        }
    }
}
`
