//go:build ze_bgp

// Design: docs/architecture/testing/interop.md -- atomic-policy oracle discrimination.
// Related: check_atomic_policy.go -- real FRR delivery remains a native lab run.
package bgp

import (
	"bytes"
	"encoding/binary"
	"encoding/hex"
	"net/netip"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ze-software/ze/internal/core/bgp/wire"
	"github.com/ze-software/ze/pkg/plugin/sdk"
)

// TestAtomicPolicyFRRRoute binds marker, MED and source identity to one valid
// FRR path. A nexthop metric or another path cannot satisfy the attribute proof.
func TestAtomicPolicyFRRRoute(t *testing.T) {
	const healthy = `{"prefix":"10.42.5.0/24","paths":[{"metric":88,"valid":true,"atomicAggregate":true,"aspath":{"string":"65004"},"peer":{"peerId":"172.30.0.2"}}]}`
	prefix := atomicPolicyPrefix(5, 24)
	peer := netip.MustParseAddr("172.30.0.2")
	if err := requireAtomicPolicyRoute(healthy, prefix, peer, true, 88); err != nil {
		t.Fatal(err)
	}
	t.Run("asdot-plus-source", func(t *testing.T) {
		text := strings.Replace(healthy, `"string":"65004"`, `"string":"0.65004"`, 1)
		if err := requireAtomicPolicyRoute(text, prefix, peer, true, 88); err != nil {
			t.Fatalf("equivalent AS notation refused: %v", err)
		}
	})
	for _, test := range []struct{ name, text string }{
		{"empty", ""}, {"null", "null"},
		{"wrong-prefix", strings.Replace(healthy, "10.42.5.0/24", "10.42.4.0/24", 1)},
		{"invalid-path", strings.Replace(healthy, `"valid":true`, `"valid":false`, 1)},
		{"missing-validity", strings.Replace(healthy, `"valid":true,`, "", 1)},
		{"lost-marker", strings.Replace(healthy, `"atomicAggregate":true`, `"atomicAggregate":false`, 1)},
		{"missing-marker", strings.Replace(healthy, `"atomicAggregate":true,`, "", 1)},
		{"old-sibling", strings.Replace(healthy, `"metric":88`, `"metric":77`, 1)},
		{"nexthop-metric", strings.Replace(healthy, `"metric":88`, `"nexthops":[{"metric":88}]`, 1)},
		{"wrong-peer", strings.Replace(healthy, "172.30.0.2", "172.30.0.9", 1)},
		{"missing-source", strings.Replace(healthy, `"string":"65004"`, `"string":""`, 1)},
		{"extra-as", strings.Replace(healthy, `"string":"65004"`, `"string":"65001 65004"`, 1)},
		{"wrong-source", strings.Replace(healthy, `"string":"65004"`, `"string":"65005"`, 1)},
		{"wrong-dotted-source", strings.Replace(healthy, `"string":"65004"`, `"string":"1.65004"`, 1)},
		{"two-paths", strings.Replace(healthy, `]}`, `,{"metric":88,"valid":true}]}`, 1)},
	} {
		t.Run(test.name, func(t *testing.T) {
			if err := requireAtomicPolicyRoute(test.text, prefix, peer, true, 88); err == nil {
				t.Fatal("faulty native route satisfied the atomic-policy oracle")
			}
		})
	}
	unmarked := strings.Replace(healthy, `"atomicAggregate":true,`, "", 1)
	if err := requireAtomicPolicyRoute(unmarked, prefix, peer, false, 88); err != nil {
		t.Fatalf("legitimate unmarked output refused: %v", err)
	}
	if err := requireAtomicPolicyRoute(healthy, prefix, peer, false, 88); err == nil {
		t.Fatal("an added marker satisfied the unmarked-output control")
	}
}

