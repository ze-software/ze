// Design: plan/spec-diag-0-umbrella.md -- VPP dataplane trace via CLI socket

//go:build linux

package vpp

import (
	"bufio"
	"fmt"
	"net"
	"os"
	"strings"
	"time"

	"codeberg.org/thomas-mangin/ze/internal/core/textbuf"
)

const (
	defaultCLISocket = "/run/vpp/cli.sock"
	cliTimeout       = 10 * time.Second
)

func cliSocketPath() string {
	if v := os.Getenv("ZE_TEST_VPP_CLI_SOCKET"); v != "" {
		return v
	}
	return defaultCLISocket
}

func execCLI(command string) (string, error) {
	sock := cliSocketPath()
	conn, err := net.DialTimeout("unix", sock, cliTimeout)
	if err != nil {
		return "", fmt.Errorf("vpp cli: dial %s: %w", sock, err)
	}
	defer conn.Close()

	if err := conn.SetDeadline(time.Now().Add(cliTimeout)); err != nil {
		return "", fmt.Errorf("vpp cli: set deadline: %w", err)
	}

	if _, err := fmt.Fprintf(conn, "%s\n", command); err != nil {
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
		if netErr, ok := err.(net.Error); ok && netErr.Timeout() {
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
