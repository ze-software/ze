// Design: docs/architecture/testing/interop.md -- the ESP form fast path kept off by a firewall rule that drops ESP.
// Related: checkers.go -- checkESPFormChange, the same two disagreeing ESP forms with no firewall.
// Related: helpers.go -- directed XFRM byte counters, ping and the Ze CLI.
package ipsec

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/ze-software/ze/internal/core/textbuf"
	"github.com/ze-software/ze/internal/le/interoplab"
)

// The esp-form-firewall-drop scenario's firewall, as ze.conf writes it, and what Ze
// reports about it.
const (
	// espFirewallTable is the table's name in ze.conf, the one `show firewall ruleset`
	// takes; Ze programs it as espFirewallKernelTable, the name the fast path's
	// analyzer reads from the kernel and the doctor finding names.
	espFirewallTable       = "espdrop"
	espFirewallKernelTable = "ze_espdrop"
	// espFirewallTerm is the one term of the input chain: `protocol esp` then drop.
	// Ze writes the term name into the rule's comment, which the finding quotes.
	espFirewallTerm = "drop-esp"
	// espFirewallDoctorCode is the finding the fast path's doctor check answers while
	// an input-path rule keeps its gate shut (doctor_esp_form_fast_path.go).
	espFirewallDoctorCode = "doctor-ipsec-esp-form-firewall"
	// espFirewallPingCount echo requests each peer sends; every one strongSwan sends
	// must be dropped by the term.
	espFirewallPingCount = 3
	// espFirewallJSONDepthMax bounds the walk over a Ze JSON answer, which nests a few
	// levels; a deeper one is not an answer this checker reads.
	espFirewallJSONDepthMax = 16
	// espFirewallLossAll is the ping summary of a run whose every echo was lost.
	espFirewallLossAll = 100
)

// espFirewallControl is the direction that proves the tunnel is up while the rule
// drops strongSwan's ESP: Ze encrypts its echo requests, and strongSwan decrypts them.
// strongSwan's replies are bare ESP, which the rule drops, so this ping is lossy and
// only its counters are read.
var espFirewallControl = []espDirection{zeEncrypts, swanDecrypts}

// espFirewallJudged is the direction the scenario judges: strongSwan encrypts toward
// Ze, and Ze's inbound SA must not decrypt any of it.
var espFirewallJudged = []espDirection{swanEncrypts, zeDecrypts}

