package cli

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/ze-software/ze/internal/test/runner"
)

// TestParseExaBGPCLIDUT reads `--dut` through the runner's own flag parser:
// ze when it is absent, exabgp when asked, and a refusal naming both spellings
// for anything else. A value read as ze in silence would run a proof against
// ze and report ExaBGP's agreement with a case that ExaBGP never saw.
func TestParseExaBGPCLIDUT(t *testing.T) {
	for _, one := range []struct {
		args []string
		want exabgpDUT
	}{
		{args: nil, want: exabgpDUTZe},
		{args: []string{"--dut", "ze"}, want: exabgpDUTZe},
		{args: []string{"--dut", "exabgp", "api-fast"}, want: exabgpDUTExaBGP},
	} {
		cli, err := parseExaBGPCLI(one.args)
		if err != nil {
			t.Fatalf("parseExaBGPCLI(%q): %v", one.args, err)
		}
		if cli.dut != one.want {
			t.Fatalf("parseExaBGPCLI(%q) dut = %d, want %d", one.args, cli.dut, one.want)
		}
	}

	_, err := parseExaBGPCLI([]string{"--dut", "gobgp"})
	if err == nil {
		t.Fatal("--dut gobgp was accepted")
	}
	for _, want := range []string{"gobgp", "ze", "exabgp"} {
		if !strings.Contains(err.Error(), want) {
			t.Fatalf("refusal %q does not name %q", err, want)
		}
	}
}

// fakeExaBGP puts an `exabgp` on PATH that runs body, so proof mode can be
// driven without a real ExaBGP.
func fakeExaBGP(t *testing.T, body string) {
	t.Helper()
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "exabgp"), []byte("#!/bin/sh\n"+body), 0o700); err != nil { //nolint:gosec // an executable test fixture
		t.Fatal(err)
	}
	t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))
}

// TestStartExaBGPDaemonHandsEveryConfig proves proof mode runs a reload case
// the way ExaBGP's own runner does: `exabgp server` gets every config the case
// names, in fixture order, so ExaBGP's reload takes the next one. Method: a
// fake exabgp that prints its arguments.
func TestStartExaBGPDaemonHandsEveryConfig(t *testing.T) {
	fakeExaBGP(t, "echo \"$@\"\n")
	test := &exabgpTestEntry{
		record:         &runner.Record{Nick: "34"},
		configs:        []string{"api-reload.1.conf", "api-reload.2.conf"},
		tcpConnections: 1,
	}
	client, configs, err := startExaBGPDaemon(t.Context(), test, 17900, "/resolved/ze")
	if err != nil {
		t.Fatalf("a two-config case was refused: %v", err)
	}
	<-client.done
	if got := strings.TrimSpace(client.stdout.String()); got != "server api-reload.1.conf api-reload.2.conf" {
		t.Fatalf("exabgp ran with %q, want every config after server", got)
	}
	if configs.path != "api-reload.1.conf" || len(configs.reloads) != 0 {
		t.Fatalf("configs = %+v, want the first config and nothing to rewrite", configs)
	}
}

// TestDeliverExaBGPDaemonSignals proves a signal step reaches the ExaBGP under
// proof by the name the fixture gives it, and a name that is no signal is
// reported rather than dropped. Method: a fake exabgp that traps SIGUSR1 and
// prints when it gets one.
func TestDeliverExaBGPDaemonSignals(t *testing.T) {
	fakeExaBGP(t, "trap 'echo got-usr1; exit 0' USR1\necho ready\nwhile :; do sleep 0.05; done\n")
	test := &exabgpTestEntry{
		record:         &runner.Record{Nick: "34"},
		configs:        []string{"api-reload.1.conf", "api-reload.2.conf"},
		tcpConnections: 1,
	}
	client, _, err := startExaBGPDaemon(t.Context(), test, 17900, "/resolved/ze")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { stopExaProcess(client) })
	deadline := time.Now().Add(5 * time.Second)
	for !strings.Contains(client.stdout.String(), "ready") {
		if time.Now().After(deadline) {
			t.Fatalf("the fake exabgp never got ready: %q", client.stdout.String())
		}
		time.Sleep(10 * time.Millisecond)
	}

	signals := make(chan string, 2)
	signals <- "SIGNOPE"
	signals <- "SIGUSR1"
	close(signals)
	deliverExaBGPDaemonSignals(signals, client)

	select {
	case <-client.done:
	case <-time.After(5 * time.Second):
		t.Fatalf("SIGUSR1 never reached exabgp: %q", client.stdout.String())
	}
	output := client.stdout.String()
	if !strings.Contains(output, "got-usr1") {
		t.Errorf("exabgp output %q does not show the SIGUSR1", output)
	}
	if !strings.Contains(output, "SIGNOPE is not a signal name") {
		t.Errorf("output %q does not report the bad signal name", output)
	}
}

// TestStartExaBGPDaemonNeedsExaBGPOnPath runs proof mode with a PATH holding
// no exabgp, and asserts the refusal says how to put one there rather than
// the exec package's bare not-found.
func TestStartExaBGPDaemonNeedsExaBGPOnPath(t *testing.T) {
	t.Setenv("PATH", t.TempDir())
	test := &exabgpTestEntry{
		record:         &runner.Record{Nick: "1"},
		configs:        []string{"api-fast.conf"},
		tcpConnections: 1,
	}
	_, _, err := startExaBGPDaemon(t.Context(), test, 17900, "/resolved/ze")
	if err == nil {
		t.Fatal("proof mode started with no exabgp on PATH")
	}
	if !strings.Contains(err.Error(), "uv run --with") {
		t.Fatalf("refusal %q does not say how to put exabgp on PATH", err)
	}
}
