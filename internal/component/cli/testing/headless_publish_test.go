package testing

import (
	"errors"
	gotesting "testing"

	"github.com/ze-software/ze/internal/component/config/yang"
)

// TestHeadlessCommandTreeFollowsThePublishedModuleSet: the headless harness
// can build its command tree before the config that declares an external
// plugin is loaded. Method: ask for the tree, publish a module, ask again, and
// read which module set the kept tree was built from.
//
// VALIDATES: the tree is kept for the module set it was built from, so after a
// publish it is the tree of the set yang.DefaultLoader answers.
// PREVENTS: a tree built once for the process answering from the module set
// that held no external module.
func TestHeadlessCommandTreeFollowsThePublishedModuleSet(t *gotesting.T) {
	t.Cleanup(yang.ResetPublishedModulesForTest())

	// A binary that links no -cmd module answers errCommandTreeIsEmpty; the
	// tree is still built and kept for its set, which is what this test reads.
	requireTreeBuilt(t, "before the publish")
	setBefore, err := yang.DefaultLoader()
	if err != nil {
		t.Fatalf("DefaultLoader before the publish: %v", err)
	}
	if headlessTreeMemo.set != setBefore {
		t.Fatal("the tree was not built from the set DefaultLoader answers")
	}

	err = yang.PublishExternalModules([]yang.ExternalModule{{
		Plugin: "acme", Module: "acme-monitor-conf", Namespace: "urn:acme:monitor",
		Text: `module acme-monitor-conf {
    namespace "urn:acme:monitor";
    prefix acme;
    container acme-monitor {
        leaf interval { type uint8; }
    }
}`,
	}}, yang.NewValidatorRegistry())
	if err != nil {
		t.Fatalf("publish: %v", err)
	}
	setAfter, err := yang.DefaultLoader()
	if err != nil {
		t.Fatalf("DefaultLoader after the publish: %v", err)
	}
	if setAfter == setBefore {
		t.Fatal("the publish did not change the set DefaultLoader answers")
	}

	requireTreeBuilt(t, "after the publish")
	if headlessTreeMemo.set != setAfter {
		t.Error("the tree is still the one of the set before the publish")
	}
}

// requireTreeBuilt asks for the headless command tree and fails the test on
// any error but errCommandTreeIsEmpty.
func requireTreeBuilt(t *gotesting.T, stage string) {
	t.Helper()
	_, err := headlessCommandTree()
	if err == nil {
		return
	}
	if !errors.Is(err, errCommandTreeIsEmpty) {
		t.Fatalf("headlessCommandTree %s: %v", stage, err)
	}
}
