// Design: docs/architecture/testing/ci-format.md — test runner CLI

package cli

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"net/netip"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/ze-software/ze/internal/core/env"
	"github.com/ze-software/ze/internal/test/peer"
)

var errModeInjectRequiresInjectPrefixInject = errors.New("--mode inject requires --inject-prefix, --inject-count (>0), --inject-nexthop, --inject-asn (>0)")

var _ = env.MustRegister(env.EnvEntry{
	Key:         "ze.test.bgp.port",
	Type:        "int",
	Default:     "179",
	Description: "BGP TCP port used by ze-test peer and the ze-test runner (test infrastructure)",
	Private:     true,
})

func cmdPeer(args []string) int {
	config, ok := zeTestParsePeerFlags(args)
	if config == nil {
		if ok {
			return 0
		}
		return 1
	}

	switch config.Mode {
	case peer.ModeSink:
		fmt.Fprintf(os.Stdout, "\nsink mode - send us whatever, we can take it! :p\n\n") //nolint:errcheck // output
	case peer.ModeEcho:
		fmt.Fprintf(os.Stdout, "\necho mode - send us whatever, we can parrot it! :p\n\n") //nolint:errcheck // output
	case peer.ModeCheck:
		if len(config.Expect) == 0 {
			fmt.Fprintln(os.Stderr, "no test data available to test against")
			return 1
		}
	case peer.ModeInject:
		fmt.Fprintf(os.Stdout, "\ninject mode - %d prefixes from %s via %s (AS %d)\n\n", //nolint:errcheck // output
			config.Inject.Count, config.Inject.Prefix, config.Inject.NextHop, config.Inject.ASN)
	}

	ctx, cancel := context.WithCancel(context.Background())

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	defer signal.Stop(sigCh)
	go func() {
		<-sigCh
		cancel()
	}()

	p, err := peer.New(config)
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to create peer: %v\n", err)
		return 1
	}
	result := p.Run(ctx)
	cancel()

	fmt.Fprintln(os.Stdout) //nolint:errcheck // terminal output

	if result.Error != nil {
		fmt.Fprintf(os.Stderr, "failed: %v\n", result.Error)
		return 1
	}

	if !result.Success {
		fmt.Fprintln(os.Stderr, "failed")
		return 1
	}

	fmt.Fprintln(os.Stdout, "successful") //nolint:errcheck // terminal output
	return 0
}

