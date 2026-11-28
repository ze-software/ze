// Design: docs/architecture/config/transaction-protocol.md -- route server import as-path-prepend refusal
// RFC: rfc/short/rfc7947.md -- Section 2.2.2.1 prohibits route-server AS_PATH modification

package rs

import (
	"errors"
	"slices"
	"strings"
	"testing"

	_ "github.com/ze-software/ze/internal/component/bgp/plugin"
	_ "github.com/ze-software/ze/internal/component/bgp/plugins/filter_modify"
	_ "github.com/ze-software/ze/internal/component/bgp/plugins/rib"
	"github.com/ze-software/ze/internal/component/config"
	"github.com/ze-software/ze/internal/component/plugin/registry"
	"github.com/ze-software/ze/internal/component/policy"
	"github.com/ze-software/ze/internal/core/opererr"
	sdk "github.com/ze-software/ze/pkg/plugin/sdk"
)

// rsSectionsFromConfig parses configText through YANG and builds the bgp +
// policy sections delivered to bgp-rs at Stage 2 and reload verify.
func rsSectionsFromConfig(t *testing.T, configText string) []sdk.ConfigSection {
	t.Helper()
	tree, err := config.ParseTreeForValidation(configText)
	if err != nil {
		t.Fatalf("parse config: %v\n%s", err, configText)
	}
	sections, err := config.BuildPluginConfigSections(tree.ToPluginMap(), []string{configRootBGP, policy.RootName})
	if err != nil {
		t.Fatalf("build plugin sections: %v", err)
	}
	return sections
}

// assertRSPrependRefusal verifies that err is a structured opererr refusal
// naming the offending target, modify definition, as-path-prepend, rs-client,
// RFC 7947 Section 2.2.2.1, and a removal fix on the last line.
func assertRSPrependRefusal(t *testing.T, err error, wantTarget, wantMod string) {
	t.Helper()
	if err == nil {
		t.Fatalf("expected refusal for %s / %s, got nil", wantTarget, wantMod)
	}
	opErr, ok := errors.AsType[*opererr.Error](err)
	if !ok {
		t.Fatalf("refusal is not *opererr.Error: %T (%v)", err, err)
	}
	msg := err.Error()
	for _, want := range []string{wantTarget, wantMod, asPathPrependLeaf, rsClientLeaf, "RFC 7947 Section 2.2.2.1"} {
		if !strings.Contains(msg, want) {
			t.Errorf("refusal missing %q:\n%s", want, msg)
		}
	}
	lines := strings.Split(strings.TrimSpace(msg), "\n")
	lastLine := lines[len(lines)-1]
	if !strings.HasPrefix(lastLine, "fix: ") {
		t.Errorf("last line must start with \"fix: \", got %q\nfull error:\n%s", lastLine, msg)
	}
	if opErr.Problem() == "" {
		t.Error("opererr.Problem is empty")
	}
	if opErr.Why() == "" {
		t.Error("opererr.Why is empty")
	}
	if !strings.Contains(opErr.Fix(), "remove") {
		t.Errorf("opererr.Fix must name removal fix, got %q", opErr.Fix())
	}
}

