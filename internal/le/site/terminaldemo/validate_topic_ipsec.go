// Design: docs/contributing/terminal-demos.md -- live VPN configuration, failure diagnosis and recovery
// Related: demos/terminal/topics/ipsec -- the same sections and operator gestures on screen

package siteterminaldemo

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"slices"
	"strings"

	"github.com/ze-software/ze/internal/component/config"
	"github.com/ze-software/ze/internal/component/ike/dataplane"
)

func validateTopicIPsec() error {
	pid, err := showcasePID()
	if err != nil {
		return err
	}
	if err := requireTopicIPsecAbsent(); err != nil {
		return err
	}
	if err := loadShowcaseSections("topics/ipsec/1-ike.conf", "topics/ipsec/2-esp.conf",
		"topics/ipsec/3-peer.conf", "topics/ipsec/4-xfrm.conf"); err != nil {
		return err
	}
	if err := awaitTopicIPsec(stageHealthy); err != nil {
		return err
	}
	if err := proveTopicIPsecTraffic(); err != nil {
		return err
	}
	if err := loadShowcaseSections("topics/ipsec/5-mismatch.conf"); err != nil {
		return err
	}
	if err := awaitTopicIPsec("refused"); err != nil {
		return err
	}
	if err := requireShowcasePID(pid); err != nil {
		return err
	}
	if err := requireTopicIPsecPingFailure(); err != nil {
		return err
	}
	if err := loadShowcaseSections("topics/ipsec/2-esp.conf"); err != nil {
		return err
	}
	answer, err := showcaseCLI("clear vpn ipsec sa peer responder | json | no-more")
	if err != nil {
		return err
	}
	if err := contains(answer, "clear-peer"); err != nil {
		return err
	}
	if err := awaitTopicIPsec(stageHealthy); err != nil {
		return err
	}
	if err := proveTopicIPsecTraffic(); err != nil {
		return err
	}
	return requireShowcasePID(pid)
}

func requireTopicIPsecAbsent() error {
	active, err := runZe([]string{commandConfig, commandCat, zeConfigFile}, scenarioEnv(showcaseLabID, demoPassword), nil)
	if err != nil {
		return err
	}
	tree, err := config.ParseTreeForValidation(active)
	if err != nil {
		return err
	}
	if tree.GetContainer("vpn") != nil {
		return fmt.Errorf("IPsec is configured before the topic commit")
	}
	answer, err := showcaseCLI("show vpn ipsec sa | json | no-more")
	if err != nil {
		return err
	}
	var peers []json.RawMessage
	if err := json.Unmarshal([]byte(answer), &peers); err != nil {
		return err
	}
	if len(peers) != 0 {
		return fmt.Errorf("IPsec SAs exist before the topic commit: %s", answer)
	}
	return requireTopicIPsecPingFailure()
}

type topicIPsecChild struct {
	Inbound  uint32  `json:"inbound-spi"`
	Outbound uint32  `json:"outbound-spi"`
	IfID     uint32  `json:"if-id"`
	Packets  *uint64 `json:"packets-in"`
}

type topicIPsecPeer struct {
	Name  string           `json:"peer-name"`
	State string           `json:"state"`
	Child *topicIPsecChild `json:"child-sa"`
}

func readTopicIPsecPeer() (topicIPsecPeer, error) {
	answer, err := showcaseCLI("show vpn ipsec peer name responder | json | no-more")
	if err != nil {
		return topicIPsecPeer{}, err
	}
	var state struct {
		Peers []topicIPsecPeer `json:"ike-sas"`
	}
	if err := json.Unmarshal([]byte(answer), &state); err != nil {
		return topicIPsecPeer{}, fmt.Errorf("decode IPsec peer: %w; %s", err, answer)
	}
	for _, peer := range state.Peers {
		if peer.Name == ipsecResponderPeer {
			if peer.State != peerEstablished {
				return topicIPsecPeer{}, fmt.Errorf("IPsec peer is %s, not established", peer.State)
			}
			return peer, nil
		}
	}
	return topicIPsecPeer{}, fmt.Errorf("IPsec responder is absent: %s", answer)
}

