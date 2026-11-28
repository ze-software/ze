// Design: docs/architecture/config/transaction-protocol.md -- no-prepend and import as-path-prepend refusal
// RFC: rfc/short/rfc7705.md -- Section 3.3 No Prepend Inbound conflicts with active import as-path-prepend

package filter_modify

import (
	"errors"
	"slices"
	"strings"
	"testing"

	_ "github.com/ze-software/ze/internal/component/bgp/plugin"
	"github.com/ze-software/ze/internal/component/config"
	"github.com/ze-software/ze/internal/component/plugin/registry"
	"github.com/ze-software/ze/internal/component/policy"
	"github.com/ze-software/ze/internal/core/opererr"
	sdk "github.com/ze-software/ze/pkg/plugin/sdk"
)

// modifySectionsFromConfig parses configText through YANG and builds the bgp +
// policy sections delivered to bgp-filter-modify at Stage 2 and reload verify.
func modifySectionsFromConfig(t *testing.T, configText string) []sdk.ConfigSection {
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

// assertNoPrependRefusal verifies that err is a structured opererr refusal
// naming the offending target, modify definition, as-path-prepend,
// local-options no-prepend, contradiction reason, and removal fix on the last
// line.
func assertNoPrependRefusal(t *testing.T, err error, wantTarget, wantMod string) {
	t.Helper()
	if err == nil {
		t.Fatalf("expected refusal for %s / %s, got nil", wantTarget, wantMod)
	}
	opErr, ok := errors.AsType[*opererr.Error](err)
	if !ok {
		t.Fatalf("refusal is not *opererr.Error: %T (%v)", err, err)
	}
	msg := err.Error()
	if strings.Contains(msg, "bgp-filter-modify: filter-modify:") {
		t.Errorf("refusal repeats plugin prefix:\n%s", msg)
	}
	for _, want := range []string{wantTarget, wantMod, asPathPrependAttr, "local-options no-prepend"} {
		if !strings.Contains(msg, want) {
			t.Errorf("refusal missing %q:\n%s", want, msg)
		}
	}
	lines := strings.Split(strings.TrimSpace(msg), "\n")
	lastLine := lines[len(lines)-1]
	if !strings.HasPrefix(lastLine, "fix: ") {
		t.Errorf("last line must start with \"fix: \", got %q\nfull error:\n%s", lastLine, msg)
	}
	if !strings.Contains(opErr.Why(), "contradict") {
		t.Errorf("opererr.Why must explain contradiction, got %q", opErr.Why())
	}
	if !strings.Contains(opErr.Fix(), "remove") {
		t.Errorf("opererr.Fix must name removal fix, got %q", opErr.Fix())
	}
}

// TestNoPrependRefusesInboundASPathPrepend proves AC-4 across offline
// validation (config.VerifyPluginConfigContent), Stage 2 configure
// (ConfigTx.Configure), and live reload verify (ConfigTx.Verify).
//
// VALIDATES: bgp-filter-modify refuses any active import as-path-prepend
// modifier on any peer or dynamic-group template whose effective
// session > asn > local-options contains no-prepend, returns a structured
// opererr naming the peer/group, modify definition, as-path-prepend,
// local-options no-prepend, why they contradict, and the removal fix, leaves
// running defsByName unchanged on reload refusal, fails closed on malformed
// JSON, declares FatalOnConfigError, and permits replace-as alone, no
// local-options, peer override clearing group no-prepend, export-only prepend
// on a no-prepend peer, deactivated import refs, foreign-prefixed refs, and
// non-prepend modify definitions.
// PREVENTS: contradictory no-prepend and import as-path-prepend settings
// loading silently with undefined precedence.
func TestNoPrependRefusesInboundASPathPrepend(t *testing.T) {
	previousDefs := defsByName.Load()
	t.Cleanup(func() { defsByName.Store(previousDefs) })

	t.Run("registration_metadata", func(t *testing.T) {
		reg := registry.Lookup(modifyPluginPrefix)
		if reg == nil {
			t.Fatal("bgp-filter-modify is not registered")
		}
		if !reg.FatalOnConfigError {
			t.Error("bgp-filter-modify must declare FatalOnConfigError: true")
		}
		if !slices.Contains(reg.ConfigReads, policy.RootName) {
			t.Errorf("bgp-filter-modify ConfigReads = %v, want %q", reg.ConfigReads, policy.RootName)
		}
		if reg.InProcessConfigVerifier == nil {
			t.Error("bgp-filter-modify must register InProcessConfigVerifier")
		}
	})

	const (
		prependModName = "PREPEND"
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
	peer cust-a {
		connection { remote { ip 192.0.2.1; } }
		session { asn { local 65010; remote 65001; local-options [ no-prepend ]; } }
		filter { import { filters [ PREPEND ]; final accept; } }
	}
}`,
			wantTarget: "peer cust-a",
			wantMod:    prependModName,
		},
		{
			name: "standalone_peer_multi_option_modify_prefix",
			cfg: policyBlock + `
bgp {
	router-id 1.2.3.4;
	session { asn { local 65000; } }
	peer cust-b {
		connection { remote { ip 192.0.2.2; } }
		session { asn { local 65010; remote 65002; local-options [ no-prepend replace-as ]; } }
		filter { import { filters [ modify:PREPEND ]; final accept; } }
	}
}`,
			wantTarget: "peer cust-b",
			wantMod:    prependModName,
		},
		{
			name: "standalone_peer_plugin_prefix",
			cfg: policyBlock + `
bgp {
	router-id 1.2.3.4;
	session { asn { local 65000; } }
	peer cust-c {
		connection { remote { ip 192.0.2.3; } }
		session { asn { local 65010; remote 65003; local-options [ no-prepend ]; } }
		filter { import { filters [ bgp-filter-modify:PREPEND ]; final accept; } }
	}
}`,
			wantTarget: "peer cust-c",
			wantMod:    prependModName,
		},
		{
			name: "grouped_peer_inherited_no_prepend_group_import",
			cfg: policyBlock + `
bgp {
	router-id 1.2.3.4;
	session { asn { local 65000; } }
	group legacy-as {
		session { asn { local 65010; local-options [ no-prepend ]; } }
		filter { import { filters [ modify:PREPEND ]; final accept; } }
		peer member-a {
			connection { remote { ip 192.0.2.10; } }
			session { asn { remote 65011; } }
		}
	}
}`,
			wantTarget: "peer member-a",
			wantMod:    prependModName,
		},
		{
			name: "grouped_peer_inherited_no_prepend_global_import",
			cfg: policyBlock + `
bgp {
	router-id 1.2.3.4;
	session { asn { local 65000; } }
	filter { import { filters [ PREPEND ]; final accept; } }
	group legacy-as {
		session { asn { local 65010; local-options [ no-prepend ]; } }
		peer member-b {
			connection { remote { ip 192.0.2.11; } }
			session { asn { remote 65012; } }
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
	group migrated {
		connection { remote { ip dynamic; range [ 198.51.100.0/24 ]; } }
		session { router-id 1.2.3.4; asn { local 65010; local-options [ no-prepend ]; } }
		filter { import { filters [ modify:PREPEND ]; final accept; } }
	}
}`,
			wantTarget: "group migrated",
			wantMod:    prependModName,
		},
	}

	seedCfg := policyBlock + `
bgp {
	router-id 1.2.3.4;
	session { asn { local 65000; } }
	peer healthy {
		connection { remote { ip 192.0.2.90; } }
		session { asn { remote 65090; } }
		filter { import { filters [ modify:LP-ONLY ]; final accept; } }
	}
}`
	seedSections := modifySectionsFromConfig(t, seedCfg)

	for _, tc := range refusedCases {
		t.Run("refuse_"+tc.name, func(t *testing.T) {
			validateErr := config.VerifyPluginConfigContent(tc.cfg)
			assertNoPrependRefusal(t, validateErr, tc.wantTarget, tc.wantMod)

			sections := modifySectionsFromConfig(t, tc.cfg)

			stage2Tx := newModifyConfigTx()
			assertNoPrependRefusal(t, stage2Tx.Configure(sections), tc.wantTarget, tc.wantMod)

			reloadTx := newModifyConfigTx()
			if err := reloadTx.Configure(seedSections); err != nil {
				t.Fatalf("seed running config: %v", err)
			}
			installedBefore := defsByName.Load()
			assertNoPrependRefusal(t, reloadTx.Verify(sections), tc.wantTarget, tc.wantMod)
			if defsByName.Load() != installedBefore {
				t.Fatal("refused verify mutated running defsByName")
			}
		})
	}

	permittedCases := []struct {
		name string
		cfg  string
	}{
		{
			name: "replace_as_alone_with_import_prepend",
			cfg: policyBlock + `
bgp {
	router-id 1.2.3.4;
	session { asn { local 65000; } }
	peer cust-replace {
		connection { remote { ip 192.0.2.20; } }
		session { asn { local 65010; remote 65020; local-options [ replace-as ]; } }
		filter { import { filters [ modify:PREPEND ]; final accept; } }
	}
}`,
		},
		{
			name: "no_local_options_with_import_prepend",
			cfg: policyBlock + `
bgp {
	router-id 1.2.3.4;
	session { asn { local 65000; } }
	peer plain-peer {
		connection { remote { ip 192.0.2.21; } }
		session { asn { remote 65021; } }
		filter { import { filters [ modify:PREPEND ]; final accept; } }
	}
}`,
		},
		{
			name: "grouped_peer_overrides_group_no_prepend_with_replace_as",
			cfg: policyBlock + `
bgp {
	router-id 1.2.3.4;
	session { asn { local 65000; } }
	group legacy-as {
		session { asn { local 65010; local-options [ no-prepend ]; } }
		peer override-peer {
			connection { remote { ip 192.0.2.22; } }
			session { asn { remote 65022; local-options [ replace-as ]; } }
			filter { import { filters [ modify:PREPEND ]; final accept; } }
		}
	}
}`,
		},
		{
			name: "no_prepend_peer_with_export_only_prepend",
			cfg: policyBlock + `
bgp {
	router-id 1.2.3.4;
	session { asn { local 65000; } }
	peer cust-export {
		connection { remote { ip 192.0.2.23; } }
		session { asn { local 65010; remote 65023; local-options [ no-prepend ]; } }
		filter { export { filters [ modify:PREPEND ]; final accept; } }
	}
}`,
		},
		{
			name: "no_prepend_peer_with_inactive_import_ref",
			cfg: policyBlock + `
bgp {
	router-id 1.2.3.4;
	session { asn { local 65000; } }
	peer cust-inactive {
		connection { remote { ip 192.0.2.24; } }
		session { asn { local 65010; remote 65024; local-options [ no-prepend ]; } }
		filter { import { filters [ inactive:PREPEND ]; final accept; } }
	}
}`,
		},
		{
			name: "no_prepend_peer_with_foreign_prefixed_ref",
			cfg: policyBlock + `
bgp {
	router-id 1.2.3.4;
	session { asn { local 65000; } }
	peer cust-foreign {
		connection { remote { ip 192.0.2.25; } }
		session { asn { local 65010; remote 65025; local-options [ no-prepend ]; } }
		filter { import { filters [ prefix-list:PREPEND ]; final accept; } }
	}
}`,
		},
		{
			name: "no_prepend_peer_with_modify_without_as_path_prepend",
			cfg: policyBlock + `
bgp {
	router-id 1.2.3.4;
	session { asn { local 65000; } }
	peer cust-lp {
		connection { remote { ip 192.0.2.26; } }
		session { asn { local 65010; remote 65026; local-options [ no-prepend ]; } }
		filter { import { filters [ modify:LP-ONLY ]; final accept; } }
	}
}`,
		},
	}

	for _, tc := range permittedCases {
		t.Run("permit_"+tc.name, func(t *testing.T) {
			if err := config.VerifyPluginConfigContent(tc.cfg); err != nil {
				t.Fatalf("VerifyPluginConfigContent refused permitted config: %v", err)
			}
			sections := modifySectionsFromConfig(t, tc.cfg)
			stage2Tx := newModifyConfigTx()
			if err := stage2Tx.Configure(sections); err != nil {
				t.Fatalf("ConfigTx.Configure refused permitted config: %v", err)
			}
			reloadTx := newModifyConfigTx()
			if err := reloadTx.Configure(seedSections); err != nil {
				t.Fatalf("seed reloadTx: %v", err)
			}
			if err := reloadTx.Verify(sections); err != nil {
				t.Fatalf("ConfigTx.Verify refused permitted config: %v", err)
			}
		})
	}

	t.Run("permit_absent_policy_section", func(t *testing.T) {
		cfg := `
bgp {
	router-id 1.2.3.4;
	session { asn { local 65000; } }
	peer no-policy-peer {
		connection { remote { ip 192.0.2.88; } }
		session { asn { local 65010; remote 65001; local-options [ no-prepend ]; } }
	}
}
`
		if err := config.VerifyPluginConfigContent(cfg); err != nil {
			t.Fatalf("VerifyPluginConfigContent refused config without policy section: %v", err)
		}
		sections := []sdk.ConfigSection{
			{Root: configRootBGP, Data: `{"bgp":{"peer":{"no-policy-peer":{"session":{"asn":{"local-options":["no-prepend"]}}}}}}`},
		}
		if err := verifyModifyConfig(sections); err != nil {
			t.Fatalf("verifyModifyConfig refused absent policy section: %v", err)
		}
	})

	t.Run("malformed_json_fails_closed", func(t *testing.T) {
		badCases := []struct {
			name     string
			wantRoot string
			sections []sdk.ConfigSection
		}{
			{
				name:     "malformed_bgp",
				wantRoot: configRootBGP,
				sections: []sdk.ConfigSection{
					{Root: configRootBGP, Data: `{"bgp":`},
					{Root: policy.RootName, Data: `{"policy":{"modify":{"PREPEND":{"set":{"as-path-prepend":"1"}}}}}`},
				},
			},
			{
				name:     "null_bgp",
				wantRoot: configRootBGP,
				sections: []sdk.ConfigSection{
					{Root: configRootBGP, Data: `null`},
					{Root: policy.RootName, Data: `{"policy":{"modify":{"PREPEND":{"set":{"as-path-prepend":"1"}}}}}`},
				},
			},
			{
				name:     "non_object_bgp",
				wantRoot: configRootBGP,
				sections: []sdk.ConfigSection{
					{Root: configRootBGP, Data: `{"bgp":"invalid"}`},
					{Root: policy.RootName, Data: `{"policy":{"modify":{"PREPEND":{"set":{"as-path-prepend":"1"}}}}}`},
				},
			},
			{
				name:     "malformed_policy",
				wantRoot: policy.RootName,
				sections: []sdk.ConfigSection{
					{Root: configRootBGP, Data: `{"bgp":{"peer":{}}}`},
					{Root: policy.RootName, Data: `{"policy":`},
				},
			},
			{
				name:     "null_policy",
				wantRoot: policy.RootName,
				sections: []sdk.ConfigSection{
					{Root: configRootBGP, Data: `{"bgp":{"peer":{}}}`},
					{Root: policy.RootName, Data: `null`},
				},
			},
			{
				name:     "non_object_policy",
				wantRoot: policy.RootName,
				sections: []sdk.ConfigSection{
					{Root: configRootBGP, Data: `{"bgp":{"peer":{}}}`},
					{Root: policy.RootName, Data: `{"policy":"invalid"}`},
				},
			},
		}
		for _, tc := range badCases {
			t.Run(tc.name, func(t *testing.T) {
				err := verifyModifyConfig(tc.sections)
				if err == nil {
					t.Fatal("malformed section was accepted by verifyModifyConfig")
				}
				if _, ok := errors.AsType[*opererr.Error](err); !ok {
					t.Fatalf("malformed section error is not *opererr.Error: %T (%v)", err, err)
				}
				if !strings.Contains(err.Error(), tc.wantRoot) {
					t.Errorf("malformed section error %q does not name root %q", err.Error(), tc.wantRoot)
				}
				tx := newModifyConfigTx()
				if seedErr := tx.Configure(seedSections); seedErr != nil {
					t.Fatalf("seed running config: %v", seedErr)
				}
				installedBefore := defsByName.Load()
				if verifyErr := tx.Verify(tc.sections); verifyErr == nil {
					t.Fatal("malformed section was accepted by ConfigTx.Verify")
				}
				if defsByName.Load() != installedBefore {
					t.Fatal("refused malformed verify mutated running defsByName")
				}
			})
		}
	})
}
