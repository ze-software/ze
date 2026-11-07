package pppoe

import (
	"context"
	"errors"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/ze-software/ze/internal/le/interoplab"
)

// namedDeviceLab scripts the Ze container and accel-ppp for
// checkNamedDeviceReconnect: two sessions, the second created after accel-ppp
// terminates the first, each under the device name the case gives it.
type namedDeviceLab struct {
	*fakeLab
	// device is the name the kernel gives the client's PPP unit.
	device string
	// extraLink is a second PPP device Ze holds beside the session, or empty.
	extraLink string
	// presentAtStart has the session device up before the checker removes
	// Docker's default route.
	presentAtStart bool
	// sameIndex keeps the first session's ifindex after the reconnect.
	sameIndex bool
	// routeDevice is the device the probe target routes through.
	routeDevice string
	// counterFrozen keeps the masquerade counter where it is.
	counterFrozen bool

	// linkReads counts PPP link listings: the first one, right after the
	// checker removed Docker's default route, finds no session yet.
	linkReads  int
	terminated bool
	counter    int
	pings      int
}

func newNamedDeviceLab() *namedDeviceLab {
	lab := &namedDeviceLab{fakeLab: &fakeLab{}, device: namedDevice, routeDevice: namedDevice}
	lab.execFn = lab.exec
	lab.queryFn = lab.query
	return lab
}

func (l *namedDeviceLab) index() int {
	if l.terminated && !l.sameIndex {
		return 21
	}
	return 20
}

func (l *namedDeviceLab) linkLines() string {
	l.linkReads++
	if l.linkReads == 1 && !l.presentAtStart {
		return ""
	}
	lines := strconv.Itoa(l.index()) + ": " + l.device + ": <POINTOPOINT,UP> mtu 1492\n"
	if l.extraLink != "" {
		lines += "30: " + l.extraLink + ": <POINTOPOINT,UP> mtu 1492\n"
	}
	return lines
}

func (l *namedDeviceLab) exec(peer string, argv []string) (interoplab.CommandResult, error) {
	command := peer + " " + strings.Join(argv, " ")
	switch command {
	case "ze ip route del default dev eth0":
		return interoplab.CommandResult{}, nil
	case "accel ip addr add 192.0.2.1/32 dev lo":
		return interoplab.CommandResult{}, nil
	case "ze ip -o link show type ppp":
		return interoplab.CommandResult{Stdout: l.linkLines()}, nil
	case "ze ip -o addr show dev " + l.device:
		local := "10.11.0.2"
		if l.terminated {
			local = "10.11.0.3"
		}
		return interoplab.CommandResult{Stdout: "20: " + l.device + " inet " + local + " peer 10.11.0.1/32\n"}, nil
	case "ze ip -o route show default proto 250":
		return interoplab.CommandResult{Stdout: "default dev " + l.device + " scope link\n"}, nil
	case "ze ip -o route get 192.0.2.1":
		return interoplab.CommandResult{Stdout: "192.0.2.1 dev " + l.routeDevice + " src 10.11.0.2\n"}, nil
	case "ze ping -c 3 -W 3 -I 198.51.100.1 192.0.2.1":
		l.pings++
		if !l.counterFrozen {
			l.counter++
		}
		return interoplab.CommandResult{}, nil
	case "accel accel-cmd terminate username alice":
		l.terminated = true
		return interoplab.CommandResult{}, nil
	default:
		return interoplab.CommandResult{}, errors.New("unexpected command: " + command)
	}
}

func (l *namedDeviceLab) query(peer string, argv []string) (string, error) {
	command := peer + " " + strings.Join(argv, " ")
	switch command {
	case "ze ip -o link show dev pppoe0":
		return strconv.Itoa(l.index()) + ": pppoe0: <POINTOPOINT,UP> mtu 1492\n", nil
	case "ze nft list chain ip ze_cpenat postrouting":
		return "table ip ze_cpenat {\n\tchain postrouting {\n\t\toifname \"pppoe0\" counter packets " +
			strconv.Itoa(l.counter) + " bytes 84 masquerade\n\t}\n}\n", nil
	case "accel accel-cmd show sessions":
		return "ifname | username\nppp0 | alice\n", nil
	default:
		return "", errors.New("unexpected query: " + command)
	}
}

// TestNamedDeviceReconnectAcceptsTheGuideCPE drives the checker over a lab in
// which every claim holds and requires it to measure both sessions: two
// masqueraded pings, the second after accel-ppp ended the first.
func TestNamedDeviceReconnectAcceptsTheGuideCPE(t *testing.T) {
	lab := newNamedDeviceLab()
	if err := checkNamedDeviceReconnect(context.Background(), &interoplab.CheckContext{Lab: lab}); err != nil {
		t.Fatalf("check: %v", err)
	}
	if !lab.terminated {
		t.Fatal("the checker never asked accel-ppp to end the first session")
	}
	if lab.pings != 2 {
		t.Fatalf("masqueraded pings = %d, want one per session", lab.pings)
	}
}

// TestNamedDeviceReconnectRefusesEachBrokenClaim breaks one claim at a time
// and requires the checker to name the stage that broke. The kernel-named
// device and the reused ifindex never become ready, so those cases run under a
// short deadline in place of the checker's 75-second wait.
func TestNamedDeviceReconnectRefusesEachBrokenClaim(t *testing.T) {
	cases := []struct {
		name   string
		breaks func(*namedDeviceLab)
		want   string
	}{
		{"kernel names the device pppN", func(l *namedDeviceLab) { l.device = "ppp0" }, "Ze PPP links: [ppp0]"},
		{"a second PPP device beside the session", func(l *namedDeviceLab) { l.extraLink = "ppp1" }, "only PPP device"},
		{"session device up before the Docker route went", func(l *namedDeviceLab) { l.presentAtStart = true }, "before the Docker default route was removed"},
		{"probe target not routed through the session", func(l *namedDeviceLab) { l.routeDevice = "eth0" }, "does not route through pppoe0"},
		{"masquerade counter does not rise", func(l *namedDeviceLab) { l.counterFrozen = true }, "did not rise"},
		{"reconnect keeps the old device", func(l *namedDeviceLab) { l.sameIndex = true }, "session after accel-ppp terminated the first"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			lab := newNamedDeviceLab()
			tc.breaks(lab)
			ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
			defer cancel()
			err := checkNamedDeviceReconnect(ctx, &interoplab.CheckContext{Lab: lab})
			if err == nil {
				t.Fatal("checker accepted a broken claim")
			}
			if !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("error = %v, want it to contain %q", err, tc.want)
			}
		})
	}
}
