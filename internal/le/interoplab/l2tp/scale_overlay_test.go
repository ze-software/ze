// Design: docs/labs/l2tp-interop.md -- isolated allocator instrumentation.
package l2tp

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/ze-software/ze/internal/le/interoplab"
)

// TestScaleBuildInputs follows registered images to their actual build inputs.
func TestScaleBuildInputs(t *testing.T) {
	_, ordinary, err := interoplab.ResolveImages(interoplab.ImageContext{}, []string{imageZe, imageAVPRelay})
	if err != nil {
		t.Fatal(err)
	}
	ze := declaredScaleBinary(t, ordinary, "ze")
	if ze.PrepareOverlay != nil {
		t.Fatal("normal Ze declares scale instrumentation")
	}
	relay := declaredScaleBinary(t, ordinary, "avp-relay")
	if relay.Package != "./internal/le/interoplab/l2tp/avprelay" || relay.Output != "test/interop-l2tp/avp-relay-linux" || relay.PrepareOverlay != nil {
		t.Fatalf("relay is not host-staged from its package: %+v", relay)
	}
	_, scale, err := interoplab.ResolveImages(interoplab.ImageContext{}, []string{imageZeScale})
	if err != nil {
		t.Fatal(err)
	}
	instrumented := declaredScaleBinary(t, scale, "ze-scale")
	if instrumented.PrepareOverlay == nil || instrumented.Output != "test/interop-l2tp/ze-scale-linux" || instrumented.Base != ze.Base {
		t.Fatalf("scale Ze lacks isolated instrumentation: %+v", instrumented)
	}
	if slices.ContainsFunc(ordinary, func(binary interoplab.LabBinary) bool { return binary.Output == instrumented.Output }) {
		t.Fatal("ordinary images stage the scale daemon")
	}
	for _, binary := range append(ordinary, scale...) {
		if !slices.ContainsFunc(LabBinaries(), func(inventory interoplab.LabBinary) bool { return inventory.Output == binary.Output }) {
			t.Errorf("lab inventory omits %s", binary.Output)
		}
	}
	for _, name := range []string{scenarioPPPCodeReject, scenarioScale} {
		plans := []interoplab.ScenarioPlan{{Source: interoplab.ScenarioSource{Name: name}}}
		selected := binariesForImages("", imageBuilds("", plans, "frr:test"))
		instrumented := slices.ContainsFunc(selected, func(binary interoplab.LabBinary) bool { return binary.PrepareOverlay != nil })
		if instrumented != (name == scenarioScale) {
			t.Errorf("scenario %s selects scale instrumentation=%t", name, instrumented)
		}
		declaredScaleBinary(t, selected, "avp-relay")
	}
}

// TestScaleAllocatorOverlayIsolation proves the source boundary and refuses drift
// before a source replacement can reach a compiler. The live scenario proves IDs.
func TestScaleAllocatorOverlayIsolation(t *testing.T) {
	_, binaries, err := interoplab.ResolveImages(interoplab.ImageContext{}, []string{imageZeScale})
	if err != nil {
		t.Fatal(err)
	}
	binary := declaredScaleBinary(t, binaries, "ze-scale")
	if binary.PrepareOverlay == nil {
		t.Fatal("scale binary has no allocator overlay")
	}
	original, err := os.ReadFile(filepath.Join("..", "..", "..", "component", "l2tp", "session.go"))
	if err != nil {
		t.Fatal(err)
	}
	for _, test := range []struct{ name, old, replacement string }{
		{name: "unchanged"},
		{name: "random_error", old: "if _, err := rand.Read(buf[:]); err != nil", replacement: "if _, err := rand.Read(buf[:]); err == nil"},
		{name: "candidate", old: "sid := binary.BigEndian.Uint16(buf[:])", replacement: "sid := uint16(1)"},
		{name: "guard", old: "if sid == 0", replacement: "if sid == 1"},
		{name: "receiver", old: "func (t *L2TPTunnel) allocateSessionID()", replacement: "func (t *L2TPTunnel) changedAllocator()"},
	} {
		t.Run(test.name, func(t *testing.T) {
			root := t.TempDir()
			path := filepath.Join(root, "internal", "component", "l2tp", "session.go")
			if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
				t.Fatal(err)
			}
			input := original
			if test.old != "" {
				if !bytes.Contains(original, []byte(test.old)) {
					t.Fatal("drift fixture no longer names the allocator producer")
				}
				input = bytes.Replace(original, []byte(test.old), []byte(test.replacement), 1)
			}
			if err := os.WriteFile(path, input, 0o600); err != nil {
				t.Fatal(err)
			}
			overlayPath, cleanup, err := binary.PrepareOverlay(root)
			if test.old != "" {
				if cleanup != nil {
					t.Cleanup(func() {
						if err := cleanup(); err != nil {
							t.Error(err)
						}
					})
				}
				if err == nil || overlayPath != "" {
					t.Fatalf("source drift accepted: overlay=%q err=%v", overlayPath, err)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if cleanup == nil {
				t.Fatal("overlay has no owner cleanup")
			}
			t.Cleanup(func() {
				if err := cleanup(); err != nil {
					t.Error(err)
				}
			})
			data, err := os.ReadFile(overlayPath)
			if err != nil {
				t.Fatal(err)
			}
			var overlay struct{ Replace map[string]string }
			if err := json.Unmarshal(data, &overlay); err != nil {
				t.Fatal(err)
			}
			if len(overlay.Replace) != 1 || overlay.Replace[path] == "" {
				t.Fatalf("overlay replaces unexpected sources: %v", overlay.Replace)
			}
			changed, err := os.ReadFile(overlay.Replace[path])
			if err != nil {
				t.Fatal(err)
			}
			// Removing only this candidate selection must recover every original
			// byte, including the random read, errors and both existing guards.
			const insertion = "\n\t\tsid = 0\n\t\tfor candidate := uint32(1); candidate < 1<<16; candidate++ {\n\t\t\tif _, exists := t.sessions[uint16(candidate)]; !exists {\n\t\t\t\tsid = uint16(candidate)\n\t\t\t\tbreak\n\t\t\t}\n\t\t}"
			if strings.Count(string(changed), insertion) != 1 || !bytes.Equal(bytes.Replace(changed, []byte(insertion), nil, 1), original) {
				t.Fatal("overlay changed more than the bounded lowest-unused candidate choice")
			}
			unchanged, err := os.ReadFile(path)
			if err != nil || !bytes.Equal(unchanged, original) {
				t.Fatalf("tracked source changed: %v", err)
			}
			if err := cleanup(); err != nil {
				t.Fatal(err)
			}
			if _, err := os.Stat(filepath.Dir(overlayPath)); !os.IsNotExist(err) {
				t.Fatalf("overlay run remains after cleanup: %v", err)
			}
		})
	}
}

func declaredScaleBinary(t *testing.T, binaries []interoplab.LabBinary, name string) interoplab.LabBinary {
	t.Helper()
	for _, binary := range binaries {
		if binary.Name == name {
			return binary
		}
	}
	t.Fatalf("registered images do not stage %s", name)
	return interoplab.LabBinary{}
}
