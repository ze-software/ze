//go:build ze_bgp

// Design: docs/architecture/testing/runner-architecture.md -- browser auth baseline replay.

package cli

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ze-software/ze/internal/component/authz"
	"github.com/ze-software/ze/internal/component/config"
	"github.com/ze-software/ze/internal/component/config/infra"
)

// TestBrowserCorpusAuthBaselineUsesDeclaredProfiles reads the emitted sequence
// baseline through the real corpus entry point. This decides structural config
// composition only; login replay and per-user authorization remain unsupported.
func TestBrowserCorpusAuthBaselineUsesDeclaredProfiles(t *testing.T) {
	for _, kind := range []string{"daemon", "web"} {
		t.Run(kind, func(t *testing.T) {
			s, path := browserCorpusCase(t, "merge", "set bgp group auth-corpus description fragment\n", "")
			body, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			workflow := "option=auth:user=admin:password=admin-fixture:role=admin\n" +
				"option=auth:user=noc:password=reader-fixture:role=read-only\n" +
				strings.Replace(string(body), "option=server:kind=daemon", "option=server:kind="+kind, 1)
			if err := os.WriteFile(path, []byte(workflow), 0o600); err != nil {
				t.Fatal(err)
			}
			sequences := s.browserSequences(path, map[string]bool{})
			if len(sequences) != 1 {
				t.Fatalf("auth baseline sequence missing: %v", s.report.findings)
			}
			tree, err := config.NewParser(s.schema).Parse(sequences[0].base)
			if err != nil {
				t.Fatal(err)
			}
			system := tree.GetContainer("system")
			if system == nil {
				t.Fatal("corpus silently discarded browser auth declarations")
			}
			local := authz.LocalAuthenticator{Users: infra.ExtractAuthUsers(system.ToMap())}
			policy, err := infra.ExtractAuthzStore(tree)
			if err != nil {
				t.Fatal(err)
			}
			if policy == nil {
				t.Fatal("corpus baseline discarded fixture authorization profiles")
			}
			for _, credential := range []authz.AuthRequest{{Username: "admin", Password: "admin-fixture"}, {Username: "noc", Password: "reader-fixture"}} {
				if _, err := local.Authenticate(credential); err != nil {
					t.Errorf("corpus baseline lost %s credential: %v", credential.Username, err)
				}
			}
			if policy.Authorize("admin", "config commit", false) != authz.Allow {
				t.Error("corpus baseline denied the declared administrator")
			}
			if policy.Authorize("noc", "show bgp", true) != authz.Allow {
				t.Error("corpus baseline denied the declared reader's reads")
			}
			if policy.Authorize("noc", "config commit", false) != authz.Deny {
				t.Error("corpus baseline promoted the declared reader")
			}
		})
	}
}

// TestBrowserCorpusAuthLoginRetainsUnsupportedFinding keeps a login ahead of a
// load explicit until replay can model the authenticated user's editor owner.
func TestBrowserCorpusAuthLoginRetainsUnsupportedFinding(t *testing.T) {
	s, path := browserCorpusCase(t, "merge", "set bgp group auth-corpus description fragment\n", "action=login:user=admin:password=admin-fixture\n")
	body, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, append([]byte("option=auth:user=admin:password=admin-fixture:role=admin\n"), body...), 0o600); err != nil {
		t.Fatal(err)
	}
	loaded := map[string]bool{}
	if sequences := s.browserSequences(path, loaded); len(sequences) != 0 {
		t.Fatal("unsupported login silently produced an accepted load composition")
	}
	if !strings.Contains(strings.Join(s.report.findings, "\n"), `browser action "login"`) {
		t.Fatalf("unsupported authenticated owner was hidden: %v", s.report.findings)
	}
	if loaded[filepath.Join(s.root, "fragment.conf")] {
		t.Fatal("unsupported login removed the standalone fragment oracle")
	}
}
