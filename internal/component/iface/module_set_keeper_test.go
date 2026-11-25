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
