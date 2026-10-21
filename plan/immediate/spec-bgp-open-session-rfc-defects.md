# Spec: bgp-open-session-rfc-defects

| Field | Value |
|-------|-------|
| Status | in-progress |
| Scope | protocol |
| Depends | - |
| Phase | 7/8 |
| Handoff | - |
| Updated | 2026-10-14 |

Recovery after compaction: `.claude/rules/post-compaction.md`.

## Task

Four defects in how Ze builds and reports its BGP OPEN exchange, found by the
RFC re-read and the softver interop work of
`spec-rfc-requirement-quote-hand-backfill`. Two are RFC
defects: a peer misframes Ze's OPEN, and a per-peer override gives peers
different BGP Identifiers. Two are Ze behaviour defects that no RFC sentence
states, each recorded by its journal row: a changed capability does not reach
the wire after a reload, and a peer refusing Ze's OPEN leaves no line at the
default log level. Each was confirmed open in HEAD on 2026-09-27.

## Defects

| ID | Source | Verbatim quote | Producer | What Ze does wrong | Audit verdict / record |
|----|--------|----------------|----------|--------------------|------------------------|
| D1 | RFC 9072 Section 2 (RFC9072-2-1) | "The rules for encoding Optional Parameters are unchanged with respect to those given in [RFC4271], except that the Parameter Length field is extended to be a two-octet unsigned integer." | `internal/component/bgp/reactor/session_negotiate.go::buildOptionalParams` | The one-octet form is chosen while `capTotal <= 255`, so a capability total of 254 or 255 yields 256 or 257 octets of Optional Parameters with `ExtendedParams` false; `Open.WriteTo` (`internal/component/bgp/message/open.go`) then writes the extended envelope (optLen above 255) around a parameter with a one-octet length, which a conformant receiver misframes. The boundary is 2 plus capTotal | no verdict; read, not reproduced |
| D4 | RFC 6286 Section 2.1 (RFC6286-2.1-1) | "The value of the BGP Identifier for a BGP speaker is determined on startup and is the same for every local interface and every BGP peer." | `internal/component/bgp/reactor/config.go::parsePeerSettings` ("Router ID from session > router-id (peer-level overrides global)") | A peer-level `router-id` gives that peer a BGP Identifier different from the others'; `docs/features/rfc-status.md` advertises the override as supported. The sentence carries no BCP 14 keyword and sits in the Section 2.1 definition; the row labels it MUST | weak (TestParsePeerFromTreeInvalid asserts only non-zero); owner, 2026-10-13: kept as a deviation (Q2) |

### Ze behaviour defects (no RFC sentence)

Neither row is an RFC defect: no RFC sentence states the behaviour, so neither
carries an RFC tag, a discrimination record or a conformance count. Each is
owed because an operator meets it, and its journal row is its record.

| ID | Journal row | Quote | Producer | What Ze does wrong | Record |
|----|-------------|-------|----------|--------------------|--------|
| D2 | `plan/journal/unwired-feature.md`, 2026-09-27 | none: a reload defect | `sdk.Plugin.Run` capability declaration at stage 3; `internal/component/bgp/plugins/softver/softver.go` `OnConfigure`; same shape in `hostname`, `role`, `llnh`, `gr` | A SIGHUP reload that changes a plugin-declared capability (softver `encoding`) bounces the session, but the new OPEN keeps the startup capability (code reading; wire not captured) | journal row, Open |
| D3 | `plan/journal/silent-fall-through.md`, 2026-09-27 | none: an observability defect | `internal/component/bgp/reactor/session_handlers.go::Session.handleNotification`, the reconnect loop in `peer_run.go` | A NOTIFICATION received in answer to Ze's OPEN is logged only at DEBUG, so a peer that refuses the OPEN never comes up and no line says why (seen in the `frr-software-version` interop scenario) | journal row, Open |

## What a fix must prove

| Obligation | Detail |
|------------|--------|
| Failing test first | D1: a unit test at capTotal 253, 254, 255 and 256 asserting the envelope and the per-parameter length agree, on the one packer. D2: a test reloading a changed softver encoding and asserting the refusal. D3: a test asserting the WARN line |
| Both polarities | D1: 253 stays one-octet with no envelope; 254 upward uses the RFC 9072 form throughout. D2: a reload that changes, adds or removes a plugin-declared capability is refused; a reload that leaves every declaration unchanged, only removes a peer, or adds a peer that carries only all-peer declarations is accepted |
| Discrimination | `./le rfc discriminate-record` for each new RFC9072-2-1 (and RFC9072-2-7) tag, and a fresh record for every existing record or tagged unit Q4 stales: the three tagged reactor units moved onto the packer, the four tagged bmp units whose bodies change with the `fabricateLocRIBOpen` error return, and the seven records whose producer body changes (`buildOpen`, `fabricateLocRIBOpen`). The full list is "Discrimination records Q4 stales" under "RFC-tagged tests that change". None for D2 and D3, which carry no RFC tag |
| Real entry point | `.ci` tests for D2 (reload refused) and D3 (peer refuses OPEN) |
| Interop | D1 against FRR with a capability set sized to 254 octets; D3 through `frr-software-version`. D2 owes none: the refusal changes no wire behaviour |

## Owner decisions

| Row | Question | Owner answer |
|-----|----------|--------------|
| D1 | One Optional Parameters packer, or the threshold fixed in place in each packer? (Q4) | **Owner, 2026-10-13: "Consolidate".** Read as one packer for every copy (main thread, 2026-10-13): it serves the reactor, the chaos peer, the perf sender, the BMP Loc-RIB OPEN, `ze analyze`'s OPEN and the test peer's active-role OPEN, and the six copies are deleted. Q5: **owner, 2026-10-14: "a"**: one packer takes typed parameters and serves all seven writers, the test peer's passive-role `encodeOpen` included |
| D2 | How does a reload that changes a plugin-declared capability behave? (Q1) | **Owner, 2026-10-13: "b".** The reload is refused, naming the plugin, the peer and the capability, and saying that ze must be restarted to apply it |
| D4 | Remove the per-peer `router-id` override, or keep it as an owner-approved deviation from RFC 6286 Section 2.1? (Q2); is the live global `router-id` reload part of the ruling? (Q3) | Q2: **owner, 2026-10-13: "we should have a global router-id which can be overwritten per peer which is over what the RFC allows"** (deviation). Q3: **owner, 2026-10-13: "Out of scope"**: the live global `router-id` reload stays as it ships |

### Owner questions (design phase, 2026-10-13)

Each question asks which way to fix, never whether. Q1 to Q6 are answered and
the design below follows them.

