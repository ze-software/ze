package peer

import (
	"bytes"
	"context"
	"net"
	"strings"
	"testing"
	"time"
)

// TestScopedUpdateFields distinguishes reachability from withdrawals and keeps
// the NLRI bytes intact for mixed messages, multiprotocol and ADD-PATH frames.
func TestScopedUpdateFields(t *testing.T) {
	for _, tc := range []struct {
		name      string
		body      string
		needle    string
		announced bool
		withdrawn bool
	}{
		{"legacy announcement", "00000000180A0000", "180A0000", true, false},
		{"legacy withdrawal", "0004180A00000000", "180A0000", false, true},
		{"mixed announcement", "000418C633640000180A0000", "180A0000", true, false},
		{"mixed withdrawal", "000418C633640000180A0000", "18C63364", false, true},
		{"same prefix both fields", "0004180A00000000180A0000", "180A0000", true, true},
		{"attribute bytes are not NLRI", "00000007C0FA04180A0000", "180A0000", false, false},
		{"mp reach", "00000010800E0D000101040101010100180A0000", "180A0000", true, false},
		{"mp unreach", "0000000A800F07000101180A0000", "180A0000", false, true},
		{"extended mp reach", "00000011900E000D000101040101010100180A0000", "180A0000", true, false},
		{"next hop is not NLRI", "0000000C800E0900010104180A000000", "180A0000", false, false},
		{"legacy add path announce", "000000000000002A180A0000", "0000002A180A0000", true, false},
		{"legacy add path withdraw", "00080000002A180A00000000", "0000002A180A0000", false, true},
		{"mp add path announce", "00000014800E110001010401010101000000002A180A0000", "0000002A180A0000", true, false},
		{"mp add path withdraw", "0000000E800F0B0001010000002A180A0000", "0000002A180A0000", false, true},
		{"wrong add path identity", "000000000000002B180A0000", "0000002A180A0000", false, false},
		{"EOR", "00000000", "180A0000", false, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			stream := wireMessage(t, tc.body).Stream()
			for _, check := range []struct {
				field string
				want  bool
			}{{"announced", tc.announced}, {"withdrawn", tc.withdrawn}} {
				rule := check.field + ":" + tc.needle
				if got := matchRule(rule, stream); got != check.want {
					t.Errorf("positive %s = %v, want %v", rule, got, check.want)
				}
				if got := rejectedUpdateField(rule, stream); got != check.want {
					t.Errorf("negative %s = %v, want %v", rule, got, check.want)
				}
			}
		})
	}
}

// TestScopedUpdateFieldsMalformed fails unreadable UPDATEs closed rather than
// letting a truncated length hide a forbidden announcement from a rejection.
func TestScopedUpdateFieldsMalformed(t *testing.T) {
	for _, body := range []string{
		"", "0000", "FFFF0000", "0000FFFF", "0000000180",
		"00000003900E00", "00000003800EFF", "00000004800E0100",
		"00000008800E05000101FF00", "00000005800F020001",
	} {
		t.Run(body, func(t *testing.T) {
			stream := wireMessage(t, body).Stream()
			if matchRule("announced:180A0000", stream) {
				t.Fatal("malformed UPDATE satisfied a positive assertion")
			}
			if !rejectedUpdateField("announced:180A0000", stream) {
				t.Fatal("malformed UPDATE escaped a scoped rejection")
			}
		})
	}
}

// TestScopedUpdateRuleParsing proves the native peer and runner share scope
// validation, and positive rules reach the same field matcher as rejections.
func TestScopedUpdateRuleParsing(t *testing.T) {
	for _, field := range []string{"announced", "withdrawn"} {
		conn, pattern, reject, err := ParseRejectRule("reject=bgp:conn=2:scope=" + field + ":pattern=180a0000")
		if err != nil || !reject || conn != 2 || pattern != field+":180A0000" {
			t.Fatalf("scope %s: conn=%d pattern=%q reject=%v err=%v", field, conn, pattern, reject, err)
		}
		conn, seq, pattern, err := parseExpectRule("expect=bgp:conn=2:seq=3:" + field + "=180a0000")
		if err != nil || conn != 2 || seq != 3 || pattern != field+":180A0000" {
			t.Fatalf("positive %s: conn=%d seq=%d pattern=%q err=%v", field, conn, seq, pattern, err)
		}
	}
	for _, rule := range []string{
		"reject=bgp:conn=1:scope=typo:pattern=180A0000",
		"reject=bgp:conn=1:scope=announced:pattern=nothex",
	} {
		if _, _, _, err := ParseRejectRule(rule); err == nil {
			t.Fatalf("invalid rule accepted: %s", rule)
		}
	}
	c, err := newChecker([]string{
		"expect=bgp:conn=1:seq=1:withdrawn=180A0000",
		"reject=bgp:conn=1:scope=announced:pattern=180A0000",
	})
	if err != nil {
		t.Fatal(err)
	}
	c.Init()
	withdrawal := wireMessage(t, "0004180A00000000")
	if _, rejected := c.rejection(withdrawal); rejected {
		t.Fatal("withdrawal mistaken for an announcement")
	}
	if !c.Expected(withdrawal) {
		t.Fatal("positive withdrawal rule was not consumed")
	}
}

