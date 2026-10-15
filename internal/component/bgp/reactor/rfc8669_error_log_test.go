// Design: docs/architecture/wire/messages.md -- RFC 7606 error log at the default level.
// RFC: rfc/short/rfc8669.md -- Sections 4, 4.1 and 6 error logging on discard.
// Related: rfc7606_session_diagnostics_test.go -- synchronized session log capture.
// Related: rfc8669_duplicate_tlv_test.go -- labeled-unicast IBGP receive fixture.

package reactor

import (
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/bgp/message"
	"github.com/ze-software/ze/internal/component/bgp/wireu"
	"github.com/ze-software/ze/internal/core/bgp/attribute"
	"github.com/ze-software/ze/internal/core/clock"
	"github.com/ze-software/ze/internal/core/slogutil"
)

// defaultSessionLogLevel is the level slogutil.Logger gives a subsystem with no
// ze.log setting, read from slogutil so a change there moves these tests too. A
// test that asks what an operator sees by default captures at this level, never
// at Debug.
const defaultSessionLogLevel = slogutil.DefaultLevel

// rfc7606ErrorLines returns the ERROR records of the RFC 7606 error log that
// carry every one of fields.
func rfc7606ErrorLines(output string, fields ...string) []string {
	var matching []string
	for line := range strings.SplitSeq(output, "\n") {
		if !strings.Contains(line, "level=ERROR") {
			continue
		}
		if !strings.Contains(line, `msg="`+rfc7606ErrorLogMessage+`"`) {
			continue
		}
		all := true
		for _, field := range fields {
			if !strings.Contains(line, field) {
				all = false
				break
			}
		}
		if all {
			matching = append(matching, line)
		}
	}
	return matching
}

// TestRFC8669PrefixSIDDiscardLoggedAtDefaultLevel receives a malformed Prefix-SID
// with the session logger at its default level and reads the log an operator
// gets. Method: the real receive entry over two framing errors, then exactly one
// ERROR record naming the attribute, its name, the event and the reason.
// RFC 8669 Section 6: "When discarding an attribute, a BGP speaker SHOULD log an error for further analysis."
// MUTATION: lower the level in Session.logRFC7606Error to Debug, or drop its call
// in the attribute-discard branch of Session.applyRFC7606; no record is captured.
//
// RFC requirement: RFC8669-6-4 positive -- an actual malformed code40 discard for a TLV overrun or trailing bytes on IPv4 labeled unicast emits, at the default session log level, exactly one ERROR record naming attribute 40, its name, the attribute-discard event, the peer and a reason.
func TestRFC8669PrefixSIDDiscardLoggedAtDefaultLevel(t *testing.T) {
	clean := labelIndexTLV("86690604")
	for _, tc := range []struct {
		name      string
		prefixSID string
	}{
		{name: "TLV overrun", prefixSID: clean[:len(clean)-2]},
		{name: "trailing bytes", prefixSID: clean + "aa"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			// No parallel execution: captureSessionLog swaps a process-global logger.
			log := captureSessionLog(t, defaultSessionLogLevel)
			_, wu, action, err := rfc8669DuplicateReceive(t, safiLabeled,
				rfc8669PrefixSIDAttr(tc.prefixSID, false), communityAttr, false)
			require.NoError(t, err)
			require.Equal(t, message.RFC7606ActionAttributeDiscard, action)
			require.NotNil(t, wu)

			lines := rfc7606ErrorLines(log.String(), " attr=40 ", " event=attribute-discard ")
			require.Lenf(t, lines, 1, "want one ERROR record at the default level; captured:\n%s", log.String())
			name := attribute.AttrPrefixSID.String()
			assert.Contains(t, lines[0], "attr-name="+name)
			assert.Contains(t, lines[0], " description=")
			assert.Contains(t, lines[0], " peer=")
		})
	}
}

// atomicAggregateLength1 is an ATOMIC_AGGREGATE (code 6) one octet long. RFC 7606
// Section 7.6 makes Ze discard it, so it puts a second discard in the UPDATE.
const atomicAggregateLength1 = "40060100"

