package client

import (
	"testing"

	"github.com/ze-software/ze/internal/component/config/yang"
)

// publishedMonitorModule is a module an external plugin declares: its own
// root, no import, so it joins any compiled-in set.
const publishedMonitorModule = `module acme-monitor-conf {
    namespace "urn:acme:monitor";
    prefix acme;
    container acme-monitor {
        leaf interval { type uint8 { range "1..10"; } }
    }
}`

// TestYANGStateFollowsThePublishedModuleSet: the YANG state this package
// derives is built before the daemon publishes the modules of its external
// plugins, because `ze` routes its own command line with it. Method: build the
// state, publish a module, and read the state again: it MUST be the state of
// the set yang.DefaultLoader answers after the publish.
//
// VALIDATES: the cli/client YANG state is keyed on the module set it was built
// from, so a publish is followed with no restart of the process.
// PREVENTS: a state built once for the process answering, for the life of the
// daemon, from the module set that held no external module.
func TestYANGStateFollowsThePublishedModuleSet(t *testing.T) {
	t.Cleanup(yang.ResetPublishedModulesForTest())

	if _, err := YANGCommandTree(); err != nil {
		t.Fatalf("YANGCommandTree before the publish: %v", err)
	}
	setBefore, err := yang.DefaultLoader()
	if err != nil {
		t.Fatalf("DefaultLoader before the publish: %v", err)
	}
	if yangStateMemo.set != setBefore {
		t.Fatal("the state was not built from the set DefaultLoader answers")
	}
	stateBefore := yangStateMemo.state

	err = yang.PublishExternalModules([]yang.ExternalModule{{
		Plugin: "acme", Module: "acme-monitor-conf", Namespace: "urn:acme:monitor", Text: publishedMonitorModule,
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

	if _, err := YANGCommandTree(); err != nil {
		t.Fatalf("YANGCommandTree after the publish: %v", err)
	}
	if yangStateMemo.set != setAfter {
		t.Error("the state is still the one of the set before the publish")
	}
	if yangStateMemo.state == stateBefore {
		t.Error("the state was not built again for the published set")
	}
}