// TestAtomicPolicyFRRFinal makes every exclusion depend on the same complete
// native snapshot as all four positive routes, including the later source fence.
func TestAtomicPolicyFRRFinal(t *testing.T) {
	const healthy = `{"vrfId":0,"vrfName":"default","tableVersion":9,"routerId":"172.30.0.3","routes":{"10.42.3.0/25":[{}],"10.42.5.0/24":[{}],"10.42.254.0/24":[{}],"10.42.255.0/24":[{}]}}`
	if err := requireAtomicPolicyFinal(healthy); err != nil {
		t.Fatal(err)
	}
	for _, text := range []string{"", "null", "{}", `{"routes":{}}`,
		strings.Replace(healthy, `"vrfName":"default"`, `"vrfName":"other"`, 1),
		strings.Replace(healthy, `"tableVersion":9,`, "", 1)} {
		if err := requireAtomicPolicyFinal(text); err == nil {
			t.Fatal("an unmeasured query satisfied a native absence assertion")
		}
	}
	for _, prefix := range []string{"10.42.3.0/25", "10.42.5.0/24", "10.42.254.0/24", "10.42.255.0/24"} {
		t.Run("missing-"+prefix, func(t *testing.T) {
			text := strings.Replace(healthy, `"`+prefix+`":[{}]`, `"`+prefix+`":[]`, 1)
			if err := requireAtomicPolicyFinal(text); err == nil {
				t.Fatal("a missing positive route satisfied the final oracle")
			}
		})
	}
	for _, prefix := range []string{"10.42.1.0/24", "10.42.1.0/25", "10.42.2.0/24", "10.42.3.0/24", "10.42.4.0/24"} {
		t.Run("retained-"+prefix, func(t *testing.T) {
			text := strings.Replace(healthy, `"routes":{`, `"routes":{"`+prefix+`":[{}],`, 1)
			if err := requireAtomicPolicyFinal(text); err == nil {
				t.Fatal("a leaked or stale source route satisfied the final oracle")
			}
		})
	}
}

// TestAtomicPolicySourceRetention refuses export-induced source deletion,
// rewritten source keys, duplicate rows and a table from the wrong peer.
func TestAtomicPolicySourceRetention(t *testing.T) {
	const healthy = `{"adj-rib-in":{"172.30.0.9":[{"key":"ipv4 unicast:10.42.1.0/24"},{"key":"ipv4 unicast:10.42.2.0/24"},{"key":"ipv4 unicast:10.42.3.0/24"},{"key":"ipv4 unicast:10.42.4.0/24"},{"key":"ipv4 unicast:10.42.5.0/24"},{"key":"ipv4 unicast:10.42.254.0/24"},{"key":"ipv4 unicast:10.42.255.0/24"}]}}`
	source := netip.MustParseAddr("172.30.0.9")
	if err := requireAtomicPolicySource(healthy, source); err != nil {
		t.Fatal(err)
	}
	for _, test := range []struct{ name, text string }{
		{"empty", ""}, {"null", "null"}, {"no-table", "{}"},
		{"no-source", `{"adj-rib-in":{}}`},
		{"wrong-source", strings.Replace(healthy, "172.30.0.9", "172.30.0.8", 1)},
		{"deleted-A", strings.Replace(healthy, `{"key":"ipv4 unicast:10.42.1.0/24"},`, "", 1)},
		{"deleted-B", strings.Replace(healthy, `{"key":"ipv4 unicast:10.42.2.0/24"},`, "", 1)},
		{"deleted-D", strings.Replace(healthy, `{"key":"ipv4 unicast:10.42.4.0/24"},`, "", 1)},
		{"rewritten-source", strings.Replace(healthy, "10.42.3.0/24", "10.42.3.0/25", 1)},
		{"duplicate-E", strings.Replace(healthy, "10.42.4.0/24", "10.42.5.0/24", 1)},
		{"invalid-key", strings.Replace(healthy, "ipv4 unicast:10.42.1.0/24", "malformed", 1)},
	} {
		t.Run(test.name, func(t *testing.T) {
			if err := requireAtomicPolicySource(test.text, source); err == nil {
				t.Fatal("faulty retained source satisfied the oracle")
			}
		})
	}
}

