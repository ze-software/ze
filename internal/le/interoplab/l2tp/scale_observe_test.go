// Design: docs/labs/l2tp-interop.md -- bound traffic and exact-session actions.
// Related: scale_observe.go -- the calls made against independent lab peers.
package l2tp

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"strings"
	"testing"

	"github.com/ze-software/ze/internal/le/interoplab"
)

type scaleObservationLab struct {
	recordingLab
	failDevice string
}

func (l *scaleObservationLab) Exec(_ context.Context, peer string, args []string, _ []interoplab.EnvironmentVariable) (interoplab.CommandResult, error) {
	l.calls = append(l.calls, recordedCall{operation: "exec", peer: peer, arguments: append([]string(nil), args...)})
	if slices.Equal(args, []string{"ip", "-j", "-4", scaleIPAddr, "show"}) {
		if peer != "kl2tpd-1" {
			return interoplab.CommandResult{Stdout: "[]"}, nil
		}
		return interoplab.CommandResult{Stdout: `[{"ifname":"sc1s01","flags":["UP","POINTOPOINT"],"addr_info":[{"family":"inet","local":"10.100.0.2","address":"10.100.0.1","prefixlen":32}]},{"ifname":"sc1s02","flags":["UP","POINTOPOINT"],"addr_info":[{"family":"inet","local":"10.100.0.3","address":"10.100.0.1","prefixlen":32}]}]`}, nil
	}
	if len(args) == 9 && args[0] == commandPing {
		if args[3] == l.failDevice {
			return interoplab.CommandResult{}, errors.New("selected direction failed")
		}
		return interoplab.CommandResult{Stdout: "1 packets transmitted, 1 received\n"}, nil
	}
	if len(args) > 0 && args[0] == "sh" {
		return interoplab.CommandResult{}, nil
	}
	if len(args) > 0 && args[0] == "radclient" {
		return interoplab.CommandResult{Stdout: "Received Disconnect-ACK Id 1 from 172.29.0.2:3799\n"}, nil
	}
	return interoplab.CommandResult{}, fmt.Errorf("unexpected scale exec %s %v", peer, args)
}

func TestScaleInterfaceBoundTraffic(t *testing.T) {
	population := scaleTestPopulation(t, scaleTestSessions()[:2])
	observed := scaleObservation{population: population, links: map[string]string{"scale-1-01": "ppp7", "scale-1-02": "ppp8"}}
	lab := &scaleObservationLab{}
	if err := requireScaleTraffic(t.Context(), lab, observed, false); err != nil {
		t.Fatal(err)
	}
	for _, expected := range []struct{ peer, device, destination string }{
		{"kl2tpd-1", "sc1s01", localPPPAddress}, {"kl2tpd-1", "sc1s02", localPPPAddress},
		{peerZe, "ppp7", "10.100.0.2"}, {peerZe, "ppp8", "10.100.0.3"},
	} {
		if !slices.ContainsFunc(lab.calls, func(call recordedCall) bool {
			return call.peer == expected.peer && slices.Equal(call.arguments, []string{commandPing, "-n", "-I", expected.device, "-c", "1", "-W", "2", expected.destination})
		}) {
			t.Fatalf("missing independent bound probe %+v", expected)
		}
	}
	if err := requireScaleTraffic(t.Context(), &scaleObservationLab{failDevice: "ppp8"}, observed, false); err == nil {
		t.Fatal("working peer-to-Ze direction hid failed Ze-to-peer traffic")
	}
}

func TestScaleSilentSessionActions(t *testing.T) {
	lab := &scaleObservationLab{}
	id := "complete-accounting-id-with-all-suffixes"
	if err := scaleDisconnect(t.Context(), lab, id); err != nil {
		t.Fatalf("silent request-file write refused: %v", err)
	}
	if err := scaleStopPeerPPP(t.Context(), lab, "scale-2-07"); err != nil {
		t.Fatalf("silent exact-process kill refused: %v", err)
	}
	var request, process bool
	for _, call := range lab.calls {
		if call.peer == peerRadius && len(call.arguments) > 4 && strings.Contains(call.arguments[4], fmt.Sprintf("Acct-Session-Id = %q\n", id)) {
			request = true
		}
		if call.peer == "kl2tpd-2" && slices.Contains(call.arguments, "/var/run/ppp-scale-2-07.pid") && slices.Contains(call.arguments, "sc2s07") {
			process = true
		}
	}
	if !request || !process {
		t.Fatalf("lost exact request/process identity: request=%v process=%v", request, process)
	}
}
