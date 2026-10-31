//go:build docker && linux

// Design: docs/architecture/testing/interop.md -- a package test's own client container (labtest)
// Related: ../../../test/interop-tftp/Dockerfile.tftp-hpa -- the tftp-hpa 7.2 client image
// Related: window_send.go -- serveFile, the windowed sender under test
//
// Independent clients against Ze's production TFTP handlers, through a relay
// that copies every datagram both ways and keeps a record of what the server
// sent. The relay constructs nothing: the client owns its state machine.
package tftpserver

import (
	"bytes"
	"crypto/sha256"
	"encoding/binary"
	"errors"
	"net"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/ze-software/ze/internal/core/slogutil"
	"github.com/ze-software/ze/internal/core/textbuf"
	"github.com/ze-software/ze/internal/core/tftp"
	"github.com/ze-software/ze/internal/le/interoplab"
	"github.com/ze-software/ze/internal/le/interoplab/labtest"
)

// TestTFTPHPAWindowsizeInterop downloads a 300-block file from Ze's server with
// the tftp-hpa 7.2 client, an RFC 7440 data receiver Ze shares no code with:
// once with its default request (windowsize 64, which Ze grants whole) and once
// with -W 1000 (granted the server maximum, 64). Each download must carry
// windowsize 64 in the OACK, must be sent in windows (more than one DATA block
// between two ACKs, which lockstep never does), and must deliver the exact
// file.
//
// MUTATION: make grantWindowsize return tftp.WindowsizeMin. The OACK then
// carries no windowsize and the transfer is lockstep, so the test fails on the
// OACK and on the window count while the download itself still succeeds.
func TestTFTPHPAWindowsizeInterop(t *testing.T) {
	client := labtest.StartContainer(t, labtest.Spec{
		Name:       "tftp-hpa-client",
		Image:      "ze-tftp-hpa:interop",
		Dockerfile: "test/interop-tftp/Dockerfile.tftp-hpa",
		// labtest.Spec declares a server's port and refuses none. This
		// container serves nothing: it idles while the test runs the client
		// in it. The port names TFTP's, which nothing in it binds, and the
		// host network publishes no port.
		Port:             interoplab.UDPPort(69),
		HostNetwork:      true,
		ReadyInContainer: []string{"tftp", "-V"},
		ReadyTimeout:     60 * time.Second,
		LogLines:         20,
	})
	version, err := client.Exec(t.Context(), "tftp", "-V")
	if err != nil {
		t.Fatalf("tftp -V: %v", err)
	}
	if !strings.HasPrefix(version.Stdout, "tftp-hpa 7.2") {
		t.Fatalf("unexpected independent client version: %q", version.Stdout)
	}

	root := t.TempDir()
	want := patterned(300 * tftp.DefaultBlockSize)
	if err := os.WriteFile(filepath.Join(root, "boot.bin"), want, 0o600); err != nil {
		t.Fatal(err)
	}
	sum := sha256.Sum256(want)
	server := startInteropTFTPServer(t, root)

	for name, tc := range map[string]struct {
		args      []string
		requested string
	}{
		"default request": {requested: "64"},
		"windowsize 1000": {args: []string{"-W", "1000"}, requested: "1000"},
	} {
		t.Run(name, func(t *testing.T) {
			relay := startOptionRelay(t, server)
			argv := []string{"tftp", "-m", "octet", "-B", "512"}
			argv = append(argv, tc.args...)
			argv = append(argv, "127.0.0.1", strconv.Itoa(relay.port()),
				"-c", "get", "boot.bin", "/tmp/boot.bin")
			result, err := client.Exec(t.Context(), argv...)
			if err != nil {
				t.Fatalf("tftp get: %v (stdout %q stderr %q)", err, result.Stdout, result.Stderr)
			}
			digest, err := client.Exec(t.Context(), "sha256sum", "/tmp/boot.bin")
			if err != nil {
				t.Fatalf("sha256sum: %v", err)
			}
			if !strings.HasPrefix(digest.Stdout, textbuf.StringHex(sum[:])) {
				t.Fatalf("tftp-hpa downloaded a file with digest %q, want %x", digest.Stdout, sum)
			}
			record := relay.finish(t)
			if record.request["windowsize"] != tc.requested {
				t.Fatalf("tftp-hpa requested windowsize %q, want %s (request %v)", record.request["windowsize"], tc.requested, record.request)
			}
			if record.oack["windowsize"] != "64" {
				t.Fatalf("OACK windowsize = %q, want 64 (OACK %v)", record.oack["windowsize"], record.oack)
			}
			if record.longestRun < 2 {
				t.Fatalf("at most %d DATA block between two ACKs: the transfer was lockstep", record.longestRun)
			}
			t.Logf("tftp-hpa 7.2: OACK %v, longest DATA run between ACKs %d, %d datagrams", record.oack, record.longestRun, record.datagrams)
		})
	}
}

