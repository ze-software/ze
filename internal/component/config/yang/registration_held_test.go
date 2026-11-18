package yang

import (
	"testing"
)

// registrationHeldSource is the ze:reference source the tests below hold.
const registrationHeldSource = "registration-held-probe"

// registrationHeldSet answers a resolved set of the embedded modules and one
// compiled-in module, text, with registrationHeldSource registered for the
// test's duration.
func registrationHeldSet(t *testing.T, text string) *Resolved {
	t.Helper()
	referenceSources[registrationHeldSource] = func(map[string]any, []string) ReferenceAnswer { return ReferenceAnswer{} }
	t.Cleanup(func() { delete(referenceSources, registrationHeldSource) })
	loader := NewLoader()
	if err := loader.LoadEmbedded(); err != nil {
		t.Fatalf("LoadEmbedded: %v", err)
	}
	if err := loader.AddModuleFromText("ze-held-conf.yang", text); err != nil {
		t.Fatalf("add module: %v", err)
	}
	set, err := loader.Resolve()
	if err != nil {
		t.Fatalf("resolve: %v", err)
	}
	return set
}

// TestCompiledSetHoldingEveryNameIsAccepted: a compiled-in module whose
// ze:validate and ze:reference names are all held is accepted.
//
// VALIDATES: the compiled-in judge refuses only an unheld name.
// PREVENTS: a judge that refuses every module carrying an extension.
func TestCompiledSetHoldingEveryNameIsAccepted(t *testing.T) {
	set := registrationHeldSet(t, `module ze-held-conf {
    namespace "urn:ze:held";
    prefix held;
    import ze-extensions { prefix ze; }
    container held {
        leaf checked { type string; ze:validate "`+joinKnownValidator+`"; }
        leaf named { type string; ze:reference "`+registrationHeldSource+`"; }
    }
}`)
	if err := RefuseUnheldCompiledRegistrations(set, joinValidators()); err != nil {
		t.Fatalf("a set holding every name was refused: %v", err)
	}
}

// TestCompiledSetNamingUnheldValidatorIsRefused: a compiled-in module whose
// leaf names a ze:validate function the registry lacks is refused as a build
// defect, naming the module, the extension, the name and the path. One unheld
// name of a "|" list is enough, as the join judges an external module.
//
// VALIDATES: the compiled-in side of the rule the join applies (AC-11).
// PREVENTS: a leaf validated by nothing, accepted with a log line alone.
func TestCompiledSetNamingUnheldValidatorIsRefused(t *testing.T) {
	set := registrationHeldSet(t, `module ze-held-conf {
    namespace "urn:ze:held";
    prefix held;
    import ze-extensions { prefix ze; }
    container held {
        leaf checked { type string; ze:validate "`+joinKnownValidator+`|held-no-such-check"; }
    }
}`)
	err := RefuseUnheldCompiledRegistrations(set, joinValidators())
	requireJoinRefusal(t, nil, err,
		`Module "ze-held-conf", compiled into ze`, `ze:validate "held-no-such-check"`, "/held/checked",
		"this ze was built wrong")
}

// TestCompiledSetNamingUnheldReferenceSourceIsRefused: the same refusal for a
// ze:reference source no component registered.
//
// VALIDATES: the compiled-in judge reads the reference sources too.
// PREVENTS: a leaf that refuses every value at commit, loaded in silence.
func TestCompiledSetNamingUnheldReferenceSourceIsRefused(t *testing.T) {
	set := registrationHeldSet(t, `module ze-held-conf {
    namespace "urn:ze:held";
    prefix held;
    import ze-extensions { prefix ze; }
    container held {
        leaf named { type string; ze:reference "held-no-such-source"; }
    }
}`)
	err := RefuseUnheldCompiledRegistrations(set, joinValidators())
	requireJoinRefusal(t, nil, err,
		`Module "ze-held-conf", compiled into ze`, `ze:reference "held-no-such-source"`, "/held/named")
}
