// VALIDATES: spec-managed-server-hardening AC-3 -- ze doctor reports a hub
// server block whose managed client listener cannot bind, because the plugin
// acceptor already holds its address.
// PREVENTS: a hub that looks configured and silently serves no managed client,
// and the opposite defect of flagging the documented central-hub config, whose
// serving block carries a plugin secret AND client entries and works.
//
// The method is the check's own entry point. diagnoseManagedListener is driven
// with hand-built blocks for the address cases. checkManagedListener is driven
// with a config tree parsed from real config text. The extraction the
// registered check depends on is therefore inside the test rather than assumed.
package doctor

import (
	"strings"
	"testing"

	"github.com/ze-software/ze/internal/component/config"
	zeplugin "github.com/ze-software/ze/internal/component/plugin"
	"github.com/ze-software/ze/internal/core/diagnostic"

	// The plugin schema is what makes `plugin { hub { ... } }` a config a
	// parser accepts. The daemon links it through the composition root. A
	// package test links only what it names, so it is named here.
	_ "github.com/ze-software/ze/internal/component/plugin/yang"
)

// managedTestSecret is 32 characters, the minimum the YANG leaf accepts.
const managedTestSecret = "0123456789abcdef0123456789abcdef"

func TestManagedListenerCollisionCases(t *testing.T) {
	cases := []struct {
		name    string
		servers []zeplugin.HubServerConfig
		want    int
	}{
		{
			name: "the acceptor block also declares clients",
			servers: []zeplugin.HubServerConfig{
				{Name: "local", Host: "127.0.0.1", Port: 1790, Secret: managedTestSecret,
					Clients: map[string]string{"edge-01": managedTestSecret}},
			},
			want: 1,
		},
		{
			name: "a second block repeats the acceptor address",
			servers: []zeplugin.HubServerConfig{
				{Name: "local", Host: "127.0.0.1", Port: 1790, Secret: managedTestSecret},
				{Name: "central", Host: "127.0.0.1", Port: 1790,
					Clients: map[string]string{"edge-01": managedTestSecret}},
			},
			want: 1,
		},
		{
			name: "the acceptor binds the wildcard and the managed block a host on it",
			servers: []zeplugin.HubServerConfig{
				{Name: "local", Host: "0.0.0.0", Port: 1790, Secret: managedTestSecret},
				{Name: "central", Host: "127.0.0.1", Port: 1790,
					Clients: map[string]string{"edge-01": managedTestSecret}},
			},
			want: 1,
		},
		{
			name: "the managed block binds the wildcard and the acceptor a host on it",
			servers: []zeplugin.HubServerConfig{
				{Name: "local", Host: "127.0.0.1", Port: 1790, Secret: managedTestSecret},
				{Name: "central", Host: "", Port: 1790,
					Clients: map[string]string{"edge-01": managedTestSecret}},
			},
			want: 1,
		},
		{
			// The central-hub example in docs/architecture/fleet-config.md. Its
			// serving block carries both a plugin secret and client entries, and
			// it works, because the acceptor binds the block ahead of it.
			name: "the documented central hub, one port each",
			servers: []zeplugin.HubServerConfig{
				{Name: "local", Host: "127.0.0.1", Port: 1790, Secret: managedTestSecret},
				{Name: "central", Host: "0.0.0.0", Port: 1791, Secret: managedTestSecret,
					Clients: map[string]string{"edge-01": managedTestSecret}},
			},
			want: 0,
		},
		{
			name: "one block, no managed clients",
			servers: []zeplugin.HubServerConfig{
				{Name: "local", Host: "127.0.0.1", Port: 1790, Secret: managedTestSecret},
			},
			want: 0,
		},
		{
			// Port 0 asks the kernel for a free port, so each listener gets its
			// own and neither bind fails. The block is unreachable for another
			// reason, and this check does not claim that one.
			name: "a shared block on port 0",
			servers: []zeplugin.HubServerConfig{
				{Name: "local", Host: "127.0.0.1", Port: 0, Secret: managedTestSecret,
					Clients: map[string]string{"edge-01": managedTestSecret}},
			},
			want: 0,
		},
		{
			name:    "no hub server block at all",
			servers: nil,
			want:    0,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			diags := diagnoseManagedListener(zeplugin.HubConfig{Servers: tc.servers})
			if len(diags) != tc.want {
				t.Fatalf("got %d diagnostics, want %d: %+v", len(diags), tc.want, diags)
			}
			for _, d := range diags {
				if d.Code != codeHubManagedCollision {
					t.Errorf("got code %q, want %q", d.Code, codeHubManagedCollision)
				}
				if d.Severity != diagnostic.SeverityError {
					t.Errorf("got severity %q, want %q", d.Severity, diagnostic.SeverityError)
				}
			}
		})
	}
}