// awaitTopicIPsec observes only. Both the recording and validator wait for the
// same real control-plane and kernel state before they present it.
func awaitTopicIPsec(stage string) error {
	if stage != stageHealthy {
		if stage != "refused" {
			return fmt.Errorf("unknown IPsec readiness stage %q", stage)
		}
	}
	// IKE negotiation and kernel SAD reconciliation get 600 attempts.
	last := awaitCheck(600, func() error { return requireTopicIPsecState(stage) })
	if last == nil {
		return nil
	}
	// A command stopped at its deadline already names itself.
	if errors.Is(last, errCommandDeadline) {
		return last
	}
	return fmt.Errorf("IPsec %s: %w", stage, last)
}

func requireTopicIPsecState(stage string) error {
	peer, err := readTopicIPsecPeer()
	if err != nil {
		return err
	}
	if stage == stageHealthy {
		if peer.Child == nil {
			return fmt.Errorf("IKE is established but has no Child SA")
		}
		if peer.Child.Inbound == 0 {
			return fmt.Errorf("IPsec Child SA has no inbound SPI")
		}
		if peer.Child.Outbound == 0 {
			return fmt.Errorf("IPsec Child SA has no outbound SPI")
		}
		if peer.Child.IfID != topicIPsecIfID {
			return fmt.Errorf("IPsec Child SA binds if-id %d, want %d", peer.Child.IfID, topicIPsecIfID)
		}
		if peer.Child.Packets == nil {
			return fmt.Errorf("IPsec kernel packets-in is unknown")
		}
	} else {
		if peer.Child != nil {
			return fmt.Errorf("mismatched ESP still has a Child SA")
		}
		answer, err := showcaseCLI("show log recent level warn | json | no-more")
		if err != nil {
			return err
		}
		var log struct {
			Entries []struct {
				Message    string `json:"message"`
				Attributes struct {
					Notify string `json:"notify"`
					Peer   string `json:"peer"`
				} `json:"attributes"`
			} `json:"entries"`
		}
		if err := json.Unmarshal([]byte(answer), &log); err != nil {
			return err
		}
		found := false
		for _, entry := range log.Entries {
			if strings.Contains(entry.Message, "keeping the IKE SA") &&
				entry.Attributes.Notify == "NO_PROPOSAL_CHOSEN" &&
				entry.Attributes.Peer == ipsecResponderPeer {
				found = true
				break
			}
		}
		if !found {
			return fmt.Errorf("no Child SA refusal entry names responder and NO_PROPOSAL_CHOSEN: %s", answer)
		}
	}
	if err := requireTopicIPsecKernel(peer.Child); err != nil {
		return err
	}
	answer, err := showcaseCLI("show vpn ipsec dataplane drift | json | no-more")
	if err != nil {
		return err
	}
	var drift []json.RawMessage
	if err := json.Unmarshal([]byte(answer), &drift); err != nil {
		return err
	}
	if len(drift) != 0 {
		return fmt.Errorf("IPsec kernel differs from the engine: %s", answer)
	}
	return nil
}

func requireTopicIPsecKernel(child *topicIPsecChild) error {
	answer, err := showcaseCLI("show vpn ipsec dataplane sa | json | no-more")
	if err != nil {
		return err
	}
	var kernel []struct {
		SPI    uint32 `json:"spi"`
		IfID   uint32 `json:"if-id"`
		Source string `json:"src"`
		Target string `json:"dst"`
		Proto  string `json:"proto"`
	}
	if err := json.Unmarshal([]byte(answer), &kernel); err != nil {
		return err
	}
	inbound, outbound := false, false
	for _, sa := range kernel {
		if sa.IfID != topicIPsecIfID {
			continue
		}
		if child == nil {
			return fmt.Errorf("refused ESP left kernel SPI %d installed", sa.SPI)
		}
		if sa.Proto != "esp" {
			return fmt.Errorf("VPN kernel SA has protocol %s, want esp", sa.Proto)
		}
		if sa.SPI == child.Inbound && sa.Source == showcaseVPNPeerEndpoint && sa.Target == showcaseVPNZeEndpoint {
			inbound = true
		}
		if sa.SPI == child.Outbound && sa.Source == showcaseVPNZeEndpoint && sa.Target == showcaseVPNPeerEndpoint {
			outbound = true
		}
	}
	if child != nil {
		if !inbound {
			return fmt.Errorf("kernel lacks the negotiated inbound SPI: %s", answer)
		}
		if !outbound {
			return fmt.Errorf("kernel lacks the negotiated outbound SPI: %s", answer)
		}
	}
	return requireTopicIPsecPolicies(child)
}

