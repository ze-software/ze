// Design: show_kernel_log_linux.go -- kmsgLevelNames is indexed by the kernel's priority number
//
// Goal: prove the levels `show system kernel-log level` offers an operator are
// the kernel level names parseLevelArg reads, so a word cannot exist on one
// side alone. Method: read the enumeration out of the loaded model with
// configyang.EnumValues, which fails on a leaf that declares no enumeration,
// hold it to kmsgLevelNames, and drive parseLevelArg with each word: a word the
// table does not hold falls to 7, which reads as debug rather than as a refusal.

//go:build linux

package cmd

import (
	"slices"
	"testing"

	configyang "github.com/ze-software/ze/internal/component/config/yang"

	// The blank import registers ze-host-cmd with the loader, which declares
	// the leaf read below.
	_ "github.com/ze-software/ze/internal/plugins/host-cmd/yang"
)

const kernelLogLevelLeaf = "show/system/kernel-log/level"

// TestKernelLogLevelsMatchTheModel holds kmsgLevelNames to the model in both
// directions.
func TestKernelLogLevelsMatchTheModel(t *testing.T) {
	model, err := configyang.EnumValues(kernelLogLevelLeaf)
	if err != nil {
		t.Fatalf("read the enumeration at %s: %v", kernelLogLevelLeaf, err)
	}

	named := slices.Sorted(slices.Values(kmsgLevelNames[:]))
	if !slices.Equal(model, named) {
		t.Errorf("the levels disagree: the model at %s holds %v and kmsgLevelNames holds %v. "+
			"A word only the model carries is read as debug, so the filter widens in silence",
			kernelLogLevelLeaf, model, named)
	}

	for _, level := range model {
		index := parseLevelArg(level)
		if kmsgLevelNames[index] != level {
			t.Errorf("%q is offered at %s and parseLevelArg reads it as %d (%s)", level, kernelLogLevelLeaf, index, kmsgLevelNames[index])
		}
	}
}
