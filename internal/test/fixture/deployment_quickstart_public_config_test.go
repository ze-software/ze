//go:build ze_bgp && ze_ssh

// Design: docs/guide/quickstart.md -- the public local lab owns no host resolver file.
package fixture

import (
	"os"
	"path/filepath"
	"testing"

	_ "github.com/ze-software/ze/internal/component/bgp/yang" // registers the public BGP configuration schema
	"github.com/ze-software/ze/internal/component/config"
	"github.com/ze-software/ze/internal/component/config/system"
	_ "github.com/ze-software/ze/internal/component/config/system/yang" // registers system DNS and authentication
	_ "github.com/ze-software/ze/internal/component/plugin/yang"        // registers the public RIB plugin declaration
	_ "github.com/ze-software/ze/internal/component/ssh/yang"           // registers the public SSH listener schema
	lepath "github.com/ze-software/ze/internal/le/le/path"
)

// TestDeploymentQuickstartPublicConfigIsolation reads the files an operator
// copies, without the functional fixture's private configuration adaptations.
// The parsed SSH login and BGP session are positive controls: an empty or
// unrelated config cannot satisfy the local lab's resolver isolation contract.
func TestDeploymentQuickstartPublicConfigIsolation(t *testing.T) {
	repository, err := lepath.Root()
	if err != nil {
		t.Fatal(err)
	}
	for _, example := range []struct {
		name    string
		peer    string
		sshPort string
	}{
		{name: "r1", peer: "r2", sshPort: "2221"},
		{name: "r2", peer: "r1", sshPort: "2222"},
	} {
		t.Run(example.name, func(t *testing.T) {
			path := filepath.Join(repository, "examples", "quickstart", example.name+".conf")
			body, err := os.ReadFile(path) //nolint:gosec // the table names canonical public examples
			if err != nil {
				t.Fatal(err)
			}
			tree, err := config.ParseTreeForValidation(string(body))
			if err != nil {
				t.Fatalf("parse public example: %v", err)
			}
			ssh := quickstartPublicContainer(t, tree, "environment/ssh")
			quickstartPublicLeaf(t, ssh, "enabled", "true")
			server := ssh.GetList("server")["main"]
			quickstartPublicLeaf(t, server, "ip", "127.0.0.1")
			quickstartPublicLeaf(t, server, "port", example.sshPort)
			auth := quickstartPublicContainer(t, tree, "system/authentication")
			quickstartPublicLeaf(t, auth.GetList("user")["lab"], "plaintext-password", "lab")
			bgp := quickstartPublicContainer(t, tree, "bgp")
			if len(bgp.GetList("peer")) != 1 {
				t.Fatal("public example must retain its one BGP peer")
			}
			peer := bgp.GetList("peer")[example.peer]
			asn := quickstartPublicContainer(t, peer, "session/asn")
			quickstartPublicLeaf(t, asn, "local", "65001")
			quickstartPublicLeaf(t, asn, "remote", "65001")

			// MUTATION: remove the public resolv-conf-path statement and the
			// extractor returns the appliance default /tmp/resolv.conf.
			if dns := tree.GetContainerPath("system/dns"); dns == nil {
				t.Error("public local lab must explicitly disable resolver file writing")
			} else {
				quickstartPublicLeaf(t, dns, "resolv-conf-path", "")
			}
			if got := system.ExtractSystemConfig(tree).ResolvConfPath; got != "" {
				t.Errorf("public local lab writes resolver output to %q; want disabled writing", got)
			}
		})
	}
}

func quickstartPublicContainer(t *testing.T, tree *config.Tree, path string) *config.Tree {
	t.Helper()
	if tree == nil {
		t.Fatalf("public example is missing parent of %s", path)
	}
	container := tree.GetContainerPath(path)
	if container == nil {
		t.Fatalf("public example is missing %s", path)
	}
	return container
}

func quickstartPublicLeaf(t *testing.T, tree *config.Tree, leaf, want string) {
	t.Helper()
	if tree == nil {
		t.Fatalf("public example is missing parent of %s", leaf)
	}
	got, present := tree.Get(leaf)
	if !present {
		t.Fatalf("public example is missing explicit %s", leaf)
	}
	if got != want {
		t.Errorf("public example %s = %q, want %q", leaf, got, want)
	}
}