func zeTestParsePeerFlags(args []string) (*peer.Config, bool) {
	if len(args) > 0 && isHelpArg(args[0]) {
		zeTestPrintPeerUsage()
		return nil, true
	}

	port := env.GetInt("ze.test.bgp.port", 179)

	config := &peer.Config{Output: os.Stdout}

	var view bool
	var mode string
	var ttl uint
	var injectPrefix, injectNextHop string
	var injectCount int
	var injectASN uint
	var injectDwell time.Duration

	fs := flag.NewFlagSet("peer", flag.ExitOnError)
	fs.IntVar(&config.Port, "port", port, "port to bind to")
	fs.StringVar(&config.BindAddr, "bind", "", "bind address (default 127.0.0.1, or ::1 with -ipv6)")
	fs.StringVar(&config.Dial, "dial", "", "dial host:port instead of listening (active BGP role)")
	fs.IntVar(&config.ASN, "asn", 0, "ASN to use (0 = extract from peer OPEN)")
	fs.StringVar(&mode, "mode", "check", "operation mode: check, sink, echo, inject")
	fs.BoolVar(&config.IPv6, "ipv6", false, "bind using IPv6")
	fs.UintVar(&ttl, "ttl", 0, "outgoing TTL / hop limit on the listen socket (255 = RFC 5082 GTSM peer)")
	fs.BoolVar(&config.Decode, "decode", false, "decode messages to human-readable format")
	fs.BoolVar(&view, "view", false, "show expected packets and exit")
	fs.StringVar(&injectPrefix, "inject-prefix", "", "inject: base prefix (e.g. 10.0.0.0/24 or 2001:db8::/48)")
	fs.IntVar(&injectCount, "inject-count", 0, "inject: number of sequential prefixes to emit")
	fs.StringVar(&injectNextHop, "inject-nexthop", "", "inject: next-hop address (family must match --inject-prefix)")
	fs.UintVar(&injectASN, "inject-asn", 0, "inject: origin ASN for single-segment AS_SEQUENCE")
	fs.DurationVar(&injectDwell, "inject-dwell", 0, "inject: hold session this long after last byte (0 = until SIGTERM)")

	fs.Usage = zeTestPrintPeerUsage

	if err := fs.Parse(args); err != nil {
		return nil, false
	}

	// Reject out of range rather than truncating: --ttl 256 silently becoming 0
	// would leave a test that asked to be a GTSM peer running without the option
	// and failing far from its cause.
	if ttl > 255 {
		fmt.Fprintf(os.Stderr, "error: --ttl %d out of range 0..255\n", ttl)
		return nil, false
	}
	config.TTL = uint8(ttl)

	var valid bool
	config.Mode, valid = peer.ParseMode(mode)
	if !valid {
		fmt.Fprintf(os.Stderr, "warning: unknown mode %q, using %q\n", mode, config.Mode)
	}

	if config.Mode == peer.ModeInject {
		spec, err := zeTestBuildInjectSpec(injectPrefix, injectCount, injectNextHop, injectASN, injectDwell)
		if err != nil {
			fmt.Fprintf(os.Stderr, "error: %v\n", err)
			return nil, false
		}
		config.Inject = spec
	}

	if fs.NArg() > 0 {
		expect, fileConfig, err := peer.LoadExpectFile(fs.Arg(0))
		if err != nil {
			fmt.Fprintf(os.Stderr, "cannot load check file: %v\n", err)
			return nil, false
		}
		config.Expect = expect
		zeTestMergePeerFileConfig(config, fileConfig)
	}

	if view {
		fmt.Fprintln(os.Stdout, "\nrules:") //nolint:errcheck // terminal output
		for i, rule := range config.Expect {
			fmt.Fprintf(os.Stdout, "       %02d  %s\n", i+1, rule) //nolint:errcheck // output
		}
		return nil, true
	}

	return config, true
}

// zeTestMergePeerFileConfig folds the options parsed from a .ci peer block into
// the config built from the command line. Command-line values win only where a
// flag can express the same setting (BindAddr); everything else is file-only,
// so the file value is taken whenever it is set.
//
// EVERY field peer.parseOptionConfig can set must be folded here. A field left
// out is parsed, accepted without complaint, and then silently dropped: the
// .ci declares an option that does nothing, which is the worst failure shape
// because the test still runs and still passes for the wrong reason.
// option=await_eor was dropped this way, so every conn_map test that opted into
// waiting for ze's End-of-RIB before sending in fact never waited, and
// test/plugin/rfc7606-relay-one-field.ci failed intermittently at exactly the
// race the option exists to close. TestPeerFileConfigMergeIsComplete pins the
// rule mechanically; add the field there when you add it here.
func zeTestMergePeerFileConfig(config, fileConfig *peer.Config) {
	if fileConfig.IPv6 {
		config.IPv6 = true
	}
	if fileConfig.SendUnknownCapability {
		config.SendUnknownCapability = true
	}
	if fileConfig.SendDefaultRoute {
		config.SendDefaultRoute = true
	}
	if len(fileConfig.SendRoutes) > 0 {
		config.SendRoutes = append(config.SendRoutes, fileConfig.SendRoutes...)
	}
	if len(fileConfig.SendBulk) > 0 {
		config.SendBulk = append(config.SendBulk, fileConfig.SendBulk...)
	}
	if fileConfig.InspectOpenMessage {
		config.InspectOpenMessage = true
	}
	if fileConfig.SendUnknownMessage {
		config.SendUnknownMessage = true
	}
	if fileConfig.ASN != 0 {
		config.ASN = fileConfig.ASN
	}
	if fileConfig.TCPConnections > 0 {
		config.TCPConnections = fileConfig.TCPConnections
	}
	if fileConfig.Linger {
		config.Linger = true
	}
	if fileConfig.Silent {
		config.Silent = true
	}
	if len(fileConfig.CapabilityOverrides) > 0 {
		config.CapabilityOverrides = fileConfig.CapabilityOverrides
	}
	if fileConfig.RouterID != nil {
		config.RouterID = fileConfig.RouterID
	}
	if fileConfig.BindAddr != "" && config.BindAddr == "" {
		config.BindAddr = fileConfig.BindAddr
	}
	if fileConfig.ConnMap != "" {
		config.ConnMap = fileConfig.ConnMap
	}
	if fileConfig.AwaitEOR {
		config.AwaitEOR = true
	}
}