// TestRFC8669PrefixSIDDiscardBesideAnotherLogsBoth discards a Prefix-SID in the
// same UPDATE as a malformed ATOMIC_AGGREGATE and reads the log at the default
// level. Method: the real receive entry, on an internal session with a TLV
// overrun (the ATOMIC_AGGREGATE before and after code 40, so each code is the
// first discard once) and on an EBGP session outside the SR domain (the
// boundary discard is recorded after the walk's own). Each must give one ERROR
// record naming attr=40 and one naming attr=6.
// RFC 8669 Section 6: "When discarding an attribute, a BGP speaker SHOULD log an error for further analysis."
// RFC 8669 Section 4: "A BGP speaker SHOULD log an error for further analysis when discarding an attribute."
// PREVENTS: the audit finding of 2026-10-15, where one record named only the
// first discarded code and the Prefix-SID discard reached the operator at Debug.
// MUTATION: log result.AttrCode and result.Description for every entry in
// Session.logRFC7606Discards; the per-code window keeps one record, so a case
// loses its attr=40 or its attr=6 record.
//
// RFC requirement: RFC8669-6-4 positive -- a Prefix-SID discarded for a TLV overrun on IPv4 labeled unicast over an internal session, with a one-octet ATOMIC_AGGREGATE discarded in the same UPDATE before or after it, emits at the default session log level exactly one attribute-discard ERROR record naming attribute 40 whose reason cites RFC 8669, and exactly one naming attribute 6 whose reason does not.
// RFC requirement: RFC8669-4-2 positive -- a Prefix-SID discarded from an EBGP peer outside the SR domain, with a one-octet ATOMIC_AGGREGATE discarded in the same UPDATE, emits at the default session log level exactly one attribute-discard ERROR record naming attribute 40 whose reason cites RFC 8669, and exactly one naming attribute 6 whose reason does not.
func TestRFC8669PrefixSIDDiscardBesideAnotherLogsBoth(t *testing.T) {
	clean := labelIndexTLV("86690604")
	overrun := rfc8669PrefixSIDAttr(clean[:len(clean)-2], false)
	receive := map[string]func(t *testing.T) (message.RFC7606Action, error){
		"IBGP TLV overrun, ATOMIC_AGGREGATE after": func(t *testing.T) (message.RFC7606Action, error) {
			_, _, action, err := rfc8669DuplicateReceive(t, safiLabeled, overrun, atomicAggregateLength1, false)
			return action, err
		},
		"IBGP TLV overrun, ATOMIC_AGGREGATE before": func(t *testing.T) (message.RFC7606Action, error) {
			_, _, action, err := rfc8669DuplicateReceive(t, safiLabeled, atomicAggregateLength1+overrun, "", false)
			return action, err
		},
		"EBGP boundary": func(t *testing.T) (message.RFC7606Action, error) {
			attrs := append(rfc8669PrefixSIDAttrs(), 0x40, 0x06, 0x01, 0x00)
			_, action, err := rfc8669EBGPSession(false).enforceRFC7606(
				wireu.NewWireUpdate(makeUpdateBody(nil, attrs, []byte{24, 10, 0, 0}), 0))
			return action, err
		},
	}
	for name, run := range receive {
		t.Run(name, func(t *testing.T) {
			// No parallel execution: captureSessionLog swaps a process-global logger.
			log := captureSessionLog(t, defaultSessionLogLevel)
			action, err := run(t)
			require.NoError(t, err)
			require.Equal(t, message.RFC7606ActionAttributeDiscard, action)

			sid := rfc7606ErrorLines(log.String(), " attr=40 ", " event=attribute-discard ")
			require.Lenf(t, sid, 1, "the Prefix-SID discard; captured:\n%s", log.String())
			assert.Contains(t, sid[0], "RFC 8669")
			atomic := rfc7606ErrorLines(log.String(), " attr=6 ", " event=attribute-discard ")
			require.Lenf(t, atomic, 1, "the ATOMIC_AGGREGATE discard; captured:\n%s", log.String())
			assert.NotContains(t, atomic[0], "RFC 8669", "each record carries its own attribute's reason")
		})
	}
}

