// Design: docs/architecture/testing/interop.md -- RFC 8097 eBGP boundary interop evidence.
// Related: check_special.go -- registers scenarioRPKIOVEBGPBoundary.
// Related: register_speaker.go -- registers the raw-wire RFC 8097 speaker oracles.
package bgp

import (
	"context"
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"net/netip"
	"strings"
	"time"

	"github.com/ze-software/ze/internal/core/bgp/attribute"
	"github.com/ze-software/ze/internal/core/textbuf"
	"github.com/ze-software/ze/internal/le/interoplab"
)

const (
	scenarioRPKIOVEBGPBoundary = "bgp-rpki-ov-ebgp-boundary-gobgp"

	speakerOracleRFC8097EBGPStrip = "rfc8097-ebgp-strip"
	speakerOracleRFC8097EBGPOptIn = "rfc8097-ebgp-optin"

	rfc8097DefaultInPrefix = "10.97.1.0/24"
	rfc8097AcceptInPrefix  = "10.97.2.0/24"
	rfc8097IBGPPrefix      = "10.97.3.0/24"

	rfc8097InboundRT = "65001:7"
	rfc8097IBGPRT    = "65000:9"

	rfc8097RouteNotePrefix = "rfc8097-route: "
	rfc8097ExtCommCode     = 16
	rfc8097ExtCommOctets   = 8
	rfc8097OVAbsent        = "absent"
)

// checkRPKIOVEBGPBoundary verifies RFC 8097 Section 2 eBGP boundary behavior
// end-to-end across Ze, GoBGP (iBGP AS 65001), default/opted-in eBGP injectors,
// and two raw-wire eBGP speaker oracles:
//   - Inbound default eBGP (inject, AS 65004): 10.97.1.0/24 carries target:65001:7
//     and 0x43/0x00:valid; Ze drops 0x43/0x00 on receive so iBGP GoBGP and
//     opted-in eBGP speaker2 see target:65001:7 alone.
//   - Inbound opted-in eBGP (ze-peer, AS 65005, accept-origin-validation-state true):
//     10.97.2.0/24 carries target:65001:7 and 0x43/0x00:valid; iBGP GoBGP sees both.
//
// RFC requirement: RFC8097-2-9 positive -- iBGP GoBGP injects 10.97.3.0/24
// with target:65000:9 and 0x43/0x00:valid; default eBGP raw-wire oracle
// speaker receives 10.97.1.0/24, 10.97.2.0/24, and 10.97.3.0/24 with sibling
// Route Targets intact and zero 0x43/0x00 entries on the wire.
// RFC requirement: RFC8097-2-9 negative -- opted-in eBGP raw-wire oracle
// speaker2 (send-origin-validation-state true) retains 0x43/0x00:valid on
// 10.97.2.0/24 and 10.97.3.0/24 while default speaker strips them.
// RFC requirement: RFC8097-2-10 positive -- opted-in eBGP raw-wire oracle
// speaker2 (send-origin-validation-state true) receives 10.97.2.0/24 and
// 10.97.3.0/24 with sibling Route Targets and 0x43/0x00:valid (Reserved=0)
// intact, and ze-peer (accept-origin-validation-state true) delivers 0x43/0x00
// on 10.97.2.0/24 to iBGP GoBGP.
// RFC requirement: RFC8097-2-10 negative -- default eBGP inject (accept
// omitted) has 0x43/0x00 dropped on 10.97.1.0/24, and default eBGP speaker
// (send omitted) receives zero 0x43/0x00 entries.
func checkRPKIOVEBGPBoundary(ctx context.Context, check *interoplab.CheckContext) error {
	fail := func(assertion int, cause error) error {
		return checkerFailure(ctx, check.Lab, scenarioRPKIOVEBGPBoundary, assertion, cause)
	}
	if !check.Network.IPv4.IsValid() {
		return fail(1, errors.New("RFC 8097 boundary scenario has no selected IPv4 network"))
	}
	zeAddress := networkHostAddress(check.Network, 2)
	gobgpAddress := networkHostAddress(check.Network, 5)

	// Assertion 2. GoBGP iBGP session to Ze is established before injecting.
	session := operation{kind: opGoBGPSession, argument: zeAddress}
	if err := runOperation(ctx, check.Network, check.Lab, &session); err != nil {
		return fail(2, err)
	}

	// Assertion 3. Inject 10.97.3.0/24 on iBGP GoBGP carrying target:65000:9
	// and RFC 8097 0x43/0x00 valid; this also wakes inject.msg's barrier.
	if _, err := check.Lab.Exec(ctx, peerGoBGP, []string{
		cmdGoBGP, gobgpGlobal, gobgpRIB, gobgpAdd, rfc8097IBGPPrefix,
		"-a", gobgpFamilyIPv4, gobgpNextHop, gobgpAddress,
		"rt", rfc8097IBGPRT, "valid",
	}, nil); err != nil {
		return fail(3, err)
	}

	// Assertions 4 and 5. Both eBGP-sourced routes arrive at iBGP GoBGP.
	for index, prefix := range []string{rfc8097DefaultInPrefix, rfc8097AcceptInPrefix} {
		arrival := operation{kind: opGoBGPRoute, argument: prefix, timeout: 90 * time.Second}
		if err := runOperation(ctx, check.Network, check.Lab, &arrival); err != nil {
			return fail(index+4, err)
		}
	}

	// Assertion 6. GoBGP's decoded RIB proves inbound eBGP polarity:
	// 10.97.1.0/24 (default eBGP) holds target:65001:7 without 0x43/0x00;
	// 10.97.2.0/24 (accept-origin-validation-state true) holds both.
	paths, err := check.Lab.Query(ctx, peerGoBGP, []string{cmdGoBGPPaths, "1", "1"}, nil)
	if err != nil {
		return fail(6, err)
	}
	if err := requireGoBGPRPKIOVPaths(paths); err != nil {
		return fail(6, interoplab.WithSearchedOutput(err, paths))
	}

	// Assertion 7. Raw-wire default eBGP speaker proves Ze strips 0x43/0x00
	// on egress while preserving sibling Route Targets on all three prefixes.
	defaultLogs, err := waitRFC8097SpeakerReport(ctx, check.Lab, peerSpeaker)
	if err != nil {
		return fail(7, err)
	}
	if err := requireRFC8097SpeakerReport(defaultLogs, speakerOracleRFC8097EBGPStrip, false); err != nil {
		return fail(7, interoplab.WithSearchedOutput(err, defaultLogs))
	}

	// Assertion 8. Raw-wire opted-in eBGP speaker2 proves Ze transmits
	// 0x43/0x00:valid (Reserved=0) on 10.97.2.0/24 and 10.97.3.0/24 when
	// send-origin-validation-state is true, while 10.97.1.0/24 stays stripped.
	optInLogs, err := waitRFC8097SpeakerReport(ctx, check.Lab, peerSpeaker2)
	if err != nil {
		return fail(8, err)
	}
	if err := requireRFC8097SpeakerReport(optInLogs, speakerOracleRFC8097EBGPOptIn, true); err != nil {
		return fail(8, interoplab.WithSearchedOutput(err, optInLogs))
	}

	// Assertion 9. GoBGP session remained Established throughout.
	if err := runOperation(ctx, check.Network, check.Lab, &session); err != nil {
		return fail(9, err)
	}
	return nil
}

