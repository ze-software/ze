// Design: docs/architecture/core-design.md -- the record the completeness gate reads
// Overview: completeness_test.go -- the gate that derives both populations and
// compares them against this record
//
// One row per historical Make target. The target population is DERIVED from the
// Make text in git history and the action population is DERIVED from the live
// command registry, so this file carries neither list. It carries only the
// judgement that joins them, which no derivation can make: which native action
// answers a target, and which target was retired rather than ported.
//
// A target that appears in neither table is reported by the gate as a producer
// with no native action. That is the intended default: a row is written when
// somebody has checked, so silence means nobody has.

package le

// portedProducer names one historical Make target and the native action that
// answers it now.
//
// Verb is empty when the area is the whole command, which is how a single-gate
// tool is typed (`le cli grammar`) and how an area that runs its own aggregate
// with no action word is typed (`le test functional`).
//
// Note carries what the pairing is not obvious from the two names alone: a
// keyword the action needs, or the operator that replaced a `-json` twin.
type portedProducer struct {
	Target string
	Area   string
	Verb   string
	Note   string
}

// retiredProducer names one historical Make target that was deliberately not
// ported, and why. Three reasons appear, and each is a statement about the
// target rather than about the effort available:
//
//   - the subject is gone: the script, interpreter, or Make machinery the
//     target drove is deleted;
//   - the job is absorbed: another native action performs it, with no separate
//     identity, and Reason names the producing function;
//   - the target was a wrapper: its recipe called a product command and added
//     nothing, so the product command is the producer.
type retiredProducer struct {
	Target string
	Reason string
}

