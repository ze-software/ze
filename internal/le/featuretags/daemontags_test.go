// Related: daemontags.go -- the daemon build-tag derivation these tests call as a function

package featuretags

import (
	"strings"
	"testing"
)

// VALIDATES: a daemon build tag list is every ze_ gate the manifest declares,
// sorted and deduplicated, with the base tags in front.
// PREVENTS: an evidence tool carrying its own literal tag list, which is the
// drift that left ze_bgp and ze_l2tp out of the daemons those tools built.
func TestDaemonBuildTagsAreTheManifestPlusTheBase(t *testing.T) {
	root := fixture(t)

	tags, err := DaemonTags(root)
	if err != nil {
		t.Fatalf("read the gate tags: %v", err)
	}
	// The fixture declares ze_bgp twice, then ze_web, then ze_lg.
	want := []string{"ze_bgp", "ze_lg", "ze_web"}
	if len(tags) != len(want) {
		t.Fatalf("the manifest answers %v, want %v", tags, want)
	}
	for i, tag := range want {
		if tags[i] != tag {
			t.Errorf("gate %d is %q, want %q", i, tags[i], tag)
		}
	}

	line, err := DaemonBuildTags(root, DaemonBase)
	if err != nil {
		t.Fatalf("build the tag line: %v", err)
	}
	if line != "ze_core ze_distro ze_bgp ze_lg ze_web" {
		t.Errorf("the tag line is %q, want the base then the sorted gates", line)
	}
}

// VALIDATES: the base is a parameter, so a caller that builds something other
// than the distro daemon says so at the call site.
// PREVENTS: a second literal base spelled inside a tool.
func TestDaemonBuildTagsTakesTheBaseFromItsCaller(t *testing.T) {
	root := fixture(t)

	line, err := DaemonBuildTags(root, "ze_core ze_appliance")
	if err != nil {
		t.Fatalf("build the tag line: %v", err)
	}
	if !strings.HasPrefix(line, "ze_core ze_appliance ") {
		t.Errorf("the tag line is %q, want the caller's base in front", line)
	}
}

// VALIDATES: a manifest declaring no gate is an ERROR, whether it declares
// nothing at all or declares only names that are not gates, and the error names
// the file it read.
// PREVENTS: the fail-open the Python module has -- an empty list builds a daemon
// with every feature compiled out, and the caller then reads "unknown top-level
// keyword: l2tp" as a protocol defect rather than as a build that carried no
// L2TP. Measured 2026-08-26: internal/le/featuretags/actions.go answers
// "ze_core ze_distro" for a manifest of comments, with no error at all.
func TestAManifestWithNoGateIsAnError(t *testing.T) {
	cases := []struct {
		name     string
		manifest string
		want     string
	}{
		{"nothing declared", "# every line here is a comment\n\n", "no feature-gate tags found"},
		{"declared but not a gate", "notatag internal/component/x\n", ErrNoGateTags.Error()},
	}

	for _, one := range cases {
		t.Run(one.name, func(t *testing.T) {
			root := t.TempDir()
			write(t, root, "feature-gates.txt", one.manifest)

			tags, err := DaemonTags(root)
			if err == nil {
				t.Fatalf("a manifest with no gate answered %v, want an error", tags)
			}
			if !strings.Contains(err.Error(), one.want) {
				t.Errorf("the error is %q, want it to say %q", err, one.want)
			}
			if !strings.Contains(err.Error(), "feature-gates.txt") {
				t.Errorf("the error is %q, want it to name the file it read", err)
			}

			if _, err := DaemonBuildTags(root, DaemonBase); err == nil {
				t.Error("the tag line was built from a manifest with no gate")
			}
		})
	}
}

// VALIDATES: an absent manifest is an error rather than an empty tag list.
// PREVENTS: a tool run outside a checkout building a featureless daemon and
// then reporting that the feature under test does not work.
func TestAnAbsentManifestIsAnError(t *testing.T) {
	if _, err := DaemonTags(t.TempDir()); err == nil {
		t.Error("an absent manifest answered a tag list")
	}
}
