// Design: plan/learned/730-diag-capture-interface.md -- AF_PACKET live capture

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

	"github.com/ze-software/ze/internal/component/iface"
	"github.com/ze-software/ze/internal/component/plugin"
	pluginserver "github.com/ze-software/ze/internal/component/plugin/server"
	"github.com/ze-software/ze/internal/core/textbuf"
)

const maxPcapBufSize = 64 << 20

func HandleCaptureInterface(_ *pluginserver.CommandContext, args []string) (*plugin.Response, error) {
	ca, err := parseCaptureArgs(args)
	if err != nil {
		return &plugin.Response{Status: plugin.StatusError, Error: err.Error()}, nil //nolint:nilerr // operational error in Response
	}

	var tb textbuf.Buffer

	// Resolve the logical interface name to its kernel device via the shared
	// iface resolver (honoring os-name / mac-match), then fetch the *net.Interface
	// the AF_PACKET capture socket needs.
	binding, rerr := iface.Resolve(ca.iface)
	if rerr != nil {
		return &plugin.Response{Status: plugin.StatusError, Error: tb.Str("interface not found: ").Str(ca.iface).String()}, nil
	}
	ifc, lookupErr := net.InterfaceByName(binding.OsName)
	if lookupErr != nil {
		return &plugin.Response{Status: plugin.StatusError, Error: tb.Reset().Str("interface not found: ").Str(ca.iface).String()}, nil
	}

	if _, loaded := activeCaptures.LoadOrStore(ca.iface, true); loaded {
		return &plugin.Response{Status: plugin.StatusError, Error: tb.Reset().Str("capture already active on ").Str(ca.iface).String()}, nil
	}
	defer activeCaptures.Delete(ca.iface)

	var rawInsns []bpf.RawInstruction
	if ca.filter != "" {
		rawInsns, err = compileBPF(ca.filter)
		if err != nil {
			return &plugin.Response{Status: plugin.StatusError, Error: tb.Reset().Str("filter compilation failed: ").Err(err).String()}, nil
		}
	}

	conn, err := packet.Listen(ifc, packet.Raw, 0, nil)
	if err != nil {
		return &plugin.Response{Status: plugin.StatusError, Error: tb.Reset().Str("capture: ").Err(err).Str(" (requires CAP_NET_RAW)").String()}, nil
	}
	defer func() { _ = conn.Close() }()

	if len(rawInsns) > 0 {
		if setErr := conn.SetBPF(rawInsns); setErr != nil {
			return &plugin.Response{Status: plugin.StatusError, Error: tb.Reset().Str("BPF attach failed: ").Err(setErr).String()}, nil
		}
	}

	ctx, cancel := context.WithTimeout(context.Background(), ca.dur)
	defer cancel()

	switch ca.format {
	case captureFormatText:
		return captureText(ctx, conn, ca), nil
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

	return &plugin.Response{ //nolint:nilerr // partial capture returned on ctx deadline/cancel; not a Go error
		Status: plugin.StatusDone,
		Data: plugin.Map{
			"format":   captureFormatPcap,
			"packets":  captured,
			"pcap":     base64.StdEncoding.EncodeToString(buf.Bytes()),
			"snap-len": ca.snapLen,
		},
	}, nil
}

func captureText(ctx context.Context, conn *packet.Conn, ca captureArgs) *plugin.Response {
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
	}
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
