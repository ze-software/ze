package server

import (
	"strings"
	"testing"

	"github.com/ze-software/ze/internal/component/config/yang"
)

// TestServerBuiltBeforeAPublishRefusesThePublish: a plugin server reads the
// module set once, keeps the command maps it derives and seeds the argument
// definitions every route judges with, so it is correct only when it is built
// after the process published the modules of its external plugins. Method:
// build a server, then publish a module.
//
// VALIDATES: NewServer records the server as a keeper of the module set, and a
// publish that comes after it is refused, naming the server.
// PREVENTS: the boot order building the plugin server before the publish and
// serving, for the life of the daemon, maps that lack the external modules.
func TestServerBuiltBeforeAPublishRefusesThePublish(t *testing.T) {
	t.Cleanup(yang.ResetPublishedModulesForTest())

	if _, err := NewServer(&ServerConfig{}, nil); err != nil {
		t.Fatalf("NewServer: %v", err)
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
		t.Fatal("a publish after a plugin server was built was accepted")
	}
	if !strings.Contains(err.Error(), moduleSetKeeperName) {
		t.Errorf("the refusal does not name the plugin server:\n%v", err)
	}
	if _, published := yang.PublishedExternalModules(); published {
		t.Error("the refused publish is recorded as a publish")
	}
}
