// Design: docs/architecture/testing/interop.md -- live address movement without replacing an IKE or Child SA.
// Related: test/interop-ipsec/scenarios/mobike-initiator and mobike-responder -- tunnel peers.
// RFC: rfc/full/rfc4555.txt -- address updates and return routability, Sections 3.5 and 3.7.
package ipsec

import (
	"context"
	"encoding/json"
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/ze-software/ze/internal/le/interoplab"
)

const (
	mobikeZeMoved   = "172.28.0.8"
	mobikeSwanMoved = "172.28.0.9"
)

var (
	mobikeSwanIKE    = regexp.MustCompile(`(?m)^ze: #[0-9]+, ESTABLISHED, IKEv2, ([0-9a-fA-F]{16})_i(\*?) ([0-9a-fA-F]{16})_r(\*?)$`)
	mobikeSwanChild  = regexp.MustCompile(`(?m)^  ze-child: #[0-9]+, reqid [0-9]+, INSTALLED, TUNNEL(?:-in-UDP)?, ESP:`)
	mobikeSwanLocal  = regexp.MustCompile(`(?m)^  local  '[^']*' @ ([^\s\[]+)\[([0-9]+)\]`)
	mobikeSwanRemote = regexp.MustCompile(`(?m)^  remote '[^']*' @ ([^\s\[]+)\[([0-9]+)\]`)
)

type mobikeIdentity struct {
	initiatorSPI string
	responderSPI string
	inboundSPI   uint32
	outboundSPI  uint32
}

type mobikeZeRecord struct {
	Peer         string `json:"peer-name"`
	State        string `json:"state"`
	Initiator    bool   `json:"is-initiator"`
	InitiatorSPI string `json:"initiator-spi"`
	ResponderSPI string `json:"responder-spi"`
	Child        *struct {
		InboundSPI  uint32 `json:"inbound-spi"`
		OutboundSPI uint32 `json:"outbound-spi"`
		Mode        string `json:"mode"`
		LocalTS     string `json:"ts-local"`
		RemoteTS    string `json:"ts-remote"`
		Remote      string `json:"remote-address"`
	} `json:"child-sa"`
}

// checkMOBIKEResponder removes strongSwan's original address while Ze answers on
// its fixed address. Neither a rekey nor a fresh establishment can satisfy the
// identity comparison, even if the replacement tunnel carries traffic.
func checkMOBIKEResponder(ctx context.Context, lab *scenarioLab) error {
	return checkMOBIKE(ctx, lab, swanPeer, mobikeSwanMoved)
}

// checkMOBIKEInitiator changes Ze's route-selected source and removes its old
// address. Only the native engine can tell strongSwan about the new endpoints;
// the checker never reloads configuration or asks either daemon to initiate.
func checkMOBIKEInitiator(ctx context.Context, lab *scenarioLab) error {
	return checkMOBIKE(ctx, lab, zePeer, mobikeZeMoved)
}

func checkMOBIKE(ctx context.Context, lab *scenarioLab, moving, moved string) error {
	if err := establish(ctx, lab); err != nil {
		return err
	}
	if err := lab.waitLog(ctx, swanPeer, "peer supports MOBIKE", lab.timeout); err != nil {
		return err
	}
	if err := lab.natInnerAddress(ctx, zePeer, natTunnelZeInner, natTunnelSwanInner); err != nil {
		return err
	}
	if err := lab.natInnerAddress(ctx, swanPeer, natTunnelSwanInner, natTunnelZeInner); err != nil {
		return err
	}
	initiator := moving == zePeer
	before, err := lab.mobikeEstablished(ctx, zeIP, swanIP, initiator)
	if err != nil {
		return fmt.Errorf("before movement: %w", err)
	}
	if err := lab.mobikeTraffic(ctx, zeIP, swanIP, before); err != nil {
		return fmt.Errorf("before movement: %w", err)
	}

	local, remote := zeIP, swanIP
	old, fixed := zeIP, swanIP
	if moving == swanPeer {
		old, fixed = swanIP, zeIP
		remote = moved
	} else {
		local = moved
	}
	if err := lab.mobikeMove(ctx, moving, old, moved, fixed); err != nil {
		return err
	}
	_, _, err = interoplab.Wait(ctx, interoplab.WaitOptions{
		Timeout: xfrmWaitTimeout, Interval: time.Second, Description: "MOBIKE migration of the original IKE and Child SAs",
	}, func(probe context.Context) (bool, error) {
		after, err := lab.mobikeEstablished(probe, local, remote, initiator)
		if err != nil {
			return false, err
		}
		if err := requireMOBIKEIdentity(before, after); err != nil {
			return false, err
		}
		return true, nil
	}, func(ready bool) bool { return ready })
	if err != nil {
		return err
	}
	if err := lab.mobikeTraffic(ctx, local, remote, before); err != nil {
		return fmt.Errorf("after movement: %w", err)
	}
	after, err := lab.mobikeEstablished(ctx, local, remote, initiator)
	if err != nil {
		return err
	}
	return requireMOBIKEIdentity(before, after)
}