// requireTopicIPsecPolicies checks SPD-I's local and transit paths as well as
// SPD-O. SAD drift alone cannot detect a missing forward PROTECT policy.
func requireTopicIPsecPolicies(child *topicIPsecChild) error {
	answer, err := showcaseCLI("show vpn ipsec dataplane policy | json | no-more")
	if err != nil {
		return err
	}
	var policies []topicIPsecPolicy
	if err := json.Unmarshal([]byte(answer), &policies); err != nil {
		return err
	}
	return checkTopicIPsecPolicies(policies, child != nil)
}

// topicIPsecPolicy is one row of `show vpn ipsec dataplane policy`, with the
// columns the check reads (policyInfoRow, internal/component/ike/cmd).
type topicIPsecPolicy struct {
	IfID       uint32 `json:"if-id"`
	Direction  string `json:"direction"`
	Role       string `json:"role"`
	Source     string `json:"src"`
	Target     string `json:"dst"`
	TunnelSrc  string `json:"tunnel-src"`
	TunnelDst  string `json:"tunnel-dst"`
	Action     string `json:"action"`
	Mode       string `json:"mode"`
	Owner      string `json:"owner"`
	OwnerKnown bool   `json:"owner-known"`
}

const (
	// topicIPsecIfID is the if_id of vpn-xfrm, which names its SPD.
	topicIPsecIfID = 42
	// The direction names policyDirName renders and the action name
	// policyActionName renders (internal/component/ike/cmd/show_dataplane.go).
	// They are literals because that package keeps both functions unexported
	// and is compiled out without the ze_ike tag.
	ipsecDirectionIn      = "in"
	ipsecDirectionForward = "fwd"
	ipsecDirectionOut     = "out"
	ipsecActionDiscard    = "discard"
)

// ipsecProtectDirections are the lookups a Child SA's protect entry must
// cover: local delivery, transit and output.
var ipsecProtectDirections = []string{ipsecDirectionIn, ipsecDirectionForward, ipsecDirectionOut}

// ipsecFinalDirections are the lookups whose final discard is an entry of the
// interface's SPD. Forward is absent: the Linux XFRM backend, which the lab
// runs, closes forward with the namespace default and installs no forward
// entry (interfaceForwardDefault, internal/component/ike/engine). A forward
// entry that is listed is still judged, and never required.
var ipsecFinalDirections = []string{ipsecDirectionIn, ipsecDirectionOut}

// checkTopicIPsecPolicies judges the SPD of the VPN's XFRM interface. That SPD
// holds two kinds of entry, told apart by the role Ze recorded
// (dataplane.PolicyRole) and never by their position in the answer:
//
//   - the final discard entries, which the engine installs for a bound
//     interface whether or not a Child SA exists (interfaceSPDCatchAll,
//     internal/component/ike/engine). In and out must be present for both
//     families, so unmatched traffic on the interface is refused.
//   - the Child SA's protect entries. With protected set, in, fwd and out must
//     each be present and equal the protected pair in tunnel mode. Without it
//     (the refused ESP proposal) none may be installed.
//
// Any other entry at this if_id is refused as a protect entry that differs.
func checkTopicIPsecPolicies(policies []topicIPsecPolicy, protected bool) error {
	protect := make(map[string]bool, len(ipsecProtectDirections))
	catchAll := make(map[string]bool, len(ipsecProtectDirections)*len(ipsecWildcardSelectors))
	for index := range policies {
		got := &policies[index]
		if got.IfID != topicIPsecIfID {
			continue
		}
		if !slices.Contains(ipsecProtectDirections, got.Direction) {
			return fmt.Errorf("IPsec kernel policy has unknown direction %q", got.Direction)
		}
		if got.Role == dataplane.PolicyRoleInterfaceCatchAll.String() {
			if err := checkTopicIPsecCatchAll(got); err != nil {
				return err
			}
			catchAll[got.Direction+" "+got.Source] = true
			continue
		}
		if !protected {
			return fmt.Errorf("refused ESP left kernel %s policy installed: %+v", got.Direction, *got)
		}
		want := topicIPsecPolicy{IfID: topicIPsecIfID, Direction: got.Direction,
			Role: dataplane.PolicyRoleChildSA.String(), Action: "protect",
			Mode: "tunnel", Owner: ipsecResponderPeer, OwnerKnown: true}
		if got.Direction == ipsecDirectionOut {
			want.Source, want.Target = "10.78.1.0/24", "10.78.2.0/24"
			want.TunnelSrc, want.TunnelDst = showcaseVPNZeEndpoint, showcaseVPNPeerEndpoint
		} else {
			want.Source, want.Target = "10.78.2.0/24", "10.78.1.0/24"
			want.TunnelSrc, want.TunnelDst = showcaseVPNPeerEndpoint, showcaseVPNZeEndpoint
		}
		if *got != want {
			return fmt.Errorf("IPsec kernel %s policy differs from the protected peer: got %+v, want %+v",
				got.Direction, *got, want)
		}
		protect[got.Direction] = true
	}
	for _, direction := range ipsecFinalDirections {
		for _, everything := range ipsecWildcardSelectors {
			if !catchAll[direction+" "+everything] {
				return fmt.Errorf("the XFRM interface SPD lacks its final discard entry for %s %s, so unmatched traffic there is not refused", direction, everything)
			}
		}
	}
	if !protected {
		return nil
	}
	for _, direction := range ipsecProtectDirections {
		if !protect[direction] {
			return fmt.Errorf("kernel lacks the protected %s policy", direction)
		}
	}
	return nil
}