type gobgpExtCommAttr struct {
	Type  uint8             `json:"type"`
	Value []gobgpExtCommRow `json:"value"`
}

type gobgpExtCommRow struct {
	Type    uint8           `json:"type"`
	Subtype uint8           `json:"subtype"`
	Value   json.RawMessage `json:"value"`
}

// requireGoBGPRPKIOVPaths validates gobgp-paths 1 1 output for the inbound
// eBGP polarity pair: default-dropped 10.97.1.0/24 and opted-in 10.97.2.0/24.
func requireGoBGPRPKIOVPaths(output string) error {
	var seenDefault, seenAccept bool
	for line := range strings.SplitSeq(output, "\n") {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			continue
		}
		prefix, _, _ := strings.Cut(trimmed, " ")
		if prefix != rfc8097DefaultInPrefix && prefix != rfc8097AcceptInPrefix {
			continue
		}
		_, rawJSON, ok := strings.Cut(trimmed, "ExtcommsJSON: ")
		if !ok {
			return fmt.Errorf("GoBGP route %s has no ExtcommsJSON extended communities", prefix)
		}
		var attr gobgpExtCommAttr
		if err := json.Unmarshal([]byte(strings.TrimSpace(rawJSON)), &attr); err != nil {
			return fmt.Errorf("decode GoBGP ExtcommsJSON for %s: %w", prefix, err)
		}
		hasRT := false
		hasOV := false
		var ovState uint8
		for _, entry := range attr.Value {
			if entry.Type == 0 && entry.Subtype == 2 {
				var rt string
				if json.Unmarshal(entry.Value, &rt) == nil && rt == rfc8097InboundRT {
					hasRT = true
				}
			}
			if entry.Type == attribute.ExtCommTypeNonTransOpaque && entry.Subtype == attribute.ExtCommSubTypeOriginValidation {
				hasOV = true
				if err := json.Unmarshal(entry.Value, &ovState); err != nil {
					return fmt.Errorf("decode GoBGP RFC 8097 state for %s: %w", prefix, err)
				}
			}
		}
		if !hasRT {
			return fmt.Errorf("GoBGP route %s lacks sibling route target %s", prefix, rfc8097InboundRT)
		}
		if prefix == rfc8097DefaultInPrefix {
			if hasOV {
				return fmt.Errorf("GoBGP route %s leaked RFC 8097 0x43/0x00 state %d from default eBGP peer", prefix, ovState)
			}
			seenDefault = true
			continue
		}
		if !hasOV {
			return fmt.Errorf("GoBGP route %s lost RFC 8097 0x43/0x00 from opted-in eBGP peer", prefix)
		}
		if ovState != uint8(attribute.OriginValidationValid) {
			return fmt.Errorf("GoBGP route %s has RFC 8097 state %d, want 0 (valid)", prefix, ovState)
		}
		seenAccept = true
	}
	if !seenDefault {
		return fmt.Errorf("GoBGP global RIB missing default eBGP route %s", rfc8097DefaultInPrefix)
	}
	if !seenAccept {
		return fmt.Errorf("GoBGP global RIB missing opted-in eBGP route %s", rfc8097AcceptInPrefix)
	}
	return nil
}

