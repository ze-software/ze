// Design: docs/architecture/testing/ci-format.md — .ci file loading and option parsing
// Overview: peer.go — Config struct populated by these parsers
// Related: checker.go — expect rules parsed here feed the Checker
// RFC: rfc/short/rfc4271.md — the Hold Time a .ci proposes here
// RFC: rfc/short/rfc4724.md — the Graceful Restart Restart Time and its 12 bits
// RFC: rfc/short/rfc9494.md — the Long-Lived Stale Time and its 24 bits
// RFC: rfc/short/rfc5492.md — the one-octet Capability Code and Capability Length
// RFC: rfc/short/draft-abraitis-idr-addpath-paths-limit.md — the Max Paths field

package peer

import (
	"bufio"
	"encoding/binary"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"net/netip"
	"slices"
	"strconv"
	"strings"

	"github.com/ze-software/ze/internal/core/cliio"
	"github.com/ze-software/ze/internal/core/family"
	"github.com/ze-software/ze/internal/test/ci"
)

// optTrue is the spelling a .ci boolean option must use to enable itself.
const optTrue = "true"

// The option=open: values that declare a fact ze-peer's OPEN asserts about
// ze-peer. Each one names the capability it resolves, and open.go names the same
// constant where it refuses the option beside raw octets for that code.
const (
	optOpenHoldTime        = "hold-time"
	optOpenGracefulRestart = "graceful-restart"
	optOpenLLGR            = "llgr"
	optOpenPathsLimit      = "paths-limit"
)

// The four .ci actions a ze-peer stdin block can carry, left of the '=' in
// `action=type:key=value`.
const (
	actionOption = "option"
	actionExpect = "expect"
	actionAction = "action"
	actionReject = "reject"
	actionCmd    = "cmd"
)

// Consumes reports whether ze-peer consumes the directive `action=lineType`,
// i.e. whether LoadExpectFile forwards such a line into Config.Expect.
//
// This is the single definition of the ze-peer-consumed directive set. It is
// deliberately exported through ConsumesLine so the test runner can reject, at
// parse time, a check-mode peer block that declares nothing ze-peer can act on.
// Such a block leaves Config.Expect empty, which makes ze-peer print
// "no test data available to test against" and exit 1 BEFORE it ever binds a
// listening socket (internal/test/cli/cmd_peer.go). Every ze dial then gets
// connection refused and the test can only ever pass vacuously.
//
// Keep this function and LoadExpectFile's switch in lockstep: they are the same
// decision, and a divergence reintroduces the silent-vacuous-test defect.
func consumes(action, lineType string) bool {
	switch action {
	case actionExpect:
		// json/stderr/syslog are handled by the test runner, not ze-peer.
		return lineType == "bgp"
	case actionReject:
		// stderr/syslog/stdout are handled by the test runner, not ze-peer.
		// Only ze-peer sees the wire, so only ze-peer can refuse wire bytes.
		return lineType == "bgp"
	case actionAction:
		switch lineType {
		case "notification", "send", "rewrite", actionClose, actionSighup, actionSigterm:
			return true
		}
	}
	return false
}

// ConsumesLine reports whether ze-peer consumes the given .ci line. Blank lines,
// comments, and runner-only directives (expect=json, option=env, cmd=,
// reject=stderr) return false. See consumes for why this is exported.
func ConsumesLine(line string) bool {
	line = strings.TrimSpace(line)
	if line == "" || strings.HasPrefix(line, "#") {
		return false
	}
	head, _, _ := strings.Cut(line, ":")
	action, lineType, ok := strings.Cut(head, "=")
	if !ok {
		return false
	}
	return consumes(action, lineType)
}

// Claim says who acts on a .ci line that sits inside a ze-peer stdin block.
//
// A stdin block is handed to ze-peer verbatim, so ze-peer's parser is the first
// reader of every line in it. Its answer decides what the test runner owes the
// line, and the runner's peer-block guard reads that answer rather than a second
// list of directive names (internal/test/runner/peer_contract.go).
type Claim int