// TestTFTPCurlTimeoutInterop downloads a file from Ze's server with curl
// 8.23.0 and its default options, which carry the RFC 2349 timeout curl
// derives from --max-time (25 s sends "timeout 5"). The OACK must acknowledge
// that timeout with curl's own value, beside tsize and blksize, and curl must
// accept the OACK and deliver the exact file.
//
// MUTATION: make sendOACKAndWait skip the timeout option. The OACK then
// carries no timeout and the test fails on it while curl still succeeds.
func TestTFTPCurlTimeoutInterop(t *testing.T) {
	root := t.TempDir()
	want := patterned(5000)
	if err := os.WriteFile(filepath.Join(root, "boot.bin"), want, 0o600); err != nil {
		t.Fatal(err)
	}
	server := startInteropTFTPServer(t, root)
	relay := startOptionRelay(t, server)

	result := labtest.Run(t, labtest.Client{
		Name: "tftp-curl-timeout", Image: tftpCurlImage, HostNetwork: true,
		Command: []string{"--silent", "--show-error", "--noproxy", "*",
			"--max-time", "25", "--output", "-",
			"tftp://127.0.0.1:" + strconv.Itoa(relay.port()) + "/boot.bin"},
		Timeout: 40 * time.Second,
	})
	if result.ExitCode != 0 {
		t.Fatalf("curl transfer exited %d: %s", result.ExitCode, result.Stderr)
	}
	if !bytes.Equal([]byte(result.Stdout), want) {
		t.Fatalf("curl downloaded %d bytes, want the exact %d-byte file", len(result.Stdout), len(want))
	}
	record := relay.finish(t)
	requested := record.request["timeout"]
	if requested == "" {
		t.Fatalf("curl sent no timeout option: request %v", record.request)
	}
	if record.oack["timeout"] != requested {
		t.Fatalf("OACK timeout = %q, want curl's own %q (request %v, OACK %v)",
			record.oack["timeout"], requested, record.request, record.oack)
	}
	t.Logf("curl 8.23.0: request %v, OACK %v", record.request, record.oack)
}

// startInteropTFTPServer serves root on a loopback port under the YANG
// defaults and answers its address. Cleanup closes the listener and ends every
// transfer.
func startInteropTFTPServer(t *testing.T, root string) *net.UDPAddr {
	t.Helper()
	conn, err := net.ListenUDP("udp4", &net.UDPAddr{IP: net.IPv4(127, 0, 0, 1)})
	if err != nil {
		t.Fatal(err)
	}
	stop := ServeConn(conn, root, 4, slogutil.DiscardLogger())
	t.Cleanup(stop)
	address, ok := conn.LocalAddr().(*net.UDPAddr)
	if !ok {
		t.Fatal("TFTP listener has a non-UDP local address")
	}
	return address
}

// relayRecord is what the relay saw of one transfer.
type relayRecord struct {
	request    map[string]string // the options of the client's RRQ
	oack       map[string]string // the options of the server's OACK
	longestRun int               // the most DATA blocks the server sent between two ACKs
	datagrams  int
}

// optionRelay copies one transfer's datagrams between a client and Ze's
// server: the client talks to the relay's port, and the relay talks to the
// server from a socket of its own, so the server's transfer port answers the
// relay. It is bounded by its deadline and by the datagram limit. Its cleanup
// closes both sockets and joins both workers.
type optionRelay struct {
	front, back *net.UDPConn
	server      *net.UDPAddr
	deadline    time.Time
	mu          sync.Mutex
	client      *net.UDPAddr
	transfer    *net.UDPAddr
	record      relayRecord
	run         int
	err         error
	done        sync.WaitGroup
}

// relayDatagramsMax bounds one relayed transfer: 300 blocks, their ACKs, and
// room for retransmissions.
const relayDatagramsMax = 5000

