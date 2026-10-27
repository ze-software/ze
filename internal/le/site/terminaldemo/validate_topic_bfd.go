// Design: docs/guide/bfd.md -- live opt-in, causal failure, retirement and recovery
// Related: demos/terminal/topics/bfd -- the same observations in the recording

package siteterminaldemo

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"
)

type networkBFDSession struct {
	Peer        string `json:"peer"`
	State       string `json:"state"`
	Profile     string `json:"profile"`
	TXPackets   uint64 `json:"tx-packets"`
	RXPackets   uint64 `json:"rx-packets"`
	Transitions []struct {
		From string `json:"from"`
		To   string `json:"to"`
		Diag string `json:"diag"`
	} `json:"transitions"`
}

type networkBGPPeer struct {
	State                  string `json:"state"`
	ConnectionsEstablished uint64 `json:"connections-established"`
	NegotiatedHoldTime     uint64 `json:"negotiated-hold-time"`
	LastNotification       *struct {
		Code    uint8 `json:"code"`
		Subcode uint8 `json:"subcode"`
	} `json:"last-notification"`
}

func networkFRRPeer() (networkBGPPeer, error) {
	answer, err := networkJSON[struct {
		Peers map[string]networkBGPPeer `json:"peers"`
	}]("show bgp peer 172.29.0.2 detail")
	if err != nil {
		return networkBGPPeer{}, err
	}
	peer, ok := answer.Peers["172.29.0.2"]
	if !ok {
		return peer, fmt.Errorf("FRR BGP peer is absent")
	}
	return peer, nil
}

type networkFRRSessionCounters struct {
	established uint64
	dropped     uint64
}

// FRR retains these counters when Ze replaces its Peer object. Ze's own
// connections-established can return to one after that otherwise hidden reset.
func readNetworkFRRSessionCounters() (networkFRRSessionCounters, error) {
	answer, err := runCommand("ip", []string{"netns", "exec", "showcase-frr", "vtysh", "-c", "show bgp neighbor 172.29.0.1 json"}, commandOptions{})
	if err != nil {
		return networkFRRSessionCounters{}, err
	}
	return parseNetworkFRRSessionCounters(answer)
}

func parseNetworkFRRSessionCounters(answer []byte) (networkFRRSessionCounters, error) {
	var peers map[string]struct {
		State       string  `json:"bgpState"`
		Established *uint64 `json:"connectionsEstablished"`
		Dropped     *uint64 `json:"connectionsDropped"`
	}
	if err := json.Unmarshal(answer, &peers); err != nil {
		return networkFRRSessionCounters{}, fmt.Errorf("decode FRR session counters: %w", err)
	}
	peer, ok := peers["172.29.0.1"]
	if !ok || peer.State != "Established" || peer.Established == nil || peer.Dropped == nil || *peer.Established == 0 {
		return networkFRRSessionCounters{}, fmt.Errorf("FRR needs an Established Ze neighbor with explicit session counters: %s", answer)
	}
	return networkFRRSessionCounters{established: *peer.Established, dropped: *peer.Dropped}, nil
}

func requireNetworkBFDContinuity(before, after networkBGPPeer, frrBefore, frrAfter networkFRRSessionCounters) error {
	if after.ConnectionsEstablished != before.ConnectionsEstablished {
		return fmt.Errorf("live BFD opt-in restarted BGP: before %+v, after %+v", before, after)
	}
	if frrAfter != frrBefore {
		return fmt.Errorf("live BFD opt-in changed FRR session counters: before %+v, after %+v", frrBefore, frrAfter)
	}
	return nil
}

type networkBGPHistory struct {
	Transitions []struct {
		From string `json:"from"`
		To   string `json:"to"`
	} `json:"transitions"`
}

func requireNetworkBGPDeparture(history networkBGPHistory) error {
	for _, transition := range history.Transitions {
		// History preserves fsm.State.String(); peer-detail uses lowercase.
		if transition.From == "ESTABLISHED" && transition.To != "" && transition.To != "ESTABLISHED" {
			return nil
		}
	}
	return fmt.Errorf("BGP history does not show departure from Established after BFD failure")
}

