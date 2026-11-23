// Design: docs/architecture/testing/interop.md -- actual ordinary transit lifecycle.
// Related: check_best_route_transit_predicate.go -- native output decisions.
// FRR frr-10.7.1 bgp_route.c:bgp_static_set applies [no] network through
// bgp_static_update/bgp_static_withdraw; route_vty_out_detail prints received
// AS_PATH and next hops. BIRD c3669497 static_start/static_announce_rte install
// RTS_STATIC routes; proto_cmd_disable and static_shutdown flush only that
// protocol's routes. bgp_update_attrs prepends its AS, and rt_show_rte prints
// the selected route and its attributes. No route-server flag is configured.
package bgp

import (
	"context"
	"errors"
	"fmt"
	"net/netip"
	"strings"
	"time"

	"github.com/ze-software/ze/internal/le/interoplab"
)

type bestRouteTransitNative interface {
	observe(context.Context, *interoplab.CheckContext, string, string) error
	withdraw(context.Context, interoplab.CheckerLab, string) error
	version(context.Context, interoplab.CheckerLab, string) error
}

type bestRouteTransitFRRNative struct{}
type bestRouteTransitBIRDNative struct{}

type bestRouteTransitPeers struct {
	source, backup, sink string
	sourceHost           uint8
}

func checkBestRouteTransitFRR(ctx context.Context, check *interoplab.CheckContext) error {
	return checkBestRouteTransit(ctx, check, bestRouteTransitFRRNative{},
		bestRouteTransitPeers{source: peerFRR, backup: bestRouteTransitFRRBackup, sink: peerFRRSink, sourceHost: 3})
}

func checkBestRouteTransitBIRD(ctx context.Context, check *interoplab.CheckContext) error {
	return checkBestRouteTransit(ctx, check, bestRouteTransitBIRDNative{},
		bestRouteTransitPeers{source: peerBIRD, backup: bestRouteTransitBIRDBackup, sink: bestRouteTransitBIRDSink, sourceHost: 4})
}

// checkBestRouteTransit asks one ordered question: does the elected native
// source route cross an ordinary Ze, change to its retained backup, and leave
// on final withdrawal, while an export exclusion and useful control survive?
// Every absence is judged beside that control in a real native table.
// RFC 4271 Section 9.2: "All newly installed routes and all newly unfeasible
// routes for which there is no replacement route SHALL be advertised to its
// peers by means of an UPDATE message."
// MUTATION: discard received selected publication in selectedDispatch.publish;
// both real-native scenarios must fail their initial useful-output phase.
func checkBestRouteTransit(ctx context.Context, check *interoplab.CheckContext, native bestRouteTransitNative, peers bestRouteTransitPeers) (result error) {
	if !check.Network.IPv4.IsValid() {
		return errors.New("ordinary transit has no selected IPv4 network")
	}
	if !check.Network.IPv4.Addr().Is4() {
		return errors.New("ordinary transit network is not IPv4")
	}
	assertion := 0
	defer func() {
		if result != nil {
			result = checkerFailure(ctx, check.Lab, check.Source.Name, assertion, result)
		}
	}()
	source := netip.MustParseAddr(networkHostAddress(check.Network, peers.sourceHost))
	backup := netip.MustParseAddr(networkHostAddress(check.Network, 14))
	for _, peer := range []string{peers.source, peers.backup, peers.sink} {
		assertion++
		image, err := check.Lab.PeerImage(ctx, peer)
		if err != nil {
			return err
		}
		if image == "" {
			return fmt.Errorf("ordinary transit %s has no consumed image identity", peer)
		}
		if err := native.version(ctx, check.Lab, peer); err != nil {
			return err
		}
	}
	if err := bestRouteTransitWaitSessions(ctx, check, peers.sourceHost); err != nil {
		return err
	}
	assertion++
	if err := bestRouteTransitWaitZe(ctx, check, source, backup); err != nil {
		return err
	}
	assertion++
	if err := native.observe(ctx, check, peers.sink, "65000 65001"); err != nil {
		return err
	}
	assertion++
	if err := native.withdraw(ctx, check.Lab, peers.source); err != nil {
		return err
	}
	assertion++
	if err := native.observe(ctx, check, peers.sink, "65000 65002 65002 65002"); err != nil {
		return err
	}
	assertion++
	if err := native.withdraw(ctx, check.Lab, peers.backup); err != nil {
		return err
	}
	assertion++
	if err := native.observe(ctx, check, peers.sink, ""); err != nil {
		return err
	}
	// Q remains a selected source-A route even after P has left both sources.
	// Its exclusion is recipient policy, never a missing source or dead session.
	assertion++
	if err := bestRouteTransitWaitDenied(ctx, check, source); err != nil {
		return err
	}
	return bestRouteTransitWaitSessions(ctx, check, peers.sourceHost)
}