// checkESPFormFirewallDrop proves that the ESP form fast path never carries a peer's
// bare ESP past a firewall rule that drops ESP on the input path.
//
// strongSwan sends bare ESP on a Child SA whose inbound state at Ze carries the
// ESP-in-UDP template: charon floated IKE to UDP 4500 with no NAT (MOBIKE), and
// build_i (src/libcharon/sa/ikev2/tasks/child_create.c, strongSwan 6.1.0) sets
// child.encap from COND_NAT_ANY, which stays false, as install_child_sa does when it
// calls child_sa->update. Ze's fast path would add the UDP header at tc ingress, ahead
// of every netfilter hook, so ze.conf's `protocol esp` drop would see UDP and pass it.
// The fast path therefore keeps its gate shut while the kernel ruleset holds that
// rule, and the rule drops the bare ESP before XFRM or the complete path's raw ESP
// reader receives it.
//
// The checker waits for Ze's doctor finding (doctor-ipsec-esp-form-firewall naming
// ze_espdrop and the drop-esp term), so the gate is judged shut before any ESP is sent.
// A control ping from Ze proves the Child SA carries ESP toward strongSwan. Then
// strongSwan pings Ze, and each clause below is an observation the others cannot make:
//
//   - strongSwan's outbound SA advanced: ESP left toward Ze, so nothing below is the
//     absence of a stimulus;
//   - every echo request was lost: nothing was delivered;
//   - Ze's inbound SA did not move: XFRM decrypted none of it;
//   - the drop-esp term counted at least the echo requests: the rule judged them;
//   - Ze's XfrmInStateMismatch did not move: no bare ESP reached XFRM, so the complete
//     path never re-presented what the rule dropped.
//
// Red proof: force the gate open, by making writeGateLocked
// (internal/component/ike/dataplane/espform_fastpath_gate_linux.go) write
// espFormFastPathGateOpen whatever it is given, and let the harness rebuild the Ze
// image. The program then rewrites strongSwan's echo requests to ESP-in-UDP at tc
// ingress, they pass the rule as UDP, Ze's inbound SA decrypts them, the pings are
// answered, and the term's counter stays still.
func checkESPFormFirewallDrop(ctx context.Context, lab *scenarioLab) error {
	if err := establish(ctx, lab); err != nil {
		return err
	}
	if err := requireESPFormsDisagree(ctx, lab); err != nil {
		return err
	}
	if err := waitESPFormFirewallFinding(ctx, lab); err != nil {
		return err
	}

	before, err := lab.espCounters(ctx, espFirewallControl)
	if err != nil {
		return err
	}
	lab.execQuiet(ctx, zePeer, pingCommand(pingProbe{peer: zePeer, target: swanIP}, espFirewallPingCount)...)
	after, err := lab.espCounters(ctx, espFirewallControl)
	if err != nil {
		return err
	}
	for _, want := range espFirewallControl {
		if err := assertESPAdvanced(before[want.peer], after[want.peer], want); err != nil {
			return fmt.Errorf("the control ping from ze did not cross the Child SA, so strongSwan's ESP cannot be judged: %w", err)
		}
	}

	dropped, err := espFirewallDropped(ctx, lab)
	if err != nil {
		return err
	}
	mismatch, err := xfrmStat(ctx, lab, zePeer, "XfrmInStateMismatch")
	if err != nil {
		return err
	}
	before, err = lab.espCounters(ctx, espFirewallJudged)
	if err != nil {
		return err
	}
	output := lab.execQuiet(ctx, swanPeer, pingCommand(pingProbe{peer: swanPeer, target: zeIP}, espFirewallPingCount)...)
	after, err = lab.espCounters(ctx, espFirewallJudged)
	if err != nil {
		return err
	}
	if err := assertESPAdvanced(before[swanPeer], after[swanPeer], swanEncrypts); err != nil {
		return fmt.Errorf("strongSwan sent no ESP toward ze, so the firewall's drop cannot be judged: %w", err)
	}
	loss, err := pingLoss(output)
	if err != nil {
		return fmt.Errorf("ping from %s to %s %w", swanPeer, zeIP, err)
	}
	if loss != espFirewallLossAll {
		return fmt.Errorf("strongSwan's bare ESP was delivered past ze's %s rule: ping lost %g%% of %d, want all lost; the fast path rewrote it before the firewall saw it: %s",
			espFirewallTerm, loss, espFirewallPingCount, output)
	}
	if err := assertESPStill(before[zePeer], after[zePeer], zeDecrypts); err != nil {
		return err
	}
	droppedAfter, err := espFirewallDropped(ctx, lab)
	if err != nil {
		return err
	}
	if droppedAfter < dropped+espFirewallPingCount {
		return fmt.Errorf("ze's %s term counted %d packets during strongSwan's %d echo requests, want at least %d: the rule did not judge strongSwan's ESP",
			espFirewallTerm, droppedAfter-dropped, espFirewallPingCount, espFirewallPingCount)
	}
	mismatchAfter, err := xfrmStat(ctx, lab, zePeer, "XfrmInStateMismatch")
	if err != nil {
		return err
	}
	if mismatchAfter != mismatch {
		return fmt.Errorf("ze's XfrmInStateMismatch moved by %d during strongSwan's ping, want 0: bare ESP reached XFRM, so the %s rule did not drop it first",
			mismatchAfter-mismatch, espFirewallTerm)
	}
	return nil
}

// requireESPFormsDisagree refuses unless the two peers disagree about the ESP form,
// the case the scenario exists for: Ze's inbound state carries the ESP-in-UDP
// template, and strongSwan sends bare ESP. strongSwan sending ESP-in-UDP would never
// meet the drop-esp rule, and a bare-ESP state at Ze would leave the fast path nothing
// to rewrite.
func requireESPFormsDisagree(ctx context.Context, lab *scenarioLab) error {
	if _, err := lab.waitXFRM(ctx, zePeer); err != nil {
		return err
	}
	zeState, err := lab.inboundXFRMState(ctx, zePeer, swanIP, zeIP)
	if err != nil {
		return err
	}
	if !strings.Contains(zeState, "encap type espinudp") {
		return interoplab.WithSearchedOutput(errors.New("ze's inbound state carries no ESP-in-UDP template, so the two ESP forms do not disagree"), zeState)
	}
	sas, err := lab.listSAs(ctx)
	if err != nil {
		return err
	}
	if strings.Contains(sas, "TUNNEL-in-UDP") {
		return interoplab.WithSearchedOutput(errors.New("strongSwan sends UDP-encapsulated ESP, which the drop-esp rule never matches"), sas)
	}
	return nil
}

