// Design: docs/guide/rpki.md -- local cache timing controls.

package rpki

import (
	"encoding/binary"
	"io"
	"net"
	"testing"
	"time"
)

// TestRPKITimerBounds checks both RFC limits before conversion to wire integers.
func TestRPKITimerBounds(t *testing.T) {
	for _, tc := range []struct {
		name           string
		refresh, retry uint64
		expire         uint32
		valid          bool
	}{
		{"defaults", 3600, 600, 7200, true},
		{"minimum", 1, 1, 7200, true},
		{"showcase", 2, 1, 7200, true},
		{"refresh maximum", 86400, 1, 86401, true},
		{"retry maximum", 1, 7200, 7201, true},
		{"refresh zero", 0, 1, 0, false},
		{"refresh too large", 86401, 1, 0, false},
		{"retry zero", 1, 0, 0, false},
		{"retry too large", 1, 7201, 0, false},
		{"refresh overflow", 1 << 32, 1, 0, false},
		{"retry overflow", 1, 1 << 32, 0, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			timers, err := newRPKITimers(tc.refresh, tc.retry)
			if !tc.valid {
				if err == nil {
					t.Fatal("out-of-range timer accepted")
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if uint64(timers.refreshSeconds) != tc.refresh || uint64(timers.retrySeconds) != tc.retry || timers.expireSeconds != tc.expire {
				t.Fatalf("unexpected wire timers: %+v", timers)
			}
		})
	}
}

// TestRPKIEndOfDataTimers reads the real response writer, not a second encoder.
func TestRPKIEndOfDataTimers(t *testing.T) {
	timers, err := newRPKITimers(2, 1)
	if err != nil {
		t.Fatal(err)
	}
	server, client := net.Pipe()
	t.Cleanup(func() { _ = server.Close(); _ = client.Close() })
	if err := client.SetDeadline(time.Now().Add(5 * time.Second)); err != nil {
		t.Fatal(err)
	}
	result := make(chan error, 1)
	go sendTimerResponse(server, timers, result)
	for range expectedResponsePDUs {
		var header [8]byte
		if _, err := io.ReadFull(client, header[:]); err != nil {
			t.Fatal(err)
		}
		length := binary.BigEndian.Uint32(header[4:8])
		if length < 8 || length > 24 {
			t.Fatalf("invalid PDU length %d", length)
		}
		var body [16]byte
		if _, err := io.ReadFull(client, body[:length-8]); err != nil {
			t.Fatal(err)
		}
		if header[1] != rpkiPDUEndOfData {
			continue
		}
		if binary.BigEndian.Uint32(body[4:8]) != 2 || binary.BigEndian.Uint32(body[8:12]) != 1 || binary.BigEndian.Uint32(body[12:16]) != 7200 {
			t.Fatalf("wrong End of Data timers: %x", body)
		}
		if err := <-result; err != nil {
			t.Fatal(err)
		}
		return
	}
	t.Fatal("response has no End of Data")
}

// The response is one Cache Response, 171 generated VRPs and one End of Data.
const expectedResponsePDUs = 173

func sendTimerResponse(conn net.Conn, timers rpkiTimers, result chan<- error) {
	server := rpkiServer{validASN: 65001, invalidASN: 65099, serial: 1, timers: timers}
	result <- server.sendResponse(conn)
}
