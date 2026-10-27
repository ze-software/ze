// Design: docs/architecture/testing/interop.md -- mock RTR data for ASPA carriers.
package rtr

import (
	"bytes"
	"encoding/binary"
	"io"
	"net"
	"net/netip"
	"os"
	"path/filepath"
	"testing"
	"time"
)

// TestRTRMockASPAPDUWireFormat pins the draft-ietf-sidrops-8210bis-27 Section
// 5.12 layout independently of Ze's decoder: flags in the header, customer at
// offset 8, and providers at offset 12. The former 16-byte base is invalid.
func TestRTRMockASPAPDUWireFormat(t *testing.T) {
	t.Parallel()
	writer, reader := net.Pipe()
	t.Cleanup(func() {
		_ = reader.Close()
		_ = writer.Close()
	})
	if err := reader.SetReadDeadline(time.Now().Add(5 * time.Second)); err != nil {
		t.Fatal(err)
	}
	if err := writer.SetWriteDeadline(time.Now().Add(5 * time.Second)); err != nil {
		t.Fatal(err)
	}
	done := make(chan error, 1)
	go func() {
		done <- rtrMockSendASPAPDU(writer, aspaFlag{customerAS: 64502, providers: []uint32{64500, 64501}})
	}()
	want := []byte{2, 11, 1, 0, 0, 0, 0, 20, 0, 0, 0xfb, 0xf6, 0, 0, 0xfb, 0xf4, 0, 0, 0xfb, 0xf5}
	got := make([]byte, len(want))
	if _, err := io.ReadFull(reader, got); err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got, want) {
		t.Fatalf("ASPA PDU = %x, want %x", got, want)
	}
	if err := <-done; err != nil {
		t.Fatal(err)
	}
}

// TestRTRMockRunRefusesWhatItCannotServe checks the words and flags Run answers without
// serving: a lone help word prints the flags and succeeds, a stray word and an address that
// is not an IP are refused, and an outage missing its start or its release file is refused.
// Each case returns, so a case that started serving instead would hang the test.
func TestRTRMockRunRefusesWhatItCannotServe(t *testing.T) {
	cases := []struct {
		args []string
		want int
	}{
		{[]string{"help"}, 0},
		{[]string{"serve"}, 1},
		{[]string{"--address", "localhost"}, 1},
		{[]string{"--outage-start", "start"}, 1},
		{[]string{"--outage-release", "release"}, 1},
	}
	for _, c := range cases {
		if got := Run(c.args); got != c.want {
			t.Errorf("Run(%q) = %d, want %d", c.args, got, c.want)
		}
	}
}

// TestRTRMockOutageClosesAndReopensOnTheSameAddress drives one outage: queries are answered
// through End of Data until the start file exists, the listener is then closed so a new
// connection is refused, and once the release file exists the same address and port accept
// and answer again.
func TestRTRMockOutageClosesAndReopensOnTheSameAddress(t *testing.T) {
	t.Parallel()
	ln, err := rtrMockListen(netip.MustParseAddrPort("127.0.0.1:0"))
	if err != nil {
		t.Fatal(err)
	}
	bound := ln.Addr().String()
	dir := t.TempDir()
	start, release := filepath.Join(dir, "start"), filepath.Join(dir, "release")
	cache := &rtrMockCache{serial: 7, refresh: 1, retry: 1}

	type outcome struct {
		ln  net.Listener
		err error
	}
	done := make(chan outcome, 1)
	go func() {
		again, outageErr := rtrMockOutage(ln, cache, start, release)
		done <- outcome{again, outageErr}
	}()

	for range 2 {
		if serial := rtrMockQueryForTest(t, bound); serial != 7 {
			t.Fatalf("End of Data serial = %d, want 7", serial)
		}
	}
	if err := os.WriteFile(start, nil, 0o600); err != nil {
		t.Fatal(err)
	}
	deadline := time.Now().Add(5 * time.Second)
	for {
		conn, dialErr := rtrMockDialForTest(t, bound)
		if dialErr != nil {
			break
		}
		_ = conn.Close()
		if time.Now().After(deadline) {
			t.Fatal("the cache still accepts connections after its outage began")
		}
		time.Sleep(20 * time.Millisecond)
	}

	if err := os.WriteFile(release, nil, 0o600); err != nil {
		t.Fatal(err)
	}
	got := <-done
	if got.err != nil {
		t.Fatal(got.err)
	}
	t.Cleanup(func() { _ = got.ln.Close() })
	if got.ln.Addr().String() != bound {
		t.Fatalf("listening again on %s, want %s", got.ln.Addr(), bound)
	}
	go rtrMockServe(got.ln, cache)
	if serial := rtrMockQueryForTest(t, bound); serial != 7 {
		t.Fatalf("End of Data serial after the outage = %d, want 7", serial)
	}
}

// rtrMockDialForTest opens one TCP connection to address, bounded by a second.
func rtrMockDialForTest(t *testing.T, address string) (net.Conn, error) {
	t.Helper()
	dialer := &net.Dialer{Timeout: time.Second}
	return dialer.DialContext(t.Context(), "tcp", address)
}

// rtrMockQueryForTest sends one version 1 Reset Query to address, reads the Cache Response
// and the End of Data that follows (the cache holds no records), and answers its serial.
func rtrMockQueryForTest(t *testing.T, address string) uint32 {
	t.Helper()
	conn, err := rtrMockDialForTest(t, address)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = conn.Close() }()
	if err := conn.SetDeadline(time.Now().Add(5 * time.Second)); err != nil {
		t.Fatal(err)
	}
	if _, err := conn.Write([]byte{1, rtrMockPDUResetQuery, 0, 0, 0, 0, 0, 8}); err != nil {
		t.Fatal(err)
	}
	reply := make([]byte, rtrMockHeaderLen+24)
	if _, err := io.ReadFull(conn, reply); err != nil {
		t.Fatal(err)
	}
	if reply[1] != rtrMockPDUCacheResp {
		t.Fatalf("first PDU type %d, want Cache Response", reply[1])
	}
	eod := reply[rtrMockHeaderLen:]
	if eod[1] != rtrMockPDUEndOfData {
		t.Fatalf("second PDU type %d, want End of Data", eod[1])
	}
	return binary.BigEndian.Uint32(eod[8:12])
}
