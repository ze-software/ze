# Knowledge routing plan

Every `plan/learned/` summary that nothing in the tree cites: 485 of 903, read
in full on 2026-08-03 and judged against the three-part usefulness test in
`ai/rules/planning.md`. Each row's evidence was machine-checked as a verbatim
substring of its summary, so no verdict rests on a paraphrase.

**Verdict: 485 KEEP, 0 WASTE.** Nothing in the uncited corpus is waste. Two
regex classifiers had claimed 40 to 63 were; reading them found none. The
failure is routing, not quality: 485 summaries hold a constraint, a rejected
alternative, or a trap somebody already paid for, and nothing points at them.

Evidence below is quoted from the summary it judges. It is reproduced inside
quotation marks because it is a citation, not this document speaking.

## Which test each summary passed

| Test | Count |
|------|-------|
| A, helps future software development | 119 |
| B, explains past design | 160 |
| C, prevents repeating a past mistake | 206 |

## Where the content goes

| Destination kind | Summaries |
|------------------|-----------|
| a rule | 185 |
| an architecture doc | 138 |
| a digest | 128 |
| a curated aggregate | 24 |
| an RFC summary | 8 |
| no home yet | 2 |

98 distinct destinations. The busiest:

| Destination | Summaries |
|-------------|-----------|
| `ai/rules/testing.md` | 40 |
| `ai/rules/plugins.md` | 20 |
| `ai/digests/bgp-reactor.md` | 17 |
| `ai/rules/config.md` | 16 |
| `ai/rules/repo-maintenance.md` | 15 |
| `ai/digests/rib.md` | 14 |
| `ai/rules/planning.md` | 14 |
| `ai/digests/iface.md` | 14 |
| `plan/learned/HOOK-FRICTION.md` | 14 |
| `docs/architecture/core-design.md` | 11 |
| `ai/digests/subscriber.md` | 11 |
| `docs/architecture/testing/ci-format.md` | 10 |

## Destinations that must be created first

These four have no file yet. Routing stalls on them until one exists.

- `docs/architecture/behavior/graceful-restart.md` <!-- doc-links: ignore (destination to create) -->
- `docs/architecture/bgp-policy-filters.md` <!-- doc-links: ignore (destination to create) -->
- `docs/architecture/installer.md` <!-- doc-links: ignore (destination to create) -->
- `docs/architecture/service-systemd.md` <!-- doc-links: ignore (destination to create) -->

## The plan