// TestScopedChecksAllowPackedWithdrawals asserts both destinations whether the
// sender packs their withdrawals or writes one UPDATE per destination.
func TestScopedChecksAllowPackedWithdrawals(t *testing.T) {
	for _, bodies := range [][]string{
		{"000818C63364180A00000000"},
		{"000418C633640000", "0004180A00000000"},
	} {
		c, err := newChecker([]string{
			"expect=bgp:conn=1:seq=1:withdrawn=18C63364",
			"expect=bgp:conn=1:seq=1:withdrawn=180A0000",
			"expect=bgp:conn=1:seq=2:announced=18C00002",
		})
		if err != nil {
			t.Fatal(err)
		}
		c.Init()
		for _, body := range bodies {
			if !c.Expected(wireMessage(t, body)) {
				t.Fatalf("withdrawal did not consume a scoped expectation: %s", body)
			}
		}
		if !c.Expected(wireMessage(t, "0000000018C00002")) {
			t.Fatal("fence announcement did not consume the next sequence")
		}
	}
}

// TestScopedAnnouncementRejectRunsOnInboundFrames exercises the consuming
// message loop, not just the matcher: a permitted withdrawal is followed by an
// otherwise-expected announcement that only the negative rule can refuse.
// MUTATION: removing runMessageLoop's p.rejected call makes the announcement
// satisfy its positive expectation and returns success instead of rejection.
func TestScopedAnnouncementRejectRunsOnInboundFrames(t *testing.T) {
	testScopedAnnouncementInbound(t, false)
}

// TestScopedAnnouncementRejectRunsWhileLingering exercises the second inbound
// consumer after the withdrawal has completed every positive expectation.
// MUTATION: removing holdSession's p.rejected call loses RejectionMarker, so an
// already-published peer success would no longer be retracted by the runner.
func TestScopedAnnouncementRejectRunsWhileLingering(t *testing.T) {
	testScopedAnnouncementInbound(t, true)
}

func testScopedAnnouncementInbound(t *testing.T, linger bool) {
	t.Helper()
	for _, tc := range []struct {
		name      string
		withdrawn string
		announced string
		needle    string
	}{
		{"legacy", "0004180A00000000", "00000000180A0000", "180A0000"},
		{"mixed legacy", "0004180A00000000", "000418C633640000180A0000", "180A0000"},
		{"MP", "0000000A800F07000101180A0000", "00000010800E0D000101040101010100180A0000", "180A0000"},
		{"legacy withdrawal with MP announcement", "0004180A00000000", "000418C633640010800E0D000101040101010100180A0000", "180A0000"},
		{"MP withdrawal with legacy announcement", "0000000A800F07000101180A0000", "0000000A800F0700010118C63364180A0000", "180A0000"},
		{"ADD-PATH", "00080000002A180A00000000", "000000000000002A180A0000", "0000002A180A0000"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			rules := []string{
				"expect=bgp:conn=1:seq=1:withdrawn=" + tc.needle,
				"reject=bgp:conn=1:scope=announced:pattern=" + tc.needle,
			}
			if !linger {
				// Deliberately accept the forbidden frame positively: without the
				// rejection consumer this run must succeed, not mismatch or time out.
				rules = append(rules, "expect=bgp:conn=1:seq=2:announced="+tc.needle)
			}
			var output bytes.Buffer
			p, err := New(&Config{
				Mode:   ModeCheck,
				Expect: rules,
				Silent: true,
				Linger: linger,
				Output: &output,
			})
			if err != nil {
				t.Fatal(err)
			}
			p.checker.Init()
			first := wireMessage(t, tc.withdrawn)
			second := wireMessage(t, tc.announced)
			ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
			defer cancel()
			local, remote := net.Pipe()
			defer local.Close()  //nolint:errcheck // Test-owned pipe teardown cannot change the observed verdict.
			defer remote.Close() //nolint:errcheck // Unblocks the writer on every test exit.
			if err := remote.SetDeadline(time.Now().Add(5 * time.Second)); err != nil {
				t.Fatal(err)
			}
			written := make(chan error, 1)
			go func() {
				defer remote.Close() //nolint:errcheck // Signals EOF after this bounded two-frame stream.
				for _, frame := range []*Message{first, second} {
					if _, err := remote.Write(frame.Header); err != nil {
						written <- err
						return
					}
					if _, err := remote.Write(frame.Body); err != nil {
						written <- err
						return
					}
				}
				written <- nil
			}()
			result := p.runMessageLoop(ctx, local, 0)
			if err := <-written; err != nil {
				t.Fatalf("consumer did not read both inbound frames: %v", err)
			}
			if result.Success {
				t.Fatal("inbound forbidden announcement reported success")
			}
			if result.Error == nil {
				t.Fatal("inbound rejection returned no error")
			}
			if !strings.Contains(result.Error.Error(), "forbids") {
				t.Fatalf("failure was not the scoped rejection: %v", result.Error)
			}
			if !strings.Contains(output.String(), RejectionMarker) {
				t.Fatalf("runner-visible rejection marker missing: %s", output.String())
			}
			if linger && !strings.Contains(output.String(), "successful") {
				t.Fatal("test never reached the post-success linger consumer")
			}
		})
	}
}

