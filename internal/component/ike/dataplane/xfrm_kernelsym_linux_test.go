// Design: docs/architecture/appliance/kernel-profiles.md -- the VPN symbols of the runtime kernel
// Related: kernelcap_linux.go -- xfrmTransformKernel, the transform-to-symbol map
// Related: xfrm_linux.go -- the algorithm tables the transforms come from
//go:build linux

package dataplane

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

// kernelFragmentDir is gokrazy/kernel, seen from this package's directory.
var kernelFragmentDir = filepath.Join("..", "..", "..", "..", "gokrazy", "kernel")

// Every kernel transform the XFRM algorithm tables can install is built into
// Ze's runtime kernel and required by its build. gokrazy ships no modprobe, so
// a transform left as a module is as absent as one left out, and the SA naming
// it fails to install (spec-appliance-kernel-vpn-modules AC-1..AC-3).
//
// Method: walk xfrmEncNames, xfrmAEADNames and xfrmAuthNames, look each
// transform up in xfrmTransformKernel, and require its symbol as `=y` in a
// fragment of the runtime build (runtime.config, or kernel.config, which the
// runtime build merges first) and as a line of runtime.require, which the
// build refuses to lose. The transforms come from the tables, never from a
// list here, so a cipher added to a table alone fails this test.
func TestXfrmTransformsHaveRequiredKernelSymbol(t *testing.T) {
	requested := kernelFragmentBuiltIn(t, "kernel.config", "runtime.config")
	required := kernelRequireLines(t, "runtime.require")

	transforms := xfrmTableTransforms()
	if len(transforms) == 0 {
		t.Fatal("the XFRM algorithm tables name no transform, so this test compares nothing")
	}
	for _, transform := range transforms {
		facts, known := xfrmTransformKernel[transform]
		if !known {
			t.Errorf("transform %s has no kernel symbol in xfrmTransformKernel", transform)
			continue
		}
		if !requested[facts.kernel] {
			t.Errorf("transform %s needs %s=y, which neither kernel.config nor runtime.config sets", transform, facts.kernel)
		}
		if !required[facts.kernel] {
			t.Errorf("transform %s needs %s, which runtime.require does not declare", transform, facts.kernel)
		}
	}
}

// xfrmTableTransforms returns every distinct kernel transform name the three
// algorithm tables hold, sorted.
func xfrmTableTransforms() []string {
	var transforms []string
	for _, transform := range xfrmEncNames {
		transforms = append(transforms, transform)
	}
	for _, transform := range xfrmAEADNames {
		transforms = append(transforms, transform)
	}
	for _, auth := range xfrmAuthNames {
		transforms = append(transforms, auth.name)
	}
	slices.Sort(transforms)
	return slices.Compact(transforms)
}

// kernelFragmentBuiltIn returns the symbols the named fragments set to `=y`.
func kernelFragmentBuiltIn(t *testing.T, names ...string) map[string]bool {
	t.Helper()
	builtIn := map[string]bool{}
	for _, name := range names {
		for _, line := range kernelFileLines(t, name) {
			symbol, value, found := strings.Cut(line, "=")
			if found && value == "y" {
				builtIn[symbol] = true
			}
		}
	}
	return builtIn
}

// kernelRequireLines returns the symbols a require manifest declares. A line
// reads CONFIG_X or CONFIG_X=y; both mean built in.
func kernelRequireLines(t *testing.T, name string) map[string]bool {
	t.Helper()
	required := map[string]bool{}
	for _, line := range kernelFileLines(t, name) {
		if strings.HasPrefix(line, "CONFIG_") {
			required[strings.TrimSuffix(line, "=y")] = true
		}
	}
	return required
}

func kernelFileLines(t *testing.T, name string) []string {
	t.Helper()
	path := filepath.Join(kernelFragmentDir, name)
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	var lines []string
	for line := range strings.SplitSeq(string(data), "\n") {
		lines = append(lines, strings.TrimSpace(line))
	}
	return lines
}