// TestManagedListenerCollisionNamesBothBlocks holds the message to what an
// operator needs to act: which block cannot serve, and which one took the
// address from it. A diagnostic that says only "collision" leaves them reading
// the config to find out where.
func TestManagedListenerCollisionNamesBothBlocks(t *testing.T) {
	diags := diagnoseManagedListener(zeplugin.HubConfig{Servers: []zeplugin.HubServerConfig{
		{Name: "local", Host: "127.0.0.1", Port: 1790, Secret: managedTestSecret},
		{Name: "central", Host: "127.0.0.1", Port: 1790,
			Clients: map[string]string{"edge-01": managedTestSecret}},
	}})
	if len(diags) != 1 {
		t.Fatalf("got %d diagnostics, want 1", len(diags))
	}
	for _, want := range []string{"central", "local", "127.0.0.1:1790"} {
		if !strings.Contains(diags[0].Message, want) {
			t.Errorf("message does not name %q: %s", want, diags[0].Message)
		}
	}
}

// TestManagedListenerCheckReadsTheConfigTree drives the registered check
// function, from config text through ExtractHubConfig to the verdict. The
// address cases above take a HubConfig the test built. This one proves the
// check reaches the same struct from what an operator writes.
func TestManagedListenerCheckReadsTheConfigTree(t *testing.T) {
	const collides = `plugin {
    hub {
        server local {
            ip 127.0.0.1;
            port 1790;
            secret "` + managedTestSecret + `";
            client edge-01 { secret "` + managedTestSecret + `"; }
        }
    }
}
`
	const separate = `plugin {
    hub {
        server local {
            ip 127.0.0.1;
            port 1790;
            secret "` + managedTestSecret + `";
        }
        server central {
            ip 0.0.0.0;
            port 1791;
            secret "` + managedTestSecret + `";
            client edge-01 { secret "` + managedTestSecret + `"; }
        }
    }
}
`
	for _, tc := range []struct {
		name string
		body string
		want int
	}{
		{name: "one block serving both roles", body: collides, want: 1},
		{name: "one block for each role", body: separate, want: 0},
	} {
		t.Run(tc.name, func(t *testing.T) {
			loaded, err := config.LoadConfig(tc.body, "", nil)
			if err != nil {
				t.Fatalf("load config: %v", err)
			}
			diags := checkManagedListener(diagnostic.DoctorCheckContext{Tree: loaded.Tree})
			if len(diags) != tc.want {
				t.Fatalf("got %d diagnostics, want %d: %+v", len(diags), tc.want, diags)
			}
		})
	}
}

// TestManagedListenerCheckIgnoresAForeignTree holds the two inputs the doctor
// runner can hand a post-config check when no config was parsed. Neither is a
// finding, and neither MUST panic.
func TestManagedListenerCheckIgnoresAForeignTree(t *testing.T) {
	for _, tc := range []struct {
		name string
		tree any
	}{
		{name: "no tree", tree: nil},
		{name: "a tree of another type", tree: struct{}{}},
		{name: "a typed nil tree", tree: (*config.Tree)(nil)},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if diags := checkManagedListener(diagnostic.DoctorCheckContext{Tree: tc.tree}); diags != nil {
				t.Fatalf("got %+v, want no diagnostics", diags)
			}
		})
	}
}
