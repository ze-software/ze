//go:build linux

package main

import (
	"bytes"
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/ze-software/ze/internal/core/textbuf"
)

type l2tpDiagnosticHalf struct {
	stdout   string
	stderr   string
	code     int
	tunnels  string
	sessions string
}

// VALIDATES: each source diagnostic and its native le action emit the same full
// page, exit with the same code, send the same displayed netlink bytes, and
// leave the same kernel objects in separate fresh network namespaces.
// PREVENTS: parity against state created by the other half, normalized byte
// comparisons, cleanup that the producer never performed, or a dump-only test.
func TestL2TPDiagnosticsKeepFullIsolatedKernelParity(t *testing.T) {
	requireL2TPDiagnosticParityHost(t)
	root := l2tpDiagnosticRoot(t)
	bin := t.TempDir()
	oldPPPoX := filepath.Join(bin, "l2tp-pppox-diag")
	oldTunnel := filepath.Join(bin, "l2tp-tunnel-diag")
	le := filepath.Join(bin, "le")
	buildL2TPDiagnosticBinary(t, root, oldPPPoX, "./scripts/evidence/l2tp-pppox-diag")
	buildL2TPDiagnosticBinary(t, root, oldTunnel, "./scripts/evidence/l2tp-tunnel-diag")
	buildL2TPLeBinary(t, root, le)

	cases := []struct {
		name      string
		oldBinary string
		oldArgs   []string
		newArgs   []string
		address   string
	}{
		{
			name: "protocol-v3 tunnel", oldBinary: oldTunnel,
			newArgs: []string{"deployment", "l2tp-tunnel-diag",
				"local", "172.30.0.1", "remote", "172.30.0.2",
				"source-port", "1701", "destination-port", "1702",
				"tunnel-id", "1", "peer-tunnel-id", "100"},
			address: "172.30.0.1/32",
		},
		{
			name: "PPPoX and PPP ioctls", oldBinary: oldPPPoX,
			newArgs: []string{"deployment", "l2tp-pppox-diag",
				"local", "0.0.0.0", "remote", "127.0.0.1",
				"source-port", "1701", "destination-port", "1701",
				"tunnel-id", "1", "peer-tunnel-id", "100",
				"session-id", "1", "peer-session-id", "100"},
		},
	}

	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			oldHalf := runL2TPDiagnosticInNamespace(t, test.oldBinary, test.oldArgs, test.address)
			newHalf := runL2TPDiagnosticInNamespace(t, le, test.newArgs, test.address)
			if oldHalf != newHalf {
				t.Fatalf("full producer parity changed:\nold: %#v\nnew: %#v", oldHalf, newHalf)
			}
			if !strings.Contains(oldHalf.stdout, "netlink message (") {
				t.Fatalf("the compared output carried no exact netlink bytes:\n%s", oldHalf.stdout)
			}
			if oldHalf.code != 0 {
				t.Fatalf("the real diagnostic did not reach its complete path: %#v", oldHalf)
			}
		})
	}
}

func requireL2TPDiagnosticParityHost(t *testing.T) {
	t.Helper()
	if os.Geteuid() != 0 {
		t.Skip("real L2TP parity requires root")
	}
	for _, program := range []string{"go", "ip"} {
		if _, err := exec.LookPath(program); err != nil {
			t.Skipf("real L2TP parity requires %s", program)
		}
	}
	if _, err := exec.LookPath("modprobe"); err == nil {
		for _, module := range []string{"ppp_generic", "l2tp_core", "l2tp_netlink", "pppox", "l2tp_ppp"} {
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			_ = exec.CommandContext(ctx, "modprobe", module).Run() //nolint:gosec // every argument is a constant above
			cancel()
		}
	}
	if body, err := exec.Command("ip", "l2tp", "show", "tunnel").CombinedOutput(); err != nil {
		t.Skipf("real L2TP parity requires the L2TP generic netlink family: %v: %s", err, body)
	}
	if info, err := os.Stat("/dev/ppp"); err != nil || info.Mode()&os.ModeCharDevice == 0 {
		t.Skip("real L2TP parity requires the /dev/ppp character device")
	}
}