// TestAtomicPolicySDKAnswers pins the intentional raw answers independently of
// the admission code they challenge, including the new MED on useful subset E.
func TestAtomicPolicySDKAnswers(t *testing.T) {
	for _, test := range []struct{ name, input, output string }{
		{"marked-attribute-edit",
			"0000001E4001010040020602010000FDEC400304AC1E00098004040000000A400600180A2A01",
			"0000001E4001010040020602010000FDEC400304AC1E00098004040000004D400600180A2A01"},
		{"marked-more-specific",
			"0000001E4001010040020602010000FDEC400304AC1E000980040400000014400600180A2A01",
			"0000001E4001010040020602010000FDEC400304AC1E00098004040000004D400600190A2A0100"},
		{"marked-marker-strip",
			"0000001E4001010040020602010000FDEC400304AC1E00098004040000001E400600180A2A02",
			"0000001B4001010040020602010000FDEC400304AC1E00098004040000004D180A2A02"},
		{"unmarked-more-specific",
			"0000001B4001010040020602010000FDEC400304AC1E000980040400000028180A2A03",
			"0000001B4001010040020602010000FDEC400304AC1E00098004040000004D190A2A0300"},
		{"marked-subset",
			"0000001E4001010140020602010000FDEC400304AC1E00098004040000003C400600180A2A04180A2A05",
			"0000001E4001010140020602010000FDEC400304AC1E000980040400000058400600180A2A05"},
	} {
		t.Run(test.name, func(t *testing.T) {
			input, err := hex.DecodeString(test.input)
			if err != nil {
				t.Fatal(err)
			}
			want, err := hex.DecodeString(test.output)
			if err != nil {
				t.Fatal(err)
			}
			answer, err := atomicPolicyAnswer(&sdk.FilterUpdateInput{Direction: "export", Filter: "rewrite", Raw: input})
			if err != nil {
				t.Fatal(err)
			}
			if answer.Action != sdk.FilterModify {
				t.Fatalf("action=%s, want modify", answer.Action)
			}
			if !bytes.Equal(answer.Raw, want) {
				t.Fatalf("raw=%X, want %X", answer.Raw, want)
			}
			clear(input)
			if !bytes.Equal(answer.Raw, want) {
				t.Fatal("SDK output aliases its borrowed input")
			}
		})
	}
	answer, err := atomicPolicyAnswer(&sdk.FilterUpdateInput{Direction: "export", Filter: "rewrite", Raw: []byte{0, 0, 0, 0}})
	if err != nil {
		t.Fatal(err)
	}
	if answer.Action != sdk.FilterAccept {
		t.Fatal("source EOR was not accepted unchanged")
	}
	if answer.Raw != nil {
		t.Fatal("source EOR acquired a synthetic rewrite")
	}
	for _, input := range []sdk.FilterUpdateInput{
		{Direction: "import", Filter: "rewrite", Raw: []byte{0, 0, 0, 0}},
		{Direction: "export", Filter: "unknown", Raw: []byte{0, 0, 0, 0}},
		{Direction: "export", Filter: "rewrite", Raw: []byte{0, 0, 0}},
		{Direction: "export", Filter: "rewrite", Raw: make([]byte, 4097)},
	} {
		if _, err := atomicPolicyAnswer(&input); err == nil {
			t.Fatal("invalid fixture input acquired a successful answer")
		}
	}
}