// TestScopedAttributeConjunction requires a route and a value from the named
// attribute in the same UPDATE, through the checker's expectation consumer.
func TestScopedAttributeConjunction(t *testing.T) {
	for _, tc := range []struct {
		name string
		body string
		want bool
	}{
		{"both match", "00000007C00804FFFF0006180A0000", true},
		{"extended length", "00000008D0080004FFFF0006180A0000", true},
		{"wrong route", "00000007C00804FFFF000618C00002", false},
		{"wrong community", "00000007C00804FFFF0007180A0000", false},
		{"wrong attribute code", "00000007C0FA04FFFF0006180A0000", false},
		{"attribute absent", "00000000180A0000FFFF0006", false},
		{"MP announcement", "00000017C00804FFFF0006800E0D000101040101010100180A0000", true},
		{"withdrawal only", "0004180A00000007C00804FFFF0006", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			c, err := newChecker([]string{
				"expect=bgp:conn=1:seq=1:announced=180A0000:attribute=8,FFFF0006",
			})
			if err != nil {
				t.Fatal(err)
			}
			c.Init()
			if got := c.Expected(wireMessage(t, tc.body)); got != tc.want {
				t.Fatalf("conjunction consumed = %v, want %v", got, tc.want)
			}
		})
	}
}

// TestScopedAttributeConjunctionNeverSpansUpdates rejects a value on one route
// and the requested route on another, while allowing two matching packed routes.
func TestScopedAttributeConjunctionNeverSpansUpdates(t *testing.T) {
	c, err := newChecker([]string{
		"expect=bgp:conn=1:seq=1:announced=180A0000:attribute=8,FFFF0006",
		"expect=bgp:conn=1:seq=1:announced=180A0002:attribute=8,FFFF0006",
	})
	if err != nil {
		t.Fatal(err)
	}
	c.Init()
	for _, body := range []string{
		"00000007C00804FFFF000618C00002",
		"00000000180A0000180A0002",
	} {
		if c.Expected(wireMessage(t, body)) {
			t.Fatal("different UPDATEs supplied the two halves of a conjunction")
		}
	}
	if !c.Expected(wireMessage(t, "00000007C00804FFFF0006180A0000180A0002")) {
		t.Fatal("packed matching routes were not consumed")
	}
	if !c.Completed() {
		t.Fatal("packed matching routes did not satisfy both conjunctions")
	}
}

// TestScopedAttributeConjunctionRejectsInvalidRules prevents an ignored or
// malformed condition from silently reducing a route-and-attribute assertion.
func TestScopedAttributeConjunctionRejectsInvalidRules(t *testing.T) {
	for _, suffix := range []string{
		"announced=180A0000:attribute=",
		"announced=180A0000:attribute=8",
		"announced=180A0000:attribute=0,FFFF0006",
		"announced=180A0000:attribute=256,FFFF0006",
		"announced=180A0000:attribute=8,",
		"announced=180A0000:attribute=8,FFF",
		"announced=180A0000:attribute=8,ZZZZ",
		"contains=180A0000:attribute=8,FFFF0006",
		"announced=180A0000:contains=FFFF0006:attribute=8,FFFF0006",
	} {
		if _, _, _, err := parseExpectRule("expect=bgp:conn=1:seq=1:" + suffix); err == nil {
			t.Fatalf("invalid conjunction accepted: %s", suffix)
		}
	}
}

// TestScopedAttributeConjunctionRejectsAmbiguousFields prevents one of two
// supplied NLRI conditions from disappearing behind parser precedence.
func TestScopedAttributeConjunctionRejectsAmbiguousFields(t *testing.T) {
	_, _, _, err := parseExpectRule(
		"expect=bgp:conn=1:seq=1:announced=180A0000:withdrawn=180A0002:attribute=8,FFFF0006",
	)
	if err == nil {
		t.Fatal("ambiguous attribute conjunction was accepted")
	}
}
