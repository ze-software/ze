// Design: docs/architecture/testing/interop.md -- registered images own their staged binaries.
// Related: register_images.go -- the strongSwan image declares its peer-local helper.
// Related: ipsec.go -- LabBinaries supplies the suite's staging preflight.

package ipsec

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ze-software/ze/internal/le/interoplab"
	lepath "github.com/ze-software/ze/internal/le/le/path"
)

// TestIPsecPeerFormWiring follows the registered image through suite staging and
// its Dockerfile COPY. It reads declarations only and starts no Docker process.
func TestIPsecPeerFormWiring(t *testing.T) {
	root, err := lepath.Root()
	if err != nil {
		t.Fatal(err)
	}
	imageContext := interoplab.ImageContext{Root: root}
	builds, binaries, err := interoplab.ResolveImages(imageContext, []string{swanImage})
	if err != nil {
		t.Fatal(err)
	}
	if len(builds) != 1 {
		t.Fatalf("strongSwan resolved %d images, want one", len(builds))
	}

	t.Run("suite stages the helper once", func(t *testing.T) {
		requireIPsecPeerFormBinary(t, LabBinaries())
	})
	t.Run("strongSwan declares the helper once", func(t *testing.T) {
		requireIPsecPeerFormBinary(t, binaries)
	})
	t.Run("one registered image owns the helper", func(t *testing.T) {
		var declared []interoplab.LabBinary
		images := interoplab.SuiteImages(imageOwner, imageContext)
		for index := range images {
			_, owned, err := interoplab.ResolveImages(imageContext, []string{images[index].Name})
			if err != nil {
				t.Fatal(err)
			}
			declared = append(declared, owned...)
		}
		requireIPsecPeerFormBinary(t, declared)
	})
	t.Run("strongSwan copies the staged executable", func(t *testing.T) {
		if want := filepath.Join(root, "test", "interop-ipsec"); builds[0].Context != want {
			t.Fatalf("strongSwan context = %q, want %q", builds[0].Context, want)
		}
		if want := filepath.Join(builds[0].Context, "Dockerfile.strongswan"); builds[0].Dockerfile != want {
			t.Fatalf("strongSwan Dockerfile = %q, want %q", builds[0].Dockerfile, want)
		}
		body, err := os.ReadFile(builds[0].Dockerfile)
		if err != nil {
			t.Fatal(err)
		}
		const instruction = "COPY ipsec-peerform-linux /usr/bin/ipsec-peerform"
		copies := 0
		for line := range strings.SplitSeq(string(body), "\n") {
			if strings.TrimSpace(line) == instruction {
				copies++
			}
		}
		if copies != 1 {
			t.Fatalf("strongSwan has %d %q instructions, want one", copies, instruction)
		}
	})
}

func requireIPsecPeerFormBinary(t *testing.T, binaries []interoplab.LabBinary) {
	t.Helper()
	count := 0
	for index := range binaries {
		binary := &binaries[index]
		if binary.Name != "ipsec-peerform" {
			continue
		}
		count++
		if binary.Package != "./internal/le/interoplab/ipsec/peerform" {
			t.Errorf("peer helper package = %q", binary.Package)
		}
		if binary.Base != "ze_core" {
			t.Errorf("peer helper tag base = %q, want ze_core", binary.Base)
		}
		if binary.Output != "test/interop-ipsec/ipsec-peerform-linux" {
			t.Errorf("peer helper staged path = %q", binary.Output)
		}
	}
	if count != 1 {
		t.Fatalf("found %d peer helper declarations, want one", count)
	}
}
