package interoplab

import (
	"errors"
	"net/netip"
	"slices"
	"strings"
	"testing"
)

// linkPeers is a scenario's planned peers for the link tests: two members at
// hosts 2 and 3, and a peer at the broadcast host.
func linkPeers() []PeerConfig {
	return []PeerConfig{{Name: "a", Host: 2}, {Name: "b", Host: 3}, {Name: "far", Host: 255}}
}

// VALIDATES: ValidateLinks refuses every link the harness cannot build as
// declared, before any container starts, with a message naming the link,
// the fault and the fix (AC-4), and accepts each boundary's last valid value.
// Method: one declaration per refusal, then one per accepted boundary.
// PREVENTS: a link that carries nothing being started, so a silent lab reads
// as the daemon's fault.
func TestLinkValidateRefusals(t *testing.T) {
	v4 := netip.MustParsePrefix("10.101.0.0/24")
	v6 := netip.MustParsePrefix("fd00:101::/64")
	both := []string{"a", "b"}
	for _, tc := range []struct {
		name  string
		links []Link
		want  string
	}{
		{"no name", []Link{{VLAN: 101, IPv4: v4, Members: both}}, "has no name"},
		{"vlan zero", []Link{{Name: "l", VLAN: 0, IPv4: v4, Members: both}}, "not a usable VLAN; declare one from 1 to 4094"},
		{"vlan 4095", []Link{{Name: "l", VLAN: 4095, IPv4: v4, Members: both}}, "not a usable VLAN; declare one from 1 to 4094"},
		{"vlan reused", []Link{{Name: "l", VLAN: 101, Members: both}, {Name: "m", VLAN: 101, Members: both}}, "link m on VLAN 101 reuses the VLAN of link l"},
		{"ipv4 family", []Link{{Name: "l", VLAN: 101, IPv4: v6, Members: both}}, "write an IPv4 network"},
		{"ipv4 host", []Link{{Name: "l", VLAN: 101, IPv4: netip.MustParsePrefix("10.101.0.1/24"), Members: both}}, "write 10.101.0.0/24"},
		{"ipv4 /25", []Link{{Name: "l", VLAN: 101, IPv4: netip.MustParsePrefix("10.101.0.0/25"), Members: both}}, "longer than a /24"},
		{"ipv6 family", []Link{{Name: "l", VLAN: 101, IPv6: v4, Members: both}}, "write an IPv6 /64"},
		{"ipv6 /63", []Link{{Name: "l", VLAN: 101, IPv6: netip.MustParsePrefix("fd00:100::/63"), Members: both}}, "not a /64"},
		{"ipv6 /65", []Link{{Name: "l", VLAN: 101, IPv6: netip.MustParsePrefix("fd00:101::/65"), Members: both}}, "not a /64"},
		{"ipv6 host", []Link{{Name: "l", VLAN: 101, IPv6: netip.MustParsePrefix("fd00:101::1/64"), Members: both}}, "write fd00:101::/64"},
		{"one member", []Link{{Name: "l", VLAN: 101, Members: []string{"a"}}}, "fewer than two members"},
		{"member twice", []Link{{Name: "l", VLAN: 101, Members: []string{"a", "a"}}}, "names a twice; name each member once"},
		{"member not planned", []Link{{Name: "l", VLAN: 101, Members: []string{"a", "ghost"}}}, "names ghost, and the scenario does not plan that peer"},
		{"broadcast host", []Link{{Name: "l", VLAN: 101, Members: []string{"a", "far"}}}, "host number 255 is the /24 broadcast"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			err := ValidateLinks("fixture", tc.links, linkPeers())
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("error = %v, want it to contain %q", err, tc.want)
			}
			if !strings.HasPrefix(err.Error(), "fixture's link ") {
				t.Fatalf("error = %v, want it to name the scenario and the link", err)
			}
		})
	}
	for _, tc := range []struct {
		name  string
		links []Link
	}{
		{"vlan 1", []Link{{Name: "l", VLAN: 1, Members: both}}},
		{"vlan 4094", []Link{{Name: "l", VLAN: 4094, Members: both}}},
		{"ipv4 /24 and ipv6 /64", []Link{{Name: "l", VLAN: 101, IPv4: v4, IPv6: v6, Members: both}}},
		{"layer 2 only", []Link{{Name: "l", VLAN: 101, Members: both}}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if err := ValidateLinks("fixture", tc.links, linkPeers()); err != nil {
				t.Fatal(err)
			}
		})
	}
}