func waitRFC8097SpeakerReport(ctx context.Context, lab interoplab.CheckerLab, peer string) (string, error) {
	var last string
	logs, _, err := interoplab.Wait(ctx, interoplab.WaitOptions{
		Timeout:     120 * time.Second,
		Interval:    time.Second,
		Description: "RFC 8097 wire speaker report from " + peer,
	}, func(probeCtx context.Context) (string, error) {
		answer, logErr := lab.Logs(probeCtx, peer, 4000)
		if logErr != nil {
			return "", logErr
		}
		if !answer.Available {
			return "", errors.New("speaker logs were not read")
		}
		last = answer.Text
		return answer.Text, nil
	}, func(text string) bool {
		return strings.Contains(text, "\nresult: ") || strings.HasPrefix(text, "result: ")
	})
	if err != nil {
		return "", interoplab.WithLastAnswer(err, last)
	}
	return logs, nil
}

// applyRFC8097EBGPStripOracle inspects raw wire UPDATEs sent to a default eBGP
// peer: any 0x43/0x00 entry fails immediately, and each IPv4 route's observed
// Route Target and 0x43/0x00 disposition are recorded in verdict.notes.
func applyRFC8097EBGPStripOracle(update speakerUpdate, verdict *speakerVerdict) {
	observeRFC8097WireUpdate(update, verdict, false)
}

// applyRFC8097EBGPOptInOracle inspects raw wire UPDATEs sent to an opted-in
// eBGP peer (send-origin-validation-state true): any non-zero Reserved octet
// [2..6] in 0x43/0x00 fails immediately, and each IPv4 route's observed Route
// Target and 0x43/0x00 disposition are recorded in verdict.notes.
func applyRFC8097EBGPOptInOracle(update speakerUpdate, verdict *speakerVerdict) {
	observeRFC8097WireUpdate(update, verdict, true)
}

func observeRFC8097WireUpdate(update speakerUpdate, verdict *speakerVerdict, allowOV bool) {
	if len(update.nlri) == 0 {
		return
	}
	prefixes, err := decodeIPv4UnicastNLRI(update.nlri)
	if err != nil {
		verdict.fail("malformed IPv4 NLRI: " + err.Error())
		return
	}
	rt := "none"
	ov := rfc8097OVAbsent
	for _, attr := range update.attributes {
		if attr.code != rfc8097ExtCommCode {
			continue
		}
		if len(attr.value)%rfc8097ExtCommOctets != 0 {
			var message textbuf.Buffer
			verdict.fail(message.Str("EXTENDED_COMMUNITIES length ").Int(int64(len(attr.value))).Str(" is not a multiple of 8").String())
			continue
		}
		for off := 0; off < len(attr.value); off += rfc8097ExtCommOctets {
			entry := attr.value[off : off+rfc8097ExtCommOctets]
			if entry[0] == 0x00 && entry[1] == 0x02 {
				asn := binary.BigEndian.Uint16(entry[2:4])
				val := binary.BigEndian.Uint32(entry[4:8])
				var formatted textbuf.Buffer
				rt = formatted.Uint16(asn).Byte(':').Uint32(val).String()
			}
			if entry[0] == attribute.ExtCommTypeNonTransOpaque && entry[1] == attribute.ExtCommSubTypeOriginValidation {
				state := entry[7]
				var stateText textbuf.Buffer
				ov = stateText.Uint8(state).String()
				if !allowOV {
					var message textbuf.Buffer
					verdict.fail(message.Str("RFC 8097 Section 2: default eBGP peer received 0x43/0x00 state=").Uint8(state).String())
				}
				for idx := 2; idx < 7; idx++ {
					if entry[idx] != 0 {
						var message textbuf.Buffer
						verdict.fail(message.Str("RFC 8097 Section 2: transmitted 0x43/0x00 has non-zero Reserved octet ").Int(int64(idx)).String())
						break
					}
				}
			}
		}
	}
	for _, prefix := range prefixes {
		var note textbuf.Buffer
		verdict.notes = append(verdict.notes,
			note.Str(rfc8097RouteNotePrefix).Str(prefix).Str(" rt=").Str(rt).Str(" ov=").Str(ov).String())
	}
}