// TestAtomicPolicySDKSubsetRecovery keeps the same exact D/E policy when the
// RIB replays original candidates separately. A denial is a policy decision,
// never a plugin failure that forces recovery to abandon the session.
func TestAtomicPolicySDKSubsetRecovery(t *testing.T) {
	const attrs = "0000001E4001010140020602010000FDEC400304AC1E00098004040000003C400600"
	const kept = "0000001E4001010140020602010000FDEC400304AC1E000980040400000058400600180A2A05"
	for _, test := range []struct {
		name   string
		nlri   string
		action sdk.FilterAction
		output string
	}{
		{"bundled-originals", "180A2A04180A2A05", sdk.FilterModify, kept},
		{"excluded-original", "180A2A04", sdk.FilterReject, ""},
		{"retained-original", "180A2A05", sdk.FilterModify, kept},
	} {
		t.Run(test.name, func(t *testing.T) {
			input, err := hex.DecodeString(attrs + test.nlri)
			if err != nil {
				t.Fatal(err)
			}
			original := bytes.Clone(input)
			want, err := hex.DecodeString(test.output)
			if err != nil {
				t.Fatal(err)
			}
			answer, err := atomicPolicyAnswer(&sdk.FilterUpdateInput{Direction: "export", Filter: "rewrite", Raw: input})
			if err != nil {
				t.Fatalf("original candidate did not receive a policy decision: %v", err)
			}
			if answer.Action != test.action {
				t.Fatalf("action=%s, want %s", answer.Action, test.action)
			}
			if !bytes.Equal(input, original) {
				t.Fatal("policy modified the borrowed original candidate")
			}
			if !bytes.Equal(answer.Raw, want) {
				t.Fatalf("raw=%X, want %X", answer.Raw, want)
			}
			if test.action == sdk.FilterReject {
				if answer.Raw != nil {
					t.Fatal("excluded original acquired a synthetic raw answer")
				}
			}
			clear(input)
			if !bytes.Equal(answer.Raw, want) {
				t.Fatal("recovery policy answer aliases its borrowed candidate")
			}
		})
	}
	for _, nlri := range []string{
		"180A2A06",         // Unknown singleton.
		"180A2A04180A2A06", // Unknown bundled sibling.
		"190A2A0500",       // A more-specific identity is not original E.
		"180A2A04180A2A04", // A duplicate D does not supply E.
		"180A2A05180A2A04", // The source fixture never sent reversed originals.
	} {
		input, err := hex.DecodeString(attrs + nlri)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := atomicPolicyAnswer(&sdk.FilterUpdateInput{Direction: "export", Filter: "rewrite", Raw: input}); err == nil {
			t.Fatalf("unknown recovery population %s acquired a policy answer", nlri)
		}
	}
}

// TestAtomicPolicyFixtureFrames verifies every actual injector frame is complete
// and reaches a defined SDK phase. This prevents a framing refusal from posing
// as a marked-policy refusal in the real daemon scenario.
func TestAtomicPolicyFixtureFrames(t *testing.T) {
	path := filepath.Join("..", "..", "..", "..", filepath.FromSlash(scenarioDirectory), atomicPolicyScenario, "inject.msg")
	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	frames, eors := 0, 0
	for line := range strings.SplitSeq(string(content), "\n") {
		if !strings.HasPrefix(line, "action=send:") {
			continue
		}
		_, encoded, found := strings.Cut(line, ":hex=")
		if !found {
			t.Fatal("injector send has no hex body")
		}
		frame, err := hex.DecodeString(encoded)
		if err != nil {
			t.Fatal(err)
		}
		if len(frame) < 19 {
			t.Fatal("injector frame is shorter than the BGP header")
		}
		if int(binary.BigEndian.Uint16(frame[16:18])) != len(frame) {
			t.Fatal("injector frame's declared length differs from its bytes")
		}
		if !bytes.Equal(frame[:16], bytes.Repeat([]byte{0xff}, 16)) {
			t.Fatal("injector frame marker is not all ones")
		}
		if frame[18] != 2 {
			t.Fatal("atomic fixture send is not an UPDATE")
		}
		body := frame[19:]
		sections, err := wire.ParseUpdateSections(body)
		if err != nil {
			t.Fatal(err)
		}
		if len(sections.NLRI(body)) == 0 {
			eors++
		}
		if _, err := atomicPolicyAnswer(&sdk.FilterUpdateInput{Direction: "export", Filter: "rewrite", Raw: body}); err != nil {
			t.Fatal(err)
		}
		frames++
	}
	if frames != 11 {
		t.Fatalf("injector frames=%d, want 11", frames)
	}
	if eors != 2 {
		t.Fatalf("injector EORs=%d, want 2", eors)
	}
	for _, token := range []string{"200AFFE901", "200AFFE902"} {
		if strings.Count(string(content), "contains="+token) != 1 {
			t.Fatalf("source readiness token %s is not unique", token)
		}
	}
}
