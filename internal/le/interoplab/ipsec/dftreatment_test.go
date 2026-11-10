// Design: docs/architecture/testing/interop.md -- the outer DF bit of a tunnel, read where strongSwan receives it.
// Related: dftreatment.go -- the counters, the judgement and the expected bit.
package ipsec

import (
	"slices"
	"strings"
	"testing"
)

// TestOuterDFFollowsTheTreatment pins the expected outer bit of RFC 4301 Section 8.1
// for each treatment and inner bit. Only the DF-set inner packet tells copy from
// clear, so that row is what makes the scenario discriminate; an unnamed treatment
// is refused rather than read as copy.
func TestOuterDFFollowsTheTreatment(t *testing.T) {
	for _, test := range []struct {
		treatment dfTreatment
		innerDF   bool
		want      bool
	}{
		{dfTreatmentClear, true, false},
		{dfTreatmentClear, false, false},
		{dfTreatmentCopy, true, true},
		{dfTreatmentCopy, false, false},
	} {
		got, err := test.treatment.outerDF(test.innerDF)
		if err != nil {
			t.Fatalf("treatment %d inner %t: %v", test.treatment, test.innerDF, err)
		}
		if got != test.want {
			t.Fatalf("treatment %d inner %t: outer DF %t, want %t", test.treatment, test.innerDF, got, test.want)
		}
	}
	if _, err := dfTreatmentUnspecified.outerDF(true); err == nil {
		t.Fatal("an unnamed treatment answered an outer bit")
	}
	if _, err := dfTreatmentUnspecified.leaf(); err == nil {
		t.Fatal("an unnamed treatment answered a df-bit leaf")
	}
	for treatment, want := range map[dfTreatment]string{dfTreatmentClear: "clear", dfTreatmentCopy: "copy"} {
		got, err := treatment.leaf()
		if err != nil {
			t.Fatal(err)
		}
		if got != want {
			t.Fatalf("treatment %d leaf %q, want %q", treatment, got, want)
		}
	}
}

// outerDFListing is an `nft list chain inet ze_outer_df input` answer in the shape
// nft prints it, with the counts the parser must read.
const outerDFListing = `table inet ze_outer_df {
	chain input {
		type filter hook input priority filter; policy accept;
		ip saddr 172.28.0.2 ip protocol esp ip frag-off & 0x4000 != 0 counter packets 7 bytes 980 comment "outer-df-set"
		ip saddr 172.28.0.2 ip protocol esp ip frag-off & 0x4000 == 0 counter packets 12 bytes 1680 comment "outer-df-clear"
	}
}
`

// TestParseOuterDFCounterReadsEachRuleByItsComment reads both counts of one listing,
// and refuses every listing where a count could only be guessed: no rule, a rule
// with no count, two rules with one comment. A zero read from a missing rule would
// satisfy the "none with the other bit" half of every step.
func TestParseOuterDFCounterReadsEachRuleByItsComment(t *testing.T) {
	for comment, want := range map[string]uint64{dfCounterSet: 7, dfCounterClear: 12} {
		got, err := parseOuterDFCounter(outerDFListing, comment)
		if err != nil {
			t.Fatal(err)
		}
		if got != want {
			t.Fatalf("%s: %d packets, want %d", comment, got, want)
		}
	}
	setLine := `ip frag-off & 0x4000 != 0 counter packets 7 bytes 980 comment "outer-df-set"`
	for name, listing := range map[string]string{
		"empty":         "",
		"other rule":    `counter packets 7 bytes 980 comment "outer-df-settled"`,
		"no count":      `ip frag-off & 0x4000 != 0 counter comment "outer-df-set"`,
		"word count":    `ip frag-off & 0x4000 != 0 counter packets many bytes 980 comment "outer-df-set"`,
		"two rules":     setLine + "\n" + setLine,
		"unquoted name": `counter packets 7 bytes 980 comment outer-df-set`,
	} {
		if got, err := parseOuterDFCounter(listing, dfCounterSet); err == nil {
			t.Fatalf("%s: read %d packets, want a refusal", name, got)
		}
	}
}

// TestJudgeOuterDFRequiresEveryPacketWithTheWantedBit accepts only the exact burst
// on the wanted counter and nothing on the other: one packet with the wrong bit, one
// missing, one extra, or a counter that went back is refused.
func TestJudgeOuterDFRequiresEveryPacketWithTheWantedBit(t *testing.T) {
	before := outerDFCount{set: 10, clear: 20}
	for _, test := range []struct {
		name    string
		after   outerDFCount
		wantSet bool
		valid   bool
	}{
		{"all set", outerDFCount{set: 13, clear: 20}, true, true},
		{"all clear", outerDFCount{set: 10, clear: 23}, false, true},
		{"set wanted, clear arrived", outerDFCount{set: 10, clear: 23}, true, false},
		{"clear wanted, set arrived", outerDFCount{set: 13, clear: 20}, false, false},
		{"one leaked", outerDFCount{set: 12, clear: 21}, true, false},
		{"one missing", outerDFCount{set: 12, clear: 20}, true, false},
		{"one extra", outerDFCount{set: 14, clear: 20}, true, false},
		{"nothing counted", before, false, false},
		{"set went back", outerDFCount{set: 9, clear: 23}, false, false},
		{"clear went back", outerDFCount{set: 13, clear: 19}, true, false},
	} {
		t.Run(test.name, func(t *testing.T) {
			err := judgeOuterDF(before, test.after, 3, test.wantSet)
			if (err == nil) != test.valid {
				t.Fatalf("verdict %v, want valid=%t", err, test.valid)
			}
		})
	}
}

