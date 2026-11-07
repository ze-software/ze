// Design: docs/guide/configuration.md -- System Host Name: syntax, validation, appliance kernel hostname
// Related: system.go -- ExtractSystemConfig, Ze's own identity from the same leaf
// Related: register.go -- installs validateSystemHostTree as a tree validator

package system

import (
	"errors"
	"fmt"
	"strings"

	"github.com/ze-software/ze/internal/component/config"
)

var (
	errHostNameEmpty  = errors.New("is empty")
	errHostLabelEmpty = errors.New("has an empty label: a dot starts it, ends it, or follows another dot")
)

// hostNameOctetsMax is the kernel's limit: sethostname(2) answers EINVAL for a
// name longer than __NEW_UTS_LEN, 64 bytes.
const hostNameOctetsMax = 64

// hostLabelOctetsMax bounds one dot-separated label: RFC 1035 Section 2.3.4
// caps a label at 63 octets.
const hostLabelOctetsMax = 63

// ValidateHostName reports why name cannot be the kernel host name, or nil.
//
// RFC 1123 Section 2.1: "The syntax of a legal Internet host name was specified
// in RFC-952 [DNS:4].  One aspect of host name syntax is hereby changed: the
// restriction on the first character is relaxed to allow either a letter or a
// digit."
//
// RFC 952 ASSUMPTIONS, item 1: "The last character must not be a minus sign or
// period." Its grammar, "<name>  ::= <let>[*[<let-or-digit-or-hyphen>]<let-or-digit>]",
// read with RFC 1123's first-character change, is the label check below.
func ValidateHostName(name string) error {
	if name == "" {
		return errHostNameEmpty
	}
	if len(name) > hostNameOctetsMax {
		return fmt.Errorf("is %d bytes long, and the kernel host name holds at most %d", len(name), hostNameOctetsMax)
	}
	for label := range strings.SplitSeq(name, ".") {
		if err := validateHostLabel(label); err != nil {
			return err
		}
	}
	return nil
}

// validateHostLabel judges one dot-separated label.
func validateHostLabel(label string) error {
	if label == "" {
		return errHostLabelEmpty
	}
	if len(label) > hostLabelOctetsMax {
		return fmt.Errorf("has the label %q of %d bytes, and a label holds at most %d", label, len(label), hostLabelOctetsMax)
	}
	for i := range len(label) {
		c := label[i]
		if !isHostLabelByte(c) {
			return fmt.Errorf("has %q in the label %q, and a label holds only letters, digits and hyphens", c, label)
		}
	}
	if label[0] == '-' {
		return fmt.Errorf("has the label %q starting with a hyphen", label)
	}
	if label[len(label)-1] == '-' {
		return fmt.Errorf("has the label %q ending with a hyphen", label)
	}
	return nil
}

// isHostLabelByte reports whether c is a letter, a digit or a hyphen.
func isHostLabelByte(c byte) bool {
	if c >= 'a' && c <= 'z' {
		return true
	}
	if c >= 'A' && c <= 'Z' {
		return true
	}
	if c >= '0' && c <= '9' {
		return true
	}
	return c == '-'
}

// ConfiguredHostName answers the `system host` leaf as written and expanded,
// and whether the config sets it at all. It never answers the os.Hostname
// fallback ExtractSystemConfig fills in: a caller that sets the kernel host
// name from it must leave the kernel alone when the leaf is unset.
func ConfiguredHostName(tree *config.Tree) (string, bool) {
	if tree == nil {
		return "", false
	}
	sys := tree.GetContainer("system")
	if sys == nil {
		return "", false
	}
	raw, ok := sys.Get("host")
	if !ok || raw == "" {
		return "", false
	}
	return ExpandEnvValue(raw), true
}

// ConfiguredHostNameFromMap is ConfiguredHostName over the map form a reload
// carries.
func ConfiguredHostNameFromMap(tree map[string]any) (string, bool) {
	sys, _ := tree["system"].(map[string]any)
	if sys == nil {
		return "", false
	}
	raw, _ := sys["host"].(string)
	if raw == "" {
		return "", false
	}
	return ExpandEnvValue(raw), true
}

// validateSystemHostTree refuses a `system host` that is not a valid host name
// once its $ENV_VAR reference is expanded, so `ze config validate` and a commit
// refuse it before any applier sees it. A reference to a variable that is not
// set is left alone: its value comes from the daemon's environment, which an
// offline check does not hold, and ExpandEnvValue keeps the literal then.
func validateSystemHostTree(tree *config.Tree) error {
	sys := tree.GetContainer("system")
	if sys == nil {
		return nil
	}
	raw, ok := sys.Get("host")
	if !ok {
		return nil
	}
	name := ExpandEnvValue(raw)
	if strings.HasPrefix(raw, "$") && name == raw {
		return nil
	}
	if err := ValidateHostName(name); err != nil {
		shown := name
		if name != raw {
			shown = name + " (from " + raw + ")"
		}
		return fmt.Errorf("system host %q %w; fix: set system host to letters, digits and hyphens, "+
			"at most 63 per dot-separated label and 64 in all, for example cpe-lab", shown, err)
	}
	return nil
}
