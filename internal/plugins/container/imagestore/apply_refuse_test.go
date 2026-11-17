//go:build linux

package imagestore

import (
	"archive/tar"
	"strconv"
	"strings"
	"testing"
)

// TestApplyParentDirectoryPathRefused proves a path that could leave the
// root, or that the store would have to correct, refuses the image: a
// parent-directory component, an absolute path, an empty component, and a
// NUL byte. Nothing outside the staged folder changes, and a "." component,
// which leaves nothing, is applied.
//
// Method: apply one layer per hostile name and check the rule and that the
// refusal names the entry, quoted. The NUL byte and a file name with a
// trailing slash cannot travel in an archive the writer accepts, so they are
// judged by the path rule itself.
func TestApplyParentDirectoryPathRefused(t *testing.T) {
	store := newAppliedStore(t, testLimits())
	store.writeMarker(t, "escape", "marker\n")
	before := store.outsideSnapshot(t)

	cases := []struct {
		name string
		rule refusalRule
	}{
		{"../escape", ruleParentComponent},
		{"a/../../escape", ruleParentComponent},
		{"a/b/..", ruleParentComponent},
		{"/escape", ruleAbsolutePath},
		{"a//escape", ruleEmptyComponent},
	}
	for _, c := range cases {
		err := store.applyTar(t, layerTar(t, fileEntry(c.name, "written\n", 0o644)))
		refusal := requireRefusal(t, err, c.rule)
		if !strings.Contains(refusal.Error(), strconv.QuoteToASCII(c.name)) {
			t.Errorf("%q: the refusal %q does not name the entry", c.name, refusal.Error())
		}
	}
	_, err := parseEntryPath("a\x00b", false)
	checkRefusal(t, err, ruleNULInName)
	// A file name with a trailing slash: the archive writer refuses to
	// encode one, so it too is judged by the path rule itself.
	_, err = parseEntryPath("escape/", false)
	checkRefusal(t, err, ruleEmptyComponent)
	store.requireOutsideUnchanged(t, before)

	store.mustApply(t, fileEntry("./a/./b", "dot\n", 0o644))
	requireContent(t, store.rootPath, "a/b", "dot\n")
}

// TestApplyReplacesEntryNeverWritesThrough proves a file whose name held a
// symbolic link, a hard link or a folder replaces what the name held, and
// writes through none of it.
//
// Method: a first layer plants a link to a file beside the store root, a
// file with a second hard link, and a folder with a child. A second layer
// writes a file under each of the three names. Each name must then be a
// new regular file with the new content; the file outside and the first
// hard link's file must keep their content.
func TestApplyReplacesEntryNeverWritesThrough(t *testing.T) {
	store := newAppliedStore(t, testLimits())
	target := store.writeMarker(t, "victim-target", "original\n")
	store.mustApply(t,
		symlinkEntry("victim", target),
		fileEntry("f", "old\n", 0o644),
		hardLinkEntry("g", "f"),
		folderEntry("x", 0o755),
		fileEntry("x/y", "child\n", 0o644),
	)
	before := store.outsideSnapshot(t)
	store.mustApply(t,
		fileEntry("victim", "new\n", 0o644),
		fileEntry("g", "new\n", 0o644),
		fileEntry("x", "new\n", 0o644),
	)

	requireContent(t, store.rootPath, "victim", "new\n")
	requireContent(t, store.rootPath, "g", "new\n")
	requireContent(t, store.rootPath, "x", "new\n")
	requireContent(t, store.rootPath, "f", "old\n")
	tree := readTree(t, store.rootPath, true)
	if tree["f"].inode == tree["g"].inode {
		t.Errorf("g is still the inode of f, %d: want a new file", tree["f"].inode)
	}
	store.requireOutsideUnchanged(t, before)
}

// TestApplyDeviceNodeUnderDevCounted proves a device node under the image's
// device folder is not created and is counted for the completion mark.
//
// Method: apply a character and a block device under dev, read the names
// back and the counts the applier hands the mark.
func TestApplyDeviceNodeUnderDevCounted(t *testing.T) {
	store := newAppliedStore(t, testLimits())
	store.mustApply(t,
		folderEntry("dev", 0o755),
		deviceEntry("dev/null", tar.TypeChar),
		deviceEntry("dev/sda", tar.TypeBlock),
		fileEntry("etc", "file\n", 0o644),
	)
	requireAbsent(t, store.rootPath, "dev/null")
	requireAbsent(t, store.rootPath, "dev/sda")

	var facts imageFacts
	store.applier.countedFacts(&facts)
	if facts.DevicesSkipped != 2 {
		t.Errorf("devices skipped %d, want 2", facts.DevicesSkipped)
	}
	if facts.Entries != 4 {
		t.Errorf("entries %d, want 4", facts.Entries)
	}
}

// TestApplyDeviceNodeElsewhereRefused proves a device node anywhere but
// under the device folder refuses the image, naming the entry, the device
// folder itself included.
//
// Method: apply one layer per node and check the rule, the name in the
// refusal, and that no node was made.
func TestApplyDeviceNodeElsewhereRefused(t *testing.T) {
	store := newAppliedStore(t, testLimits())
	for _, name := range []string{"etc/null", "dev", "devices/null"} {
		err := store.applyTar(t, layerTar(t, deviceEntry(name, tar.TypeChar)))
		refusal := requireRefusal(t, err, ruleDeviceNode)
		if !strings.Contains(refusal.Error(), strconv.QuoteToASCII(name)) {
			t.Errorf("%s: the refusal %q does not name the entry", name, refusal.Error())
		}
		requireAbsent(t, store.rootPath, name)
	}
}