const (
	// ClaimNone: ze-peer has no branch for the line's action. Nothing in the
	// peer reads it, so the runner must be able to parse it where it stands.
	ClaimNone Claim = iota
	// ClaimPeer: ze-peer acts on the line itself.
	ClaimPeer
	// ClaimRunner: ze-peer reads the line and hands it to the test runner, so
	// the runner must parse it where it stands or the line changes nothing.
	ClaimRunner
	// ClaimNarration: ze-peer records the line as documentation of what
	// produced the expected bytes. Nothing acts on it and nothing is lost.
	ClaimNarration
)

// ClaimLine reports what ze-peer does with one line of a stdin block, and the
// error its own parser raises for that line.
//
// Every branch below is the branch LoadExpectFile takes for the same line, so
// the runner's guard cannot drift from what ze-peer really does. A blank line or
// a comment claims nothing and raises nothing.
func ClaimLine(line string) (Claim, error) {
	line = strings.TrimSpace(line)
	if line == "" || strings.HasPrefix(line, "#") {
		return ClaimNarration, nil
	}
	parts := strings.Split(line, ":")
	action, lineType, ok := strings.Cut(parts[0], "=")
	if !ok {
		return ClaimNone, nil
	}
	switch action {
	case actionOption:
		kv, err := ci.ParseKVPairs(parts[1:])
		if err != nil {
			return ClaimNone, fmt.Errorf("%s: %w", parts[0], err)
		}
		claimed, err := parseOptionConfig(&Config{}, lineType, kv)
		if err != nil {
			return ClaimNone, err
		}
		if claimed {
			return ClaimPeer, nil
		}
		return ClaimRunner, nil
	case actionReject:
		if !consumes(action, lineType) {
			return ClaimRunner, nil
		}
		// Validated HERE, with the parser ze-peer itself uses, so a malformed
		// needle fails the file at parse time. Left to run time it would fail
		// inside peer.New, before the listener binds, and the runner would
		// report a bind timeout instead of the typo (peerBindFailure).
		//
		// isReject false means the line said `reject=bgp` and stopped, with no
		// key=value tail at all. consumes() has already answered true for it, so
		// LoadExpectFile would forward it and parseExpectRule would fail on it
		// inside the peer -- the same bind timeout by a longer route.
		_, _, isReject, err := ParseRejectRule(line)
		if err != nil {
			return ClaimNone, err
		}
		if !isReject {
			return ClaimNone, fmt.Errorf(
				"%q carries no conn= or pattern=; write reject=bgp:conn=N:pattern=<hex>", line)
		}
		return ClaimPeer, nil
	case actionExpect, actionAction:
		if consumes(action, lineType) {
			return ClaimPeer, nil
		}
		return ClaimRunner, nil
	case actionCmd:
		return ClaimNarration, nil
	}
	return ClaimNone, nil
}

// LoadExpectFile loads expected messages from a file.
// Uses format: action:type:key=value:key=value:...
func LoadExpectFile(path string) ([]string, *Config, error) {
	f, err := cliio.OpenReader(path) // "-" reads stdin
	if err != nil {
		return nil, nil, err
	}
	defer func() { _ = f.Close() }()
	return ParseExpect(f)
}