func startOptionRelay(t *testing.T, server *net.UDPAddr) *optionRelay {
	t.Helper()
	front, err := net.ListenUDP("udp4", &net.UDPAddr{IP: net.IPv4(127, 0, 0, 1)})
	if err != nil {
		t.Fatal(err)
	}
	back, err := net.ListenUDP("udp4", &net.UDPAddr{IP: net.IPv4(127, 0, 0, 1)})
	if err != nil {
		closeUDP(t, front)
		t.Fatal(err)
	}
	relay := &optionRelay{front: front, back: back, server: server, deadline: time.Now().Add(60 * time.Second)}
	relay.done.Add(2)
	go relay.fromClient()
	go relay.fromServer()
	t.Cleanup(func() {
		closeUDP(t, front)
		closeUDP(t, back)
		relay.done.Wait()
	})
	return relay
}

func (r *optionRelay) port() int {
	address, ok := r.front.LocalAddr().(*net.UDPAddr)
	if !ok {
		return 0
	}
	return address.Port
}

// fromClient copies the client's datagrams to the server: the RRQ to its
// listener, every later one to its transfer port.
func (r *optionRelay) fromClient() {
	defer r.done.Done()
	buf := make([]byte, 65536)
	for range relayDatagramsMax {
		if err := r.front.SetReadDeadline(r.deadline); err != nil {
			return
		}
		n, source, err := r.front.ReadFromUDP(buf)
		if err != nil {
			return
		}
		r.mu.Lock()
		r.client = source
		r.record.datagrams++
		target := r.transfer
		if n >= 2 && binary.BigEndian.Uint16(buf[:2]) == tftp.OpRRQ {
			r.record.request = requestOptions(buf[:n])
			target = r.server
		}
		if n >= 2 && binary.BigEndian.Uint16(buf[:2]) == tftp.OpACK {
			r.record.longestRun = max(r.record.longestRun, r.run)
			r.run = 0
		}
		r.mu.Unlock()
		if target == nil {
			r.fail(errors.New("a client datagram before the server answered"))
			return
		}
		if _, err := r.back.WriteToUDP(buf[:n], target); err != nil {
			return
		}
	}
}

// fromServer copies the server's datagrams to the client and records the
// OACK and the DATA runs.
func (r *optionRelay) fromServer() {
	defer r.done.Done()
	buf := make([]byte, 65536)
	for range relayDatagramsMax {
		if err := r.back.SetReadDeadline(r.deadline); err != nil {
			return
		}
		n, source, err := r.back.ReadFromUDP(buf)
		if err != nil {
			return
		}
		r.mu.Lock()
		r.transfer = source
		r.record.datagrams++
		if n >= 2 && binary.BigEndian.Uint16(buf[:2]) == tftp.OpOACK {
			r.record.oack = fieldPairs(buf[2:n])
		}
		if n >= 2 && binary.BigEndian.Uint16(buf[:2]) == tftp.OpDATA {
			r.run++
		}
		client := r.client
		r.mu.Unlock()
		if _, err := r.front.WriteToUDP(buf[:n], client); err != nil {
			return
		}
	}
}

func (r *optionRelay) fail(err error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.err == nil {
		r.err = err
	}
}

// finish answers what the relay saw. It MUST be called after the client
// exited.
func (r *optionRelay) finish(t *testing.T) relayRecord {
	t.Helper()
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.err != nil {
		t.Fatalf("relay: %v", r.err)
	}
	r.record.longestRun = max(r.record.longestRun, r.run)
	return r.record
}

// requestOptions answers the option pairs of an RRQ, after its filename and
// mode.
func requestOptions(pkt []byte) map[string]string {
	fields := bytes.Split(bytes.TrimSuffix(pkt[2:], []byte{0}), []byte{0})
	if len(fields) < 2 {
		return map[string]string{}
	}
	return pairs(fields[2:])
}

// fieldPairs answers the option pairs of an OACK body.
func fieldPairs(body []byte) map[string]string {
	return pairs(bytes.Split(bytes.TrimSuffix(body, []byte{0}), []byte{0}))
}

func pairs(fields [][]byte) map[string]string {
	options := map[string]string{}
	for i := 0; i+1 < len(fields); i += 2 {
		options[strings.ToLower(string(fields[i]))] = string(fields[i+1])
	}
	return options
}