// VALIDATES: a member reaches its Nth link as eth<N>, counting only the links
// it joins in declaration order; it holds its host number in the last octet
// of the IPv4 prefix and of the IPv6 /64 (nodad); and a link with no prefix
// brings the device up with no address (AC-1, AC-2, AC-3).
// Method: three links, of which member b joins the second and third, and
// read the rendered script line by line.
// PREVENTS: a member addressed on a link it does not join, an ordinal that
// counts other members' links, and an address on a pure layer-2 segment.
func TestLinkRenderDevicesAndAddresses(t *testing.T) {
	links := []Link{
		{Name: "lan", VLAN: 10, IPv4: netip.MustParsePrefix("10.10.0.0/24"), Members: []string{"a", "c"}},
		{Name: "core", VLAN: 20, IPv4: netip.MustParsePrefix("10.20.0.0/16"), IPv6: netip.MustParsePrefix("fd00:20::/64"), Members: []string{"a", "b"}},
		{Name: "access", VLAN: 30, Members: []string{"b", "a"}},
	}
	memberships := LinkMemberships(links, "b", 7)
	want := []LinkMembership{
		{Link: "core", Device: "eth1", IPv4: netip.MustParseAddr("10.20.0.7"), IPv6: netip.MustParseAddr("fd00:20::7")},
		{Link: "access", Device: "eth2"},
	}
	if !slices.Equal(memberships, want) {
		t.Fatalf("memberships = %+v, want %+v", memberships, want)
	}
	script := linkScript(links, memberships)
	wantScript := "ip link add link eth0 name eth1 type vlan id 20\n" +
		"ip address add 10.20.0.7/16 dev eth1\n" +
		"ip -6 address add fd00:20::7/64 dev eth1 nodad\n" +
		"ip link set eth1 up\n" +
		"ip link add link eth0 name eth2 type vlan id 30\n" +
		"ip link set eth2 up\n"
	if script != wantScript {
		t.Fatalf("script =\n%s\nwant\n%s", script, wantScript)
	}
	if got := LinkMemberships(links, "a", 2); len(got) != 3 || got[2].Device != "eth3" || got[0].IPv4 != netip.MustParseAddr("10.10.0.2") {
		t.Fatalf("a's memberships = %+v, want lan, core and access as eth1 to eth3", got)
	}
	if got := LinkMemberships(links, "outsider", 9); len(got) != 0 {
		t.Fatalf("a peer on no link holds %+v", got)
	}
}

