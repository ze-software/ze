// Design: docs/architecture/testing/interop.md -- FlowSpec predicate boundaries.
package bgp

import (
	"encoding/hex"
	"encoding/json"
	"strings"
	"testing"
)

const flowSpecBaselineJSON = `"[destination: 10.99.0.0/24]":[{"nlri":{"value":[{"type":1,"value":{"prefix":"10.99.0.0/24"}}]},"best":true,"neighbor-ip":"172.30.0.10","attrs":[{"type":16,"value":[{"type":128,"subtype":6,"as":0,"rate":0}]}]}]`
const flowSpecTargetJSON = `"[destination: 10.99.77.0/24]":[{"nlri":{"value":[{"type":1,"value":{"prefix":"10.99.77.0/24"}}]},"best":true,"neighbor-ip":"172.30.0.10","attrs":[{"type":16,"value":[{"type":128,"subtype":6,"as":0,"rate":9600}]}]}]`
const flowSpecTargetBody = "000000264001010040020602010000fdecc010088006000046160000800e0b00018500000501180a634d"
const flowSpecWithdrawBody = "0000000c800f090001850501180a634d"

// flowSpecCheckerBranches supplies TestBespokeCheckerBranches with both-polarity
// foreign-decoder, wire-normalization, continuous-session and replay-fence cases.
func flowSpecCheckerBranches(t *testing.T) {
	t.Run("foreign rule and action", TestFlowSpecForeignEvidence)
	t.Run("wire and replay fences", TestFlowSpecWireEvidence)
	t.Run("cover route", TestFlowSpecForeignCover)
	t.Run("no source readvertisement", TestFlowSpecSourceComplete)
}

// TestFlowSpecForeignEvidence pins GoBGP's actual v3.31.0 JSON schema. A rate on
// another path, stale/unselected route, failed query or absent AS0/rate0 field
// cannot certify either usable state or unauthorized absence.
func TestFlowSpecForeignEvidence(t *testing.T) {
	positive := "{" + flowSpecBaselineJSON + "," + flowSpecTargetJSON + "}"
	if err := flowSpecForeignEvidence(positive, "172.30.0.10", true); err != nil {
		t.Fatal(err)
	}
	absent := "{" + flowSpecBaselineJSON + "}"
	if err := flowSpecForeignEvidence(absent, "172.30.0.10", false); err != nil {
		t.Fatal(err)
	}
	for name, output := range map[string]string{
		"empty query":           "",
		"null query":            "null",
		"empty table":           "{}",
		"wrong JSON shape":      "[]",
		"no baseline":           "{" + flowSpecTargetJSON + "}",
		"missing target":        absent,
		"wrong rate":            strings.Replace(positive, `"rate":9600`, `"rate":9601`, 1),
		"missing rate":          strings.Replace(positive, `,"rate":9600`, "", 1),
		"missing zero":          strings.ReplaceAll(positive, `,"rate":0`, ""),
		"missing AS zero":       strings.ReplaceAll(positive, `"as":0,`, ""),
		"wrong action":          strings.ReplaceAll(positive, `"subtype":6`, `"subtype":7`),
		"nontransitive action":  strings.ReplaceAll(positive, `"type":128`, `"type":192`),
		"unselected":            strings.ReplaceAll(positive, `"best":true`, `"best":false`),
		"stale":                 "{" + flowSpecBaselineJSON + "," + strings.Replace(flowSpecTargetJSON, `"best":true`, `"best":true,"stale":true`, 1) + "}",
		"withdrawn":             "{" + flowSpecBaselineJSON + "," + strings.Replace(flowSpecTargetJSON, `"best":true`, `"best":true,"withdrawal":true`, 1) + "}",
		"wrong peer":            strings.ReplaceAll(positive, "172.30.0.10", "172.30.0.11"),
		"wrong decoded prefix":  strings.Replace(positive, `"prefix":"10.99.77.0/24"`, `"prefix":"10.99.78.0/24"`, 1),
		"rate only on baseline": "{" + strings.Replace(flowSpecBaselineJSON, `"rate":0`, `"rate":9600`, 1) + "," + strings.Replace(flowSpecTargetJSON, `"rate":9600`, `"rate":0`, 1) + "}",
	} {
		t.Run(name, func(t *testing.T) {
			if err := flowSpecForeignEvidence(output, "172.30.0.10", true); err == nil {
				t.Fatal("invalid foreign evidence passed")
			}
		})
	}
	for _, output := range []string{
		positive, "{}", "null", "", `{"error":"no routes"}`,
		strings.Replace(positive, "[destination: 10.99.77.0/24]", "[destination: 10.99.78.0/24]", 1),
		"{" + flowSpecBaselineJSON + `,"[destination: 10.99.77.0/24]":[]}`,
	} {
		if err := flowSpecForeignEvidence(output, "172.30.0.10", false); err == nil {
			t.Fatalf("invalid absence evidence passed: %s", output)
		}
	}
	// Selected nondefault networks affect the peer identity, not the rule.
	remapped := strings.ReplaceAll(positive, "172.30.0.10", "10.254.77.10")
	if err := flowSpecForeignEvidence(remapped, "10.254.77.10", true); err != nil {
		t.Fatal(err)
	}
	if err := flowSpecForeignEvidence(remapped, "172.30.0.10", true); err == nil {
		t.Fatal("default-network peer identity passed a remapped query")
	}
}