func bestRouteTransitWaitSessions(ctx context.Context, check *interoplab.CheckContext, source uint8) error {
	for _, host := range []uint8{source, 14, 15} {
		address := networkHostAddress(check.Network, host)
		command := zeCommand("show bgp peer " + address + " detail")
		last, _, err := interoplab.Wait(ctx, interoplab.WaitOptions{Timeout: 60 * time.Second, Interval: time.Second, Description: "ordinary transit first physical session and initial EOR"},
			func(probe context.Context) (string, error) {
				return check.Lab.Query(probe, "ze", command, queryEnvironment("ze", command))
			},
			func(output string) bool { return requireSRv6IdentityZeSession(output, address) == nil })
		if err != nil {
			return interoplab.WithSearchedOutput(err, last)
		}
	}
	return nil
}

func bestRouteTransitWaitZe(ctx context.Context, check *interoplab.CheckContext, source, backup netip.Addr) error {
	command := zeCommand("show bgp adj-rib-in")
	last, _, err := interoplab.Wait(ctx, interoplab.WaitOptions{Timeout: 60 * time.Second, Interval: time.Second, Description: "both native transit candidates retained before source withdrawal"},
		func(probe context.Context) (string, error) {
			return check.Lab.Query(probe, "ze", command, queryEnvironment("ze", command))
		},
		func(output string) bool { return requireBestRouteTransitInputs(output, source, backup) == nil })
	if err != nil {
		return interoplab.WithSearchedOutput(err, last)
	}
	return bestRouteTransitWaitDenied(ctx, check, source)
}

func bestRouteTransitWaitDenied(ctx context.Context, check *interoplab.CheckContext, source netip.Addr) error {
	command := zeCommand("show bgp rib best | prefix " + bestRouteTransitDenied)
	last, _, err := interoplab.Wait(ctx, interoplab.WaitOptions{Timeout: 60 * time.Second, Interval: time.Second, Description: "export-excluded transit route remains elected at its original source"},
		func(probe context.Context) (string, error) {
			return check.Lab.Query(probe, "ze", command, queryEnvironment("ze", command))
		},
		func(output string) bool { return requireBestRouteTransitSelected(output, source) == nil })
	if err != nil {
		return interoplab.WithSearchedOutput(err, last)
	}
	return nil
}

func (bestRouteTransitFRRNative) observe(ctx context.Context, check *interoplab.CheckContext, sink, path string) error {
	ze := netip.MustParseAddr(networkHostAddress(check.Network, 2))
	last, _, err := interoplab.Wait(ctx, interoplab.WaitOptions{Timeout: 60 * time.Second, Interval: time.Second, Description: "FRR ordinary transit path " + path},
		func(probe context.Context) (bestRouteTransitSnapshot, error) {
			return bestRouteTransitRead(probe, check.Lab, sink, path, []string{cmdVtysh, "-c", showTableJSON},
				func(prefix string) []string {
					return []string{cmdVtysh, "-c", "show bgp ipv4 unicast " + prefix + " json"}
				})
		}, func(reading bestRouteTransitSnapshot) bool {
			return requireBestRouteTransitFRR(reading, path, ze) == nil
		})
	if err != nil {
		return fmt.Errorf("FRR ordinary transit: %w; last=%+v", err, last)
	}
	return nil
}

