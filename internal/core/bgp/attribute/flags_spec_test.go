// Related: flags_spec.go — flagsSpecs, the declaration these tests read
//
// VALIDATES: the specified Optional and Transitive values are declared once, per
// attribute, and every declared rule carries the handling its specification mandates.
// PREVENTS: a conflicting flag bit reaching a caller as "no conflict", which is what the
// zero value of FlagsConflict would look like if a rule were declared without a handling.
package attribute

import "testing"

// TestFlagsSpecDeclaresHandlingForEveryRule holds the authoring hole shut.
//
// A rule that fixes a bit value but declares no handling for a conflict returns
// FlagsConflictUnspecified, and a caller cannot tell that from a verdict it does not know.
// The mandate is checked with it, because an operator reads the mandate to find the section
// that decided the verdict.
func TestFlagsSpecDeclaresHandlingForEveryRule(t *testing.T) {
	for code, spec := range flagsSpecs {
		for name, requirement := range map[string]flagRequirement{
			"optional":   spec.optional,
			"transitive": spec.transitive,
		} {
			if requirement.rule == flagRuleUnspecified {
				continue
			}
			if requirement.conflict == FlagsConflictUnspecified {
				t.Errorf("attribute %d: the %s rule declares no handling for a conflict", code, name)
			}
			if requirement.mandate == "" {
				t.Errorf("attribute %d: the %s rule names no section that mandates its handling", code, name)
			}
		}
	}
}

// TestEveryRecognizedAttributeDeclaresItsFlags reads the recognition registry rather than a
// list, so an attribute a plugin adds tomorrow is inside the population automatically.
//
// VALIDATES: RFC 7606 Section 3.c reaches every attribute ze holds a meaning for, plugin
// attributes included. Recognition and the flags declaration are written by one function
// (RegisterName), and this is the assertion that says they cannot come apart.
// PREVENTS: the state BGP-LS (29) and OTC (35) were in until 2026-09-20, recognized by the
// names registry and absent from the flags declaration, so a conflicting Optional or
// Transitive bit on either of them was accepted.
func TestEveryRecognizedAttributeDeclaresItsFlags(t *testing.T) {
	for code := range 256 {
		attr := AttributeCode(code)
		if !attr.Recognized() {
			continue
		}
		if !flagsSpecs[code].declared() {
			t.Errorf("attribute %d (%s) is recognized and fixes no Optional or Transitive "+
				"value, so RFC 7606 Section 3.c judges no flags octet it carries", code, attr)
		}
	}
}

// TestRegisterNameRefusesAnUndeclaredSpec pins the startup refusal.
//
// An attribute registered with the zero FlagsSpec would be recognized and unjudged, which
// is the hole the two-argument registration left open. The panic is the answer because the
// call site is an init(): a daemon that cannot judge an attribute's flags must not reach
// the socket (ai/rules/principles.md, a value that is silently wrong must not be
// reachable).
func TestRegisterNameRefusesAnUndeclaredSpec(t *testing.T) {
	defer func() {
		if recover() == nil {
			t.Error("RegisterName accepted a specification that fixes neither bit, which " +
				"registers an attribute ze recognizes and never judges the flags of")
		}
	}()
	RegisterName(AttributeCode(239), "TEST_ONLY_UNDECLARED", FlagsSpec{})
}

// TestFlagsConflictReadsTheDeclaration drives AttributeCode.FlagsConflict over one case for
// each shape the declaration holds.
//
// VALIDATES: the generic RFC 7606 Section 3.c verdict, the two attributes whose own
// specification mandates different handling, the strongest-action rule when both bits
// conflict, and the pass-through for an attribute ze holds no specification for.
// PREVENTS: a verdict that depends on which bit is read first, and a code with no
// declaration being judged at all.
func TestFlagsConflictReadsTheDeclaration(t *testing.T) {
	for _, one := range []struct {
		name  string
		code  AttributeCode
		flags AttributeFlags
		want  FlagsConflict
	}{
		{"well-known ORIGIN marked optional", AttrOrigin, FlagOptional | FlagTransitive, FlagsConflictTreatAsWithdraw},
		{"well-known AS_PATH not transitive", AttrASPath, 0, FlagsConflictTreatAsWithdraw},
		{"well-known ORIGIN as specified", AttrOrigin, FlagTransitive, FlagsConflictNone},
		{"MED marked transitive", AttrMED, FlagOptional | FlagTransitive, FlagsConflictTreatAsWithdraw},
		{"MED as specified", AttrMED, FlagOptional, FlagsConflictNone},
		{"CLUSTER_LIST marked transitive", AttrClusterList, FlagOptional | FlagTransitive, FlagsConflictTreatAsWithdraw},
		{"COMMUNITIES not transitive", AttrCommunity, FlagOptional, FlagsConflictTreatAsWithdraw},
		{"MP_REACH marked well-known", AttrMPReachNLRI, FlagTransitive, FlagsConflictSessionReset},
		{"MP_UNREACH as specified", AttrMPUnreachNLRI, FlagOptional, FlagsConflictNone},
		{"AIGP marked transitive", AttrAIGP, FlagOptional | FlagTransitive, FlagsConflictAttributeDiscard},
		{"AIGP marked well-known and transitive", AttrAIGP, FlagTransitive, FlagsConflictTreatAsWithdraw},
		{"AIGP as specified", AttrAIGP, FlagOptional, FlagsConflictNone},
		{"ATTR_TOMBSTONE keeps a derived transitive bit", AttrTombstone, FlagOptional | FlagTransitive, FlagsConflictNone},
		{"ATTR_TOMBSTONE marked well-known", AttrTombstone, 0, FlagsConflictTreatAsWithdraw},
		{"an attribute ze holds no specification for", AttributeCode(200), FlagTransitive, FlagsConflictNone},
	} {
		t.Run(one.name, func(t *testing.T) {
			got, mandate := one.code.FlagsConflict(one.flags)
			if got != one.want {
				t.Fatalf("FlagsConflict(%s, %#x) = %d, want %d", one.code, byte(one.flags), got, one.want)
			}
			if one.want == FlagsConflictNone && mandate != "" {
				t.Errorf("a conflict-free octet named the mandate %q", mandate)
			}
			if one.want != FlagsConflictNone && mandate == "" {
				t.Error("a conflict named no section that mandates its handling")
			}
		})
	}
}
