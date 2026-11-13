// Design: docs/plugin-development/protocol.md -- canonical forwarding publication JSON.
// Related: yang/ze-sysrib-api.yang -- resolved publication leaf constraints.

package sysrib

import (
	"encoding/json"
	"net/netip"
	"strings"
	"testing"

	gyang "github.com/openconfig/goyang/pkg/yang"

	configyang "github.com/ze-software/ze/internal/component/config/yang"
	sysribevents "github.com/ze-software/ze/internal/component/sysrib/events"
	"github.com/ze-software/ze/internal/core/bgp/routeaction"
	"github.com/ze-software/ze/internal/core/family"
	"github.com/ze-software/ze/internal/core/redistevents"
)

func routeViewPublicationSchema(t *testing.T) (*configyang.Validator, *gyang.Entry) {
	t.Helper()
	schema, err := configyang.DefaultLoader()
	if err != nil {
		t.Fatal(err)
	}
	module := schema.GetEntry("ze-sysrib-api")
	if module == nil {
		t.Fatal("system RIB API schema is not registered")
	}
	method := module.Dir["route-view-publish"]
	if method == nil || method.RPC == nil || method.RPC.Input == nil {
		t.Fatal("publication RPC input is missing")
	}
	batch := method.RPC.Input.Dir["batch"]
	if batch == nil || batch.Dir["changes"] == nil {
		t.Fatal("publication changes schema is missing")
	}
	return configyang.NewValidator(schema), batch.Dir["changes"]
}

func routeViewPublicationSchemaChange(t *testing.T, entry sysribevents.BestChangeEntry) map[string]any {
	t.Helper()
	data, err := json.Marshal(sysribevents.BestChangeBatch{
		Family: family.IPv4Unicast, Changes: []sysribevents.BestChangeEntry{entry},
	})
	if err != nil {
		t.Fatal(err)
	}
	var wire struct {
		Changes []map[string]any `json:"changes"`
	}
	if err := json.Unmarshal(data, &wire); err != nil {
		t.Fatal(err)
	}
	if len(wire.Changes) != 1 {
		t.Fatalf("canonical event has %d changes", len(wire.Changes))
	}
	return wire.Changes[0]
}

// Canonical withdrawals retain an explicit empty protocol. Validate that value
// against the actual registered RPC leaf, with an ordinary gateway as control.
func TestRouteViewPublicationSchemaCanonicalWithdrawal(t *testing.T) {
	validator, changes := routeViewPublicationSchema(t)
	leaf := changes.Dir["protocol"]
	if leaf == nil || leaf.Type == nil {
		t.Fatal("publication protocol schema is missing")
	}
	for _, entry := range []sysribevents.BestChangeEntry{
		routeViewOutputBatch().Changes[0],
		{Action: routeaction.Withdraw, Prefix: netip.MustParsePrefix("192.0.2.0/24")},
	} {
		wire := routeViewPublicationSchemaChange(t, entry)
		value, present := wire["protocol"]
		if !present {
			t.Fatal("canonical event omitted its protocol field")
		}
		if err := validator.ValidateLeafType("route-view-publish/batch/changes/protocol", leaf, leaf.Type, value); err != nil {
			t.Errorf("canonical %s protocol %q was refused: %v", entry.Action, value, err)
		}
	}
}

// Device-only ECMP and repair members serialize an explicit empty address.
// Each real publication leaf must accept that sentinel and the gateway control.
func TestRouteViewPublicationSchemaCanonicalDeviceMembers(t *testing.T) {
	validator, changes := routeViewPublicationSchema(t)
	for _, group := range []string{"ecmp-paths", "backup"} {
		t.Run(group, func(t *testing.T) {
			member := changes.Dir[group]
			if member == nil || member.Dir["next-hop"] == nil || member.Dir["interface"] == nil {
				t.Fatal("publication member schema is missing")
			}
			for _, gateway := range []netip.Addr{netip.MustParseAddr("198.51.100.2"), {}} {
				entry := routeViewOutputBatch().Changes[0]
				hop := sysribevents.ECMPPath{NextHop: gateway, Interface: "port2", Weight: 7}
				entry.ECMPPaths = []sysribevents.ECMPPath{hop}
				entry.Backup = []sysribevents.ECMPPath{hop}
				wire := routeViewPublicationSchemaChange(t, entry)
				members, ok := wire[group].([]any)
				if !ok || len(members) != 1 {
					t.Fatalf("canonical %s members are missing: %v", group, wire[group])
				}
				values, ok := members[0].(map[string]any)
				if !ok {
					t.Fatalf("canonical %s member is not an object", group)
				}
				for _, name := range []string{"next-hop", "interface"} {
					value, present := values[name]
					if !present {
						t.Fatalf("canonical %s omitted %s", group, name)
					}
					leaf := member.Dir[name]
					if err := validator.ValidateLeafType(group+"/"+name, leaf, leaf.Type, value); err != nil {
						t.Errorf("canonical %s %s %q was refused: %v", group, name, value, err)
					}
				}
			}
		})
	}
}