// VALIDATES: the one start wrapper runs the links first, then the peer's
// Prestart, then execs the start Docker would have run: the image's exec-form
// entrypoint and command, the peer's command in place of the image's, and a
// peer --entrypoint (both spellings) replacing the image's entrypoint and
// dropping its command, as Docker does.
// Method: compose four peers against fixture image starts and compare argv.
// PREVENTS: a daemon started with the wrong argv, a Prestart that runs before
// the device it configures exists, and a re-quoted entrypoint.
func TestStartWrapperComposesLinksPrestartEntrypoint(t *testing.T) {
	links := []Link{{Name: "l", VLAN: 101, IPv4: netip.MustParsePrefix("10.101.0.0/24"), Members: []string{"a", "b"}}}
	shellForm := ImageStart{Entrypoint: []string{"/bin/sh", "-c", "touch /run/x && exec daemon --flag 'quoted arg'"}}
	execForm := ImageStart{Entrypoint: []string{"/sbin/tini", "--"}, Command: []string{"/usr/lib/frr/docker-start"}}
	wantScript := "ip link add link eth0 name eth1 type vlan id 101\n" +
		"ip address add 10.101.0.2/24 dev eth1\n" +
		"ip link set eth1 up\n" +
		"ip route add 10.9.0.0/24 via 10.101.0.3\n" +
		`exec "$@"`
	for _, tc := range []struct {
		name          string
		peer          PeerConfig
		start         ImageStart
		wantArguments []string
		wantTail      []string
	}{
		{"exec form", PeerConfig{Name: "a", Host: 2, Prestart: "ip route add 10.9.0.0/24 via 10.101.0.3\n",
			Arguments: []string{"--sysctl", "x=1"}}, execForm,
			[]string{"--sysctl", "x=1", "--entrypoint", "/bin/sh"}, []string{"/sbin/tini", "--", "/usr/lib/frr/docker-start"}},
		{"peer command", PeerConfig{Name: "a", Host: 2, Prestart: "ip route add 10.9.0.0/24 via 10.101.0.3",
			Command: []string{"start", "/etc/ze/ze.conf"}}, ImageStart{Entrypoint: []string{"tini", "--", "ze"}},
			[]string{"--entrypoint", "/bin/sh"}, []string{"tini", "--", "ze", "start", "/etc/ze/ze.conf"}},
		{"shell form", PeerConfig{Name: "a", Host: 2, Prestart: "ip route add 10.9.0.0/24 via 10.101.0.3"}, shellForm,
			[]string{"--entrypoint", "/bin/sh"}, shellForm.Entrypoint},
		{"peer entrypoint", PeerConfig{Name: "a", Host: 2, Prestart: "ip route add 10.9.0.0/24 via 10.101.0.3",
			Arguments: []string{"--entrypoint", "/usr/bin/own"}}, execForm,
			[]string{"--entrypoint", "/bin/sh"}, []string{"/usr/bin/own"}},
		{"peer entrypoint equals", PeerConfig{Name: "a", Host: 2, Prestart: "ip route add 10.9.0.0/24 via 10.101.0.3",
			Arguments: []string{"--entrypoint=/usr/bin/own"}, Command: []string{"-v"}}, execForm,
			[]string{"--entrypoint", "/bin/sh"}, []string{"/usr/bin/own", "-v"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			composed := ComposeStart(tc.peer, links, tc.start)
			if !slices.Equal(composed.Arguments, tc.wantArguments) {
				t.Fatalf("arguments = %q, want %q", composed.Arguments, tc.wantArguments)
			}
			want := append([]string{"-ec", wantScript, "--"}, tc.wantTail...)
			if !slices.Equal(composed.Command, want) {
				t.Fatalf("command = %q, want %q", composed.Command, want)
			}
			if composed.Prestart != "" {
				t.Fatalf("composed peer still carries Prestart %q, so a second compose would run it twice", composed.Prestart)
			}
		})
	}
}

// VALIDATES: the image start is read from `docker image inspect`'s Config in
// exec form, and an image with neither ENTRYPOINT nor CMD, or an unreadable
// answer, is refused naming the image (A-2).
// Method: parse fixture answers.
// PREVENTS: a wrapper that execs nothing and exits, read as the daemon's fault.
func TestParseImageStart(t *testing.T) {
	start, err := parseImageStart("ze-interop", `{"Entrypoint":["tini","--","ze"],"Cmd":null,"Env":["PATH=/bin"]}`+"\n")
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(start.Entrypoint, []string{"tini", "--", "ze"}) || len(start.Command) != 0 {
		t.Fatalf("start = %+v", start)
	}
	for _, answer := range []string{`{"Entrypoint":null,"Cmd":null}`, `not json`} {
		if _, err := parseImageStart("bare", answer); err == nil || !strings.Contains(err.Error(), "image bare") {
			t.Fatalf("answer %q: error = %v, want a refusal naming the image", answer, err)
		}
	}
}

