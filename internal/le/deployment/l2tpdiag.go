// Design: ai/rules/cli.md -- every value follows a keyword
// Related: actions.go -- the two gateless diagnostic actions
// Detail: l2tpdiag_linux.go -- the Linux implementation
// Detail: l2tpdiag_other.go -- the refusal on other operating systems

package deployment

import (
	"errors"
	"fmt"
	"net/netip"
	"os"
	"strconv"

	"github.com/ze-software/ze/internal/core/textbuf"
	"github.com/ze-software/ze/internal/le/leaction"
)

const (
	l2tpPPPoXDiagnosticName  = "l2tp-pppox-diag"
	l2tpTunnelDiagnosticName = "l2tp-tunnel-diag"
)

// l2tpDiagnosticOptions contains the common tunnel selectors. The session IDs
// are used only by the PPPoX diagnostic.
type l2tpDiagnosticOptions struct {
	Local           [4]byte
	Remote          [4]byte
	SourcePort      uint16
	DestinationPort uint16
	TunnelID        uint32
	PeerTunnelID    uint32
	SessionID       uint16
	PeerSessionID   uint16
}

func defaultPPPoXDiagnosticOptions() l2tpDiagnosticOptions {
	return l2tpDiagnosticOptions{
		Local:           [4]byte{0, 0, 0, 0},
		Remote:          [4]byte{127, 0, 0, 1},
		SourcePort:      1701,
		DestinationPort: 1701,
		TunnelID:        1,
		PeerTunnelID:    100,
		SessionID:       1,
		PeerSessionID:   100,
	}
}

func defaultTunnelDiagnosticOptions() l2tpDiagnosticOptions {
	return l2tpDiagnosticOptions{
		Local:           [4]byte{172, 30, 0, 1},
		Remote:          [4]byte{172, 30, 0, 2},
		SourcePort:      1701,
		DestinationPort: 1702,
		TunnelID:        1,
		PeerTunnelID:    100,
	}
}

func pppoxDiagnosticParameters() []leaction.Parameter {
	return []leaction.Parameter{
		{Keyword: "local", Value: "addr"},
		{Keyword: "remote", Value: "addr"},
		{Keyword: "source-port", Value: "port"},
		{Keyword: "destination-port", Value: "port"},
		{Keyword: "tunnel-id", Value: "id"},
		{Keyword: "peer-tunnel-id", Value: "id"},
		{Keyword: "session-id", Value: "id"},
		{Keyword: "peer-session-id", Value: "id"},
	}
}

func tunnelDiagnosticParameters() []leaction.Parameter {
	return pppoxDiagnosticParameters()[:6]
}

func parsePPPoXDiagnosticArguments(args leaction.Arguments) (l2tpDiagnosticOptions, error) {
	options := defaultPPPoXDiagnosticOptions()
	if err := parseCommonDiagnosticArguments(args, &options); err != nil {
		return l2tpDiagnosticOptions{}, err
	}
	var err error
	var tunnelID uint16
	if tunnelID, err = uint16Argument(args, "tunnel-id", uint16(options.TunnelID)); err != nil {
		return l2tpDiagnosticOptions{}, err
	}
	options.TunnelID = uint32(tunnelID)
	var peerTunnelID uint16
	if peerTunnelID, err = uint16Argument(args, "peer-tunnel-id", uint16(options.PeerTunnelID)); err != nil {
		return l2tpDiagnosticOptions{}, err
	}
	options.PeerTunnelID = uint32(peerTunnelID)
	if options.SessionID, err = uint16Argument(args, "session-id", options.SessionID); err != nil {
		return l2tpDiagnosticOptions{}, err
	}
	if options.PeerSessionID, err = uint16Argument(args, "peer-session-id", options.PeerSessionID); err != nil {
		return l2tpDiagnosticOptions{}, err
	}
	return options, nil
}

func parseTunnelDiagnosticArguments(args leaction.Arguments) (l2tpDiagnosticOptions, error) {
	options := defaultTunnelDiagnosticOptions()
	if err := parseCommonDiagnosticArguments(args, &options); err != nil {
		return l2tpDiagnosticOptions{}, err
	}
	return options, nil
}

func parseCommonDiagnosticArguments(args leaction.Arguments, options *l2tpDiagnosticOptions) error {
	var err error
	if options.Local, err = ipv4Argument(args, "local", options.Local); err != nil {
		return err
	}
	if options.Remote, err = ipv4Argument(args, "remote", options.Remote); err != nil {
		return err
	}
	if options.SourcePort, err = uint16Argument(args, "source-port", options.SourcePort); err != nil {
		return err
	}
	if options.DestinationPort, err = uint16Argument(args, "destination-port", options.DestinationPort); err != nil {
		return err
	}
	if options.TunnelID, err = uint32Argument(args, "tunnel-id", options.TunnelID); err != nil {
		return err
	}
	if options.PeerTunnelID, err = uint32Argument(args, "peer-tunnel-id", options.PeerTunnelID); err != nil {
		return err
	}
	return nil
}

func ipv4Argument(args leaction.Arguments, keyword string, fallback [4]byte) ([4]byte, error) {
	raw, present := args[keyword]
	if !present {
		return fallback, nil
	}
	parsed, err := netip.ParseAddr(raw)
	if err != nil || !parsed.Is4() || parsed.String() != raw {
		var tb textbuf.Buffer
		return [4]byte{}, errors.New(tb.Str(keyword).Byte(' ').Quoted(raw).
			Str(" is not a dotted-quad IPv4 address").String())
	}
	return parsed.As4(), nil
}

func uint16Argument(args leaction.Arguments, keyword string, fallback uint16) (uint16, error) {
	value, err := unsignedArgument(args, keyword, uint64(fallback), 16)
	return uint16(value), err
}

func uint32Argument(args leaction.Arguments, keyword string, fallback uint32) (uint32, error) {
	value, err := unsignedArgument(args, keyword, uint64(fallback), 32)
	return uint32(value), err
}

func unsignedArgument(args leaction.Arguments, keyword string, fallback uint64, bits int) (uint64, error) {
	raw, present := args[keyword]
	if !present {
		return fallback, nil
	}
	value, err := strconv.ParseUint(raw, 10, bits)
	if err != nil {
		var tb textbuf.Buffer
		return 0, errors.New(tb.Str(keyword).Byte(' ').Quoted(raw).Str(" is outside the unsigned ").
			Int(int64(bits)).Str("-bit range").String())
	}
	return value, nil
}

// diagnosticRunError carries the producer's output channel. Reported means the
// transcript already contains the complete failure line.
type diagnosticRunError struct {
	err      error
	prefix   string
	reported bool
}

func (e *diagnosticRunError) Error() string { return e.err.Error() }
func (e *diagnosticRunError) Unwrap() error { return e.err }

//nolint:unused // the callers are in l2tpdiag_linux.go, which this platform does not build
func fatalDiagnosticError(prefix string, err error) error {
	return &diagnosticRunError{err: err, prefix: prefix}
}

//nolint:unused // the caller is in l2tpdiag_linux.go, which this platform does not build
func reportedDiagnosticError(err error) error {
	return &diagnosticRunError{err: err, reported: true}
}

func reportL2TPDiagnosticError(err error) {
	if err == nil {
		return
	}
	var runError *diagnosticRunError
	if !errors.As(err, &runError) {
		leaction.ReportError(err)
		return
	}
	if runError.reported {
		return
	}
	var tb textbuf.Buffer
	fmt.Fprintln(os.Stderr, tb.Str(runError.prefix).Err(runError.err).String()) //nolint:errcheck // CLI output
}