// ParseExpect parses an expectation block from r, in the format LoadExpectFile
// reads. It is exported so the functional runner reads a peer's options (for
// example option=linger) with the parser the peer itself uses, rather than with
// a second copy that could disagree with it.
func ParseExpect(r io.Reader) ([]string, *Config, error) {
	config := &Config{}
	var expect []string

	lineNum := 0
	scanner := bufio.NewScanner(r)
	for scanner.Scan() {
		lineNum++
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		// Parse format: action=type:key=value:...
		// First segment is action=type, remaining segments are key=value pairs
		parts := strings.Split(line, ":")
		if len(parts) < 1 {
			return nil, nil, fmt.Errorf("line %d: invalid format %q", lineNum, line)
		}

		// First segment is action=type
		actionType := strings.SplitN(parts[0], "=", 2)
		if len(actionType) != 2 {
			return nil, nil, fmt.Errorf("line %d: invalid format %q, expected action=type:key=value", lineNum, line)
		}
		action := actionType[0]
		lineType := actionType[1]
		switch action {
		case actionOption:
			// Only option= is split here: the other lines are forwarded whole
			// or read by the runner, whose parsers own their key=value tails.
			kv, err := ci.ParseKVPairs(parts[1:])
			if err != nil {
				return nil, nil, fmt.Errorf("line %d: %s: %w", lineNum, parts[0], err)
			}
			if _, err := parseOptionConfig(config, lineType, kv); err != nil {
				return nil, nil, fmt.Errorf("line %d: %w", lineNum, err)
			}

		case actionExpect, actionAction, actionReject:
			// Pass through the ze-peer-consumed directives only:
			//   expect=bgp:conn=N:seq=N:hex=...
			//   reject=bgp:conn=N:pattern=...
			//   action=notification|send|rewrite|close|sighup|sigterm:conn=N:seq=N:...
			// expect=json/stderr/syslog and reject=stderr/syslog/stdout are
			// handled by the test runner.
			if consumes(action, lineType) {
				expect = append(expect, line)
			}

		case actionCmd:
			// Ignore - documentation only
		}
	}

	if err := scanner.Err(); err != nil {
		return nil, nil, err
	}
	// Checked after the whole block is read, because each refusal it holds is a
	// PAIR of lines and either order is legal to write.
	if err := config.validateOpenDeclarations(); err != nil {
		return nil, nil, err
	}
	return expect, config, nil
}

// parseOptionConfig parses option lines into Config.
//
// It returns an error for a directive whose misreading would make a test assert
// against an input it never sent: a non-numeric asn or tcp_connections, and an
// open= or update= value ze-peer has no arm for. Each of those used to fail
// soft, which put a typo in the same class as the dropped directive this
// parser's Claim answer exists to close: the line is written, nothing reads it,
// and the test passes.
//
// claimed reports whether ze-peer acted on the option. It is false both for an
// option the test runner owns (file, timeout, env) and for one no parser knows,
// because ze-peer treats them alike: it reads neither. ClaimLine turns that into
// the runner's obligation to parse the line where it stands, and the runner's
// own "unknown option type" error is what separates the two cases.
func parseOptionConfig(config *Config, optType string, kv map[string]string) (claimed bool, err error) {
	arm, claimed, err := peerOptionArmFor(optType, kv)
	if err != nil {
		return false, err
	}
	if !claimed {
		// No arm here, so ze-peer reads nothing. The runner decides whether
		// the option exists at all.
		return false, nil
	}
	if err := arm.parse(config, kv); err != nil {
		return false, err
	}
	return true, nil
}

// parsePeerOptionASN reads option=asn. Two unkeyed declarations in one block
// are a contradiction, and taking the last would resolve it by accident of
// order. A block that wants a different AS per session keys each line with
// peer=<ip>.
func parsePeerOptionASN(config *Config, kv map[string]string) error {
	binding, err := parseOpenASBinding(kv)
	if err != nil {
		return err
	}
	if !binding.Addr.IsValid() {
		for _, held := range config.OpenAS {
			if held.Addr.IsValid() {
				continue
			}
			return fmt.Errorf(
				"option=asn:value=%d follows option=asn:value=%d with no peer= key on either, "+
					"so the block states two ASNs for the same session; key each line with peer=<ip>",
				binding.AS, held.AS)
		}
	}
	config.OpenAS = append(config.OpenAS, binding)
	return nil
}

// parsePeerOptionBind reads option=bind: ipv6 selects the IPv6 listener, any
// other value is the bind address.
func parsePeerOptionBind(config *Config, kv map[string]string) error {
	v := kv[keyValue]
	if v == "ipv6" {
		config.IPv6 = true
	} else if v != "" {
		config.BindAddr = v
	}
	return nil
}

func parsePeerOptionConnMap(config *Config, kv map[string]string) error {
	config.ConnMap = kv[keyValue]
	return nil
}

func parsePeerOptionAwaitEOR(config *Config, kv map[string]string) error {
	config.AwaitEOR = kv[keyValue] == optTrue
	return nil
}

func parsePeerOptionLinger(config *Config, kv map[string]string) error {
	config.Linger = kv[keyValue] == optTrue
	return nil
}

func parsePeerOptionSilent(config *Config, kv map[string]string) error {
	config.Silent = kv[keyValue] == optTrue
	return nil
}