// Allowing publication sentinels must retain lexical and size constraints.
// The selected-view gateway leaf continues to refuse an explicit empty value.
func TestRouteViewPublicationSchemaRetainsTargetBounds(t *testing.T) {
	validator, changes := routeViewPublicationSchema(t)
	for _, test := range []struct {
		name  string
		leaf  *gyang.Entry
		value string
	}{
		{"protocol-length", changes.Dir["protocol"], strings.Repeat("p", 65)},
		{"primary-address", changes.Dir["next-hop"], "invalid-gateway"},
		{"ecmp-address", changes.Dir["ecmp-paths"].Dir["next-hop"], "invalid-gateway"},
		{"backup-address", changes.Dir["backup"].Dir["next-hop"], "invalid-gateway"},
		{"ecmp-interface-empty", changes.Dir["ecmp-paths"].Dir["interface"], ""},
		{"backup-interface-length", changes.Dir["backup"].Dir["interface"], strings.Repeat("p", 256)},
	} {
		t.Run(test.name, func(t *testing.T) {
			if test.leaf == nil || test.leaf.Type == nil {
				t.Fatal("bounded publication leaf is missing")
			}
			if err := validator.ValidateLeafType(test.name, test.leaf, test.leaf.Type, test.value); err == nil {
				t.Fatalf("publication schema accepted invalid value %q", test.value)
			}
		})
	}
	schema, err := configyang.DefaultLoader()
	if err != nil {
		t.Fatal(err)
	}
	selected := schema.GetEntry("ze-sysrib-api").Dir["route-view-read"].RPC.Output.Dir["entries"].Dir["path"].Dir["ecmp"].Dir["next-hop"]
	if err := validator.ValidateLeafType("route-view-read/entries/path/ecmp/next-hop", selected, selected.Type, ""); err == nil {
		t.Fatal("selected-view schema accepted an explicit empty gateway")
	}
}

// Canonical sentinel support cannot grant ownerless installations or targetless
// members. Exercise the actual JSON decoder and common runtime batch validator.
func TestRouteViewPublicationSchemaKeepsRuntimeAuthorization(t *testing.T) {
	redistevents.RegisterProtocol("static")
	for _, test := range []struct {
		name string
		edit func(*sysribevents.BestChangeEntry)
	}{
		{"add-empty-protocol", func(entry *sysribevents.BestChangeEntry) { entry.Protocol = "" }},
		{"update-empty-protocol", func(entry *sysribevents.BestChangeEntry) {
			entry.Action = routeaction.Update
			entry.Protocol = ""
		}},
		{"ecmp-no-target", func(entry *sysribevents.BestChangeEntry) { entry.ECMPPaths = []sysribevents.ECMPPath{{}} }},
		{"backup-no-target", func(entry *sysribevents.BestChangeEntry) { entry.Backup = []sysribevents.ECMPPath{{}} }},
	} {
		t.Run(test.name, func(t *testing.T) {
			batch := routeViewOutputBatch()
			test.edit(&batch.Changes[0])
			data, err := json.Marshal(batch)
			if err != nil {
				t.Fatal(err)
			}
			decoded, err := routeViewDecodeOutputBatch(data)
			if err != nil {
				t.Fatalf("canonical JSON did not reach runtime authorization: %v", err)
			}
			if err := routeViewValidateOutput(&decoded); err == nil {
				t.Fatal("runtime publication accepted missing ownership or target")
			}
			control := routeViewOutputBatch()
			if err := routeViewValidateOutput(&control); err != nil {
				t.Fatalf("healthy gateway control was refused: %v", err)
			}
		})
	}
}