// VALIDATES: a link member's image that lacks sh or ip is refused naming the
// image and the Dockerfile to fix, or the pulled tag when it has none (AC-5).
// Method: render both refusals.
// PREVENTS: a refusal that leaves the operator guessing which file to edit.
func TestLinkToolRefusalNamesTheDockerfile(t *testing.T) {
	built := linkToolRefusal("ze-interop", "ze-a", ImageBuild{Name: "ze", Tag: "ze-interop", Dockerfile: "/r/test/interop/Dockerfile.ze"}, errExample)
	if !strings.Contains(built.Error(), "install iproute2 in /r/test/interop/Dockerfile.ze") || !strings.Contains(built.Error(), "ze-a") {
		t.Fatalf("refusal = %v", built)
	}
	pulled := linkToolRefusal("frr:10", "frr", ImageBuild{Name: "frr", Tag: "frr:10"}, errExample)
	if !strings.Contains(pulled.Error(), "build frr:10 from a Dockerfile that installs iproute2") {
		t.Fatalf("refusal = %v", pulled)
	}
}

var errExample = errors.New("exit status 127")

// VALIDATES: a link may have one member when a planned peer that is not a
// member terminates it in its own stack (External), and is refused when that
// end is missing, not planned, also a member, or the only end. The external
// end joins no wrapper and holds nothing on the link.
// Method: one declaration per refusal, one accepted declaration, and the
// memberships of both ends of the accepted one.
// PREVENTS: a one-sided link to freeRouter refused as having fewer than two
// members, and an external end that silently gets a device it never reads.
func TestLinkExternalEnd(t *testing.T) {
	v4 := netip.MustParsePrefix("10.0.14.0/24")
	for _, tc := range []struct {
		name string
		link Link
		want string
	}{
		{"one member, no external end", Link{Name: "l", VLAN: 100, Members: []string{"a"}}, "fewer than two members"},
		{"external end not planned", Link{Name: "l", VLAN: 100, Members: []string{"a"}, External: "ghost"},
			"names ghost as its external end, and the scenario does not plan that peer"},
		{"external end also a member", Link{Name: "l", VLAN: 100, Members: []string{"a", "b"}, External: "b"},
			"names b both as a member and as its external end"},
		{"external end alone", Link{Name: "l", VLAN: 100, External: "b"}, "has an external end and no member"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			err := ValidateLinks("fixture", []Link{tc.link}, linkPeers())
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("error = %v, want it to contain %q", err, tc.want)
			}
		})
	}
	links := []Link{{Name: "protected", VLAN: 100, IPv4: v4, Members: []string{"a"}, External: "b"}}
	if err := ValidateLinks("fixture", links, linkPeers()); err != nil {
		t.Fatal(err)
	}
	want := []LinkMembership{{Link: "protected", Device: "eth1", IPv4: netip.MustParseAddr("10.0.14.2")}}
	if got := LinkMemberships(links, "a", 2); !slices.Equal(got, want) {
		t.Fatalf("member's memberships = %+v, want %+v", got, want)
	}
	if got := LinkMemberships(links, "b", 3); len(got) != 0 {
		t.Fatalf("the external end holds %+v, want nothing", got)
	}
}