func observeTopicBFD(stage string) error {
	if stage == "absent" {
		if err := requireNetworkTopicAbsent("bfd"); err != nil {
			return err
		}
		answer, err := showcaseCLI("show bfd sessions | json | no-more")
		if err != nil && strings.Contains(answer+err.Error(), "bfd: plugin not loaded") {
			return nil
		}
		if err != nil {
			return err
		}
		if strings.TrimSpace(answer) != "[]" && strings.TrimSpace(answer) != "null" {
			return fmt.Errorf("BFD sessions exist before opt-in: %s", answer)
		}
		return nil
	}
	sessions, err := networkJSON[[]networkBFDSession]("show bfd sessions")
	if err != nil {
		return err
	}
	if stage == "retired" {
		for _, session := range sessions {
			if session.Peer == "172.29.0.2" {
				return fmt.Errorf("released FRR BFD session still retained in %s", session.State)
			}
		}
		return nil
	}
	peer, err := networkFRRPeer()
	if err != nil {
		return err
	}
	for _, session := range sessions {
		if session.Peer != "172.29.0.2" {
			continue
		}
		switch stage {
		case "healthy":
			if session.State == "up" && session.Profile == "fast" && session.TXPackets > 0 && session.RXPackets > 0 && peer.State == "established" {
				return nil
			}
		case "failed":
			for _, transition := range session.Transitions {
				if transition.From == "up" && transition.To == "down" && transition.Diag == "control-detection-time-expired" && peer.State != "established" && peer.LastNotification != nil && peer.LastNotification.Code == 6 && peer.LastNotification.Subcode == 10 {
					return nil
				}
			}
		default:
			return fmt.Errorf("unknown BFD stage %q", stage)
		}
		return fmt.Errorf("BFD stage %s: session %+v; BGP %+v", stage, session, peer)
	}
	return fmt.Errorf("FRR BFD session absent at %s", stage)
}

func awaitTopicBFD(stage string) error {
	timeout := 30 * time.Second
	if stage == "failed" {
		timeout = 8 * time.Second
	}
	return awaitNetworkObservation("BFD "+stage, timeout, func() error { return observeTopicBFD(stage) })
}

func checkTopicBFD() error {
	pid, err := showcasePID()
	if err != nil {
		return err
	}
	if err := observeTopicBFD("absent"); err != nil {
		return err
	}
	before, err := networkFRRPeer()
	if err != nil {
		return err
	}
	if before.State != "established" || before.NegotiatedHoldTime <= 8 {
		return fmt.Errorf("BFD needs an established peer with hold time longer than failure deadline: %+v", before)
	}
	frrBefore, err := readNetworkFRRSessionCounters()
	if err != nil {
		return err
	}
	if err := loadShowcaseSections("topics/bfd/1-profile.conf", "topics/bfd/2-peer.conf"); err != nil {
		return err
	}
	if err := awaitTopicBFD("healthy"); err != nil {
		return err
	}
	after, err := networkFRRPeer()
	if err != nil {
		return err
	}
	frrAfter, err := readNetworkFRRSessionCounters()
	if err != nil {
		return err
	}
	if err := requireNetworkBFDContinuity(before, after, frrBefore, frrAfter); err != nil {
		return err
	}
	if err := requireShowcasePID(pid); err != nil {
		return err
	}
	cut := time.Now()
	if err := controlNetworkFixture("frr", "down"); err != nil {
		return err
	}
	if err := awaitTopicBFD("failed"); err != nil {
		return err
	}
	if elapsed := time.Since(cut); elapsed >= time.Duration(before.NegotiatedHoldTime)*time.Second {
		return fmt.Errorf("BFD failure took %s, outside negotiated BGP hold time %ds", elapsed, before.NegotiatedHoldTime)
	}
	history, err := networkJSON[networkBGPHistory]("show bgp peer 172.29.0.2 history")
	if err != nil {
		return err
	}
	if err := requireNetworkBGPDeparture(history); err != nil {
		return err
	}
	if err := requireShowcasePID(pid); err != nil {
		return err
	}
	if err := awaitTopicBFD("retired"); err != nil {
		return err
	}
	if err := controlNetworkFixture("frr", "up"); err != nil {
		return err
	}
	if err := awaitTopicBFD("healthy"); err != nil {
		return err
	}
	return requireShowcasePID(pid)
}
