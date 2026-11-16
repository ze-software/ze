package iface

import (
	"strings"
	"testing"

	"github.com/ze-software/ze/internal/component/config/yang"
)

// keeperProbeModule is the module an external plugin declares in these tests.
const keeperProbeModule = `module acme-monitor-conf {
    namespace "urn:acme:monitor";
    prefix acme;
    container acme-monitor {
        leaf interval { type uint8; }
    }
}`

// publishKeeperProbe publishes keeperProbeModule and answers the publish's
// error.
func publishKeeperProbe() error {
	return yang.PublishExternalModules([]yang.ExternalModule{{
		Plugin: "acme", Module: "acme-monitor-conf", Namespace: "urn:acme:monitor", Text: keeperProbeModule,
	}}, yang.NewValidatorRegistry())
}

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

	err := publishKeeperProbe()
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

// TestIfaceNameValidatedBeforeAPublishRefusesThePublish: the reserved
// interface names are read once from the command tree of the module set, so
// the set is correct only when the first name is validated after the process
// published the modules of its external plugins. Method: validate a name,
// then publish a module.
//
// VALIDATES: ValidateIfaceName records the reserved set as a keeper of the
// module set, and a publish that comes after a use is refused, naming it.
// PREVENTS: a config parsed before the publish leaving the reserved names of
// the earlier set in place with nothing reporting it.
func TestIfaceNameValidatedBeforeAPublishRefusesThePublish(t *testing.T) {
	t.Cleanup(yang.ResetPublishedModulesForTest())

	if err := ValidateIfaceName("eth0"); err != nil {
		t.Fatalf("ValidateIfaceName: %v", err)
	}

	err := publishKeeperProbe()
	if err == nil {
		t.Fatal("a publish after an interface name was validated was accepted")
	}
	if !strings.Contains(err.Error(), reservedIfaceNamesKeeper) {
		t.Errorf("the refusal does not name the reserved interface names:\n%v", err)
	}
	if _, published := yang.PublishedExternalModules(); published {
		t.Error("the refused publish is recorded as a publish")
	}
}
