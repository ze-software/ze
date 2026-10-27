// Design: docs/architecture/config/yang-config-design.md -- mandatory nodes
// Related: config.go -- parseInterfaceQoS, which reads an absent qdisc as none
package traffic_test

import (
	"strings"
	"testing"

	"github.com/ze-software/ze/internal/component/config"
	_ "github.com/ze-software/ze/internal/component/traffic/yang"
)

// TestTrafficQdiscIsPresence proves `traffic control interface X qdisc` is a
// presence container, so its mandatory type is required only when the qdisc
// is configured.
//
// VALIDATES: an interface entry with no qdisc loads, as parseInterfaceQoS reads
// an absent qdisc as "no qdisc"; a qdisc with a default class and no type is
// refused, naming the type.
// PREVENTS: the RFC 7950 Section 7.6.5 walk treating qdisc as a non-presence
// container, which required a type in every interface entry and refused an
// entry the consumer reads as "no qdisc".
//
// Method: config.LoadConfig over the two configs.
func TestTrafficQdiscIsPresence(t *testing.T) {
	bare := "traffic {\n\tcontrol {\n\t\tinterface eth0 {\n\t\t}\n\t}\n}\n"
	if _, err := config.LoadConfig(bare, "test.conf", nil); err != nil {
		t.Errorf("LoadConfig refused a traffic interface with no qdisc: %v", err)
	}

	noType := "traffic {\n\tcontrol {\n\t\tinterface eth0 {\n\t\t\tqdisc {\n\t\t\t\tdefault-class bulk\n\t\t\t}\n\t\t}\n\t}\n}\n"
	_, err := config.LoadConfig(noType, "test.conf", nil)
	if err == nil {
		t.Fatal("LoadConfig accepted a qdisc with no type")
	}
	if !strings.Contains(err.Error(), "traffic/control/interface[eth0]/qdisc/type") {
		t.Errorf("refusal %q does not name the qdisc type", err)
	}
}

func TestTrafficRootTBFRateSchema(t *testing.T) {
	text := "traffic {\n control {\n  interface eth0 {\n   qdisc {\n    type tbf\n    rate 10mbit\n   }\n  }\n }\n}\n"
	if _, err := config.LoadConfig(text, "test.conf", nil); err != nil {
		t.Fatalf("classless root rate refused: %v", err)
	}
}