// ipsecWildcardSelectors are the selectors of the final entry, one per family,
// as the row renders the prefixes anyNetFor builds
// (internal/component/ike/engine/bypass.go).
var ipsecWildcardSelectors = []string{"0.0.0.0/0", "::/0"}

// checkTopicIPsecCatchAll accepts only an entry that discards everything of
// one family. An entry that carries the role and lets traffic through would
// otherwise count as the final discard.
func checkTopicIPsecCatchAll(got *topicIPsecPolicy) error {
	if got.Action != ipsecActionDiscard {
		return fmt.Errorf("the XFRM interface SPD's final %s entry has action %s, not discard: %+v", got.Direction, got.Action, *got)
	}
	if got.Source != got.Target {
		return fmt.Errorf("the XFRM interface SPD's final %s entry does not match everything: %+v", got.Direction, *got)
	}
	if !slices.Contains(ipsecWildcardSelectors, got.Source) {
		return fmt.Errorf("the XFRM interface SPD's final %s entry does not match everything: %+v", got.Direction, *got)
	}
	return nil
}

func proveTopicIPsecTraffic() error {
	before, err := readTopicIPsecPeer()
	if err != nil {
		return err
	}
	if before.Child == nil {
		return fmt.Errorf("cannot prove traffic without a Child SA")
	}
	if before.Child.Packets == nil {
		return fmt.Errorf("cannot read packets-in before protected traffic")
	}
	var ping bytes.Buffer
	if err := runShowcaseLab("control", []string{showcaseVPN, controlPing}, &ping); err != nil {
		return err
	}
	if err := contains(ping.String(), "4 received"); err != nil {
		return err
	}
	after, err := readTopicIPsecPeer()
	if err != nil {
		return err
	}
	if after.Child == nil {
		return fmt.Errorf("protected traffic lost its Child SA")
	}
	if after.Child.Packets == nil {
		return fmt.Errorf("cannot read packets-in after protected traffic")
	}
	if after.Child.Inbound != before.Child.Inbound {
		return fmt.Errorf("inbound SPI changed while measuring protected traffic")
	}
	if *after.Child.Packets <= *before.Child.Packets {
		return fmt.Errorf("protected ping did not increase packets-in: before %d, after %d",
			*before.Child.Packets, *after.Child.Packets)
	}
	return requireTopicIPsecKernel(after.Child)
}

func requireTopicIPsecPingFailure() error {
	answer, err := runCommand("ip", []string{ipNetns, commandExec, showcaseHostLeft,
		programPing, "-c", "1", "-W", "2", "10.78.2.2"}, commandOptions{stdout: nil, stderr: io.Discard})
	return requireNoPingReply(string(answer), err)
}

// requireNoPingReply accepts only a probe that ran and got no reply: ping
// failed, and its summary counts zero replies. A ping that could not run (no
// namespace, no program) fails as well but prints no summary, and proves
// nothing about whether the hosts can communicate.
func requireNoPingReply(answer string, err error) error {
	if err == nil {
		return fmt.Errorf("protected host ping succeeded without an IPsec Child SA")
	}
	return contains(answer, "0 received")
}
