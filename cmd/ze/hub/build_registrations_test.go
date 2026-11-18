package hub

import (
	"strings"
	"testing"

	configyang "github.com/ze-software/ze/internal/component/config/yang"
	pluginserver "github.com/ze-software/ze/internal/component/plugin/server"
	"github.com/ze-software/ze/internal/core/statestore"
)

// TestStartAcceptsTheShippedBuild: this test binary links the composition
// root, so every name its compiled-in modules use is held, and the start goes
// on.
//
// VALIDATES: refuseWronglyBuilt passes a ze built right.
// PREVENTS: a check that ends every start.
func TestStartAcceptsTheShippedBuild(t *testing.T) {
	if refuseWronglyBuilt() {
		t.Fatal("the start was refused for the shipped module set")
	}
}

// TestStartRefusesABuildNamingAnUnheldValidator: a compiled-in module that
// names a ze:validate function no package registers ends the start, and the
// last-stop record names the build defect for the next `ze doctor`.
//
// Method: a module registered for the test's duration (the seam the plugin/all
// test uses) adds one such leaf; a store is set so the record is written.
//
// VALIDATES: ze start refuses a wrongly built ze before it parses the config,
// under doctor-build-registration-unheld.
// PREVENTS: a leaf checked by nothing, with a log line as the only word.
func TestStartRefusesABuildNamingAnUnheldValidator(t *testing.T) {
	const module = `module ze-test-hub-unheld-conf {
    namespace "urn:ze:test:hub-unheld";
    prefix hubunheld;
    import ze-extensions { prefix ze; }
    container ze-test-hub-unheld {
        leaf target {
            type string;
            ze:validate "ze-test-hub-no-such-check";
        }
    }
}`
	t.Cleanup(configyang.RegisterModuleForTest("ze-test-hub-unheld-conf.yang", module))
	statestore.SetStore(newTestStore(t, t.TempDir()))
	t.Cleanup(func() { statestore.SetStore(nil) })

	if !refuseWronglyBuilt() {
		t.Fatal("the start went on with a compiled-in module naming an unregistered ze:validate function")
	}
	record, found, err := pluginserver.ReadLastStop()
	if err != nil {
		t.Fatalf("read the last-stop record: %v", err)
	}
	if !found || len(record.Entries) == 0 {
		t.Fatal("the refused start left no last-stop record")
	}
	entry := record.Entries[len(record.Entries)-1]
	if entry.Reason != pluginserver.LastStopBuildRegistrationUnheld {
		t.Fatalf("the last-stop entry is %v, want the build-registration reason", entry.Reason)
	}
	for _, want := range []string{`"ze-test-hub-unheld-conf"`, `ze:validate "ze-test-hub-no-such-check"`, "this ze was built wrong"} {
		if !strings.Contains(entry.Detail, want) {
			t.Errorf("the record's detail lacks %q: %s", want, entry.Detail)
		}
	}
	if !strings.Contains(entry.Fix, "rebuild ze") {
		t.Errorf("the record's fix is %q, want the rebuild", entry.Fix)
	}
}
