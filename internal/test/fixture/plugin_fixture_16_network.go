package fixture

import (
	"context"
	"fmt"
	"io"
	"net"
	"os"
	"os/exec"
	"regexp"
	"strconv"
	"strings"
	"time"
)

func init() {
	Register("plugin/tacacs-show", plugin16TacacsShow)
}

func plugin16TacacsShow(ctx context.Context, _ []string) error {
	if err := os.WriteFile("daemon.ready", nil, 0o600); err != nil {
		return err
	}
	if err := os.Remove("mock.addr"); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("remove stale mock address: %w", err)
	}
	executable, err := os.Executable()
	if err != nil {
		return err
	}
	mockLog, err := os.OpenFile("mock.log", os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0o600)
	if err != nil {
		return err
	}
	defer mockLog.Close()
	mock := exec.Command(executable, "tacacs-mock", "--port", "0", "--key", "ze-mock-key", "--user", "admin:testpass:15", "--addr-file", "mock.addr")
	mock.Stdout = io.Discard
	mock.Stderr = mockLog
	mockDone, err := plugin16StartProcess(mock)
	if err != nil {
		return fmt.Errorf("start tacacs mock: %w", err)
	}
	defer plugin16StopProcess(mock, mockDone)

	var mockAddress string
	if !Poll(ctx, 30, 100*time.Millisecond, func() bool {
		content, readErr := os.ReadFile("mock.addr")
		mockAddress = strings.TrimSpace(string(content))
		return readErr == nil && mockAddress != ""
	}) {
		_ = mockLog.Sync()
		content, _ := os.ReadFile("mock.log")
		return fmt.Errorf("mock did not report address: %s", content)
	}
	mockIP, mockPortText, err := net.SplitHostPort(mockAddress)
	if err != nil {
		return fmt.Errorf("parse mock address %q: %w", mockAddress, err)
	}
	mockPort, err := strconv.Atoi(mockPortText)
	if err != nil {
		return fmt.Errorf("parse mock port %q: %w", mockPortText, err)
	}
	dead, err := net.Listen("tcp4", "127.0.0.1:0")
	if err != nil {
		return fmt.Errorf("pick dead port: %w", err)
	}
	deadPort := dead.Addr().(*net.TCPAddr).Port
	if err := dead.Close(); err != nil {
		return err
	}
	fmt.Fprintf(os.Stderr, "dead port: %d\n", deadPort)
	config := fmt.Sprintf(`system {
	 authentication {
		tacacs {
			server %s { port %d; key "ze-mock-key"; }
			server 127.1.1.1 { port %d; key "never-up"; }
			timeout 1;
		}
	}
}
`, mockIP, mockPort, deadPort)
	if err := os.WriteFile("probe.conf", []byte(config), 0o600); err != nil {
		return err
	}
	table, _ := exec.CommandContext(ctx, "ze", "tacacs", "show", "probe.conf").CombinedOutput()
	fmt.Fprintln(os.Stderr, "--- table output ---")
	_, _ = os.Stderr.Write(table)
	if len(table) != 0 && table[len(table)-1] != '\n' {
		fmt.Fprintln(os.Stderr)
	}
	jsonOutput, jsonErr := exec.CommandContext(ctx, "ze", "tacacs", "show", "--json", "probe.conf").CombinedOutput()
	fmt.Fprintln(os.Stderr, "--- json output ---")
	_, _ = os.Stderr.Write(jsonOutput)
	if len(jsonOutput) != 0 && jsonOutput[len(jsonOutput)-1] != '\n' {
		fmt.Fprintln(os.Stderr)
	}
	if jsonErr != nil {
		return fmt.Errorf("--json exit: %w", jsonErr)
	}
	rowPattern := regexp.MustCompile(`(?m)^` + regexp.QuoteMeta(mockIP+":"+mockPortText) + `[[:space:]].* yes `)
	if !rowPattern.Match(table) {
		return fmt.Errorf("mock server row missing yes in table")
	}
	if !strings.Contains(string(table), " no ") {
		return fmt.Errorf("expected at least one no row")
	}
	fmt.Fprintln(os.Stderr, "OK: ze tacacs show reports mock reachable, second server unreachable")
	return nil
}