// VALIDATES: Devices gives a member the device name a file outside the
// harness already names, on that link only; the member's other links keep
// eth<N> numbered with no gap, and the rendered script builds the named
// device with the link's VLAN and the member's address.
// Method: member a joins three links, the second under the name prot0, and
// read its memberships and script.
// PREVENTS: a checker or daemon configuration naming a device the harness
// built under another name, and an override that shifts or collides with
// the numbered devices.
func TestLinkDeviceNames(t *testing.T) {
	links := []Link{
		{Name: "lan", VLAN: 10, Members: []string{"a", "b"}},
		{Name: "protected", VLAN: 100, IPv4: netip.MustParsePrefix("10.0.14.0/24"), Members: []string{"a", "b"},
			Devices: map[string]string{"a": "prot0"}},
		{Name: "core", VLAN: 20, Members: []string{"a", "b"}},
	}
	if err := ValidateLinks("fixture", links, linkPeers()); err != nil {
		t.Fatal(err)
	}
	memberships := LinkMemberships(links, "a", 2)
	want := []LinkMembership{
		{Link: "lan", Device: "eth1"},
		{Link: "protected", Device: "prot0", IPv4: netip.MustParseAddr("10.0.14.2")},
		{Link: "core", Device: "eth2"},
	}
	if !slices.Equal(memberships, want) {
		t.Fatalf("a's memberships = %+v, want %+v", memberships, want)
	}
	if got := LinkMemberships(links, "b", 3); len(got) != 3 || got[1].Device != "eth2" {
		t.Fatalf("b's memberships = %+v, want the protected link as its own eth2", got)
	}
	script := linkScript(links, memberships)
	wantProtected := "ip link add link eth0 name prot0 type vlan id 100\n" +
		"ip address add 10.0.14.2/24 dev prot0\n" +
		"ip link set prot0 up\n"
	if !strings.Contains(script, wantProtected) {
		t.Fatalf("script =\n%s\nwant it to build\n%s", script, wantProtected)
	}
}

// VALIDATES: ValidateLinks refuses every device name a member cannot hold:
// one for a peer that is not a member, a name Linux or the shell refuses, a
// name the harness numbers or the container holds, and a name the member
// holds on another link; and accepts the boundary names.
// Method: one declaration per refusal, then one per accepted name.
// PREVENTS: a container whose wrapper fails at `ip link add`, a name the
// shell splits, and two links of one member built as one device.
func TestLinkDeviceNameRefusals(t *testing.T) {
	both := []string{"a", "b"}
	named := func(device string) []Link {
		return []Link{{Name: "l", VLAN: 101, Members: both, Devices: map[string]string{"a": device}}}
	}
	for _, tc := range []struct {
		name  string
		links []Link
		want  string
	}{
		{"not a member", []Link{{Name: "l", VLAN: 101, Members: both, Devices: map[string]string{"far": "prot0"}}},
			"names device prot0 for far, which is not a member"},
		{"empty", named(""), "a device name is 1 to 15 lowercase letters and digits"},
		{"sixteen octets", named("abcdefghijklmnop"), "a device name is 1 to 15 lowercase letters and digits"},
		{"uppercase", named("Prot0"), "a device name is 1 to 15 lowercase letters and digits"},
		{"shell word", named("prot0;reboot"), "a device name is 1 to 15 lowercase letters and digits"},
		{"eth0", named("eth0"), "eth<N> is the name the harness numbers"},
		{"eth3", named("eth3"), "eth<N> is the name the harness numbers"},
		{"loopback", named("lo"), "every container holds lo already"},
		{"twice on one member", []Link{
			{Name: "l", VLAN: 101, Members: both, Devices: map[string]string{"a": "prot0"}},
			{Name: "m", VLAN: 102, Members: both, Devices: map[string]string{"a": "prot0"}},
		}, "names device prot0 for a, which a holds on link l already"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			err := ValidateLinks("fixture", tc.links, linkPeers())
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("error = %v, want it to contain %q", err, tc.want)
			}
			if !strings.HasPrefix(err.Error(), "fixture's link ") {
				t.Fatalf("error = %v, want it to name the scenario and the link", err)
			}
		})
	}
	for _, device := range []string{"p", "abcdefghijklmno", "eth", "ethx1", "prot0"} {
		t.Run("accepts "+device, func(t *testing.T) {
			if err := ValidateLinks("fixture", named(device), linkPeers()); err != nil {
				t.Fatal(err)
			}
		})
	}
	twoMembers := []Link{
		{Name: "l", VLAN: 101, Members: both, Devices: map[string]string{"a": "prot0"}},
		{Name: "m", VLAN: 102, Members: both, Devices: map[string]string{"b": "prot0"}},
	}
	if err := ValidateLinks("fixture", twoMembers, linkPeers()); err != nil {
		t.Fatalf("one name on two members' own containers: %v", err)
	}
}
