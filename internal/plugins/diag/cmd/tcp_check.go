// Design: plan/learned/727-diag-core.md -- TCP port connectivity check (nc replacement)

package cmd

import (
	"context"
	"errors"
	"net"
	"strconv"
	"time"

	"github.com/ze-software/ze/internal/component/plugin"
	pluginserver "github.com/ze-software/ze/internal/component/plugin/server"
)

var (
	errTCPCheckMissingHost    = errors.New("tcp-check: missing host")
	errTCPCheckMissingPort    = errors.New("tcp-check: missing port")
	errTCPCheckPortOutOfRange = errors.New("tcp-check: port must be 1-65535")
)

const (
	argTimeout = "timeout"

	defaultTCPCheckTimeout = 5 * time.Second
	maxTCPCheckTimeout     = 30 * time.Second

	tcpCheckResultConnected = "connected"
	tcpCheckResultRefused   = "refused"
	tcpCheckResultTimeout   = "timeout"
)

func HandleTCPCheck(_ *pluginserver.CommandContext, args []string) (*plugin.Response, error) {
	host, port, source, timeout, err := parseTCPCheckArgs(args)
	if err != nil {
		return &plugin.Response{Status: plugin.StatusError, Error: err.Error()}, nil //nolint:nilerr // operational error in Response, not a Go error
	}

	addr := net.JoinHostPort(host, strconv.Itoa(port))
	start := time.Now()

	dialer := net.Dialer{Timeout: timeout}
	if source != "" {
		dialer.LocalAddr = &net.TCPAddr{IP: net.ParseIP(source)}
	}

	var conn net.Conn
	conn, err = dialer.DialContext(context.Background(), "tcp", addr)

	latency := time.Since(start)

	result := map[string]any{
		"host":       host,
		"port":       port,
		"latency-ms": float64(latency.Microseconds()) / 1000.0,
	}

	if err != nil {
		if isTimeout(err) {
			result["result"] = tcpCheckResultTimeout
		} else {
			result["result"] = tcpCheckResultRefused
		}
		result["error"] = err.Error()
	} else {
		if closeErr := conn.Close(); closeErr != nil {
			result["close-error"] = closeErr.Error()
		}
		result["result"] = tcpCheckResultConnected
	}

	if source != "" {
		result["source"] = source
	}

	return &plugin.Response{Status: plugin.StatusDone, Data: plugin.Map(result)}, nil
}

func isTimeout(err error) bool {
	var netErr net.Error
	if errors.As(err, &netErr) {
		return netErr.Timeout()
	}
	return false
}

func parseTCPCheckArgs(args []string) (host string, port int, source string, timeout time.Duration, err error) {
	timeout = defaultTCPCheckTimeout
	positional := 0
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case argTimeout:
			if i+1 >= len(args) {
				return "", 0, "", 0, errors.New("tcp-check: timeout requires a value (e.g. 3s)")
			}
			i++
			d, parseErr := time.ParseDuration(args[i])
			if parseErr != nil || d < time.Second || d > maxTCPCheckTimeout {
				return "", 0, "", 0, errors.New("tcp-check: timeout must be 1s-30s")
			}
			timeout = d
		case "source":
			if i+1 >= len(args) {
				return "", 0, "", 0, errors.New("tcp-check: source requires an IP address")
			}
			i++
			if net.ParseIP(args[i]) == nil {
				return "", 0, "", 0, errors.New("tcp-check: invalid source IP " + args[i])
			}
			source = args[i]
		default:
			switch positional {
			case 0:
				host = args[i]
			case 1:
				p, parseErr := strconv.Atoi(args[i])
				if parseErr != nil || p < 1 || p > 65535 {
					return "", 0, "", 0, errTCPCheckPortOutOfRange
				}
				port = p
			}
			positional++
		}
	}
	if host == "" {
		return "", 0, "", 0, errTCPCheckMissingHost
	}
	if len(host) > 253 {
		return "", 0, "", 0, errors.New("tcp-check: host exceeds 253-character limit")
	}
	if port == 0 {
		return "", 0, "", 0, errTCPCheckMissingPort
	}
	return host, port, source, timeout, nil
}
