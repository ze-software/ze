//go:build linux

package imagestore

import (
	"strings"
	"testing"
)

// TestApplyOverlayControlAttributeRefused proves an overlay control
// attribute, in the trusted or the user namespace, refuses the image, as
// does an attribute of a name the store does not keep and one in the
// libarchive form, and that the names the store keeps pass the same rule.
//
// Method: apply one file per attribute and check the rule and that no file
// was made. The kept names, and the 64 KiB bound on one entry, are judged
// by the attribute rule itself, which needs no filesystem that stores them.
func TestApplyOverlayControlAttributeRefused(t *testing.T) {
	store := newAppliedStore(t, testLimits())
	cases := []struct {
		record string
		rule   refusalRule
	}{
		{paxAttributePrefix + "trusted.overlay.opaque", ruleOverlayAttribute},
		{paxAttributePrefix + "user.overlay.redirect", ruleOverlayAttribute},
		{paxAttributePrefix + "security.selinux", ruleAttributeName},
		{paxAttributePrefix + "trusted.other", ruleAttributeName},
		{paxAttributePrefix + "user.", ruleAttributeName},
		{paxLibarchiveAttributePrefix + "user.comment", ruleAttributeName},
	}
	for _, c := range cases {
		entry := fileEntry("marked", "content\n", 0o644)
		entry.records = map[string]string{c.record: "y"}
		err := store.applyTar(t, layerTar(t, entry))
		checkRefusal(t, err, c.rule)
		requireAbsent(t, store.rootPath, "marked")
	}

	for _, name := range []string{attributeFileCapability, attributeACLAccess, attributeACLDefault, "user.comment"} {
		if err := checkAttributeName("kept", name); err != nil {
			t.Errorf("%s: want kept, got %v", name, err)
		}
	}

	name := "user.big"
	atBound := map[string]string{paxAttributePrefix + name: strings.Repeat("v", entryAttributesMax-len(name))}
	attributes, err := entryAttributes("big", atBound)
	if err != nil {
		t.Fatalf("attributes of exactly 64 KiB: %v", err)
	}
	if len(attributes) != 1 {
		t.Fatalf("attributes %v, want one", attributes)
	}
	overBound := map[string]string{paxAttributePrefix + name: strings.Repeat("v", entryAttributesMax-len(name)+1)}
	_, err = entryAttributes("big", overBound)
	checkRefusal(t, err, ruleAttributeBytes)
}