// TestRFC8669InvalidPrefixSIDDiscardBesideAnotherLogsBoth receives an "invalid"
// Prefix-SID (well-formed, but on labeled unicast with no Label-Index TLV) in
// the same UPDATE as a one-octet ATOMIC_AGGREGATE, both of which Ze discards.
// Method: the real receive entry on an internal session at the default level,
// with the ATOMIC_AGGREGATE before and after code 40, then one ERROR record
// naming attr=40 with the missing Label-Index and one naming attr=6.
// RFC 8669 Section 4.1: "In the case of an "invalid" BGP Prefix-SID attribute, a BGP speaker MUST follow the error-handling rules specified in Section 6. A BGP speaker SHOULD log an error for further analysis."
// MUTATION: log result.AttrCode and result.Description for every entry in
// Session.logRFC7606Discards; a case loses its attr=40 or its attr=6 record.
//
// RFC requirement: RFC8669-4.1-15 positive -- an invalid Prefix-SID (an Originator SRGB TLV and no Label-Index TLV on IPv4 labeled unicast) discarded over an internal session, with a one-octet ATOMIC_AGGREGATE discarded in the same UPDATE before or after it, emits at the default session log level exactly one attribute-discard ERROR record naming attribute 40 whose reason names the missing Label-Index TLV, and exactly one naming attribute 6 whose reason does not.
func TestRFC8669InvalidPrefixSIDDiscardBesideAnotherLogsBoth(t *testing.T) {
	invalid := rfc8669PrefixSIDAttr(originatorSRGBTLV, false)
	for _, tc := range []struct {
		name   string
		first  string
		second string
	}{
		{name: "ATOMIC_AGGREGATE after", first: invalid, second: atomicAggregateLength1},
		{name: "ATOMIC_AGGREGATE before", first: atomicAggregateLength1 + invalid, second: ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			// No parallel execution: captureSessionLog swaps a process-global logger.
			log := captureSessionLog(t, defaultSessionLogLevel)
			_, _, action, err := rfc8669DuplicateReceive(t, safiLabeled, tc.first, tc.second, false)
			require.NoError(t, err)
			require.Equal(t, message.RFC7606ActionAttributeDiscard, action)

			sid := rfc7606ErrorLines(log.String(), " attr=40 ", " event=attribute-discard ")
			require.Lenf(t, sid, 1, "the invalid Prefix-SID discard; captured:\n%s", log.String())
			assert.Contains(t, sid[0], "no Label-Index TLV")
			atomic := rfc7606ErrorLines(log.String(), " attr=6 ", " event=attribute-discard ")
			require.Lenf(t, atomic, 1, "the ATOMIC_AGGREGATE discard; captured:\n%s", log.String())
			assert.NotContains(t, atomic[0], "Label-Index", "each record carries its own attribute's reason")
		})
	}
}

// TestRFC8669InvalidPrefixSIDLoggedAtDefaultLevel receives an "invalid"
// Prefix-SID, a labeled-unicast one whose only TLV is an Originator SRGB, so it
// carries no Label-Index. Method: the real receive entry at the default level,
// then exactly one ERROR record naming attribute 40 and the missing Label-Index.
// RFC 8669 Section 4.1: "In the case of an "invalid" BGP Prefix-SID attribute, a BGP speaker MUST follow the error-handling rules specified in Section 6. A BGP speaker SHOULD log an error for further analysis."
// MUTATION: lower the level in Session.logRFC7606Error to Debug; no record is captured.
//
// RFC requirement: RFC8669-4.1-15 positive -- an invalid Prefix-SID (no Label-Index TLV) on IPv4 labeled unicast is discarded and emits, at the default session log level, exactly one ERROR record naming attribute 40 and the missing Label-Index as the reason.
func TestRFC8669InvalidPrefixSIDLoggedAtDefaultLevel(t *testing.T) {
	log := captureSessionLog(t, defaultSessionLogLevel)
	_, _, action, err := rfc8669DuplicateReceive(t, safiLabeled,
		rfc8669PrefixSIDAttr(originatorSRGBTLV, false), communityAttr, false)
	require.NoError(t, err)
	require.Equal(t, message.RFC7606ActionAttributeDiscard, action)

	lines := rfc7606ErrorLines(log.String(), " attr=40 ", " event=attribute-discard ")
	require.Lenf(t, lines, 1, "captured:\n%s", log.String())
	assert.Contains(t, lines[0], "no Label-Index TLV")
}