func l2tpDiagnosticRoot(t *testing.T) string {
	t.Helper()
	root, err := filepath.Abs(filepath.Join("..", ".."))
	if err != nil {
		t.Fatalf("repository root: %v", err)
	}
	return root
}

func buildL2TPLeBinary(t *testing.T, root, output string) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	command := exec.CommandContext(ctx, "make", "--no-print-directory", "le-build",
		"ZE_BIN_DIR="+filepath.Dir(output))
	command.Dir = root
	command.Env = append(os.Environ(), "CGO_ENABLED=0")
	if body, err := command.CombinedOutput(); err != nil {
		t.Fatalf("build cmd/ze le personality: %v\n%s", err, body)
	}
}

func buildL2TPDiagnosticBinary(t *testing.T, root, output, pkg string) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	command := exec.CommandContext(ctx, "go", "build", "-buildvcs=false", "-o", output, pkg)
	command.Dir = root
	command.Env = append(os.Environ(), "CGO_ENABLED=0")
	if body, err := command.CombinedOutput(); err != nil {
		t.Fatalf("build %s: %v\n%s", pkg, err, body)
	}
}

func runL2TPDiagnosticInNamespace(t *testing.T, binary string, args []string, address string) l2tpDiagnosticHalf {
	t.Helper()
	cleanName := strings.Map(func(value rune) rune {
		if value == '/' || value == ' ' {
			return '-'
		}
		return value
	}, t.Name())
	var named textbuf.Buffer
	name := named.Str("ze-l2tp-").Int(int64(os.Getpid())).Byte('-').
		Str(filepath.Base(binary)).Byte('-').Str(cleanName).String()
	if len(name) > 63 {
		name = name[:63]
	}
	runIPDiagnostic(t, "netns", "add", name)
	t.Cleanup(func() {
		command := exec.Command("ip", "netns", "del", name)
		if body, err := command.CombinedOutput(); err != nil {
			t.Errorf("delete namespace %s: %v: %s", name, err, body)
		}
	})
	runIPDiagnostic(t, "-n", name, "link", "set", "lo", "up")
	if address != "" {
		runIPDiagnostic(t, "-n", name, "address", "add", address, "dev", "lo")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	argv := append([]string{"netns", "exec", name, binary}, args...)
	command := exec.CommandContext(ctx, "ip", argv...)
	command.Env = withoutL2TPRecordingEnvironment(os.Environ())
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	command.Stdout = &stdout
	command.Stderr = &stderr
	err := command.Run()
	code := 0
	if err != nil {
		var exit *exec.ExitError
		if !errors.As(err, &exit) {
			t.Fatalf("run %s: %v", binary, err)
		}
		code = exit.ExitCode()
	}
	return l2tpDiagnosticHalf{
		stdout: stdout.String(), stderr: stderr.String(), code: code,
		tunnels: l2tpKernelDump(t, name, "tunnel"),
		sessions: l2tpKernelDump(t, name, "session"),
	}
}

func runIPDiagnostic(t *testing.T, args ...string) {
	t.Helper()
	command := exec.Command("ip", args...)
	if body, err := command.CombinedOutput(); err != nil {
		var tb textbuf.Buffer
		t.Fatalf("ip %s: %v\n%s", tb.Join(args, " ").String(), err, body)
	}
}

func l2tpKernelDump(t *testing.T, namespace, kind string) string {
	t.Helper()
	command := exec.Command("ip", "netns", "exec", namespace, "ip", "-details", "l2tp", "show", kind)
	body, err := command.CombinedOutput()
	if err != nil {
		var tb textbuf.Buffer
		return tb.Str("exit=").Int(int64(command.ProcessState.ExitCode())).
			Str(" output=").Str(string(body)).String()
	}
	return string(body)
}

func withoutL2TPRecordingEnvironment(environment []string) []string {
	const prefix = "ZE_TEST_L2TP_DIAGNOSTIC_RECORD="
	result := make([]string, 0, len(environment))
	for _, entry := range environment {
		if !strings.HasPrefix(entry, prefix) {
			result = append(result, entry)
		}
	}
	return result
}