// parsePeerOptionEstablishedFile reads option=established-file. A missing path
// would make the peer write nothing, and the fixture waiting on the file would
// then fail as a timeout naming no cause.
func parsePeerOptionEstablishedFile(config *Config, kv map[string]string) error {
	if kv["path"] == "" {
		return errors.New("option=established-file needs path=<name>, the file ze-peer creates on the daemon's first UPDATE")
	}
	config.EstablishedFile = kv["path"]
	return nil
}

// parsePeerOptionAwaitFile reads option=await-file. A missing path would make
// the wait a no-op, and the race the option exists to close would come back
// with no sign of why.
func parsePeerOptionAwaitFile(config *Config, kv map[string]string) error {
	if kv["path"] == "" {
		return errors.New("option=await-file needs path=<name>, the file ze-peer waits for before its first send (another peer block's established-file)")
	}
	config.AwaitFile = kv["path"]
	return nil
}

func parsePeerOptionTCPConnections(config *Config, kv map[string]string) error {
	v, err := strconv.Atoi(kv[keyValue])
	if err != nil {
		return fmt.Errorf("option=tcp_connections:value=%q is not a number, so ze-peer would accept one connection", kv[keyValue])
	}
	config.TCPConnections = v
	return nil
}

func parseOpenSendUnknownCapability(config *Config, _ map[string]string) error {
	config.SendUnknownCapability = true
	return nil
}

func parseOpenInspectMessage(config *Config, _ map[string]string) error {
	config.InspectOpenMessage = true
	return nil
}

func parseOpenSendUnknownMessage(config *Config, _ map[string]string) error {
	config.SendUnknownMessage = true
	return nil
}

func parseOpenDropCapability(config *Config, kv map[string]string) error {
	code, err := parseCapabilityCode(optOpenDropCapability, kv["code"])
	if err != nil {
		return err
	}
	config.CapabilityOverrides = append(config.CapabilityOverrides, CapabilityOverride{
		Code: code, Add: false,
	})
	return nil
}

// parseOpenRouterID reads option=open:value=router-id:id=<a.b.c.d>, which
// sends this BGP Identifier instead of the derived ze identifier + 1. It drives
// the RFC 6286 Section 2.2 rejection tests.
func parseOpenRouterID(config *Config, kv map[string]string) error {
	addr, err := netip.ParseAddr(kv["id"])
	if err != nil {
		return fmt.Errorf("option=open:value=router-id:id=%q is not an address: %w", kv["id"], err)
	}
	if !addr.Is4() {
		return fmt.Errorf("option=open:value=router-id:id=%q is not IPv4, and RFC 4271 Section 4.2 makes the BGP Identifier four octets", kv["id"])
	}
	octets := addr.As4()
	id := binary.BigEndian.Uint32(octets[:])
	config.RouterID = &id
	return nil
}

func parseOpenHoldTimeOption(config *Config, kv map[string]string) error {
	seconds, err := parseOpenHoldTime(config, kv["seconds"])
	if err != nil {
		return err
	}
	config.HoldTime = &seconds
	return nil
}

func parseOpenGracefulRestart(config *Config, kv map[string]string) error {
	decl, err := parseGracefulRestartDecl(config, kv)
	if err != nil {
		return err
	}
	config.GracefulRestart = decl
	return nil
}

func parseOpenLLGR(config *Config, kv map[string]string) error {
	decl, err := parseLLGRDecl(config, kv)
	if err != nil {
		return err
	}
	config.LLGR = decl
	return nil
}

func parseOpenPathsLimit(config *Config, kv map[string]string) error {
	entries, err := parsePathsLimitDecl(config, kv)
	if err != nil {
		return err
	}
	config.PathsLimit = entries
	return nil
}

func parseOpenAddCapability(config *Config, kv map[string]string) error {
	code, err := parseCapabilityCode(optOpenAddCapability, kv["code"])
	if err != nil {
		return err
	}
	val, err := hex.DecodeString(kv["hex"])
	if err != nil {
		return fmt.Errorf("option=open:value=add-capability:hex=%q is not hex: %w", kv["hex"], err)
	}
	// RFC 5492 Section 4 gives the Capability Length one octet, so a longer
	// value cannot be stated. Truncating it would put a capability on the
	// wire that says something the .ci never asked for.
	if len(val) > 255 {
		return fmt.Errorf("option=open:value=add-capability:hex= carries %d octets, RFC 5492 Section 4 states at most 255", len(val))
	}
	config.CapabilityOverrides = append(config.CapabilityOverrides, CapabilityOverride{
		Code: code, Value: val, Add: true,
	})
	return nil
}

