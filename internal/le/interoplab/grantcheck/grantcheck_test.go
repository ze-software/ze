package grantcheck

import (
	"slices"
	"strings"
	"testing"

	"github.com/ze-software/ze/internal/le/interoplab"
)

// VALIDATES: Parse reads both flag spellings and the declared capabilities,
// and Problems names every extra grant, including the ones a presence check
// misses (ALL, seccomp=unconfined, --privileged in any true spelling, a host
// namespace, the host network, a device cgroup rule, a volume, a lab peer's
// AppArmor profile and a mount that is writable or comes from outside the
// lab's staged files).
// PREVENTS: the exact-set check passing a container that holds more than its
// daemons need.
func TestProblemsCatchEveryExtraGrant(t *testing.T) {
	want := Grants{Capabilities: []string{"NET_ADMIN"}, Devices: []string{"/dev/ppp"}}
	clean := Parse([]string{"NET_ADMIN"}, []string{"--device", "/dev/ppp", "--privileged=false"})
	if problems := Problems(clean, want); len(problems) != 0 {
		t.Fatalf("an exact grant set reported %v", problems)
	}

	for name, argv := range map[string][]string{
		"cap ALL":             {"--device", "/dev/ppp", "--cap-add", "ALL"},
		"joined cap":          {"--device=/dev/ppp", "--cap-add=SYS_ADMIN"},
		"seccomp unconfined":  {"--device", "/dev/ppp", "--security-opt", "seccomp=unconfined"},
		"apparmor unconfined": {"--device", "/dev/ppp", "--security-opt=apparmor=unconfined"},
		"privileged":          {"--device", "/dev/ppp", "--privileged"},
		"extra device":        {"--device", "/dev/ppp", "--device", "/dev/net/tun"},
		"host pid":            {"--device", "/dev/ppp", "--pid", "host"},
		"missing device":      {},
		"privileged=1":        {"--device", "/dev/ppp", "--privileged=1"},
		"privileged typo":     {"--device", "/dev/ppp", "--privileged=yes"},
		"host network":        {"--device", "/dev/ppp", "--network", "host"},
		"joined net":          {"--device", "/dev/ppp", "--net=host"},
		"device cgroup rule":  {"--device", "/dev/ppp", "--device-cgroup-rule", "c *:* rwm"},
		"docker socket":       {"--device", "/dev/ppp", "-v", "/var/run/docker.sock:/var/run/docker.sock"},
		"volume root":         {"--device", "/dev/ppp", "--volume=/:/host"},
		"mount":               {"--device", "/dev/ppp", "--mount", "type=bind,source=/,target=/host"},
	} {
		got := Parse([]string{"NET_ADMIN"}, argv)
		if problems := Problems(got, want); len(problems) == 0 {
			t.Errorf("%s: %v passed the exact check", name, argv)
		}
	}

	peer := interoplab.PeerConfig{
		Capabilities: []string{"NET_ADMIN"},
		Mounts:       []interoplab.Mount{{Source: "/lab/scenario/ze.conf", Target: "/etc/ze/ze.conf", ReadOnly: true}},
	}
	if problems := Problems(ParsePeer(peer, "/lab"), Grants{Capabilities: []string{"NET_ADMIN"}}); len(problems) != 0 {
		t.Fatalf("a read-only staged config mount reported %v", problems)
	}
	for name, change := range map[string]func(*interoplab.PeerConfig){
		"apparmor profile": func(p *interoplab.PeerConfig) { p.AppArmorProfile = "ze-kernel-probe" },
		"writable mount":   func(p *interoplab.PeerConfig) { p.Mounts[0].ReadOnly = false },
		"outside the lab": func(p *interoplab.PeerConfig) {
			p.Mounts = append(p.Mounts, interoplab.Mount{Source: "/var/run/docker.sock", Target: "/s", ReadOnly: true})
		},
		"whole lab root": func(p *interoplab.PeerConfig) {
			p.Mounts = append(p.Mounts, interoplab.Mount{Source: "/lab", Target: "/l", ReadOnly: true})
		},
		"escapes the root": func(p *interoplab.PeerConfig) { p.Mounts[0].Source = "/lab/../etc/shadow" },
	} {
		changed := peer
		changed.Mounts = slices.Clone(peer.Mounts)
		change(&changed)
		if problems := Problems(ParsePeer(changed, "/lab"), Grants{Capabilities: []string{"NET_ADMIN"}}); len(problems) == 0 {
			t.Errorf("%s: %+v passed the exact check", name, changed)
		}
	}

	got := Parse(nil, []string{"--security-opt=systempaths=unconfined"})
	if !slices.Equal(got.SecurityOptions, []string{"systempaths=unconfined"}) {
		t.Errorf("joined --security-opt read as %v", got.SecurityOptions)
	}
	problems := Problems(Parse(nil, []string{"--privileged"}), Grants{})
	if len(problems) != 1 || !strings.Contains(problems[0], "--privileged") {
		t.Errorf("--privileged reported as %v", problems)
	}
}