// TestRFC8669EBGPBoundaryDiscardLoggedAtDefaultLevel drives a Prefix-SID from
// an EBGP peer outside the SR domain, which Section 4 makes Ze discard, and
// reads the log at the default level.
// RFC 8669 Section 4: "A BGP speaker SHOULD log an error for further analysis when discarding an attribute."
// MUTATION: drop the Session.logRFC7606Error call in the attribute-discard
// branch of Session.applyRFC7606; no record is captured.
//
// RFC requirement: RFC8669-4-2 positive -- a Prefix-SID discarded from an EBGP peer outside the SR domain emits, at the default session log level, exactly one ERROR record naming attribute 40, the attribute-discard event and the peer.
func TestRFC8669EBGPBoundaryDiscardLoggedAtDefaultLevel(t *testing.T) {
	log := captureSessionLog(t, defaultSessionLogLevel)
	s := rfc8669EBGPSession(false)
	body := makeUpdateBody(nil, rfc8669PrefixSIDAttrs(), []byte{24, 10, 0, 0})
	_, action, err := s.enforceRFC7606(wireu.NewWireUpdate(body, 0))
	require.NoError(t, err)
	require.Equal(t, message.RFC7606ActionAttributeDiscard, action)

	lines := rfc7606ErrorLines(log.String(), " attr=40 ", " event=attribute-discard ",
		" peer="+s.settings.Address.String()+" ")
	require.Lenf(t, lines, 1, "captured:\n%s", log.String())
}

// TestRFC8669CleanPrefixSIDNoErrorLog pairs the malformed inputs with the
// well-formed Label-Index at the default level: nothing is discarded, so no
// error is logged.
// RFC 8669 Section 6: "When discarding an attribute, a BGP speaker SHOULD log an error for further analysis."
// RFC 8669 Section 4.1: "In the case of an "invalid" BGP Prefix-SID attribute, a BGP speaker MUST follow the error-handling rules specified in Section 6. A BGP speaker SHOULD log an error for further analysis."
// MUTATION: call Session.logRFC7606Error from the ActionNone branch of
// Session.applyRFC7606, or make the validator read this Label-Index as
// malformed; an ERROR record appears for an UPDATE that discarded nothing.
//
// RFC requirement: RFC8669-6-4 negative -- a well-formed Label-Index Prefix-SID on IPv4 labeled unicast is not discarded (ActionNone) and emits, at the default session log level, no ERROR record naming attribute 40.
// RFC requirement: RFC8669-4.1-15 negative -- a valid Prefix-SID (a Label-Index TLV on IPv4 labeled unicast) is kept with ActionNone and emits, at the default session log level, no ERROR record naming attribute 40.
func TestRFC8669CleanPrefixSIDNoErrorLog(t *testing.T) {
	log := captureSessionLog(t, defaultSessionLogLevel)
	_, _, action, err := rfc8669DuplicateReceive(t, safiLabeled,
		rfc8669PrefixSIDAttr(labelIndexTLV("86690604"), false), communityAttr, false)
	require.NoError(t, err)
	require.Equal(t, message.RFC7606ActionNone, action)
	assert.Empty(t, rfc7606ErrorLines(log.String(), " attr=40 "))
}