// portedProducers is the join between a historical target and a live action.
// The gate verifies every Area against the registry, so a renamed area or a
// deleted verb fails here rather than going quiet.
var portedProducers = []portedProducer{
	// Repository, tier and tree gates. The Make recipes forwarded to the
	// Python le under a compatibility verb that preserved the target name
	// (`le repo ze-tier-selftest`); the native areas do not.
	{Target: "ze-tier-check", Area: "arch tier", Verb: "check"},
	{Target: "ze-iface-resolution-check", Area: "arch iface-resolution"},
	{Target: "ze-plugin-boundary-check", Area: "plugin boundary", Verb: "check"},
	{Target: "ze-config-coercion-check", Area: "config coercion", Verb: "check"},
	{Target: "ze-ci-dispatch-check", Area: "cli dispatch", Verb: "check"},
	{Target: "ze-fs-persistence-check", Area: "arch fs-persistence", Verb: "check"},
	{Target: "ze-dash-stdio-check", Area: "cli stdio", Verb: "check"},
	{Target: "ze-port-defaults-check", Area: "config ports", Verb: "check"},
	{Target: "ze-yang-leaf-mentions-report", Area: "config unread-leaves", Verb: "report"},
	{Target: "ze-test-sensitivity-check", Area: "test sensitivity", Verb: "check"},
	{Target: "ze-test-weakened-check", Area: "test weakened", Verb: "check"},
	{Target: "ze-staticcheck-feature-matrix-check", Area: "go staticcheck", Verb: "check"},
	{Target: "ze-repository-tracked-build-check", Area: "repo tracked-build", Verb: "check"},
	{Target: "ze-repository-check", Area: "repo", Verb: "check"},
	{Target: "ze-repository-tree-check", Area: "repo", Verb: "tree-check"},
	{Target: "ze-le-tracked-import-check", Area: "repo tracked-le"},
	{Target: "ze-platform-vet", Area: "go vet-platforms", Verb: "darwin", Note: "the freebsd verb carries the second half the target ran"},
	{Target: "ze-test-health-check", Area: "test health", Verb: "check"},
	{Target: "ze-test-health-update", Area: "test health", Verb: "update"},
	{Target: "ze-test-health-record", Area: "test health", Verb: "record"},
	{Target: "ze-site-facts-check", Area: "site facts", Verb: "check"},
	{Target: "ze-site-facts-update", Area: "site facts", Verb: "update"},
	{Target: "ze-working-tree-check", Area: "repo working-tree"},
	{Target: "ze-journal-report", Area: "spec journal", Verb: "report"},
	{Target: "ze-verify-scope-selector", Area: "repo changed", Verb: "scope"},

	// Generated artifacts.
	{Target: "generate", Area: "repo", Verb: "generate", Note: "the six codegen steps are stages of the one generate action"},
	{Target: "ze-generated-files-update", Area: "repo", Verb: "generate"},
	{Target: "ze-generated-files-check", Area: "repo", Verb: "generated-check"},
	{Target: "ze-plugin-imports-check", Area: "plugin imports", Verb: "check"},
	{Target: "ze-yang-glue-check", Area: "yang glue", Verb: "check"},
	{Target: "ze-feature-tags-check", Area: "repo feature-tags", Verb: "check"},
	{Target: "ze-web-assets-check", Area: "web assets", Verb: "check"},
	{Target: "ze-vendor-web-check", Area: "web vendor", Verb: "check"},
	{Target: "ze-vendor-web-sync", Area: "web vendor", Verb: "sync"},
	{Target: "ze-vendor-web-update-report", Area: "web vendor", Verb: "update-report"},
	{Target: "ze-htmx-upgrade-check", Area: "web htmx", Verb: "check"},
	{Target: "ze-htmx-upgrade-report", Area: "web htmx", Verb: "report"},
	{Target: "ze-arch-map-check", Area: "repo arch-map", Verb: "check"},
	{Target: "ze-arch-map-update", Area: "repo arch-map", Verb: "update"},
	{Target: "ze-discovery-index-update", Area: "repo package-map", Verb: "update"},
	{Target: "ze-ai-skills-sync", Area: "ai sync", Verb: "write"},
	{Target: "ze-ai-sync-check", Area: "ai sync", Verb: "check"},
	{Target: "ze-ai-instructions-generate", Area: "ai sync", Verb: "write", Note: "ai sync write writes CLAUDE.md and AGENTS.md from ai/INSTRUCTIONS.md"},
	{Target: "ze-proto-generate", Area: "setup", Verb: "proto-generate"},

	// CLI and documentation gates.
	{Target: "ze-cli-grammar-check", Area: "cli grammar"},
	{Target: "ze-cli-grammar-check-json", Area: "cli grammar", Note: "JSON is the `| json` operator on the same action"},
	{Target: "ze-command-ownership-check", Area: "cli ownership"},
	{Target: "ze-command-ownership-check-json", Area: "cli ownership", Note: "JSON is the `| json` operator on the same action"},
	{Target: "ze-config-claims-check", Area: "config claims"},
	{Target: "ze-config-claims-check-json", Area: "config claims", Note: "JSON is the `| json` operator on the same action"},
	{Target: "ze-command-contract-check", Area: "doc yang-contract", Verb: "command-contract"},
	{Target: "ze-command-contract-check-json", Area: "doc yang-contract", Verb: "command-contract", Note: "JSON is the `| json` operator on the same action"},
	{Target: "ze-doc-drift-check", Area: "doc yang-contract", Verb: "doc-drift"},
	{Target: "ze-docs-pipe-operators-update", Area: "doc yang-contract", Verb: "pipe-operators-update"},
	{Target: "ze-doc-links-check", Area: "doc check", Verb: "links"},
	{Target: "ze-doc-verify", Area: "doc check", Verb: "verify"},
	{Target: "ze-templ-output-check", Area: "doc check", Verb: "templ-output"},
	{Target: "ze-templ-orphan-check", Area: "doc wiring", Verb: "templ-orphans"},
	{Target: "ze-doc-wiring-check", Area: "doc wiring", Verb: "check", Note: "the router is one action: `changed-file <path>` and `dry-run` are its keywords, and the whole-tree form the gate ran is the bare line"},
	{Target: "ze-doc-index-check", Area: "doc index", Verb: "check"},
	{Target: "ze-doc-index-update", Area: "doc index", Verb: "write"},
	{Target: "ze-digest-check", Area: "ai digest"},
	{Target: "ze-consistency-check", Area: "doc consistency"},
	{Target: "ze-spec-citation-check", Area: "spec citation"},
	{Target: "ze-spec-status", Area: "spec status"},
	{Target: "ze-spec-status-json", Area: "spec status", Note: "JSON is the `| json` operator on the same action"},
	{Target: "ze-ste-check", Area: "doc ste", Verb: "check"},
	{Target: "ze-ste-review", Area: "doc ste", Verb: "review"},
	{Target: "ze-ste-review-changed", Area: "doc ste", Verb: "review-changed"},
	{Target: "ze-ste-review-json", Area: "doc ste", Verb: "review", Note: "JSON is the `| json` operator on the same action"},
	{Target: "ze-wiki-commands-update", Area: "cli catalog", Verb: "update"},
	{Target: "ze-wiki-update", Area: "cli catalog", Verb: "update", Note: "the target sequenced ze-wiki-commands-update alone"},
	{Target: "ze-site-generate", Area: "site", Verb: "build"},

	// The rule corpus.
	{Target: "ze-rules-lint", Area: "ai rules", Verb: "lint"},
	{Target: "ze-rules-render-check", Area: "ai rules", Verb: "render-check"},
	{Target: "ze-rules-render-update", Area: "ai rules", Verb: "render-update"},
	{Target: "ze-rules-index-check", Area: "ai rules", Verb: "index-check"},
	{Target: "ze-rules-index-update", Area: "ai rules", Verb: "index-update"},
	{Target: "ze-rules-condensed-check", Area: "ai rules", Verb: "condensed-check"},
	{Target: "ze-rules-condensed-update", Area: "ai rules", Verb: "condensed-update"},
	{Target: "ze-rules-points-roundtrip-check", Area: "ai rules", Verb: "points-roundtrip-check"},
	{Target: "ze-rules-gate-map-report", Area: "ai rules", Verb: "gate-map-report"},
	{Target: "ze-rules-payload-report", Area: "ai rules", Verb: "payload-report"},
	{Target: "ze-rules-router-report", Area: "ai rules", Verb: "router-report"},
	{Target: "ze-rules-router-report-json", Area: "ai rules", Verb: "router-report", Note: "JSON is the `| json` operator on the same action"},

	// Inventories and reports.
	{Target: "ze-inventory", Area: "repo inventory"},
	{Target: "ze-inventory-json", Area: "repo inventory", Note: "JSON is the `| json` operator on the same action"},
	{Target: "ze-command-list", Area: "cli list"},
	{Target: "ze-command-list-json", Area: "cli list", Note: "JSON is the `| json` operator on the same action"},
	{Target: "ze-token-economy-report", Area: "ai tokens", Note: "the bare area is the report; its keywords select the store and the caps"},

	// RFC conformance.
	{Target: "ze-rfc-check", Area: "rfc", Verb: "check"},
	{Target: "ze-rfc-index-update", Area: "rfc", Verb: "index-update"},
	{Target: "ze-rfc-reseal", Area: "rfc", Verb: "reseal"},
	{Target: "ze-rfc-extraction-create", Area: "rfc", Verb: "extraction-create"},
	{Target: "ze-rfc-extraction-status", Area: "rfc", Verb: "extraction-status"},

	// Verification, linting and the dependency stages.
	{Target: "ze-precommit-verify", Area: "verify", Verb: "current", Note: "mode full"},
	{Target: "ze-precommit-verify-changed", Area: "verify", Verb: "current", Note: "mode changed"},
	{Target: "ze-precommit-verify-list", Area: "verify", Verb: "list"},
	{Target: "ze-verify-worktree", Area: "verify", Verb: "worktree"},
	{Target: "ze-verify-debt-clear", Area: "commit", Verb: "debt-clear"},
	{Target: "ze-lint", Area: "go lint", Verb: "run"},
	{Target: "ze-lint-changed", Area: "go lint", Verb: "run", Note: "scope takes the package list `le repo changed packages` derives"},
	{Target: "ze-evidence-vet", Area: "verify deps", Verb: "evidence-vet"},
	{Target: "ze-dependency-vulnerability-check", Area: "verify deps", Verb: "vulnerability"},
	{Target: "ze-unit-test-cached", Area: "verify deps", Verb: "unit-cached"},
	{Target: "ze-unit-test-race-changed", Area: "verify deps", Verb: "unit-race-changed"},
	{Target: "ze-alloc-check", Area: "verify deps", Verb: "alloc"},

	// Unit and hook suites.
	{Target: "ze-unit-bgp-test", Area: "test unit", Verb: "bgp"},
	{Target: "ze-unit-core-test", Area: "test unit", Verb: "core"},
	{Target: "ze-unit-plugins-test", Area: "test unit", Verb: "plugins"},
	{Target: "ze-unit-config-test", Area: "test unit", Verb: "config"},
	{Target: "ze-unit-cli-test", Area: "test unit", Verb: "cli"},
	{Target: "ze-unit-test", Area: "test unit", Verb: "all", Note: "the verb runs the whole checkout under the race detector, which is the package population this target passed as ZE_PACKAGES, and then the installer group its own build tags hide from that run"},
	{Target: "ze-unit-hook-test", Area: "ai hooks", Verb: "unit"},
	{Target: "ze-unit-test-changed", Area: "verify deps", Verb: "unit-race-changed", Note: "the stage races the changed groups and the rest complement; `le repo changed packages` is the derivation the target shelled out for"},
	{Target: "ze-unit-installer-test", Area: "test unit", Verb: "installer", Note: "the group carries the target's own GOOS=linux and ze_core ze_installer tags, runs the tests on Linux and type-checks them elsewhere, and is in the bare sweep as the target was a prerequisite of ze-unit-test; `le test qemu all-tests` runs them for real off Linux"},
	{Target: "ze-unit-linux-test", Area: "test qemu", Verb: "run", Note: "the packages and command keywords carry the Linux-only Go package the golang container ran"},

	// Functional suites. The verb is the suite name; the area's own table is
	// derived from internal/le/test/functional/suites.go.
	{Target: "ze-functional-test", Area: "test functional", Verb: "gating", Note: "the bare area lists the suites, so the gating run is named"},
	{Target: "ze-functional-encode-test", Area: "test functional", Verb: "encode"},
	{Target: "ze-functional-plugin-test", Area: "test functional", Verb: "plugin"},
	{Target: "ze-functional-parse-test", Area: "test functional", Verb: "parse"},
	{Target: "ze-functional-decode-test", Area: "test functional", Verb: "decode"},
	{Target: "ze-functional-reload-test", Area: "test functional", Verb: "reload"},
	{Target: "ze-functional-ui-test", Area: "test functional", Verb: "ui"},
	{Target: "ze-functional-editor-test", Area: "test functional", Verb: "editor"},
	{Target: "ze-functional-managed-test", Area: "test functional", Verb: "managed"},
	{Target: "ze-functional-l2tp-test", Area: "test functional", Verb: "l2tp"},
	{Target: "ze-functional-firewall-test", Area: "test functional", Verb: "firewall"},
	{Target: "ze-functional-policy-test", Area: "test functional", Verb: "policy"},
	{Target: "ze-functional-ipsec-test", Area: "test functional", Verb: "ipsec"},
	{Target: "ze-functional-ldp-test", Area: "test functional", Verb: "ldp"},
	{Target: "ze-functional-rsvpte-test", Area: "test functional", Verb: "rsvpte"},
	{Target: "ze-functional-isis-test", Area: "test functional", Verb: "isis"},
	{Target: "ze-functional-ospf-test", Area: "test functional", Verb: "ospf"},
	{Target: "ze-functional-ospfv3-test", Area: "test functional", Verb: "ospfv3"},
	{Target: "ze-functional-web-test", Area: "test functional", Verb: "web"},
	{Target: "ze-functional-install-test", Area: "test functional", Verb: "install"},
	{Target: "ze-functional-appliance-test", Area: "test functional", Verb: "appliance"},
	{Target: "ze-functional-l2tp-wire-test", Area: "test functional", Verb: "l2tp-wire"},
	{Target: "ze-functional-isis-wire-test", Area: "test functional", Verb: "isis-wire"},
	{Target: "ze-functional-ospf-wire-test", Area: "test functional", Verb: "ospf-wire"},
	{Target: "ze-functional-runner-test", Area: "test functional", Verb: "runner"},
	{Target: "ze-functional-static-test", Area: "test functional", Verb: "static"},
	{Target: "ze-functional-traffic-test", Area: "test functional", Verb: "traffic"},
	{Target: "ze-functional-flow-export-test", Area: "test functional", Verb: "flow-export"},
	{Target: "ze-functional-vpp-test", Area: "test functional", Verb: "vpp"},
	{Target: "ze-functional-vrrp-test", Area: "test functional", Verb: "vrrp"},
	{Target: "ze-functional-exabgp-test", Area: "test functional", Verb: "exabgp-test"},

	// Integration, interop, stress and live proofs.
	{Target: "ze-interop-test", Area: "test integration", Verb: "interop"},
	{Target: "ze-interop-ipsec-test", Area: "test integration", Verb: "interop-ipsec"},
	{Target: "ze-stress-test", Area: "test integration", Verb: "stress"},
	{Target: "ze-stress-bird-test", Area: "test integration", Verb: "stress-bird"},
	{Target: "ze-stress-web-test", Area: "test integration", Verb: "stress-web"},
	{Target: "ze-stress-fleet-test", Area: "test integration", Verb: "stress-fleet"},
	{Target: "ze-stress-profile", Area: "test integration", Verb: "stress", Note: "the stress.scenario setting selects 05-profile-1m, which internal/le/test/integration/stress.go declares"},
	{Target: "ze-live-rpki-test", Area: "test integration", Verb: "live-rpki"},
	{Target: "ze-live-test", Area: "test integration", Verb: "live-rpki", Note: "the target sequenced ze-live-rpki-test alone"},
	{Target: "ze-integration-iface-test", Area: "test integration", Verb: "iface"},
	{Target: "ze-integration-fib-test", Area: "test integration", Verb: "fib"},
	{Target: "ze-integration-firewall-test", Area: "test integration", Verb: "firewall"},
	{Target: "ze-integration-traffic-test", Area: "test integration", Verb: "traffic"},
	{Target: "ze-integration-gtsm-test", Area: "test integration", Verb: "gtsm"},
	{Target: "ze-integration-as112-test", Area: "test integration", Verb: "as112"},
	{Target: "ze-evidence-release-candidate-check", Area: "verify evidence", Verb: "release-candidate"},

	// Deployment against a peer daemon.
	{Target: "ze-deployment-l2tp-test", Area: "test deployment", Verb: "l2tp-test"},
	{Target: "ze-deployment-l2tp-ppp-test", Area: "test deployment", Verb: "l2tp-ppp-test"},
	{Target: "ze-deployment-docker-l2tp-ppp-test", Area: "test deployment", Verb: "docker-l2tp-ppp-test"},
	{Target: "ze-deployment-docker-pppoe-accel-test", Area: "test deployment", Verb: "docker-pppoe-accel-test"},
	{Target: "ze-deployment-gokrazy-l2tp-ppp-test", Area: "test deployment", Verb: "gokrazy-l2tp-ppp-test"},
	{Target: "ze-qemu-l2tp-ppp-test", Area: "test deployment", Verb: "gokrazy-l2tp-ppp-test", Note: "the same L2TP PPP/NCP proof, driven against the gokrazy appliance image rather than an Alpine guest"},
	{Target: "ze-deployment-vpp-test", Area: "test deployment", Verb: "vpp-test"},
	{Target: "ze-deployment-vpp-iface-test", Area: "test deployment", Verb: "vpp-iface-test"},

	// The virtual machine proofs.
	{Target: "ze-qemu-test-all", Area: "test qemu", Verb: "all-tests"},
	{Target: "ze-qemu-needs-linux-test", Area: "test qemu", Verb: "all-tests", Note: "`only needs-linux` asks for the same population the target filtered to; the run exports ZE_QEMU_LINUX_ONLY to every child, where the filter lives (parseAndAdd, internal/test/runner/record_parse.go), and records the selection in its report"},
	{Target: "ze-qemu-install-test", Area: "test qemu", Verb: "install-test"},
	{Target: "ze-qemu-install-iso-test", Area: "test qemu", Verb: "install-iso-test"},
	{Target: "ze-qemu-install-scenarios-test", Area: "test qemu", Verb: "install-scenarios-test"},
	{Target: "ze-qemu-install-ventoy-test", Area: "test qemu", Verb: "install-ventoy-test"},
	{Target: "ze-qemu-netns-test", Area: "test qemu", Verb: "netns-test"},
	{Target: "ze-netns-test", Area: "test qemu", Verb: "netns-test", Note: "the action runs the lab on this host; firewall, policy, ospf and ospfv3 are the suites the target named"},
	{Target: "ze-qemu-pppoe-test", Area: "test qemu", Verb: "pppoe-test"},
	{Target: "ze-qemu-pppoe-accel-test", Area: "test qemu", Verb: "pppoe-accel-test"},
	{Target: "ze-qemu-vrrp-keepalived-test", Area: "test qemu", Verb: "vrrp-keepalived-test"},
	{Target: "ze-qemu-vpp-hugepages-test", Area: "test qemu", Verb: "vpp-hugepages-test"},
	{Target: "ze-qemu-shell", Area: "test qemu", Verb: "run", Note: "the keep-alive keyword is the interactive form the target ran"},
	{Target: "ze-qemu-debug", Area: "test qemu", Verb: "run", Note: "the command keyword carries the argv the target took as RUN="},
	{Target: "ze-qemu-integration-test", Area: "test qemu", Verb: "run", Note: "the packages and command keywords carry the integration-tagged package list the target named"},
	{Target: "ze-qemu-isis-frr-test", Area: "test qemu", Verb: "run", Note: "packages frr, command `go test -run TestISISInteropFRR ./internal/plugins/isis/...`"},
	{Target: "ze-qemu-ldp-frr-test", Area: "test qemu", Verb: "run", Note: "packages frr, command `go test -run TestLDPInteropFRR ./internal/plugins/ldp/...`"},
	{Target: "ze-qemu-traffic-usage-test", Area: "test qemu", Verb: "run", Note: "packages iproute2 and kmod, command the trafficusage TCX integration tests"},

	// Chaos, fuzzing and mutation.
	{Target: "ze-chaos-lint", Area: "chaos selftest", Verb: "lint"},
	{Target: "ze-chaos-unit-test", Area: "chaos selftest", Verb: "unit", Note: "the cli-unit verb carries the second half the target ran"},
	{Target: "ze-fuzz-test", Area: "test fuzz", Verb: "run"},
	{Target: "ze-fuzz-test-one", Area: "test fuzz", Verb: "run", Note: "the name, time and package keywords select one target"},
	{Target: "ze-mutation-report", Area: "test mutation", Verb: "combine"},

	// Scratch, sessions and the appliance artifacts.
	{Target: "ze-scratch-links-ensure", Area: "scratch", Verb: "links-ensure"},
	{Target: "ze-scratch-migrate", Area: "scratch", Verb: "migrate"},
	{Target: "ze-session-reap", Area: "session", Verb: "reap"},
	{Target: "ze-host-build", Area: "build host-driver"},
	{Target: "ze-setup-build", Area: "build host-driver", Note: "ze-host carries ze_core and ze_setup, and it is the binary that runs `ze appliance ...` on the build host"},
	{Target: "ze-installer-build", Area: "build installer", Verb: "amd64", Note: "arm64 carries the second half the target ran"},
	{Target: "ze-gokrazy-gosum-check", Area: "build gosum"},
	{Target: "ze-netlab-render-check", Area: "test netlab", Verb: "render-check"},
	{Target: "ze-perf-suggestion-report", Area: "perf", Verb: "suggest"},
	{Target: "ze-perf-bench", Area: "perf", Verb: "run", Note: "the action drives the multi-DUT runner that test/perf/run.py was ported into (internal/test/perfrunner.Runner.Execute), which cross-builds a linux le for the sender container, and writes the suggestion marker the target ended on"},
	{Target: "ze-perf-history-record", Area: "perf", Verb: "history-record", Note: "the recipe's `ze-perf track --append` never existed, so the action appends each result as one compacted line of test/perf/history/<dut>.ndjson, which is what the Python did"},
	{Target: "ze-evidence-perf-record", Area: "perf", Verb: "evidence-record", Note: "run, append, then `le perf track --check` over the history, which is the regression gate the recipe ended on"},

	// The published terminal demonstrations.
	{Target: "ze-terminal-demo-image-build", Area: "site terminal-demo", Verb: "image-build", Note: "the action reads the tag from demos/terminal/manifest.json rather than repeating it, so the image it builds is the one renderDemo runs"},
	{Target: "ze-terminal-demo-check-all", Area: "site terminal-demo", Verb: "check-all"},
	{Target: "ze-terminal-demo-validation-check-all", Area: "site terminal-demo", Verb: "validation-check-all"},
	{Target: "ze-terminal-demo-release-check-all", Area: "site terminal-demo", Verb: "release-check-all"},
	{Target: "ze-terminal-demo-render-all", Area: "site terminal-demo", Verb: "render-all"},
	{Target: "ze-terminal-demo-render", Area: "site terminal-demo", Verb: "render", Note: "`render name <id>` selects the one demo the target selected with DEMO=; RenderOne shares validateAndRender with RenderAll"},
	{Target: "ze-terminal-demo-release-render-all", Area: "site terminal-demo", Verb: "render-all", Note: "the target sequenced ze-terminal-demo-render-all alone"},
	{Target: "ze-terminal-demo-binaries-build", Area: "site terminal-demo", Verb: "binaries-build-ze", Note: "binaries-build-ze-test carries the second half the target ran"},
	{Target: "ze-release-assets-update", Area: "site terminal-demo", Verb: "render-all", Note: "the target sequenced ze-terminal-demo-release-render-all alone"},
	{Target: "ze-release-assets-check", Area: "site terminal-demo", Verb: "release-check-all", Note: "the target sequenced ze-terminal-demo-release-check-all alone"},

	// One Go test, named by its -run pattern and its own flag. The area is the
	// generic job grammar `le job run label <label> command <argv...>`, which
	// ai/rules/commands.md names for work no registered action owns. That area
	// publishes a usage line rather than a verb table, so the row names no verb.
	{Target: "ze-unit-pkg-test", Area: "job", Note: "command carries `go test` over the one package a developer is working on"},
	{Target: "ze-unit-reactor-test-race", Area: "job", Note: "command carries `go test -race -count=20 ./internal/component/bgp/reactor/...`"},
	{Target: "ze-web-golden-check", Area: "job", Note: "command carries the five web and lg golden `go test -run` invocations"},
	{Target: "ze-web-golden-update", Area: "job", Note: "command carries the same five invocations with -update-golden"},
	{Target: "ze-chaos-golden-update", Area: "job", Note: "command carries `go test -run TestChaosGoldenOutput ./internal/chaos/web/ -update-golden`"},
	{Target: "ze-plugin-snapshot-update", Area: "job", Note: "command carries the three plugin-registry snapshot tests with -update"},
	{Target: "ze-templ-port-check", Area: "job", Note: "command carries the web and lg templ port-fidelity tests with -port-ref"},

	// Tool probes. Each target listed the tools one class of heavy evidence
	// needs, and exited non-zero on a missing one.
	{Target: "ze-deployment-preflight", Area: "setup", Verb: "check", Note: "the tool population is internal/le/setup/tools.go, which probes docker, qemu and xl2tpd"},
	{Target: "ze-evidence-release-preflight", Area: "setup", Verb: "check", Note: "the same probe answers for the docker and qemu the release categories need"},
}

