package all

import (
	"strings"
	"testing"

	"github.com/ze-software/ze/internal/component/config"
	configyang "github.com/ze-software/ze/internal/component/config/yang"
)

// TestCompiledModulesNameOnlyHeldRegistrations: every ze:validate function and
// every ze:reference source a module compiled into ze names is registered in
// this binary, which links the composition root and so every module and every
// registering package the build carries under the tags the test runs with.
//
// Method: config.RefuseUnheldRegistrations judges the module set DefaultLoader
// answers with the judge the join applies to an external module.
//
// VALIDATES: a shipped ze, under the feature tags of this run, holds every
// registration its YANG names (a gated module's names are registered by a
// package under the same gate).
// PREVENTS: a module that names a validator no package registers, whose leaf
// would then accept every value its type admits with no word to anyone.
func TestCompiledModulesNameOnlyHeldRegistrations(t *testing.T) {
	if err := config.RefuseUnheldRegistrations(defaultSet(t)); err != nil {
		t.Fatalf("a compiled-in module names a registration this build does not hold: %v", err)
	}
}

// defaultSet answers the module set DefaultLoader resolves in this binary.
func defaultSet(t *testing.T) *configyang.Resolved {
	t.Helper()
	set, err := configyang.DefaultLoader()
	if err != nil {
		t.Fatalf("resolve the compiled-in YANG modules: %v", err)
	}
	return set
}

// TestCompiledModuleNamingUnheldValidatorIsRefused: the check above goes red
// when a module of this binary names a ze:validate function no package
// registers, naming the module, the extension, the name and the schema path.
//
// Method: a module registered for the test's duration adds one such leaf to
// the full compiled-in set.
//
// VALIDATES: TestCompiledModulesNameOnlyHeldRegistrations can fail.
// PREVENTS: a check that answers held for every set.
func TestCompiledModuleNamingUnheldValidatorIsRefused(t *testing.T) {
	const module = `module ze-test-unheld-validate-conf {
    namespace "urn:ze:test:unheld-validate";
    prefix unheldvalidate;
    import ze-extensions { prefix ze; }
    container ze-test-unheld {
        leaf target {
            type string;
            ze:validate "ze-test-no-such-check";
        }
    }
}`
	t.Cleanup(configyang.RegisterModuleForTest("ze-test-unheld-validate-conf.yang", module))

	err := config.RefuseUnheldRegistrations(defaultSet(t))
	if err == nil {
		t.Fatal("a compiled-in module naming an unregistered ze:validate function was not refused")
	}
	for _, want := range []string{
		`"ze-test-unheld-validate-conf"`, "compiled into ze", `ze:validate "ze-test-no-such-check"`,
		"/ze-test-unheld/target", "this ze was built wrong",
	} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("refusal lacks %q: %v", want, err)
		}
	}
}
