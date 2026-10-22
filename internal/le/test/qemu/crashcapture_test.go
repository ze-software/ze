package testqemu

import (
	"bufio"
	"bytes"
	"errors"
	"net"
	"slices"
	"strings"
	"testing"
	"time"
)

// VALIDATES: the crash listing reader picks out kernel-kind artifacts by name
// and ignores userspace crash files and malformed entries.
// PREVENTS: a panic proof passing on a userspace crash file, or an OTA proof
// failing on one.
func TestCrashListingKernelArtifacts(t *testing.T) {
	listing := crashListing{crashes: []any{
		map[string]any{"name": "crash-1.log", "kind": "panic"},
		map[string]any{"name": "crash-2-dmesg-ramoops-0-kernel.log", "kind": "kernel"},
		"not an object",
		map[string]any{"kind": "kernel"},
	}}
	got := listing.kernelArtifacts()
	want := []string{"crash-2-dmesg-ramoops-0-kernel.log"}
	if !slices.Equal(got, want) {
		t.Fatalf("kernel artifacts = %v, want %v", got, want)
	}
}

// VALIDATES: findKey reaches a key nested inside objects and arrays, and stops
// at its depth bound.
// PREVENTS: a readiness block wrapped by the daemon's envelope being missed.
func TestCrashFindKeyIsBounded(t *testing.T) {
	document := map[string]any{"data": []any{map[string]any{"readiness": "here"}}}
	if got := findKey(document, "readiness", findDepthMax); got != "here" {
		t.Fatalf("findKey = %v, want here", got)
	}
	if got := findKey(document, "readiness", 2); got != nil {
		t.Fatalf("findKey at depth 2 = %v, want nil", got)
	}
}

// VALIDATES: the boot time is read from `show host kernel | json`, and an
// answer without one is refused rather than read as zero.
// PREVENTS: a missing field reading as a boot time that never changes.
func TestCrashBootTimeOf(t *testing.T) {
	booted, ok := bootTimeOf(`{"kernel":{"release":"7.2","boot-time-unix":1760000000}}`)
	if !ok || booted != 1760000000 {
		t.Fatalf("bootTimeOf = %d, %v; want 1760000000, true", booted, ok)
	}
	if _, ok := bootTimeOf(`{"kernel":{"release":"7.2"}}`); ok {
		t.Fatal("bootTimeOf accepted an answer with no boot time")
	}
	if _, ok := bootTimeOf("not json"); ok {
		t.Fatal("bootTimeOf accepted text that is not JSON")
	}
}

// VALIDATES: the VM arguments add the update-server forward to the SSH NIC and
// a monitor on its own port.
// PREVENTS: an OTA proof whose POST reaches nothing, or an NMI with no monitor.
func TestCrashQEMUArgsAddMonitorAndForward(t *testing.T) {
	lab := &CrashCapture{run: &Hugepages{Arch: ArchAMD64}, lab: CrashLabOTAUnaffected}
	argv := lab.qemuArgs("image.img", crashPorts{ssh: 2201, monitor: 2202, http: 2203}, HugepagesReport{Accelerator: "kvm", MemoryMiB: 1024})
	joined := strings.Join(argv, " ")
	for _, want := range []string{"hostfwd=tcp:127.0.0.1:2201-:22,hostfwd=tcp:127.0.0.1:2203-:80", "-monitor tcp:127.0.0.1:2202,server=on,wait=off"} {
		if !strings.Contains(joined, want) {
			t.Errorf("argv %q lacks %q", joined, want)
		}
	}
}

// VALIDATES: the panic line is found inside the artifact text.
// PREVENTS: an artifact without the panic passing the harvest proof.
func TestCrashLineWith(t *testing.T) {
	text := "=== Kernel Crash ===\nKind: kernel\n\n  Kernel panic - not syncing: NMI: Not continuing\n"
	if got := lineWith(text, panicText); got != "Kernel panic - not syncing: NMI: Not continuing" {
		t.Fatalf("lineWith = %q", got)
	}
	if got := lineWith("Kind: kernel\n", panicText); got != "" {
		t.Fatalf("lineWith on text without a panic = %q, want empty", got)
	}
}

// monitorBanner is the greeting QEMU's human monitor sends on connect, before
// it has read any command.
const monitorBanner = "QEMU 8.2.2 monitor - type 'help' for more information\r\n(qemu) "

// monitorProcessing is how long the fake monitor waits after reading a command
// line before it runs it, standing in for QEMU's main loop.
const monitorProcessing = 300 * time.Millisecond

// fakeMonitor imitates QEMU's human monitor on one TCP connection. It sends
// the banner and the prompt first, reads one command line, and runs it only if
// the client is still connected once the main loop reaches it, because QEMU
// drops a line it has not processed when the connection closes. With answer
// set it then echoes the line and prints the next prompt; without it, it closes
// instead. What happened to the line goes on outcome.
type fakeMonitor struct {
	listener net.Listener
	answer   bool
	outcome  chan string
}

