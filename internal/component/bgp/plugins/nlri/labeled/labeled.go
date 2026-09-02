// Design: docs/architecture/wire/nlri.md — labeled unicast NLRI plugin
// RFC: rfc/short/rfc8277.md
//
// Package bgp_labeled implements a Labeled Unicast family plugin for ze.
// It handles Labeled Unicast NLRI (RFC 8277, SAFI 4).
package labeled

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net"
	"strings"

	"github.com/ze-software/ze/internal/core/slogutil"
	"github.com/ze-software/ze/internal/core/textbuf"
	sdk "github.com/ze-software/ze/pkg/plugin/sdk"
)

// Address family names this plugin registers and decodes. The plugin registry,
// the CLI and the reactor all match a family by exact string.
const (
	familyIPv4Labeled = "ipv4/mpls-label" // AFI 1, SAFI 4
	familyIPv6Labeled = "ipv6/mpls-label" // AFI 2, SAFI 4

	// familyModeDecode declares a family this plugin decodes but never encodes.
	// The plugin server reads it as sdk.FamilyDecl.Mode.
	familyModeDecode = "decode"

	// cmdDecode is the text-protocol verb this plugin answers on stdin.
	cmdDecode = "decode"
)

var logger = slogutil.DiscardLogger()

// SetLogger sets the package-level logger.
func SetLogger(l *slog.Logger) {
	if l != nil {
		logger = l
	}
}

// runLabeledPlugin runs the labeled unicast plugin using the SDK RPC protocol.
func runLabeledPlugin(conn net.Conn) int {
	logger.Debug("labeled plugin starting (RPC)")

	p := sdk.NewWithConn("bgp-nlri-labeled", conn)
	defer func() { _ = p.Close() }()

	ctx, cancel := sdk.SignalContext()
	defer cancel()
	err := p.Run(ctx, sdk.Registration{
		Families: []sdk.FamilyDecl{
			{Name: familyIPv4Labeled, Mode: familyModeDecode, AFI: 1, SAFI: 4},
			{Name: familyIPv6Labeled, Mode: familyModeDecode, AFI: 2, SAFI: 4},
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
		_, e := fmt.Fprintf(errOut, format, args...) //nolint:errcheck // output
		_ = e
	}

	data, err := DecodeNLRIHex(family, hexData)
	if err != nil {
		writeErr("error: %v\n", err)
		return 1
	}

	raw, err := json.Marshal(data)
	if err != nil {
		writeErr("error: %v\n", err)
		return 1
	}

	if textOutput {
		text := formatLabeledText(string(raw))
		if _, e := fmt.Fprintln(output, text); e != nil { //nolint:errcheck // output
			return 1
		}
		return 0
	}

	if _, e := fmt.Fprintln(output, string(raw)); e != nil { //nolint:errcheck // output
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
	var sb textbuf.Buffer
	sb.Str(result.Prefix)
	for i, l := range result.Labels {
		if i == 0 {
			fmt.Fprintf(&sb, " label=%d", l) //nolint:errcheck // buffer output
		} else {
			fmt.Fprintf(&sb, ",%d", l) //nolint:errcheck // buffer output
		}
	}
	return sb.String()
}

// RunDecode runs the labeled unicast plugin in decode mode for ze bgp decode.
// It reads "decode nlri <family> <hex>" requests from input and writes
// "decoded json <json>" or "decoded unknown" responses to output.
func RunDecode(input io.Reader, output io.Writer) int {
	write := func(s string) {
		if _, err := fmt.Fprintln(output, s); err != nil { //nolint:errcheck // output
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
		if len(parts) >= 4 && parts[0] == cmdDecode && parts[1] == "nlri" {
			fam := parts[2]
			hexData := parts[3]

			data, err := DecodeNLRIHex(fam, hexData)
			if err == nil {
				if raw, merr := json.Marshal(data); merr == nil {
					write("decoded json " + string(raw))
					continue
				}
			}
		}

		write("decoded unknown")
	}
	// bufio.Scanner reports a read failure and an over-long line through Err(),
	// never through Scan(). Without this the caller sees a clean, complete decode.
	if err := scanner.Err(); err != nil {
		var eb textbuf.Buffer
		write(eb.Str("decoded error ").Err(err).String())
		return 1
	}
	return 0
}