// mobikeMove leaves a host route to the fixed peer before deleting the old /24.
// The new /32 answers ARP on the same Docker bridge. Unlike flushing eth0, this
// preserves a usable route after the old connected route disappears. The lab
// owns and destroys the network namespaces, so no host address is changed.
func (l *scenarioLab) mobikeMove(ctx context.Context, peer, old, moved, fixed string) error {
	commands := [][]string{
		{"ip", "address", "add", moved + "/32", "dev", "eth0"},
		{"ip", "route", "replace", fixed + "/32", "dev", "eth0", "src", moved},
		{"ip", "address", "del", old + "/24", "dev", "eth0"},
	}
	for _, command := range commands {
		if _, err := l.exec(ctx, peer, command...); err != nil {
			return fmt.Errorf("move %s from %s to %s: %w", peer, old, moved, err)
		}
	}
	answer, err := l.exec(ctx, peer, "ip", "-j", "route", "get", fixed)
	if err != nil {
		return err
	}
	var routes []struct {
		Source string `json:"prefsrc"`
		Device string `json:"dev"`
	}
	if err := json.Unmarshal([]byte(answer), &routes); err != nil {
		return fmt.Errorf("decode %s route after movement: %w", peer, err)
	}
	if len(routes) != 1 {
		return fmt.Errorf("%s has no unique route to %s after movement: %s", peer, fixed, answer)
	}
	if routes[0].Source != moved {
		return fmt.Errorf("%s did not select new source %s: %s", peer, moved, answer)
	}
	if routes[0].Device != "eth0" {
		return fmt.Errorf("%s route no longer uses the lab bridge: %s", peer, answer)
	}
	return nil
}

func (l *scenarioLab) mobikeEstablished(ctx context.Context, local, remote string, initiator bool) (mobikeIdentity, error) {
	answer, err := l.zeCLI(ctx, "show vpn ipsec sa | json")
	if err != nil {
		return mobikeIdentity{}, err
	}
	identity, err := mobikeZeIdentity(answer, remote, initiator)
	if err != nil {
		return mobikeIdentity{}, err
	}
	sas, err := l.listSAs(ctx)
	if err != nil {
		return mobikeIdentity{}, err
	}
	if err := requireMOBIKESwan(sas, local, remote, initiator, identity); err != nil {
		return mobikeIdentity{}, err
	}
	for _, peer := range []string{zePeer, swanPeer} {
		if _, err := l.mobikeLifetimes(ctx, peer, local, remote, identity); err != nil {
			return mobikeIdentity{}, err
		}
	}
	return identity, nil
}

func mobikeZeIdentity(answer, remote string, initiator bool) (mobikeIdentity, error) {
	var records []mobikeZeRecord
	if err := json.Unmarshal([]byte(answer), &records); err != nil {
		return mobikeIdentity{}, fmt.Errorf("decode Ze MOBIKE SA: %w", err)
	}
	if len(records) != 1 {
		return mobikeIdentity{}, fmt.Errorf("expected exactly one Ze IKE SA: %s", answer)
	}
	sa := &records[0]
	if sa.Peer != swanConfigPeer || sa.State != "established" || sa.Initiator != initiator {
		return mobikeIdentity{}, fmt.Errorf("Ze IKE SA is not established in the expected role: %s", answer)
	}
	for _, spi := range []string{sa.InitiatorSPI, sa.ResponderSPI} {
		value, err := strconv.ParseUint(spi, 16, 64)
		if err != nil || len(spi) != 16 || value == 0 {
			return mobikeIdentity{}, fmt.Errorf("Ze reported invalid IKE SPI %q", spi)
		}
	}
	if sa.Child == nil {
		return mobikeIdentity{}, fmt.Errorf("Ze has no Child SA: %s", answer)
	}
	if sa.Child.InboundSPI == 0 || sa.Child.OutboundSPI == 0 {
		return mobikeIdentity{}, fmt.Errorf("Ze reported an unkeyed Child SA: %s", answer)
	}
	if sa.Child.Mode != "tunnel" || sa.Child.Remote != remote {
		return mobikeIdentity{}, fmt.Errorf("Ze Child SA did not reach tunnel endpoint %s: %s", remote, answer)
	}
	if sa.Child.LocalTS != natTunnelZeInner+"/32" || sa.Child.RemoteTS != natTunnelSwanInner+"/32" {
		return mobikeIdentity{}, fmt.Errorf("MOBIKE changed the inner traffic selectors: %s", answer)
	}
	return mobikeIdentity{
		initiatorSPI: sa.InitiatorSPI, responderSPI: sa.ResponderSPI,
		inboundSPI: sa.Child.InboundSPI, outboundSPI: sa.Child.OutboundSPI,
	}, nil
}

