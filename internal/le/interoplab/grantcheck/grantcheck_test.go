package grantcheck

import (
	"slices"
	"strings"
	"testing"
)

// VALIDATES: Parse reads both flag spellings and the declared capabilities,
// and Problems names every extra grant, including the ones a presence check
// misses (ALL, seccomp=unconfined, --privileged, a host namespace).
// PREVENTS: the exact-set check passing a container that holds more than its
// daemons need.
func TestProblemsCatchEveryExtraGrant(t *testing.T) {
	want := Grants{Capabilities: []string{"NET_ADMIN"}, Devices: []string{"/dev/ppp"}}
	clean := Parse([]string{"NET_ADMIN"}, []string{"--device", "/dev/ppp", "--network", "lab"})
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
	} {
		got := Parse([]string{"NET_ADMIN"}, argv)
		if problems := Problems(got, want); len(problems) == 0 {
			t.Errorf("%s: %v passed the exact check", name, argv)
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
