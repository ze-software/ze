// Design: docs/labs/pppoe-interop.md -- two independent clients on one access
//go:build ze_l2tp

package pppoe

import (
	"context"
	"net"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	discovery "github.com/ze-software/ze/internal/component/l2tp/pppoe"
	"github.com/ze-software/ze/internal/le/interoplab"
)

// The plan must select two actual client containers and include both in cleanup.
func TestSharedAccessPlanSelectsBothClients(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "ze.conf"), []byte("fixture"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "role"), []byte(roleZeAC), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, ok := checkers()[sharedAccessScenario]; !ok {
		t.Fatal("shared access checker is not registered")
	}
	plan, err := scenarioPlan(interoplab.ScenarioSource{Name: sharedAccessScenario, Directory: dir}, "two")
	if err != nil {
		t.Fatal(err)
	}
	if len(plan.Peers) != 3 {
		t.Fatalf("peers=%d, want actual Ze and two pppd clients", len(plan.Peers))
	}
	if len(plan.Containers) != len(plan.Peers) {
		t.Fatal("cleanup misses a declared peer")
	}
	for i, peer := range plan.Peers {
		if !slices.Contains(plan.Containers, peer.Container) {
			t.Fatalf("cleanup misses %s", peer.Name)
		}
		if i == 0 {
			continue
		}
		if peer.Image != clientImageTag {
			t.Fatalf("%s substitutes client image %s", peer.Name, peer.Image)
		}
		if !slices.Contains(peer.Arguments, "/dev/ppp") {
			t.Fatalf("%s lacks real PPP device", peer.Name)
		}
	}
	if plan.Peers[1].Name == plan.Peers[2].Name {
		t.Fatal("clients share a peer selector")
	}
	if plan.Peers[1].Container == plan.Peers[2].Container {
		t.Fatal("clients share a container")
	}
	if plan.Peers[1].Host == plan.Peers[2].Host {
		t.Fatal("clients share an infrastructure address")
	}
}

// Readiness and termination must target the selected client's own PID file.
func TestSharedAccessPeerCommandsRemainIndependent(t *testing.T) {
	peers := sharedAccessPeers()
	lab := &fakeLab{}
	lab.detachedFn = func(string, []string) error { return nil }
	lab.execFn = func(string, []string) (interoplab.CommandResult, error) {
		return interoplab.CommandResult{Stdout: "19\n"}, nil
	}
	for _, peer := range peers {
		if err := sharedAccessDial(context.Background(), lab, peer); err != nil {
			t.Fatal(err)
		}
		if _, err := sharedAccessPPPCommand(context.Background(), lab, peer, 19, true); err != nil {
			t.Fatal(err)
		}
	}
	if len(lab.detachedCalls) != 2 {
		t.Fatal("did not launch both actual peer slots")
	}
	for i, peer := range peers {
		launch := lab.detachedCalls[i]
		if launch[0] != peer.name {
			t.Fatalf("launch selected %s, want %s", launch[0], peer.name)
		}
		joined := strings.Join(launch, " ")
		for _, value := range []string{peer.username, peer.device, peer.logPath(), sharedAccessDevice} {
			if !strings.Contains(joined, value) {
				t.Fatalf("launch lost own %q", value)
			}
		}
		command := lab.execCalls[i]
		if command[0] != peer.name {
			t.Fatalf("stop selected %s, want %s", command[0], peer.name)
		}
		if !slices.Contains(command, peer.pidPath()) {
			t.Fatal("stop lost original peer PID file")
		}
		if !slices.Contains(command, peer.device) {
			t.Fatal("stop lost original peer interface")
		}
		if strings.Contains(strings.Join(command, " "), "pkill") {
			t.Fatal("stop became a global process match")
		}
	}
}

// A missing peer or reused original row cannot stand in for the live survivor.
func TestSharedAccessRowsRequireExactSurvivor(t *testing.T) {
	first := sharedAccessRow{SID: 11, MAC: "02:00:00:00:00:11", Access: sharedAccessDevice,
		Service: pppoeService, State: "session", Device: "ppp11", Index: 21}
	second := sharedAccessRow{SID: 12, MAC: "02:00:00:00:00:12", Access: sharedAccessDevice,
		Service: pppoeService, State: "session", Device: "ppp12", Index: 22}
	if err := sharedAccessRequireSurvivor([]sharedAccessRow{second}, first, second); err != nil {
		t.Fatal(err)
	}
	for _, rows := range [][]sharedAccessRow{nil, {first}, {first, second}, {second, second}} {
		if err := sharedAccessRequireSurvivor(rows, first, second); err == nil {
			t.Fatalf("invalid survivor rows passed: %+v", rows)
		}
	}
	changed := second
	changed.Index++
	if err := sharedAccessRequireSurvivor([]sharedAccessRow{changed}, first, second); err == nil {
		t.Fatal("replacement native index passed")
	}
}

// Kernel parsing must observe the named link and actual negotiated addresses.
func TestSharedAccessLinkRejectsWrongClient(t *testing.T) {
	good := `[{"ifindex":31,"ifname":"ppp-a","address":"02:00:00:00:00:11","flags":["POINTOPOINT","UP"],"addr_info":[{"family":"inet","local":"10.20.0.2","address":"10.20.0.1","prefixlen":32}]}]`
	if _, err := sharedAccessReadPPP([]byte(good), "ppp-a", "10.20.0.1"); err != nil {
		t.Fatal(err)
	}
	for _, raw := range []string{"broken", "null", "[]", strings.ReplaceAll(good, "ppp-a", "ppp-b"), strings.ReplaceAll(good, "10.20.0.1", "192.0.2.1")} {
		if _, err := sharedAccessReadPPP([]byte(raw), "ppp-a", "10.20.0.1"); err == nil {
			t.Fatalf("wrong client accepted: %s", raw)
		}
	}
}