// retiredProducers is the deliberate not-ported list. Each row states what
// happened to the job, not that nobody had time for it.
var retiredProducers = []retiredProducer{
	{
		Target: "ze-discovery-index-check",
		Reason: "the subject is gone: the target compared ai/PACKAGE-MAP.md against a re-render of the tree, and the map is no longer tracked. A write to a file that feeds it removes it (hookruntime.postInvalidateDerived) and a command that names it rebuilds it (hookruntime.preMaterializeDerived), so there is no committed copy for a comparison to judge",
	},
	{
		Target: "help",
		Reason: "Make itself: the recipe echoed the Makefile's own target list. leroot.Usage prints the command list from the registry, so the list cannot go stale",
	},
	{
		Target: "help-test",
		Reason: "Make itself: a second hand-written page of the same target list, replaced by the same registry listing",
	},
	{
		Target: "help-deploy",
		Reason: "Make itself: a third hand-written page of the same target list, replaced by the same registry listing",
	},
	{
		Target: "help-dev",
		Reason: "Make itself: a fourth hand-written page of the same target list, replaced by the same registry listing",
	},
	{
		Target: "all",
		Reason: "Make itself: the default goal, sequencing ze-lint, ze-unit-test and build. Each member is judged on its own row, and the native verification population is `le verify current` (internal/le/verifyengine)",
	},
	{
		Target: "build",
		Reason: "Make itself: a prerequisite list over the nine binary file targets, which the launchers and the native build actions produce on demand",
	},
	{
		Target: "check",
		Reason: "Make itself: a prerequisite list over fmt and vet, both of which are bare Go toolchain commands",
	},
	{
		Target: "fmt",
		Reason: "the subject is one Go toolchain command: `gofmt -w` over the tracked Go files, with no repository logic between the developer and the tool",
	},
	{
		Target: "vet",
		Reason: "the subject is one Go toolchain command: `go vet ./...`. The repository's own vetting is `le go lint run` over every build flavor",
	},
	{
		Target: "tidy",
		Reason: "the subject is one Go toolchain command: `go mod tidy`, with no repository logic",
	},
	{
		Target: "ze-standard-test",
		Reason: "Make itself: a prerequisite list over ze-lint, ze-unit-test, ze-functional-test, ze-functional-exabgp-test and ze-fuzz-test, each judged on its own row",
	},
	{
		Target: "ze-verify-all",
		Reason: "Make itself: a prerequisite list over ze-precommit-verify and ze-chaos-verify, each judged on its own row",
	},
	{
		Target: "ze-smoke-verify",
		Reason: "Make itself: a prerequisite list over ze-lint, ze-unit-test and build, each judged on its own row",
	},
	{
		Target: "ze-ci-verify",
		Reason: "Make itself: a prerequisite list naming ze-smoke-verify alone",
	},
	{
		Target: "ze-test-all",
		Reason: "Make itself: a prerequisite list over the test targets, each judged on its own row",
	},
	{
		Target: "ze-chaos-test",
		Reason: "Make itself: a prerequisite list over the four chaos targets, each judged on its own row",
	},
	{
		Target: "ze-integration-test",
		Reason: "Make itself: a prerequisite list over the six integration targets, each of which is an action of the integration area",
	},
	{
		Target: "ze-functional-test-warm",
		Reason: "absorbed: warmCITestPackages (internal/le/test/functional/run.go) compiles the .ci-invoked packages inside the suite action, so the warm step has no separate identity",
	},
	{
		Target: "ze-session-binary-path",
		Reason: "the subject is gone: the recipe printed one Make variable holding bin/ze. A session's binaries now live under its own directory and are resolved by internal/le/test/functional/binaries.go, which is why ai/rules/commands.md forbids naming bin/ze at all",
	},
	{
		Target: "ze-iso-check",
		Reason: "a wrapper: the recipe was `ze appliance iso --check` and added nothing. The product command is the producer",
	},
	{
		Target: "ze-perf-report",
		Reason: "a wrapper: the recipe was `ze-perf report test/perf/results/*.json --md` and added nothing. The ze-perf program is the producer",
	},
	{
		Target: "ze-evidence-docker-run",
		Reason: "the subject is gone: the recipe ran scripts/evidence/docker-run.py against another Python evidence script, and the scripts/evidence/ tree is deleted",
	},
	{
		Target: "clean",
		Reason: "the subject is gone: the recipe called scripts/dev/session-scratch.sh and scripts/dev/ensure-links.py, both deleted. What it did beside them is `le session reap`, `le scratch links-ensure`, and `go clean -cache`",
	},
	{
		Target: "clean-all",
		Reason: "the subject is gone: the recipe emptied the in-tree tmp/ directory, which `le scratch links-ensure` and `le scratch migrate` now place outside the checkout, and then ran `go clean -cache`",
	},
	{
		Target: "ze-scratch-clean",
		Reason: "the subject is gone: the recipe deleted in-tree tmp/ entries by mtime. Scratch is placed outside the checkout now, and a session directory is removed by ownership rather than by age (`le session reap`)",
	},
	{
		Target: "ze-session-clean",
		Reason: "absorbed: removing session directories is `le session reap` (internal/le/session), which removes only the ones whose owning process is provably gone, so a date cutoff has no separate identity",
	},
	{
		Target: "le-build",
		Reason: "absorbed: the ./le launcher builds bin/le from ./cmd/ze under the ze_le tag and the feature-gate tags on first use, so a build target has nothing left to do",
	},
	{
		Target: "ze-build",
		Reason: "absorbed: testfunctional.Prepare builds ze into the invoking session's own directory (internal/le/test/functional/binaries.go, buildCommands), so nothing writes a shared bin/ze for a later target to find",
	},
	{
		Target: "ze-test-build",
		Reason: "absorbed: testfunctional.Prepare builds ze-test beside ze in the same isolated set (internal/le/test/functional/binaries.go), so the harness binary has no build of its own",
	},
	{
		Target: "ze-stripped-build",
		Reason: "absorbed: testfunctional.Prepare builds ze-stripped under ze_core and ze_ssh in the same isolated set (internal/le/test/functional/binaries.go)",
	},
	{
		Target: "ze-chaos-build",
		Reason: "absorbed: testfunctional.Prepare builds ze-chaos under ze_chaos and ze_bgp when an invocation needs it (internal/le/test/functional/binaries.go, buildCommands)",
	},
	{
		Target: "ze-analyze-build",
		Reason: "the subject is one Go toolchain command: `go build -tags ze_analyze ./cmd/ze`, with no repository logic between the developer and the tool",
	},
	{
		Target: "ze-appliance-build",
		Reason: "the subject is one Go toolchain command: `go build -tags 'ze_core ze_appliance' ./cmd/ze`. The appliance personality is driven through the ze-host binary `le build host-driver` writes",
	},
	{
		Target: "ze-perf-build",
		Reason: "the subject is one Go toolchain command: `go build -tags 'ze_perf ze_bgp' ./cmd/ze`, with no repository logic between the developer and the tool",
	},
	{
		Target: "ze-cadence-daily-run",
		Reason: "Make itself: the recipe re-entered make for ze-repository-check, ze-ste-check, ze-spec-status, ze-journal-report and the setup probe, each judged on its own row",
	},
	{
		Target: "ze-cadence-weekly-run",
		Reason: "Make itself: the recipe re-entered make for ze-consistency-check, ze-unit-reactor-test-race, ze-chaos-verify and four reports, each judged on its own row",
	},
	{
		Target: "ze-cadence-monthly-run",
		Reason: "Make itself: the recipe re-entered make for ze-deployment-preflight, ze-qemu-integration-test, ze-evidence-perf-record, ze-mutation-test-changed and ze-ste-review, each judged on its own row",
	},
	{
		Target: "ze-chaos-functional-test",
		Reason: "a wrapper: the recipe was `ze-chaos --in-process --duration --peers --routes --seed --quiet` and added nothing. The ze-chaos program is the producer",
	},
	{
		Target: "ze-chaos-integration-test",
		Reason: "a wrapper: the recipe was `ze-test bgp chaos --all -t 40s` and added nothing. The ze-test program is the producer",
	},
	{
		Target: "ze-chaos-web-test",
		Reason: "a wrapper: the recipe was `ze-test bgp chaos-web --all` and added nothing. The ze-test program is the producer",
	},
	{
		Target: "ze-chaos-verify",
		Reason: "Make itself: a prerequisite list over ze-chaos-lint and the four chaos test targets under the shared verify lock, each judged on its own row",
	},
	{
		Target: "ze-docker-build",
		Reason: "a wrapper: the recipe was `docker build -f docker/Dockerfile` with the version build arguments, and added nothing. The docker program is the producer",
	},
	{
		Target: "ze-docker-lab-build",
		Reason: "a wrapper: the recipe was `docker build -f docker/Dockerfile.lab` with the same build arguments, and added nothing. The docker program is the producer",
	},
	{
		Target: "ze-evidence-functional-test",
		Reason: "Make itself: a shell loop that re-entered make for the static, traffic, vpp and l2tp-wire functional targets, each judged on its own row",
	},
	{
		Target: "ze-evidence-release-verify",
		Reason: "Make itself: a shell loop that re-entered make for twelve release categories behind docker and qemu guards, each judged on its own row",
	},
	{
		Target: "ze-functional-docker-exec-check",
		Reason: "the subject is gone: the scan read the functional harness Python for fail-open docker_exec_quiet call sites against a floor in test/health/docker-exec-baseline.json, and neither file exists",
	},
	{
		Target: "ze-generated-files-reconcile",
		Reason: "Make itself: it ran the generated-files update and then re-checked the same paths. Both halves are `le repo generate` and `le repo generated-check`, each judged on its own row",
	},
	{
		Target: "ze-gokrazy-build",
		Reason: "absorbed: `ze appliance build` assembles the ZeFS database, runs gok, and formats and injects the /perm ext4 partition (internal/appliance/cmd_build.go), so the hand-written dd and debugfs sequence has no separate identity",
	},
	{
		Target: "ze-gokrazy-deps-download",
		Reason: "the subject is one Go toolchain command per packed builddir: `go mod download all` under gokrazy/ze/builddir, reached by a find loop and no repository logic",
	},
	{
		Target: "ze-gokrazy-run",
		Reason: "absorbed: booting a built appliance image in QEMU is `ze appliance run <name>` (internal/appliance/cmd_run.go), so the hand-written qemu-system invocation has no separate identity",
	},
	{
		Target: "ze-iso-build",
		Reason: "a wrapper: the recipe sequenced `ze appliance kernel`, `ze appliance initrd`, `ze appliance build` and `ze appliance iso`, and added nothing. The `ze appliance` command tree is the producer",
	},
	{
		Target: "ze-iso-build-full",
		Reason: "a wrapper: the same four `ze appliance` commands preceded by `ze appliance init --config`, adding only argument checks those commands already make",
	},
	{
		Target: "ze-iso-initialize",
		Reason: "a wrapper: the recipe was `ze appliance init --config <file> <name>` and added nothing. The `ze appliance` command tree is the producer",
	},
	{
		Target: "ze-pxe-build",
		Reason: "the subject is gone: the recipe cloned github.com/ipxe/ipxe and ran its Makefile to embed a boot script. Ze builds no iPXE binary now, and the provision plugin serves pre-built ones (internal/plugins/provision/staging.go)",
	},
	{
		Target: "ze-kernel-build",
		Reason: "absorbed: assembleKernelPackage (internal/le/test/deployment/gokrazyimage.go) copies the pinned modcache, the vmlinuz, the modules and the DTBs into the out-of-tree package, so the shell assembly has no separate identity",
	},
	{
		Target: "ze-kernel-vmlinuz-stage",
		Reason: "absorbed: resolveKernelPackage (internal/le/test/deployment/gokrazyimage.go) materializes the runtime kernel from the durable cache that `ze appliance kernel --target runtime --print-cache-dir` names",
	},
	{
		Target: "ze-kernel-clean",
		Reason: "the subject is gone: the recipe ran `make -C gokrazy/kernel clean` and undid a go.mod replace. That directory holds no Makefile, and a build names its kernel package per run through ze.gok.kernel-package",
	},
	{
		Target: "ze-mutation-test",
		Reason: "a wrapper: the recipe was `gomu run --output json --incremental=false` and added nothing. The gomu program is the producer, and the two steps after it are `le test mutation record-history` and `le test health record`",
	},
	{
		Target: "ze-mutation-test-changed",
		Reason: "a wrapper: the same `gomu run` with --incremental and --base-branch=main over the changed packages, and it added nothing. The gomu program is the producer",
	},
	{
		Target: "ze-mutation-pkg-test",
		Reason: "a wrapper: the recipe looped `gomu run` over each named package and added nothing. The gomu program is the producer, and merging its per-package reports is `le test mutation combine`",
	},
	{
		Target: "ze-netns-plugin-test",
		Reason: "absorbed: the three capability-needing plugin cases run in the privileged QEMU guest that allTestsRun.Execute drives (internal/le/test/qemu/alltests.go, the plugin row of vmSuites), so no host-side netns wrapper remains",
	},
	{
		Target: "ze-unit-rest-test",
		Reason: "absorbed: the complement of the five component groups is selection.Rest, which planUnitRaceChanged appends to the race package list (internal/le/verify/deps/verifydeps.go), so the rest group has no command of its own",
	},
	{
		Target: "ze-unit-test-coverage",
		Reason: "the subject is two Go toolchain commands: `go test -coverprofile=coverage.out` and `go tool cover -html`, with no repository logic between the developer and the tools",
	},
}
