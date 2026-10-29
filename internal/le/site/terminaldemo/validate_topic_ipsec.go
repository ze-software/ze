// Design: docs/contributing/terminal-demos.md -- live VPN configuration, failure diagnosis and recovery
// Related: demos/terminal/topics/ipsec -- the same sections and operator gestures on screen

package siteterminaldemo

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/ze-software/ze/internal/component/config"
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
	var last error
	for range 600 {
		last = requireTopicIPsecState(stage)
		if last == nil {
			return nil
		}
		// sleep(poll-interval): wait for IKE negotiation and kernel SAD reconciliation.
		time.Sleep(100 * time.Millisecond)
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
		if peer.Child.IfID != 42 {
			return fmt.Errorf("IPsec Child SA binds if-id %d, want 42", peer.Child.IfID)
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
		if sa.IfID != 42 {
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
	type policy struct {
		IfID       uint32 `json:"if-id"`
		Direction  string `json:"direction"`
		Source     string `json:"src"`
		Target     string `json:"dst"`
		TunnelSrc  string `json:"tunnel-src"`
		TunnelDst  string `json:"tunnel-dst"`
		Action     string `json:"action"`
		Mode       string `json:"mode"`
		Owner      string `json:"owner"`
		OwnerKnown bool   `json:"owner-known"`
	}
	var policies []policy
	if err := json.Unmarshal([]byte(answer), &policies); err != nil {
		return err
	}
	var inbound, forward, outbound bool
	for _, got := range policies {
		if got.IfID != 42 {
			continue
		}
		if child == nil {
			return fmt.Errorf("refused ESP left kernel %s policy installed", got.Direction)
		}
		want := policy{IfID: 42, Direction: got.Direction, Action: "protect",
			Mode: "tunnel", Owner: ipsecResponderPeer, OwnerKnown: true}
		switch got.Direction {
		case "in", "fwd":
			want.Source, want.Target = "10.78.2.0/24", "10.78.1.0/24"
			want.TunnelSrc, want.TunnelDst = showcaseVPNPeerEndpoint, showcaseVPNZeEndpoint
			if got.Direction == "in" {
				inbound = true
			} else {
				forward = true
			}
		case "out":
			want.Source, want.Target = "10.78.1.0/24", "10.78.2.0/24"
			want.TunnelSrc, want.TunnelDst = showcaseVPNZeEndpoint, showcaseVPNPeerEndpoint
			outbound = true
		default:
			return fmt.Errorf("IPsec kernel policy has unknown direction %q", got.Direction)
		}
		if got != want {
			return fmt.Errorf("IPsec kernel %s policy differs from the protected peer: got %+v, want %+v",
				got.Direction, got, want)
		}
	}
	if child == nil {
		return nil
	}
	if !inbound {
		return fmt.Errorf("kernel lacks the protected inbound policy")
	}
	if !forward {
		return fmt.Errorf("kernel lacks the protected forward policy")
	}
	if !outbound {
		return fmt.Errorf("kernel lacks the protected outbound policy")
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
	if err == nil {
		return fmt.Errorf("protected host ping succeeded without an IPsec Child SA")
	}
	return contains(string(answer), "0 received")
}