// TestDFPingCommandSetsTheInnerBitItNames proves the two bursts differ in -M alone:
// do for an inner DF set, dont for clear, both from Ze's inner address.
func TestDFPingCommandSetsTheInnerBitItNames(t *testing.T) {
	for innerDF, mode := range map[bool]string{true: "do", false: "dont"} {
		command := dfPingCommand(innerDF)
		joined := strings.Join(command, " ")
		if !strings.Contains(joined, "-M "+mode+" ") {
			t.Fatalf("inner DF %t: %q lacks -M %s", innerDF, joined, mode)
		}
		if !strings.Contains(joined, "-I "+dfZeInner+" ") {
			t.Fatalf("inner DF %t: %q does not leave from %s", innerDF, joined, dfZeInner)
		}
		if command[len(command)-1] != dfSwanInner {
			t.Fatalf("inner DF %t: %q does not target %s", innerDF, joined, dfSwanInner)
		}
	}
	if slices.Equal(dfPingCommand(true), dfPingCommand(false)) {
		t.Fatal("the DF-set and DF-clear bursts are the same command")
	}
}

// TestSoleOutboundSPIWaitsForTheReplacement answers an SPI only when the listing
// holds exactly one ESP state and, after the reload, only when it is not the first
// phase's SPI.
func TestSoleOutboundSPIWaitsForTheReplacement(t *testing.T) {
	one := "src 172.28.0.2 dst 172.28.0.3\n\tproto esp spi 0xc0000001 reqid 1 mode tunnel\n"
	two := one + "src 172.28.0.2 dst 172.28.0.3\n\tproto esp spi 0xc0000002 reqid 2 mode tunnel\n"
	if spi, ok := soleOutboundSPI(one, ""); !ok || spi != "0xc0000001" {
		t.Fatalf("one state: %q %t", spi, ok)
	}
	if _, ok := soleOutboundSPI(one, "0xc0000001"); ok {
		t.Fatal("the first phase's SPI was taken as its replacement")
	}
	if _, ok := soleOutboundSPI(two, "0xc0000001"); ok {
		t.Fatal("a listing with the old and the new state was taken as the replacement alone")
	}
	if _, ok := soleOutboundSPI("", ""); ok {
		t.Fatal("an empty listing answered an SPI")
	}
	if !holdsESPSPI(two, "0xc0000002") {
		t.Fatal("holdsESPSPI missed a listed SPI")
	}
	if holdsESPSPI(two, "0xc0000003") {
		t.Fatal("holdsESPSPI found an SPI the listing does not hold")
	}
}

// TestRequireDataplaneDFReadsTheOutboundSA accepts the treatment on Ze's outbound SA
// and ignores the inbound one, whose df-bit the producer renders null (saInfoToMap,
// internal/component/ike/cmd/show_dataplane.go: the side that cannot carry an option
// renders it null). It refuses another value, the producer's unread shape
// (options-known false, every option null), a missing options-known, a null or
// missing df-bit on the outbound row, no outbound SA, and an answer that is not a
// list.
func TestRequireDataplaneDFReadsTheOutboundSA(t *testing.T) {
	const inbound = `{"spi":1,"src":"172.28.0.3","dst":"172.28.0.2","mode":"tunnel","options-known":true,"df-bit":null,"dscp-map":null,"icmp-error-check":true}`
	answer := func(outbound string) string {
		return `[` + inbound + `,{"spi":2,"src":"172.28.0.2","dst":"172.28.0.3","mode":"tunnel"` + outbound + `}]`
	}
	if err := requireDataplaneDF(answer(`,"options-known":true,"df-bit":"clear","dscp-map":"copy","icmp-error-check":null`), "clear"); err != nil {
		t.Fatal(err)
	}
	for name, outbound := range map[string]string{
		"other value":      `,"options-known":true,"df-bit":"copy","dscp-map":"copy","icmp-error-check":null`,
		"unread":           `,"options-known":false,"df-bit":null,"dscp-map":null,"icmp-error-check":null`,
		"no options-known": `,"df-bit":"clear"`,
		"null df-bit":      `,"options-known":true,"df-bit":null,"dscp-map":"copy","icmp-error-check":null`,
		"no df-bit":        `,"options-known":true`,
	} {
		if err := requireDataplaneDF(answer(outbound), "clear"); err == nil {
			t.Fatalf("%s: accepted %s", name, outbound)
		}
	}
	for name, whole := range map[string]string{
		"no outbound": `[` + inbound + `]`,
		"empty list":  `[]`,
		"table":       "SPI  SRC  DST",
	} {
		if err := requireDataplaneDF(whole, "clear"); err == nil {
			t.Fatalf("%s: accepted %s", name, whole)
		}
	}
}