| Summary | Test | Destination | Evidence |
|---------|------|-------------|----------|
| `401-rib-replay-on-peerup.md` | A | `ai/digests/rib.md` | "Replay runs after releasing the write lock to avoid deadlock (buildReplayCommands takes RLock, updateRoute does I/O)" |
| `402-rpki-1-validation-gate.md` | B | `ai/digests/rib.md` | "The validation gate must handle the case where bgp-rpki crashes mid-validation. The timeout sweep is the safety net -- without it, routes would be stu" |
| `403-rpki-5-wiring.md` | C | `ai/digests/rib.md` | "Config must include both the rpki plugin AND the adj-rib-in plugin. Without adj-rib-in, the validation gate commands have no handler." |
| `404-rpki-decorator.md` | B | `ai/digests/plugin-transport.md` | "Decorator is a plugin (not engine infrastructure) -- engine stays content-agnostic" |
| `405-llgr-1-capability.md` | A | `rfc/short/rfc9494.md` | "LLGR MUST be ignored if GR capability (code 64) is not also present -- tested in 'TestHandleEventOpenLLGR_NoGR'" |
| `406-llgr-2-state-machine.md` | A | `docs/architecture/behavior/graceful-restart.md` | "Callback-based event flow: state manager fires 'onLLGREnter', 'onLLGRFamilyExpired', 'onLLGREntryDone', 'onLLGRComplete' after releasing the lock -- p" |
| `407-llgr-3-rib-integration.md` | B | `docs/architecture/route-selection.md` | "Spec designed 'rib enter-llgr' and 'rib depreference-stale' as dedicated commands, but implementation used generic 'attach-community' + 'delete-with-c" |
| `408-dynamic-send-types.md` | B | `ai/rules/plugins.md` | "A hollow new plugin would violate single-responsibility." |
| `409-gc-pressure-reduction.md` | A | `ai/rules/performance.md` | "Stack arrays beat sync.Pool for function-scoped caches." |
| `410-validate-completion.md` | A | `ai/rules/completion.md` | "YANG list keys (e.g., 'family[name]') go through 'listKeyCompletions()', not 'valueCompletions()' -- ze:validate on a list key leaf does NOT automatic" |
| `411-peer-selector-asn.md` | C | `ai/rules/commands.md` | "Adding a new selector format requires updating all three." |
| `412-rpki-test-isolation.md` | C | `ai/rules/plugins.md` | "config uses short names ('adj-rib-in'), registry uses full names ('bgp-adj-rib-in'). 'hasConfiguredPlugin()' did exact match only." |
| `413-prefix-limit.md` | B | `ai/digests/bgp-reactor.md` | "Prefix maximum is mandatory per family, not optional. Chose strictness over convenience because an unconfigured peer defeats the safety purpose." |
| `414-forward-barrier.md` | C | `ai/rules/testing.md` | "'cmd=api' lines in '.ci' tests are commands that ze-peer sends to ze via the API, not just declarations." |
| `416-login-warnings.md` | B | `ai/digests/cli-editor.md` | "Chose closure injection via 'SetLoginWarnings' over a WarningProvider registry on the plugin server, because the SSH server has no reference to the pl" |
| `417-perf.md` | B | `docs/guide/benchmarking.md` | "TCP-only benchmarking over in-process testing because fair cross-implementation comparison requires real network." |
| `418-rib-family-ribout.md` | C | `docs/architecture/api/commands.md` | "The command dispatch model splits 'rib clear out !192.168.1.1 ipv4/unicast' into command='rib clear out', peer='*', args='[!192.168.1.1, ipv4/unicast]" |
| `419-arch-7-subsystem-wiring.md` | B | `docs/architecture/subsystem-wiring.md` | "Bus is a **notification layer**, not data transport — chosen over routing UPDATEs through Bus because 'OnMessageReceived' returns cache consumer count" |
| `420-arch-8-config-provider-wiring.md` | B | `ai/digests/hub-engine.md` | "Added 'Provider.SetRoot(name, tree)' to accept pre-parsed config trees — chosen over 'Provider.Load(path)' because config is already parsed by the YAN" |
| `421-arch-9-plugin-manager-wiring.md` | B | `docs/architecture/plugin-manager-wiring.md` | "first attempt defined 'RunPluginStartup(ctx)' on Server, but PluginManager.StartAll fires before Server exists" |
| `422-structured-delivery.md` | C | `ai/rules/plugins.md` | "Any event type the handler doesn't process is silently dropped" |
| `423-reactor-bus-subscribe.md` | B | `ai/digests/bgp-reactor.md` | "'Deliver()' must never hold 'reactor.mu' (deadlock risk with 'publishBusNotification')" |
| `424-forward-backpressure.md` | B | `docs/architecture/forward-congestion-pool.md` | "The industry consensus (BIRD, GoBGP, RustBGPd) is unanimous: never drop routes silently." |
| `426-blob-namespaces.md` | B | `docs/architecture/zefs-format.md` | "if init writes 'meta/ssh/*' but client reads 'ssh/*', all CLI commands break" |
| `427-per-user-drafts.md` | B | `ai/digests/cli-editor.md` | "Change file format must be sparse (only changes), not full tree." |
| `428-command-history.md` | C | `ai/rules/testing.md` | "The '.ci' test framework cannot test interactive TUI features" |
| `429-prefix-limit.md` | C | `ai/rules/testing.md` | "AC-linked tests must assert behavior, not mechanism absence." |
| `430-peer-local-remote.md` | C | `ai/digests/config-pipeline.md` | "Two separate 'parsePeersFromTree' functions exist (config.go and reactor_api.go)" |
| `431-update-verb.md` | B | `ai/rules/commands.md` | "YANG module named 'ze-cli-update-cmd.yang' (not 'ze-update-cmd.yang') to follow the 'ze-cli-*-cmd.yang' convention used by other verb packages." |
| `432-rpki-6-container-test.md` | C | `ai/rules/testing.md` | "'assert' in a subtest allows downstream subtests to run against an empty cache, producing false passes" |
| `433-interop-coverage.md` | C | `docs/architecture/testing/interop.md` | "'destination-ipv4' works in config blocks but the text API requires 'destination'" |
| `434-apply-mods.md` | A | `docs/architecture/update-building.md` | "If a filter reuses a buffer across calls, data corruption is possible." |
| `435-show-restructure.md` | B | `ai/digests/cli-editor.md` | "Sources select WHAT to display; pipes modify HOW it's displayed." |
| `436-route-loop-detection.md` | A | `docs/architecture/route-selection.md` | "AS loop detection applies to both eBGP and iBGP (RFC 4271 makes no distinction)." |
| `437-hub-lockfree-dispatch.md` | A | `docs/architecture/hub-architecture.md` | "Every post-freeze mutation path (Unregister, UnregisterAll) must republish the frozen snapshot." |
| `438-event-stream.md` | C | `docs/architecture/api/commands.md` | "Lowercasing args destroys case-sensitive peer names." |
| `439-rib-extensibility.md` | A | `ai/digests/rib.md` | "'RegisterCommunityName' is idempotent (same value+name is a no-op) but rejects re-registration with a different name." |
| `440-bgp-dashboard.md` | C | `ai/digests/cli-editor.md` | "Dashboard JSON parsing must expect the summary wrapper, not a response envelope." |
| `441-gr-marker.md` | C | `docs/architecture/testing/ci-format.md` | "'reject=stderr:contains=' is silently ignored by the .ci test runner" |
| `442-filter-community.md` | A | `ai/rules/plugins.md` | "New plugins with filters MUST set FilterStage explicitly; zero value = FilterStageProtocol (highest priority)." |
| `443-command-inventory.md` | A | `ai/rules/commands.md` | "The script requires blank imports for every cmd package that registers RPCs. New cmd packages must be added to the import list." |
| `444-fleet-config.md` | B | `docs/architecture/fleet-config.md` | "'config-fetch' has no name parameter; the hub uses the authenticated name. Prevents one client from fetching another's config." |
| `445-forward-congestion-phase4-5.md` | B | `docs/architecture/forward-congestion-pool.md` | "Chose **GR-aware teardown** (TCP close without NOTIFICATION for GR peers, Cease/OutOfResources for non-GR) over uniform NOTIFICATION." |
| `446-feature-inventory-ci-gaps.md` | C | `ai/rules/completion.md` | "Completion scripts (bash.go, zsh.go) and their tests maintain independent command lists that must stay in sync with the dispatch table" |
| `447-fsm-transitions.md` | B | `docs/architecture/behavior/fsm.md` | "'EventConnectRetryTimerExpires' is architecturally unreachable: Ze's blocking DialContext means the FSM never sits in Connect/Active waiting for a ret" |
| `448-handler-reorg.md` | A | `ai/rules/plugins.md` | "'all/all.go' can only contain imports whose transitive closure doesn't include 'plugin/process' or 'plugin/server'." |
| `449-strip-private.md` | C | `docs/architecture/testing/ci-format.md` | "'test/parse/' only extracts 'stdin=config' blocks and runs 'ze validate'. 'cmd=foreground', 'tmpfs=', and 'expect=stdout:contains=' are silently skipp" |
| `450-iter-elements.md` | A | `docs/architecture/wire/attributes.md` | "'AttrIterator.Next()' returns 'value' as a subslice of the original buffer. Writing to 'value[i]' writes directly to the underlying buffer" |
| `451-rib-show-filters.md` | C | `ai/rules/testing.md` | "Plugin subprocess stderr is consumed by 'relayStderrFrom()' inside the daemon" |
| `452-ssh-server.md` | C | `ai/digests/aaa-auth.md` | "SECURITY: omitting 'authentication {}' caused Wish to set 'NoClientAuth = true' -- always register a password handler that rejects all when no users c" |
| `453-prometheus-deep.md` | A | `ai/digests/observation-telemetry.md` | "'metrics list' returns metric family names. Vec metrics only appear in 'metrics values' after their first label combination is used" |
| `454-web-htmx-architecture.md` | C | `docs/architecture/web-interface.md` | "The web server started, accepted TLS, returned Go's default 404 for every request. No error logged." |
| `455-web-interface-session1.md` | C | `docs/architecture/web-components.md` | "Go templates fail silently on missing struct fields, and 'RenderFragment' returned empty string. Every HTMX click got an empty 200 response." |
| `456-zefs-integration.md` | A | `ai/digests/config-pipeline.md` | "'store.List(configDir)' returns an error when the directory prefix does not exist in an empty blob (unlike filesystem which returns empty slice)." |
| `457-forward-congestion-phase2.md` | A | `docs/architecture/forward-congestion-pool.md` | "'overflowItems{peer=X}' is destination X's overflow depth. 'overflowRatio{source=X}' is source X's overflow fraction." |
| `458-peer-groups.md` | A | `docs/architecture/resolve.md` | "the reactor never sees groups. Group name is injected as ''group-name'' key in each resolved peer map." |
| `459-plugin-tcp-transport.md` | B | `ai/digests/plugin-transport.md` | "SCM_RIGHTS FD passing only works over Unix domain sockets. With TLS transport, the connection handler feature needs a different mechanism if needed in" |
| `460-python-plugin-timeouts.md` | C | `docs/architecture/testing/ci-format.md` | "Hardcoded timeouts in Python plugin scripts are invisible to the Go timeout infrastructure" |
| `461-rpki-7-decoration.md` | B | `ai/digests/api-ipc.md` | "Attractive abstraction but creates tight coupling between UPDATE delivery and decorator latency (pending state, timeouts, chain tracking, JSON rebuild" |
| `462-yang-analysis.md` | B | `docs/architecture/config/yang-config-design.md` | "'ze schema' is discovery-oriented (list modules, show YANG text). 'ze yang' is analysis-oriented (find naming problems, generate docs). Different conc" |
| `463-zefs-socket-locking.md` | B | `plan/learned/DESIGN-HISTORY.md` | "Spec went through three major revisions before reaching the simple design: v1 was a new RPC protocol with editor-as-server (massively overengineered)," |
| `464-role-otc.md` | C | `ai/digests/bgp-reactor.md` | "Config keys by peer NAME (from config resolution), but filters look up by IP (from reactor's netip.Addr). Critical key mismatch found by deep review -" |
| `465-rpki-0-umbrella.md` | B | `docs/architecture/plugin/plugin-relationships.md` | "bgp-rpki sends 'adj-rib-in enable-validation' at startup. Routes held as pending until accept/reject command received. Modeled after GR retain/release" |
| `466-set-with.md` | C | `ai/digests/config-pipeline.md` | "'ApplyConfigDiff' in production re-reads from disk via 'reloadFn', ignoring the provided tree. This was the critical finding from deep review." |
| `467-route-metadata.md` | A | `docs/architecture/core-design.md` | "JSON round-trip converts numeric meta values to float64. Current usage (string values) unaffected. Future numeric meta must use comma-ok type assertio" |
| `468-web-1-foundation.md` | B | `docs/architecture/web-interface.md` | "'EventSource' (SSE) does not support custom headers -- this drove the switch from Basic Auth to session cookies." |
| `469-web-2-config-view.md` | C | `ai/digests/web.md` | "List keys consume 2 path segments during schema walk (list name + key value) -- the web handler must mirror this exactly or navigation breaks." |
| `470-web-3-config-edit.md` | C | `ai/digests/web.md` | "The 'RLock' fast path for session lookup had a write to 'lastActivity' -- data race caught by '-race' detector, fixed by using full 'Lock'." |
| `471-web-4-admin-commands.md` | B | `docs/architecture/web-interface.md` | "Three-tier URL scheme ('/show/', '/config/', '/admin/') over single '/api/' prefix -- matches CLI's two modes (edit + command) plus future fleet manag" |
| `472-web-5-cli-modes.md` | C | `ai/digests/web.md` | "Completer thread safety: 'Complete()' is read-only but 'SetTree()' is not -- web component must ensure no concurrent 'SetTree()' during autocomplete" |
| `473-web-6-live-updates.md` | B | `docs/architecture/web-interface.md` | "HTMX's SSE extension inserts event data via 'innerHTML' -- if the data contained unescaped HTML, it would be an XSS vector. Pre-rendering through 'htm" |
| `474-web-admin-finder.md` | B | `docs/architecture/web-components.md` | "Chose unified Finder navigation for both config and admin over card stacking, because the command tree is hierarchical (peer > teardown, cache > list)" |
| `475-web-0-umbrella-retrospective.md` | C | `ai/rules/planning.md` | "ACs tied to specific visual layouts are brittle -- behavior ACs ('user can navigate to a peer and see its config') survive design evolution, layout AC" |
| `476-env-registry-consistency.md` | B | `docs/architecture/config/environment.md` | "Adopted '<domain>.<component>[.<concern>]' naming for log subsystems over flat names. Domains: 'bgp', 'plugin', 'web', 'hub', 'cli', 'chaos'." |
| `477-zefs-key-registry.md` | B | `docs/architecture/zefs-format.md` | "Chose centralized registration in 'pkg/zefs/' (bottom of import graph) over scattered per-package registration, because consumers like 'cmd/ze/init/' " |
| `478-decorator.md` | B | `docs/architecture/web-components.md` | "Chose YANG extension ('ze:decorate') over hardcoded renderer logic, because it keeps enrichment declarations in the schema rather than scattered in Go" |
| `480-consistency-cleanup.md` | A | `ai/rules/performance.md` | "'ChunkMPNLRI' append looked like a buffer-first violation but is actually building a slice-of-subslices index (zero-copy). The buffer-first rule targe" |
| `481-managed-config.md` | B | `docs/architecture/fleet-config.md` | "Chose per-client secrets nested under server blocks ('server central { client edge-01 { secret } }') over a separate authorization layer. Auth = authz" |
| `482-prometheus-plugin-health.md` | A | `docs/architecture/plugin-manager-wiring.md` | "Delete only the status gauge label on disable (not counters), over deleting all labels, because Prometheus counters must not be deleted mid-lifetime (" |
| `483-exabgp-bridge-muxconn.md` | C | `ai/digests/plugin-transport.md` | "'bufio.Scanner' reads ahead in chunks -- reusing the scanner after the text-based startup is essential." |
| `484-unified-cli.md` | B | `ai/digests/plugin-transport.md` | "SSH channel as plugin transport, over TLS connect-back, because SSH already handles auth and provides a bidirectional stream." |
| `485-exabgp-dynamic-port.md` | B | `docs/functional-tests.md` | "Chose server-reports-port-via-stdout over environment variables or return channels because the test architecture already captures subprocess stdout to" |
| `486-cli-nav-sync.md` | C | `ai/digests/web.md` | "'cli_bar.html' is parsed in both the layout template set (no 'joinpath') and the fragments template set (has 'joinpath')." |
| `487-gr-marker-printf-portability.md` | C | `docs/architecture/testing/ci-format.md` | "All '.ci' scripts that write binary data via '/bin/sh' must use octal escapes, never hex." |
| `489-iface-0-umbrella.md` | B | `ai/digests/iface.md` | "JunOS-style two-layer units over VyOS flat model: physical/logical split, unit 0 = parent, VLAN units create OS subinterfaces via netlink." |
| `490-iface-1-monitor.md` | C | `ai/digests/iface.md` | "'isLinkUp' checks both 'OperState == OperUnknown' and 'IFF_UP' flag: virtual interfaces (dummy, veth) report 'OperUnknown' even when administratively " |
| `493-iface-4-advanced.md` | B | `ai/digests/iface.md` | "'TC_ACT_PIPE' for mirror action over 'TC_ACT_STOLEN': PIPE continues packet processing after mirroring, so the original traffic path is unaffected." |
| `494-iface-5-vm-tests.md` | C | `ai/rules/platform-linux.md` | "'runtime.LockOSThread()' is mandatory before any namespace switch. Without it, the Go scheduler can move the goroutine to a different OS thread, leavi" |
| `495-etxtbsy-test-runner.md` | C | `docs/architecture/testing/runner-architecture.md` | "The root cause is Go issue #22315: 'fork()' in one goroutine inherits write-open fds from another goroutine's 'os.WriteFile', causing 'ETXTBSY' on the" |
| `496-cli-dispatch.md` | A | `ai/rules/commands.md` | "New commands only need a YANG module (with 'ze:command' extension) and an 'init()' handler registration. No static help strings, no switch cases." |
| `497-check-ci-slowness.md` | C | `ai/digests/plugin-transport.md` | "'read_line()' checked '_pending_events' but not '_pending_requests'." |
| `498-lg-overhaul.md` | C | `docs/architecture/web-interface.md` | "Adding 'WriteTimeout: 60s' to the HTTP server breaks SSE connections." |
| `499-rib-inject.md` | A | `ai/digests/rib.md` | "Routes injected via 'rib inject' are indistinguishable from routes received via BGP." |
| `500-rib-inject-nexthop-validation.md` | A | `docs/architecture/encoding-context.md` | "the registry never removes entries by design. Old IDs remain valid. Peers reconnecting with different capabilities get new IDs." |
| `501-watchdog-ssh-perf.md` | B | `docs/functional-tests.md` | "Chose persistent SSH connection (one connect, many exec_command calls) over making" |
| `502-signal-status-command.md` | C | `docs/architecture/behavior/signals.md` | "'stop' and 'restart' are handled by SSH server hardcoded checks before the dispatcher, so they send bare strings" |
| `503-listener-0-umbrella.md` | A | `ai/rules/config.md` | "Adding a new listener service requires: YANG with 'uses zt:listener' + 'ze:listener' + 'enabled' leaf, and adding a row to 'knownListenerServices' in " |
| `504-colored-slog.md` | C | `docs/architecture/cli/color-system.md` | "Package-level eager loggers ('var logger = slogutil.Logger(...)') create handlers during init, before 'main()' parses flags." |
| `505-help-colors.md` | C | `docs/architecture/cli/color-system.md` | "ANSI codes add invisible bytes. '%-16s' padding counts bytes not visible characters, so colored names get under-padded." |
| `506-listener-6-compound-env.md` | B | `docs/architecture/config/environment.md` | "Chose compound 'ip:port,ip:port' format over separate host/port because it naturally maps to 'net.Listen()' arguments and supports multi-endpoint in a" |
| `507-resolve-cli.md` | A | `docs/architecture/resolve.md` | "'flag.Parse' stops at the first non-flag argument. 'ze resolve peeringdb max-prefix --url ...'" |
| `508-cli-route-topology.md` | A | `ai/digests/rib.md` | "MaxNodes guard (100) prevents terminal flooding on large topologies" |
| `509-llgr-4-readvertisement.md` | B | `rfc/short/rfc9494.md` | "EBGP non-LLGR: 'return false' (suppress) over ModAccumulator withdrawal -- ModAccumulator lacks a withdrawal mechanism. Explicit withdrawal happens wh" |
| `510-yang-required.md` | B | `docs/architecture/config/yang-config-design.md` | "Chose list-level extensions ('ze:required', 'ze:suggest') over leaf-level, because inheritance context belongs to the list, not the leaf. A leaf canno" |
| `511-llgr-0-umbrella.md` | A | `rfc/short/rfc9494.md` | "LLGR capability has no global header unlike GR -- tuple count is 'len(value) / 7', not parsed from a header field." |
| `512-healthcheck-1-watchdog-med.md` | A | `docs/guide/healthcheck.md` | "MED override bypasses the per-peer 'announced' boolean dedup, because the pool tracks state (announced/withdrawn) not command content -- without bypas" |
| `513-healthcheck-2-core.md` | A | `ai/rules/plugins.md` | "The 'cmd.Cancel' process group kill pattern should be used for any future shell execution in plugins." |
| `514-healthcheck-3-5-modes-ip-hooks-cli-external.md` | C | `plan/learned/HOOK-FRICTION.md` | "'block-silent-ignore.sh' hook rejects 'default:' in switch statements. Use explicit case listing or move unreachable return outside switch." |
| `515-port-defaults.md` | C | `docs/architecture/config/environment.md` | "Did not set a Default for ze.mcp.listen until YANG was fixed first -- the Default must be YANG-sourced, not a lie about a Go constant." |
| `517-multipeer-ci.md` | C | `docs/architecture/testing/ci-format.md` | "Differentiate peers by IP, not port. In BGP, peers are identified by IP." |
| `518-shell-completion-v2.md` | B | `ai/rules/completion.md` | "Added 'ValueHints func() []Suggestion' to 'command.Node', chose this over extending" |
| `519-fwd-auto-sizing.md` | C | `docs/architecture/forward-congestion-pool.md` | "**Swap-delete corrupted block IDs**: first single-pool implementation used" |
| `520-peer-yang-reorg.md` | A | `docs/architecture/config/yang-config-design.md` | "All plugin YANG augments now target 'bgp:session/bgp:capability' (three paths each: standalone peer, grouped peer, group). Any new plugin augmenting p" |
| `521-listener-7-migrate-remaining.md` | A | `docs/exabgp/exabgp-migration.md` | "Topic mapping ('packets' -> 'bgp.wire', not 'bgp.packets') is defined in 'internal/exabgp/topics/topics.go', not in the migration code itself. Future " |
| `522-fib-0-umbrella.md` | B | `ai/digests/fib-programming.md` | "Chose batch event format (one Bus event carries an array of prefix changes) over per-prefix events, because a full-table peer-down would produce 900K " |
| `523-iface-mac-discovery.md` | C | `ai/rules/go-standards.md` | "goimports silently drops imports when two packages share the same base name. Use aliased imports to disambiguate." |
| `524-fib-config-autoload.md` | B | `docs/architecture/plugin-manager-wiring.md` | "Chose convention-based auto-loading (config container present = start matching plugin) over a 'ze:load' YANG extension. The YANG is already loaded at " |
| `525-mcp-auto-tools.md` | B | `docs/architecture/mcp/overview.md` | "Group commands by prefix (depth-1 or depth-2), over one-tool-per-command. Produces ~15 tools with action enums instead of 60+ individual tools. Depth-" |
| `526-iface-backend-split.md` | C | `ai/digests/iface.md` | "'LoadBackend' MUST close the previous backend. The original implementation silently overwrote 'activeBackend', leaking the old monitor goroutine. Foun" |
| `527-fib-admin-distance.md` | B | `docs/architecture/route-selection.md` | "Moved admin-distance from 'fib { }' to 'sysrib { }' YANG config, over keeping it under fib." |
| `528-yang-path-separator.md` | A | `ai/rules/config.md` | "The 'yang' sub-package is the one place that must use '/' directly due to the import cycle." |
| `529-retrospective-319-sessions.md` | C | `plan/learned/RECURRING-PATTERNS.md` | "Deferred functional tests are the #1 gap that rules and hooks have not yet closed" |
| `530-bgp-as-plugin.md` | C | `docs/architecture/core-design.md` | "'hasConfiguredPlugin' substring match was the root cause of 63/218 test failures." |
| `531-config-inline-container.md` | C | `ai/rules/config.md` | "This is a class of bug: text-based tree manipulation is fragile when the text format changes." |
| `532-mcp-0-umbrella.md` | B | `plan/learned/DESIGN-HISTORY.md` | "Closed umbrella without implementing child specs (mcp-1 through mcp-4), because auto-generation (learned/525) superseded the entire plan." |
| `533-bgp-boundary-cleanup.md` | B | `docs/architecture/core-design.md` | "Three new init()-based registration hooks: 'RegisterPluginExtractor', 'RegisterMigrateFunc', 'RegisterReactorFactory'. All set-once-at-init, read-at-r" |
| `534-rib-alloc.md` | B | `docs/architecture/plugin/rib-storage-design.md` | "Chose BART for non-ADD-PATH only, map fallback for ADD-PATH. BART keys on 'netip.Prefix' which has no path-ID concept." |
| `535-config-tx-consumers.md` | A | `docs/architecture/config/transaction-protocol.md` | "WantsConfig must be set in sdk.Registration (Stage 1 runtime) for a plugin to" |
| `536-family-registry.md` | B | `docs/architecture/core-design.md` | "**Package location: 'internal/core/family/'** over 'internal/component/bgp/nlri/' because Family/AFI/SAFI are core protocol primitives, not BGP-specif" |
| `537-config-tx-protocol.md` | B | `docs/architecture/config/transaction-protocol.md` | "Plugins within a tier run concurrently so the tier cost is the max; tiers are serialized so tier costs are summed." |
| `538-report-bus.md` | B | `docs/architecture/api/commands.md` | "Producers MUST pick the right severity; the bus does not auto-promote." |
| `539-decouple-0-umbrella.md` | B | `docs/architecture/core-design.md` | "The 'InfraHook' pattern introduces a global callback in bgpconfig -- if the hook is not set (e.g., tests calling 'LoadReactor' directly), SSH/authz wi" |
| `540-decouple-1-cli-contract.md` | B | `docs/architecture/core-design.md` | "cli.Completer.SetTree signature changed from *config.Tree to any -- callers passing *config.Tree are unaffected, but the compiler no longer catches wr" |
| `542-plugin-metrics.md` | A | `docs/plugin-development/metrics.md` | "Chose 'ze_{scope}_{subject}_{detail}' naming taxonomy over ad-hoc names: counters always end in '_total', gauges never do, histograms use unit suffix " |
| `543-redistribution-filter-phase2.md` | B | `ai/digests/bgp-reactor.md` | "AS-PATH is skipped in the text delta, over allowing text-level replacement. EBGP AS-PATH prepend happens at the wire layer in ForwardUpdate" |
| `544-api-0-umbrella.md` | B | `docs/architecture/api/architecture.md` | "Chose lazy OpenAPI generation ('sync.Once' on first request) over eager startup generation, because plugins register commands during startup after the" |
| `545-debug-plugin-test-cluster.md` | C | `ai/rules/evidence.md` | "'known-failures.md' hypotheses that say 'not caused by X work' are not proof." |
| `546-bio-routing-followups.md` | A | `docs/architecture/behavior/fsm-established.md` | "ROUTE-REFRESH does not fire 'EventUpdateMsg' and therefore does not" |
| `547-record-parse-peer-block-hardening.md` | C | `ai/rules/testing.md` | "Every future '.ci' file with 'option=env' inside 'stdin=peer' fails at" |
| `548-cmd-4-prefix-filter.md` | A | `ai/digests/bgp-reactor.md` | "**Non-unicast families (EVPN, flowspec, VPN, BGP-LS, MVPN) flow through the prefix filter unchanged** because 'FormatUpdateForFilter' does not emit 'n" |
| `549-plugin-startup-dispatcher-barrier.md` | B | `docs/architecture/api/process-protocol.md` | "Chose this over a barrier in the startup coordinator because the coordinator" |
| `550-ci-observer-exit-code-fix.md` | C | `ai/rules/testing.md` | "This pattern is silently broken: 'ze-test' checks only ze's exit code." |
| `552-cmd-4-prefix-filter-phase2.md` | A | `ai/digests/bgp-reactor.md` | "**extractLegacyNLRIOverride must return a non-nil empty slice (not nil)" |
| `553-cmd-4-docs-sweep.md` | B | `plan/learned/DESIGN-HISTORY.md` | "**Rejected: adding a standalone 'docs/guide/prefix-list.md' page.**" |
| `554-named-service-listeners.md` | B | `docs/architecture/hub-architecture.md` | "**All-or-nothing bind with rollback over 'bind what you can'.**" |
| `555-bfd-skeleton.md` | B | `docs/architecture/bfd.md` | "**BIRD 3 express-loop over goroutine-per-session.** At 50 ms intervals" |
| `557-iface-tunnel.md` | A | `ai/digests/iface.md` | "**'ze config validate' does NOT invoke plugin OnConfigVerify**: it only" |
| `558-ci-observer-per-test-audit.md` | C | `ai/rules/testing.md` | "**No forwarding plugin loaded means egress filters never fire.** ze" |
| `559-bfd-2-transport-hardening.md` | A | `docs/architecture/bfd.md` | "**'IP_RECVTTL' delivers the TTL as a 32-bit int in host byte order**" |
| `561-bfd-4-operator-ux.md` | C | `ai/rules/plugins.md` | "**Plugin initialization ordering is now a cross-cutting concern.**" |
| `562-bfd-5-authentication.md` | B | `docs/architecture/bfd.md` | "**Simple Password rejected at parse time, not at runtime.**" |
| `563-bfd-6-echo-mode.md` | B | `docs/architecture/bfd.md` | "**Echo wire format is a 16-byte 'ZEEC' envelope.**" |
| `565-bfd-3b-frr-interop.md` | C | `ai/rules/interop-and-goal-validation.md` | "**Interop scenarios are not part of 'make ze-precommit-verify'.**" |
| `567-iface-tunnel-mac-per-case.md` | A | `ai/digests/iface.md` | "'parseTunnelEntry' is specific to tunnels and must not be applied generically." |
| `568-listener-dynamic-walk.md` | A | `docs/architecture/config/yang-config-design.md` | "The 'hasEnabledLeaf' check must inspect the schema parent container, not the config" |
| `569-cmd-5-aspath-filter.md` | C | `docs/architecture/config/syntax.md` | "Ze's config parser consumes backslashes in quoted strings: ''\d'' becomes ''d''." |
| `570-cmd-6-community-match.md` | C | `plan/learned/DESIGN-HISTORY.md` | "The spec said 'extend existing bgp-filter-community' but that was architecturally wrong." |
| `571-cmd-7-route-modify.md` | A | `ai/digests/bgp-reactor.md` | "'textDeltaToModOps' skips 'as-path' and 'nlri' -- these cannot be modified via the text delta mechanism." |
| `573-cmd-0-umbrella.md` | B | `docs/architecture/bgp-policy-filters.md` | "**Filter plugins, not route-maps.** Ze separates match from modify for composability" |
| `575-ipv6-forward.md` | C | `ai/rules/testing.md` | "Each 'expect=bgp:conn=N:seq=M:contains=' line in '.ci' tests consumes a separate message. Multiple 'contains=' checks on a single-message flow cause t" |
| `576-gokrazy-1-dhcp-wiring.md` | B | `ai/digests/iface.md` | "Factory callback pattern ('SetDHCPClientFactory') over direct import -- the iface package" |
| `577-gokrazy-2-ntp.md` | C | `ai/rules/config.md` | "YANG registration requires TWO things: (1) 'YANG' field on Registration for runtime," |
| `578-gokrazy-3-build.md` | B | `ai/rules/platform-linux.md` | "Removed 'WaitForClock: true' from ze's package config -- ze owns the clock via" |
| `579-gokrazy-4-resilience.md` | C | `ai/digests/iface.md` | "The v6 resolv.conf write initially used an early 'return' when the path was" |
| `580-gokrazy-0-umbrella.md` | C | `ai/rules/platform-linux.md` | "gokrazy's 'WaitForClock: true' had to be removed from config.json since ze" |
| `581-sysctl-0-plugin.md` | C | `ai/rules/plugins.md` | "**'ze' binary must be rebuilt** after adding blank imports to 'all.go'. Config validation" |
| `582-iface-route-priority.md` | A | `ai/digests/iface.md` | "Upper bound 4294966271 (2^32 - 1 - 1024) ensures configured + 1024 never overflows uint32." |
| `583-sysctl-1-profiles.md` | C | `docs/architecture/core-design.md` | "'clearProfileDefaults' interface matching must use '.conf.'+name+'.' to avoid VLAN substring" |
| `584-fw-1-data-model.md` | B | `docs/architecture/core-design.md` | "Chose abstract expression types over nftables-native types, because VPP should not need to reverse-engineer nftables register chains. 42 types: 18 mat" |
| `585-fw-4-yang-config.md` | C | `ai/rules/go-standards.md` | "Rate suffix matching with map iteration is non-deterministic in Go. Always use ordered slice for prefix/suffix stripping when shorter entries are subs" |
| `586-fw-2-firewall-nft.md` | B | `ai/digests/firewall.md` | "Non-Linux stub returns an error immediately from the factory, over providing a no-op backend, because silent no-ops would mask misconfiguration." |
| `587-fw-3-traffic-netlink.md` | A | `docs/architecture/core-design.md` | "tc handle math uses '(major << 16) \| minor'. Getting this wrong produces silent misrouting of traffic. The 'makeHandle' helper centralizes this." |
| `588-fw-5-cli.md` | C | `plan/learned/HOOK-FRICTION.md` | "goimports removes imports for packages it can't resolve (new packages in the same module). Add import and usage in the same Edit call. Even then, the " |
| `589-iface-ipv6-default-route.md` | B | `ai/digests/iface.md` | "Chose netlink NeighSubscribe + NTF_ROUTER flag over raw RA packet parsing, because the kernel already processes RAs and exposes router identity throug" |
| `590-cmd-1-rr-nexthop.md` | B | `ai/digests/bgp-reactor.md` | "The 'AttrModHandler' registry is now the established pattern for all egress attribute modification (ORIGINATOR_ID, CLUSTER_LIST, next-hop, community s" |
| `591-cmd-3-multipath.md` | B | `ai/digests/rib.md` | "Post-selection approach: 'SelectMultipath' runs AFTER 'SelectBest' picks the single winner, then scans remaining candidates for equal-cost matches. Th" |
| `592-cmd-9-ops.md` | A | `ai/digests/rib.md` | "The 'comparePairWithReason' function is the single source of truth for both 'ComparePair' (hot path, discards narrative) and 'SelectBestExplain' (CLI " |
| `594-l2tp-1-wire.md` | C | `docs/architecture/wire/l2tp.md` | "**Length field validation is two-step.** The parser's initial Length" |
| `595-l2tp-2-reliable.md` | A | `docs/architecture/wire/l2tp.md` | "**Retransmit 'Nr' rewrite relies on control-header layout stability.**" |
| `596-l2tp-3-tunnel.md` | C | `ai/digests/subscriber.md` | "Challenge AVP length validation MUST run at the reactor edge (parseSCCRQ)," |
| `597-l2tp-4-session.md` | C | `rfc/short/rfc2661.md` | "Header Session ID in ICRP/OCRP must be the peer's assigned SID (recipient's" |
| `598-aaa-registry.md` | B | `ai/digests/aaa-auth.md` | "**Backend registration is by priority, not name order**: tacacs at 100" |
| `599-l2tp-5-kernel.md` | C | `ai/digests/subscriber.md` | "'SetKernelWorker' must be called BEFORE 'reactor.Start()'. The reactor" |
| `600-user-login.md` | B | `ai/digests/aaa-auth.md` | "**New 'ze:bcrypt' extension instead of overloading 'ze:sensitive'.** The" |
| `602-l2tp-6a-lcp-base.md` | B | `ai/digests/subscriber.md` | "**Package at 'internal/component/ppp/', not 'internal/component/l2tp/ppp/'.** PPP is transport-agnostic" |
| `603-make-pool-audit.md` | A | `ai/digests/wire-and-pools.md` | "'internal/core/bufpool/' is now the canonical multi-goroutine" |
| `606-eventbus-typed.md` | B | `ai/digests/plugin-transport.md` | "**Replaced the EventBus interface outright** rather than adding a parallel" |
| `609-l2tp-6b-auth.md` | B | `ai/digests/subscriber.md` | "**Proxy-auth (RFC 2661 §18 'trust-proxy') dropped** after surveying" |
| `610-vpp-7-test-harness.md` | C | `ai/digests/vpp-dataplane.md` | "**GoVPP 'sockclnt_create' is typed as 'ReplyMessage' and" |
| `611-vpp-1-lifecycle.md` | C | `ai/digests/vpp-dataplane.md` | "GoVPP AsyncConnect returns a channel of connection events. The Manager" |
| `612-vpp-6-telemetry.md` | C | `ai/digests/vpp-dataplane.md` | "Stats client connects to a different socket than the binary API" |
| `613-vpp-2-fib.md` | B | `ai/digests/vpp-dataplane.md` | "**Per-route dispatch** over time-based batch accumulation. VPP has no multi-route batch" |
| `614-fmt-0-append.md` | B | `ai/rules/performance.md` | "**AppendText shape, not WriteTo.** The stdlib idiom ('strconv.AppendUint'," |
| `615-vpp-4-iface.md` | B | `ai/digests/vpp-dataplane.md` | "**Lazy channel acquisition over eager dial.** 'newVPPBackend'" |
| `616-l2tp-6c-ncp.md` | B | `ai/digests/subscriber.md` | "**Reuse the RFC 1661 FSM verbatim for NCPs** over a generic parameterized" |
| `619-fmt-1-text-update.md` | B | `ai/digests/wire-and-pools.md` | "**Rename 'nlri.JSONWriter' -> 'nlri.JSONAppender', change signature to" |
| `620-l2tp-7-subsystem.md` | B | `ai/digests/subscriber.md` | "**Service locator in the 'l2tp' package itself**, not a" |
| `621-backend-feature-gate.md` | B | `ai/rules/config.md` | "**Chose a declarative YANG extension 'ze:backend '<names>'' over a Go-side registry.**" |
| `622-l2tp-7b-ci-coverage.md` | A | `ai/digests/hub-engine.md` | "Subsystem authors can trust that 'Reload(ctx, cfg)' will be called" |
| `623-fw-9-traffic-lifecycle.md` | C | `ai/rules/plugins.md` | "Removing a plugin's root entirely auto-stops the plugin" |
| `624-fmt-2-json-append.md` | C | `plan/learned/HOOK-FRICTION.md` | "Test deletion hook still requires user approval." |
| `625-rs-fastpath-1-profile.md` | A | `ai/digests/plugin-transport.md` | "The cost is structural -- every batch the rs plugin sends becomes a text command that the engine re-tokenizes." |
| `626-rs-fastpath-2-adjrib.md` | A | `ai/rules/plugins.md` | "when declaring an 'OptionalDependencies' entry, the owner MUST provide a run-time fallback for absence" |
| `628-env-cleanup.md` | C | `ai/rules/config.md` | "'env.MustRegister' silently overwrites on duplicate key." |
| `632-op-1-easy-wins.md` | A | `ai/rules/commands.md` | "Future CLI commands should default to the single-key wrapper shape" |
| `633-op-0-umbrella.md` | B | `ai/digests/firewall.md` | "Firewall config ('firewall { table ... }') now actually reaches the" |
| `634-bgp-redistribute.md` | B | `plan/learned/DESIGN-HISTORY.md` | "After the implementation landed, the user pointed out that the design they" |
| `635-fw-10-linux-gaps.md` | A | `ai/rules/config.md` | "'type empty' is now effectively banned in the firewall schema. Future schema additions should use presence containers." |
| `641-l2tp-7c-redistribute.md` | C | `ai/digests/subscriber.md` | "'ReleaseBatch' zeroes the batch after Emit returns." |
| `642-l2tp-7c-ac8-multi-peer-nexthop.md` | C | `ai/rules/testing.md` | "The 'option=bind:value=' directive in ze-peer stdin blocks is not" |
| `643-bgp-functional-test-evidence.md` | C | `ai/rules/evidence.md` | "'docs/functional-tests.md' now says that egress claims require an actual" |
| `644-request-context-and-caller-identity.md` | A | `docs/architecture/api/commands.md` | "Preserve the rule that identity is injected only by trusted wiring." |
| `649-system-nameserver.md` | B | `ai/digests/dns-services.md` | "Static name-servers take priority over DHCP-discovered servers: when static servers are configured, DHCP skips resolv.conf writes." |
| `651-policy-routing.md` | B | `ai/digests/firewall.md` | "Per-policy tables were rejected because multiple base chains at the same hook/priority produce non-deterministic evaluation order in nftables" |
| `652-diag-1-runtime-state.md` | C | `ai/rules/planning.md` | "umbrella spec assumed it didn't. Always verify 'does not exist' claims during" |
| `654-config-3-deactivate.md` | C | `ai/digests/cli-editor.md` | "'Editor.WalkPathWithSchema' halts at leaves (returns nil because 'walkSchemaNode' returns '(false, nil, 0)' for leaf-like types); using it for termina" |
| `655-l2tp-12-ppp-interop-lab.md` | B | `ai/rules/interop-and-goal-validation.md` | "Docker preflight probe pattern: run a temporary --rm --privileged container" |
| `657-l2tp-8a-auth-pool.md` | B | `ai/digests/aaa-auth.md` | "MS-CHAPv2 explicitly rejected by local auth." |
| `658-l2tp-8b-radius.md` | A | `ai/digests/aaa-auth.md` | "Acct byte counters are hardcoded to 0: kernel counters are not yet wired" |
| `659-l2tp-8c-shaper.md` | B | `ai/digests/subscriber.md` | "**TBF and HTB only.** Config validation rejects other qdisc types." |
| `662-l2tp-0-umbrella.md` | B | `ai/digests/subscriber.md` | "**Reactor pattern throughout.** No per-tunnel goroutines." |
| `667-bng-4-scale-testing.md` | B | `docs/architecture/testing/runner-architecture.md` | "**Loopback only:** No root, no namespaces, no Docker, no kernel modules." |
| `668-bng-3-ipv6-pools.md` | C | `ai/digests/subscriber.md` | "**Port 547 bind conflict.** Multiple sessions each binding [::]:547 with SO_BINDTODEVICE requires SO_REUSEADDR; without it, only the first session get" |
| `670-fw-8-lns-gaps.md` | C | `ai/digests/firewall.md` | "**Audit before implementing.** Discovering that all four gaps were already coded saved reimplementation." |
| `672-fw-0-umbrella.md` | C | `ai/digests/firewall.md` | "**VPP ACLInterfaceSetACLList replaces full list.** Input and output ACLs" |
| `674-bgp-1-interop-fixes.md` | C | `ai/rules/interop-and-goal-validation.md` | "**FRR leniency masks bugs**: FRR accepted a 4-byte VPN next-hop that GoBGP (correctly) rejected." |
| `680-config-4-container-inactive.md` | B | `ai/digests/config-pipeline.md` | "**Delete 'isInactiveTree' helper**, replace all 14 call sites with '.IsInactive()'." |
| `683-mcp-0-umbrella-modernization.md` | B | `ai/digests/mcp.md` | "The protocol version stayed at '2025-06-18' throughout." |
| `685-redist-producers.md` | C | `ai/digests/rib.md` | "'addrPayload.Unit' must be 'int' to match 'iface.AddrPayload' and 'netlink.addrEventPayload'. An initial 'string' type caused silent JSON unmarshal ze" |
| `686-web-2-operator-workbench.md` | B | `docs/architecture/web-interface.md` | "**UIMode switch with env var rollback** over feature flags or per-user cookie switching." |
| `687-web-3-foundation.md` | B | `docs/architecture/web-components.md` | "**Data model structs ('WorkbenchTableData', 'WorkbenchDetailData', 'WorkbenchFormData')** over template-level construction." |
| `693-host-4-web.md` | B | `ai/digests/web.md` | "**HTMX polling at 10-second intervals** over SSE for live updates." |
| `694-web-1-identity.md` | B | `docs/architecture/web-interface.md` | "**Direct-link fleet model** over a proxy model. Each fleet peer entry is a simple URL link that opens the remote Ze instance in the browser." |
| `698-plugin-ipc-raw-bytes.md` | A | `ai/rules/performance.md` | "**Synchronous CallRPC is the safety invariant.** The 'unsafe.String' is" |
| `699-canonical-in-repo-docs.md` | C | `plan/learned/HOOK-FRICTION.md` | "Documentation-only specs have awkward fits with code-oriented spec validators" |
| `700-config-versioning.md` | C | `docs/architecture/zefs-format.md` | "which resolves to 'file/active/<name>.draft' in blob storage, not 'file/draft/<name>'" |
| `701-redist-explicit-dest.md` | C | `ai/rules/quality.md` | "**Consumer never registered (BLOCKER):** The interface and implementation" |
| `702-vpp-3-mpls.md` | C | `ai/digests/fib-programming.md` | "When adding a parallel data store (side-data), ensure all removal paths" |
| `703-cli-1-command-grammar.md` | B | `ai/rules/cli.md` | "Removed old command names entirely (no backward compat aliases) over a deprecation period, because ze is pre-1.0 and the CLI grammar is still stabiliz" |
| `704-vpp-0-umbrella.md` | B | `plan/learned/DESIGN-HISTORY.md` | "Retired. Design dead-end: VPP features belong under ze abstractions (iface/firewall/traffic), not a parallel config surface" |
| `707-gap-3-bgp-keepalive-timer.md` | C | `ai/rules/config.md` | "**Two config parsers, different tree shapes.** 'config.go:parsePeerFromTree'" |
| `709-gap-1-kernel-redistribute.md` | A | `ai/digests/fib-programming.md` | "Any future netlink route consumer registers with 'routewatch.Global()' instead of opening a new subscription." |
| `711-cpe-3-console-serial.md` | C | `ai/rules/platform-linux.md` | "rationalization 'requires a real serial device.' This was wrong. The project" |
| `712-cpe-4-conntrack-helpers.md` | C | `ai/rules/go-standards.md` | "Changing a struct's size can cascade through unrelated callers." |
| `713-textbuf-alloc-sweep.md` | A | `ai/rules/performance.md` | "**'Slice()' (zero-copy) is only safe for synchronous call chains.**" |
| `715-grpc-domain-types.md` | B | `ai/digests/api-ipc.md` | "\| All engine methods take '*Request' structs \| Uniformity, three-line handlers, positional safety \|" |
| `718-iface-3-unit-naming.md` | B | `ai/digests/iface.md` | "**Label, not Name.** The Go field is 'Label' not 'Name' because 'ifaceEntry'" |
| `719-iface-4-dhcp-per-family.md` | A | `ai/rules/config.md` | "**Kept 'ze:backend 'netlink'' on dhcp/dhcpv6.** Unlike 'ze:os', the" |
| `720-config-2-archive.md` | C | `ai/rules/plugins.md` | "YANG schema registration requires 'init()' imports in both 'plugin/all/all.go' (schema) and 'cmd/ze/cli/main.go' (RPC handler). Missing either causes " |
| `724-aigp-attribute.md` | B | `ai/digests/wire-and-pools.md` | "Pool dedup (AC-6) deliberately skipped. AIGP metrics are typically unique per route (accumulated cost varies per path), so a dedicated pool would have" |
| `731-ntp-1-diagnostics.md` | B | `ai/rules/plugins.md` | "Chose 'pluginserver.RegisterRPCs()' from the NTP plugin's 'init()' over a leaf state package" |
| `732-masquerade-options.md` | A | `ai/digests/firewall.md` | "'expr.Masq.ToPorts' gates the marshal path: when true, flags are ignored by the kernel." |
| `747-zefs-remote-creds.md` | B | `docs/architecture/zefs-format.md` | "This prevents confusing cross-contamination between env and pointer." |
| `749-ai-agent-tooling.md` | C | `plan/learned/HOOK-FRICTION.md` | "The 'block-test-deletion.sh' hook prevents removing test functions even when" |
| `751-flowspec-firewall.md` | A | `ai/digests/flow-ddos.md` | "Term ordering in chains must be deterministic (sorted by name hash) since nftables evaluates rules sequentially" |
| `766-json-safe-string-append.md` | A | `ai/digests/wire-and-pools.md` | "The safety contract is: callers must guarantee input is clean, either by" |
| `769-install-subcommand.md` | A | `ai/digests/config-pipeline.md` | "Config ends with '\x00' so ze can start parsing without waiting for EOF. Pipe stays open; EOF signals shutdown." |
| `773-doctor-coverage.md` | B | `ai/rules/repo-maintenance.md` | "RADIUS probe sends a real authenticated Access-Request rather than relying on UDP Dial (which always succeeds for unbound ports)." |
| `775-fib-richroute-functional-tests.md` | A | `ai/digests/fib-programming.md` | "sysrib does not populate RouteType or TableID on outgoing BestChangeEntry." |
| `777-service-systemd.md` | B | `docs/architecture/service-systemd.md (does not exist)` | "Warned on active config containing 'daemon { user }', because systemd already applies 'User=ze' before exec and a second in-process privilege drop can" |
| `778-cross-scope-verification.md` | C | `ai/rules/testing.md` | "Shared 'database.zefs' state can leak into functional tests if daemon-style 'ze <config>' runs use blob storage." |
| `779-transactional-config-commit.md` | B | `docs/architecture/config/transaction-protocol.md` | "Session candidate staging must write the version and candidate pointer under the same guard, or another commit can race in between." |
| `782-pol-2-actions.md` | C | `ai/digests/bgp-reactor.md` | "encodeCommunityValue concatenates all values into one buffer, but removeValues expects exactly valueSize bytes" |
| `790-debug-flags.md` | B | `docs/architecture/zefs-format.md` | "The 'state/debug/' namespace is the first use of 'state/' keys in zefs" |
| `793-show-bgp-peer-detail.md` | B | `ai/digests/bgp-reactor.md` | "'writeUpdate()' is a third write path alongside 'writeMessage()' and 'writeRawUpdateBody()'. All three need 'onWrite' for accurate last-write timestam" |
| `798-fib-depth-vpp-parity.md` | B | `ai/digests/fib-programming.md` | "TableID is always 0. The VPP delete path must look up the stored tableID from the installed map, not trust the incoming change." |
| `799-response-typed-data.md` | A | `docs/architecture/api/commands.md` | "Replace 'Data any' with 'Data ResponseData' (marker interface) and add 'Error string'." |
| `801-pki-certificate-export.md` | A | `ai/rules/cli.md` | "The dispatcher's prefix-matching model means any CLI command that takes a positional argument cannot have YANG sub-container children that add keyword" |
| `834-stripped-build-and-iso-coverage.md` | C | `ai/rules/testing.md` | "Functional coverage for new CLI behavior must follow the shipped entry point." |
| `836-unified-ze-test-selection.md` | A | `docs/architecture/testing/runner-architecture.md` | "Chose one shared 'runner.Selection' contract for '--list', '--all', '--start ID', '--pattern TEXT', and positional ids or names." |
| `840-web-test-wait-settle.md` | C | `docs/architecture/testing/runner-architecture.md` | "'agent-browser wait --fn' IGNORES '--timeout'." |
| `841-doc-drift-parser-claims.md` | C | `ai/rules/repo-maintenance.md` | "Stale parser claims can be semantically wrong while all source anchor paths still exist, so source-anchor validation is necessary but not sufficient." |
| `845-plugin-self-containment.md` | C | `docs/architecture/command-ownership.md` | "The pki export '.ci' was passing **falsely**: a broad 'try/except' swallowed" |
| `846-bgp-decode-encode-ownership.md` | C | `docs/architecture/command-ownership.md` | "Drop it and decode/encode silently vanish from help" |
| `847-rules-index-generator.md` | C | `ai/rules/repo-maintenance.md` | "'CLAUDE.md'/'AGENTS.md' are git-ignored generated artifacts: edit 'ai/INSTRUCTIONS.md' and run 'make ze-ai-instructions-generate'; they never appear in 'git st" |
| `848-command-surface-ownership.md` | C | `docs/architecture/command-ownership.md` | "**cmd/ze tests share one process registry.** Never call 'ResetForTest()' from a 'cmd/ze'" |
| `852-cmd-to-plugin.md` | C | `ai/rules/plugins.md` | "any 'register.go' under 'internal/plugins/' without 'codegen:skip' gets added to 'plugin/all'" |
| `855-clear-command-ownership.md` | B | `docs/architecture/command-ownership.md` | "**Full handler+schema move, not schema-only.** Moving only schema would leave the central package importing 'ike/engine' and depending on hub injectio" |
| `857-ze-setup-appliance-binary.md` | B | `ai/rules/architecture.md` | "Documentation uses 'bin/ze-setup' in all examples, not bare 'ze'. The appliance commands do not exist in the 'ze' binary, so examples must be honest a" |
| `861-ze-test-build-tags.md` | C | `ai/rules/architecture.md` | "Every new source file added to 'cmd/ze/' that imports daemon-only packages MUST have '//go:build !ze_test' or the ze-test binary will bloat." |
| `862-ze-chaos-build-tags.md` | A | `ai/rules/architecture.md` | "Every new build-tag variant of cmd/ze must add '&& !ze_<name>' to all existing exclusion tags." |
| `864-cmd-reorg.md` | C | `ai/rules/architecture.md` | "**Import cycles kill flat package placement.**" |
| `865-gomu-mutation-testing.md` | B | `plan/learned/DESIGN-HISTORY.md` | "Chose gomu over go-mutesting: go-mutesting has zero parallelism, making it unusable at Ze's scale" |
| `866-hook-dispatcher-consolidation.md` | C | `ai/rules/repo-maintenance.md` | "Aggregate parity can mask an individual broken check when a dominant hook ('pre-write-go', 'require-design-ref') also fires on the same file; per-hook" |
| `869-ze-test-one-based-ids.md` | C | `ai/rules/testing.md` | "Tests that build trees manually must use 'SetSlice('name-server', ...)' for leaf-lists and 'AddListEntry' only for keyed lists." |
| `871-owner-override-commit.md` | C | `ai/rules/git-safety.md` | "The agent treated an agent safety rule as if it constrained the owner, which turned a useful guardrail into friction." |
| `873-risks-assumptions-in-specs.md` | B | `ai/rules/planning.md` | "validation method per assumption over a likelihood/impact scoring matrix," |
| `874-session-commit-apply.md` | A | `docs/architecture/config/yang-config-design.md` | "Invariant (now documented in yang-config-design.md): leaf-list nodes MUST use the multi-value Tree API in every write/apply path; the scalar map only " |
| `877-web-ui-integrity.md` | C | `ai/digests/web.md` | "**Inline script in templates:** A test ('TestTemplatesAvoidInlineScriptAndStyle')" |
| `886-doctor-plugin-registration.md` | A | `ai/rules/plugins.md` | "When 'registry.Registration' needs a new validated field whose valid values come from" |
| `888-l2tp-env-promote.md` | B | `docs/architecture/config/environment.md` | "Used YANG 'range '0 \| 5..86400'' for reauth-interval over runtime clamping," |
| `889-reactor-tuning-yang.md` | A | `docs/architecture/config/environment.md` | "Assumption A-1 ('reactor config struct carries YANG leaves') was wrong. There is no config struct." |
| `890-rs-env-promote.md` | C | `ai/rules/config.md` | "Initial implementation used 'ze.rs.worker-queue-size' which violates the" |
| `893-config-nop-keyword.md` | B | `docs/architecture/config/syntax.md` | "**D-1 Keyword:** 'nop' ('no operation') chosen over 'off'/'rem'/'nil'/'not'. Precise semantics: the line exists but produces no operational effect." |
| `897-mechanical-anti-workaround-guards.md` | C | `plan/learned/HOOK-FRICTION.md` | "'hook-parity-check.py' runs the hook in a FRESH temp 'CLAUDE_PROJECT_DIR', so any" |
| `898-filter-irr-fixes.md` | C | `ai/rules/testing.md` | "A verify-failure snapshot is only as fresh as its tree; a feature committed" |
| `899-sleep-test-determinism.md` | A | `ai/rules/testing.md` | "A field/return/branch that only tests consume is a smell, even when it makes" |
| `902-paths-limit.md` | B | `docs/architecture/encoding-context.md` | "**Enforcement in CommitService only:** static route initial sync doesn't need enforcement (config-originated, one path per prefix)." |
| `910-installer-initrd-console-and-ci-gotchas.md` | C | `ai/rules/platform-linux.md` | "**A POSIX shell function whose last executed command is a conditional can abort" |
| `911-exabgp-flaky-eor-race-not-encoding-bugs.md` | C | `ai/rules/testing.md` | "Re-run the suite 2-3 times and diff the failure SETS before triaging." |
| `914-exabgp-compat-sync.md` | B | `docs/architecture/wire/attributes.md` | "Attribute JSON rendering uses a registry, not a switch." |
| `916-web-suite-harness-not-product-bugs.md` | C | `ai/rules/testing.md` | "A uniformly-failing suite is almost always ONE harness root cause, not N" |
| `917-sr-policy-migration.md` | A | `docs/architecture/update-building.md` | "'packAttributesOrderedInto' puts MP_REACH_NLRI last and appends 'rawAttrs' after that." |
| `918-exabgp-compat-sync.md` | C | `docs/architecture/testing/interop.md` | "the exabgp-compat test runner ('bin/bgp') skips ':json:' lines entirely" |
| `939-bug-review-1-inventory-and-self-containment.md` | C | `ai/rules/architecture.md` | "A package absent from 'plugin/all' is not automatically missing." |
| `940-bug-review-2-plugin-engine-and-system-plugins.md` | A | `ai/rules/plugins.md` | "A plugin that fails after registering commands, families, or capabilities must be indistinguishable from one that never loaded." |
| `941-bug-review-3-bgp-engine-core.md` | A | `ai/digests/bgp-reactor.md` | "Message validation must run before delivery to plugin or observer callbacks." |
| `942-bug-review-4-bgp-plugins-and-protocol-codecs.md` | C | `ai/rules/plugins.md` | "Silent parser fall-through is worse than unsupported input." |
| `943-bug-review-5-verification-and-fix-backlog.md` | C | `ai/rules/planning.md` | "A fix spec is incomplete if its Implementation Summary, Audit, Goal Validation, Review Gate, or Pre-Commit Verification section still has template pla" |
| `944-bug-review-backlog-closure.md` | C | `ai/rules/planning.md` | "Reusable lesson: review Task promises against the AC table, not just the ACs" |
| `946-qemu-runner-for-interop-labs.md` | C | `ai/rules/platform-linux.md` | "A Docker interop lab that depends on host-kernel features does not run on macOS" |
| `949-iface-resolve-1-model.md` | C | `ai/rules/go-standards.md` | "Growing a cross-boundary value type past gocritic 'rangeValCopy: sizeThreshold' breaks unchanged consumers repo-wide." |
| `951-iface-resolve-mac-match.md` | C | `ai/rules/config.md` | "Don't rely on YANG 'unique' for ze config validation." |
| `952-iface-resolve-consumers.md` | A | `ai/digests/iface.md` | "One-shot CLI commands lack the backend; in-daemon command handlers have it." |
| `953-iface-resolve-dispatch-guard.md` | B | `ai/digests/iface.md` | "Best-effort translation: identity on failure." |
| `954-friction-rule-pattern-reporting.md` | B | `ai/rules/repo-maintenance.md` | "Update the existing rule instead of creating a sibling." |
| `976-chaos-mrt-recording.md` | B | `docs/architecture/mrt.md` | "Carry wire bytes on 'peer.Event' via a new 'BGPMessage []byte' field, chosen over" |
| `978-tiers-4-ospf-self-containment.md` | C | `ai/rules/architecture.md` | "BREAK when you nest the guarded package under that very prefix" |
| `979-tiers-5-b1-unify-discovery.md` | C | `ai/rules/architecture.md` | "An all.go-only parse is NOT sufficient" |
| `985-unit-test-feature-tags.md` | C | `ai/rules/testing.md` | "**Absent-feature compile-out tests MUST live under 'cmd/ze/hub'.**" |
| `991-geodns-0-umbrella.md` | B | `ai/digests/dns-services.md` | "**Edge plugin, SDK engine.** geodns is a config-driven engine nothing depends on" |
| `996-observer-dispatch-show-prefix.md` | C | `ai/rules/testing.md` | "The failure mode is insidious: a wrong command string yields a green test, so" |
| `997-l2tp-dead-peer-detection.md` | B | `docs/architecture/wire/l2tp.md` | "Chosen over: (a) a hold-timer on 'lastActivity' -- which is deliberately NOT" |
| `998-observer-harness-hardening.md` | B | `plan/learned/DESIGN-HISTORY.md` | "Handoff was aspirational: API + test + a vendored fd-passing lib, but no engine" |
| `1001-ze-help-ai-subcommand-grammar.md` | B | `ai/rules/cli.md` | "Per-command hints do not scale." |
| `1002-mcp-ze-reference-aihelp.md` | C | `ai/digests/mcp.md` | "'internal/' cannot import 'cmd/ze' (package main)." |
| `1003-family-direction-policy.md` | A | `ai/digests/bgp-reactor.md` | "there is no single egress chokepoint." |
| `1004-cp-survival-1-bgp-gtsm.md` | C | `ai/digests/bgp-reactor.md` | "## The load-bearing trap: SYN-ACK comes from the listen socket" |
| `1009-selected-spec-to-per-session-marker.md` | C | `plan/learned/HOOK-FRICTION.md` | "Using it makes every SID lookup fail. Use 'set -eo pipefail'." |
| `1010-verify-producer-before-claiming.md` | C | `ai/rules/evidence.md` | "it is a hypothesis, not a finding." |
| `1012-root-layout-reorg.md` | C | `ai/rules/testing.md` | "returned 0 files, and hit 't.Skip('no config files found')'." |
| `1014-functional-test-timeout-flakiness.md` | A | `docs/architecture/testing/runner-architecture.md` | "Load-induced timeout flakiness is not 'raise this one timeout.'" |
| `1018-registration-over-hardcoding.md` | B | `ai/rules/plugins.md` | "Enforced in '.claude/hooks/validate-spec.sh' as a WARNING, not a hard ERROR," |
| `1020-tiers-5-structure-tidy.md` | B | `ai/rules/architecture.md` | "Did NOT cluster AAA (platform infra consumed by api, bgp, ssh, web)," |
| `1023-installer-network-rescue-gate.md` | B | `docs/architecture/installer.md` | "**NIC selection** — pin to the NIC iPXE booted from, carried on the kernel" |
| `1025-installer-dhcp-broadcast-flag.md` | A | `ai/rules/protocol.md` | "**A DHCP client doing DORA from 0.0.0.0 must set the BOOTP broadcast flag**" |
| `1026-installer-seed-overwrite.md` | A | `docs/architecture/installer.md` | "When two builders write the same on-disk artifact, the *last writer wins*" |
| `1047-cli-verb-first-followup.md` | C | `ai/rules/cli.md` | "**Moving a YANG 'ze:command' container changes its dispatch key.**" |
| `1053-config-surface-review-checks.md` | A | `ai/rules/config.md` | "**Nested ConfigRoot read with a flat key.**" |
| `1059-archive-credential-sanitization.md` | B | `ai/rules/config.md` | "The YANG schema defines 'location' as 'type string' with no constraint on embedded credentials; this is by design since HTTP basic auth via URL is a v" |
| `1061-service-source-address.md` | C | `ai/rules/go-standards.md` | "**Managed TLS hostname verification silently dropped.**" |
| `1063-ownership-1-rs-invariant.md` | A | `ai/digests/config-pipeline.md` | "Config-validation schema is a UNION of ALL init()-registered YANG modules" |
| `1064-ownership-2-coordinator-types.md` | A | `docs/architecture/plugin-manager-wiring.md` | "The registry leaf now has one component->component lateral edge ('config/storage')." |
| `1065-ownership-3-reactor-modes.md` | A | `ai/digests/bgp-reactor.md` | "Any future reactor consumer must now state its mode." |
| `1067-generated-discovery-indexes.md` | B | `ai/rules/repo-maintenance.md` | "Folding into 'ze-doc-verify' alone would have been a no-op gate" |
| `1068-digest-anchor-validator.md` | C | `ai/rules/repo-maintenance.md` | "The resolver fails closed on cross-base ambiguity" |
| `1073-ddos-flowspec-wire.md` | C | `ai/rules/testing.md` | "a peer-based functional test is not optional for origination features -- parsing green != sending works." |
| `1076-structural-gate-known-red.md` | C | `ai/rules/git-safety.md` | "A structural gate red is not 'pre-existing noise' to scope around." |
| `1078-unify-filters.md` | B | `docs/architecture/core-design.md` | "Egress is NOT symmetric with ingress: egress in-process filters defer into 'ModAccumulator' and the export policy chain reads the ORIGINAL payload" |
| `1079-unify-config-diff.md` | C | `ai/rules/stale-comments.md` | "A stale defensive comment can freeze a duplication in place" |
| `1081-unify-replay.md` | B | `docs/architecture/core-design.md` | "Canonical shape = the token-correlated request" |
| `1082-unify-response-envelope.md` | B | `docs/architecture/hub-api-commands.md` | "Winner envelope = 'plugin.Response' in place" |
| `1084-radius-admin-backend.md` | A | `ai/digests/aaa-auth.md` | "the effective auth chain is RADIUS(50) → TACACS+(100) → local(200). A reject stops the chain; a timeout/unreachable falls through to the next backend." |
| `1085-unify-tree-deactivation.md` | B | `ai/digests/config-pipeline.md` | "Effective-config accessors ('GetSlice'/'GetMultiValues'/'ToMap') return **active-only** clean values" |
| `1087-lint-linux-platform.md` | C | `ai/rules/platform-linux.md` | "'*_linux.go' is only linted when golangci runs on a linux host" |
| `1088-layout-1-hygiene.md` | C | `ai/rules/evidence.md` | "Lesson: when a spec lists referrers for a file move, disambiguate keyword-vs-filename" |
| `1090-layout-3-naming-glossary.md` | B | `ai/rules/go-standards.md` | "'wireu' KEPT, not renamed (user decision)" |
| `1091-layout-4-protocol-skeleton.md` | B | `ai/rules/protocol.md` | "Advisory-only, over an enforced gate" |
| `1109-ddos-detect-enhancements.md` | C | `ai/digests/flow-ddos.md` | "flow-export conntrack export needs FOUR kernel prerequisites, none provided by a bare module load" |
| `1114-ospf-0-umbrella.md` | C | `ai/rules/planning.md` | "When children are closed individually, put the umbrella's" |
| `1117-test-sync-quiesce.md` | A | `ai/rules/testing.md` | "the control plane is ALREADY synchronous" |
| `1119-cli-hyphen-namespace-split.md` | A | `ai/rules/cli.md` | "'ze:related command='...'' is production-validated, not cosmetic." |
| `1121-negative-test-must-fail-for-its-reason.md` | C | `ai/rules/testing.md` | "A negative test must fail for the reason it claims." |
| `1123-rib-arch-6-rs-fastpath-consumer.md` | A | `ai/digests/rib.md` | "AddRef under the RIB write lock, process off-lock in a worker." |
| `1125-rib-arch-5-bmp-locrib.md` | C | `ai/rules/testing.md` | "**The decoder could not round-trip its own Loc-RIB Peer Up.**" |
| `1128-rib-arch-umbrella-closure.md` | C | `plan/learned/HOOK-FRICTION.md` | "**A core-package change pulls its whole reverse-dep closure into 'ze-lint-changed',**" |
| `1129-dns-resolver.md` | A | `ai/rules/config.md` | "New YANG modules that define containers under 'environment' must be explicitly loaded in" |
| `1130-update-groups.md` | C | `ai/digests/bgp-reactor.md` | "The original 'single-threaded reactor' assumption was wrong for these lifecycle callbacks." |
| `1131-plugin-tls-hardening.md` | B | `ai/digests/plugin-transport.md` | "Kept 'InsecureSkipVerify: true' even with fingerprint pinning, because Go's TLS requires it to skip chain validation when using self-signed certs; 'Ve" |
| `1132-resolve-component.md` | C | `plan/learned/HOOK-FRICTION.md` | "Must add import and usage in the same edit." |
| `1133-listener-7-migrate-remaining.md` | C | `plan/learned/HOOK-FRICTION.md` | "The 'block-legacy-log.sh' hook rejects any file containing ''log'' as a string literal." |
| `1134-rib-4-extraction.md` | B | `ai/digests/rib.md` | "Two-level map with uint16 outer key beats string-prefix keys (no alloc per lookup) and composite struct keys (no struct hashing)." |
| `1135-bmp-6-looking-glass.md` | B | `ai/digests/rib.md` | "StaleLevel would have conflicted with GR/LLGR staleness semantics." |
| `1136-perf-1-rib-cache-layout.md` | A | `docs/architecture/pool-architecture.md` | "Mutex released before inner releases to prevent lock ordering issues." |
| `1137-bng-2-accounting-counters.md` | A | `rfc/short/rfc2869.md` | "Packet counts truncate at uint32 (no RFC-defined Gigapackets attribute); sessions with >4 billion packets will wrap silently." |
| `1138-rs-fastpath-0-umbrella.md` | C | `ai/rules/performance.md` | "Docker / colima / macOS scheduler variance can swing raw rps by 2x on M4 Max; treat same-day ze vs bird numbers as the only comparable snapshots, not " |
| `1139-iface-per-family-address.md` | C | `ai/rules/config.md` | "Rule: grep 'test/' for any field you remove or relocate in YANG." |
| `1140-l2tp-8-plugins.md` | A | `ai/digests/subscriber.md` | "Panic recovery + reject-and-continue keeps sessions alive across handler bugs." |
| `1142-enum-over-string-text-events.md` | C | `ai/rules/go-standards.md` | "Replacing with numeric handles would add complexity and indirection for no gain." |
| `1143-support.md` | A | `STAY` | "Adding a new support module requires one line in 'moduleRegistry' (modules.go)." |
| `1144-rpki-validation-store-not-retry.md` | B | `plan/learned/DESIGN-HISTORY.md` | "'dispatchValidation' retried up to 20 times at 50ms intervals (up to 1s total)." |
| `1146-isis-frr-interop-wire-bugs.md` | C | `ai/rules/interop-and-goal-validation.md` | "round-trips perfectly ze <-> ze and only fails against a different implementation." |
| `1147-needs-linux-qemu-runner.md` | C | `docs/architecture/testing/ci-format.md` | "The directive is INERT inside the VM — do not also gate it behind 'skip-os'." |
| `1148-filter-irr-startup-decoupling.md` | C | `ai/rules/plugins.md` | "A synchronous network call in 'OnConfigure' can exceed the engine stage barrier" |
| `1149-iface-absent-link-graceful.md` | B | `ai/digests/iface.md` | "- Only the **synchronous** phases needed the explicit skip: address reconcile" |
| `1150-rib-arch-1-store-vs-delta.md` | B | `docs/architecture/plugin/rib-storage-design.md` | "- **Keep the event-bus delta model; do not build a central per-protocol store.** Chosen" |
| `1151-rib-arch-3-inject-rfc5549.md` | A | `ai/digests/rib.md` | "- 'extractMPNextHopAddr' reads the storage's NORMALIZED 'OtherAttrs' format" |
| `1152-rib-arch-8-nlri-rewrite.md` | A | `ai/digests/bgp-reactor.md` | "- 'Len()' still counts only attribute ops; use 'HasModifications()' to detect a rewrite." |
| `1153-bare-go-test-drops-feature-tags.md` | C | `ai/rules/commands.md` | "- **A wrong known-failure is worse than an unlogged one.** It does not merely" |
| `1154-rib-arch-4-fib-ecmp-realtime.md` | B | `ai/digests/fib-programming.md` | "- **Design C over design B.** BGP arbitrates ONE best across peers, so it inserts one Loc-RIB" |
| `1156-rebase-learned-driver.md` | C | `ai/rules/git-safety.md` | "guards on 'has_unstaged_changes()', so ANY unstaged tracked change blocks the" |
| `1160-cli-compare-isolate-changes.md` | B | `ai/digests/cli-editor.md` | "- **'compare' prunes the YANG tree; 'format' serializes it** -- over pruning the rendered diff lines." |
| `1162-session-id-shared-marker.md` | C | `plan/learned/HOOK-FRICTION.md` | "- **The fallback was not a safe default, it was the bug.** A constant is stable, which" |
| `1163-dispatcher-trailing-token-swallow.md` | C | `ai/rules/commands.md` | "The natural fix -- reject leftover args when 'len(ArgDefs) == 0' -- is wrong and" |
| `1164-deferral-destination-spec-gate.md` | C | `ai/rules/planning.md` | "- The gate blocked the very commit that introduced it. Enforcement that is honest about existing debt is unlandable until the debt is paid, so budget " |
| `1166-rfc-clause-map-needs-producers.md` | C | `ai/rules/rfc-compliance.md` | "**The unit of coverage is 'clause -> producers -> tests per producer', never" |
| `1167-fixit-parser-fuzz-gaps.md` | C | `ai/rules/testing.md` | "- 'go test -fuzz=X ./pkg/...' fails with 'matches more than one package' whenever the tree under" |
| `1168-rfc-requirement-coverage.md` | C | `ai/rules/rfc-compliance.md` | "- **A negative test that trips a NEIGHBORING rule proves nothing.** The §5.3-4/5.3-5" |
| `1170-cli-dash-stdio.md` | C | `ai/rules/cli.md` | "- **The '.ci' functional harness cannot test the literal stdin fd for 'ze' commands.**" |
| `1174-cgo-race-requires-cgo.md` | C | `ai/rules/testing.md` | "- A global 'export CGO_ENABLED := 0' and 'go test -race' are mutually exclusive by construction;" |
| `1175-ze-suffix-test-isolation.md` | C | `ai/rules/commands.md` | "- **ze derives its config/DB dir from its own binary location, and only accepts a parent named" |
| `1176-cache-tree-atomic-stage-rename.md` | C | `ai/rules/platform-linux.md` | "- **Shell 'mv' SILENTLY NESTS without '-T'.** When two agents populate the same" |
| `1178-fixit-ldp-hello-read-loop.md` | B | `ai/digests/mpls-signaling.md` | "- **Mirror the ISIS dedicated-reader model, do not invent a new one.**" |
| `1179-fixit-local-asn-config-key.md` | C | `ai/digests/bgp-reactor.md` | "- The config tree delivers YANG leaf values as JSON STRINGS (Tree.values is" |
| `1182-fixit-firewall-concurrency-deadlock.md` | A | `ai/digests/firewall.md` | "silently DELETE the other owner's live drop rule while the registry believes it is installed." |
| `1183-fixit-fuzz-target-discovery.md` | C | `ai/rules/testing.md` | "Go's '-fuzz' value is an unanchored regexp (substring match)," |
| `1184-fixit-ping-monitor-cadence.md` | B | `ai/rules/go-standards.md` | "The spec floated a 'mutex-guarded shared in-flight map'. Instead the map is" |
| `1185-fixit-static-interface-nexthops.md` | A | `ai/digests/fib-programming.md` | "A zero 'netip.Addr' has 'Is4() == false' AND 'Is6() == false'; code that reads" |
| `1186-fixit-ddos-test-infra.md` | C | `ai/rules/testing.md` | "**Stderr assertions depend on the slog Text handler with color OFF.**" |
| `1190-fixit-recent-cache-buffer-reclaim.md` | B | `docs/architecture/update-cache.md` | "Rejected alternatives: a hard cap that sheds the oldest at 'Add' (can drop a live" |
| `1191-fixit-plugin-event-subscription.md` | A | `ai/digests/plugin-transport.md` | "**EventTypeID is GLOBAL, not per-namespace**" |
| `1194-fixit-private-asn-leak-deferred-nil-api-fail-open.md` | C | `ai/rules/testing.md` | "**Trap for the next agent:** 'reactorLogger()' warns are NOT capturable via" |
| `1195-fixit-supply-chain-hardening.md` | C | `ai/rules/repo-maintenance.md` | "Do not trust spec file paths for a 'fresh area': verify the package exists before" |
| `1197-fixit-agent-tooling-misleads.md` | C | `plan/learned/HOOK-FRICTION.md` | "'python3 scripts/dev/learned_index.py --help' REGENERATES 'ai/LEARNED-FULL-INDEX.md'" |
| `1198-fixit-perf-alloc-ci-gate.md` | A | `ai/rules/performance.md` | "**allocs/op is the stable column; B/op is not.**" |
| `1200-fixit-mgmt-listener-auth-guard.md` | A | `docs/architecture/hub-architecture.md` | "Non-loopback classification must fail closed on UNPARSEABLE hosts, not just on" |
| `1205-fixit-show-ping-serial-pacing.md` | C | `ai/rules/testing.md` | "**Fake-clock test trap: RTT timestamp races a post-inject clock advance.**" |
| `1217-fixit-doc-gate-and-refs.md` | C | `ai/rules/repo-maintenance.md` | "**Curated vs generated indexes**: only 'ai/LEARNED-FULL-INDEX.md' is generated" |
| `1218-fixit-pppoe-orphaned-tests.md` | C | `ai/rules/testing.md` | "An orphaned test suite is invisible because two independent failures cancel." |
| `1219-fixit-runner-kill-background.md` | A | `docs/architecture/testing/ci-format.md` | "**Fail closed on unknown name.** A '.ci'-supplied name that matches no tracked" |
| `1221-fixit-cli-view-registry.md` | A | `ai/digests/cli-editor.md` | "**A view-switch must release the OUTGOING view's context, or it leaks.**" |
| `1222-config-require-reload.md` | B | `ai/rules/cli.md` | "Removed '--no-reload' entirely and added '--reload' (opt-in), over keeping" |
| `1223-rfc-gate-regression-ratchets.md` | C | `ai/rules/evidence.md` | "**A ratchet creates an incentive; check where it points.**" |
| `1224-rfc7606-close-gaps.md` | A | `rfc/short/rfc7606.md` | "RFC 7606 grades its actions, and a wrapper must not flatten that grade." |
| `1228-rule-format-condensed-eager-load.md` | B | `ai/rules/repo-maintenance.md` | "'@import' in CLAUDE.md loads at launch, it does NOT defer." |
| `1229-ci-validation-github-not-codeberg.md` | C | `ai/rules/testing.md` | "blocks a test tier, the runner/forge is the fix, not weakening the test." |
| `1238-fixit-as4path-missing-on-rewrite.md` | A | `rfc/short/rfc6793.md` | "Emit AS4_PATH iff a non-mappable ASN is present AND the destination is non-AS4." |
| `1239-fixit-tombstone-ebgp-transitive.md` | B | `docs/architecture/wire/attributes.md` | "The MUST is honoured for ordinary EBGP peers (per-destination re-encode clears the bit)." |
| `1241-fixit-spec-hygiene-tooling.md` | B | `ai/rules/planning.md` | "learned->spec references are EXCLUDED from FAIL." |
| `1243-fixit-ci-accept-only-tests.md` | C | `docs/architecture/testing/ci-format.md` | "'contains=' truncates its value at the first colon; use 'pattern=' for JSON needles." |
| `1246-fixit-session-id-collision.md` | C | `plan/learned/HOOK-FRICTION.md` | "Editing the live hooks that gate your own Write/Edit" |
| `1247-fixit-static-per-route-isolation.md` | C | `ai/digests/fib-programming.md` | "'teardownRouteLocked' does not remove the kernel route" |
| `1248-fixit-config-file-positional-grammar.md` | A | `ai/rules/cli.md` | "Position 1 of 'ze' is now a closed set: YANG verbs, registered roots, and the" |
| `1250-shared-buffer-second-producer.md` | A | `ai/rules/go-standards.md` | "Go already serializes concurrent 'conn.Write'." |
| `1251-feature-gate-11-bmp-mrt.md` | B | `ai/rules/plugins.md` | "Verify byte-stability before the manifest change." |
| `1257-ci-timeout-option-ignored.md` | C | `docs/architecture/testing/ci-format.md` | "An option that is accepted and then discarded is worse than one that is" |
| `1258-qemu-gate-ran-a-stripped-daemon.md` | C | `ai/rules/platform-linux.md` | "A green run that did nothing looks exactly like a green run." |
| `1259-bgp-plugin-speaker.md` | C | `docs/architecture/testing/interop.md` | "Empirical byte-level evidence had to correct the code-reading twice, in both directions." |
| `1260-website-wiki-content-migration.md` | B | `STAY` | "Canonical 'main/docs/' pages are the preferred publication source over copying wiki text" |
| `1261-migrate-plugin-sleeps.md` | A | `ai/rules/testing.md` | "The engine quiesce barrier does NOT cover inbound processing; inbound assertions must use" |
| `1264-netlink-suite-recovery-2.md` | A | `ai/digests/firewall.md` | "Per-element nft timeout: set 'HasTimeout' on the parent set, not just the element." |
| `1265-ospf-only-daemon-external-plugins.md` | C | `ai/rules/evidence.md` | "A no-op fallback that satisfies a nil-check is worse than a nil" |
| `1266-fixit-peer-verdict-and-forward-rail.md` | A | `docs/architecture/testing/ci-format.md` | "A check peer that finishes its script must 'option=linger'" |
| `1267-netns-uid-drop-and-ddos-victim.md` | C | `docs/architecture/config/syntax.md` | "A repeated leaf-list statement must accumulate, not overwrite." |
| `1268-ordered-leaf-list-as-path-labels.md` | A | `docs/architecture/config/yang-config-design.md` | "- YANG authors: a leaf-list whose duplicates are meaningful (ordered sequence)" |
| `1269-hostload-contention-single-source.md` | C | `ai/rules/testing.md` | "- Two detectors that 'look the same' drift silently: the load-gate divergence" |
| `1272-module-path-rename.md` | C | `ai/rules/repo-maintenance.md` | "- **The protobuf descriptor fails silently.** It compiles. It only shows up when" |
| `1273-plan-template-lifecycle-split.md` | B | `ai/rules/planning.md` | "- **Split by lifecycle, not by topic.** 'plan/TEMPLATE.md' (328 lines) is" |
| `1276-fixit-ci-red-classes.md` | C | `ai/rules/testing.md` | "- **'skip-os:value=darwin' is not a capability declaration, and the distinction" |
| `1277-fixit-rfc6286-bgp-identifier.md` | A | `rfc/short/rfc6286.md` | "- **Gate the self-identifier rejection on the peer being internal**, per RFC 6286 Section" |
| `1278-fixit-sleeps-cli-harness.md` | C | `ai/rules/planning.md` | "- **A spec is a claim about the code, not a fact about it, and it rots silently.** The" |
| `1279-perf-harness-rot-and-tmp-reaper.md` | C | `ai/rules/repo-maintenance.md` | "- **'open(dest, 'w')' + 'check=False' is silent data loss.** It truncates before" |
| `1280-fixit-rs-community-strip-arity.md` | C | `ai/rules/performance.md` | "**A guard that rejects an input is also a limit on the work that input can" |
| `1281-spec-delegation-subagents.md` | A | `ai/rules/planning.md` | "a subagent has no LSP tool and cannot hold a dialogue with the user" |
| `1282-fixit-verify-plugin-stage-and-draft-tests.md` | C | `ai/rules/testing.md` | "**'Passes in isolation' is a starting point, not a conclusion.**" |
| `1283-fixit-ci-plugin-suite-nine.md` | C | `ai/rules/testing.md` | "**Generalize:** a passing test is not evidence that its stated mechanism ran. Here" |
| `1284-test-draft-workflow.md` | A | `ai/rules/testing.md` | "Worth repeating whenever a new gate is added: a skip that is merely believed is not" |
| `1286-plugin-metrics-never-registered.md` | A | `ai/rules/plugins.md` | "- A plugin author must not read the metrics struct during 'init()' or at the top" |
| `1287-rule-routing-severity-and-wip-cap.md` | C | `ai/rules/rule-format.md` | "**The digest keeps only the FIRST SENTENCE of the first prose paragraph of a" |
| `1288-commit-view-discovery-index.md` | C | `ai/rules/repo-maintenance.md` | "- **Fail-closed applies to a check that broke, not to a check that does not exist.**" |
| `1289-delegation-by-default.md` | B | `ai/rules/planning.md` | "- **Override the guard rather than fight it.** The guard self-exempts on 'unless" |
| `1290-mp-reach-next-hop-invisible-in-show.md` | B | `ai/digests/rib.md` | "- **Fall back to MP_REACH in the renderer, mirroring the forward path.** The" |
| `1291-pipe-gate-producer-and-shapes.md` | C | `ai/rules/repo-maintenance.md` | "- **Rescoping a guard is two changes, and the loosening half is the invisible" |
| `1292-web-expectations-sampled-once.md` | A | `docs/functional-tests.md` | "- **Positive expectations poll; negative expectations do not.** 'element:id'," |
| `1293-zefs-truncation-sigbus.md` | A | `docs/architecture/zefs-format.md` | "'pkg/zefs' now has a structural invariant: no non-test file in the package may" |
| `1294-shared-store-contention-and-gate-residuals.md` | C | `ai/rules/testing.md` | "A test for a gate is worthless if the ungated path returns the same answer in" |
| `1295-rfcgate-1-extraction.md` | B | `ai/rules/rfc-compliance.md` | "A skeleton generator that emits a file its own parser refuses bricks the ENTIRE gate, not just that RFC." |
| `1297-rfcgate-3-audit-teeth.md` | C | `ai/rules/rfc-compliance.md` | "A test suite over dead code reads exactly like coverage." |
| `1298-mcp2026-0-umbrella.md` | B | `docs/architecture/mcp/overview.md` | "The goal was full conformance as a **clean cutover** -- the older revisions" |
| `1299-mcp2026-1-stateless-core.md` | A | `docs/architecture/mcp/overview.md` | "Validation order is now load-bearing and fixed: header ('-32020')" |
| `1300-mcp2026-2-mrtr.md` | C | `docs/architecture/mcp/overview.md` | "The feature was reachable in code and unreachable through its own interface." |
| `1301-mcp2026-3-tasks-extension.md` | A | `docs/architecture/mcp/overview.md` | "A deadline that forces an entry terminal must also stop later writes." |
| `1302-mcp2026-4-caching-apps.md` | C | `docs/architecture/mcp/overview.md` | "A guard that derives from a hand-written list guards nothing." |
| `1303-rfcgate-4-ledger.md` | C | `ai/rules/rfc-compliance.md` | "A stated rejection rule was a six-phrase blacklist." |
| `1305-simplified-technical-english.md` | C | `ai/rules/writing.md` | "A checker that documents its own escape hatch will exempt itself." |
| `1307-rfc-evidence-tier-vacuity.md` | A | `ai/rules/testing.md` | "'all_suites' declares the set. 'run_suite' performs it. A suite needs BOTH." |
| `1311-rfc-compliance-docs.md` | C | `ai/rules/evidence.md` | "A regex over 'rfc/short/*.md' overcounts annotations." |
| `1312-bgp-update-withdraw-order.md` | C | `ai/rules/protocol.md` | "Ze applied every insert before every remove. The withdrawal therefore deleted the route" |
| `1317-wire-edit-1-base-index.md` | A | `docs/architecture/wire/attributes.md` | "Any future edit that calls 'Attrs' earlier in 'enforceRFC7606' reintroduces the" |
| `1323-rule-coverage-always-on-exclusion.md` | C | `plan/learned/HOOK-FRICTION.md` | "Never pin a count from this corpus in prose." |
| `1324-poll-loop-gate.md` | B | `ai/rules/commands.md` | "Gate on whether a loop CAN END, not on whether the wait is justified." |
| `1325-review-loop-bounded-scope.md` | B | `ai/rules/planning.md` | "Bounded the SCOPE, not the number of passes." |
| `1326-deferral-shard-outlives-its-spec.md` | B | `ai/rules/planning.md` | "The live row wins, not the deletion." |
| `1327-enabled-gate-discards-service-settings.md` | C | `ai/rules/config.md` | "When an extractor returns '(Config, ok)', ask what a caller does with 'ok=false'." |
| `1328-rule-corpus-merge-and-line-ref-strip.md` | C | `ai/rules/repo-maintenance.md` | "A merge agent can silently drop a source and still report success." |
