# Handover 64: ze always starts, looking glass HTTP/HTTPS, fleet-1 approval, approval carry-over

Sessions of 2026-11-23 to 2026-11-26. Thomas's words below are voice-transcribed.
Continues plan/handover/63-fleet-release-specs-and-listener-certificate.md, which
stays until the fleet set closes; its open items are carried here (rows 9 to 12).

## Landed

| Commit | What |
|--------|------|
| 2984d3eaa5 | Web/LG listener drops a removed certificate name on reload |
| 7b6cd880d6, fbe3d27b87 | fleet-ui-5 renamed to the alarm list; fleet-0 umbrella YANG row |
| e6eda5e8d8, 80f69b9f18 | fleet-1 design revised, judged, approved by Thomas ("approved, C-11 (a)", digest e1897d459b10) |
| b243c9221b, a5be5a3361 | Looking glass serves HTTP and HTTPS at once: tls per server entry, ze.looking-glass.tls removed (rulings in plan/records/ruling-looking-glass-http-and-https.md) |
| 4f5d63d02a | ./le spec design carry: an approval carries to text that only adds his own decisions (plan/records/ruling-design-approval-carry-over.md) |
| 07dd975ccd, c8d4c7130b | fleet-1: two derived decision rows; paths moved under bus hub (commit 34f8c2b490); certificate page section rewritten |
| fc01140865, d057ae65c0 | Ze always starts (plan/records/ruling-ze-always-starts.md): web/LG certificate that cannot be presented, LG endpoint tls true with no store, unparseable ze.web.listen / ze.mcp.listen / ze.looking-glass.listen, LG next-hop ranges, REST/gRPC listen and build, MCP validation: each leaves only its service or endpoint down, reported under a doctor code; reload still refused |

## Outstanding, every item

| # | Item | State and next step |
|---|------|---------------------|
| 1 | Ze always starts: checkMgmtListeners exit (cmd/ze/hub/main.go runYANGConfig) | Not started. Per-listener: needs internal/core/network Exposure.Refusals to say which addresses breach, then filter webAddrs, mcpAddrs, apiCfg.REST/GRPC and skip the gNMI build. mgmt_guard*_test.go and any .ci expecting the boot refusal need ze-test-judge |
| 2 | Ze always starts: bngRegister exit | Not started. registerBNGSubsystems (cmd/ze/hub/register_l2tp.go) returns on the first error; make it return per-subsystem problems so L2TP and PPPoE stay down independently |
| 3 | Answer 1a: an undefined certificate name (any ze:reference under one service's block) leaves that block out at start | Designed, no code. Design: each service's register.go registers its block path with config; a failure under a registered block (longest match) is scoped; RegisterTreeValidator gains an optional owning block. A start-only third caller of the loadConfig body deletes scoped blocks from the in-memory tree, re-runs the walk (refuse with the original error if it still fails), returns the blocks left out; any unscoped failure refuses as today. Never rewrite the stored file: CHECK FIRST whether applyEvolutions (main_evolve.go) serialises loadResult.Tree back. Report through reportServiceNotStarted. validate/commit/reload stay strict. Functional draft: ze start with environment web certificate naming an undefined entry starts, web down, doctor names it |
| 4 | Answer 2a: a kernel-only feature on a VPP daemon must not stop start | Decision row in docs/architecture/core-design.md (2026-11-26). Code belongs to the dataplane guard's owner (plan/release/fixes/spec-dataplane-compatibility-guard.md): refuseDataPlaneMismatches (internal/component/config/dataplane_verdict.go) called from loadConfig (loader.go). It needs a start-only form; validate/commit/reload keep refusing. Tell that spec's owner; amend its spec if no session holds it |
| 5 | test/ui/start-serves-around-unreadable-listen.ci does not discriminate its web half | Removing webEnabled = false under webListenProblem leaves it green: the test daemon has no database store, so web never starts. Give the fixture a store (ze init) and re-run the mutation |
| 6 | Docs for item 1-3 and the six converted exits | tls-listeners.md and docs/guide pages for web, MCP, API: state that each service stays down and is reported (doctor-and-health-checks.md already has the table) |
| 7 | fleet-1 design approval | Stale since 07dd975ccd/c8d4c7130b (Task and AC paths changed, so carry does not apply). Judge refused digest db019235da29 on the start/reload asymmetry; Thomas then ruled ze always starts, which makes AC-41 (managed listener unbound, ze runs) the rule. Next: update fleet-1's tls-listeners rows to the page as it now reads (fc01140865 changed it), re-run ze-design-judge, then Thomas runs ./le spec design approve. Implementation in a fresh session after that |
| 8 | Lint | Never ran clean on cmd/ze/hub, internal/component/lg/config/web for b243c9221b..d057ae65c0: other sessions' edits kept changing its inputs. Pre-existing findings in handler_rfc8522*.go, server_test.go (a body comparison), main_reload_managed_test.go are not ours |
| 9 | spec-fleet-10-drift-hash-chain.md | Another session's uncommitted rewrite still names the old stem spec-fleet-ui-5-tasks-alarms-notifications; it must use spec-fleet-ui-5-alarm-list when it lands |
| 10 | Remaining fleet design choices for Thomas | fleet-ui-8 O-1..O-3 and its display rules; fleet-10 departure from "full config on change"; presented with each spec's design |
| 11 | website/compare/nos.md | Still lists ze.looking-glass.tls among env keys; needs Thomas's OK (website review) |
| 12 | Reds from other work, seen not fixed | cmd/ze/hub TestConfiguredPluginConformance*, TestCheckKeyFile*, MCP TLS tests (~/.zt group-writable), TestEveryCommandOnEverySurface; ui doctor-aaa-unreachable and doctor-kernelcap-denied (doctor-ppp-original-unit-attachment, /dev/ppp), doctor-config-claims (ze-stripped missing); internal/le/hookruntime TestGuardVerdictsOfHeadStand and ./le ai hooks unit hook-source-drift (another session's hookruntime edits) |
