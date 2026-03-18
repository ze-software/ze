// Design: docs/architecture/wire/nlri.md — labeled unicast NLRI plugin
// RFC: rfc/short/rfc8277.md
//
// Package bgp_labeled implements a Labeled Unicast family plugin for ze.
// It handles Labeled Unicast NLRI (RFC 8277, SAFI 4).
package labeled

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net"
	"strings"

	"codeberg.org/thomas-mangin/ze/internal/core/slogutil"
	sdk "codeberg.org/thomas-mangin/ze/pkg/plugin/sdk"
)

var logger = slogutil.DiscardLogger()

// SetLogger sets the package-level logger.
func SetLogger(l *slog.Logger) {
	if l != nil {
		logger = l
	}
}

// RunLabeledPlugin runs the labeled unicast plugin using the SDK RPC protocol.
func RunLabeledPlugin(conn net.Conn) int {
	logger.Debug("labeled plugin starting (RPC)")

	p := sdk.NewWithConn("bgp-labeled", conn)
	defer func() { _ = p.Close() }()

	ctx := context.Background()
	err := p.Run(ctx, sdk.Registration{
		Families: []sdk.FamilyDecl{
			{Name: "ipv4/mpls-label", Mode: "decode"},
			{Name: "ipv6/mpls-label", Mode: "decode"},
		},
	})
	if err != nil {
		logger.Error("labeled plugin failed", "error", err)
		return 1
	}

	return 0
}

// RunCLIDecode decodes labeled unicast NLRI from hex for CLI usage (ze plugin bgp-labeled --nlri).
func RunCLIDecode(hexData, family string, textOutput bool, output, errOut io.Writer) int {
	writeErr := func(format string, args ...any) {
		_, e := fmt.Fprintf(errOut, format, args...)
		_ = e
	}

	jsonStr, err := DecodeNLRIHex(family, hexData)
	if err != nil {
		writeErr("error: %v\n", err)
		return 1
	}

	if textOutput {
		text := formatLabeledText(jsonStr)
		if _, e := fmt.Fprintln(output, text); e != nil {
			return 1
		}
		return 0
	}

	if _, e := fmt.Fprintln(output, jsonStr); e != nil {
		return 1
	}
	return 0
}

// formatLabeledText converts JSON output to human-readable text.
// Input: {"prefix":"10.0.0.0/8","labels":[100]}
// Output: 10.0.0.0/8 label=100.
func formatLabeledText(jsonStr string) string {
	var result struct {
		Prefix string   `json:"prefix"`
		Labels []uint32 `json:"labels"`
	}
	if err := json.Unmarshal([]byte(jsonStr), &result); err != nil {
		return jsonStr
	}
	var sb strings.Builder
	sb.WriteString(result.Prefix)
	for i, l := range result.Labels {
		if i == 0 {
			fmt.Fprintf(&sb, " label=%d", l)
		} else {
			fmt.Fprintf(&sb, ",%d", l)
		}
	}
	return sb.String()
}

// RunDecode runs the labeled unicast plugin in decode mode for ze bgp decode.
// It reads "decode nlri <family> <hex>" requests from input and writes
// "decoded json <json>" or "decoded unknown" responses to output.
func RunDecode(input io.Reader, output io.Writer) int {
	write := func(s string) {
		if _, err := fmt.Fprintln(output, s); err != nil {
			logger.Debug("write error", "err", err)
		}
	}

	scanner := bufio.NewScanner(input)
	for scanner.Scan() {
		line := scanner.Text()
		if line == "" {
			continue
		}

		parts := strings.Fields(line)
		if len(parts) >= 4 && parts[0] == "decode" && parts[1] == "nlri" {
			family := parts[2]
			hexData := parts[3]

			jsonStr, err := DecodeNLRIHex(family, hexData)
			if err == nil {
				write("decoded json " + jsonStr)
				continue
			}
		}

		write("decoded unknown")
	}
	return 0
}