// requireMOBIKESwan reads charon's account, not Ze's own belief. The starred SPI
// identifies the initiator, while local and remote name the current IKE endpoints.
func requireMOBIKESwan(answer, local, remote string, initiator bool, identity mobikeIdentity) error {
	matches := mobikeSwanIKE.FindAllStringSubmatch(answer, -1)
	if len(matches) != 1 || strings.Count(answer, "IKEv2,") != 1 {
		return fmt.Errorf("strongSwan did not retain exactly one established IKE SA: %s", answer)
	}
	match := matches[0]
	if match[1] != identity.initiatorSPI || match[3] != identity.responderSPI {
		return fmt.Errorf("Ze and strongSwan disagree on IKE SPIs: %s", answer)
	}
	starI, starR := "*", ""
	if initiator {
		starI, starR = "", "*"
	}
	if match[2] != starI || match[4] != starR {
		return fmt.Errorf("strongSwan reports the wrong IKE role: %s", answer)
	}
	if len(mobikeSwanChild.FindAllString(answer, -1)) != 1 {
		return fmt.Errorf("strongSwan has no unique installed tunnel Child SA: %s", answer)
	}
	for _, endpoint := range []struct {
		pattern *regexp.Regexp
		address string
	}{
		{pattern: mobikeSwanLocal, address: remote},
		{pattern: mobikeSwanRemote, address: local},
	} {
		matches := endpoint.pattern.FindAllStringSubmatch(answer, -1)
		if len(matches) != 1 {
			return fmt.Errorf("strongSwan reported no unique IKE endpoint: %s", answer)
		}
		if matches[0][1] != endpoint.address || matches[0][2] != "4500" {
			return fmt.Errorf("strongSwan IKE endpoint is not %s:4500: %s", endpoint.address, answer)
		}
	}
	return nil
}

func requireMOBIKEIdentity(before, after mobikeIdentity) error {
	if before != after {
		return fmt.Errorf("MOBIKE replaced an IKE or Child SA instead of migrating it: before=%+v after=%+v", before, after)
	}
	return nil
}

func (l *scenarioLab) mobikeLifetimes(ctx context.Context, peer, local, remote string, identity mobikeIdentity) (map[readbackSAKey]espLifetime, error) {
	answer, err := l.exec(ctx, peer, "ip", "-s", xfrmCommand, "state")
	if err != nil {
		return nil, err
	}
	lifetimes, err := readbackLifetimes(answer)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", peer, err)
	}
	if err := requireMOBIKEEndpoints(lifetimes, local, remote, identity); err != nil {
		return nil, fmt.Errorf("%s: %w", peer, err)
	}
	return lifetimes, nil
}

// requireMOBIKEEndpoints requires both original SPIs at the new endpoints and
// no stale state. An unordered SPI set alone would miss a one-direction move.
func requireMOBIKEEndpoints(lifetimes map[readbackSAKey]espLifetime, local, remote string, identity mobikeIdentity) error {
	if len(lifetimes) != 2 {
		return fmt.Errorf("expected exactly two directed ESP states, got %v", lifetimes)
	}
	for _, key := range []readbackSAKey{
		{source: local, target: remote, spi: identity.outboundSPI},
		{source: remote, target: local, spi: identity.inboundSPI},
	} {
		if _, ok := lifetimes[key]; !ok {
			return fmt.Errorf("missing migrated ESP state %+v; observed %v", key, lifetimes)
		}
	}
	return nil
}

// mobikeTraffic measures bytes AND packets on all four simplex kernel SAs.
// Each sample brackets traffic on one endpoint tuple. The kernel integration
// tests separately require Ze's migration to preserve its live lifetime counters.
func (l *scenarioLab) mobikeTraffic(ctx context.Context, local, remote string, identity mobikeIdentity) error {
	before := make(map[string]map[readbackSAKey]espLifetime, 2)
	for _, peer := range []string{zePeer, swanPeer} {
		lifetimes, err := l.mobikeLifetimes(ctx, peer, local, remote, identity)
		if err != nil {
			return err
		}
		before[peer] = lifetimes
	}
	for _, probe := range []pingProbe{
		{peer: zePeer, source: natTunnelZeInner, target: natTunnelSwanInner},
		{peer: swanPeer, source: natTunnelSwanInner, target: natTunnelZeInner},
	} {
		if err := l.requireLosslessPingFrom(ctx, probe, 4); err != nil {
			return err
		}
	}
	for _, peer := range []string{zePeer, swanPeer} {
		after, err := l.mobikeLifetimes(ctx, peer, local, remote, identity)
		if err != nil {
			return err
		}
		for key, previous := range before[peer] {
			current := after[key]
			if current.bytes <= previous.bytes || current.packets <= previous.packets {
				return fmt.Errorf("%s ESP %+v carried no bidirectional traffic: before=%+v after=%+v", peer, key, previous, current)
			}
		}
	}
	return nil
}