// TestFlowSpecWireEvidence uses literal UPDATEs, not a production FlowSpec
// encoder. The parser must reject nonzero NH and lost action and keep the newest
// target event, including a withdrawal after an earlier correct announcement.
func TestFlowSpecWireEvidence(t *testing.T) {
	liveText := flowSpecTestCapture(t, flowSpecTargetBody)
	live, err := flowSpecWireEvidence(liveText, "172.30.0.2")
	if err != nil || !live.present || live.frame != 3 {
		t.Fatalf("live state=%+v, error=%v", live, err)
	}
	withdrawText := flowSpecTestCapture(t, flowSpecTargetBody, flowSpecWithdrawBody)
	withdraw, err := flowSpecWireEvidence(withdrawText, "172.30.0.2")
	if err != nil || withdraw.present || withdraw.frame <= live.frame {
		t.Fatalf("withdraw state=%+v, error=%v", withdraw, err)
	}
	replay, err := flowSpecWireEvidence(flowSpecTestCapture(t, flowSpecTargetBody, flowSpecWithdrawBody, flowSpecTargetBody), "172.30.0.2")
	if err != nil || !replay.present || replay.frame <= withdraw.frame {
		t.Fatalf("replay state=%+v, error=%v", replay, err)
	}
	// An unrelated NLRI after withdrawal must not resurrect the target.
	other := strings.Replace(flowSpecTargetBody, "0501180a634d", "0501180a634e", 1)
	latest, err := flowSpecWireEvidence(flowSpecTestCapture(t, flowSpecTargetBody, flowSpecWithdrawBody, other), "172.30.0.2")
	if err != nil || latest.present || latest.frame != withdraw.frame {
		t.Fatalf("unrelated route changed target state=%+v, error=%v", latest, err)
	}
	for name, body := range map[string]string{
		"received NH leaked":        "000000364001010040020602010000fdecc010088006000046160000800e1b0001851020010db800000000000000000000dead000501180a634d",
		"action omitted":            "0000001b4001010040020602010000fdec800e0b00018500000501180a634d",
		"action changed":            strings.Replace(flowSpecTargetBody, "46160000", "46170000", 1),
		"truncated attribute":       flowSpecTargetBody[:len(flowSpecTargetBody)-2],
		"truncated NLRI":            strings.Replace(flowSpecTargetBody, "0501180a634d", "0601180a634d", 1),
		"empty NLRI":                strings.Replace(flowSpecTargetBody, "0501180a634d", "0001180a634d", 1),
		"truncated extended length": "0000000b800e080001850000f00501",
	} {
		t.Run(name, func(t *testing.T) {
			if _, err := flowSpecWireEvidence(flowSpecTestCapture(t, body), "172.30.0.2"); err == nil {
				t.Fatal("bad wire evidence passed")
			}
		})
	}
	for name, text := range map[string]string{
		"empty":                "",
		"truncated JSON":       liveText[:len(liveText)-3],
		"trailing frame bytes": strings.Replace(liveText, `46160000`, `4616000000`, 1),
		"replaced session":     liveText + flowSpecTestCapture(t, flowSpecTargetBody),
		"rewritten update":     strings.Replace(liveText, `"original":"ffffffffffffffffffffffffffffffff003d`, `"delivered":"00","original":"ffffffffffffffffffffffffffffffff003d`, 1),
		"notification":         liveText + `{"original":"ffffffffffffffffffffffffffffffff0015030600"}`,
	} {
		t.Run(name, func(t *testing.T) {
			if _, err := flowSpecWireEvidence(text, "172.30.0.2"); err == nil {
				t.Fatal("invalid or noncontinuous capture passed")
			}
		})
	}
	if _, err := flowSpecWireEvidence(liveText, "172.31.77.2"); err == nil {
		t.Fatal("wrong selected-network router ID passed")
	}
}