// TestRFC8669EBGPAcceptedPrefixSIDNoErrorLog pairs the EBGP boundary discard
// with its exception: an EBGP peer configured to accept the Prefix-SID keeps it,
// so nothing is discarded and nothing is logged at the default level.
// RFC 8669 Section 4: "A BGP speaker receiving a BGP Prefix-SID attribute from an External BGP (EBGP) neighbor residing outside the boundaries of the SR domain MUST discard the attribute unless it is configured to accept the attribute from the EBGP neighbor.  A BGP speaker SHOULD log an error for further analysis when discarding an attribute."
// MUTATION: ignore AcceptSRv6PrefixSID in Session.validateRFC7606Attrs; the
// attribute is discarded and an ERROR record names attribute 40.
//
// RFC requirement: RFC8669-4-2 negative -- a Prefix-SID from an EBGP peer configured to accept it is kept (ActionNone) and emits, at the default session log level, no ERROR record naming attribute 40.
func TestRFC8669EBGPAcceptedPrefixSIDNoErrorLog(t *testing.T) {
	log := captureSessionLog(t, defaultSessionLogLevel)
	s := rfc8669EBGPSession(true)
	body := makeUpdateBody(nil, rfc8669PrefixSIDAttrs(), []byte{24, 10, 0, 0})
	_, action, err := s.enforceRFC7606(wireu.NewWireUpdate(body, 0))
	require.NoError(t, err)
	require.Equal(t, message.RFC7606ActionNone, action)
	assert.Emptyf(t, rfc7606ErrorLines(log.String(), " attr=40 "), "captured:\n%s", log.String())
}

// TestRFC7606TreatAsWithdrawLoggedAtDefaultLevel: a malformed ORIGIN withdraws
// the route, and at the default level the operator sees why.
// PREVENTS: the 2026-10-09 journal row, where a route vanished with a
// Debug-only trace. RFC 7606 Section 6: "At a minimum, such facilities must
// include logging an error listing the NLRI involved and containing the entire
// malformed UPDATE message when such an attribute is detected." The NLRI and
// the UPDATE stay in the Debug diagnostics; the error says one arrived.
func TestRFC7606TreatAsWithdrawLoggedAtDefaultLevel(t *testing.T) {
	log := captureSessionLog(t, defaultSessionLogLevel)
	s := rfc7606DiagSession()
	_, action, err := s.enforceRFC7606(wireu.NewWireUpdate(malformedOriginUpdate(), 0))
	require.NoError(t, err)
	require.Equal(t, message.RFC7606ActionTreatAsWithdraw, action)

	lines := rfc7606ErrorLines(log.String(), " attr=1 ", " event=treat-as-withdraw ",
		" peer="+s.settings.Address.String()+" ")
	require.Lenf(t, lines, 1, "captured:\n%s", log.String())
}

// steppedClock is a clock whose Now the test sets.
type steppedClock struct {
	clock.RealClock
	now time.Time
}

func (c *steppedClock) Now() time.Time { return c.now }

// TestRFC7606ErrorLogRateLimited floods one attribute code from one peer and
// reads what reaches the log. Method: an injected clock, so the window is a
// value and not a sleep. Each attribute code has its own window, and the first
// record after a window reports the count it replaced.
// PREVENTS: a hostile peer turning malformed UPDATEs into a log flood.
func TestRFC7606ErrorLogRateLimited(t *testing.T) {
	log := captureSessionLog(t, defaultSessionLogLevel)
	s := rfc7606DiagSession()
	c := &steppedClock{now: time.Unix(1_000_000, 0)}
	s.SetClock(c)

	for range 500 {
		s.logRFC7606Error("attribute-discard", 40, "flood")
	}
	require.Len(t, rfc7606ErrorLines(log.String(), " attr=40 "), 1, "a burst logs once")
	assert.Len(t, rfc7606ErrorLines(log.String(), " attr=40 ", " suppressed=0"), 1)

	// Another attribute code is not muted by the first one's flood.
	s.logRFC7606Error("treat-as-withdraw", 1, "other")
	assert.Len(t, rfc7606ErrorLines(log.String(), " attr=1 "), 1)

	// Suppressed calls cost nothing, so a flood buys the peer no work.
	allocs := testing.AllocsPerRun(50, func() {
		s.logRFC7606Error("attribute-discard", 40, "flood")
	})
	assert.Zero(t, allocs, "a suppressed record must allocate nothing")

	// The window reopens and the record carries the count it replaced:
	// 499 from the loop, 50 from AllocsPerRun plus its warm-up call.
	c.now = c.now.Add(rfc7606ErrorLogInterval)
	s.logRFC7606Error("attribute-discard", 40, "flood")
	assert.Len(t, rfc7606ErrorLines(log.String(), " attr=40 ", " suppressed=550"), 1,
		"captured:\n%s", log.String())
}