func parseUpdateSendDefaultRoute(config *Config, _ map[string]string) error {
	config.SendDefaultRoute = true
	return nil
}

func parseUpdateSendRoute(config *Config, kv map[string]string) error {
	route, err := parseSendRoute(kv)
	if err != nil {
		return err
	}
	config.SendRoutes = append(config.SendRoutes, route)
	return nil
}

func parseUpdateSendBulk(config *Config, kv map[string]string) error {
	spec, err := parseBulkSpec(kv)
	if err != nil {
		return err
	}
	config.SendBulk = append(config.SendBulk, spec)
	return nil
}

// parseBulkSpec reads option=update:value=send-bulk into an InjectSpec.
//
// Keys: prefix, count, next-hop, origin-as, max-msg (optional, default 4096),
// eor (optional, "true" appends an End-of-RIB marker).
//
// Every key is validated and a bad one is an error rather than a zero value: a
// spec that silently degrades to count=0 sends nothing, and a test asserting
// that a route was NOT forwarded would then pass for the wrong reason
// (ai/rules/evidence.md).
func parseBulkSpec(kv map[string]string) (InjectSpec, error) {
	spec := InjectSpec{}

	prefix, err := netip.ParsePrefix(kv[keyPrefix])
	if err != nil {
		return spec, fmt.Errorf("send-bulk prefix %q: %w", kv[keyPrefix], err)
	}
	spec.Prefix = prefix

	count, err := strconv.Atoi(kv["count"])
	if err != nil || count < 1 {
		return spec, fmt.Errorf("send-bulk count %q: want a positive integer", kv["count"])
	}
	spec.Count = count

	nextHop, err := netip.ParseAddr(kv["next-hop"])
	if err != nil {
		return spec, fmt.Errorf("send-bulk next-hop %q: %w", kv["next-hop"], err)
	}
	spec.NextHop = nextHop

	asn, err := strconv.ParseUint(kv["origin-as"], 10, 32)
	if err != nil {
		return spec, fmt.Errorf("send-bulk origin-as %q: want a 32-bit ASN", kv["origin-as"])
	}
	spec.ASN = uint32(asn) //nolint:gosec // G115: bounded by ParseUint's 32-bit width

	if v := kv["max-msg"]; v != "" {
		maxMsg, err := strconv.Atoi(v)
		if err != nil || maxMsg < bgpEORLen || maxMsg > bgpExtMsgLen {
			return spec, fmt.Errorf("send-bulk max-msg %q: want %d..%d", v, bgpEORLen, bgpExtMsgLen)
		}
		spec.MaxMsgLen = maxMsg
	}

	// Default false: the EOR marker is a separate message, and a test that
	// asserts on frame sequence must be able to choose whether one arrives.
	spec.EndOfRIB = kv["eor"] == optTrue

	return spec, nil
}

// parseOpenHoldTime reads the seconds= key of an option=open:value=hold-time
// line.
//
// RFC 4271 Section 4.2: "Hold Time: ... This 2-octet unsigned integer indicates
// the number of seconds the sender proposes for the value of the Hold Timer",
// and the value "MUST be either zero or at least three seconds". 1 and 2 are
// therefore refused HERE, where the .ci is read, rather than clamped: a hold
// time the harness silently raised would make the file assert a negotiation it
// never asked for.
func parseOpenHoldTime(config *Config, value string) (uint16, error) {
	if config.HoldTime != nil {
		return 0, fmt.Errorf(
			"the peer block states option=open:value=hold-time twice, so it proposes two hold times " +
				"for one OPEN; state one")
	}
	seconds, err := strconv.ParseUint(value, 10, 64)
	if err != nil {
		return 0, fmt.Errorf("option=open:value=hold-time:seconds=%q is not a number", value)
	}
	if seconds > 65535 {
		return 0, fmt.Errorf(
			"option=open:value=hold-time:seconds=%d is above 65535, the two-octet Hold Time field of "+
				"RFC 4271 Section 4.2", seconds)
	}
	if seconds == 1 || seconds == 2 {
		return 0, fmt.Errorf(
			"option=open:value=hold-time:seconds=%d is refused: RFC 4271 Section 4.2 states the Hold "+
				"Time \"MUST be either zero or at least three seconds\"", seconds)
	}
	return uint16(seconds), nil //nolint:gosec // the range check above bounds it
}