// TestRouteServerRefusesInboundASPathPrepend proves AC-3 across offline
// validation (config.VerifyPluginConfigContent), Stage 2 configure
// (ConfigTx.Configure), and live reload verify (ConfigTx.Verify).
//
// VALIDATES: bgp-rs refuses any active import as-path-prepend modifier on any
// peer or dynamic-group template with effective session > rs-client true,
// returns a structured opererr citing RFC 7947 Section 2.2.2.1, leaves running
// worker-queue-size intact on reload refusal, fails closed on malformed JSON,
// declares FatalOnConfigError and ConfigReads["policy"] while keeping
// OptionalDependencies intact, and permits non-RS peers, rs-client false
// overrides, export-only prepend, deactivated import refs, foreign-prefixed
// refs, non-prepend modify definitions, and absent policy sections.
// PREVENTS: an ingress import as-path-prepend silently rewriting AS_PATH on
// route-server client routes before forwarding to other IXP clients.
func TestRouteServerRefusesInboundASPathPrepend(t *testing.T) {
	t.Run("registration_metadata", func(t *testing.T) {
		reg := registry.Lookup(rsPluginName)
		if reg == nil {
			t.Fatal("bgp-rs is not registered")
		}
		if !reg.FatalOnConfigError {
			t.Error("bgp-rs must declare FatalOnConfigError: true")
		}
		if !slices.Contains(reg.ConfigReads, policy.RootName) {
			t.Errorf("bgp-rs ConfigReads = %v, want %q", reg.ConfigReads, policy.RootName)
		}
		if reg.InProcessConfigVerifier == nil {
			t.Error("bgp-rs must register InProcessConfigVerifier")
		}
		if !slices.Equal(reg.Dependencies, []string{"bgp-rib"}) {
			t.Errorf("bgp-rs Dependencies = %v, want [bgp-rib]", reg.Dependencies)
		}
		if !slices.Equal(reg.OptionalDependencies, []string{"bgp-adj-rib-in"}) {
			t.Errorf("bgp-rs OptionalDependencies = %v, want [bgp-adj-rib-in]", reg.OptionalDependencies)
		}
		stage1 := routeServerRegistration()
		if !slices.Contains(stage1.WantsConfig, policy.RootName) {
			t.Errorf("routeServerRegistration WantsConfig = %v, want %q", stage1.WantsConfig, policy.RootName)
		}
	})

	const (
		prependModName = "PREPEND"
		seedQueueSize  = "8192"
		policyBlock    = `
policy {
	modify PREPEND {
		set {
			as-path-prepend 1;
		}
	}
	modify LP-ONLY {
		set {
			local-preference 200;
		}
	}
}
`
	)

	refusedCases := []struct {
		name       string
		cfg        string
		wantTarget string
		wantMod    string
	}{
		{
			name: "standalone_peer_bare_ref",
			cfg: policyBlock + `
bgp {
	router-id 1.2.3.4;
	session { asn { local 65000; } }
	peer client-a {
		connection { remote { ip 192.0.2.1; } }
		session { asn { remote 65001; } rs-client true; }
		filter { import { filters [ PREPEND ]; final accept; } }
	}
}`,
			wantTarget: "peer client-a",
			wantMod:    prependModName,
		},
		{
			name: "standalone_peer_modify_prefix",
			cfg: policyBlock + `
bgp {
	router-id 1.2.3.4;
	session { asn { local 65000; } }
	peer client-b {
		connection { remote { ip 192.0.2.2; } }
		session { asn { remote 65002; } rs-client true; }
		filter { import { filters [ modify:PREPEND ]; final accept; } }
	}
}`,
			wantTarget: "peer client-b",
			wantMod:    prependModName,
		},
		{
			name: "standalone_peer_plugin_prefix",
			cfg: policyBlock + `
bgp {
	router-id 1.2.3.4;
	session { asn { local 65000; } }
	peer client-c {
		connection { remote { ip 192.0.2.3; } }
		session { asn { remote 65003; } rs-client true; }
		filter { import { filters [ bgp-filter-modify:PREPEND ]; final accept; } }
	}
}`,
			wantTarget: "peer client-c",
			wantMod:    prependModName,
		},
		{
			name: "grouped_peer_inherited_rs_client_group_import",
			cfg: policyBlock + `
bgp {
	router-id 1.2.3.4;
	session { asn { local 65000; } }
	group ixp-clients {
		session { rs-client true; }
		filter { import { filters [ modify:PREPEND ]; final accept; } }
		peer member-a {
			connection { remote { ip 192.0.2.10; } }
			session { asn { remote 65010; } }
		}
	}
}`,
			wantTarget: "peer member-a",
			wantMod:    prependModName,
		},
		{
			name: "grouped_peer_inherited_rs_client_global_import",
			cfg: policyBlock + `
bgp {
	router-id 1.2.3.4;
	session { asn { local 65000; } }
	filter { import { filters [ PREPEND ]; final accept; } }
	group ixp-clients {
		session { rs-client true; }
		peer member-b {
			connection { remote { ip 192.0.2.11; } }
			session { asn { remote 65011; } }
		}
	}
}`,
			wantTarget: "peer member-b",
			wantMod:    prependModName,
		},
		{
			name: "dynamic_group_template",
			cfg: policyBlock + `
bgp {
	router-id 1.2.3.4;
	session { asn { local 65000; } }
	group rs-clients {
		connection { remote { ip dynamic; range [ 198.51.100.0/24 ]; } }
		session { router-id 1.2.3.4; rs-client true; }
		filter { import { filters [ modify:PREPEND ]; final accept; } }
	}
}`,
			wantTarget: "group rs-clients",
			wantMod:    prependModName,
		},
	}

	for _, tc := range refusedCases {
		t.Run("refuse_"+tc.name, func(t *testing.T) {
			validateErr := config.VerifyPluginConfigContent(tc.cfg)
			assertRSPrependRefusal(t, validateErr, tc.wantTarget, tc.wantMod)

			sections := rsSectionsFromConfig(t, tc.cfg)

			_, stage2Tx := newQueueSizeTestTx(t, false)
			assertRSPrependRefusal(t, stage2Tx.Configure(sections), tc.wantTarget, tc.wantMod)

			wp, reloadTx := newQueueSizeTestTx(t, false)
			if err := reloadTx.Configure(queueSizeSection(seedQueueSize)); err != nil {
				t.Fatalf("seed running config: %v", err)
			}
			assertRSPrependRefusal(t, reloadTx.Verify(sections), tc.wantTarget, tc.wantMod)
			if got := newWorkerCapacity(t, wp, rsFixtureKey("10.0.0.1", 1, 101)); got != 8192 {
				t.Fatalf("refused verify changed running worker-queue-size to %d, want 8192", got)
			}
		})
	}

	t.Run("refuse_native_json_bool_rs_client", func(t *testing.T) {
		sections := []sdk.ConfigSection{
			{Root: configRootBGP, Data: `{"bgp":{"peer":{"native-bool":{"session":{"rs-client":true},"filter":{"import":{"filters":["modify:PREPEND"],"final":"accept"}}}}}}`},
			{Root: policy.RootName, Data: `{"policy":{"modify":{"PREPEND":{"set":{"as-path-prepend":2}}}}}`},
		}
		assertRSPrependRefusal(t, verifyRSConfig(sections), "peer native-bool", prependModName)
	})

	permittedCases := []struct {
		name string
		cfg  string
	}{
		{
			name: "non_rs_client_import_prepend",
			cfg: policyBlock + `
bgp {
	router-id 1.2.3.4;
	session { asn { local 65000; } }
	peer transit-a {
		connection { remote { ip 192.0.2.20; } }
		session { asn { remote 65020; } }
		filter { import { filters [ modify:PREPEND ]; final accept; } }
	}
}`,
		},
		{
			name: "grouped_peer_overrides_rs_client_false",
			cfg: policyBlock + `
bgp {
	router-id 1.2.3.4;
	session { asn { local 65000; } }
	group mixed {
		session { rs-client true; }
		peer transit-override {
			connection { remote { ip 192.0.2.21; } }
			session { asn { remote 65021; } rs-client false; }
			filter { import { filters [ modify:PREPEND ]; final accept; } }
		}
	}
}`,
		},
		{
			name: "rs_client_export_only_prepend",
			cfg: policyBlock + `
bgp {
	router-id 1.2.3.4;
	session { asn { local 65000; } }
	peer client-export {
		connection { remote { ip 192.0.2.22; } }
		session { asn { remote 65022; } rs-client true; }
		filter { export { filters [ modify:PREPEND ]; final accept; } }
	}
}`,
		},
		{
			name: "rs_client_inactive_import_ref",
			cfg: policyBlock + `
bgp {
	router-id 1.2.3.4;
	session { asn { local 65000; } }
	peer client-inactive {
		connection { remote { ip 192.0.2.23; } }
		session { asn { remote 65023; } rs-client true; }
		filter { import { filters [ inactive:PREPEND ]; final accept; } }
	}
}`,
		},
		{
			name: "rs_client_foreign_prefixed_ref",
			cfg: policyBlock + `
bgp {
	router-id 1.2.3.4;
	session { asn { local 65000; } }
	peer client-foreign {
		connection { remote { ip 192.0.2.24; } }
		session { asn { remote 65024; } rs-client true; }
		filter { import { filters [ prefix-list:PREPEND ]; final accept; } }
	}
}`,
		},
		{
			name: "rs_client_modify_without_as_path_prepend",
			cfg: policyBlock + `
bgp {
	router-id 1.2.3.4;
	session { asn { local 65000; } }
	peer client-lp {
		connection { remote { ip 192.0.2.25; } }
		session { asn { remote 65025; } rs-client true; }
		filter { import { filters [ modify:LP-ONLY ]; final accept; } }
	}
}`,
		},
		{
			name: "absent_policy_section_with_worker_queue_size",
			cfg: `
bgp {
	router-id 1.2.3.4;
	session { asn { local 65000; } }
	route-server { worker-queue-size 8192; }
	peer client-plain {
		connection { remote { ip 192.0.2.26; } }
		session { asn { remote 65026; } rs-client true; }
	}
}`,
		},
	}

	for _, tc := range permittedCases {
		t.Run("permit_"+tc.name, func(t *testing.T) {
			if err := config.VerifyPluginConfigContent(tc.cfg); err != nil {
				t.Fatalf("VerifyPluginConfigContent refused permitted config: %v", err)
			}
			sections := rsSectionsFromConfig(t, tc.cfg)
			_, stage2Tx := newQueueSizeTestTx(t, false)
			if err := stage2Tx.Configure(sections); err != nil {
				t.Fatalf("ConfigTx.Configure refused permitted config: %v", err)
			}
			_, reloadTx := newQueueSizeTestTx(t, false)
			if err := reloadTx.Configure(queueSizeSection("4096")); err != nil {
				t.Fatalf("seed reloadTx: %v", err)
			}
			if err := reloadTx.Verify(sections); err != nil {
				t.Fatalf("ConfigTx.Verify refused permitted config: %v", err)
			}
		})
	}

	t.Run("malformed_json_fails_closed", func(t *testing.T) {
		badCases := []struct {
			name     string
			sections []sdk.ConfigSection
		}{
			{
				name:     "malformed_bgp",
				sections: []sdk.ConfigSection{{Root: configRootBGP, Data: `{"bgp":`}},
			},
			{
				name: "malformed_policy",
				sections: []sdk.ConfigSection{
					{Root: configRootBGP, Data: `{"bgp":{"route-server":{"worker-queue-size":"8192"}}}`},
					{Root: policy.RootName, Data: `{"policy":`},
				},
			},
			{
				name:     "null_bgp",
				sections: []sdk.ConfigSection{{Root: configRootBGP, Data: `null`}},
			},
			{
				name: "non_object_policy",
				sections: []sdk.ConfigSection{
					{Root: configRootBGP, Data: `{"bgp":{}}`},
					{Root: policy.RootName, Data: `{"policy":"invalid"}`},
				},
			},
		}
		for _, tc := range badCases {
			t.Run(tc.name, func(t *testing.T) {
				err := verifyRSConfig(tc.sections)
				if err == nil {
					t.Fatal("malformed section was accepted by verifyRSConfig")
				}
				if _, ok := errors.AsType[*opererr.Error](err); !ok {
					t.Fatalf("malformed section error is not *opererr.Error: %T (%v)", err, err)
				}
				wp, tx := newQueueSizeTestTx(t, false)
				if seedErr := tx.Configure(queueSizeSection(seedQueueSize)); seedErr != nil {
					t.Fatalf("seed running config: %v", seedErr)
				}
				if verifyErr := tx.Verify(tc.sections); verifyErr == nil {
					t.Fatal("malformed section was accepted by ConfigTx.Verify")
				}
				if got := newWorkerCapacity(t, wp, rsFixtureKey("10.0.0.9", 9, 109)); got != 8192 {
					t.Fatalf("refused malformed verify changed worker-queue-size to %d, want 8192", got)
				}
			})
		}
	})
}
