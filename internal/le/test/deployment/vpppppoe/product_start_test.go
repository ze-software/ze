// Design: docs/labs/pppoe-interop.md -- the product daemon's store stays off the 9p evidence share.
// Related: product_check.go -- startProductDaemon, the one start of every product action.
package testvpppppoe

import (
	"context"
	"os"
	"path"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/ze-software/ze/internal/le/interoplab"
)

// productStartLab records the one detached command a product start sends.
type productStartLab struct {
	interoplab.CheckerLab
	peer string
	argv []string
}

func (lab *productStartLab) ExecDetached(_ context.Context, peer string, argv []string, _ []interoplab.EnvironmentVariable) error {
	lab.peer = peer
	lab.argv = argv
	return nil
}

// Goal: every product action starts Ze on a configuration outside /evidence,
// so Ze's config store (opened beside the configuration, refused when another
// uid owns it) never lands on the docker-lab 9p share, where every file carries
// the host user's uid and a root daemon refuses its own database.lock.
// Method: drive startProductDaemon against a recording lab; the configuration
// must reach the evidence directory, and the daemon must start in the VPP peer
// on the container-local copy, with its log still on /evidence.
func TestProductStartKeepsTheStoreOffTheEvidenceShare(t *testing.T) {
	directory := t.TempDir()
	lab := &productStartLab{}
	check := &interoplab.CheckContext{Source: interoplab.ScenarioSource{Directory: directory}, Lab: lab}
	if err := startProductDaemon(context.Background(), check, "pppoe { enabled true; }\n"); err != nil {
		t.Fatal(err)
	}
	written, err := os.ReadFile(filepath.Join(directory, "product.conf"))
	if err != nil {
		t.Fatal(err)
	}
	if string(written) != "pppoe { enabled true; }\n" {
		t.Fatalf("evidence configuration=%q", written)
	}
	if lab.peer != vppPeer {
		t.Fatalf("daemon started in peer %q, expected %q", lab.peer, vppPeer)
	}
	if len(lab.argv) != 3 || lab.argv[0] != "sh" || lab.argv[1] != "-c" {
		t.Fatalf("daemon start argv=%q", lab.argv)
	}
	words := strings.Fields(lab.argv[2])
	start := slices.Index(words, "start")
	if start < 1 || start+1 >= len(words) || words[start-1] != "/product/ze" {
		t.Fatalf("command starts no staged daemon: %s", lab.argv[2])
	}
	config := words[start+1]
	if path.Dir(config) != productStoreDirectory {
		t.Fatalf("daemon starts on %s, outside %s", config, productStoreDirectory)
	}
	if strings.HasPrefix(path.Clean(productStoreDirectory)+"/", evidencePath+"/") {
		t.Fatalf("store directory %s is on the evidence share %s", productStoreDirectory, evidencePath)
	}
	copied := "cp /evidence/product.conf " + config
	if !strings.Contains(lab.argv[2], copied) {
		t.Fatalf("command does not copy the evidence configuration (%s): %s", copied, lab.argv[2])
	}
	if !strings.Contains(lab.argv[2], "> /evidence/product-ze.log") {
		t.Fatalf("daemon log left the evidence directory: %s", lab.argv[2])
	}
}