// parseGracefulRestartDecl reads an option=open:value=graceful-restart line.
//
// RFC 4724 Section 3 gives the Restart Time 12 bits, so 4095 is the largest
// value the field can state and 4096 is refused rather than truncated: the
// encoder masks with 0x0FFF, and a masked restart time is a number the .ci
// never wrote reaching the wire in its name.
func parseGracefulRestartDecl(config *Config, kv map[string]string) (*GracefulRestartDecl, error) {
	if config.GracefulRestart != nil {
		return nil, fmt.Errorf(
			"the peer block states option=open:value=graceful-restart twice, so it states two Restart " +
				"Times for one capability; state one")
	}
	restart, err := strconv.ParseUint(kv["restart-time"], 10, 64)
	if err != nil {
		return nil, fmt.Errorf("option=open:value=graceful-restart:restart-time=%q is not a number", kv["restart-time"])
	}
	if restart > 4095 {
		return nil, fmt.Errorf(
			"option=open:value=graceful-restart:restart-time=%d is above 4095, the 12-bit Restart Time "+
				"of RFC 4724 Section 3", restart)
	}
	families, err := parseOptionFamilies(optOpenGracefulRestart, kv[keyFamily])
	if err != nil {
		return nil, err
	}
	forward, err := parseOptionFlag(optOpenGracefulRestart, "forward-state", kv["forward-state"])
	if err != nil {
		return nil, err
	}
	return &GracefulRestartDecl{
		Families:     families,
		RestartTime:  uint16(restart), //nolint:gosec // the range check above bounds it
		ForwardState: forward,
	}, nil
}

// parseLLGRDecl reads an option=open:value=llgr line.
//
// RFC 9494 Section 3 gives the Long-Lived Stale Time 24 bits, so 16777215 is the
// largest value a tuple can state. Ze clamps its own configured value to that
// number (parseLLGRCapValue, internal/component/bgp/plugins/gr/gr_llgr.go); the
// harness refuses instead, because a clamp inside the builder would put a stale
// time on the wire that no line of the .ci states.
func parseLLGRDecl(config *Config, kv map[string]string) (*LLGRDecl, error) {
	if config.LLGR != nil {
		return nil, fmt.Errorf(
			"the peer block states option=open:value=llgr twice, so it states two stale times for one " +
				"capability; state one")
	}
	stale, err := strconv.ParseUint(kv["stale-time"], 10, 64)
	if err != nil {
		return nil, fmt.Errorf("option=open:value=llgr:stale-time=%q is not a number", kv["stale-time"])
	}
	if stale > 16777215 {
		return nil, fmt.Errorf(
			"option=open:value=llgr:stale-time=%d is above 16777215, the 24-bit Long-Lived Stale Time "+
				"of RFC 9494 Section 3", stale)
	}
	families, err := parseOptionFamilies(optOpenLLGR, kv[keyFamily])
	if err != nil {
		return nil, err
	}
	forward, err := parseOptionFlag(optOpenLLGR, "forward-state", kv["forward-state"])
	if err != nil {
		return nil, err
	}
	return &LLGRDecl{
		Families:     families,
		StaleTime:    uint32(stale), //nolint:gosec // the range check above bounds it
		ForwardState: forward,
	}, nil
}