func flowSpecTestCapture(t *testing.T, bodies ...string) string {
	t.Helper()
	var text strings.Builder
	// Literal OPEN with router ID .2, then its delivered capability6 variant.
	text.WriteString(`{"original":"ffffffffffffffffffffffffffffffff001d0104fde9005aac1e000200","delivered":"ffffffffffffffffffffffffffffffff00210104fde9005aac1e00020402020600"}` + "\n")
	text.WriteString(`{"original":"ffffffffffffffffffffffffffffffff001304"}` + "\n")
	encoder := json.NewEncoder(&text)
	for _, body := range bodies {
		data, err := hex.DecodeString(body)
		if err != nil {
			t.Fatal(err)
		}
		if err := encoder.Encode(extendedRelayFrame{Original: hex.EncodeToString(speakerMessage(bgpUpdate, data))}); err != nil {
			t.Fatal(err)
		}
	}
	return text.String()
}

// TestFlowSpecForeignCover refuses error documents and foreign paths with the
// right map key but no selected NLRI from the actual relay neighbor.
func TestFlowSpecForeignCover(t *testing.T) {
	const positive = `{"10.99.77.0/24":[{"nlri":{"prefix":"10.99.77.0/24"},"best":true,"neighbor-ip":"172.30.0.10"}]}`
	if err := flowSpecForeignCover(positive, "172.30.0.10"); err != nil {
		t.Fatal(err)
	}
	for _, value := range []string{
		"{}", "null", "", `{"error":"10.99.77.0/24"}`,
		strings.Replace(positive, `"best":true`, `"best":false`, 1),
		strings.Replace(positive, `"best":true`, `"best":true,"stale":true`, 1),
		strings.Replace(positive, `"prefix":"10.99.77.0/24"`, `"prefix":"10.99.78.0/24"`, 1),
		strings.Replace(positive, "172.30.0.10", "172.30.0.11", 1),
	} {
		if err := flowSpecForeignCover(value, "172.30.0.10"); err == nil {
			t.Fatalf("invalid cover evidence passed: %s", value)
		}
	}
}

// TestFlowSpecSourceComplete proves an extra source advertisement cannot stand
// in for stored replay, and a success token without actual writes cannot pass.
func TestFlowSpecSourceComplete(t *testing.T) {
	const sent = "sending 47 bytes to peer\nsending 77 bytes to peer\nsending 27 bytes to peer\nsending 47 bytes to peer\n"
	if err := flowSpecSourceComplete(sent + "successful\n"); err != nil {
		t.Fatal(err)
	}
	for _, logs := range []string{
		"", "successful\n", sent, "successful\n" + sent,
		sent + "sending 77 bytes to peer\nsuccessful\n",
		sent + "successful\nsending 77 bytes to peer\n",
		strings.Replace(sent, "sending 77", "sending 47", 1) + "successful\n",
	} {
		if err := flowSpecSourceComplete(logs); err == nil {
			t.Fatalf("invalid source sequence passed: %s", logs)
		}
	}
}