func zeTestBuildInjectSpec(prefixStr string, count int, nextHopStr string, asn uint, dwell time.Duration) (*peer.InjectSpec, error) {
	if prefixStr == "" || nextHopStr == "" || count <= 0 || asn == 0 {
		return nil, errModeInjectRequiresInjectPrefixInject
	}
	prefix, err := netip.ParsePrefix(prefixStr)
	if err != nil {
		return nil, fmt.Errorf("invalid --inject-prefix %q: %w", prefixStr, err)
	}
	nh, err := netip.ParseAddr(nextHopStr)
	if err != nil {
		return nil, fmt.Errorf("invalid --inject-nexthop %q: %w", nextHopStr, err)
	}
	if asn > (1<<32)-1 {
		return nil, fmt.Errorf("--inject-asn %d exceeds 32 bits", asn)
	}
	return &peer.InjectSpec{
		Prefix:   prefix,
		Count:    count,
		NextHop:  nh,
		ASN:      uint32(asn), //nolint:gosec // bounds checked above
		EndOfRIB: true,
		Dwell:    dwell,
	}, nil
}

func zeTestPrintPeerUsage() {
	fmt.Fprintf(os.Stderr, `Usage: ze-test peer [options] [expect-file]

BGP test peer for validating BGP implementations.

Modes:
  --mode check    Validate messages against expect-file (default)
  --mode sink     Accept any messages, reply keepalive
  --mode echo     Accept any messages, echo them back
  --mode inject   Stream a bulk UPDATE image after OPEN (stress tests)

Options:
  --port N           Port to bind (default: 179, or ze_test_bgp_port env)
  --asn N            ASN to use (0 = extract from peer OPEN)
  --ipv6             Bind using IPv6
  --decode           Decode messages to human-readable format
  --view             Show expected packets and exit

Inject options (all required when --mode inject):
  --inject-prefix P  Base prefix, e.g. 10.0.0.0/24 or 2001:db8::/48
  --inject-count N   Number of sequential prefixes to emit
  --inject-nexthop A Next-hop address (family must match --inject-prefix)
  --inject-asn N     Origin ASN (single-segment AS_SEQUENCE, 4-byte)
  --inject-dwell D   Hold the session open this long after the last byte
                     is written (default: until SIGTERM).
  --dial H:P         Act as the active BGP role: dial H:P instead of
                     listening, for a daemon that only ACCEPTS (a dynamic
                     peer group, or any passive peer). With --mode inject,
                     sends OPEN first with minimal capabilities (one MP-BGP
                     family inferred from --inject-prefix, plus 4-byte ASN).
                     In the default check mode it runs the ordinary .ci
                     expect/action script over the dialed connection.

Examples:
  ze-test peer --mode sink --port 1790
  ze-test peer --mode echo --port 1790
  ze-test peer --port 1790 test/encode/basic.msg
  ze-test peer --view test/encode/basic.msg
  ze-test peer --mode inject --port 1790 \
      --inject-prefix 10.0.0.0/24 --inject-count 1000000 \
      --inject-nexthop 172.31.0.3 --inject-asn 65100
  ze-test peer --mode inject --dial 172.31.0.2:179 \
      --inject-prefix 10.0.0.0/24 --inject-count 1000000 \
      --inject-nexthop 172.31.0.3 --inject-asn 65100 --inject-dwell 60s
`)
}
