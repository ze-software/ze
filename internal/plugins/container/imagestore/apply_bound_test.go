//go:build linux

package imagestore

import (
	"strings"
	"testing"

	"github.com/ze-software/ze/internal/core/textbuf"
)

// TestApplyExpandedSizeBound proves an image whose decompressed layers are
// exactly the largest image size applies, and one byte more is refused,
// naming the bound.
//
// Method: build one layer, then apply it with the bound set to its length
// and to its length less one byte, which is the same image one byte over.
// The count the mark records is the length.
func TestApplyExpandedSizeBound(t *testing.T) {
	archive := layerTar(t, fileEntry("payload", strings.Repeat("x", 3000), 0o644))
	size := uint64(len(archive))
	root := preparedRoot(t)

	atBound := newAppliedStoreIn(t, root, "at-bound", applyLimits{expandedBytesMax: size, entriesMax: 10})
	if err := atBound.applyTar(t, archive); err != nil {
		t.Fatalf("apply at the bound: %v", err)
	}
	var facts imageFacts
	atBound.applier.countedFacts(&facts)
	if facts.ExpandedBytes != size {
		t.Errorf("expanded bytes %d, want %d", facts.ExpandedBytes, size)
	}

	over := newAppliedStoreIn(t, root, "over-bound", applyLimits{expandedBytesMax: size - 1, entriesMax: 10})
	refusal := requireRefusal(t, over.applyTar(t, archive), ruleExpandedBytes)
	if !strings.Contains(refusal.Error(), textbuf.StringUint(size-1)) {
		t.Errorf("the refusal %q does not name the bound", refusal.Error())
	}

	if got := defaultApplyLimits(1 << 30); got.expandedBytesMax != 1<<30 {
		t.Errorf("default limits %+v, want the largest image size", got)
	}
}

// TestApplyEntryCountBound proves an image of exactly the entry bound
// applies and one entry more is refused, that a folder a layer names for
// the first time on the way to an entry counts, and that the bound is
// 500,000 when no test lowers it.
//
// Method: with the bound lowered to eight, apply eight files, then nine,
// then one file eight folders deep, which is one entry and eight folders.
func TestApplyEntryCountBound(t *testing.T) {
	limits := applyLimits{expandedBytesMax: 1 << 20, entriesMax: 8}
	files := func(count int) []testEntry {
		entries := make([]testEntry, 0, count)
		for i := range count {
			entries = append(entries, fileEntry("file-"+textbuf.StringInt(int64(i)), "x", 0o644))
		}
		return entries
	}
	root := preparedRoot(t)

	newAppliedStoreIn(t, root, "at-bound", limits).mustApply(t, files(8)...)

	over := newAppliedStoreIn(t, root, "over-bound", limits)
	refusal := requireRefusal(t, over.applyTar(t, layerTar(t, files(9)...)), ruleEntries)
	if !strings.Contains(refusal.Error(), "file-8") {
		t.Errorf("the refusal %q does not name the ninth entry", refusal.Error())
	}

	deep := newAppliedStoreIn(t, root, "deep", limits)
	err := deep.applyTar(t, layerTar(t, fileEntry("a/b/c/d/e/f/g/h/file", "x", 0o644)))
	checkRefusal(t, err, ruleEntries)
	requireAbsent(t, deep.rootPath, "a")

	if got := defaultApplyLimits(1 << 30).entriesMax; got != 500_000 {
		t.Errorf("default entry bound %d, want 500000", got)
	}
}

// TestApplyLayerCountBound proves 128 layers apply and the 129th is
// refused.
//
// Method: apply 128 empty layers into one root, then one more.
func TestApplyLayerCountBound(t *testing.T) {
	store := newAppliedStore(t, testLimits())
	empty := layerTar(t)
	for i := range layersMax {
		if err := store.applyTar(t, empty); err != nil {
			t.Fatalf("layer %d: %v", i+1, err)
		}
	}
	checkRefusal(t, store.applyTar(t, empty), ruleLayers)
}

// TestApplyNameAndPathLengthBound proves the name, path, link target and
// owner bounds at their last valid value and one past it.
//
// Method: judge each value by its rule directly: an archive cannot be
// built for some of them, and the rule is what the applier calls.
func TestApplyNameAndPathLengthBound(t *testing.T) {
	if _, err := parseEntryPath(strings.Repeat("n", entryNameMax), false); err != nil {
		t.Errorf("a name of 255 bytes: %v", err)
	}
	_, err := parseEntryPath(strings.Repeat("n", entryNameMax+1), false)
	checkRefusal(t, err, ruleNameLength)

	path := strings.Repeat("n/", entryPathMax/2-1) + "nn"
	if len(path) != entryPathMax {
		t.Fatalf("the test path is %d bytes, want %d", len(path), entryPathMax)
	}
	if _, err := parseEntryPath(path, false); err != nil {
		t.Errorf("a path of 4096 bytes: %v", err)
	}
	_, err = parseEntryPath(path+"n", false)
	checkRefusal(t, err, rulePathLength)

	if err := checkLinkTarget("link", strings.Repeat("t", entryPathMax)); err != nil {
		t.Errorf("a target of 4096 bytes: %v", err)
	}
	checkRefusal(t, checkLinkTarget("link", strings.Repeat("t", entryPathMax+1)), ruleLinkTarget)
	checkRefusal(t, checkLinkTarget("link", ""), ruleLinkTarget)

	if _, _, err := entryOwner("owned", ownerMax, ownerMax); err != nil {
		t.Errorf("owner %d: %v", ownerMax, err)
	}
	_, _, err = entryOwner("owned", ownerMax+1, 0)
	checkRefusal(t, err, ruleOwner)
	_, _, err = entryOwner("owned", 0, -1)
	checkRefusal(t, err, ruleOwner)
}
