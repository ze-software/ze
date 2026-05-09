// Design: plan/spec-diag-0-umbrella.md -- raw capture activation and pcap export

package show

import (
	"bytes"
	"encoding/base64"
	"time"

	"codeberg.org/thomas-mangin/ze/internal/component/l2tp"
	"codeberg.org/thomas-mangin/ze/internal/component/plugin"
	pluginserver "codeberg.org/thomas-mangin/ze/internal/component/plugin/server"
)

const fmtPcap = "pcap"

func init() {
	pluginserver.RegisterRPCs(
		pluginserver.RPCRegistration{WireMethod: "ze-show:capture-raw", Handler: handleCaptureRaw},
	)
}

func handleCaptureRaw(ctx *pluginserver.CommandContext, args []string) (*plugin.Response, error) {
	action := ""
	protocol := ""
	format := "json"
	limit := 0
	for _, a := range args {
		switch a {
		case "start", "stop", "dump":
			action = a
		case capL2TP, capBGP:
			protocol = a
		case fmtPcap, "json":
			format = a
		case argCount:
			limit = extractCountFilter(args)
		}
	}

	switch action {
	case "start":
		return captureRawStart(ctx, protocol)
	case "stop":
		return captureRawStop(ctx, protocol)
	case "dump":
		return captureRawDump(ctx, protocol, format, limit)
	default:
		return &plugin.Response{
			Status: plugin.StatusError,
			Data:   "usage: capture-raw [start|stop|dump] [l2tp|bgp] [pcap|json] [count N]",
		}, nil
	}
}

func captureRawStart(ctx *pluginserver.CommandContext, protocol string) (*plugin.Response, error) {
	started := []string{}
	if protocol == "" || protocol == capL2TP {
		svc := l2tp.LookupService()
		if svc != nil {
			svc.EnableRawCapture()
			started = append(started, capL2TP)
		}
	}
	if protocol == "" || protocol == capBGP {
		if ctx != nil && ctx.Reactor() != nil {
			if rcp, ok := ctx.Reactor().(plugin.BGPRawCaptureProvider); ok {
				rcp.EnableRawCapture()
				started = append(started, capBGP)
			}
		}
	}
	return &plugin.Response{
		Status: plugin.StatusDone,
		Data: map[string]any{
			"action":  "start",
			"started": started,
		},
	}, nil
}

func captureRawStop(ctx *pluginserver.CommandContext, protocol string) (*plugin.Response, error) {
	stopped := []string{}
	if protocol == "" || protocol == capL2TP {
		svc := l2tp.LookupService()
		if svc != nil {
			svc.DisableRawCapture()
			stopped = append(stopped, capL2TP)
		}
	}
	if protocol == "" || protocol == capBGP {
		if ctx != nil && ctx.Reactor() != nil {
			if rcp, ok := ctx.Reactor().(plugin.BGPRawCaptureProvider); ok {
				rcp.DisableRawCapture()
				stopped = append(stopped, capBGP)
			}
		}
	}
	return &plugin.Response{
		Status: plugin.StatusDone,
		Data: map[string]any{
			"action":  "stop",
			"stopped": stopped,
		},
	}, nil
}

func captureRawDump(ctx *pluginserver.CommandContext, protocol, format string, limit int) (*plugin.Response, error) {
	result := map[string]any{}

	if protocol == "" || protocol == capL2TP {
		svc := l2tp.LookupService()
		if svc == nil {
			result["l2tp"] = msgSubsystemNotRunning
		} else {
			entries := svc.RawCaptureSnapshot(limit)
			switch {
			case entries == nil:
				result["l2tp"] = "raw capture not enabled (use capture-raw start l2tp)"
			case format == fmtPcap:
				pcapData, err := exportL2TPPcap(entries)
				if err != nil {
					result["l2tp-error"] = err.Error()
				} else {
					result["l2tp-pcap"] = base64.StdEncoding.EncodeToString(pcapData)
					result["l2tp-packets"] = len(entries)
				}
			default:
				result["l2tp"] = rawEntriesToJSON(entries)
				result["l2tp-count"] = len(entries)
			}
		}
	}

	if protocol == "" || protocol == capBGP {
		if ctx != nil && ctx.Reactor() != nil {
			if rcp, ok := ctx.Reactor().(plugin.BGPRawCaptureProvider); ok {
				entries := rcp.BGPRawCaptureSnapshot(limit)
				switch {
				case entries == nil:
					result["bgp"] = "raw capture not enabled (use capture-raw start bgp)"
				case format == fmtPcap:
					pcapData, err := exportBGPPcap(entries)
					if err != nil {
						result["bgp-error"] = err.Error()
					} else {
						result["bgp-pcap"] = base64.StdEncoding.EncodeToString(pcapData)
						result["bgp-packets"] = len(entries)
					}
				default:
					rows := make([]map[string]any, 0, len(entries))
					for _, e := range entries {
						rows = append(rows, map[string]any{
							"timestamp": e.Timestamp,
							"direction": e.Direction,
							"bytes":     len(e.Data),
						})
					}
					result["bgp"] = rows
					result["bgp-count"] = len(rows)
				}
			}
		} else if protocol == capBGP {
			result["bgp"] = "reactor not available"
		}
	}

	return &plugin.Response{Status: plugin.StatusDone, Data: result}, nil
}

func rawEntriesToJSON(entries []l2tp.RawCaptureEntry) []map[string]any {
	rows := make([]map[string]any, 0, len(entries))
	for _, e := range entries {
		dir := "in"
		if e.Direction == 1 {
			dir = "out"
		}
		rows = append(rows, map[string]any{
			"timestamp": e.Timestamp.UTC().Format("2006-01-02T15:04:05Z07:00"),
			"direction": dir,
			"bytes":     len(e.Data),
		})
	}
	return rows
}

func exportBGPPcap(entries []plugin.BGPRawCaptureEntry) ([]byte, error) {
	var buf bytes.Buffer
	if err := writePcapHeader(&buf, 4096, LinkTypeRaw); err != nil {
		return nil, err
	}
	for i := len(entries) - 1; i >= 0; i-- {
		e := &entries[i]
		ts, _ := time.Parse("2006-01-02T15:04:05Z07:00", e.Timestamp)
		if err := writePcapPacket(&buf, ts, e.Data); err != nil {
			return nil, err
		}
	}
	return buf.Bytes(), nil
}

func exportL2TPPcap(entries []l2tp.RawCaptureEntry) ([]byte, error) {
	var buf bytes.Buffer
	if err := writePcapHeader(&buf, 1500, LinkTypeRaw); err != nil {
		return nil, err
	}
	for i := len(entries) - 1; i >= 0; i-- {
		e := &entries[i]
		if err := writePcapPacket(&buf, e.Timestamp, e.Data); err != nil {
			return nil, err
		}
	}
	return buf.Bytes(), nil
}