func (bestRouteTransitFRRNative) withdraw(ctx context.Context, lab interoplab.CheckerLab, source string) error {
	asn := "65001"
	if source == bestRouteTransitFRRBackup {
		asn = "65002"
	}
	_, err := lab.Exec(ctx, source, []string{cmdVtysh, "-c", frrConfigureTerminal, "-c", "router bgp " + asn,
		"-c", "address-family ipv4 unicast", "-c", "no network " + bestRouteTransitSubject}, nil)
	return err
}

func (bestRouteTransitFRRNative) version(ctx context.Context, lab interoplab.CheckerLab, peer string) error {
	output, err := lab.Query(ctx, peer, []string{cmdVtysh, "-c", frrShowVersion}, nil)
	if err != nil {
		return err
	}
	return requireContains(output, "FRRouting", "10.7.1")
}

func (bestRouteTransitBIRDNative) observe(ctx context.Context, check *interoplab.CheckContext, sink, path string) error {
	last, _, err := interoplab.Wait(ctx, interoplab.WaitOptions{Timeout: 60 * time.Second, Interval: time.Second, Description: "BIRD ordinary transit path " + path},
		func(probe context.Context) (bestRouteTransitSnapshot, error) {
			return bestRouteTransitRead(probe, check.Lab, sink, path, []string{cmdBirdc, birdShowRouteAll},
				func(prefix string) []string { return []string{cmdBirdc, "show route for " + prefix + " all"} })
		}, func(reading bestRouteTransitSnapshot) bool { return requireBestRouteTransitBIRD(reading, path) == nil })
	if err != nil {
		return fmt.Errorf("BIRD ordinary transit: %w; last=%+v", err, last)
	}
	return nil
}

func (bestRouteTransitBIRDNative) withdraw(ctx context.Context, lab interoplab.CheckerLab, source string) error {
	_, err := lab.Exec(ctx, source, []string{cmdBirdc, "disable transit_subject"}, nil)
	return err
}

func (bestRouteTransitBIRDNative) version(ctx context.Context, lab interoplab.CheckerLab, peer string) error {
	output, err := lab.Query(ctx, peer, []string{cmdBirdc, cmdShowStatus}, nil)
	if err != nil {
		return err
	}
	return requireContains(output, "BIRD 3.4.0")
}

type bestRouteTransitSnapshot struct{ table, subject, control string }

func bestRouteTransitRead(ctx context.Context, lab interoplab.CheckerLab, sink, path string, table []string, prefix func(string) []string) (bestRouteTransitSnapshot, error) {
	var reading bestRouteTransitSnapshot
	var err error
	reading.table, err = lab.Query(ctx, sink, table, nil)
	if err != nil {
		return reading, err
	}
	reading.control, err = lab.Query(ctx, sink, prefix(bestRouteTransitControl), nil)
	if err != nil {
		return reading, err
	}
	if path != "" {
		reading.subject, err = lab.Query(ctx, sink, prefix(bestRouteTransitSubject), nil)
	}
	return reading, err
}

// bestRouteTransitPrefixFields reads only complete first-field route names,
// not covering prefixes, descriptions or a command error containing a prefix.
func bestRouteTransitPrefixFields(output string) map[netip.Prefix]bool {
	prefixes := make(map[netip.Prefix]bool)
	for line := range strings.SplitSeq(output, "\n") {
		fields := strings.Fields(line)
		if len(fields) == 0 {
			continue
		}
		prefix, err := netip.ParsePrefix(fields[0])
		if err == nil {
			prefixes[prefix] = true
		}
	}
	return prefixes
}
