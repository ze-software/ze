package grantcheck

import (
	"slices"
	"strings"
	"testing"

	"github.com/ze-software/ze/internal/le/interoplab"
)

// VALIDATES: Parse reads every flag spelling Docker reads (--flag value,
// --flag=value, -v/:/host, -v=/:/host, combined short booleans) and the
// declared capabilities, stops at the image, and Problems names every extra
// grant, including the ones a presence check misses (ALL,
// seccomp=unconfined, --privileged in any true spelling, a host namespace,
// the host network, a device cgroup rule, a volume, --volumes-from, --gpus,
// --runtime, --cgroup-parent, --user, --group-add, a lab peer's AppArmor
// profile and a mount that is writable or comes from outside the lab's
// staged files), and every flag nobody classified.
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
		"joined short volume": {"--device", "/dev/ppp", "-v/:/host"},
		"short volume equals": {"--device", "/dev/ppp", "-v=/:/host"},
		"named short volume":  {"--device", "/dev/ppp", "-vX"},
		"combined short":      {"--device", "/dev/ppp", "-itv/:/host"},
		"volumes from":        {"--device", "/dev/ppp", "--volumes-from", "other"},
		"gpus":                {"--device", "/dev/ppp", "--gpus=all"},
		"runtime":             {"--device", "/dev/ppp", "--runtime", "runc"},
		"cgroup parent":       {"--device", "/dev/ppp", "--cgroup-parent=/"},
		"user":                {"--device", "/dev/ppp", "--user", "0"},
		"short user":          {"--device", "/dev/ppp", "-u0"},
		"group add":           {"--device", "/dev/ppp", "--group-add=disk"},
		"unclassified long":   {"--device", "/dev/ppp", "--oom-kill-disable"},
		"unclassified short":  {"--device", "/dev/ppp", "-z"},
		"unclassified joined": {"--device", "/dev/ppp", "--shm-size=1g"},
		"value missing":       {"--device", "/dev/ppp", "--cap-add"},
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

	// Known non-grant flags pass, in every spelling, and nothing after the
	// image is read as a docker flag: it is the container's own command.
	quiet := Parse([]string{"NET_ADMIN"}, []string{
		"run", "--rm", "-it", "--name=peer", "--platform", "linux/amd64", "-e", "A=1", "-w/src",
		"--device", "/dev/ppp", "image:tag", "bash", "-lc", "--privileged -v /:/host",
	})
	if problems := Problems(quiet, want); len(problems) != 0 {
		t.Errorf("classified non-grant flags and a container command reported %v", problems)
	}
	if got := Parse(nil, []string{"-v/:/host"}); !slices.Equal(got.Mounts, []string{"/:/host"}) {
		t.Errorf("joined -v read as %v", got.Mounts)
	}
	if got := Parse(nil, []string{"--shm-size=1g"}); !slices.Equal(got.Unclassified, []string{"--shm-size=1g"}) {
		t.Errorf("an unclassified flag read as %v", got.Unclassified)
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
