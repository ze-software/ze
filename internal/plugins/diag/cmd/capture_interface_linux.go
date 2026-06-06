// Design: plan/spec-diag-capture-interface.md -- AF_PACKET live capture

//go:build linux

package cmd

import (
	"bytes"
	"context"
	"encoding/base64"
	"fmt"
	"net"
	"time"

	"github.com/mdlayher/packet"
	"github.com/packetcap/go-pcap/filter"
	"golang.org/x/net/bpf"

	"codeberg.org/thomas-mangin/ze/internal/component/plugin"
	pluginserver "codeberg.org/thomas-mangin/ze/internal/component/plugin/server"
)

const maxPcapBufSize = 64 << 20

func HandleCaptureInterface(_ *pluginserver.CommandContext, args []string) (*plugin.Response, error) {
	ca, err := parseCaptureArgs(args)
	if err != nil {
		return &plugin.Response{Status: plugin.StatusError, Error: err.Error()}, nil //nolint:nilerr // operational error in Response
	}

	ifc, lookupErr := net.InterfaceByName(ca.iface)
	if lookupErr != nil {
		return &plugin.Response{Status: plugin.StatusError, Error: "interface not found: " + ca.iface}, nil
	}

	if _, loaded := activeCaptures.LoadOrStore(ca.iface, true); loaded {
		return &plugin.Response{Status: plugin.StatusError, Error: "capture already active on " + ca.iface}, nil
	}
	defer activeCaptures.Delete(ca.iface)

	var rawInsns []bpf.RawInstruction
	if ca.filter != "" {
		rawInsns, err = compileBPF(ca.filter)
		if err != nil {
			return &plugin.Response{Status: plugin.StatusError, Error: "filter compilation failed: " + err.Error()}, nil
		}
	}

	conn, err := packet.Listen(ifc, packet.Raw, 0, nil)
	if err != nil {
		return &plugin.Response{Status: plugin.StatusError, Error: "capture: " + err.Error() + " (requires CAP_NET_RAW)"}, nil
	}
	defer func() { _ = conn.Close() }()

	if len(rawInsns) > 0 {
		if setErr := conn.SetBPF(rawInsns); setErr != nil {
			return &plugin.Response{Status: plugin.StatusError, Error: "BPF attach failed: " + setErr.Error()}, nil
		}
	}

	ctx, cancel := context.WithTimeout(context.Background(), ca.dur)
	defer cancel()

	switch ca.format {
	case captureFormatText:
		return captureText(ctx, conn, ca)
	default:
		return capturePcap(ctx, conn, ca)
	}
}

func capturePcap(ctx context.Context, conn *packet.Conn, ca captureArgs) (*plugin.Response, error) {
	var buf bytes.Buffer
	snapLen := uint32(ca.snapLen)
	if err := writePcapHeader(&buf, snapLen, linkTypeEthernet); err != nil {
		return nil, fmt.Errorf("capture: pcap header: %w", err)
	}

	rb := make([]byte, maxCaptureSnapLen)
	captured := 0

	for captured < ca.count {
		if ctx.Err() != nil {
			break
		}
		if buf.Len() > maxPcapBufSize {
			break
		}
		deadline, ok := ctx.Deadline()
		if ok {
			if setErr := conn.SetReadDeadline(deadline); setErr != nil {
				break
			}
		}

		n, _, readErr := conn.ReadFrom(rb)
		if readErr != nil {
			break
		}
		if n == 0 {
			continue
		}

		ts := time.Now()
		data := rb[:n]
		if n > ca.snapLen {
			data = rb[:ca.snapLen]
		}
		if err := writePcapPacketWithOrigLen(&buf, ts, data, n); err != nil {
			break
		}
		captured++
	}

	return &plugin.Response{
		Status: plugin.StatusDone,
		Data: plugin.Map{
			"format":   captureFormatPcap,
			"packets":  captured,
			"pcap":     base64.StdEncoding.EncodeToString(buf.Bytes()),
			"snap-len": ca.snapLen,
		},
	}, nil
}

func captureText(ctx context.Context, conn *packet.Conn, ca captureArgs) (*plugin.Response, error) {
	rb := make([]byte, maxCaptureSnapLen)
	lines := make([]string, 0, min(ca.count, 256))
	captured := 0

	for captured < ca.count {
		if ctx.Err() != nil {
			break
		}
		deadline, ok := ctx.Deadline()
		if ok {
			if setErr := conn.SetReadDeadline(deadline); setErr != nil {
				break
			}
		}

		n, _, readErr := conn.ReadFrom(rb)
		if readErr != nil {
			break
		}
		if n == 0 {
			continue
		}

		ts := time.Now()
		data := rb[:n]
		if n > ca.snapLen {
			data = rb[:ca.snapLen]
		}
		line := formatPacketLine(ts, data)
		lines = append(lines, line)
		captured++
	}

	return &plugin.Response{
		Status: plugin.StatusDone,
		Data: plugin.Map{
			"format":  captureFormatText,
			"packets": captured,
			"lines":   lines,
		},
	}, nil
}

func compileBPF(expr string) ([]bpf.RawInstruction, error) {
	parsed := filter.NewExpression(expr)
	f := parsed.Compile()
	insns, err := f.Compile()
	if err != nil {
		return nil, err
	}
	return bpf.Assemble(insns)
}
