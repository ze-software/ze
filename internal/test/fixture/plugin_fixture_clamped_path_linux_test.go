//go:build linux

package fixture

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

// TestParseNetnsRunArgsPeers proves the clamped-path grammar keeps each peer
// in argument order with its namespace, script and wait file, sets the IPv6
// plane, and still hands the rest of the line to `run`. Method: parse one line
// carrying every new keyword and compare the plan field by field.
func TestParseNetnsRunArgsPeers(t *testing.T) {
	args := strings.Fields("netns zell ipv6 peer far receive.peer " +
		"peer-after far.up router inject.peer run ze start ze.conf")
	plan, err := parseNetnsRunArgs(args)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if !plan.ipv6 {
		t.Error("ipv6 not set")
	}
	want := []netnsPeer{
		{namespace: "far", script: "receive.peer"},
		{namespace: "router", script: "inject.peer", after: "far.up"},
	}
	if !slices.Equal(plan.peers, want) {
		t.Errorf("peers = %+v, want %+v", plan.peers, want)
	}
	if got := strings.Join(plan.command, " "); got != "ze start ze.conf" {
		t.Errorf("command = %q", got)
	}
}

// TestParseNetnsRunArgsRefusesBadPeers proves every malformed peer is refused
// at parse time, before a namespace exists: an unknown namespace, and each
// keyword cut short. Method: one table row per shape, each MUST error and name
// what it expected.
func TestParseNetnsRunArgsRefusesBadPeers(t *testing.T) {
	for _, tc := range []struct {
		line, want string
	}{
		{"netns x peer elsewhere a.peer run ze", "expected sender, router or far"},
		{"netns x peer far", "peer takes a namespace and a script"},
		{"netns x peer-after f.up far", "peer-after takes a file, a namespace and a script"},
		{"netns x peer-after f.up nowhere a.peer run ze", "expected sender, router or far"},
	} {
		_, err := parseNetnsRunArgs(strings.Fields(tc.line))
		if err == nil {
			t.Errorf("%q: parsed, want an error naming %q", tc.line, tc.want)
			continue
		}
		if !strings.Contains(err.Error(), tc.want) {
			t.Errorf("%q: error %q does not name %q", tc.line, err, tc.want)
		}
	}
}

// TestFarDaemonConfigOwnDirectory proves two far-daemon configurations written
// side by side, as a .ci's tmpfs files always are, each start from a directory
// of their own. `ze start <file>` keeps its exclusively owned live store beside
// the file, so a shared directory lets only one daemon start. Method: prepare
// two files from one directory and compare the directories and the bodies.
func TestFarDaemonConfigOwnDirectory(t *testing.T) {
	work := t.TempDir()
	bodies := map[string]string{"far-b.conf": "vpn { b }\n", "far-c.conf": "vpn { c }\n"}
	dirs := map[string]bool{work: true}
	for name, body := range bodies {
		source := filepath.Join(work, name)
		if err := os.WriteFile(source, []byte(body), 0o600); err != nil {
			t.Fatal(err)
		}
		prepared, err := farDaemonConfig(source)
		if err != nil {
			t.Fatalf("prepare %s: %v", name, err)
		}
		if !filepath.IsAbs(prepared) {
			t.Errorf("%s: prepared path %q is not absolute", name, prepared)
		}
		if filepath.Base(prepared) != name {
			t.Errorf("%s: prepared file is named %q", name, filepath.Base(prepared))
		}
		dir := filepath.Dir(prepared)
		if dirs[dir] {
			t.Errorf("%s: directory %s is shared with another daemon or the test", name, dir)
		}
		dirs[dir] = true
		got, err := os.ReadFile(prepared)
		if err != nil {
			t.Fatal(err)
		}
		if string(got) != body {
			t.Errorf("%s: prepared body %q, want %q", name, got, body)
		}
	}
	if _, err := farDaemonConfig(filepath.Join(work, "absent.conf")); err == nil {
		t.Error("an absent configuration was prepared without an error")
	}
}
