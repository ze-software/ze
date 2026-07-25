// Design: plan/learned/673-diag-0-umbrella.md -- VPP dataplane trace via CLI socket

//go:build linux

package vpp

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"strings"
	"time"

	"github.com/ze-software/ze/internal/core/env"
	"github.com/ze-software/ze/internal/core/textbuf"
)

const (
	defaultCLISocket = "/run/vpp/cli.sock"
	cliTimeout       = 10 * time.Second
)

var _ = env.MustRegister(env.EnvEntry{Key: "ze.test.vpp.cli.socket", Type: "string", Description: "Override the VPP CLI unix socket path (tests)"})

func cliSocketPath() string {
	if v := env.Get("ze.test.vpp.cli.socket"); v != "" {
		return v
	}
	return defaultCLISocket
}

func execCLI(command string) (string, error) {
	sock := cliSocketPath()
	dialer := net.Dialer{Timeout: cliTimeout}
	conn, err := dialer.DialContext(context.Background(), "unix", sock)
	if err != nil {
		return "", fmt.Errorf("vpp cli: dial %s: %w", sock, err)
	}
	defer func() {
		if cerr := conn.Close(); cerr != nil {
			slog.Warn("vpp cli: close failed", "sock", sock, "error", cerr)
		}
	}()

	if err := conn.SetDeadline(time.Now().Add(cliTimeout)); err != nil {
		return "", fmt.Errorf("vpp cli: set deadline: %w", err)
	}

	if _, err := fmt.Fprintf(conn, "%s\n", command); err != nil { //nolint:errcheck // output
		return "", fmt.Errorf("vpp cli: write: %w", err)
	}

	var sb strings.Builder
	scanner := bufio.NewScanner(conn)
	scanner.Buffer(make([]byte, 0, 64*1024), 4*1024*1024)
	for scanner.Scan() {
		sb.WriteString(scanner.Text())
		sb.WriteByte('\n')
	}
	if err := scanner.Err(); err != nil {
		var netErr net.Error
		if errors.As(err, &netErr) {
			if sb.Len() > 0 {
				return sb.String(), nil
			}
		}
		return sb.String(), fmt.Errorf("vpp cli: read: %w", err)
	}
	return sb.String(), nil
}

// TraceStart sends "trace add <inputNode> <count>" to VPP.
func TraceStart(inputNode string, count int) (string, error) {
	var b textbuf.Buffer
	cmd := b.Reset().Str("trace add ").Str(inputNode).Byte(' ').Int(int64(count)).String()
	return execCLI(cmd)
}

// TraceShow sends "show trace" and returns raw output.
func TraceShow() (string, error) {
	return execCLI("show trace")
}

// TraceClear sends "clear trace".
func TraceClear() (string, error) {
	return execCLI("clear trace")
}

// ShowRuntime sends "show runtime" for node counters.
func ShowRuntime() (string, error) {
	return execCLI("show runtime")
}
