package firewall

import (
	"strings"
	"testing"

	"github.com/ze-software/ze/internal/component/config/yang"
)

// TestBackendGateUsedBeforeAPublishRefusesThePublish: the backend gate builds
// its config schema once for the life of the process, so it is correct only
// when its first use comes after the process published the modules of its
// external plugins. Method: run the gate, then publish a module.
//
// VALIDATES: validateBackendGate records its schema as a keeper of the module
// set, and a publish that comes after a use is refused, naming it.
// PREVENTS: the gate judging a commit, for the life of the daemon, with a
// schema that lacks the external modules.
func TestBackendGateUsedBeforeAPublishRefusesThePublish(t *testing.T) {
	t.Cleanup(yang.ResetPublishedModulesForTest())

	if err := validateBackendGate(nil, ""); err != nil {
		t.Fatalf("validateBackendGate with no section: %v", err)
	}

	err := yang.PublishExternalModules([]yang.ExternalModule{{
		Plugin: "acme", Module: "acme-monitor-conf", Namespace: "urn:acme:monitor",
		Text: `module acme-monitor-conf {
    namespace "urn:acme:monitor";
    prefix acme;
    container acme-monitor {
        leaf interval { type uint8; }
    }
}`,
	}}, yang.NewValidatorRegistry())
	if err == nil {
		t.Fatal("a publish after the backend gate ran was accepted")
	}
	if !strings.Contains(err.Error(), backendGateSchemaKeeper) {
		t.Errorf("the refusal does not name the backend gate schema:\n%v", err)
	}
	if _, published := yang.PublishedExternalModules(); published {
		t.Error("the refused publish is recorded as a publish")
	}
}
