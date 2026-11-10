// Design: docs/architecture/testing/interop.md -- an ICMP error on a Child SA, delivered or dropped by its payload header.
// Related: icmperror.go -- the step observation, its judgement and the reqid reader.
package ipsec

import (
	"strings"
	"testing"
)

// TestICMPErrorJudgeNeedsAnErrorThatRodeTheSA refuses every step in which Ze was
// never offered an error over the Child SA, whichever delivery the caller wants,
// because "not delivered" is also what a step that sent nothing reads. It then
// requires both delivery observations for the consistent error and neither for the
// inconsistent one.
func TestICMPErrorJudgeNeedsAnErrorThatRodeTheSA(t *testing.T) {
	delivered := icmpErrorObservation{generated: 3, carried: 300, delivered: 3, printed: true}
	dropped := icmpErrorObservation{generated: 3, carried: 300}
	for _, test := range []struct {
		name          string
		observation   icmpErrorObservation
		wantDelivered bool
		valid         bool
	}{
		{"consistent delivered", delivered, true, true},
		{"inconsistent dropped", dropped, false, true},
		{"consistent dropped", dropped, true, false},
		{"inconsistent delivered", delivered, false, false},
		{"counter moved, ping printed nothing", icmpErrorObservation{generated: 3, carried: 300, delivered: 3}, true, false},
		{"ping printed, counter still", icmpErrorObservation{generated: 3, carried: 300, printed: true}, false, false},
		{"no error generated", icmpErrorObservation{carried: 300}, false, false},
		{"error left in the clear", icmpErrorObservation{generated: 3}, false, false},
		{"error left in the clear, delivered", icmpErrorObservation{generated: 3, delivered: 3, printed: true}, true, false},
	} {
		t.Run(test.name, func(t *testing.T) {
			err := test.observation.judge(test.wantDelivered)
			if (err == nil) != test.valid {
				t.Fatalf("verdict %v, want valid=%t", err, test.valid)
			}
		})
	}
}

// TestReqidOfReadsOneSA reads the reqid ip(8) prints and refuses a listing with none,
// with two different ones, or with one that is not a 32-bit number.
func TestReqidOfReadsOneSA(t *testing.T) {
	state := "src 172.28.0.3 dst 172.28.0.2\n\tproto esp spi 0xc1234567 reqid 7 mode tunnel\n\treplay-window 0 flag af-unspec\n"
	got, err := reqidOf(state)
	if err != nil {
		t.Fatal(err)
	}
	if got != "7" {
		t.Fatalf("reqid %q, want 7", got)
	}
	if got, err := reqidOf(state + state); err != nil || got != "7" {
		t.Fatalf("two states with one reqid: %q %v", got, err)
	}
	for name, listing := range map[string]string{
		"none":      "src 172.28.0.3 dst 172.28.0.2\n\tproto esp spi 0xc1234567 mode tunnel\n",
		"two":       state + strings.Replace(state, "reqid 7", "reqid 8", 1),
		"too large": strings.Replace(state, "reqid 7", "reqid 4294967296", 1),
	} {
		if got, err := reqidOf(listing); err == nil {
			t.Fatalf("%s: read reqid %q, want a refusal", name, got)
		}
	}
}

// TestPingPrintedErrorFromNamesTheSource accepts an iputils error line from the
// error source alone: a reply, an error from another router and a summary naming the
// address are not it.
func TestPingPrintedErrorFromNamesTheSource(t *testing.T) {
	printed := "PING 10.1.0.77 (10.1.0.77) from 10.2.0.1 : 56(84) bytes of data.\n" +
		"From 10.9.9.9 icmp_seq=1 Time to live exceeded\n"
	if !pingPrintedErrorFrom(printed, icmpErrorSource) {
		t.Fatal("missed the error line from the source")
	}
	for name, output := range map[string]string{
		"silent":       "PING 10.7.0.5 (10.7.0.5) from 10.2.0.1 : 56(84) bytes of data.\n\n--- 10.7.0.5 ping statistics ---\n3 packets transmitted, 0 received, 100% packet loss\n",
		"other router": "From 10.9.9.99 icmp_seq=1 Time to live exceeded\n",
		"reply":        "64 bytes from 10.9.9.9: icmp_seq=1 ttl=64 time=0.1 ms\n",
	} {
		if pingPrintedErrorFrom(output, icmpErrorSource) {
			t.Fatalf("%s: read as an error from %s", name, icmpErrorSource)
		}
	}
}

// TestDirectedBytesSumsOneDirection sums every SA from strongSwan to Ze, including
// the two SPIs a rekey leaves, ignores the other direction, and refuses a map with no
// SA in the direction rather than answering zero.
func TestDirectedBytesSumsOneDirection(t *testing.T) {
	counters := map[saKey]uint64{
		{source: swanIP, target: zeIP, spi: "0x1"}: 100,
		{source: swanIP, target: zeIP, spi: "0x2"}: 20,
		{source: zeIP, target: swanIP, spi: "0x3"}: 999,
	}
	got, err := directedBytes(counters, swanIP, zeIP)
	if err != nil {
		t.Fatal(err)
	}
	if got != 120 {
		t.Fatalf("%d octets, want 120", got)
	}
	if got, err := directedBytes(map[saKey]uint64{{source: zeIP, target: swanIP, spi: "0x3"}: 999}, swanIP, zeIP); err == nil {
		t.Fatalf("no inbound SA read as %d octets", got)
	}
}

// TestICMPPingCommandExpiresAtTheFirstHop pins TTL 1 from Ze's inner address, which
// is what makes strongSwan, the first router, answer with a Time Exceeded.
func TestICMPPingCommandExpiresAtTheFirstHop(t *testing.T) {
	joined := strings.Join(icmpPingCommand(icmpConsistentTarget), " ")
	for _, want := range []string{"-t 1 ", "-I " + icmpZeInner + " ", " " + icmpConsistentTarget} {
		if !strings.Contains(joined, want) {
			t.Fatalf("%q lacks %q", joined, want)
		}
	}
}