// parsePathsLimitDecl reads an option=open:value=paths-limit line and appends
// its entries to the ones already declared.
//
// draft-abraitis-idr-addpath-paths-limit Section 3 gives Max Paths two octets
// and one entry per family, so a family stated twice is two answers to one
// question and is refused. Repeating the LINE with other families is how a .ci
// states a limit for several families.
func parsePathsLimitDecl(config *Config, kv map[string]string) ([]PathsLimitDecl, error) {
	limit, err := strconv.ParseUint(kv["limit"], 10, 64)
	if err != nil {
		return nil, fmt.Errorf("option=open:value=paths-limit:limit=%q is not a number", kv["limit"])
	}
	if limit > 65535 {
		return nil, fmt.Errorf(
			"option=open:value=paths-limit:limit=%d is above 65535, the two-octet Max Paths field of "+
				"draft-abraitis-idr-addpath-paths-limit Section 3", limit)
	}
	families, err := parseOptionFamilies(optOpenPathsLimit, kv[keyFamily])
	if err != nil {
		return nil, err
	}
	if len(families) == 0 {
		return nil, fmt.Errorf(
			"option=open:value=paths-limit states no family=, and the capability carries one Max Paths " +
				"for each family rather than one for the session")
	}

	entries := config.PathsLimit
	for _, fam := range families {
		for _, held := range entries {
			if held.Family != fam {
				continue
			}
			return nil, fmt.Errorf(
				"the peer block states option=open:value=paths-limit for %s twice, with limits %d and "+
					"%d, so it states two limits for one family; state one",
				fam, held.Limit, limit)
		}
		entries = append(entries, PathsLimitDecl{Family: fam, Limit: uint16(limit)}) //nolint:gosec // the range check above bounds it
	}
	return entries, nil
}

// parseOptionFamilies reads a family= key holding a comma-separated list of
// family names, and returns the empty list when the key is absent.
//
// A name no family registry knows fails the file where it is read. The builder
// cannot refuse it later: it would have to choose between sending a capability
// short of a family the .ci asked for and sending none at all, and both are the
// silent drop this parser exists to close.
func parseOptionFamilies(option, value string) ([]family.Family, error) {
	if value == "" {
		return nil, nil
	}
	var families []family.Family
	for name := range strings.SplitSeq(value, ",") {
		name = strings.TrimSpace(name)
		fam, known := family.LookupFamily(name)
		if !known {
			return nil, fmt.Errorf("option=open:value=%s:family=%q names no family ze knows", option, name)
		}
		if slices.Contains(families, fam) {
			return nil, fmt.Errorf("option=open:value=%s:family= names %s twice", option, name)
		}
		families = append(families, fam)
	}
	return families, nil
}

// parseOptionFlag reads a boolean key of an option=open line, which defaults to
// true when the key is absent.
//
// It refuses every spelling that is not "true" or "false", because the usual
// `kv[key] == "true"` test reads a typo as false: the .ci would then state the
// opposite of what it wrote, and the F bit is exactly the field where that
// silently inverts what a receiver believes about the sender.
func parseOptionFlag(option, key, value string) (bool, error) {
	switch value {
	case "":
		return true, nil
	case optTrue:
		return true, nil
	case "false":
		return false, nil
	default:
		return false, fmt.Errorf("option=open:value=%s:%s=%q is neither true nor false", option, key, value)
	}
}

// parseCapabilityCode reads the code= key of a drop-capability or
// add-capability option.
//
// RFC 5492 Section 4 gives the Capability Code one octet, and 0 is unassigned.
// A value outside that range used to be dropped in silence, so the .ci carried a
// line nothing acted on and the test asserted against an OPEN it never asked for.
func parseCapabilityCode(option, value string) (uint8, error) {
	code, err := strconv.Atoi(value)
	if err != nil {
		return 0, fmt.Errorf("option=open:value=%s:code=%q is not a number", option, value)
	}
	if code < 1 || code > 255 {
		return 0, fmt.Errorf("option=open:value=%s:code=%d is outside 1..255, the one-octet Capability Code of RFC 5492 Section 4", option, code)
	}
	return uint8(code), nil //nolint:gosec // the range check above bounds it
}

