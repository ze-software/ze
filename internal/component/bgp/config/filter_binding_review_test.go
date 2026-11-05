// Design: docs/architecture/api/process-protocol.md -- Filter Insertion.
// Related: filter_state_binding.go -- grants follow the configured process identity.

package bgpconfig

import (
	"slices"
	"strings"
	"testing"

	"github.com/ze-software/ze/internal/component/config"
	pluginserver "github.com/ze-software/ze/internal/component/plugin/server"
	"github.com/ze-software/ze/internal/component/policy"
	bgpevents "github.com/ze-software/ze/internal/core/bgp/events"
	"github.com/ze-software/ze/internal/core/events"
	"github.com/ze-software/ze/pkg/plugin/rpc"
)

// TestFilterOwnerAliasDelivery follows parsed config through the derived grant
// into the real delivery index, including the state channel used for removal.
func TestFilterOwnerAliasDelivery(t *testing.T) {
	registerInserter(t)
	for _, process := range []string{"renamed", "fake-inserter"} {
		t.Run(process, func(t *testing.T) {
			input := "plugin { internal " + process + " { use fake-inserter; } }\n" +
				strings.Replace(insertionConfig, "%A%", "filter { import { filters [ fake-inserter:early ]; final accept; } }", 1)
			schema, err := config.YANGSchema()
			if err != nil {
				t.Fatal(err)
			}
			tree, err := config.NewParser(schema).Parse(input)
			if err != nil {
				t.Fatal(err)
			}
			peers, _, err := peersAndDynamicGroups(tree)
			if err != nil {
				t.Fatal(err)
			}
			srv := &pluginserver.Server{}
			srv.UpdateDeliveryGraph(bgpevents.Namespace, deliveryPeers(peers))
			graph := srv.DeliveryGraph()
			got := fedBy(graph, bgpevents.EventState, events.DirUnspecified, "192.0.2.1")
			if !slices.Contains(got, process) {
				t.Fatalf("state/removal recipients = %v, missing configured process %s", got, process)
			}
			if !slices.Contains(got, "bgp-rib") {
				t.Fatalf("state recipients = %v, missing independent RIB grant", got)
			}
			if process == "renamed" {
				if slices.Contains(got, "fake-inserter") {
					t.Fatalf("state/removal granted to non-running canonical owner: %v", got)
				}
			}
			for _, recipient := range fedBy(graph, bgpevents.EventState, events.DirUnspecified, "192.0.2.2") {
				if recipient == process {
					t.Fatalf("unrelated peer grants state to filter owner %s", recipient)
				}
				if recipient == "fake-inserter" {
					t.Fatalf("unrelated peer grants state to canonical filter owner %s", recipient)
				}
			}
			for _, recipient := range fedBy(graph, bgpevents.EventUpdate, events.DirReceived, "192.0.2.1") {
				if recipient == process {
					t.Fatalf("filter ownership grants unrelated updates to %s", recipient)
				}
				if recipient == "fake-inserter" {
					t.Fatalf("filter ownership grants unrelated updates to %s", recipient)
				}
			}
		})
	}
}

// TestInsertedExternalAttachmentGuard makes an inserted declaration cross the
// same config boundary as an explicit reference, on static and dynamic peers.
func TestInsertedExternalAttachmentGuard(t *testing.T) {
	for _, field := range []string{"tag", "prefix"} {
		for _, dynamic := range []bool{false, true} {
			name := field
			if dynamic {
				name += " dynamic"
			}
			t.Run(name, func(t *testing.T) {
				ins := rpc.FilterInsertion{Filter: "fake-inserter:late", Direction: rpc.FilterImport, Priority: 5, Config: rrClient}
				if dynamic {
					ins.Config = nil
					ins.Session = bilateralEBGP[:1]
				}
				registerInserter(t, ins)
				clearPending(t)
				declareRuntime(t, "fake-inserter", []rpc.FilterDecl{{Name: "late", Direction: rpc.FilterBoth, Reads: []string{field}}})
				reg := emptyPolicyRegistry(t)
				explicit := validateChainNames(reg, []policy.Ref{{Name: ins.Filter}}, dynamic, "bgp import", "explicit import")
				var err error
				if dynamic {
					_, err = loadTemplated(t, "ALLOW-65010", "")
				} else {
					_, err = insertionPeers(t, "")
				}
				if field == "tag" {
					requireRefusal(t, explicit, ins.Filter, "tag", "bgp import")
					requireRefusal(t, err, ins.Filter, "tag", "bgp import")
					return
				}
				if explicit != nil {
					t.Fatal(explicit)
				}
				if err != nil {
					t.Fatalf("valid inserted declaration refused: %v", err)
				}
			})
		}
	}
}