// Every discovery leg belongs to its own peer and the daemon's actual SID.
func TestSharedAccessDiscoveryNeedsBothActualClients(t *testing.T) {
	peers := sharedAccessPeers()
	clients := [2]sharedAccessClient{
		{peer: peers[0], vlan: sharedAccessVLANIdentity{11, "02:00:00:00:00:11"}, row: sharedAccessRow{SID: 11}},
		{peer: peers[1], vlan: sharedAccessVLANIdentity{12, "02:00:00:00:00:12"}, row: sharedAccessRow{SID: 12}},
	}
	ac := [6]byte{2, 0, 0, 0, 0, 1}
	frames := sharedAccessTestFrames(t, ac, &clients)
	if err := sharedAccessDiscoveryEvidence(buildCapture(t, frames...), net.HardwareAddr(ac[:]).String(), &clients); err != nil {
		t.Fatal(err)
	}
	for removed := range frames {
		kept := append(slices.Clone(frames[:removed]), frames[removed+1:]...)
		if err := sharedAccessDiscoveryEvidence(buildCapture(t, kept...), net.HardwareAddr(ac[:]).String(), &clients); err == nil {
			t.Fatalf("missing discovery leg %d passed", removed)
		}
	}
	wrong := clients
	wrong[1].row.SID = clients[0].row.SID
	if err := sharedAccessDiscoveryEvidence(buildCapture(t, frames...), net.HardwareAddr(ac[:]).String(), &wrong); err == nil {
		t.Fatal("other client's PADS SID passed")
	}
	if err := sharedAccessDiscoveryEvidence(buildCapture(t), net.HardwareAddr(ac[:]).String(), &clients); err == nil {
		t.Fatal("empty discovery passed")
	}
}

func sharedAccessTestFrames(t *testing.T, ac [6]byte, clients *[2]sharedAccessClient) [][]byte {
	t.Helper()
	var frames [][]byte
	for i := range clients {
		client := &clients[i]
		mac, err := net.ParseMAC(client.vlan.mac)
		if err != nil {
			t.Fatal(err)
		}
		peer := [6]byte(mac)
		for _, leg := range []struct {
			source, target [6]byte
			code           uint8
			sid            uint16
		}{
			{peer, [6]byte(discovery.BroadcastMAC), discovery.CodePADI, 0},
			{ac, peer, discovery.CodePADO, 0}, {peer, ac, discovery.CodePADR, 0}, {ac, peer, discovery.CodePADS, client.row.SID},
		} {
			var buf [discovery.EthMaxLen]byte
			builder := discovery.NewBuilder(buf[:], leg.source, leg.target, leg.code, leg.sid)
			frame := builder.Finish()
			if frame == nil {
				t.Fatal("test discovery builder failed")
			}
			frames = append(frames, frame)
		}
	}
	return frames
}

// Bound traffic must use the selected PPP route, never Docker's Ethernet path.
func TestSharedAccessRouteRequiresSelectedPPP(t *testing.T) {
	if err := sharedAccessRequireRoute([]byte(`[{"dev":"ppp-b"}]`), "ppp-b"); err != nil {
		t.Fatal(err)
	}
	for _, raw := range []string{`null`, `[]`, `[{"dev":"eth0"}]`, `[{"dev":"ppp-a"}]`} {
		if err := sharedAccessRequireRoute([]byte(raw), "ppp-b"); err == nil {
			t.Fatalf("wrong route passed: %s", raw)
		}
	}
}

// sessionJSON publishes StateSession as "session"; a mock-only "active" value
// must not make the native checker accept a contract the daemon never emits.
func TestSharedAccessRowsMatchProducerSessionState(t *testing.T) {
	producer := `[{"sid":17,"mac":"02:00:00:00:00:11","interface":"eth0.100","service-name":"internet","state":"session","unit":7,"ppp-interface":"ppp7","interface-index":42,"created-at":"2026-11-17T12:00:00Z"}]`
	read := func(raw string) ([]sharedAccessRow, error) {
		lab := &fakeLab{queryFn: func(string, []string) (string, error) {
			return `{"status":"done","data":` + raw + `}`, nil
		}}
		return sharedAccessRows(t.Context(), lab)
	}
	rows, err := read(producer)
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 1 {
		t.Fatal("producer-shaped session row was lost")
	}
	if rows[0].State != "session" {
		t.Fatal("daemon's actual StateSession spelling changed")
	}
	if rows[0].SID != 17 {
		t.Fatal("producer SID was changed")
	}
	if rows[0].Device != "ppp7" {
		t.Fatal("producer native PPP device was changed")
	}
	if rows[0].Index != 42 {
		t.Fatal("producer native index was changed")
	}
	for _, state := range []string{"active", "discovery", "teardown"} {
		raw := strings.Replace(producer, `"state":"session"`, `"state":"`+state+`"`, 1)
		if _, err := read(raw); err == nil {
			t.Fatalf("non-session producer state %q passed", state)
		}
	}
}
