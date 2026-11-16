package gnmi

import (
	"strings"
	"testing"

	"github.com/ze-software/ze/internal/component/config/yang"
)

// TestServerBuiltBeforeAPublishRefusesThePublish: a gNMI server builds its
// config schema and its model list once and keeps them, so it is correct only
// when it is built after the process published the modules of its external
// plugins. Method: build a server, then publish a module.
//
// VALIDATES: NewServer records the server as a keeper of the module set, and a
// publish that comes after it is refused, naming the server.
// PREVENTS: a gNMI server answering paths and capabilities from the module set
// that held no external module, with nothing reporting it.
func TestServerBuiltBeforeAPublishRefusesThePublish(t *testing.T) {
	t.Cleanup(yang.ResetPublishedModulesForTest())

	NewServer(Config{}, nil, nil, nil, nil)

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
		t.Fatal("a publish after a gNMI server was built was accepted")
	}
	if !strings.Contains(err.Error(), moduleSetKeeperName) {
		t.Errorf("the refusal does not name the gNMI server:\n%v", err)
	}
	if _, published := yang.PublishedExternalModules(); published {
		t.Error("the refused publish is recorded as a publish")
	}
}