func decodeIPv4UnicastNLRI(data []byte) ([]string, error) {
	prefixes := make([]string, 0, 2)
	for pos := 0; pos < len(data); {
		bits := int(data[pos])
		if bits > 32 {
			return nil, fmt.Errorf("IPv4 prefix length %d exceeds 32", bits)
		}
		octets := (bits + 7) / 8
		if pos+1+octets > len(data) {
			return nil, errors.New("truncated IPv4 NLRI")
		}
		var addr [4]byte
		copy(addr[:octets], data[pos+1:pos+1+octets])
		prefixes = append(prefixes, netip.PrefixFrom(netip.AddrFrom4(addr), bits).String())
		pos += 1 + octets
	}
	return prefixes, nil
}

// requireRFC8097SpeakerReport checks a native speaker's report for PASS,
// established=yes, the expected plugin oracle name, and exact per-prefix
// Route Target and 0x43/0x00 wire dispositions.
func requireRFC8097SpeakerReport(logs, wantPlugin string, optIn bool) error {
	result, plugin, established := "", "", ""
	routes := make(map[string]string, 3)
	for line := range strings.SplitSeq(logs, "\n") {
		trimmed := strings.TrimSpace(line)
		switch {
		case strings.HasPrefix(trimmed, "result: "):
			result = strings.TrimPrefix(trimmed, "result: ")
		case strings.HasPrefix(trimmed, "plugin: "):
			plugin = strings.TrimPrefix(trimmed, "plugin: ")
		case strings.HasPrefix(trimmed, "note: established: "):
			established = strings.TrimPrefix(trimmed, "note: established: ")
		case strings.HasPrefix(trimmed, "note: "+rfc8097RouteNotePrefix):
			rest := strings.TrimPrefix(trimmed, "note: "+rfc8097RouteNotePrefix)
			prefix, attrs, ok := strings.Cut(rest, " ")
			if ok {
				routes[prefix] = attrs
			}
		}
	}
	if plugin != wantPlugin {
		return fmt.Errorf("speaker ran oracle %q, want %q", plugin, wantPlugin)
	}
	if established != logValueYes {
		return fmt.Errorf("speaker %s was not established (got %q)", wantPlugin, established)
	}
	if result != speakerResultPass {
		return fmt.Errorf("speaker %s reported result=%q, want %s", wantPlugin, result, speakerResultPass)
	}
	wantOV := rfc8097OVAbsent
	if optIn {
		wantOV = "0"
	}
	expected := []struct {
		prefix string
		rt     string
		ov     string
	}{
		// 10.97.1.0/24 arrived from default eBGP inject -> dropped on inbound, so ov=absent on BOTH speakers.
		{prefix: rfc8097DefaultInPrefix, rt: rfc8097InboundRT, ov: rfc8097OVAbsent},
		// 10.97.2.0/24 arrived from opted-in eBGP ze-peer -> stripped on default speaker, kept on opt-in speaker2.
		{prefix: rfc8097AcceptInPrefix, rt: rfc8097InboundRT, ov: wantOV},
		// 10.97.3.0/24 arrived from iBGP GoBGP -> stripped on default speaker, kept on opt-in speaker2.
		{prefix: rfc8097IBGPPrefix, rt: rfc8097IBGPRT, ov: wantOV},
	}
	for _, want := range expected {
		got, found := routes[want.prefix]
		if !found {
			return fmt.Errorf("speaker %s never observed wire UPDATE for %s", wantPlugin, want.prefix)
		}
		var expectedAttrs textbuf.Buffer
		wantStr := expectedAttrs.Str("rt=").Str(want.rt).Str(" ov=").Str(want.ov).String()
		if got != wantStr {
			return fmt.Errorf("speaker %s route %s had %q, want %q", wantPlugin, want.prefix, got, wantStr)
		}
	}
	return nil
}