| # | Row | Question | Answer | Consequence |
|---|-----|----------|--------|-------------|
| Q1 | D2 | Re-declare on apply, refuse the reload, or restart the plugin process? | **Owner, 2026-10-13: "b"**: config-verify refuses a reload that changes a plugin-declared capability, including a peer added on reload that would carry one | AC-2 to AC-2e are the refusal and its acceptance polarity. No RPC, injector or engine change: each capability plugin hands the SDK its capability extractor once, and the SDK runs it at stage 2 and again at config-verify and refuses a changed set; no plugin writes a verify handler for it (see "D2 refusal design"). New restriction against today: a family edit on a `gr` peer with no `graceful-restart family` list, and adding a peer that carries a plugin-declared capability, now need a daemon restart (Known Limitations) |
| Q2 | D4 | Remove the peer- and group-level `session router-id` leaf (one BGP Identifier per speaker), or keep it as an owner-approved deviation | **Answered by the owner, 2026-10-13: "we should have a global router-id which can be overwritten per peer which is over what the RFC allows". The leaf stays as an owner-approved deviation from RFC 6286 Section 2.1: AC-6 documents it, RFC6286-2.1-1 leaves the conformance count** | No config or test-corpus change; RFC6286-2.1-1 becomes `{partial}`/deviation in `rfc/short/rfc6286.md` and leaves the conformance count; AC-4 brings RFC9072-2-1 to `enforced` and RFC6286-2.1-1 carries the deviation record |
| Q3 | D4 | The same Section 2.1 sentence says the identifier is "determined on startup". Ze applies a changed global `router-id` on SIGHUP and restarts sessions (`bgp-reload-global-router-id`, `test/reload/reload-global-router-id.ci`). Is that live change part of the D4 ruling, or out of scope? | **Owner, 2026-10-13: "Out of scope"** | The live reload stays as it ships: `reactorAPIAdapter.applyGlobalsJournaled` (`internal/component/bgp/reactor/reload_globals.go`) stores the new `Globals.RouterID` in `r.config.RouterID`, and the peer diff that follows restarts every peer whose effective Identifier (`PeerSettings.RouterID`) changed, so each restarted peer gets the new Identifier in a new OPEN. No AC, test or code here touches it; recorded under Known Limitations |
| Q4 | D1 | Fix the threshold in place in the packers, or consolidate the Optional Parameters packers into one producer? | **Owner, 2026-10-13: "Consolidate"** (asked against a list of four; the main thread reads it as every copy, 2026-10-13) | One packer, `capability.PackOptionalParams` in `internal/core/bgp/capability` (see "D1 consolidation design"); the six copies are deleted, and under Q5 "a" `encodeOpen`'s own framing too. The D1 threshold fix and the 253/254/255/256 boundary tests stay, now on the one packer. The RFC-tagged units the consolidation changes move under the owner's approval of 2026-10-13 ("approve all RFC-tagged test changes"), each with a fresh discrimination record; the list is under "RFC-tagged tests that change" |
| Q6 | D1 (AC-1r tests) | Found 2026-10-14 by review round 5: neither `.ci` can discriminate through the behaviour the recorded families drive without forcing replay order (rr sends its End-of-RIB only after a non-empty replay; rs's forward filter is bypassed by `rs-fast-path` and by the peer-up replay). Which observable: (a) a read-only `families` field on each row of `show rr peers` and `show bgp rs peers`, an additive CLI output change (see "Recorded-families field"); or (b) the order-forced behaviour tests (rr: a second source session whose route is in adj-rib-in before the receiver's peer-up, the replayed UPDATE before the ipv6 End-of-RIB, `manual-eor` with its dropped-marker window; rs: no `rs-fast-path`, the source UPDATE held until the receiver is Established) | **Owner, 2026-10-14: "a"**: the read-only `families` field is added to both commands' rows (designed first under the main thread's delegation as the simpler test that is still a real operator path) | The two `.ci` rows, the Wiring rows, AC-1s, the two peer-status unit tests, the observer fixture and the docs rows follow (a). If the owner chooses (b), the round-5 review's B1, I1 and I2 fixes apply instead and the field is not added |
| Q5 | D1 | Found 2026-10-13 while verifying ISSUE-2: the test peer's passive-role `encodeOpen` (`internal/test/peer/open.go`) is a seventh Optional Parameters writer. It already frames RFC 9072 correctly (extended when the sum of 2 + value length over its parameters exceeds 255, refused above the 4096-octet OPEN), but it writes a LIST of typed parameters, including non-capability parameters mirrored from ze's OPEN as raw bytes, where `PackOptionalParams` writes one type-2 parameter from a capability list. Which way: (a) the one packer takes typed parameters, so all seven share it; or (b) `encodeOpen` stays, with the reason (it reproduces ze's parameter layout, which the capability packer has no reason to accept) written above it | **Owner, 2026-10-14: "a"**: one packer takes typed parameters and serves all seven writers, `encodeOpen` included | `capability.PackOptionalParams` takes a list of typed parameters (see "D1 consolidation design"); `encodeOpen`'s own framing is deleted and it calls the packer; its parameter layout (order, several type-2 parameters, raw non-capability parameters) is kept as the list it hands the packer. AC-1q names seven writers and the no-layering grep has no exception; AC-1p is the `encodeOpen` row |

### D2 refusal design (Q1 = b, 2026-10-13)

**Where declarations are compared on reload today: nowhere.** Read at the producers on 2026-10-13:

| Producer | What it does | Why it cannot refuse |
|----------|--------------|----------------------|
| `sdk.Plugin.SetCapabilities` (`pkg/plugin/sdk/sdk_callbacks.go`) | stores the set in `Plugin.capabilities`; `Plugin.Run` sends it once as `DeclareCapabilitiesInput` at stage 3 (`pkg/plugin/sdk/sdk.go`) | the stored set is the startup declaration and is never recomputed |
| `CapabilityInjector.AddPluginCapabilities` / `RemovePluginCapabilities` (`internal/component/plugin/registration.go`) | holds each plugin's set for `buildOpen`; removal only on plugin restart | the engine does not know a plugin's capability semantics, so it cannot compute the candidate set |
| `peerSettingsSwapPlan` (`internal/component/bgp/reactor/peer_settings_apply.go`), `Session.negotiationOutcomeUnchanged` (`peer_settings_negotiation.go`) | compare the reactor's own settings (`CapabilityConfigJSON`) and an OPEN built from the stage-3 store | they decide restart against swap; they see no plugin re-computation |
| `extractSoftverCapabilities`, `extractHostnameCapabilities`, `extractRoleCapabilities`, `extractLLNHCapabilities`, `extractGRCapabilities` and `extractLLGRCapabilities` | compute each plugin's declaration from a `bgp` config section; every entry's `Peers` selector is `configjson.CapabilitySelector` (the peer's name, or `group:<name>` for a dynamic group's template) | called only from `OnConfigure` (stage 2) |
| `capabilityDecls` (`internal/plugins/exabgp/bridgeplugin/internal.go`), called by `bridgeRunner.applyConfig` | route-refresh (code 2) and add-path (code 69) from the bridge container under the `exabgp` root (`configRoot = "exabgp"`, `bridgeplugin/config.go`); the add-path payload is built from the bridge's families (`bridge.EncodeAddPathHex(cfg.Families, cfg.AddPath)`); no `Peers` selector, so both apply to every peer | `applyConfig` returns nil once the bridge started ("family/capability changes need a plugin restart"), so a reload that changes them is accepted and silently not applied: the D2 class in a sixth plugin |

The external `internal/plugins/exabgp/main_sdk.go` also calls `SetCapabilities`,
from command-line flags before `Run`, so no reload can change its set. It
registers `WantsConfig: []string{"bgp"}` and no `OnConfigure`, and stage 2 is
still served for it (`sdk.go`, `serveOne(ctx, callbackConfigure, p.handleConfigure)`),
so it can register a constant extractor over the sections it already receives
(read 2026-10-14, review round 2 ISSUE-2).

`OnConfigVerify` already delivers the full candidate sections to a plugin
inside the reload transaction, before any apply ("The handler receives the full
candidate config sections", `sdk_callbacks.go`). `llnh`
(`refuseLinkLocalCapabilityWithoutAddressSections`) and `gr`
(`refuseUncarriedGRSections`) refuse there today; `softver`, `hostname`,
`role` and the bridge register no capability check there. Only the plugin can
compute its candidate declaration (`CapabilityInjector` and the engine hold
only the declared set), so the plugin supplies the computation. The SDK owns
the comparison, so no plugin can skip it and stage 2 and verify cannot compute
the declaration two ways.

**One SDK registration, no per-plugin verify handler, one declaration path.**
The plugin hands the SDK its capability extractor once, before `Run`.
`SetCapabilities` is deleted: every plugin that declares capabilities, the
external `main_sdk.go` included, registers an extractor, so the SDK has one
declaration path and needs no rule keeping two paths exclusive.

1. Stage 2, inside `Run` only: after the stage-2 `serveOne` returns, `Run` runs
   the extractor on the sections that stage-2 configure delivered and stores the
   result as the stage-3 declaration; an extractor error fails stage 2 the way
   an `OnConfigure` error does. `handleConfigure` itself never runs the
   extractor.
2. After stage 3 the stored declaration is never rewritten. The engine
   redelivers `configure` at runtime after a refused removal
   (`internal/component/plugin/server/startup_removal.go`, `conn.SendConfigure`
   with the committed sections), and the SDK routes it to `handleConfigure`
   (`sdk_callbacks.go`, `callbackConfigure`), which reruns `OnConfigure`. Were the
   extractor run there, that redelivery would overwrite the baseline with the
   committed config's declaration, which lacks peers removed by accepted
   reloads, while the injector still holds their declarations; a later re-add of
   such a peer would then be refused. Running it only on the `Run` path closes
   that.
3. Config-verify: the SDK composes the plugin's own verify handler with the
   comparison at `Run`, whatever order `OnConfigVerify` and the extractor were
   registered in. `OnConfigVerify` overwrites `p.callbacks[callbackConfigVerify]`
   (`sdk_callbacks.go`), so installing the comparison as that callback at
   registration would be lost when `llnh` (`OnConfigVerify` in its run function)
   or `gr` registers its handler afterwards. The plugin's own handler answers
   first (the `llnh` and `gr` refusals keep answering first), then the extractor
   runs on the candidate sections and the two declarations are compared under
   the verdict table below.

| Piece | Home | What it does |
|-------|------|--------------|
| `Plugin.DeclareCapabilitiesFrom` (new method; final name set at implementation) | `pkg/plugin/sdk/sdk_callbacks.go`, where `SetCapabilities` was | Registered before `Run`. Carries the extractor and the plugin's capability leaf names by code, and no config root: the SDK hands the extractor the sections it was given, as `OnConfigure` does, and the registration's `WantsConfig` stays the one declaration of the roots (a second root here could name one the engine never sends a candidate for, `reload.go` dispatching verify only to plugins whose `WantsConfigRoots` changed, and the comparison would silently never run). Steps 1 to 3 above: the stage-2 run inside `Run`, no rewrite after stage 3, the composed config-verify. No RPC, engine or injector change |
| The extractor | each plugin | Input: the config sections the plugin received; it filters `section.Root` itself, as every extractor's `OnConfigure` and the bridge (`internal.go`, `s.Root != configRoot`) already do. Output: the declaration list and the candidate's peer selectors, or an error. A pure function of the sections (A-6) |
| `configjson.CapabilitySelectors` (new function) | `internal/component/bgp/configjson/traverse.go` | Every peer and dynamic-group selector of a `bgp` tree, from the same `ForEachPeer` and `CapabilitySelector` the extractors use, so the two sets cannot disagree; the `bgp`-rooted extractors return it as their selectors |
| Callers | `softver`, `hostname`, `role`, `llnh`, `gr` (`extractGRCapabilities` and `extractLLGRCapabilities`), the exabgp bridge, the external `main_sdk.go` | each deletes its `SetCapabilities` call and registers its extractor; `OnConfigure` keeps the plugin's other config work (for example `role`'s `filterPeerConfigs`). The bridge's extractor is `parseConfig` then `capabilityDecls(cfg bridgeConfig)`, over the sections whose root is `exabgp`, with no selectors. `main_sdk.go` registers a constant extractor: it ignores the `bgp` sections and returns the flag-built set with no selectors, so its declaration cannot change on reload and the comparison always accepts |
| Leaf names | each plugin, beside its capability code constant | The `capability` leaf the operator wrote (`software-version`, `hostname`, `role`, the llnh and gr leaves, the bridge's `route-refresh` and `add-path`), so the refusal names configuration, not a code number |
| `SetCapabilities` | deleted (no layering) | its last caller, `main_sdk.go`, registers a constant extractor instead. The earlier reason for keeping it ("that plugin reads no config root") was false: `main_sdk.go` registers `WantsConfig: []string{"bgp"}` (review round 2, ISSUE-2). With one declaration path there is no `Run` refusal of a plugin using both |

| Candidate against the stage-3 declaration, per selector | Verdict |
|--------------------------------------------------------|---------|
| Same codes with the same encoding and payload | Accept |
| A code whose payload or encoding differs (softver `encoding`, a hostname, a role, a GR restart time or family list) | Refuse |
| A code declared and absent from the candidate while the selector is still a candidate peer (capability removed from a live peer) | Refuse: the injector would keep sending it |
| A code in the candidate that was never declared (capability added, or a peer added on reload that carries one) | Refuse: the injector holds nothing for it |
| A declared selector that is no longer a candidate peer (peer removed) | Accept: the peer stops and sends no OPEN |
| An all-peer declaration (empty `Peers`, the exabgp bridge) | Compared as one selector, "every peer": a changed, added or removed code is refused and the refusal names "every peer". Adding or removing a peer leaves it unchanged and is accepted, because `CapabilityInjector.AddPluginCapabilities` stores an empty-`Peers` entry in `globalCaps`, which every peer's OPEN reads, a peer added on reload included (`internal/component/plugin/registration.go`) |

**New restriction against today.** Today a reload that edits a peer's
capability block is accepted: the peer restarts, and its new OPEN carries the
stale plugin store. After this spec the reloads below, accepted today, are
refused and need a daemon restart (the bridge row is accepted today and silently
not applied):

| Reload | Why it is refused now |
|--------|-----------------------|
| A session family added or removed on a `gr` peer whose `graceful-restart` container has no `family` list | `grAdvertisedFamilies` (`gr_capability.go`) returns the session families when the list is absent, so the Graceful Restart payload changes |
| A peer added that carries a `softver`, `hostname`, `role`, `llnh` or `gr` capability | the injector holds no declaration for the new selector |
| A family edit under the exabgp bridge while its `add-path` is set | the bridge's add-path payload is built from the bridge's families (`bridge.EncodeAddPathHex(cfg.Families, cfg.AddPath)`), so the payload changes; the refusal names `add-path` |

With an explicit `graceful-restart family` list, the GR payload is that list
(`grAdvertisedFamilies` returns it), so adding a session family leaves the
payload unchanged and the reload is accepted. Removing a session family the list
names is refused already, by `refuseUncarriedGRFamilies`, before this check. Both
restrictions are correct under Q1 = b (the alternative is a stale OPEN), and both
reduce what a reload can do; Known Limitations and the owner report state them.

Refusal text (`ai/rules/user-facing-errors.md`: problem first, why, then the
command on its own line). One line per changed peer and capability, sorted by
peer, then the fix once. Lines 3 to 6 are one route per line, the same shape
`spec-plugin-reload-applies-or-refuses` AC-8 prints, and they come from one
helper both specs call, `sdk.RestartSteps() string` in
`pkg/plugin/sdk/restart_text.go`; whichever spec lands first creates it, the
other calls it (aligned 2026-10-14 with that spec's design review round 3,
ISSUE-2 and ISSUE-3, decided by the main thread under the owner's delegation:
the earlier line 5 joined the non-systemd and appliance routes, and no test
pinned it). Sample for one change:

| Line | Text |
|------|------|
| 1 | `peer 192.0.2.1: capability software-version cannot change while ze runs (plugin bgp-softver).` |
| 2 | `The plugin sends its capabilities only when ze starts, so ze would keep sending the old value in its OPEN to this peer.` |
| 3 | `To apply the change, stop the ze daemon and start it again:` |
| 4 | `  under systemd: systemctl restart ze` |
| 5 | `  without systemd: ze signal stop, then start ze the way it was started (ze start, or ze start <config-file>)` |
| 6 | `  on the ze appliance: request reboot in the ze CLI, or ze signal reboot from a shell` |

For an all-peer declaration, line 1 reads `every peer:` in place of the peer.

The refusal is reached through the reload transaction's verify, which a SIGHUP
and an editor commit share (the `llnh` comment in `runLLNHPlugin` records the
same path).

**Why the text never names `ze signal restart`** (read 2026-10-13). That command
stops ze and does not bring it back: `cmd/ze/hub/service_ssh.go` wires restart as
`stopForRestart()` plus `SignalShutdownRequested()`, and `hub.Run`
(`cmd/ze/hub/main.go`) then prints "Ze stopped." and returns 0. The unit ze
generates says `Restart=on-failure` (`internal/plugins/systemd/unit.go`), so
systemd does not restart a clean exit, and `docs/guide/appliance.md` says the
appliance's gokrazy init restarts ze only on a non-zero status. The commands
named instead:

| Where ze runs | Command | Producer read |
|---------------|---------|---------------|
| systemd | `systemctl restart ze` | `serviceName = "ze.service"` (`internal/plugins/systemd/unit.go`); systemd stops with SIGTERM, which `hub.Run` takes on `stopCh` (`signal.Notify(stopCh, syscall.SIGINT, syscall.SIGTERM)`) and shuts down as `ze signal stop` does, then starts ze again |
| no supervisor | `ze signal stop`, then ze started again the way it was (`ze start`, or `ze start <config-file>`) | `docs/guide/lifecycle.md` action table and start line; `cmd/ze/ze_core_start.go` usage `ze start [<config-file>] [options]`: the config file is optional and the default is the live store |
| the appliance | `request reboot` in the ze CLI, or `ze signal reboot` from a shell | `request reboot` is `container reboot` under `container request` in `internal/core/ipc/yang/ze-system-cmd.yang` (`ze-plugin:system-daemon-reboot`), listed by `internal/component/plugin/server/system.go` and served by `handleDaemonReboot`; `ze signal reboot` is the exec verb of `internal/plugins/signal/main.go`. A bare `reboot` is not a ze CLI command: it is an SSH exec lifecycle verb that `execMiddleware` (`internal/component/ssh/ssh.go`) handles only for a non-interactive command ("Interactive sessions (no command) pass through"), and the CLI dispatcher registers no lifecycle verb. `request reboot` reaches `apiSrv.SetRebootFunc` (`cmd/ze/hub/infra_setup.go`) and `ze signal reboot` reaches `sshSrv.SetRebootFunc` (`cmd/ze/hub/service_ssh.go`, which also writes the GR marker through `stopForRestart`); both set `rebootRequested`, and `hub.Run` (`cmd/ze/hub/main.go`) calls `reboot.Reboot()` after the stop. Both routes call the same `stopForRestart` closure (`infra_setup.go`: `infra.WriteGRMarker(...)` then `r.StopForRestart()`, which is `r.stop(false)` in `reactor.go`, no NOTIFICATION; handed to the SSH wiring as `StopForRestart`), so both write the Graceful Restart marker and stop without a NOTIFICATION (read 2026-10-14, round-3 review ISSUE-3) |

Trade-off stated: `systemctl restart ze` and `ze signal stop` end each session
with a normal teardown, so peers withdraw Ze's routes until it is back.
The appliance routes do not: `request reboot` and `ze signal reboot` write the
Graceful Restart marker and stop without a NOTIFICATION (`stopForRestart`), so
peers that negotiated Graceful Restart retain Ze's routes for the restart time.
A restart that keeps the Graceful Restart marker under systemd is `ze signal
restart`, a wait until the process has exited, then `systemctl start ze`; the
refusal does not print it, because a start issued before the exit is a no-op,
and `docs/guide/plugins.md` carries it. On the appliance `ze signal restart` is
not a route either: it exits 0 ("Ze stopped.", `hub.Run`), and
`docs/guide/appliance.md` states that gokrazy's init restarts ze only on a
non-zero status ("Gokrazy's init restarts Ze if it exits with a non-zero status
(except 125, which means \"don't restart\")"), so ze would stay stopped; line 6
names `request reboot` and `ze signal reboot` (settled from that page, round-4
review N2; the gokrazy source is not in this module, so the page is the basis).

Interop: the refusal changes no wire behaviour (the running session keeps its
startup OPEN), so it falls under the config-only exemption of
`ai/rules/interop-and-goal-validation.md`, and no reload interop scenario is
owed. Goal validation is the `.ci` through the real reload entry
point.

### D1 consolidation design (Q4 = Consolidate, 2026-10-13)

The seven Optional Parameters writers, read at the producers on 2026-10-13 and
2026-10-14 (`bgpBuildOpen` and `buildActiveOpen` found by the review of
2026-10-13, ISSUE-2; `encodeOpen` by Q5; a tree grep for a type-2 parameter
written with a one-octet length finds the first six and no other, and a grep for
`ExtendedParamMarker` finds the extended envelope written only by
`message/open.go` and `encodeOpen`):

| Packer | Framing today | Defect |
|--------|---------------|--------|
| `buildOptionalParams` (`internal/component/bgp/reactor/session_negotiate.go`), called by `Session.buildOpen` | one type-2 parameter; one-octet form while `capTotal <= 255`; two-octet form and `ExtendedParams` above; `slog.Error` and nil parameters when `capTotal > maxExtendedParamLen` (65535) | D1 at capTotal 254 and 255; the upper bound lets capTotal 65533 to 65535 through although the Extended Opt. Parm. Length (3 + capTotal) then exceeds 65535 |
| `buildOptionalParams` (`internal/chaos/peer/session.go`), called by chaos `BuildOpen` | one type-2 parameter per capability, one-octet lengths, `ExtendedParams` never set | D1 sibling above 255 octets |
| `packOptionalParams` (`internal/perf/session.go`), called by perf `BuildOpen` | one type-2 parameter per capability, one-octet lengths | the same shape; the perf set (three capabilities) cannot reach 255 |
| inline packing in `fabricateLocRIBOpen` (`internal/component/bgp/plugins/bmp/bmp_locrib.go`), constant `optParamCapabilities` | one type-2 parameter, one-octet length | none reachable: four capabilities of 8 octets or fewer |
| inline packing in `bgpBuildOpen` (`internal/analyze/bgpconn.go`), called by `ze analyze`'s `inject.go` and `serve.go` | one type-2 parameter, one-octet length, three fixed capabilities (MP IPv4, MP IPv6, 4-octet AS) | none reachable: 18 octets |
| inline packing in `buildActiveOpen` (`internal/test/peer/inject.go`), the test peer's active-role OPEN | one type-2 parameter, one-octet length (MP and 4-octet AS) | none reachable: a fixed small set |
| `encodeOpen` (`internal/test/peer/open.go`), the test peer's passive-role OPEN, over `openParam{kind, caps [][]byte, raw []byte}` | a LIST of typed parameters in the order ze's OPEN carried them: type-2 parameters of whole capability TLVs (several, since `addedParams` makes one per `add-capability` override and one for `send-unknown-capability`), and any other parameter type as raw bytes; base form while the sum over parameters of 2 + value length is at most 255, else every length two octets; writes the header and the 255/255 envelope itself; refuses an OPEN above 4096 octets (`openMsgMax`) | none: it frames RFC 9072 correctly. It is a second packer and a second envelope writer (Q5 "a") |

**Home: `internal/core/bgp/capability`, as `capability.PackOptionalParams`.** It
is a core library (`ai/rules/architecture.md`: no config lifecycle, no registry
side effect) whose non-test imports are `internal/core/family` and
`internal/core/textbuf` only (`go list`, 2026-10-13); and it holds the inverse,
`ParseFromOptionalParams(optParams, extended)`, so encoder and decoder sit
together. Six of the seven callers already import it; `internal/analyze` does not
(`go list`: it imports `internal/core/bgp/asn`, `msgtype` and other
`internal/core` packages) and gains this one `internal/core` import, the same
direction it already takes. `./le arch tier check` (the tool behind the
module-tier rule; `./le arch rules` does not exist) exits 2 at today's tree on
violations no touched package is in (for example
`internal/component/bgp/reactor/reactor.go` importing
`internal/component/bgp/plugins/cmd/cache`, review of 2026-10-13), so the proof
owed is that its report names neither `internal/core/bgp/capability` nor any of
the seven changed caller packages, with its "core import direction clean" line.
`internal/component/bgp/message` was rejected: it imports
`internal/component/plugin/registry` (`go list`), so `internal/analyze`, which
imports neither package today, would gain a component import to reach the
packer (review round 2, N5).

**The packer takes typed parameters (Q5 "a").** Its input is a list of
parameters. A parameter is ONE concrete struct with no interface field, the shape
the test peer's `openParam{kind, caps, raw}` (`internal/test/peer/open.go`)
already has, but its fields are private to `internal/core/bgp/capability` and
only two constructors build it (round-4 review, ISSUE-3; final names set at
implementation):

| Private field | Type | Description |
|---------------|------|-------------|
| type | one octet | the Optional Parameter Type (2 for Capabilities, RFC 5492 Section 4) |
| capabilities | a list of `capability.Capability` | the Parameter Value as capabilities: its length is the sum of their `Len`, and each writes itself in place with `WriteTo` at its offset |
| raw | octets | the Parameter Value as given: its length is the octet count, and the packer copies it |

| Constructor | Takes | Builds |
|-------------|-------|--------|
| the Capabilities constructor | a list of `capability.Capability` | type fixed to 2, the list in `capabilities`, `raw` empty |
| the Raw constructor | a Type and octets | that type, the octets in `raw`, `capabilities` empty |

Outside the package a parameter that carries both value forms, or a capability
list under a Type other than 2, cannot be built, so the packer has no refusal for
either (`docs/contributing/ze-go-style.md`, "One state, one variant" and
"Validated at construction";
`ai/rules/points/go-standards/directives/preserve-validity-from-construction-through-use.md`).
A sealed interface with one variant per form was rejected because it brings back
the heap escape the struct removes (below). Zero-value contract: the zero value
is a Raw parameter of Type 0 with an empty value, which is a well-formed
parameter (two octets, `00 00`), so the packer writes it as given and refuses
nothing. Nothing in this spec builds one; a receiver refuses type 0 as an
unsupported parameter (`ParseFromOptionalParams`, `ErrUnsupportedParameter`),
which is the same answer any other non-type-2 Raw parameter gets.

| Constructor used | Who calls it |
|------------------|--------------|
| Capabilities | the reactor (its whole capability set as one type-2 parameter), chaos, perf, bmp, `ze analyze`, the active test peer |
| Raw | `encodeOpen`: each type-2 `openParam` hands its whole TLVs joined in their order (they are well formed: `splitCapabilityTLVs` checks the TLVs it mirrors, `addedParams` bounds each at 255, and the TLVs the test peer adds come from capability `WriteTo`), and any other parameter type hands its raw octets verbatim |

The value is a struct field rather than an interface (round-3 review, ISSUE-1,
probed under go1.27.2 with `-gcflags=-m`): a capability list converted to an
interface value escapes to the heap when it flows into the interface's
`Len`/`WriteTo` calls, so a reactor that builds its parameter list per OPEN
would allocate twice, where a slice of concrete structs does not escape and the
output buffer stays the one allocation `buildOptionalParams` makes today. The
Raw field is also why no exported constructor for `capability.Unknown` is
added: the test peer's TLVs pass through as octets, where wrapping each in an
`Unknown` would reproduce the same bytes through a new core API that only test
tooling calls (round-3 review, N1, `ai/rules/simplicity.md`).

The packer, as steps (one code path for every caller):

1. No parameters: no octets, base form. A caller with no capabilities hands no parameter.
2. base is the sum over parameters of 2 + the value length (a parameter from the Capabilities constructor: the sum of its capabilities' `Len`; from the Raw constructor: the octet count).
3. While base is at most 255: each parameter written as Type, a one-octet Parameter Length, then its value; extended false (RFC 4271 / RFC 5492 form; RFC 9072 Section 2 SHOULD).
4. Else ext is the sum over parameters of 3 + the value length; while ext is at most 65535: each parameter written with a two-octet Parameter Length; extended true (RFC 9072 Section 2 MUST).
5. Above that: an error naming the size and the limit, no octets. Today's `slog.Error` with nil parameters is a silently wrong OPEN (`ai/rules/principles.md`); `buildOpen` already returns an error, so the reactor propagates it. This size error is the packer's only error. R-4 records the separate 4096-octet OPEN limit (not fixed here).
6. The output is allocated once, at the size step 3 or 4 chose, and each capability writes itself into it at its offset with `WriteTo` (a Raw constructor's octets are copied in): no per-parameter or per-capability intermediate buffer, and no allocation besides the output, the parameter list being a slice of concrete structs. The result is the Optional Parameters octets and the extended flag, which the caller stores in `message.Open.OptionalParams` and `ExtendedParams`.

For one parameter the boundaries are unchanged: base form up to a value of 253
octets (2 + 253 = 255), extended from 254, and the error from 65533 (3 + 65533
above 65535). For `encodeOpen` the rule is the one it applies today, so its
bytes do not change.

Raw octets as the ONLY value form were rejected: the reactor would serialise its
capabilities into an intermediate buffer and copy it, which breaks the
one-allocation property of `buildOptionalParams` (`session_negotiate.go`, the
in-place `c.WriteTo(buf, off)` loop) and adds a third form on the way to the wire
(`ai/rules/performance.md`).

| Caller | Change |
|--------|--------|
| `Session.buildOpen` | calls the packer, sets `OptionalParams` and `ExtendedParams`, returns its error |
| chaos `BuildOpen` | calls the packer and sets `ExtendedParams`; the chaos peer's OPEN moves from one parameter per capability to one parameter for all, which every RFC 5492 receiver accepts |
| perf `BuildOpen` | the same; `TestBuildOpenIPv6` walks the parameters and must read the one-parameter form |
| `fabricateLocRIBOpen` | calls the packer; bytes unchanged; `optParamCapabilities` is deleted (its only reader is this packing, grep 2026-10-13) |
| `bgpBuildOpen` | calls the packer; bytes unchanged (one parameter of 18 octets); gains an error return |
| `buildActiveOpen` | calls the packer; bytes unchanged; gains an error return, and `inject.go`'s handshake returns it as `Result{Error: ...}` the way its `write OPEN` failure does |
| `encodeOpen` | its own length sums and parameter framing are deleted; it maps its `openParam` list to typed parameters (each parameter's Type kept, its value in Raw: a type-2 parameter's TLVs joined in order, any other type's octets verbatim; parameter order kept) and calls the packer; the header and envelope move to `message.Open.WriteTo` (below); it keeps its 4096-octet OPEN refusal and its existing error return; bytes unchanged for every existing `.ci`, multi-parameter layout kept |

None of chaos `BuildOpen`, perf `BuildOpen`, `fabricateLocRIBOpen`,
`bgpBuildOpen` or `buildActiveOpen` returns an error today. Each gains one and
its callers handle it, because a dropped error is the silent path this spec
removes. Under the fixed capability sets of perf (three capabilities), bmp (four
of at most 8 octets), `ze analyze` (18 octets) and the active test peer, the
error cannot happen; the return is kept anyway, because a packer that can answer
"too large" must not have that answer dropped (`ai/rules/principles.md`), and a
later capability added to one of those sets would otherwise reach a silent path.

Error propagation, resolved 2026-10-13 by reading every call site:

| Call site | Error path it gains |
|-----------|---------------------|
| chaos `simulator.go` (session OPEN) | emits `EventError` and returns, as its `sending OPEN` failure does |
| chaos `simulator_actions.go` storm loop | closes the connection and breaks, as its write failure does |
| chaos `simulator_actions.go` collision | closes the connection and returns, as its write failure does |
| perf `doHandshake` (`session.go`) | returns the error, which it already does for `WriteMessage` |
| bmp `ensureLocRIBPeerUp` | logs and returns before the sender loop; no claim is held yet |
| bmp `primeLocRIBPeerUp` | resets `ss.locRIBUpSent` (the claim is held), logs and returns, the way its `writePeerUpLocked` failure does |
| `ze analyze` `bgpOpenExchange` (`inject.go`) and the `serve.go` handshake | return the error, as their `bgpWrite` failure does |
| test peer `inject.go` | returns `Result{Error: ...}` |
| test peer `encodeOpen` caller (`open.go`, `return encodeOpen(zeBody, id, params)`) | unchanged: `encodeOpen` already returns an error, and the packer's error joins it |

The chaos peer and both test peer roles now share Ze's encoder, so an encoder
bug would reach them too; the decoder (`ParseFromOptionalParams`) is separate
code, so the AC-1b, AC-1p and AC-1q round trips still catch an asymmetric
error, and AC-1p's byte-identity over the existing `.ci` corpus pins the
passive peer's bytes independently of the encoder.

**One envelope writer (N6, verified 2026-10-14).** `encodeOpen` is the only
envelope writer outside `message/open.go` (a tree grep for
`ExtendedParamMarker` writes finds `writeToExtended` and `encodeOpen` alone), so
it moves onto `message.Open.WriteTo`: it fills a `message.Open` (Version
mirrored from ze's OPEN, My AS from `myASField`, its Hold Time and BGP
Identifier, the packer's octets and extended flag, and no `ASN4`, so My AS is
written as given), sizes it with `Open.Len`, refuses above `openMsgMax` (4096)
before allocating the message (the packer has already allocated the parameter
octets by then), allocates the message once and calls `Open.WriteTo`. The packer returns
only the parameter octets and the flag; the envelope stays with `Open`, whose
`ExtendedParams` rule already makes the two agree.

Deleted, no layering: reactor `buildOptionalParams` and `maxExtendedParamLen`,
chaos `buildOptionalParams`, perf `packOptionalParams`, the bmp inline packing,
the `bgpBuildOpen` inline packing, the `buildActiveOpen` inline packing, and
`encodeOpen`'s length sums, parameter framing, header and envelope writing
(`paramValueLen` goes with them if nothing else reads it).

### Received-OPEN readers (RFC 9072 decode, brought in 2026-10-14)

Found by the review of 2026-10-14 (out of scope there) and brought into this
spec by the main thread, because it is RFC 9072 decoding and D1 makes Ze emit
extended OPENs: once D1 lands, Ze's own OPEN at capTotal 254 and up is one these
readers misframe. Read at the producers on 2026-10-14:

| Reader | What it does | Defect |
|--------|--------------|--------|
| `routeReflector.handleStructuredOpen` (`internal/component/bgp/plugins/rr/rr.go`) | `message.UnpackOpen`, then walks `open.OptionalParams` with a one-octet Parameter Length, `capability.Parse` on each type-2 value, collects families (Multiprotocol) and ASN4 | ignores `open.ExtendedParams`: an RFC 9072 OPEN's two-octet lengths are read as one octet, so the walk misframes from the first parameter and rr sees wrong or no families and capabilities |
| `parseStructuredOpen` (`internal/component/bgp/plugins/rs/server.go`) | the same walk, `formatCapInfo` per capability, ASN4 | the same |
| `exabgpNegotiatedEncoding` (`internal/le/interoplab/bgp/exabgp_server.go`), interop test tooling | reads the length from `openBody[9]` and calls `capability.ParseFromOptionalParams(..., false)` | an extended OPEN (255/255 marker) is read as 255 octets of one-octet parameters: ADD-PATH is missed or the parse fails |
| `bgpExtractAS4` (`internal/analyze/bgpconn.go`), called by `serveBGPOpen` (`serve.go`) for `ze analyze serve` (brought in by the main thread, 2026-10-14) | reads the length from `body[9]` and walks the parameters with a one-octet length for the Four-octet AS capability, answering My AS on any failure | an extended OPEN is read as 255 octets of one-octet parameters, so the AS is missed; an unreadable OPEN answers My AS. Fixed: `bgpPeerAS` reads the RFC 9072 envelope (`bgpPeerOptionalParams`, `internal/analyze` imports no `message`, see the D1 home rationale) and the parameters through `ParseFromOptionalParams`, and an unreadable OPEN is an error; `TestAnalyzePeerASReadsExtendedOpen` (red with the old walk restored: RFC 9072 form AS 23456) |
| `grPlugin.handleStructuredOpen` (`internal/component/bgp/plugins/gr/gr.go`), brought in by the main thread under the owner's delegation, 2026-10-14 | `message.UnpackOpen`, then walks `open.OptionalParams` with a one-octet Parameter Length and hands each type-2 value to `extractGRCaps`, which decodes codes 64 and 71 from the raw octets (the N bit and the per-family flags `capability.GracefulRestart` does not carry) | an extended OPEN is misframed, so the peer's Graceful Restart Capability is missed and its routes are purged rather than retained: a protocol defect. Fixed: walks with `capability.WalkOptionalParams(open.OptionalParams, open.ExtendedParams, ...)`; a framing error logs at DEBUG and leaves the earlier capability (the reactor refuses that OPEN); `TestGRReadsExtendedOpen` (red with one-octet framing forced: capability not stored, open-impl/readers3-red.log) |
| `extractCapabilitiesFromOptParams` (`internal/component/bgp/server/validate.go`), called by `openMessageToRPC` for the validate-open RPC (brought in 2026-10-14) | the same one-octet walk, raw `{code, hex}` pairs per capability | an extended OPEN reaches the policy plugins (role) as a misframed or empty capability list. Fixed: walks with `WalkOptionalParams` and `open.ExtendedParams`; a framing error is an error, so `openMessageToRPC` returns `(msg, error)` and `broadcastValidateOpen` refuses the pair instead of sending a shorter list; `TestOpenMessageToRPCReadsExtendedOpen` (red: short read on the RFC 9072 form) |
| `parseOpen` and `parseCapabilities` (`internal/mrt/bgp.go`), the offline MRT decoder (brought in 2026-10-14) | reads the length from `body[9]` and walks with a one-octet Parameter Length | an extended OPEN is read as 255 octets of one-octet parameters: AS 23456 and no capabilities. Fixed: `openOptionalParams` reads the RFC 9072 envelope (mrt imports no component package; `message` imports `plugin/registry`, the same reason as `bgpPeerOptionalParams`), `parseCapabilities` walks with `WalkOptionalParams`, and a parameter past the field is an error; `TestParseOpenReadsExtendedOpen` (red: AS 23456, no capabilities) |

`plan/journal/helper-bypassed-by-an-open-coded-copy.md` carries the HEAD defect
(2026-10-14, "designed in spec-bgp-open-session-rfc-defects").

**Design: one reader.** `capability.ParseFromOptionalParams(optParams, extended)`
already decodes both framings and is what the reactor, bmp and the CLI decoder
call.
It now decodes through `capability.WalkOptionalParams(optParams, extended, fn)`,
the one reader of the parameter framing, which the readers that need a
capability's raw octets (gr, the validate-open RPC, mrt) call directly; it moved
with `ParseFromOptionalParams` into `optional_params.go` beside the packer.

| Reader | Change |
|--------|--------|
| rr `handleStructuredOpen` | the parameter walk is deleted; it calls `ParseFromOptionalParams(open.OptionalParams, open.ExtendedParams)` and reads families and ASN4 from the result; on an error from `message.UnpackOpen` (today a silent `return`) or from `ParseFromOptionalParams` it logs the rr warning below and returns before taking `rr.mu`, so it records nothing for that OPEN (no ASN, no families, no capabilities, and no implicit ipv4/unicast); the dead `if open.ASN4 > 0 { asn = open.ASN4 }` branch is deleted, since `UnpackOpen` never sets `ASN4` (`message/open.go`: a receiver must not read it) |
| rs `parseStructuredOpen` | the same call, then `formatCapInfo` per capability; on an error from `message.UnpackOpen` (today a silent `return nil`) or from `ParseFromOptionalParams` it logs the rs warning below and returns nil, so `handleOpen` (`server_handlers.go`) is not called and nothing is recorded; the same dead `open.ASN4` branch is deleted |

**The warning (round-4 review, ISSUE-1 and N4).** Both decode failures warn with
the same text, because they have the same outcome: the reactor refuses the same
OPEN (an `UnpackOpen` error and a `ParseFromOptionalParams` error both reach
`rejectOpenCapabilityError` in `session_handlers.go::handleOpen` and
`session_open_as.go::validateOpenPeerAS`), and the plugin records nothing. At
the default WARN level this is the only line that says why in the operator's
terms. The reactor's own lines give a code and no cause: the NOTIFICATION it
sends logs `notification sent` at WARN with the peer, code and subcode
(`session_write.go`; a failed send logs `notification send failed` through
`logNotifyErr` in `session.go`), and `peer_run.go` logs the returned error only
at DEBUG (`safeRunOnce returned`). The round-4 review read this line as the only
one at WARN; `session_write.go` shows the success WARN too (read 2026-10-14). One
WARN record per refused OPEN, through the plugin's `logger().Warn`
(`ai/rules/user-facing-errors.md`: the problem first, why it matters, then the
next step; the decoder error follows as evidence, never as the whole message).
The record's message is the three lines joined by a space, a constant that
carries neither the peer address nor the decoder error: the peer address is in
the `peer` attribute and the decoder error in the `error` attribute, the
attribute names rr and rs already use, and the logger renders both after the
message on the same line (round-5 review, I3). Message lines for rr:

| Line | Text |
|------|------|
| 1 | `The route reflector cannot read this peer's OPEN message, so it records no address families for this peer.` |
| 2 | `ze refuses this OPEN, so the session with this peer does not come up.` |
| 3 | `Next step: check the capabilities configured on this peer, then compare them with the error attribute.` |

Rendered record (output sample, one line): `level=WARN msg="<lines 1 to 3>"
peer=192.0.2.1 error="capability: unsupported optional parameter"`.

rs uses the same three lines with line 1 reading `The route server cannot read
this peer's OPEN message, so it records no capabilities for this peer.` The two
texts are separate constants, one in each plugin, because each names its own
plugin and what it records; nothing else differs. `TestRouteReflectorReadsExtendedOpen`
and `TestRouteServerReadsExtendedOpen` capture the plugin's logger and assert,
for each refused case and for a body `UnpackOpen` refuses, one WARN record whose
message holds the three sentences and whose `peer` and `error` attributes carry
the peer address and the decoder error.
| `exabgpNegotiatedEncoding` | the hand length read is deleted; it calls `message.UnpackOpen(openBody)` (the package already imports `message` elsewhere, `check_ospf_bgpls.go`) and then `ParseFromOptionalParams(open.OptionalParams, open.ExtendedParams)` |

Behaviour that changes beyond the framing, in rr and rs only (round-3 review,
ISSUE-2, read 2026-10-14). The OPEN event reaches them BEFORE the reactor
validates the OPEN: `session_read.go` calls `s.onMessageReceived(...)` ahead of
`case msgtype.TypeOPEN: err = s.handleOpen(body)`, and the callback runs
`peer_run.go` `p.messageCallback`, `reactor_notify.go::notifyMessageReceiver`,
`server/events.go::messageTypeToEventKind` (OPEN to `EventKindOpen`), then
`rr.go` `case rpc.EventKindOpen` and `rs/server.go` `parseStructuredOpen`. So
an OPEN carrying a non-type-2 parameter, an empty type-2 parameter, or one
malformed type-2 parameter among several does reach them. The open-coded walks
skip that parameter and keep the others; rr then stores the implicit
ipv4/unicast default when no Multiprotocol capability survived, and rs, handed
an empty capability list, does the same in `handleOpen`.
`ParseFromOptionalParams` fails the whole OPEN instead
(`ErrUnsupportedParameter`, `ErrInvalidLength` for a Parameter Length of 0, or
the `capability.Parse` error), and the reactor refuses that same OPEN right
after (`handleOpen`, `validateOpenPeerAS` in `session_open_as.go`,
`ParseFromOptionalParams`, `rejectOpenCapabilityError`), so the session never
reaches Established. On that error rr and rs record nothing and log a warning,
and never fall back to ipv4/unicast: a default stored for an OPEN that could not
be read is a silently wrong value (`ai/rules/principles.md`). An entry left
from an earlier session of the same peer stays in place, and the next accepted
OPEN overwrites it. Its readers, read 2026-10-14 (round-4 review, N5): rr
forwarding (`validation.go`), rs forwarding (`server_forward.go`), rs
`handleRefresh` and rs `server_validation.go` test `peer.Up`, so they skip it
while the peer is down; rr `sendEOR` (`rr.go`) and rs `sendEOR`
(`server_handlers.go`) test `ReplayGen`, which only an up event advances
(`handleStructuredState` does not bump it on down), so a replay goroutine
started for the earlier session can still read the entry's families after a
down. That is no worse under this design: it reads the earlier session's real
families, where the old walk would have stored the partial or default set. The exabgp mock changes nothing here: it
already calls `ParseFromOptionalParams` (with `extended` false), and ExaBGP only
ever emits type-2 parameters (`capabilities.py::pack_capabilities`: one
Capabilities parameter), although it does emit the RFC 9072 form above 255
octets, which is why the mock needs AC-1r (round-3 review, N5). Each call
carries `// RFC 9072 Section 2` (`ai/rules/rfc-compliance.md`, callers).
These units carry no RFC tag: RFC9072-2-2 ("MUST accept") is met by the
reactor's decode, which already reads the framing; the rr and rs misread is a
Ze-internal reader defect behind it.

### Recorded-families field (round 5, 2026-10-14; owner question Q6)

The two `.ci` tests read what rr and rs recorded through the operator's peer
command (see "Why these assertions" under "Wiring Test"). Each row of
`show rr peers` (`rr.go::peerStatus`) and `show bgp rs peers`
(`rs/server_handlers.go::peerStatus`) gains one read-only key. No command,
argument or YANG node is added: neither command declares an output schema
(`rr/yang/ze-rr-cmd.yang` declares the command only; rs registers its commands
in `server.go`), and the payload stays structured data, so `| json`, `| table`
and the row operators render it unchanged (`ai/rules/cli.md`).

| Field | Type | Description |
|-------|------|-------------|
| `families` | list of family names (`ipv6/unicast` form, `family.Family.String`), sorted | The families the plugin recorded from the peer's received OPEN for the current session: the Multiprotocol families, or `ipv4/unicast` alone when the OPEN carried no Multiprotocol capability (RFC 4760 Section 1). rs: an empty list means the OPEN carried Multiprotocol capabilities naming no family the plugin knows (`handleOpen` filters through `family.LookupFamily`); rr stores every Multiprotocol AFI/SAFI pair (`handleStructuredOpen`), so its list is never empty when Multiprotocol is present |

When the key is absent, and why absence is the answer rather than an empty
list (`ai/rules/principles.md`, no silently wrong value):

| Row state | `families` | Why |
|-----------|------------|-----|
| `up` false | absent | the entry may hold an earlier session's families (a refused OPEN records nothing, "Received-OPEN readers"); no reader uses them while the peer is down (A-7), so the row shows none |
| `up` true, no OPEN recorded (`Families` nil in either plugin) | absent | nothing is recorded; for rs a nil set means `SupportsFamily` applies no filter (`rs/peer.go`), which an empty list would misstate |
| `up` true, OPEN read | present | the recorded set, as defined above |

A stale set under `up` true is not reachable: the received OPEN is delivered
before the reactor handles it (A-7), so before Established and before the
state-up event, and each plugin has one delivery queue (`plugin/process/delivery.go`,
one event channel per process, cited in `rs/server_handlers.go`), so the current
session's OPEN is recorded before its row turns up. Stronger still (round-6
review, N4): `onMessageReceived` (`bgp/server/events.go`) blocks on every
delivery's `Result`, which `deliverBatch` (`plugin/process/delivery.go`) signals
after `sendBatch` returns, and `session_read.go` calls it before `handleOpen`;
rr and rs handle the OPEN inline in their structured handler. This is a reading
of the producers; the two `.ci` polls do not rest on it. The comment on
`rs/peer.go::SupportsFamily` still describes "the window between state-up and
OPEN processing", which this reading contradicts; it is corrected under Files
to Modify.

The key name is kebab-case and matches the term the plugins' own code uses
(`peer.Families`). rs's command declaration (`server.go`, `show bgp rs peers`:
`Columns` address, remote, up) is left unchanged: `families` is not declared as
a column because `server_shape_test.go::TestDeclaredColumnsExistInPayload`
requires every declared column in every row and the key is absent on some rows,
and an undeclared key still renders after the declared ones, alphabetically
(`internal/component/command/column_order.go`). rr declares no column order. Unit tests: `TestRouteReflectorPeerStatusFamilies` and
`TestRouteServerPeerStatusFamilies` (TDD Test Plan).

## Re-verification at HEAD `3e2770062b` (2026-10-13)

No row is fixed. Nothing is dropped.

| Row | Still open? | Evidence |
|-----|-------------|----------|
| D1 | Yes | `buildOptionalParams` still takes the one-octet branch on `capTotal <= 255` (`internal/component/bgp/reactor/session_negotiate.go`, the `if capTotal <= 255` line); the Optional Parameters field it returns is `2+capTotal` octets, so capTotal 254 and 255 give 256 and 257 octets with `ExtendedParams` false, and `Open.WriteTo` takes the extended envelope on `optLen > 255`. No commit touched the function since the skeleton (`git log` on the file: latest `12f2cd322a`, which changed reload bindings, not this function) |
| D1 sibling | Yes, new | `internal/chaos/peer/session.go::buildOptionalParams` writes one type-2 parameter per capability with one-octet lengths and never sets `ExtendedParams`, so above 255 total octets `Open.WriteTo` wraps one-octet parameters in the extended envelope: the same misframing, in the chaos test peer. `internal/perf/session.go` (three capabilities) and `internal/component/bgp/plugins/bmp/bmp_locrib.go::fabricateLocRIBOpen` (bounded set) cannot reach 254, nor can `internal/analyze/bgpconn.go::bgpBuildOpen` and `internal/test/peer/inject.go::buildActiveOpen` (fixed small sets). Under Q4 = Consolidate all six use the one packer |
| D2 | Yes | `sdk.Plugin.SetCapabilities` is documented "Must be called before Run()" and is read once at stage 3 (`pkg/plugin/sdk/sdk.go`, "Stage 3: declare-capabilities"); the engine has no runtime declare method (the `ze-plugin-engine:` method list holds `declare-capabilities` only as a startup stage); `CapabilityInjector` entries are removed only by `releasePluginRegistrations` (`internal/component/plugin/server/restart.go`). `softver`, `hostname`, `role` register `OnConfigure` only; `llnh` and `gr` add an `OnConfigVerify` refusal and no apply. A reload that edits a peer's capability block restarts the peer (`peerSettingsSwapPlan`, `CapabilityConfigJSON` differs), and `buildOpen` reads plugin capabilities through `pluginCapGetter`, i.e. the stage-3 store. `negotiatedCapabilitySettings`' comment (`peer_settings_negotiation.go`) records the same gap |
| D3 | Yes | `Session.handleNotification` logs only the RFC 8203 shutdown message (INFO) or an invalid one (WARN); the received code and subcode reach the counters and the report bus (`incrNotificationReceived`, `raiseNotificationError`: visible in `ze show errors`), and the reconnect loop logs `safeRunOnce returned` at DEBUG. `slogutil.Logger` defaults to WARN, so at the default level an OPEN refusal writes no log line. The skeleton's "WARN or INFO" is narrowed to WARN: an INFO line is invisible at the default level |
| D4 | Yes | `parsePeerSettings` (`internal/component/bgp/reactor/config.go`): "Router ID from session > router-id (peer-level overrides global)"; the `peer-fields` grouping of `ze-bgp-conf.yang` carries `leaf router-id` ("Override the router ID for this peer.") |
| RFC9072-2-1 verdict | weak | `rfc/audit/rfc9072.json`: "the procedure's two-octet Parameter Length per parameter is not asserted in this row's units" |
| RFC6286-2.1-1 verdict | weak | `rfc/audit/rfc6286.json`: "Ze accepts a per-peer router-id override ... nothing tagged goes red on it" |
| spec-rfc-verdict-fix-bgp (BGP child of `plan/pre-release/spec-rfc-verdict-test-fix-pass.md`, closed 2026-10-21) | 2 rows pointed here; AC-4 owns them alone | RFC6286-2.1-1 and RFC9072-2-1, both `weak`; the parent's Wiring note names this spec's AC-4 |
| `./le rfc check` | exit 0 | 4147 gated requirements; it prints no per-row verdict for these two, which the audit files above carry |
| False claim found | Yes | The `rfc/short/rfc6286.md` RFC6286-2.1-1 row annotation says the value "is read once at config load into reactor.Config.RouterID and used for every peer and every OPEN", which the per-peer override contradicts. `ai/rules/rfc-compliance.md` requires the correction immediately; AC-5 carries it |
| R48 (`plan/handover/rfc-verdict-test-fix-pass/RULINGS.md`) | Not this spec | The RFC 4271 Section 8.2.2 "any other event" row was routed to the verdict-fix child, not here; `docs/architecture/behavior/fsm-open-sent.md` now documents `fsmMessageEvent` sending 5/0 for it. Named so it is not mistaken for a dropped row |

## Required Reading

### Architecture Docs
- [ ] `docs/architecture/config/apply-ordering.md` - reload order of plugin sections against BGP peer operations
  → Constraint: verify runs for the whole transaction before any apply, so the D2 refusal (Q1 = b, the SDK's config-verify comparison) stops the reload before any peer restarts; the apply order is not relied on
- [ ] `docs/architecture/api/process-protocol.md` - the 5-stage handshake; stage 3 `ze-plugin-engine:declare-capabilities`
  → Constraint: capabilities are declared once, in stage 3, by `DeclareCapabilitiesInput`, and stay fixed for the plugin process's life. Q1 = b keeps that protocol; the page gains one sentence saying a capability plugin refuses a reload that would change its declaration
- [ ] `docs/architecture/behavior/fsm-open-sent.md` - received NOTIFICATION in OpenSent (Event 24 vs 25)
  → Constraint: Event 25 in OpenSent is answered with 5/0 by `fsmMessageEvent`; the D3 log line MUST NOT change that FSM behaviour, only add the WARN
- [ ] `docs/architecture/testing/interop.md` - scenario layout, typed checkers, `opWaitLogContains`, the revert-to-red rule
  → Constraint: every interop scenario is a named directory with a typed checker in `internal/le/interoplab/bgp`; a reload scenario follows `bgp-reload-global-router-id` (`ze-reload.conf`, `opExec` copy, `opSignal` HUP)

### RFC Summaries (Scope: protocol)
- [ ] `rfc/short/rfc9072.md`, `rfc/full/rfc9072.txt` Section 2
  → Constraint: RFC 9072 Section 2: "However, if the length of the Optional Parameters in the BGP OPEN message does exceed 255, the OPEN message MUST be encoded according to the procedure below." The length is the WHOLE Optional Parameters field, parameter headers included, so with Ze's single type-2 parameter it is 2 + capTotal and the boundary is capTotal 254
  → Constraint: RFC 9072 Section 2: "The rules for encoding Optional Parameters are unchanged with respect to those given in [RFC4271], except that the Parameter Length field is extended to be a two-octet unsigned integer." The envelope and every parameter inside it must agree (`Open.ExtendedParams` exists for this)
  → Constraint: RFC 9072 Section 2: "In the event that the length of the Optional Parameters in the BGP OPEN message does not exceed 255, the encodings of the base BGP specification [RFC4271] SHOULD be used without alteration." So 255 octets of parameters (capTotal 253) stays one-octet: the negative polarity
- [ ] `rfc/short/rfc6286.md`, `rfc/full/rfc6286.txt` Section 2.1
  → Constraint: RFC 6286 Section 2.1: "For a BGP speaker that supports the AS-wide Unique BGP Identifier, the BGP Identifier is specified as the following: ... The value of the BGP Identifier for a BGP speaker is determined on startup and is the same for every local interface and every BGP peer." The sentence carries no BCP 14 keyword (RFC 6286 has no BCP 14 boilerplate); the summary labels the row MUST. RFC 4271 Section 4.2 carries the same sentence ("The value of the BGP Identifier is determined upon startup and is the same for every local interface and BGP peer."); `rfc/short/rfc4271.md` has no row for it, and RFC scope is frozen (no new rows planned here)
- [ ] `rfc/full/rfc8654.txt` Section 6 - OPEN size
  → Constraint: RFC 8654 Section 6 keeps OPEN at 4096 octets ("This document changes the latter number to 65,535 for all messages except for OPEN and KEEPALIVE messages."), so the D1 boundary tests stay far below any message-size limit

**Key insights:**
- D1 is an off-by-two: the one-octet branch must hold only while 2 + capTotal <= 255; it is fixed once, in the one packer (Q4)
- D2 has no runtime path: declaration is stage 3 only, removal only on plugin restart; Q1 = b refuses the reload that would need one
- D3 is visible today only in `ze show errors` and counters, never in the log at the default WARN level
- D4 is decided as an owner-approved deviation (Q2): no config or test-corpus change, documentation and the RFC6286-2.1-1 deviation record only

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `internal/component/bgp/reactor/session_negotiate.go` - `buildOptionalParams` packs every capability into ONE type-2 parameter; one-octet form while `capTotal <= 255`, extended form (3 + capTotal, `ExtendedParams` true) above, `slog.Error` and nil parameters above 65535. `buildOpen` is the one OPEN producer, called by `sendOpen` and by the reload probe `negotiationOutcomeUnchanged`
  → Constraint: `buildOpen` must stay the single OPEN producer of the reactor; the reload probe compares OPENs it builds. Its packer moves to `capability.PackOptionalParams` (Q4)
- [ ] `internal/component/bgp/message/open.go` - `Open.Len`/`Open.WriteTo` take the extended envelope when `ExtendedParams || optLen > 255`; `writeToExtended` writes 255/255 and a two-octet length
  → Constraint: preserve; the envelope decision is correct, the packer's framing choice is what is wrong
- [ ] `internal/chaos/peer/session.go` - `buildOptionalParams` (chaos test peer): one parameter per capability, one-octet lengths, no `ExtendedParams`
  → Decision: the sibling of D1; deleted, the chaos `BuildOpen` calls the one packer (AC-1b, Q4)
- [ ] `internal/perf/session.go` - `packOptionalParams`, called by perf `BuildOpen`: one parameter per capability
  → Decision: deleted, perf `BuildOpen` calls the one packer (Q4)
- [ ] `internal/component/bgp/plugins/bmp/bmp_locrib.go` - `fabricateLocRIBOpen` packs one type-2 parameter inline with `optParamCapabilities`
  → Decision: the inline packing and the constant (read nowhere else) are deleted; it calls the one packer, bytes unchanged (Q4)
- [ ] `internal/analyze/bgpconn.go` - `bgpBuildOpen` packs three fixed capabilities into one type-2 parameter inline
  → Decision: the inline packing is deleted; it calls the one packer, bytes unchanged, and returns its error (Q4)
- [ ] `internal/test/peer/inject.go` - `buildActiveOpen` packs MP and 4-octet AS into one type-2 parameter inline; `internal/test/peer/open.go::encodeOpen` writes a list of typed parameters with RFC 9072 framing
  → Decision: `buildActiveOpen`'s packing is deleted and it calls the one packer (Q4); `encodeOpen`'s framing and envelope are deleted, it hands its parameter list to the packer as typed parameters and writes through `message.Open.WriteTo`, keeping its 4096-octet refusal (Q5 "a")
- [ ] `internal/component/bgp/plugins/rr/rr.go::handleStructuredOpen`, `internal/component/bgp/plugins/rs/server.go::parseStructuredOpen`, `internal/le/interoplab/bgp/exabgp_server.go::exabgpNegotiatedEncoding` - received-OPEN readers with a one-octet Parameter Length that ignore `ExtendedParams`
  → Decision: each calls `capability.ParseFromOptionalParams` with the OPEN's own framing; the open-coded walks are deleted (AC-1r)
- [ ] `internal/plugins/exabgp/bridgeplugin/internal.go` - `capabilityDecls` (route-refresh, add-path, no `Peers`), `bridgeRunner.applyConfig` (returns nil once started)
  → Decision: its extractor is `capabilityDecls`, registered with the SDK like the five bgp plugins (Q1 = b)
- [ ] `internal/core/bgp/capability/capability.go` - `ParseFromOptionalParams(optParams, extended)`, the decoder; package imports only `internal/core/family` and `internal/core/textbuf`
  → Decision: home of `PackOptionalParams` (Q4)
- [ ] `pkg/plugin/sdk/sdk_callbacks.go`, `pkg/plugin/sdk/sdk.go`, `pkg/plugin/sdk/sdk_dispatch.go` - `SetCapabilities` stores the set in `Plugin.capabilities` for stage 3; `OnConfigVerify` receives the full candidate sections inside the reload transaction and overwrites `p.callbacks[callbackConfigVerify]`; `Run` serves stage 2 through `serveOne(ctx, callbackConfigure, p.handleConfigure)`, and a runtime `configure` redelivery reaches the same `handleConfigure`
  → Decision: home of `Plugin.DeclareCapabilitiesFrom`, the one capability registration: the extractor runs for the declaration only on the stage-2 path inside `Run`, never in `handleConfigure`, and again at config-verify for the comparison, composed with the plugin's own verify handler at `Run`; `SetCapabilities` is deleted (Q1 = b)
- [ ] `internal/plugins/exabgp/main_sdk.go` - flag-built capabilities through `SetCapabilities`; `WantsConfig: []string{"bgp"}`, no `OnConfigure`
  → Decision: registers a constant extractor
- [ ] `internal/component/plugin/server/startup_removal.go` - after a refused removal the engine redelivers `configure` with the committed sections (`conn.SendConfigure`)
  → Constraint: the redelivery must not rewrite the stored declaration
- [ ] `internal/component/plugin/server/startup.go` - `onCapabilities` converts and `capInjector.AddPluginCapabilities`; a conflict is an error
- [ ] `internal/component/plugin/registration.go` - `CapabilityInjector.AddPluginCapabilities`, `RemovePluginCapabilities`
  → Constraint: unchanged under Q1 = b; the injector keeps the stage-3 sets, which is why a change to them is refused
- [ ] `internal/component/bgp/plugins/softver/softver.go`, `hostname/hostname.go`, `role/role.go`, `llnh/llnh.go`, `gr/gr.go` - each extracts per-peer `CapabilityDecl` in `OnConfigure` and calls `SetCapabilities`; `llnh` and `gr` also refuse in `OnConfigVerify`
  → Decision: each deletes its `SetCapabilities` call and registers its extractor with the SDK; no plugin gains a verify handler for this, and the `llnh` and `gr` handlers keep their own refusals, which the SDK runs first (Q1 = b). `role`'s `filterPeerConfigs` for `OnValidateOpen` cannot drift from the advertised role, because a role change is refused
- [ ] `internal/component/bgp/configjson/traverse.go` - `ForEachPeer`, `CapabilitySelector`
  → Decision: home of `CapabilitySelectors` (Q1 = b)
- [ ] `internal/component/bgp/reactor/peer_settings_apply.go` - `peerSettingsSwapPlan`: a changed `CapabilityConfigJSON` forces a restart; `peer_settings_negotiation.go::negotiatedCapabilitySettings` documents why
  → Constraint: preserve the restart for every capability change the reload accepts; a plugin-declared change never reaches it, because verify refused the reload
- [ ] `internal/component/bgp/reactor/session_handlers.go` - `handleNotification`: counters and report bus via `onNotifRecv`, INFO for an RFC 8203 message, FSM event, close, returns `ErrNotificationRecv`
- [ ] `internal/component/bgp/reactor/peer_run.go` - reconnect loop logs `safeRunOnce returned` at DEBUG; "session closed" INFO only on leaving Established
- [ ] `internal/core/slogutil/slogutil.go` - `Logger`: default level WARN
  → Constraint: D3's line must be WARN to be seen at the default level
- [ ] `internal/component/bgp/reactor/config.go` - `parsePeerSettings` reads `session router-id` per peer and group, overriding the global; `parseRouterID` refuses 0.0.0.0
- [ ] `internal/component/bgp/yang/ze-bgp-conf.yang` - global `leaf router-id` (mandatory unless every session carries one) and the `peer-fields` `leaf router-id` override
- [ ] `internal/test/peer/expect.go`, `internal/test/peer/peer.go` - ze-peer `option=open:value=inspect-open-message` checks the received OPEN against `expect=bgp` rules; `action=notification|sighup|rewrite` exist
- [ ] `test/reload/reload-capability-restart-names-both-fields.ci` - the model for a capability-block reload `.ci` (rewrite + sighup + Cease 6/6 + conn 2)
- [ ] `test/interop/scenarios/frr-software-version/`, `internal/le/interoplab/bgp/check_softver.go` - FRR refuses the draft form with 2/0; the checker reads Ze's `show bgp peer ... detail`

**Behavior to preserve:**
- OPENs whose Optional Parameters fit 255 octets stay byte-identical (every `test/encode` and `test/decode` OPEN expectation)
- The reload restart decision and its logged reason (`reload-capability-restart-names-both-fields.ci`)
- The FSM's answer to a received NOTIFICATION (`fsm-open-sent.md` table), the counters and the `ze show errors` entries
- `allow-shared-router-id` and the RFC 6286 Section 2.2/2.3 behaviour
- The live global `router-id` reload (`applyGlobalsJournaled` and the peer diff restart, `test/reload/reload-global-router-id.ci`), out of scope by Q3
- The BMP Loc-RIB OPEN bytes (`fabricateLocRIBOpen`), which the one packer reproduces

**Behavior to change:** D1 and its sibling in the one packer (Q4), D2 as a refusal (Q1 = b), D3, and D4 as the documented deviation (Q2); the false RFC6286-2.1-1 annotation. The chaos and perf OPENs move from one parameter per capability to one parameter for all. The rr, rs and exabgp-mock readers of a received OPEN decode the RFC 9072 framing through `capability.ParseFromOptionalParams` (AC-1r).

**New restriction (Q1 = b), stated as one:** a reload that today is accepted is refused and needs a daemon restart in two cases: a session family added or removed on a `gr` peer with no `graceful-restart family` list (the GR payload follows the session families, `grAdvertisedFamilies`), and a peer added that carries a plugin-declared capability (`softver`, `hostname`, `role`, `llnh`, `gr`). A reload that changes the exabgp bridge's `route-refresh` or `add-path`, or edits the bridge's families while `add-path` is set, is refused too; today it is accepted and not applied.

## Data Flow (MANDATORY - see `ai/rules/architecture.md`)

### Entry Point
- Session start: `sendOpen` → `buildOpen` (D1)
- SIGHUP / `ze config` commit: the config transaction (verify, apply, operation graph) (D2, D4)
- A NOTIFICATION read in OpenSent or OpenConfirm (D3)
- Config load: `parsePeerSettings` (D4)

### Transformation Path
1. Config load: plugins receive the `bgp` section at stage 2 and declare per-peer capabilities at stage 3; the engine stores them in `CapabilityInjector`
2. `buildOpen` merges configured capabilities with `pluginCapGetter` (injector), coalesces PATHS-LIMIT, calls `capability.PackOptionalParams`, returns `message.Open`; the chaos peer, the perf sender, the BMP Loc-RIB OPEN, `ze analyze` and the test peer's active-role and passive-role OPENs call the same packer, the passive role through typed parameters that keep its parameter layout
3. `Open.WriteTo` chooses the envelope; `writeMessage` sends it
4. Reload: verify sections to plugins (in each capability plugin the SDK runs the plugin's own verify handler, then the registered extractor on the candidate, and refuses a changed declaration, Q1 = b), apply coarse plugin sections, then peer operations; a changed capability block the reload accepted restarts the peer, and the new connection repeats 2-3
5. Receive: `handleNotification` counts, reports, runs the FSM event, closes; the peer loop reconnects after backoff
6. Receive OPEN in rr and rs: the structured OPEN event's raw bytes → `message.UnpackOpen` → `capability.ParseFromOptionalParams(open.OptionalParams, open.ExtendedParams)` → families and capabilities

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Plugin ↔ Engine | stage-3 `declare-capabilities`, unchanged; the refusal travels as the config-verify answer the plugin already returns | Yes, by reading `OnConfigVerify` and `SetCapabilities` |
| Engine ↔ Reactor | `pluginCapGetter` reads the injector per OPEN | Yes, by reading `buildOpen` |
| Ze ↔ peer | OPEN wire bytes | Yes, by reading `Open.WriteTo` |

### Integration Points
- `sdk.Plugin.DeclareCapabilitiesFrom` - the one capability registration: the SDK runs the plugin's extractor at stage 2 for the declaration and at config-verify for the comparison (Q1 = b)
- `configjson.CapabilitySelectors` - the candidate's peer selectors (Q1 = b)
- The six capability plugins (`softver`, `hostname`, `role`, `llnh`, `gr`, the exabgp bridge) and the external `main_sdk.go` - each registers its extractor (a constant one for `main_sdk.go`); none writes a comparison
- `capability.PackOptionalParams` - the one Optional Parameters packer, over typed parameters, for seven callers (Q4, Q5 "a")
- `capability.ParseFromOptionalParams` - the one Optional Parameters reader, now also for rr, rs and the exabgp mock (AC-1r)

### Architectural Verification
| Check | Holds? | Evidence |
|-------|--------|----------|
| No bypassed layers | Yes (design) | capabilities still reach the OPEN only through the injector and `buildOpen` |
| No unintended coupling | Yes (design) | the reactor does not learn plugin capability semantics; plugins still own their declarations |
| No duplicated functionality | Yes (design) | one Optional Parameters packer for all seven writers (Q4, Q5 "a"); one envelope writer, `message.Open.WriteTo`; one Optional Parameters reader, `ParseFromOptionalParams` (AC-1r); one capability extractor per plugin, run by the SDK for both the stage-2 declaration and the verify comparison, so the two cannot be computed differently, and one declaration path (`SetCapabilities` deleted) (Q1 = b) |
| Zero-copy preserved | Yes (design) | `buildOptionalParams` allocates once today (the rest of `buildOpen` allocates its `caps`/`otherCaps` slices, the `ASN4` capability, the `seen` map and the `Open`, unchanged here); the packer keeps that one allocation, on a cold path, because each capability writes itself in place (`WriteTo` at an offset) rather than handing the packer bytes to copy, and a parameter is a concrete struct (no interface value, which would escape to the heap and make the reactor's per-OPEN parameter list a second allocation, round-3 review ISSUE-1); `TestPackOptionalParams` asserts 1 allocation with the parameter list built inside the measured function, as the reactor builds it |
| Registration over hardcoding, outbound | Yes (design) | each plugin registers its own extractor with the SDK; no per-plugin case in the engine or the SDK |
| Registration over hardcoding, inbound | Yes (design) | the SDK reads the plugin's own name, its registered extractor and the stage-3 set; the leaf names live in each plugin beside its code, no central list learns a plugin or a capability |
| Module tiers | Yes (design) | `internal/core/bgp/capability` gains no import; `internal/analyze` gains one `internal/core` import; `./le arch tier check` names no touched package (it exits 2 today on unrelated violations) |

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| Session start with a capability set of 254 octets | → | `buildOpen` → `capability.PackOptionalParams` → `Open.WriteTo` | `test/encode/open-optional-params-254.ci` (new: ze-peer `inspect-open-message` on the exact extended OPEN hex) |
| SIGHUP with a changed softver `encoding` | → | reload verify → SDK config-verify → softver's registered extractor → comparison with the stage-3 declaration → refusal | `test/reload/reload-softver-encoding-refused.ci` (new; model `test/reload/bgp-bfd-disable-refused-live.ci`: the reload is refused naming `bgp-softver`, the peer and `software-version`, and the session stays up on one TCP connection) |
| SIGHUP adding a peer with `software-version` | → | same path, new selector | `test/reload/reload-add-peer-softver-refused.ci` (new) |
| le test peer answers Ze's OPEN with NOTIFICATION 2/0 | → | `handleNotification` WARN line | `test/plugin/open-refused-notification-warn.ci` (new: `expect=stderr` on the WARN) |
| le test peer sends an RFC 9072 extended OPEN (two `add-capability` lines push the parameters above 255 octets) to a ze running the route reflector with ipv6/unicast configured; the operator runs `show rr peers` | → | structured OPEN event → rr `handleStructuredOpen` → `ParseFromOptionalParams` → `peer.Families` → `show rr peers` → `forwardShowRRPeers` (`rr/cmd_show.go`) → rr `handleCommand` → `peerStatus` → the row's `families` field | `test/plugin/rr-extended-open-families.ci` (new: the peer's row lists exactly `ipv6/unicast`) |
| the same extended OPEN to a ze running the route server; the operator runs `show bgp rs peers` | → | rs `parseStructuredOpen` → `ParseFromOptionalParams` → `handleOpen` → `peer.Families` → `show bgp rs peers` → rs `handleCommand` → `peerStatus` → the row's `families` field | `test/plugin/rs-extended-open-families.ci` (new: the peer's row lists exactly `ipv6/unicast`) |

Why these assertions (round-5 review, B1 and I1, re-read 2026-10-14). The
observable is what each plugin recorded, read through the operator's own
command, so it depends on no replay order and no forwarding path. Round 4 chose
behaviour the families drive, because `show rr peers` and `show bgp rs peers`
returned only `address`, `remote.as` and `up` (`rr.go::peerStatus`,
`rs/server_handlers.go::peerStatus`). Round 5 showed both choices could not
discriminate: rr's `replayForPeer` calls `sendEOR` only when `replayed > 0`, so
a single session gets no rr End-of-RIB before or after the fix (B1); rs forwards
through `reactorForwardRS` when `rs-fast-path` is on, bypassing
`SupportsFamily`, and a source UPDATE that reaches ze before the receiver's
peer-up is delivered by the adj-rib-in replay, which has no `SupportsFamily`
filter (I1). Forcing those orders needed a second session, a reconnect and the
`manual-eor` marker window (I2). Each peer row therefore gains a read-only
`families` field (see "Recorded-families field"), a CLI output change under
`ai/rules/cli.md` (owner question Q6). The red at HEAD (round-6 review, I1):
`peerStatus` in both plugins writes only `address`, `remote` and `up`, so the
row carries no `families` key; the observer keeps polling a row that is up
without the field, exhausts its poll and fails naming the absent field. The
discriminating red is recorded later, in Implementation Step 2, after the AC-1s
key lands and before the reader fix: rr and rs misframe the extended OPEN, find
no Multiprotocol capability and record the ipv4/unicast default, so the row
lists `ipv4/unicast`; the Deliverables row records it by restoring the
one-octet walk. After the fix the row lists `ipv6/unicast`: green. `behavior { manual-eor true }`, the second session and `rs-fast-path` are
no longer part of either test.

How ze-peer produces the extended OPEN (round-5 review, N2, read at the
producers 2026-10-14): ze-peer mirrors ze's OPEN, so its families come from ze's
family config (`ipv6/unicast` only, so ze-peer's Multiprotocol capability names
it), not from `add-capability`. Each `add-capability` line (value at most 255
octets, `parseOptionConfig`) becomes its own type-2 parameter (`addedParams`,
`internal/test/peer/open.go`), and `encodeOpen` switches to the RFC 9072 form
when the parameter total exceeds 255 octets. Two `add-capability` lines of 200
octets each, with codes no ze capability decodes (chosen at implementation from
the registered codes), give several type-2 parameters above 255 octets; RFC 5492
Section 3 has ze ignore the unknown codes. Whether today's one-octet walk reads
no Multiprotocol capability from those exact bytes (round-5 review, N3) is
settled by the recorded red the Deliverables row requires.

How each test ends: one ze-peer session in check mode with
`option=linger:value=true` and no expectation, so it holds the session open. A
native observer fixture, `plugin/peer-families-show` with argument `rr` or `rs`
(model: `plugin/vpn-withdrawal-inventory`, `register_vpn_withdrawal.go`, which
polls the same two commands with `Poll`), polls the plugin's peer command until
the row for 127.0.0.1 has `up` true and a `families` field, then prints
`PASS: <command> families ipv6/unicast` when that field is exactly
`["ipv6/unicast"]`, and fails naming the field it read otherwise. The `.ci`
awaits that PASS line (`await=`) and rejects the observer's failure line. The
test ends from the fixture, as the model does (round-6 review, N1): a lingering
peer never exits by itself (ci-format.md "Await"), so on completion the
observer dispatches `request shutdown` (`runPlugin13(..., shutdown=true)`,
`plugin_fixture_13.go`) and the `.ci` expects `expect=exit:code=0`. ze's
config attaches the rr or rs process with `receive [ ... state open-received ... ]`
(model `test/draft/plugin/vpn-withdrawal-inventory-rr-ipv4.ci`), so the plugin
receives the OPEN (round-6 review, N2). A row
that is up without the field keeps the poll going, so the result does not rest
on the event order argued under "Recorded-families field".

Q2 = deviation (answered 2026-10-13) leaves no config refusal for a peer-level `router-id`, so no wiring row is owed for it.

## Acceptance Criteria

AC-1 to AC-4 keep the skeleton's intent. AC-2's either/or is resolved by Q1
(owner, 2026-10-13: "b", refuse); AC-3 is narrowed from "WARN or INFO" to WARN
because the default level is WARN.

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1q | The reactor, the chaos peer, the perf sender, the BMP Loc-RIB OPEN, `ze analyze`'s OPEN, and the test peer's active-role and passive-role OPENs (Q4, Q5 "a") | Each builds its Optional Parameters through `capability.PackOptionalParams` over typed parameters; reactor `buildOptionalParams` and `maxExtendedParamLen`, chaos `buildOptionalParams`, perf `packOptionalParams`, the inline packing in `fabricateLocRIBOpen`, `bgpBuildOpen` and `buildActiveOpen`, and `encodeOpen`'s own framing and envelope no longer exist; a tree grep finds no Optional Parameter writer outside the packer and no extended-envelope writer outside `message/open.go`, with no exception; the BMP Loc-RIB, `ze analyze` and both test peer roles' OPEN bytes are unchanged; no `./le arch tier check` violation names `internal/core/bgp/capability` or a touched caller package |
| AC-1p | The test peer's passive-role OPEN (`encodeOpen`) over every existing `.ci`, and over a parameter list of several type-2 parameters plus a non-capability raw parameter, below and above 255 octets of parameters | Bytes identical to today's `encodeOpen` output for the same list: the order, the several type-2 parameters and the raw parameter are kept; base form while the sum of 2 + value length is at most 255, extended (two-octet lengths, 255/255 envelope written by `message.Open.WriteTo`) above; an OPEN above 4096 octets is still refused: with today's 4096 error while the parameters fit the packer (Extended Opt. Parm. Length at most 65535), and with the packer's size error above 65535 octets of parameters, because the packer now runs before the 4096 check where today `encodeOpen` sums the total and answers the 4096 error at any size |
| AC-1r | rr, rs and the exabgp interop mock receive an RFC 9072 extended OPEN (several type-2 parameters, two-octet lengths, above 255 octets) and a base-form OPEN | Each reads every capability through `capability.ParseFromOptionalParams` with the OPEN's own `ExtendedParams`: rr records the peer's families and ASN4, rs its capability list and ASN4, the mock its ADD-PATH modes, identically for both framings; an OPEN `ParseFromOptionalParams` refuses (a non-type-2 parameter, an empty type-2 parameter, one malformed type-2 parameter among several) and an OPEN `message.UnpackOpen` refuses leave rr and rs with nothing recorded for it and one WARN record with the text of "The warning" (problem, why, next step; peer and decoder error as attributes), never the ipv4/unicast default; the open-coded parameter walks in `rr.go` and `server.go` and the `openBody[9]` length read in `exabgp_server.go` no longer exist |
| AC-1s | `show rr peers` and `show bgp rs peers` while a peer that sent an RFC 9072 extended OPEN advertising ipv6/unicast is up (Q6 = a) | The peer's row carries `families` listing exactly `ipv6/unicast`, the set the plugin recorded; a row whose peer is down, or for which no OPEN is recorded, carries no `families` key; the other keys (`address`, `remote.as`, `up`) are unchanged ("Recorded-families field") |
| AC-1x | capabilities totalling 65533 octets or more | The packer returns an error naming the size and the limit, and `buildOpen` returns it; no OPEN with missing capabilities is sent |
| AC-1 | `buildOpen` with capabilities totalling 254 or 255 octets (and 256) | The OPEN carries Non-Ext OP Len 255, Non-Ext OP Type 255, Extended Opt. Parm. Length = 3 + capTotal, and one type-2 parameter whose two-octet Parameter Length = capTotal; a conforming decode (`message.UnpackOpen` then `capability.ParseFromOptionalParams`) returns every capability unchanged |
| AC-1n | capabilities totalling 253 octets (and 0, 1) | The OPEN uses the RFC 4271 form: Opt Parm Len = 2 + capTotal (255 at 253), no 255/255 marker, one-octet Parameter Length; bytes identical to today |
| AC-1b | chaos test peer with a capability set whose parameters exceed 255 octets | The chaos peer's OPEN round-trips through `message.UnpackOpen` and `capability.ParseFromOptionalParams` with every capability intact (its parameters use the RFC 9072 two-octet framing whenever the envelope is extended) |
| AC-2 | SIGHUP that changes `capability software-version encoding` on a running peer (legacy → draft) | The reload is refused with the text of "D2 refusal design": it names the peer, `software-version`, plugin `bgp-softver`, that the change cannot apply while ze runs because ze would keep sending the old value in its OPEN, and `systemctl restart ze` on its own line, followed by the unsupervised route (`ze signal stop`, then `ze start` or `ze start <config-file>`) and the appliance route (`request reboot` in the ze CLI, or `ze signal reboot` from a shell); it never names `ze signal restart` or a bare `reboot`. Nothing is applied: the session is not restarted and keeps its startup OPEN |
| AC-2b | SIGHUP that adds a new peer with `software-version` (own or inherited from its group) | Refused the same way, naming the new peer |
| AC-2c | SIGHUP that removes `software-version` from a peer that stays configured | Refused the same way |
| AC-2d | The same changes for the `hostname`, `role`, `llnh` and `gr` capabilities, including a session family added on a `gr` peer whose `graceful-restart` container has no `family` list (the GR payload follows the session families) | Each is refused naming its plugin and its `capability` leaf |
| AC-2g | A session family added on a `gr` peer with an explicit `graceful-restart family` list that does not name it | Accepted: the GR payload is the explicit list and does not change; the peer restarts as today for the capability-block change |
| AC-2f | SIGHUP that changes the exabgp bridge's `route-refresh` or `add-path`, or edits the bridge's families while `add-path` is set (the add-path payload is built from those families) | Refused naming `every peer`, plugin `exabgp-bridge` and the leaf (`add-path` for the family edit) |
| AC-2e | A reload that leaves every plugin-declared capability unchanged, one that only removes a peer, one that adds a peer with no plugin capability, and one that adds a peer while the exabgp bridge declares an unchanged all-peer set | Each is accepted; no refusal line appears |
| AC-2h | A runtime `configure` redelivery (the engine's resend after a refused removal, `startup_removal.go`) to a capability plugin, after an accepted reload removed a peer that carries a plugin capability | The stored stage-3 declaration is unchanged (the extractor runs for the declaration only on the stage-2 path inside `Run`), so a later reload re-adding that peer with the same capability is accepted, as the injector still holds its declaration |
| AC-2i | The SDK after this spec | `SetCapabilities` no longer exists; every declaring plugin, the external `main_sdk.go` included (a constant extractor), registers through `DeclareCapabilitiesFrom`; the plugin's own verify handler answers before the comparison whichever of `OnConfigVerify` and the extractor was registered first |
| AC-3 | A NOTIFICATION received in OpenSent or OpenConfirm (Ze's OPEN refused) | Exactly one WARN line per received NOTIFICATION naming the peer, the FSM state, the code and subcode as names and numbers, and the RFC 8203 message when present; visible with no `ze.log` override |
| AC-3n | A NOTIFICATION received in Established | The same one WARN line (one line per NOTIFICATION, whatever the state); the existing INFO "peer shutdown communication" line is folded into it, not duplicated (no-layering) |
| AC-4 | The `weak` verdicts of RFC9072-2-1 and RFC6286-2.1-1, after this spec's producer fix | RFC9072-2-1 reaches `enforced`: a tagged test proves the quoted sentence, and an agent that did not write that test re-judges it with `./le rfc audit-stamp ... mode rejudge`. RFC6286-2.1-1 carries the deviation record (`{partial}`, reason, not counted conformant), and its verdict is re-judged against that record. Moved here from the "Blocked by" table of the BGP child spec-rfc-verdict-fix-bgp, closed 2026-10-21 (P-3 of `plan/pre-release/spec-rfc-verdict-test-fix-pass.md`, 2026-09-28) |
| AC-5 | `rfc/short/rfc6286.md` RFC6286-2.1-1 annotation and the `docs/features/rfc-status.md` RFC 6286 row | Neither claims the identifier is the same for every peer while the per-peer override exists; done FIRST, before any code, because `ai/rules/rfc-compliance.md` does not let a false claim wait |
| AC-6 | A config with a peer-level `router-id` (Q2 = deviation) | Accepted as today; the YANG description and `docs/guide/configuration.md` say the override departs from RFC 6286 Section 2.1 |
| AC-7 | `plan/journal/unwired-feature.md` (2026-09-27), `plan/journal/silent-fall-through.md` (2026-09-27) and `plan/journal/helper-bypassed-by-an-open-coded-copy.md` (2026-10-14, received-OPEN readers) rows | Marked fixed with the commit, in the closing commit |

## End-to-End User Stories

| # | User does | Path through system | Test proving it works |
|---|-----------|--------------------|-----------------------|
| 1 | Configures enough families and capabilities that the set is 254 octets, peers with FRR | config → plugins stage 3 → `buildOpen` → `capability.PackOptionalParams` → `Open.WriteTo` → FRR | `bgp-open-extended-params-boundary-frr` |
| 2 | Changes `software-version encoding` and sends SIGHUP | transaction verify → SDK config-verify → softver's registered extractor → comparison → refusal naming the restart; the session stays up | `reload-softver-encoding-refused.ci` |
| 3 | Peers with a speaker that refuses Ze's OPEN, reads the log | wire NOTIFICATION → `handleNotification` → WARN | `open-refused-notification-warn.ci`, `frr-software-version` (log assertion added) |
| 4 | Peers with a speaker whose OPEN uses the RFC 9072 extended form, runs `show rr peers` or `show bgp rs peers` | wire OPEN → structured OPEN event → rr / rs → `ParseFromOptionalParams` → `peerStatus` → the row's `families` | `rr-extended-open-families.ci`, `rs-extended-open-families.ci` |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestRFC9072OptionalParamsBoundary` (table: capTotal 0, 1, 253, 254, 255, 256, 300) | `internal/component/bgp/reactor/rfc9072_optional_params_boundary_test.go` (new) | AC-1, AC-1n: builds the OPEN through `buildOpen` (which reaches `capability.PackOptionalParams`) plus `Open.WriteTo`, reads the envelope octets and walks the parameters with the framing the envelope announces; tagged `RFC9072-2-1 positive` (254, 255, 256 extended with two-octet Parameter Length) and `RFC9072-2-1 negative` (253 and below carry no marker and one-octet length; the tag quotes the Section 2 SHOULD sentence, see the audit table below) | |
| `TestPackOptionalParams` (table: empty, one capability, several, capTotal 253, 254, 65532, 65533; several type-2 parameters whose sum of 2 + length is 255 and 256; a raw non-capability parameter beside a type-2 one, below and above 255) | `internal/core/bgp/capability/optional_params_test.go` (new) | the packer's own framing and flag over typed parameters, the type-2 cases round-tripped through `ParseFromOptionalParams` and the raw case read back octet by octet, the one-allocation property on the reactor's real shape (`testing.AllocsPerRun` is 1 with the parameter list, one type-2 parameter over a capability list, built inside the measured closure per call, as `Session.buildOpen` builds it per OPEN), the zero-value parameter written as `00 00`, and the AC-1x error, the packer's only error (no both-forms case: the two constructors cannot build one); replaces `TestBuildOptionalParams_Empty`, `_SingleCap`, `_MultipleCaps` (`reactor/session_negotiate_test.go`, untagged), which are deleted with the function they call | |
| `TestChaosPeerOpenRoundTripsAbove255` | `internal/chaos/peer/session_test.go` | AC-1b, and that the chaos peer reaches the one packer | |
| `TestPerfOpenUsesOnePacker` | `internal/perf/session_test.go` | AC-1q for perf: the OPEN round-trips through `message.UnpackOpen` and `capability.ParseFromOptionalParams`; `TestBuildOpenIPv6` reads the one-parameter form | |
| `TestAnalyzeOpenUsesOnePacker` | `internal/analyze/bgpconn_test.go` (new) | AC-1q for `ze analyze`: `bgpBuildOpen`'s bytes are unchanged and round-trip through `capability.ParseFromOptionalParams` | |
| `TestActiveOpenUsesOnePacker` | `internal/test/peer/inject_test.go` | AC-1q for the active test peer: `buildActiveOpen`'s bytes are unchanged and round-trip | |
| `TestPassiveOpenUsesOnePacker` (table: one type-2 parameter; several type-2 parameters from `addedParams`; a raw non-capability parameter; parameters above 255 octets; an OPEN above 4096 octets) | `internal/test/peer/open_test.go` | AC-1p: `encodeOpen`'s bytes equal a golden captured from today's `encodeOpen` for each list (captured before the edit), order and layout kept, extended above 255, the 4096 refusal kept (an OPEN above 4096 with parameters under 65535 octets gets the 4096 error, one above 65535 octets of parameters the packer's size error) | |
| `TestRouteReflectorReadsExtendedOpen` (base form and RFC 9072 form, several type-2 parameters; refused cases: a non-type-2 parameter, an empty type-2 parameter, one malformed type-2 parameter among several) | `internal/component/bgp/plugins/rr/rr_test.go` | AC-1r for rr: `handleStructuredOpen` records the same families and ASN4 for both framings, red today on the extended form; for each refused case, and for a body `message.UnpackOpen` refuses, `rr.peers` holds no entry for the peer (no ipv4/unicast default) and the captured logger holds one WARN record with the rr text of "The warning" (its three sentences, `peer` and `error` attributes), red today because the walk skips the parameter and stores the default and nothing is logged | |
| `TestRouteServerReadsExtendedOpen` (the same two forms and the same three refused cases) | `internal/component/bgp/plugins/rs/server_test.go` | AC-1r for rs: `parseStructuredOpen` returns the same capability list and ASN4 for both framings, red today on the extended form; for each refused case, and for a body `message.UnpackOpen` refuses, it returns nil and the captured logger holds one WARN record with the rs text of "The warning", red today because it returns an event whose empty list `handleOpen` turns into the ipv4/unicast default, and the `UnpackOpen` case returns nil with nothing logged | |
| `TestRouteReflectorPeerStatusFamilies` (rows: up with a recorded set of several families; up with the ipv4/unicast default; down with an earlier session's set; up with no OPEN recorded) | `internal/component/bgp/plugins/rr/rr_test.go` | AC-1s for rr: `peerStatus` emits `families` sorted for the first two rows and no `families` key for the last two; red today because the key does not exist | |
| `TestRouteServerPeerStatusFamilies` (the same four rows, the last with `Families` nil) | `internal/component/bgp/plugins/rs/server_test.go` | AC-1s for rs, the same assertions | |
| `TestExabgpMockReadsExtendedOpen` | `internal/le/interoplab/bgp/exabgp_server_test.go` (new) | AC-1r for the mock: `exabgpNegotiatedEncoding` reads ADD-PATH from an extended OPEN body as from a base one | |
| existing bmp Loc-RIB OPEN tests (the tagged ones are listed under "RFC-tagged tests that change") | `internal/component/bgp/plugins/bmp/` | AC-1q for BMP: every direct caller of `fabricateLocRIBOpen` handles its new error return, and the OPEN bytes stay identical | |
| `TestDeclareCapabilitiesFrom` (table over the six rows of the D2 verdict table) | `pkg/plugin/sdk/` | AC-2 to AC-2g at the comparison: changed payload, removed code on a live peer, new code, new peer refused; unchanged and peer-removed accepted; an all-peer declaration changed is refused naming `every peer`, unchanged with a peer added is accepted; the stage-2 declaration is the extractor's output; a runtime `configure` redelivery through `handleConfigure` leaves the stored declaration unchanged (AC-2h); the plugin's own verify handler answers first whether `OnConfigVerify` was called before or after the extractor was registered (AC-2i); a constant extractor (the `main_sdk.go` shape) always accepts; the text names plugin, peer, leaf, `systemctl restart ze`, `ze start` and `request reboot`, and never `ze signal restart` or a bare `reboot` | |
| `TestCapabilitySelectors` | `internal/component/bgp/configjson/` | standalone, grouped and dynamic-group-template selectors, the same strings the extractors emit | |
| `TestSoftverCapabilityExtractorRegistered` and the same for `hostname`, `role`, `llnh`, `gr` and the exabgp bridge | each plugin's `_test.go` | each plugin registers its extractor, the SDK refuses a change of its own capability and accepts an unchanged one (AC-2, AC-2d, AC-2e, AC-2f, AC-2g); for `llnh` and `gr` the existing refusal still answers first; for the bridge a family edit with `add-path` set is refused naming `add-path` | |
| `TestHandleNotificationLogsWarn` | `internal/component/bgp/reactor/session_handlers_test.go` | AC-3, AC-3n: a captured logger at WARN sees exactly one line with peer, state, code, subcode; with an RFC 8203 message the message is in that line and no second line exists | |

### Boundary Tests (numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| capTotal, one-octet form | 0-253 | 253 | N/A | 254 (must switch to RFC 9072 form) |
| capTotal, RFC 9072 form | 254-65532 (Extended Opt. Parm. Length = 3 + capTotal <= 65535) | 65532 | 253 (must stay RFC 4271 form) | 65533 (refused with an error, see R-4) |

### RFC-tagged tests that change (owner approval)

Q4 = Consolidate (owner, 2026-10-13) deletes `buildOptionalParams`, so every
test that calls it by name moves onto `capability.PackOptionalParams`, and it
gives `fabricateLocRIBOpen` an error return, so every test that calls it
directly changes body. Read at HEAD on 2026-10-13 by locating each call inside
its enclosing function and the `RFC requirement:` tags in the comment block
directly above it:

| Test unit | Tag | Call | What changes | Owed |
|-----------|-----|------|--------------|------|
| `reactor.TestRFC9072ParameterType255OnlyAsTheExtendedMarker` (`rfc9072_param_type_test.go`) | RFC9072-3-2 positive and negative | packs the peer-side parameters with `buildOptionalParams`; its VALIDATES comment names it | call and comment move to the packer | owner approval (given), fresh discrimination record |
| `reactor.TestMRTMigrationEpochUsesActualLocalOPEN` (`rfc8050_migration_epoch_test.go`) | RFC8050-x-4 positive | packs a peer OPEN | call moves | owner approval (given), fresh discrimination record |
| `reactor.TestOpenAdvertisesVPNv6Capability` (`session_negotiate_test.go`) | RFC4659-3.4-1 positive | packs `ps.Capabilities` to decode them back | call moves. Not in the earlier list: found 2026-10-13 | owner approval (given, "approve all RFC-tagged test changes"), fresh discrimination record |
| `reactor.TestOpenAdvertisedASReadsAS4Capability` (`rfc6286_routerid_unique_test.go`) | none on this function | packs a peer OPEN | call moves | nothing beyond the edit |
| the peer-OPEN helper in `session_first_as_test.go` and `establishNegotiatedSession` (`peer_settings_negotiation_test.go`) | none (both files carry no tag) | pack the peer's capabilities | call moves | nothing beyond the edit |
| `TestBuildOptionalParams_Empty`, `_SingleCap`, `_MultipleCaps` (`session_negotiate_test.go`) | none | test the deleted function | deleted; `TestPackOptionalParams` covers them in the packer's package | nothing beyond the edit |
| `bmp.TestFabricatedLocRIBOpenCarriesOnlyCapabilitiesTheRouteMonitoringUses` (`rfc9069_verdict_test.go`) | RFC9069-5.2-1 positive | calls `fabricateLocRIBOpen` | handles the new error return | owner approval (given), fresh discrimination record |
| `bmp.TestLocRIBIPv4RouteWithIPv6NextHopIsBackedByExtendedNextHop` (`rfc9069_verdict_test.go`) | RFC9069-5.2-1 positive | calls `fabricateLocRIBOpen` | handles the new error return | owner approval (given), fresh discrimination record |
| `bmp.TestFabricatedLocRIBOpenCarriesTheRequiredCapabilities` (`rfc9069_bmp_locrib_test.go`) | RFC9069-5.2-1 positive | calls `fabricateLocRIBOpen` | handles the new error return | owner approval (given); it carries no record today, and a changed tagged unit owes one |
| `bmp.TestReceiverRecordsThePeerUpAddressFamilies` (`rfc9069_test.go`) | RFC9069-6.1.1-1 positive | calls `fabricateLocRIBOpen` | handles the new error return | owner approval (given), fresh discrimination record |
| `TestFabricatedOpenUsesASTransForFourByteASN` (`rfc9069_bmp_locrib_test.go`), `TestOpenMultiprotocolFamiliesRefusesAMalformedOPEN` (`rfc9069_test.go`), `TestStateLocRIBIdentityChangeKeepsReportsSeparate` (`state_test.go`), and the helpers `locRIBPeerUpSeeds` (`fuzz_test.go`), `replayPlugin` (`rfc7854_replay_test.go`), `addOpeningPeer` (`rfc7854_session_opening_test.go`) | none directly above | call `fabricateLocRIBOpen` | handle the new error return | nothing beyond the edit |

The existing tagged envelope tests (`TestOpenPackExtendedParams`,
`TestTheExtendedEnvelopeAndItsParametersAgree`) do not call the packer and stay
as they are. Q2 = deviation changes no tagged test (audit below).

#### Discrimination records Q4 stales

A record fingerprints its unit (`unit-sha`) and its producer (`producer-sha`),
and a stale record "becomes a violation at the commit that carries the edit"
(`docs/contributing/rfc-conformance-gates.md`). Q4 changes the body of
`Session.buildOpen` (the packer call) and of `fabricateLocRIBOpen` (inline
packing removed, error return added), and deletes `buildOptionalParams`. Read
from `rfc/discrimination/*.json` on 2026-10-13 (fields `rid`, `polarity`,
`unit`, `producer`; every record below has route `revert`):

| # | Record (`rid`, polarity) | Unit | Producer today | Stale because |
|---|--------------------------|------|----------------|---------------|
| 1 | DRAFT-IETF-IDR-BGP-BFD-STRICT-MODE-6-1 positive | `reactor_b_open_caps_test.go::TestBFDStrictCapabilityInSentOpen` | `session_negotiate.go::buildOpen` | producer body |
| 2 | DRAFT-IETF-IDR-BGP-BFD-STRICT-MODE-6-1 negative | `reactor_b_open_caps_test.go::TestBFDStrictCapabilityInSentOpen` | `session_negotiate.go::buildOpen` | producer body |
| 3 | RFC2918-2-1 positive | `reactor_b_open_caps_test.go::TestRFC2918RouteRefreshCapabilityOctetsInSentOpen` | `session_negotiate.go::buildOpen` | producer body |
| 4 | RFC7705-4.2-3 negative | `rfc7705_session_as_migration_test.go::TestMigrationOpenCarriesResolvedASN` | `session_negotiate.go::buildOpen` | producer body |
| 5 | RFC9069-5.2-1 positive | `rfc9069_verdict_test.go::TestFabricatedLocRIBOpenCarriesOnlyCapabilitiesTheRouteMonitoringUses` | `bmp_locrib.go::fabricateLocRIBOpen` | producer body and unit body |
| 6 | RFC9069-5.2-1 positive | `rfc9069_verdict_test.go::TestLocRIBIPv4RouteWithIPv6NextHopIsBackedByExtendedNextHop` | `bmp_locrib.go::fabricateLocRIBOpen` | producer body and unit body |
| 7 | RFC9069-6.1.1-2 positive | `rfc9069_verdict_test.go::TestLocRIBPeerUpOpenIndicatesTheAddressFamilies` | `bmp_locrib.go::fabricateLocRIBOpen` | producer body (the unit does not call it directly) |
| 8 | RFC9072-3-2 positive | `rfc9072_param_type_test.go::TestRFC9072ParameterType255OnlyAsTheExtendedMarker` | `session_negotiate.go::buildOptionalParams` | producer deleted and unit body; the new record names `capability.PackOptionalParams` |
| 9 | RFC9072-3-2 negative | the same unit | `session_negotiate.go::buildOptionalParams` | the same |
| 10 | RFC8050-x-4 positive | `rfc8050_migration_epoch_test.go::TestMRTMigrationEpochUsesActualLocalOPEN` | (its own record) | unit body |
| 11 | RFC9069-6.1.1-1 positive | `rfc9069_test.go::TestReceiverRecordsThePeerUpAddressFamilies` | (its own record) | unit body |

Two changed tagged units carry no record today and owe a first one:
`TestOpenAdvertisesVPNv6Capability` (RFC4659-3.4-1) and
`TestFabricatedLocRIBOpenCarriesTheRequiredCapabilities` (RFC9069-5.2-1). Each
row is re-recorded with `./le rfc discriminate-record id <rid> polarity
<polarity> unit <unit> route revert producer <producer>` in the commit that
carries the edit. Records 1 to 7 are the seven producer-drift records the
review of 2026-10-13 named (ISSUE-1); rows 8 to 11 and the two first records
belong to the seven tagged units whose bodies change.

#### Independent audit of the tagged tests (2026-10-13, Q2 answered: deviation)

Source: `tmp/session/2026-10-12-0c3648c8-b9d7-45d8-b3f0-2473bceefb25/scratch/rfc-tagged-test-compliance-review.md`, section 11.
Re-checked against `config_test.go` at HEAD, `rfc/full/rfc6286.txt` Section 2.1 and `rfc/full/rfc9072.txt` Section 2; agreed on both rows.

| Tagged unit | Audit verdict | What the change owes |
|-------------|---------------|----------------------|
| `config_test.go::TestParsePeerFromTreeInvalid`, case `zero_router_id_rejected` (RFC6286-2.1-1 positive) | COMPLIANT, unchanged under the deviation | Its claim is limited to "non-zero" ("The BGP Identifier is a 4-octet, unsigned, non-zero integer", RFC 6286 Section 2.1), which the body proves by refusing a peer-level `0.0.0.0`. Under the deviation it is the Tested scope of the `{partial}` row in `rfc/short/rfc6286.md`, "the same for every local interface and every BGP peer" is the declared deviation gap, and the row contributes nothing to the conformance count. Body and tag unchanged |
| new `TestRFC9072OptionalParamsBoundary` (RFC9072-2-1 positive and negative) | COMPLIANT once the negative's prose is fixed | The boundary at capTotal 254 is right: the field length is 2 + capTotal, and the MUST binds when it "does exceed 255". The negative (capTotal 253 and below stay in the base form) is NOT proved by the MUST sentence: the same paragraph says "an implementation MUST accept an OPEN message that uses the encoding of this specification even if the length of the Optional Parameters is 255 or less", so the extended form at or below 255 is permitted. The base form there is the SHOULD sentence plus Ze's choice. The negative tag line must quote it: RFC 9072 Section 2, "In the event that the length of the Optional Parameters in the BGP OPEN message does not exceed 255, the encodings of the base BGP specification [RFC4271] SHOULD be used without alteration." That sentence is enrolled as its own row, RFC9072-2-7 [SHOULD]; the same 253 case may also carry an `RFC9072-2-7 positive` tag, with its own discrimination record, since it proves that row directly |

No existing tagged test changes under Q1 = b or Q2 = deviation: confirmed. Q4 = Consolidate changes the seven tagged units listed above (three reactor, four bmp).

#### Owner approval commands

Approved by the owner on 2026-10-13: "approve all RFC-tagged test changes". Q4 is answered "Consolidate", so each of the seven tagged units whose body changes needs a command: the three that call `buildOptionalParams` and the four bmp units that call `fabricateLocRIBOpen`. A unit whose body does not change (records 1 to 4 and 7 above) owes a fresh record and no approval. The gate reads approvals from the committing session's own file (`tmp/commit-rfc-approved-<session>.md`), and a row is used up by the commit that carries its `RFC-approved:` trailer, so the owner runs these at the prompt of the session that commits the consolidation. A unit is `<package directory>.<top-level test function>`.

7 commands:

```
./le rfc approve unit reactor.TestRFC9072ParameterType255OnlyAsTheExtendedMarker reason 'Thomas, 2026-10-13: "approve all RFC-tagged test changes" (spec-bgp-open-session-rfc-defects, Q4 "Consolidate": buildOptionalParams call moves to capability.PackOptionalParams)'
./le rfc approve unit reactor.TestMRTMigrationEpochUsesActualLocalOPEN reason 'Thomas, 2026-10-13: "approve all RFC-tagged test changes" (spec-bgp-open-session-rfc-defects, Q4 "Consolidate": buildOptionalParams call moves to capability.PackOptionalParams)'
./le rfc approve unit reactor.TestOpenAdvertisesVPNv6Capability reason 'Thomas, 2026-10-13: "approve all RFC-tagged test changes" (spec-bgp-open-session-rfc-defects, Q4 "Consolidate": buildOptionalParams call moves to capability.PackOptionalParams)'
./le rfc approve unit bmp.TestFabricatedLocRIBOpenCarriesOnlyCapabilitiesTheRouteMonitoringUses reason 'Thomas, 2026-10-13: "approve all RFC-tagged test changes" (spec-bgp-open-session-rfc-defects, Q4 "Consolidate": fabricateLocRIBOpen calls capability.PackOptionalParams and returns its error)'
./le rfc approve unit bmp.TestLocRIBIPv4RouteWithIPv6NextHopIsBackedByExtendedNextHop reason 'Thomas, 2026-10-13: "approve all RFC-tagged test changes" (spec-bgp-open-session-rfc-defects, Q4 "Consolidate": fabricateLocRIBOpen calls capability.PackOptionalParams and returns its error)'
./le rfc approve unit bmp.TestFabricatedLocRIBOpenCarriesTheRequiredCapabilities reason 'Thomas, 2026-10-13: "approve all RFC-tagged test changes" (spec-bgp-open-session-rfc-defects, Q4 "Consolidate": fabricateLocRIBOpen calls capability.PackOptionalParams and returns its error)'
./le rfc approve unit bmp.TestReceiverRecordsThePeerUpAddressFamilies reason 'Thomas, 2026-10-13: "approve all RFC-tagged test changes" (spec-bgp-open-session-rfc-defects, Q4 "Consolidate": fabricateLocRIBOpen calls capability.PackOptionalParams and returns its error)'
```

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `open-optional-params-254` | `test/encode/open-optional-params-254.ci` | Ze's OPEN with a 254-octet capability set is the exact RFC 9072 form | |
| `reload-softver-encoding-refused` | `test/reload/` | legacy → draft on SIGHUP is refused with the D2 text; one TCP connection for the whole test, so the session never restarted (AC-2) | |
| `reload-add-peer-softver-refused` | `test/reload/` | AC-2b | |
| `reload-softver-removed-refused` | `test/reload/` | AC-2c | |
| `reload-gr-family-change-refused` | `test/reload/` | AC-2d through a second plugin: a session family added on a `gr` peer with no `graceful-restart family` list is refused; `hostname`, `role` and `llnh` are covered by their unit tests over the same SDK registration | |
| `reload-gr-explicit-family-list-accepted` | `test/reload/` | AC-2g: the same edit on a `gr` peer with an explicit `graceful-restart family` list is accepted and the peer restarts as today | |
| `reload-exabgp-bridge-capability-refused` | `test/reload/` | AC-2f: a changed `route-refresh` under `exabgp bridge` is refused naming `every peer` and `exabgp-bridge` | |
| `reload-capability-unchanged-accepted` | `test/reload/` | AC-2e: a reload that changes a non-capability leaf on a `software-version` peer and removes another peer is accepted | |

-> Decision (2026-10-14): the seven reload `.ci` above exist and pass at 343d23709b plus the working tree (`./le test bgp reload --pattern <name>`, logs in the session scratch `open-impl/ci-*.log`). A-2 validated: the live-stages fixture asserts the refusal text on stderr. The gr cases need `plugin { internal gr { use bgp-gr } }` and `attach process gr`; the explicit-list case logs its restart reason only at `ze.log.bgp.reactor` debug, and both OPENs carry the same GR payload `4006007800010100`. The red phase is OWED: disabling the SDK comparison in `composeCapabilityVerify` was refused by the harness's permission classifier, so no red is recorded for the five refused cases; the two accepted cases owe a red from a comparison that refuses every change.
-> Constraint (2026-10-14, Phase 4 part 4): the red phase for the seven capability reload `.ci` and for the SDK/plugin units (`TestDeclareCapabilitiesFrom`, `TestCapabilitySelectors`, each plugin's `CheckCapabilityReload` test) is OWED and needs an owner decision. `./le rfc discriminate-record` cannot take it (it resolves only RFC-tagged units, and D2 carries no tag), and no break of the SDK comparison is attempted, by edit, overlay or otherwise, without the owner. Two routes: (a) the owner allows a one-line temporary break of the SDK config-verify comparison (`composeCapabilityVerify` / `verifyCandidate`, `pkg/plugin/sdk/`), which reddens the five refused cases with the comparison disabled and the two accepted cases with a comparison that refuses every change; (b) run the five refused-case `.ci` against a ze built from `e6e1ee5a6c~1` (pre-D2, accepts every change), which reddens them without touching the tree but cannot redden the two accepted cases.
-> Constraint (2026-10-14, Phase 5-7 agent): the owner chose route (a) (Thomas, 2026-10-14: "a"). The harness's auto-mode classifier refused the first step, a read of `pkg/plugin/sdk/capability_declaration.go` ("Security Weaken"), and then a read of `internal/le/interoplab/bgp/check_softver.go` ("Security Test Removal"); per the main thread's instruction the agent did not retry. The D2 red stays OWED: route (a) needs the owner to allow the edit in the harness permission settings, or a run from a session the classifier admits. The file was never modified.
-> Constraint (2026-10-14, Phase 6-7 continuation): the overlay route (a scratch ze built with a Go `-overlay` replacing only `capabilitySource.verifyCandidate` in `pkg/plugin/sdk/capability_declaration.go` with a broken copy, the tree untouched; recipe from the SR Policy spec's state file) was stopped at its first step: a `sed -n` read of that function was denied by the auto-mode classifier ("Auto-Mode Bypass"). Per the main thread, the agent stopped there and did not attempt route (b) either, since it serves the same red after a denial. The runner does take another binary (`ZE_BIN=<path> LE_TEST_NO_BUILD=1`, `internal/test/runner/runner.go`), so route (b) is technically possible here. The D2 red stays OWED and needs the owner to admit the action in the harness permission settings.
-> Decision (2026-10-21, D2 red by route (b)): the seven reload `.ci` were run at HEAD `539a98a861` and against a ze built from `d21d5f4874` (`e6e1ee5a6c~1`, pre-D2), exported with `git archive` into session scratch and built with the runner's test tag set (`zetest,ze_core,ze_distro,ze_setup` plus every `ze_` gate of that tree's `feature-gates.txt`); no producer was read, edited or overlaid. Red run: `ZE_BIN=<scratch>/src/bin/ze LE_TEST_NO_BUILD=1 ./le test bgp reload -v <seven names>`; the log shows every daemon step ran that binary (24 references, none to the session's HEAD build). HEAD: 7/7 PASS. Pre-D2: 5 FAIL, 2 PASS. Each refused case fails at step 3 on the missing refusal line, and the three single-connection cases also fail the peer exchange with Cease subcode 6 (Other Configuration Change) where subcode 2 (Administrative Shutdown) was expected, so the pre-D2 daemon accepted the reload and bounced the session, the defect the journal row reported:
  - `reload-softver-encoding-refused`: output does not contain "peer peer1: capability software-version cannot change while ze runs (plugin bgp-softver)."; NOTIFICATION subcode expected=2, got=6
  - `reload-softver-removed-refused`: same refusal line missing; subcode expected=2, got=6
  - `reload-gr-family-change-refused`: output does not contain "peer peer1: capability graceful-restart cannot change while ze runs (plugin bgp-gr)."; subcode expected=2, got=6
  - `reload-add-peer-softver-refused`: output does not contain "peer peer2: capability software-version cannot change while ze runs (plugin bgp-softver)."
  - `reload-exabgp-bridge-capability-refused`: output does not contain "every peer: capability route-refresh cannot change while ze runs (plugin exabgp-bridge)."
  - `reload-gr-explicit-family-list-accepted` and `reload-capability-unchanged-accepted`: PASS pre-D2, as predicted above. Route (b) cannot redden the acceptance polarity: a pre-D2 daemon accepts every reload. Their red (a comparison that refuses every change) is still OWED and needs route (a), which the harness has refused twice.
-> Decision (2026-10-14, Phase 6): `test/plugin/open-refused-notification-warn.ci` is written (le test peer answers the OPEN with a raw 2/0 by `action=send`, `await=` the line `then=stop`, `expect=stderr:pattern=` the whole WARN). Its first run failed on the config (`prefix maximum is mandatory`, fixed); the second could not run: building ze failed in `internal/component/gnmi` (another session's uncommitted `pathToSegments` / `KeyLeafDefault` change), so the `.ci` has no green and no red yet. Red for the unit is recorded: `TestHandleNotificationLogsWarn` failed on all three subtests before `logNotificationReceived` existed (log `open-impl` scratch `d3red.log`) and passes after (`d3green.log`; six other reactor tests fail there on `family ipv4/mpls-vpn has no decoder plugin`, untouched by D3). A-4 stays unvalidated until the `.ci` runs.
| `open-refused-notification-warn` | `test/plugin/` | le test peer answers the OPEN with 2/0; `expect=stderr:contains=` the WARN with peer, state, code, subcode; no `ze.log` override | |
| `rr-extended-open-families` | `test/draft/plugin/` while iterated, then `test/plugin/` | AC-1r and AC-1s through the real entry: one ze-peer session whose two `add-capability` lines push its OPEN above 255 octets of parameters (RFC 9072 form, several type-2 parameters), ze configured with ipv6/unicast only and the route reflector loaded; the `plugin/peer-families-show rr` observer polls `show rr peers` until the peer's row is up with a `families` field and passes only when it is exactly `["ipv6/unicast"]` (see "Wiring Test"). ze's config attaches the rr process with `receive [ ... state open-received ... ]` (model `test/draft/plugin/vpn-withdrawal-inventory-rr-ipv4.ci`): without `open-received` the plugin gets no OPEN and the key stays absent. Red at HEAD: the row has no `families` key, so the observer fails naming the absent field. Discriminating red, recorded in Step 2 after the AC-1s key lands by restoring the one-octet walk: rr records the ipv4/unicast default and the row lists `ipv4/unicast` | |
| `rs-extended-open-families` | `test/draft/plugin/` while iterated, then `test/plugin/` | AC-1r and AC-1s for rs: the same session and OPEN, the route server loaded, the `plugin/peer-families-show rs` observer on `show bgp rs peers`; ze's config attaches the rs process with `receive [ ... state open-received ... ]`, as for rr. Red at HEAD: the row has no `families` key, so the observer fails naming the absent field. Discriminating red, recorded in Step 2 after the AC-1s key lands by restoring the one-octet walk: rs records the ipv4/unicast default and the row lists `ipv4/unicast` | |

### Interop Tests (Scope: protocol)
| Scenario | Directory | Peer Daemon | What It Proves | Status |
|----------|-----------|-------------|----------------|--------|
| `bgp-open-extended-params-boundary-frr` (new) | `test/interop/scenarios/` | FRR | Ze's OPEN at capTotal 254 (sized with `software-version` padding plus families) reaches Established with FRR and FRR reports Ze's capabilities (D1). Red phase: revert the threshold, rebuild the Ze image, FRR refuses the OPEN | |
| `frr-software-version` (extended) | `test/interop/scenarios/frr-software-version/` | FRR | `opWaitLogContains` on Ze's WARN naming FRR's 2/0 for peer draft (D3) | |

-> Decision (2026-10-14, Phase 7): `frr-software-version` is extended in `internal/le/interoplab/bgp/check_softver.go` (`requireZeWarnsOpenRefused`, called at the end of `requireFRRRefusesDraftForm`: polls `lab.Logs` of ze for `level=WARN msg="notification received"` with `peer=<FRR v6>` and `code="OPEN Message Error" code-number=2 subcode=Unspecific subcode-number=0`). NOT RUN: Docker refuses the kernel on this machine, and `go vet` of the package could not run either (a foreign import cycle in `internal/le/go/stdpatch`); its revert-rebuild-red is owed. `bgp-open-extended-params-boundary-frr` is NOT written yet: the continuation owes it (scenario directory, a checker sized to capTotal 254, its registration in `specialCheckers`), then its run and red.
-> Decision (2026-10-14, Phase 6-7 continuation): committed ce52e6664c (Phases 5 and 6) and 8a79f4b16d (the `frr-software-version` WARN checker; `go vet ./internal/le/interoplab/bgp/` exit 0). `open-refused-notification-warn.ci` first ran red on the test's own assertion: the real line carries `subsystem=bgp.reactor.session` between `msg` and `peer`, and ze-peer can only send after its OPEN/KEEPALIVE handshake (seq=1 is the first message after it; no `option=open` value refuses the OPEN), so ze reads the 2/0 in ESTABLISHED, not OpenSent/OpenConfirm. The assertion was corrected to what the path produces and the header now says the `.ci` proves AC-3n plus the default-level log path of AC-3; AC-3's OpenSent/OpenConfirm state stays proved by `TestHandleNotificationLogsWarn` and by `frr-software-version`. Green: `./le test bgp plugin open-refused-notification-warn` PASS (scratch `job-ci-warn2-*.log`). Its red is OWED: the `.ci` carries no `RFC requirement:` tag, so `./le rfc discriminate-record` cannot take it, and no scratch revert is allowed; the unit's red (d3red.log) stands. A-4 validated (the WARN is visible at the default level).
-> Decision (2026-10-14, Phase 7): `bgp-open-extended-params-boundary-frr` is written: `test/interop/scenarios/bgp-open-extended-params-boundary-frr/{ze.conf,frr.conf}`, checker `check_open_boundary.go::checkOpenExtendedParamsBoundaryFRR` registered in `register_open_boundary.go`. Sizing uses the hostname capability, not software-version: MP ipv4/unicast 6 + four-octet AS 6 + hostname 2+1+200+1+38 = 242, total 254 (the base MP + ASN4 set is the one `test/plugin/explicit-plugin-config.ci` shows ze sending with no capability block; graceful-restart disabled). The checker requires FRR Established and FRR's `hostname` to be a non-empty prefix of the 200-octet host (FRR keeps 64 octets). `TestOpenBoundaryHostMatchesScenario` ties the checker constant to ze.conf (it caught a 201-octet host on first run) and `TestFRRReceivedHostname` refuses absent/empty; `ze config validate` accepts ze.conf. NOT RUN (Docker); the 254 total is arithmetic until the first run confirms it, and the revert-threshold red is owed. `docs/architecture/testing/interop.md` names both scenario changes.

## Files to Modify
- `internal/component/bgp/reactor/session_negotiate.go` - `buildOptionalParams` and `maxExtendedParamLen` deleted; `buildOpen` calls `capability.PackOptionalParams` and returns its error (Q4)
- `internal/chaos/peer/session.go` - chaos `buildOptionalParams` deleted; `BuildOpen` calls the packer, sets `ExtendedParams`, returns the error (Q4)
- `internal/perf/session.go` - `packOptionalParams` deleted; `BuildOpen` calls the packer and returns the error (Q4)
- `internal/component/bgp/plugins/bmp/bmp_locrib.go` - inline packing and `optParamCapabilities` deleted; `fabricateLocRIBOpen` calls the packer and returns the error (Q4)
- `internal/analyze/bgpconn.go` - `bgpBuildOpen`'s inline packing deleted; it calls the packer and returns the error (Q4)
- `internal/test/peer/inject.go` - `buildActiveOpen`'s inline packing deleted; it calls the packer and returns the error (Q4)
- `internal/test/peer/open.go` - `encodeOpen`'s length sums, parameter framing, header and envelope writing deleted; it maps `openParam` to typed parameters, calls the packer, fills a `message.Open` and writes it with `Open.WriteTo`, keeping the 4096-octet refusal (Q5 "a"); the `encodeOpen` comment updated (stale-comments rule)
- `internal/component/bgp/plugins/rr/rr.go` - `handleStructuredOpen`'s parameter walk and dead `open.ASN4` branch deleted; it calls `capability.ParseFromOptionalParams(open.OptionalParams, open.ExtendedParams)`, and on its error logs a warning and records nothing (AC-1r, A-7)
- `internal/component/bgp/plugins/rs/server.go` - `parseStructuredOpen`'s parameter walk and dead `open.ASN4` branch deleted; the same call, and on its error a warning and nil (AC-1r, A-7)
- `internal/component/bgp/plugins/rr/rr.go` `peerStatus` and `internal/component/bgp/plugins/rs/server_handlers.go` `peerStatus` - each row carries the `families` key of "Recorded-families field" (AC-1s, Q6 = a); the `show bgp rs peers` declaration comment in `rs/server.go` names the new key and why it is not a declared column
- `internal/component/bgp/plugins/rs/peer.go` `SupportsFamily` - the comment's "window between state-up and OPEN processing" is stale: the OPEN is delivered and handled before the state-up event (the blocking `Result` wait under "Recorded-families field"), so the comment says a nil `Families` means no OPEN was recorded for the session, and accept-all is the answer then (round-6 review, N4; pre-existing, stale-comments rule)
- `docs/architecture/api/commands.md` (the `ze-bgp:show-rr-peers` row, and a new row for `show bgp rs peers`, which has none today, so rs's key is not left documented only in the guide; round-6 review N5) and `docs/guide/route-reflection.md` (which names neither peer command today; it gains both, with the `families` key) - the `families` key, and when it is absent (AC-1s)
- `internal/le/interoplab/bgp/exabgp_server.go` - `exabgpNegotiatedEncoding`'s `openBody[9]` length read deleted; `message.UnpackOpen` then `ParseFromOptionalParams` with the OPEN's framing (AC-1r)
- the callers of chaos `BuildOpen`, perf `BuildOpen`, `fabricateLocRIBOpen`, `bgpBuildOpen` and `buildActiveOpen` - handle the new error (table in "D1 consolidation design")
- `internal/component/bgp/message/open.go` - the `writeToExtended` comment "optLen validated ≤ maxOptLen (65535)" names a validation no code performs today; after Q4 the packer performs it for every caller of the packer, so the comment names `capability.PackOptionalParams` (stale-comments rule)
- the reactor and bmp tests listed under "RFC-tagged tests that change" - calls move to the packer, or handle the new error
- `pkg/plugin/sdk/sdk_callbacks.go` (and `sdk.go` for the stage-2 run inside `Run` and the composed config-verify) - `Plugin.DeclareCapabilitiesFrom`; `SetCapabilities` deleted (Q1 = b)
- `internal/plugins/exabgp/main_sdk.go` - its `SetCapabilities` call replaced by a constant extractor (Q1 = b)
- `internal/component/bgp/configjson/traverse.go` - `CapabilitySelectors` (Q1 = b)
- `internal/component/bgp/plugins/softver/softver.go`, `hostname/hostname.go`, `role/role.go`, `llnh/llnh.go`, `gr/gr.go` (and `gr_capability.go`), `internal/plugins/exabgp/bridgeplugin/internal.go` - `SetCapabilities` call deleted, extractor registered, leaf names beside the codes (Q1 = b); the bridge's `applyConfig` comment ("family/capability changes need a plugin restart") is replaced by the refusal it now gets
- `internal/component/bgp/reactor/peer_settings_negotiation.go` - the `negotiatedCapabilitySettings` comment that records the stale plugin store: it now says the plugins refuse that reload (stale-comments rule)
- `internal/component/bgp/reactor/session_handlers.go` - D3 WARN
- `internal/component/bgp/yang/ze-bgp-conf.yang` - the `peer-fields` `router-id` description says the override departs from RFC 6286 Section 2.1 (Q2 = deviation, AC-6)
- `docs/guide/configuration.md` - the peer-level `router-id` text says the same (AC-6)
- `rfc/short/rfc6286.md`, `rfc/short/rfc9072.md`, `rfc/audit/rfc6286.json`, `rfc/audit/rfc9072.json`
- `rfc/discrimination/rfc9072.json`, `rfc9069.json`, `rfc2918.json`, `rfc7705.json`, `rfc8050.json`, `rfc4659.json`, `draft-ietf-idr-bgp-bfd-strict-mode.json` - the re-records and first records of "Discrimination records Q4 stales"
- `plan/journal/unwired-feature.md`, `plan/journal/silent-fall-through.md`, `plan/journal/helper-bypassed-by-an-open-coded-copy.md` (the 2026-10-14 received-OPEN reader row marked fixed)

## Files to Create
- `internal/core/bgp/capability/optional_params.go` - `PackOptionalParams`, with the RFC 9072 Section 2 and RFC 5492 Section 4 quotes (Q4)
- `pkg/plugin/sdk/restart_text.go` and `restart_text_test.go` - `RestartSteps() string`, lines 3 to 6 of the D2 refusal, the one restart-advice text this spec's D2 refusal and every refusal of `spec-plugin-reload-applies-or-refuses` (its AC-8) append; whichever spec lands first creates it with `TestRestartStepsNamesEveryRouteAndNeverSignalRestart`, the other only calls it (constraint aligned 2026-10-14 with that spec's design review round 3, ISSUE-2)
- the unit tests, `.ci` tests and interop scenarios named above, each with its typed checker in `internal/le/interoplab/bgp`
- `internal/test/fixture/` - the native observer fixture `plugin/peer-families-show` (argument `rr` or `rs`), on the `register_vpn_withdrawal.go` model (Q6 = a)

### Integration Checklist
| Integration Point | Applies? | File / reason |
|-------------------|----------|---------------|
| YANG schema (new RPCs/config) | No | no leaf added or removed; the `peer-fields` `router-id` description changes only (Q2 = deviation) |
| YANG validation constraints | No | no new leaf |
| YANG custom validators | No | no new leaf |
| CLI commands/flags | Output only (Q6 = a) | no command or flag added; `show rr peers` and `show bgp rs peers` rows gain the `families` key, structured data in the existing payload (`ai/rules/cli.md`: kebab-case key, `ResponseData`) |
| CLI grammar (keyword before value) | N-A | no command |
| Editor autocomplete | No | the schema keeps every leaf |
| Functional test for new RPC/API | No new RPC | the refusal travels in the existing config-verify answer; the `test/reload/` `.ci` files exercise it |
| Pipe completeness | Yes | the `families` key is part of the payload the pipes already render; rs's declared `Columns` are unchanged and the key renders after them (`column_order.go`) |
| Env var registration | No | none |
| Doctor check for runtime dependencies | N-A | no file, socket, port, module or binary added |
| Prometheus counters/metrics | No | the NOTIFICATION counters exist; D3 adds a log line only |
| BGP family surface (new SAFI / capability / attribute) | N-A | no new capability code; existing capabilities change only in when they are declared |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature, or scope changed? | One output key (owner Q6 = a, 2026-10-14); otherwise defects | the `families` key is documented under row 3 |
| 2 | Config syntax changed? | No syntax change; the peer-level `router-id` text changes (Q2 = deviation) | `docs/guide/configuration.md` and the `ze-bgp-conf.yang` `peer-fields` `router-id` description: the override departs from RFC 6286 Section 2.1 (AC-6) |
| 3 | CLI command added/changed? | Output of two existing commands (Q6 = a) | `docs/architecture/api/commands.md` (the `ze-bgp:show-rr-peers` row, and a new row for `show bgp rs peers`, round-6 review N5) and `docs/guide/route-reflection.md`: the `families` key and when it is absent |
| 4 | API/RPC added/changed? | No | the engine RPC is plugin protocol, row 8 |
| 5 | Plugin added/changed? | Yes | `docs/guide/plugins.md`: a capability plugin refuses a reload that changes its capability, the two new restrictions (a family edit on a `gr` peer with no `graceful-restart family` list, a peer added with a plugin capability), and how to restart: `systemctl restart ze`; without systemd `ze signal stop` then `ze start` or `ze start <config-file>`, the way it was started; on the appliance `request reboot` in the ze CLI, or `ze signal reboot` from a shell; which of these end sessions with a normal teardown (`systemctl restart ze`, `ze signal stop`) and which write the Graceful Restart marker and stop without a NOTIFICATION (both appliance routes, through `stopForRestart`); and the Graceful-Restart-keeping sequence under systemd (`ze signal restart`, wait for the exit, `systemctl start ze`). `docs/plugin-development/` page for the SDK: `SetCapabilities` is gone; every plugin that declares capabilities registers an extractor (a constant one when the set does not come from config), and the SDK runs it at stage 2 and at config-verify |
| 6 | Has a user guide page? | Yes | `docs/guide/monitoring.md` (the NOTIFICATION WARN line) |
| 7 | Wire format changed? | Yes | `docs/architecture/wire/capabilities.md` (the one packer, the 254-octet boundary, the 65532 limit) |
| 8 | Plugin SDK/protocol changed? | SDK method only, no protocol change (Q1 = b) | `docs/architecture/api/process-protocol.md` (declarations fixed for the process life; capability plugins refuse a reload that would change them) |
| 9 | RFC behavior changed or proven? | Yes | `rfc/short/rfc9072.md`, `rfc/short/rfc6286.md`, `docs/features/rfc-status.md` rows (generated from the summaries' Meta) |
| 10 | Test infrastructure changed? | Only if A-3 fails | `docs/functional-tests.md` |
| 11 | Affects daemon comparison? | No | - |
| 12 | Internal architecture changed? | Yes | `docs/architecture/behavior/fsm-open-sent.md` (WARN on Event 24/25) |
| 13 | Route metadata keys? | No | - |
| 14 | Prometheus counters? | No | - |
| 15 | Registered plugin/capability inventory changed? | No | - |
| 16 | Changed sources referenced by doc anchors? | Derived at implementation | `./le spec citation anchors spec plan/immediate/spec-bgp-open-session-rfc-defects.md`. Declared by `// Design:` headers of files this spec changes: `docs/architecture/chaos-web-dashboard.md` (`internal/chaos/peer/session.go`): unaffected, it describes the chaos dashboard, not the chaos peer's OPEN framing; `docs/architecture/core-design.md` (`internal/component/bgp/reactor/config.go`): unaffected by D1-D3 (it states no capability-declaration or router-id rule); `rfc/short/rfc9069.md` (`internal/component/bgp/plugins/bmp/bmp_locrib.go`): unaffected, the Loc-RIB OPEN keeps its capabilities and bytes; `docs/architecture/wire/capabilities.md` (`internal/core/bgp/capability/capability.go`): gains the one packer and its 254-octet boundary (row 7); `docs/architecture/api/process-protocol.md` (`pkg/plugin/sdk/sdk_callbacks.go`): gains the refusal sentence (row 8); `docs/architecture/config/syntax.md` (`internal/component/bgp/configjson/traverse.go`): re-read 2026-10-14 for the selector helper: unaffected, the page documents config syntax and carries no `configjson` anchor, and `capability_extractor.go` declares `process-protocol.md` instead; `internal/perf/session.go` declares none; `docs/architecture/wire/messages.md` (`internal/component/bgp/message/open.go`): its RFC 9072 paragraph ("When the Optional Parameters exceed 255 octets ... Ze writes the RFC 9072 extended form") stays true and gains that the one packer chooses the form at 2 + capTotal above 255 and refuses above 65535; `docs/architecture/mrt.md` (`internal/analyze/bgpconn.go`): unaffected, the `ze analyze` OPEN bytes do not change; `docs/architecture/testing/ci-format.md` (`internal/test/peer/inject.go`, `internal/test/peer/open.go`): unaffected, both test peer roles' OPEN bytes do not change (`encodeOpen` moves onto the one packer and `message.Open.WriteTo`, Q5 "a"); re-read for any sentence that says ze-peer frames its own parameters; the `// Design:` headers of `rr/rr.go`, `rs/server.go` and `interoplab/bgp/exabgp_server.go` are read at implementation for a page describing their OPEN read; `docs/architecture/exabgp-bridge.md` (`internal/plugins/exabgp/bridgeplugin/internal.go`): its "Stage ordering constrains the family declaration" section gains that a reload changing the bridge's capabilities is refused |
| 17 | Existing docs show examples for this area? | No | the peer-level `router-id` stays (Q2 = deviation), so no example changes |

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis (file/doc/user statement) | If wrong | Validated by | Status |
|----|-----------|--------------------------------|----------|--------------|--------|
| A-1 | Every capability plugin receives config-verify for its root (`bgp`, or `exabgp` for the bridge) on a reload, before any apply, whether or not it registered its own verify handler | `OnConfigVerify` doc ("full candidate config sections"); `llnh` and `gr` already refuse there | the SDK's comparison never runs for a plugin without a verify handler; the SDK registers the config-verify callback itself when an extractor is registered, and the engine's verify dispatch must reach every plugin that declared capabilities | read at the producers 2026-10-14 (review round 2, N7): verify dispatch picks plugins by `WantsConfigRoots` overlapping the changed roots (`internal/component/plugin/server/reload.go`, `reload_compensation.go::reloadConfigSections`), not by whether a verify handler is registered, and the SDK's default config-verify answers OK (`sdk_callbacks.go`, `callbackConfigVerify: marshalStatusOK`). The one condition is that the plugin's root changed, which suffices under A-6: an unchanged root yields an unchanged declaration. `reload-softver-encoding-refused.ci` and `reload-exabgp-bridge-capability-refused.ci` still prove it end to end | validated by reading, with that condition |
| A-2 | The live-stages reload fixture can assert a refusal's text, as `bgp-bfd-disable-refused-live.ci` does | that test's header ("the reload is refused, naming the peer") | fixture extension, `docs/functional-tests.md` row | first D2 `.ci`: `reload-softver-encoding-refused.ci` asserts the refusal line and the restart step on stderr and passes (2026-10-14) | validated |
| A-3 | `Plugin.capabilities` still holds the stage-3 set when config-verify runs | after this spec its one writer is the stage-2 extractor run inside `Run` (`SetCapabilities` deleted). Today's basis "nothing writes it after stage 3" was false (review round 2, ISSUE-3): the engine redelivers `configure` at runtime after a refused removal (`startup_removal.go`, `conn.SendConfigure`), the SDK routes it to `handleConfigure` (`sdk_callbacks.go` `callbackConfigure`, `sdk_dispatch.go`), and that reruns `OnConfigure`, which calls `SetCapabilities` today. The design therefore keeps the extractor out of `handleConfigure` | the comparison runs against a wrong baseline: a redelivery would drop the declarations of peers removed by accepted reloads, and a re-add would be refused | `TestDeclareCapabilitiesFrom` redelivery case (AC-2h) | confirmed 2026-10-14: `TestDeclareCapabilitiesFromStageTwoOnly` (`pkg/plugin/sdk/capability_declaration_test.go`) redelivers `configure` through the callback map and the declaration is unchanged; the extractor runs only in `handleStartupConfigure` |
| A-4 | le test peer can answer Ze's OPEN with a NOTIFICATION before or instead of a KEEPALIVE (`action=notification` at conn 1 seq 1) | `expect.go` action list | a le test peer `open` option to refuse | first D3 `.ci` | unvalidated |
| A-5 | FRR accepts an RFC 9072 OPEN of 254+ octets and reaches Established | FRR implements RFC 9072 (`bgp_open.c` extended opt params) | interop scenario uses another daemon (GoBGP, BIRD) | `bgp-open-extended-params-boundary-frr` | unvalidated |
| A-6 | `softver`, `hostname`, `role`, `llnh`, `gr` and the exabgp bridge derive their declaration only from the config the reload delivers (the review of 2026-10-13 found no `os.Hostname`, clock or random read in the five bgp extractors; `capabilityDecls` reads only `bridgeConfig`), so the same config yields the same declaration | each plugin's `OnConfigure` | a declaration that reads host state (a hostname default, say) could differ with no config change and refuse a reload the operator did not change; that plugin needs its own design | Phase 4 per plugin: grep 2026-10-14 over softver.go, hostname.go, role/config.go, role/role.go, llnh.go, gr_capability.go, gr_llgr.go and bridgeplugin finds no `os.Hostname`, `time.Now`, `rand.`, `os.Getenv`, `env.Get`, `clock.` or file read (bridgeplugin's one `os` use is the registration-failure exit in register.go) | validated |
| A-7 | rr and rs receive a received OPEN BEFORE the reactor validates it, so an OPEN `ParseFromOptionalParams` refuses (a non-type-2 parameter, an empty type-2 parameter, a malformed type-2 parameter) does reach them; the changed outcome (nothing recorded and a warning logged, where the walks skipped the parameter and stored the ipv4/unicast default) applies only to an OPEN the reactor then refuses, so no Established session is affected | `session_read.go` calls `s.onMessageReceived` before `s.handleOpen(body)`, which reaches rr `case rpc.EventKindOpen` and rs `parseStructuredOpen` through `peer_run.go`, `reactor_notify.go::notifyMessageReceiver` and `server/events.go::messageTypeToEventKind`; the reactor then refuses through `validateOpenPeerAS` (`session_open_as.go`), `ParseFromOptionalParams` and `rejectOpenCapabilityError`; rr and rs read stored families only for a peer that is up (`rr/validation.go`, `rs/server_forward.go`) | if the reactor accepted an OPEN the decoder refuses, rr or rs would hold no entry for an Established peer; that would be a disagreement between the reactor and the one decoder they share, not something rr or rs could repair | read 2026-10-14 at the producers named (round-3 review, ISSUE-2) | validated by reading |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | The refusal fires on a reload the operator thinks harmless and that is accepted today (a family edit on a `gr` peer with no `graceful-restart family` list; a peer added that carries a plugin capability) | operator report | a new restriction, correct under Q1 = b: the restarted OPEN would carry the stale plugin store. The text says why and names the restart; `docs/guide/plugins.md` and Known Limitations state it; an explicit `graceful-restart family` list avoids the first case |
| R-2 | The candidate selectors and the declared selectors disagree in form, so an unchanged peer reads as removed and added | `TestCapabilitySelectors`, AC-2e red | both come from `configjson.CapabilitySelector`; the reload's candidate set and the reactor's injected set both come from `configjson.PeerSelectors` (name, remote IP, dynamic group); no second spelling |

### Mistake Log
| Against | Wrong assumption | How it surfaced | Correction |
|---------|------------------|-----------------|------------|
| R-2 | Every plugin declares under the peer's name, so the candidate peer set could be peer names only | `TestRoleCapabilityExtractorRegistered` failed with "a changed role was accepted" (2026-10-14). Role keys its declaration by the remote IP (`role/config.go` `extractPeerRoleConfigs`); the candidate set held no IP, so the SDK read role's selector as a removed peer and accepted every role change | `configjson.PeerSelectors(name, addr, dynamicGroup)` is the one selector set: `CapabilitySelectors` (reload candidate) and reactor `capabilitySelectors` (live injection) both call it; `TestCapabilitySelectors` and `TestPeerSelectors` cover the IP |
| R-4 | An OPEN above RFC 8654's 4096 octets is still sent | code reading | the one packer now refuses above 65532 octets (AC-1x), which closes the `maxExtendedParamLen` half; the 4096-octet half gets one row in `plan/journal/silent-fall-through.md` at implementation start, not fixed here (rule-precedence: closing first) |
| R-5 | The D3 WARN floods the log when a peer refuses every retry | ConnectRetry cadence in `frr-software-version` (nine refusals) | one line per received NOTIFICATION is bounded by the reconnect backoff; FRR and BIRD log every received NOTIFICATION at their default level too |
| R-8 | A consumer of `show rr peers` or `show bgp rs peers` asserts the exact row keys and breaks on the added `families` key (Q6 = a) | a red in `rs/server_shape_test.go`, `rs/server_peer_order_test.go`, `rr/cmd_show_test.go`, `internal/test/fixture/plugin_fixture_15_show.go` or `ui_fixture_show_bgp_plugin_shapes.go` | each reads named keys or counts today (read 2026-10-14); a test asserting the whole row is corrected to the new row, which is the behaviour this spec adds. Under Q6 = b instead, the risk is rr's End-of-RIB dropped by `AnnounceEOR` while `shouldQueue() \|\| initialSyncEOROwed` (`reactor_api_forward.go`, `logEORSuppressed`) with `manual-eor` leaving no reactor marker as fallback; that test must then reject the `end-of-rib suppressed` WARN (round-5 review, I2) |
| R-7 | The chaos or perf OPEN's move to one parameter for all capabilities changes a byte expectation somewhere | a chaos or perf test red | every RFC 5492 receiver accepts it; correct the expectation, which asserted framing no RFC requires |

## Blast Radius

| Question | Answer |
|----------|--------|
| What breaks if this is wrong? | D1: OPENs of 254-255 octets (today broken) and, if the threshold were moved wrongly, every OPEN; the encode/decode `.ci` corpus pins the common sizes. D2: capability plugins on reload, and every capability plugin's stage-2 declaration, which now runs through the SDK registration. D3: log only. D4: documentation and the deviation record only |
| How is it reverted? | Single commit per row |
| Who else touches this path? | `plan/spec-bgp-fsm-delayopen-and-automatic-start.md` (OPEN and session handling), `spec-bgp-live-receive-binding-and-bfd-keep-session` (reload swap plan in `peer_settings_apply.go`, closed 2026-10-13), `plan/pre-release/spec-rfc-verdict-test-fix-pass.md` (verdict rows; its BGP child spec-rfc-verdict-fix-bgp closed 2026-10-21), `plan/pre-release/spec-config-yang-when-unique-choice.md` (router-id mandatory ruling) |

## Implementation Steps

1. **Phase: Wiring and the false claim (FIRST)** - AC-5 edit of `rfc/short/rfc6286.md` and the rfc-status row; write the failing wiring tests of the Wiring Test table
   - Verify: each new test fails for the stated reason; for `rr-extended-open-families.ci` and `rs-extended-open-families.ci` that reason is the HEAD red (no `families` key, the observer fails naming the absent field), and their discriminating `ipv4/unicast` red is recorded in Step 2 after the AC-1s key lands
2. **Phase: D1 (Q4 = Consolidate, Q5 = a)** - capture `encodeOpen`'s output for the `TestPassiveOpenUsesOnePacker` lists as goldens BEFORE any edit; `TestPackOptionalParams` and `TestRFC9072OptionalParamsBoundary` red at 254/255; delete reactor `buildOptionalParams` and `maxExtendedParamLen`, then write `capability.PackOptionalParams` over typed parameters (one concrete struct with private fields built only by the Capabilities constructor, Type fixed to 2, and the Raw constructor, Type and octets; zero value a type-0 empty Raw parameter) with the RFC 9072 Section 2 quote above the threshold; move `buildOpen`, then delete chaos `buildOptionalParams`, perf `packOptionalParams`, the inline packing in `fabricateLocRIBOpen`, `bgpBuildOpen` and `buildActiveOpen`, and `encodeOpen`'s framing and envelope, and move their callers, with the new error handled (`encodeOpen` onto the packer and `message.Open.WriteTo`, its 4096 refusal kept); then AC-1s: `TestRouteReflectorPeerStatusFamilies` and `TestRouteServerPeerStatusFamilies` red, the `families` key in both `peerStatus`, green, and the `plugin/peer-families-show` fixture; then AC-1r: `TestRouteReflectorReadsExtendedOpen`, `TestRouteServerReadsExtendedOpen`, `TestExabgpMockReadsExtendedOpen` and the two `.ci` (written in Step 1 in `test/draft/plugin/`, red there on the absent key) now red with the row listing `ipv4/unicast`, the discriminating red, delete the rr and rs walks and the mock's length read, call `ParseFromOptionalParams`, green; correct the `writeToExtended` comment; move the tests of "RFC-tagged tests that change" (owner runs the seven approval commands); green; `./le arch tier check` names no touched package; `./le rfc discriminate-record` for each new RFC9072-2-1 / RFC9072-2-7 tag and for every row of "Discrimination records Q4 stales" plus the two first records; `open-optional-params-254.ci`
3. **Phase: D2 registration (Q1 = b)** - `TestDeclareCapabilitiesFrom` and `TestCapabilitySelectors` red; delete `SetCapabilities`, then write `Plugin.DeclareCapabilitiesFrom` (no root parameter; the stage-2 run inside `Run` only, never in `handleConfigure`; the config-verify comparison composed after the plugin's own handler at `Run`, whatever the registration order), `configjson.CapabilitySelectors`; `main_sdk.go` registers its constant extractor; green
4. **Phase: D2 plugins** - softver first (the journal row), then hostname, role, llnh, gr and the exabgp bridge: delete the `SetCapabilities` call, register the extractor and the leaf names, each plugin's test; reload `.ci` per AC-2 to AC-2g; correct the `negotiatedCapabilitySettings` and bridge `applyConfig` comments
5. **Phase: D4 (Q2 = deviation)** - `rfc/short/rfc6286.md` RFC6286-2.1-1 `{partial}` with the deviation reason; the YANG `router-id` description and `docs/guide/configuration.md` wording (AC-6)
6. **Phase: D3** - `TestHandleNotificationLogsWarn` red, WARN line, fold the RFC 8203 INFO into it, `.ci`
7. **Phase: interop** - the three scenarios with revert-rebuild-red records
8. **Phase: verdicts** - AC-4: fresh-agent `./le rfc audit-stamp ... mode rejudge` for both rows; journal rows marked fixed (AC-7)

### Critical Review Checklist
| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | Every AC-N has an implementation at file:line |
| Correctness | the threshold compares the Optional Parameters field length (2 + capTotal), not capTotal; 253 stays byte-identical |
| Correctness | the WARN carries code and subcode names and numbers and the FSM state; one line, not two, when an RFC 8203 message exists |
| Data flow | capabilities still enter the OPEN only through `buildOpen`; no reactor code learns softver/hostname/role semantics |
| Rule: rfc-compliance | RFC 9072 Section 2 quote above the threshold; new tags carry discrimination records; AC-5 done first |
| Rule: no-layering | the INFO RFC 8203 line is replaced, not kept beside the WARN; no Optional Parameters packing survives outside `capability.PackOptionalParams`: a tree-wide grep over `internal`, `pkg` and `cmd` for an Optional Parameter write finds only the packer, and for an extended-envelope write only `message/open.go`, with no exception; no Optional Parameters walk survives outside `ParseFromOptionalParams` in rr, rs and the exabgp mock; `SetCapabilities` no longer exists |
| Correctness | the `families` key is absent, never an empty list, on a row whose peer is down or whose plugin recorded no OPEN; it is not added to rs's declared `Columns` (AC-1s) |
| Correctness | the refusal compares per selector and per code, treats an empty `Peers` as one "every peer" selector, accepts a removed peer, and names plugin, peer, leaf and `systemctl restart ze`, never `ze signal restart` |

### Deliverables Checklist
| Deliverable | Verification method |
|-------------|---------------------|
| D1 fixed | `TestRFC9072OptionalParamsBoundary` and `open-optional-params-254.ci` pass; revert red recorded |
| D1 consolidated | one packer over typed parameters, seven callers, `encodeOpen` bytes golden-identical; one reader for rr, rs and the exabgp mock, `rr-extended-open-families.ci` and `rs-extended-open-families.ci` pass with a red recorded by restoring the one-octet walk (the row lists `ipv4/unicast`); no `./le arch tier check` violation names a touched package; every row of "Discrimination records Q4 stales" re-recorded and the two first records written |
| D2 fixed | reload `.ci` files pass; red recorded by removing the SDK's config-verify comparison |
| D3 fixed | `open-refused-notification-warn.ci` passes |
| D4 applied as a deviation | YANG description and `docs/guide/configuration.md` name the departure from RFC 6286 Section 2.1; `rfc/short/rfc6286.md` row is `{partial}` with the reason |
| Verdicts | `rfc/audit/rfc9072.json` RFC9072-2-1 `enforced`; RFC6286-2.1-1 re-judged against its deviation record |

### Security Review Checklist
| Check | What to look for |
|-------|-----------------|
| Input validation | the refusal text quotes peer names and leaf names from config, never raw payload bytes; a NOTIFICATION's RFC 8203 message is logged through the existing sanitising path (`ShutdownMessage`), never raw bytes |
| Resource exhaustion | the packer refuses above 65532 octets instead of sending a truncated OPEN |

### Failure Routing

| Failure | Route To |
|---------|----------|
| Compilation error | Fix in the phase that introduced it |
| Test fails for the wrong reason | Fix the test assertion or setup |
| Test fails on behavior mismatch | Re-read the source in Current Behavior. If misunderstood → RESEARCH |
| A-1 broken (a capability plugin gets no config-verify although its root changed; A-1 is validated by reading, so this is a regression in `reload.go` dispatch) | DESIGN: route verify to every plugin that declared capabilities |
| Functional test fails | Check the AC: wrong AC → DESIGN, correct AC → IMPLEMENT |
| 3 fix attempts failed | STOP. Report all 3 approaches. Ask the user |

## Key Design Decisions
| Decision | Alternatives Considered | Rationale |
|----------|------------------------|-----------|
| D1: one packer, `capability.PackOptionalParams`, over typed parameters (one concrete struct with private fields: Type, a capability list written in place by each capability's `WriteTo`, raw octets; built only by a Capabilities constructor with Type fixed to 2 and a Raw constructor from Type and octets, so no both-forms refusal; no interface), base form while the sum of 2 + value length is at most 255 (owner, 2026-10-13: "Consolidate"; Q5, 2026-10-14: "a") | home in `internal/component/bgp/message`; a raw-bytes value per parameter; `encodeOpen` kept as a second packer (Q5 "b"); exported fields with a both-set refusal (round 3); a sealed interface per value form | the capability package is a core leaf six of the seven callers already import and holds the decoder, while `message` imports `internal/component/plugin/registry`, so `internal/analyze` would gain a component import; a raw-bytes value would make the reactor serialise into an intermediate buffer and copy it, losing the one allocation; Q5 "a" chose one packer; exported fields let a caller build both value forms, or a capability list under a Type other than 2, which `ze-go-style.md` "One state, one variant" forbids (round-4 review, ISSUE-3); a sealed interface would escape to the heap and cost the reactor a second allocation per OPEN (round-3 review, ISSUE-1) |
| D1: rr, rs and the exabgp mock read a received OPEN through `ParseFromOptionalParams` | fix the one-octet walk in place in each (read `ExtendedParams` there) | three copies of a walk the decoder already performs would stay; one reader is the fix at the root (`ai/rules/simplicity.md`) |
| D2: `SetCapabilities` deleted, `main_sdk.go` registers a constant extractor | keep `SetCapabilities` for the flag-built set and refuse a plugin using both at `Run` | the kept path existed only for `main_sdk.go`, whose stated reason (no config root) was false; deleting it removes the second declaration path and the refusal that kept the two exclusive (`ai/rules/no-layering.md`) |
| D2: refuse a reload that changes a plugin-declared capability (owner, 2026-10-13: "b"), through one SDK registration of the plugin's extractor | the comparison in the engine; one SDK comparison method plus a verify handler in each plugin | only the plugin can compute its candidate declaration, so the engine cannot compare; a per-plugin verify handler is opt-in and already missed the exabgp bridge (review of 2026-10-13, ISSUE-6), while a registration that the SDK runs at stage 2 and at verify cannot be skipped and cannot compute the two declarations differently |
| D2: each plugin's test drives `sdk.CheckCapabilityReload(plugin, extract, leaves, startup, candidate)`, which shares `verifyCandidate` with the composed config-verify, over the package vars `Run` registers | run each plugin's `Run` over `net.Pipe` with a fake engine | one comparison either way; the pure call proves the plugin's own extractor and leaves without an engine fake per plugin, and the `.ci` files prove registration end to end |
| D2: compare per selector and accept a removed peer | compare the whole lists | a whole-list compare would refuse every peer removal, which leaves no stale OPEN |
| D3: one WARN per received NOTIFICATION in `handleNotification` | WARN only before Established; raise the reconnect-loop DEBUG to WARN | the producer is the one place that knows code, subcode and state; one rule for all states matches FRR and BIRD defaults; the reconnect loop sees only an error string |

## Known Limitations
- R-4: the 4096-octet OPEN limit is recorded as a journal row, not fixed here; the two-octet limit is refused by the one packer (AC-1x)
- Out of scope by Q3 (owner, 2026-10-13: "Out of scope"): the live global `router-id` reload stays as it ships. RFC 6286 Section 2.1 says the identifier "is determined on startup"; Ze applies a changed global `router-id` on SIGHUP (`reactorAPIAdapter.applyGlobalsJournaled`, `internal/component/bgp/reactor/reload_globals.go`) and the peer diff restarts every peer whose effective Identifier changed, so every restarted peer sees the new identifier in a new OPEN (`test/reload/reload-global-router-id.ci`). Not refused, not counted as a deviation here
- A plugin-declared capability change (softver, hostname, role, llnh, gr, the exabgp bridge) needs a daemon restart (Q1 = b); the reload refuses it and says so
- New restriction against today (Q1 = b): two reloads that are accepted today are refused and need a daemon restart: a session family added or removed on a `gr` peer whose `graceful-restart` container has no `family` list (`grAdvertisedFamilies` derives the GR payload from the session families), and a peer added that carries a plugin-declared capability. An explicit `graceful-restart family` list avoids the first. A family edit under the exabgp bridge while `add-path` is set is refused as well (accepted and not applied today)
- Of the restarts the refusal names, `systemctl restart ze` and `ze signal stop` then `ze start` or `ze start <config-file>` end sessions with a normal teardown, so peers withdraw Ze's routes until it is back; the appliance routes, `request reboot` in the ze CLI and `ze signal reboot` from a shell, both call `stopForRestart` (`cmd/ze/hub/infra_setup.go`), which writes the Graceful Restart marker and stops without a NOTIFICATION, so peers retain Ze's routes for the restart time; the Graceful-Restart-keeping sequence is in `docs/guide/plugins.md`, not in the refusal
- RFC 4271 Section 4.2 carries the same "same for every local interface and BGP peer" sentence and has no row; RFC scope is frozen, so no row is added

## RFC Documentation (Scope: protocol)

Add `// RFC 9072 Section 2: "However, if the length of the Optional Parameters in the BGP OPEN message does exceed 255, the OPEN message MUST be encoded according to the procedure below."` above the threshold in `capability.PackOptionalParams`, and the RFC 9072 Section 2 SHOULD sentence above the base-form branch. Each caller of the packer carries `// RFC 9072 Section 2` at the call (`ai/rules/rfc-compliance.md`, functions added from 2026-09-24).

## Design Review Log

| Date | Round | Result |
|------|-------|--------|
| 2026-10-13 | owner answers applied 2026-10-13; review round owed (Q1 refusal, Q4 consolidation) | Q1 "b", Q3 "Out of scope", Q4 "Consolidate" recorded; D2 refusal design and D1 consolidation design written; review items named in both designs (the restart command; error propagation from chaos, perf and bmp OPEN builders) |
| 2026-10-13 | review of the owner-answer edits applied (`tmp/session/2026-10-12-0c3648c8-b9d7-45d8-b3f0-2473bceefb25/scratch/review-open-session-owner-answers.md`: 0 BLOCKER, 8 ISSUE, 6 NIT) | ISSUE-1: eleven stale records and two first records listed, seven approval commands. ISSUE-2: six packers (`bgpBuildOpen`, `buildActiveOpen` added), all deleted onto the one packer, grep widened to the tree; a seventh writer, `encodeOpen`, opened as Q5. ISSUE-3: AC-1q names no tier violation in a touched package. ISSUE-4: refusal names `systemctl restart ze`, the unsupervised and appliance routes, never `ze signal restart` (exits 0, `Restart=on-failure`). ISSUE-5: new restriction stated in Behavior to change, Known Limitations and R-1; GR claim scoped to no `graceful-restart family` list; AC-2d scoped, AC-2g accepted polarity. ISSUE-6: one SDK registration (`DeclareCapabilitiesFrom`) covering the five bgp plugins and the exabgp bridge, no per-plugin verify handler; empty-`Peers` verdict row; AC-2f. ISSUE-7: "ze would keep sending". ISSUE-8: every Q2-remove branch deleted; deviation names the YANG description and `docs/guide/configuration.md` throughout. NITs: N1 left; N2 sentence; N3 line; N4 resolved table; N5 `writeToExtended` comment in Files to Modify; N6 nothing. **Next round owed, narrow: the SDK registration (its shape, the kept `SetCapabilities`, A-1 dispatch to plugins without a verify handler) and the six-packer consolidation (plus Q5)** |
| 2026-10-14 | review round 2 applied (`tmp/session/2026-10-12-0c3648c8-b9d7-45d8-b3f0-2473bceefb25/scratch/review-open-session-r2.md`: 0 BLOCKER, 5 ISSUE, 7 NIT) | ISSUE-1: owner Q5 "a" (2026-10-14) recorded in the Q5 row and the D1 owner row; the packer takes typed parameters (Type plus a value with `Len` and `WriteTo`: capability lists, `Unknown` TLVs, raw bytes), one code path, base form while the sum of 2 + length is at most 255, extended while the sum of 3 + length is at most 65535, else error; seven writers, `encodeOpen` included, with AC-1p, `TestPassiveOpenUsesOnePacker` and `internal/test/peer/open.go` in Files; the one-allocation row kept true. ISSUE-2: `SetCapabilities` deleted, `main_sdk.go` registers a constant extractor, no `Run` refusal (AC-2i). ISSUE-3: the extractor runs for the declaration only on the stage-2 path inside `Run`, the baseline is never rewritten after stage 3, A-3 basis corrected, AC-2h and the redelivery test case. ISSUE-4: the appliance route is `request reboot` in the ze CLI or `ze signal reboot` from a shell, everywhere. ISSUE-5: `DeclareCapabilitiesFrom` has no root parameter; extractors filter `section.Root`. N1: verify composed at `Run` whatever the order. N2: `ze start` or `ze start <config-file>`. N3: bridge root `exabgp`, `parseConfig` then `capabilityDecls`. N4: bridge family edit with `add-path` refused (New restriction, AC-2f). N5: home rationale restated (`message` imports `plugin/registry`). N6: `encodeOpen` is the only other envelope writer (grep), so it moves onto `message.Open.WriteTo`. N7: A-1 validated by reading, with its condition. Out-of-scope find brought in by the main thread: rr, rs and the exabgp mock read a received OPEN with a one-octet length, ignoring `ExtendedParams` (verified at the producers); routed through `ParseFromOptionalParams` (AC-1r, three unit tests, two `.ci`, A-7); journal row in `plan/journal/helper-bypassed-by-an-open-coded-copy.md`. **Next round owed, narrow: the typed-parameter packer (value kinds, `Unknown` constructor, `encodeOpen` on `message.Open.WriteTo`) and the rr/rs/mock move onto the one decoder (A-7)** |
| 2026-10-14 | review round 3 applied (`tmp/session/2026-10-12-0c3648c8-b9d7-45d8-b3f0-2473bceefb25/scratch/review-open-session-r3.md`: 0 BLOCKER, 3 ISSUE, 5 NIT), producers re-read 2026-10-14 | ISSUE-1: a parameter is a concrete struct (Type, Capabilities, Raw; no interface), the `openParam` shape, so the reactor's per-OPEN list does not escape and the output stays the one allocation; a parameter with both value fields set is refused; `TestPackOptionalParams` asserts AllocsPerRun 1 with the list built inside the measured closure; Zero-copy row and D1 decision row follow. N1: the `Unknown` constructor is dropped (Files entry removed); `encodeOpen` hands its type-2 TLVs as Raw. ISSUE-2: A-7 rewritten and validated by reading (OPEN delivery precedes `handleOpen`); on a `ParseFromOptionalParams` error rr and rs record nothing and log a warning, never the ipv4/unicast default; the three refused cases added to AC-1r and both unit tests. ISSUE-3: both appliance routes call `stopForRestart` (GR marker, no NOTIFICATION); "normal teardown" limited to `systemctl restart ze` and `ze signal stop` in the restart table, the trade-off, Known Limitations and docs row 5. N2: AC-1p and `TestPassiveOpenUsesOnePacker` name which error each size range gets. N3: "before allocating the message". N4: dead `open.ASN4` branches deleted in the rr and rs rows and Files. N5: the skip-to-error change named as rr and rs only; the mock and ExaBGP's emission stated. **Next round owed, narrow, because ISSUE-2 adds design: the rr and rs error path (return before `rr.mu` / nil from `parseStructuredOpen`, the warning line an operator sees, and an earlier session's entry left in place but read only while the peer is up), plus the packer's both-fields refusal from ISSUE-1** |
| 2026-10-14 | review round 4 applied (`tmp/session/2026-10-12-0c3648c8-b9d7-45d8-b3f0-2473bceefb25/scratch/review-open-session-r4.md`: 0 BLOCKER, 3 ISSUE, 6 NIT), producers re-read 2026-10-14 | ISSUE-1: "The warning" written: one WARN record per refused OPEN, three lines (problem, why, next step) with `peer` and `error` attributes, separate rr and rs constants differing only in plugin name and what is recorded; asserted in `TestRouteReflectorReadsExtendedOpen` and `TestRouteServerReadsExtendedOpen`, and in AC-1r. The reviewer's "only log line at WARN" is corrected at the producer: `session_write.go` also logs `notification sent` at WARN (code and subcode, no cause), so this is the only line that says why. ISSUE-2: Wiring rows 5 and 6 and the two `.ci` rows assert what the families drive, not `peerStatus` (which returns `address`, `remote.as`, `up`): rr the ipv6/unicast End-of-RIB from `sendEOR`, rs the forwarded ipv6/unicast UPDATE selected by `SupportsFamily` (two sessions, `conn_map` by remote IP). Beyond the reviewer's proposal: the reactor's initial sync sends an End-of-RIB for every negotiated family (`peer_initial_sync.go`, deduplicated with rr's through `claimInitialSyncEOR`), so an rr EOR assertion alone cannot go red; the rr `.ci` sets `behavior { manual-eor true }` so `AnnounceEOR` from rr is the only producer. ISSUE-3: one concrete struct with private fields, a Capabilities constructor (Type fixed to 2) and a Raw constructor (Type and octets); zero value is a type-0 empty Raw parameter, written as `00 00`; the both-set refusal and its `TestPackOptionalParams` case dropped; the size error is the packer's only error; steps 2 and 6, step 2 of Implementation, and the D1 decision row follow. N1: stray "A" deleted. N2: settled from `docs/guide/appliance.md` (gokrazy restarts only on non-zero), `ze signal restart` exits 0 so is no appliance route. N3: "`buildOptionalParams` allocates once today". N4: both decode failures (`UnpackOpen`, `ParseFromOptionalParams`) warn with the same text, both refused by the reactor through `rejectOpenCapabilityError`. N5: every reader of an earlier session's entry named (`Up` gates: forwarding, rs refresh and validation; `ReplayGen` gates: both `sendEOR`). N6: "R-4 records the separate 4096-octet limit (not fixed here)". **Round 5 owed, narrow, on one point only: the `manual-eor` discriminator in `rr-extended-open-families.ci`, which this round added beyond the reviewer's proposal and which no independent reader has checked. Everything else applies the reviewer's own proposals and adds no design** |
| 2026-10-14 | review round 5 applied (`tmp/session/2026-10-12-0c3648c8-b9d7-45d8-b3f0-2473bceefb25/scratch/review-open-session-r5.md`: 1 BLOCKER, 3 ISSUE, 4 NIT), producers re-read 2026-10-14 | B1 (rr `sendEOR` only when `replayed > 0`, so a single session never gets rr's End-of-RIB) and I1 (rs's `SupportsFamily` filter bypassed by `rs-fast-path` and by the peer-up adj-rib-in replay): confirmed at `rr.go::replayForPeer` and `rs/server_handlers.go`. Resolved by changing the observable rather than forcing order: each `show rr peers` / `show bgp rs peers` row gains a read-only `families` key ("Recorded-families field", AC-1s, Q6 = a, owner confirmation owed as an additive CLI output change), and both `.ci` poll it through a native observer (`plugin/peer-families-show`, model `register_vpn_withdrawal.go`); one session each, no `manual-eor`, no second session, no `rs-fast-path`; `rs-extended-open-capabilities` renamed `rs-extended-open-families`. The order-forced design the reviewer proposed is option (b) of Q6. I2 (the `AnnounceEOR` dropped-marker window under `manual-eor`): moot under (a), recorded in R-8 for (b). I3: the message is a constant without the peer or the decoder error, line 3 names "the error attribute", and a rendered one-line sample shows both attributes after the message. N1, N4: confirmations, nothing to change. N2: how ze-peer reaches the RFC 9072 form with two `add-capability` lines and where its ipv6/unicast comes from, under "Wiring Test". N3: left to the recorded red, stated there. Also: the absent-key rule for down peers and unrecorded OPENs, rs `Columns` unchanged (`TestDeclaredColumnsExistInPayload`), R-8, user story 4, Integration and Documentation rows 3. **Round 6 owed, narrow: the two tests and the field they read (the observer's poll and pass condition, the absent-key rule, the event-order reading, the red at HEAD), checked by a reader who did not write them** |
| 2026-10-14 | review round 6 applied (`tmp/session/2026-10-12-0c3648c8-b9d7-45d8-b3f0-2473bceefb25/scratch/review-open-session-r6.md`: 0 BLOCKER, 1 ISSUE, 5 NIT) | I1: the HEAD red of both `.ci` is "no `families` key, the observer fails naming the absent field" (`peerStatus` writes only `address`, `remote`, `up`); the discriminating `ipv4/unicast` red is recorded in Step 2 after the AC-1s key lands, by restoring the one-octet walk; the Wiring note, both `.ci` Functional rows and Implementation Steps 1 and 2 follow, the two `.ci` staying in Step 1 as the Wiring Test requires. N1: the test ends with the observer's `request shutdown` (`runPlugin13(..., shutdown=true)`) and `expect=exit:code=0`. N2: ze's config attaches rr/rs with `receive [ ... state open-received ... ]`, in the Wiring note and both `.ci` rows. N3: the empty-list sentence scoped to rs; rr's list is never empty when Multiprotocol is present. N4: the blocking `Result` wait cited under "Recorded-families field"; the stale `rs/peer.go::SupportsFamily` comment added to Files to Modify. N5: a `show bgp rs peers` row added to `docs/architecture/api/commands.md` in Files and Documentation row 3. **Round 7 not owed: every change applies the reviewer's own proposal and adds no design** |
| 2026-10-14 | cross-spec alignment from `spec-plugin-reload-applies-or-refuses` design review round 3 (ISSUE-2, ISSUE-3), decided by the main thread under the owner's delegation ("Do what you think is right", 2026-10-14) | The D2 refusal's restart routes are one per line (lines 3 to 6; the earlier line 5 joined the non-systemd and appliance routes, and no test pinned it), and they come from the shared helper `sdk.RestartSteps()` in `pkg/plugin/sdk/restart_text.go`, which whichever spec lands first creates (Files to Create). The "line 5" reference to the appliance routes now reads line 6 |

## Checklist

### Pre-Spec Verification (before the design is presented)
- [ ] Metadata table present, with a valid Status, Depends, Phase and Updated
- [ ] `ai/INDEX.md` keyword table checked
- [ ] An `rfc/short/` summary exists for every RFC referenced
- [ ] No code snippets
- [ ] Files to Modify names feature code, not only tests
- [ ] AC-N rows carry testable assertions
- [ ] Every assumption has a Basis and a validation method; every failure mode is a risk row

### TDD
- [ ] Tests written
- [ ] Tests FAIL (before the fix)
- [ ] Tests PASS (after the fix)
- [ ] Boundary tests for capTotal
- [ ] Functional `.ci` tests
- [ ] Interop tests

### Verification
- [ ] `./le verify worktree`

### Closure
- [ ] Append `plan/TEMPLATE-CLOSURE.md` and complete every section in it
- [ ] `/ze-review` gate clean
- [ ] **Commit A:** code + tests + docs + edited spec + journal rows
- [ ] **Commit B:** `remove plan/immediate/spec-bgp-open-session-rfc-defects.md`