// parseOpenASBinding reads one option=asn line into the AS ze-peer opens with.
//
// RFC 6793 Section 3 makes a four-octet AS representable rather than refusable:
// AS_TRANS goes in the My Autonomous System field and the real AS in the
// Capability Value of capability 65. So the range is the whole AS space, and
// only a value outside it is refused.
//
// The refusal happens HERE, where the .ci is read, and never in the builder. A
// value the harness cannot honor must stop the file before a socket exists,
// rather than produce a peer that quietly claims someone else's AS
// (plan/learned/005-runner-drops-what-it-cannot-honor.md).
//
// peer=<ip> binds the declaration to one of ze's endpoint addresses, for the
// tests where one ze-peer process serves several of ze's peers at once and each
// is configured for its own AS.
func parseOpenASBinding(kv map[string]string) (OpenASBinding, error) {
	value := kv[keyValue]
	as, err := strconv.ParseUint(value, 10, 64)
	if err != nil {
		return OpenASBinding{}, fmt.Errorf("option=asn:value=%q is not a number, and ze-peer must not open with an AS the .ci did not state", value)
	}
	if as < 1 || as > asMax {
		return OpenASBinding{}, fmt.Errorf("option=asn:value=%d is outside 1..%d; RFC 7607 Section 2 reserves AS 0 and RFC 6793 Section 3 makes %d the largest AS", as, uint64(asMax), uint64(asMax))
	}

	binding := OpenASBinding{AS: uint32(as)} //nolint:gosec // the range check above bounds it
	key := kv["peer"]
	if key == "" {
		return binding, nil
	}
	addr, aerr := netip.ParseAddr(key)
	if aerr != nil {
		return OpenASBinding{}, fmt.Errorf("option=asn:peer=%q is not an address: %w", key, aerr)
	}
	binding.Addr = addr
	return binding, nil
}

// parseSendRoute reads option=update:value=send-route into a RouteToSend.
//
// An absent key keeps its zero (no origin AS, no AS_PATH, no label), which is
// what a test that omits it asks for. A key that is present and does not parse
// is an error: it used to be dropped, so `origin-as=6500x` sent AS 0 and the
// test asserted against a route it never described.
func parseSendRoute(kv map[string]string) (RouteToSend, error) {
	route := RouteToSend{
		Prefix:  kv[keyPrefix],
		NextHop: kv["next-hop"],
		ASSet:   kv["as-set"] == optTrue,
	}
	if v, ok := kv["origin-as"]; ok {
		asn, err := parseSendRouteASN("origin-as", v)
		if err != nil {
			return route, err
		}
		route.OriginAS = asn
	}
	if v := kv["as-path"]; v != "" {
		for s := range strings.SplitSeq(v, ",") {
			asn, err := parseSendRouteASN("as-path", strings.TrimSpace(s))
			if err != nil {
				return route, err
			}
			route.ASPath = append(route.ASPath, asn)
		}
	}
	if v := kv["originator-id"]; v != "" {
		id, err := parseSendRouteIPv4("originator-id", v)
		if err != nil {
			return route, err
		}
		route.OriginatorID = id
	}
	if v := kv["cluster-list"]; v != "" {
		for s := range strings.SplitSeq(v, ",") {
			id, err := parseSendRouteIPv4("cluster-list", strings.TrimSpace(s))
			if err != nil {
				return route, err
			}
			route.ClusterList = append(route.ClusterList, id)
		}
	}
	if v := kv["label"]; v != "" {
		label, err := strconv.ParseUint(v, 10, 32)
		if err != nil {
			return route, fmt.Errorf("option=update:value=send-route:label=%q is not a number", v)
		}
		route.Labels = []uint32{uint32(label)}
	}
	return route, nil
}

// parseSendRouteASN reads one 32-bit AS number of a send-route key.
func parseSendRouteASN(key, value string) (uint32, error) {
	asn, err := strconv.ParseUint(value, 10, 32)
	if err != nil {
		return 0, fmt.Errorf("option=update:value=send-route:%s= holds %q, which is not a 32-bit AS number", key, value)
	}
	return uint32(asn), nil
}

// parseSendRouteIPv4 reads one IPv4 address of a send-route key as the
// four-octet number the path attribute carries.
func parseSendRouteIPv4(key, value string) (uint32, error) {
	addr, err := netip.ParseAddr(value)
	if err != nil || !addr.Is4() {
		return 0, fmt.Errorf("option=update:value=send-route:%s= holds %q, which is not an IPv4 address", key, value)
	}
	octets := addr.As4()
	return binary.BigEndian.Uint32(octets[:]), nil
}