// waitESPFormFirewallFinding waits until `show doctor | json` answers the firewall
// finding of the fast path's check, naming the table and the term that keep its gate
// shut. The fast path judges the ruleset once the first Child SA is watched, after
// IKE_AUTH, so the finding can trail the established SA.
func waitESPFormFirewallFinding(ctx context.Context, lab *scenarioLab) error {
	var tb textbuf.Buffer
	description := tb.Str("show doctor naming ").Str(espFirewallDoctorCode).Str(" for ").Str(espFirewallKernelTable).String()
	answer, err := lab.waitOutput(ctx, zePeer, interoplab.ZeCLICommand("show doctor | json"), lab.timeout, description, func(answer string) bool {
		return espFirewallFinding(answer) == nil
	})
	if err != nil {
		return interoplab.WithSearchedOutput(err, answer)
	}
	return nil
}

// espFirewallFinding answers nil when the doctor answer holds a finding under
// espFirewallDoctorCode whose message names the kernel table and the term, and an
// error naming what it found otherwise.
func espFirewallFinding(answer string) error {
	var decoded any
	if err := json.Unmarshal([]byte(strings.TrimSpace(answer)), &decoded); err != nil {
		return fmt.Errorf("show doctor | json is not JSON: %w", err)
	}
	var coded, named int
	espFirewallObjects(decoded, 0, func(object map[string]any) {
		if object["code"] != espFirewallDoctorCode {
			return
		}
		coded++
		message, ok := object["message"].(string)
		if !ok {
			return
		}
		if strings.Contains(message, espFirewallKernelTable) && strings.Contains(message, espFirewallTerm) {
			named++
		}
	})
	if coded == 0 {
		return fmt.Errorf("show doctor answers no %s finding", espFirewallDoctorCode)
	}
	if named == 0 {
		return fmt.Errorf("show doctor answers %d %s findings, none naming table %s and term %s", coded, espFirewallDoctorCode, espFirewallKernelTable, espFirewallTerm)
	}
	return nil
}

// espFirewallDropped reads the drop-esp term's packet counter through
// `show firewall ruleset espdrop | json`, which reads the kernel's rule counters
// (handleShowFirewallRuleset, internal/plugins/firewall/nft/cmd_show.go). The
// table holds one term of that name, so exactly one term row must carry it.
func espFirewallDropped(ctx context.Context, lab *scenarioLab) (uint64, error) {
	var tb textbuf.Buffer
	command := tb.Str("show firewall ruleset ").Str(espFirewallTable).Str(" | json").String()
	answer, err := lab.zeCLI(ctx, command)
	if err != nil {
		return 0, err
	}
	var decoded any
	if err := json.Unmarshal([]byte(strings.TrimSpace(answer)), &decoded); err != nil {
		return 0, interoplab.WithSearchedOutput(fmt.Errorf("%s is not JSON: %w", command, err), answer)
	}
	var packets []float64
	espFirewallObjects(decoded, 0, func(object map[string]any) {
		if object["name"] != espFirewallTerm {
			return
		}
		count, ok := object["packets"].(float64)
		if !ok {
			return
		}
		packets = append(packets, count)
	})
	if len(packets) != 1 {
		return 0, interoplab.WithSearchedOutput(fmt.Errorf("%s answers %d %s term rows with a packet count, want 1", command, len(packets), espFirewallTerm), answer)
	}
	return uint64(packets[0]), nil
}

// assertESPStill refuses unless the ONE SA the direction names moved no bytes between
// two snapshots of its peer's dump. A direction with no SA, or an SA retired between
// the reads, is refused too: its stillness would prove nothing.
func assertESPStill(before, after map[saKey]uint64, want espDirection) error {
	beforeBytes := directionCounters(before, want)
	if len(beforeBytes) == 0 {
		return fmt.Errorf("%s reports no SA for src %s dst %s, so its stillness proves nothing", want.peer, want.source, want.target)
	}
	afterBytes := directionCounters(after, want)
	for spi, previous := range beforeBytes {
		latest, ok := afterBytes[spi]
		if !ok {
			return fmt.Errorf("%s retired SA %s for src %s dst %s between the reads, so its stillness proves nothing", want.peer, spi, want.source, want.target)
		}
		if latest != previous {
			return fmt.Errorf("%s SA %s for src %s dst %s moved from %d to %d bytes: XFRM decrypted ESP the firewall should have dropped first",
				want.peer, spi, want.source, want.target, previous, latest)
		}
	}
	return nil
}

// espFirewallObjects calls visit on every JSON object inside value, value included.
// The recursion is bounded by espFirewallJSONDepthMax: a Ze answer nests a few levels,
// and the bound stops at a deeper one rather than follow it.
func espFirewallObjects(value any, depth int, visit func(map[string]any)) {
	if depth > espFirewallJSONDepthMax {
		return
	}
	switch v := value.(type) {
	case map[string]any:
		visit(v)
		for _, child := range v {
			espFirewallObjects(child, depth+1, visit)
		}
	case []any:
		for _, child := range v {
			espFirewallObjects(child, depth+1, visit)
		}
	}
}
