package dhcpserver

import (
	"bytes"
	"log/slog"
	"net"
	"strings"
	"testing"
)

func captureLogger() (*slog.Logger, *bytes.Buffer) {
	var buf bytes.Buffer
	h := slog.NewTextHandler(&buf, &slog.HandlerOptions{Level: slog.LevelDebug})
	return slog.New(h), &buf
}

func TestLogExchangePXEOffer(t *testing.T) {
	h := newTestPXEServer(t)
	mac := net.HardwareAddr{0x60, 0xbe, 0xb4, 0x20, 0xe8, 0xd4}
	req := buildPXEDiscover(mac, 0x1234, pxeArchUEFIx64)
	resp := h.handle(req)
	if resp == nil {
		t.Fatal("expected an OFFER reply")
	}

	log, buf := captureLogger()
	logExchange(log, req, resp)
	out := buf.String()

	for _, want := range []string{"OFFER", mac.String(), "bootfile"} {
		if !strings.Contains(out, want) {
			t.Errorf("log %q missing %q", out, want)
		}
	}
}

func TestLogExchangeNoReply(t *testing.T) {
	mac := net.HardwareAddr{0xaa, 0xbb, 0xcc, 0xdd, 0xee, 0xff}
	req := buildPXEDiscover(mac, 0x1, pxeArchUEFIx64)

	log, buf := captureLogger()
	logExchange(log, req, nil)
	out := buf.String()

	if !strings.Contains(out, "no reply") || !strings.Contains(out, mac.String()) {
		t.Errorf("expected a no-reply log carrying the MAC, got %q", out)
	}
}

func TestLogExchangeShortPacketIgnored(t *testing.T) {
	log, buf := captureLogger()
	logExchange(log, []byte{1, 2, 3}, nil)
	if buf.Len() != 0 {
		t.Errorf("expected no log for a runt packet, got %q", buf.String())
	}
}

func TestMsgTypeName(t *testing.T) {
	cases := map[byte]string{
		msgDiscover: "DISCOVER",
		msgOffer:    "OFFER",
		msgRequest:  "REQUEST",
		msgAck:      "ACK",
		msgNak:      "NAK",
		0xff:        "UNKNOWN",
	}
	for code, want := range cases {
		if got := msgTypeName(code); got != want {
			t.Errorf("msgTypeName(%d) = %q, want %q", code, got, want)
		}
	}
}