// newFakeMonitor listens on a loopback port and serves one connection. The
// caller MUST call close when the test ends.
func newFakeMonitor(t *testing.T, answer bool) *fakeMonitor {
	t.Helper()
	var config net.ListenConfig
	listener, err := config.Listen(t.Context(), "tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	m := &fakeMonitor{listener: listener, answer: answer, outcome: make(chan string, 1)}
	go m.serve()
	return m
}

func (m *fakeMonitor) port() int {
	address, ok := m.listener.Addr().(*net.TCPAddr)
	if !ok {
		return 0
	}
	return address.Port
}

func (m *fakeMonitor) close() {
	m.listener.Close() //nolint:errcheck // the test is over
}

func (m *fakeMonitor) serve() {
	conn, err := m.listener.Accept()
	if err != nil {
		m.outcome <- "accept: " + err.Error()
		return
	}
	defer conn.Close() //nolint:errcheck // the fake is done with the connection

	if err := conn.SetDeadline(time.Now().Add(10 * time.Second)); err != nil {
		m.outcome <- "deadline: " + err.Error()
		return
	}
	if _, err := conn.Write([]byte(monitorBanner)); err != nil {
		m.outcome <- "banner: " + err.Error()
		return
	}
	reader := bufio.NewReader(conn)
	line, err := reader.ReadString('\n')
	if err != nil {
		m.outcome <- "no command line: " + err.Error()
		return
	}
	if err := conn.SetReadDeadline(time.Now().Add(monitorProcessing)); err != nil {
		m.outcome <- "deadline: " + err.Error()
		return
	}
	// A read that times out means the client is still connected; one that
	// ends means it closed before the main loop reached the line.
	_, err = reader.ReadByte()
	var timeout net.Error
	if !errors.As(err, &timeout) || !timeout.Timeout() {
		m.outcome <- "dropped " + strings.TrimSpace(line)
		return
	}
	m.outcome <- "ran " + strings.TrimSpace(line)
	if !m.answer {
		return
	}
	if _, err := conn.Write([]byte(strings.TrimSpace(line) + "\r\n(qemu) ")); err != nil {
		return
	}
	// Hold the connection until the client closes it, as QEMU does.
	if err := conn.SetReadDeadline(time.Now().Add(10 * time.Second)); err != nil {
		return
	}
	_, _ = reader.ReadByte() //nolint:errcheck // any end is the client leaving
}

func (m *fakeMonitor) result(t *testing.T) string {
	t.Helper()
	select {
	case outcome := <-m.outcome:
		return outcome
	case <-time.After(10 * time.Second):
		t.Fatal("the fake monitor reported nothing")
	}
	return ""
}

// VALIDATES: injectNMI keeps the monitor connection open until the monitor
// has run the nmi line, against a monitor that greets with a banner and a
// prompt before it reads anything.
// PREVENTS: the 2026-10-13 regression, where one read returned the connect
// banner, the connection closed, QEMU dropped the unprocessed line, and no NMI
// reached the guest while injectNMI reported success.
func TestCrashInjectNMIWaitsForTheCommandToRun(t *testing.T) {
	m := newFakeMonitor(t, true)
	defer m.close()

	if failure := injectNMI(m.port()); failure != "" {
		t.Fatalf("injectNMI = %q, want success", failure)
	}
	if got := m.result(t); got != "ran nmi" {
		t.Fatalf("the monitor reports %q, want %q", got, "ran nmi")
	}
}

// VALIDATES: injectNMI fails when the monitor closes without the prompt that
// follows the command.
// PREVENTS: a monitor that never confirmed the command being read as one that
// ran it.
func TestCrashInjectNMIRefusesAMonitorThatNeverAnswers(t *testing.T) {
	m := newFakeMonitor(t, false)
	defer m.close()

	failure := injectNMI(m.port())
	if failure == "" {
		t.Fatal("injectNMI succeeded against a monitor that printed no prompt after nmi")
	}
	if !strings.Contains(failure, "nmi") {
		t.Fatalf("injectNMI = %q, want the failure to name the nmi command", failure)
	}
	m.result(t)
}

// VALIDATES: awaitMonitorPrompt finds a prompt split across two reads, finds
// one that ends exactly at its byte bound, and stops at the bound when no
// prompt ends within it, even when one would end a few bytes later.
// PREVENTS: an endless read of a monitor that never prints a prompt, a bound
// that lets a read run past monitorReplyMax, and a prompt missed because QEMU
// wrote it in two pieces.
// Each write lands on a net.Pipe as its own read, so a case's writes are the
// reads awaitMonitorPrompt sees.
func TestCrashAwaitMonitorPromptIsBounded(t *testing.T) {
	padding := func(octets int) []byte { return bytes.Repeat([]byte("x"), octets) }
	cases := []struct {
		name   string
		writes [][]byte
		want   error
	}{
		{"prompt split across two reads", [][]byte{[]byte("(qe"), []byte("mu) ")}, nil},
		{"prompt ends at the bound", [][]byte{padding(monitorReplyMax - len(monitorPrompt)), []byte(monitorPrompt)}, nil},
		{"prompt ends one byte past the bound", [][]byte{padding(monitorReplyMax - len(monitorPrompt) + 1), []byte(monitorPrompt)}, errNoMonitorPrompt},
		{"no prompt within the bound", [][]byte{padding(monitorReplyMax + 512)}, errNoMonitorPrompt},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			client, server := net.Pipe()
			defer client.Close() //nolint:errcheck // the test is over
			defer server.Close() //nolint:errcheck // the test is over

			go writeAll(server, tc.writes)
			if err := client.SetDeadline(time.Now().Add(10 * time.Second)); err != nil {
				t.Fatalf("deadline: %v", err)
			}
			if got := awaitMonitorPrompt(client); !errors.Is(got, tc.want) {
				t.Fatalf("awaitMonitorPrompt = %v, want %v", got, tc.want)
			}
		})
	}
}

// writeAll writes each piece to conn in order and stops at the first error,
// which is the reader closing early by design.
func writeAll(conn net.Conn, writes [][]byte) {
	for _, data := range writes {
		if _, err := conn.Write(data); err != nil {
			return
		}
	}
}
