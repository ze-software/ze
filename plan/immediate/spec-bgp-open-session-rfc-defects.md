# Spec: bgp-open-session-rfc-defects

| Field | Value |
|-------|-------|
| Status | design |
| Scope | protocol |
| Depends | - |
| Phase | - |
| Handoff | - |
| Updated | 2026-10-13 |

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
| D1 | One Optional Parameters packer, or the threshold fixed in place in each packer? (Q4) | **Owner, 2026-10-13: "Consolidate".** Read as one packer for every copy (main thread, 2026-10-13): it serves the reactor, the chaos peer, the perf sender, the BMP Loc-RIB OPEN, `ze analyze`'s OPEN and the test peer's active-role OPEN, and the six copies are deleted. A seventh writer, the test peer's `encodeOpen`, is open as Q5 |
| D2 | How does a reload that changes a plugin-declared capability behave? (Q1) | **Owner, 2026-10-13: "b".** The reload is refused, naming the plugin, the peer and the capability, and saying that ze must be restarted to apply it |
| D4 | Remove the per-peer `router-id` override, or keep it as an owner-approved deviation from RFC 6286 Section 2.1? (Q2); is the live global `router-id` reload part of the ruling? (Q3) | Q2: **owner, 2026-10-13: "we should have a global router-id which can be overwritten per peer which is over what the RFC allows"** (deviation). Q3: **owner, 2026-10-13: "Out of scope"**: the live global `router-id` reload stays as it ships |

### Owner questions (design phase, 2026-10-13)

Each question asks which way to fix, never whether. Q1 to Q4 are answered and
the design below follows them; Q5 is open.

| # | Row | Question | Answer | Consequence |
|---|-----|----------|--------|-------------|
| Q1 | D2 | Re-declare on apply, refuse the reload, or restart the plugin process? | **Owner, 2026-10-13: "b"**: config-verify refuses a reload that changes a plugin-declared capability, including a peer added on reload that would carry one | AC-2 to AC-2e are the refusal and its acceptance polarity. No RPC, injector or engine change: each capability plugin hands the SDK its capability extractor once, and the SDK runs it at stage 2 and again at config-verify and refuses a changed set; no plugin writes a verify handler for it (see "D2 refusal design"). New restriction against today: a family edit on a `gr` peer with no `graceful-restart family` list, and adding a peer that carries a plugin-declared capability, now need a daemon restart (Known Limitations) |
| Q2 | D4 | Remove the peer- and group-level `session router-id` leaf (one BGP Identifier per speaker), or keep it as an owner-approved deviation | **Answered by the owner, 2026-10-13: "we should have a global router-id which can be overwritten per peer which is over what the RFC allows". The leaf stays as an owner-approved deviation from RFC 6286 Section 2.1: AC-6 documents it, RFC6286-2.1-1 leaves the conformance count** | No config or test-corpus change; RFC6286-2.1-1 becomes `{partial}`/deviation in `rfc/short/rfc6286.md` and leaves the conformance count; AC-4 brings RFC9072-2-1 to `enforced` and RFC6286-2.1-1 carries the deviation record |
| Q3 | D4 | The same Section 2.1 sentence says the identifier is "determined on startup". Ze applies a changed global `router-id` on SIGHUP and restarts sessions (`bgp-reload-global-router-id`, `test/reload/reload-global-router-id.ci`). Is that live change part of the D4 ruling, or out of scope? | **Owner, 2026-10-13: "Out of scope"** | The live reload stays as it ships: `reactorAPIAdapter.applyGlobalsJournaled` (`internal/component/bgp/reactor/reload_globals.go`) stores the new `Globals.RouterID` in `r.config.RouterID`, and the peer diff that follows restarts every peer whose effective Identifier (`PeerSettings.RouterID`) changed, so each restarted peer gets the new Identifier in a new OPEN. No AC, test or code here touches it; recorded under Known Limitations |
| Q4 | D1 | Fix the threshold in place in the packers, or consolidate the Optional Parameters packers into one producer? | **Owner, 2026-10-13: "Consolidate"** (asked against a list of four; the main thread reads it as every copy, 2026-10-13) | One packer, `capability.PackOptionalParams` in `internal/core/bgp/capability` (see "D1 consolidation design"); the six copies are deleted. The D1 threshold fix and the 253/254/255/256 boundary tests stay, now on the one packer. The RFC-tagged units the consolidation changes move under the owner's approval of 2026-10-13 ("approve all RFC-tagged test changes"), each with a fresh discrimination record; the list is under "RFC-tagged tests that change" |
| Q5 | D1 | Open, for the main thread (found 2026-10-13 while verifying ISSUE-2): the test peer's passive-role `encodeOpen` (`internal/test/peer/open.go`) is a seventh Optional Parameters writer. It already frames RFC 9072 correctly (extended when the sum of 2 + value length over its parameters exceeds 255, refused above the 4096-octet OPEN), but it writes a LIST of typed parameters, including non-capability parameters mirrored from ze's OPEN as raw bytes, where `PackOptionalParams` writes one type-2 parameter from a capability list. Which way: (a) the one packer takes typed parameters, so all seven share it; or (b) `encodeOpen` stays, with the reason (it reproduces ze's parameter layout, which the capability packer has no reason to accept) written above it | unanswered | Until answered, AC-1q and the no-layering grep name six writers and exclude `encodeOpen` by name |

### D2 refusal design (Q1 = b, 2026-10-13)

**Where declarations are compared on reload today: nowhere.** Read at the producers on 2026-10-13:

| Producer | What it does | Why it cannot refuse |
|----------|--------------|----------------------|
| `sdk.Plugin.SetCapabilities` (`pkg/plugin/sdk/sdk_callbacks.go`) | stores the set in `Plugin.capabilities`; `Plugin.Run` sends it once as `DeclareCapabilitiesInput` at stage 3 (`pkg/plugin/sdk/sdk.go`) | the stored set is the startup declaration and is never recomputed |
| `CapabilityInjector.AddPluginCapabilities` / `RemovePluginCapabilities` (`internal/component/plugin/registration.go`) | holds each plugin's set for `buildOpen`; removal only on plugin restart | the engine does not know a plugin's capability semantics, so it cannot compute the candidate set |
| `peerSettingsSwapPlan` (`internal/component/bgp/reactor/peer_settings_apply.go`), `Session.negotiationOutcomeUnchanged` (`peer_settings_negotiation.go`) | compare the reactor's own settings (`CapabilityConfigJSON`) and an OPEN built from the stage-3 store | they decide restart against swap; they see no plugin re-computation |
| `extractSoftverCapabilities`, `extractHostnameCapabilities`, `extractRoleCapabilities`, `extractLLNHCapabilities`, `extractGRCapabilities` and `extractLLGRCapabilities` | compute each plugin's declaration from a `bgp` config section; every entry's `Peers` selector is `configjson.CapabilitySelector` (the peer's name, or `group:<name>` for a dynamic group's template) | called only from `OnConfigure` (stage 2) |
| `capabilityDecls` (`internal/plugins/exabgp/bridgeplugin/internal.go`), called by `bridgeRunner.applyConfig` | route-refresh (code 2) and add-path (code 69) from the `exabgp.bridge` config; no `Peers` selector, so both apply to every peer | `applyConfig` returns nil once the bridge started ("family/capability changes need a plugin restart"), so a reload that changes them is accepted and silently not applied: the D2 class in a sixth plugin |

The external `internal/plugins/exabgp/main_sdk.go` also calls `SetCapabilities`,
but from command-line flags before `Run`, so no reload can change its set.

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

**One SDK registration, no per-plugin verify handler.** The plugin hands the SDK
its capability extractor once, before `Run`. The SDK runs it on the stage-2
sections and stores the result as the stage-3 declaration, which replaces each
plugin's own `SetCapabilities` loop. On config-verify, the SDK runs the plugin's
own verify handler first when one is registered (the `llnh` and `gr` refusals
keep answering first), then the extractor on the candidate sections, and
compares the two declarations under the verdict table below.

| Piece | Home | What it does |
|-------|------|--------------|
| `Plugin.DeclareCapabilitiesFrom` (new method; final name set at implementation) | `pkg/plugin/sdk/sdk_callbacks.go`, beside `SetCapabilities` | Registered before `Run`. Carries the config root the extractor reads, the extractor, and the plugin's capability leaf names by code. Stage 2: runs the extractor and stores its declaration in `Plugin.capabilities`; an extractor error fails stage 2 the way an `OnConfigure` error does. Config-verify: after the plugin's own verify handler, runs the extractor on the candidate and refuses a changed declaration. No RPC, engine or injector change |
| The extractor | each plugin | Input: the config sections of the registered root. Output: the declaration list and the candidate's peer selectors, or an error. A pure function of the sections (A-6) |
| `configjson.CapabilitySelectors` (new function) | `internal/component/bgp/configjson/traverse.go` | Every peer and dynamic-group selector of a `bgp` tree, from the same `ForEachPeer` and `CapabilitySelector` the extractors use, so the two sets cannot disagree; the `bgp`-rooted extractors return it as their selectors |
| Callers | `softver`, `hostname`, `role`, `llnh`, `gr` (`extractGRCapabilities` and `extractLLGRCapabilities`), the exabgp bridge | each deletes its `SetCapabilities` call and registers its extractor; `OnConfigure` keeps the plugin's other config work (for example `role`'s `filterPeerConfigs`). The bridge's extractor is `capabilityDecls` over the `exabgp.bridge` root, with no selectors |
| Leaf names | each plugin, beside its capability code constant | The `capability` leaf the operator wrote (`software-version`, `hostname`, `role`, the llnh and gr leaves, the bridge's `route-refresh` and `add-path`), so the refusal names configuration, not a code number |
| `SetCapabilities` | unchanged, for a declaration no config can change | its one remaining caller is the external `main_sdk.go` (flags). A plugin that both registers an extractor and calls `SetCapabilities` is refused at `Run`, so one plugin has one declaration path. The simpler design, deleting `SetCapabilities` and having `main_sdk.go` hand a constant extractor, was not taken: that plugin reads no config root, and the registration would gain a no-root case for one caller. Next review round to confirm |

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
stale plugin store. After this spec two reloads that work today are refused and
need a daemon restart:

| Reload | Why it is refused now |
|--------|-----------------------|
| A session family added or removed on a `gr` peer whose `graceful-restart` container has no `family` list | `grAdvertisedFamilies` (`gr_capability.go`) returns the session families when the list is absent, so the Graceful Restart payload changes |
| A peer added that carries a `softver`, `hostname`, `role`, `llnh` or `gr` capability | the injector holds no declaration for the new selector |

With an explicit `graceful-restart family` list, the GR payload is that list
(`grAdvertisedFamilies` returns it), so adding a session family leaves the
payload unchanged and the reload is accepted. Removing a session family the list
names is refused already, by `refuseUncarriedGRFamilies`, before this check. Both
restrictions are correct under Q1 = b (the alternative is a stale OPEN), and both
reduce what a reload can do; Known Limitations and the owner report state them.

Refusal text (`ai/rules/user-facing-errors.md`: problem first, why, then the
command on its own line). One line per changed peer and capability, sorted by
peer, then the fix once. Sample for one change:

| Line | Text |
|------|------|
| 1 | `peer 192.0.2.1: capability software-version cannot change while ze runs (plugin bgp-softver).` |
| 2 | `The plugin sends its capabilities only when ze starts, so ze would keep sending the old value in its OPEN to this peer.` |
| 3 | `To apply the change, stop the ze daemon and start it again. Under systemd, run:` |
| 4 | `  systemctl restart ze` |
| 5 | `Without systemd, run 'ze signal stop', then start ze again with 'ze start <config>'. On the ze appliance, run 'reboot' from the ze CLI.` |

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
| no supervisor | `ze signal stop`, then `ze start <config>` | `docs/guide/lifecycle.md` action table and start line |
| the appliance | `reboot` from the ze CLI | `SetRebootFunc` (`service_ssh.go`) sets `rebootRequested`, writes the GR marker through `stopForRestart`, and `hub.Run` calls `reboot.Reboot()` after the stop |

Trade-off stated: `systemctl restart ze` and `ze signal stop` end each session
with a normal teardown, so peers withdraw Ze's routes until it is back. A
restart that keeps the Graceful Restart marker under systemd is `ze signal
restart`, a wait until the process has exited, then `systemctl start ze`; the
refusal does not print it, because a start issued before the exit is a no-op,
and `docs/guide/plugins.md` carries it. Review item (unverified here, the
gokrazy source is not in this module): if gokrazy restarts ze after a 0 exit,
`ze signal restart` alone is the appliance route and keeps the marker, and line
5 names it instead of `reboot`.

Interop: the refusal changes no wire behaviour (the running session keeps its
startup OPEN), so it falls under the config-only exemption of
`ai/rules/interop-and-goal-validation.md`, and no reload interop scenario is
owed. Goal validation is the `.ci` through the real reload entry
point.

### D1 consolidation design (Q4 = Consolidate, 2026-10-13)

The six packers, read at the producers on 2026-10-13 (the last two found by the
review of 2026-10-13, ISSUE-2; a tree grep for a type-2 parameter written with
a one-octet length finds these six and no other, and `encodeOpen` is Q5):

| Packer | Framing today | Defect |
|--------|---------------|--------|
| `buildOptionalParams` (`internal/component/bgp/reactor/session_negotiate.go`), called by `Session.buildOpen` | one type-2 parameter; one-octet form while `capTotal <= 255`; two-octet form and `ExtendedParams` above; `slog.Error` and nil parameters when `capTotal > maxExtendedParamLen` (65535) | D1 at capTotal 254 and 255; the upper bound lets capTotal 65533 to 65535 through although the Extended Opt. Parm. Length (3 + capTotal) then exceeds 65535 |
| `buildOptionalParams` (`internal/chaos/peer/session.go`), called by chaos `BuildOpen` | one type-2 parameter per capability, one-octet lengths, `ExtendedParams` never set | D1 sibling above 255 octets |
| `packOptionalParams` (`internal/perf/session.go`), called by perf `BuildOpen` | one type-2 parameter per capability, one-octet lengths | the same shape; the perf set (three capabilities) cannot reach 255 |
| inline packing in `fabricateLocRIBOpen` (`internal/component/bgp/plugins/bmp/bmp_locrib.go`), constant `optParamCapabilities` | one type-2 parameter, one-octet length | none reachable: four capabilities of 8 octets or fewer |
| inline packing in `bgpBuildOpen` (`internal/analyze/bgpconn.go`), called by `ze analyze`'s `inject.go` and `serve.go` | one type-2 parameter, one-octet length, three fixed capabilities (MP IPv4, MP IPv6, 4-octet AS) | none reachable: 18 octets |
| inline packing in `buildActiveOpen` (`internal/test/peer/inject.go`), the test peer's active-role OPEN | one type-2 parameter, one-octet length (MP and 4-octet AS) | none reachable: a fixed small set |

**Home: `internal/core/bgp/capability`, as `capability.PackOptionalParams`.** It
is a core library (`ai/rules/architecture.md`: no config lifecycle, no registry
side effect) whose non-test imports are `internal/core/family` and
`internal/core/textbuf` only (`go list`, 2026-10-13); and it holds the inverse,
`ParseFromOptionalParams(optParams, extended)`, so encoder and decoder sit
together. Five of the six callers already import it; `internal/analyze` does not
(`go list`: it imports `internal/core/bgp/asn`, `msgtype` and other
`internal/core` packages) and gains this one `internal/core` import, the same
direction it already takes. `./le arch tier check` (the tool behind the
module-tier rule; `./le arch rules` does not exist) exits 2 at today's tree on
violations no touched package is in (for example
`internal/component/bgp/reactor/reactor.go` importing
`internal/component/bgp/plugins/cmd/cache`, review of 2026-10-13), so the proof
owed is that its report names neither `internal/core/bgp/capability` nor any of
the six changed caller packages, with its "core import direction clean" line.
`internal/component/bgp/message` was rejected: it does not import `capability`
today, and the packer would add that edge for no reader.

The packer, as steps:

1. No capabilities: no parameters, base form.
2. capTotal is the sum of the capabilities' lengths.
3. While 2 + capTotal is at most 255: one type-2 parameter with a one-octet Parameter Length; extended false (RFC 4271 / RFC 5492 form; RFC 9072 Section 2 SHOULD).
4. While 3 + capTotal is at most 65535: one type-2 parameter with a two-octet Parameter Length; extended true (RFC 9072 Section 2 MUST).
5. Above that: an error naming the size and the limit, no parameters. Today's `slog.Error` with nil parameters is a silently wrong OPEN (`ai/rules/principles.md`); `buildOpen` already returns an error, so the reactor propagates it. R-4 keeps the separate 4096-octet OPEN limit.

| Caller | Change |
|--------|--------|
| `Session.buildOpen` | calls the packer, sets `OptionalParams` and `ExtendedParams`, returns its error |
| chaos `BuildOpen` | calls the packer and sets `ExtendedParams`; the chaos peer's OPEN moves from one parameter per capability to one parameter for all, which every RFC 5492 receiver accepts |
| perf `BuildOpen` | the same; `TestBuildOpenIPv6` walks the parameters and must read the one-parameter form |
| `fabricateLocRIBOpen` | calls the packer; bytes unchanged; `optParamCapabilities` is deleted (its only reader is this packing, grep 2026-10-13) |
| `bgpBuildOpen` | calls the packer; bytes unchanged (one parameter of 18 octets); gains an error return |
| `buildActiveOpen` | calls the packer; bytes unchanged; gains an error return, and `inject.go`'s handshake returns it as `Result{Error: ...}` the way its `write OPEN` failure does |

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

The chaos peer and the active test peer now share Ze's encoder, so an encoder
bug would reach them too; the decoder (`ParseFromOptionalParams`) is separate
code, so the AC-1b and AC-1q round trips still catch an asymmetric error.

Deleted, no layering: reactor `buildOptionalParams` and `maxExtendedParamLen`,
chaos `buildOptionalParams`, perf `packOptionalParams`, the bmp inline packing,
the `bgpBuildOpen` inline packing, the `buildActiveOpen` inline packing.

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
| `plan/pre-release/spec-rfc-verdict-fix-bgp.md` | 2 rows point here | RFC6286-2.1-1 and RFC9072-2-1, both `weak`; the parent's Wiring note names this spec's AC-4 |
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
  → Decision: `buildActiveOpen`'s packing is deleted and it calls the one packer (Q4); `encodeOpen` waits on Q5
- [ ] `internal/plugins/exabgp/bridgeplugin/internal.go` - `capabilityDecls` (route-refresh, add-path, no `Peers`), `bridgeRunner.applyConfig` (returns nil once started)
  → Decision: its extractor is `capabilityDecls`, registered with the SDK like the five bgp plugins (Q1 = b)
- [ ] `internal/core/bgp/capability/capability.go` - `ParseFromOptionalParams(optParams, extended)`, the decoder; package imports only `internal/core/family` and `internal/core/textbuf`
  → Decision: home of `PackOptionalParams` (Q4)
- [ ] `pkg/plugin/sdk/sdk_callbacks.go`, `pkg/plugin/sdk/sdk.go` - `SetCapabilities` stores the set in `Plugin.capabilities` for stage 3; `OnConfigVerify` receives the full candidate sections inside the reload transaction
  → Decision: home of `Plugin.DeclareCapabilitiesFrom`, the one capability registration that runs the extractor at stage 2 and at config-verify (Q1 = b)
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

**Behavior to change:** D1 and its sibling in the one packer (Q4), D2 as a refusal (Q1 = b), D3, and D4 as the documented deviation (Q2); the false RFC6286-2.1-1 annotation. The chaos and perf OPENs move from one parameter per capability to one parameter for all.

**New restriction (Q1 = b), stated as one:** a reload that today is accepted is refused and needs a daemon restart in two cases: a session family added or removed on a `gr` peer with no `graceful-restart family` list (the GR payload follows the session families, `grAdvertisedFamilies`), and a peer added that carries a plugin-declared capability (`softver`, `hostname`, `role`, `llnh`, `gr`). A reload that changes the exabgp bridge's `route-refresh` or `add-path` is refused too; today it is accepted and not applied.

## Data Flow (MANDATORY - see `ai/rules/architecture.md`)

### Entry Point
- Session start: `sendOpen` → `buildOpen` (D1)
- SIGHUP / `ze config` commit: the config transaction (verify, apply, operation graph) (D2, D4)
- A NOTIFICATION read in OpenSent or OpenConfirm (D3)
- Config load: `parsePeerSettings` (D4)

### Transformation Path
1. Config load: plugins receive the `bgp` section at stage 2 and declare per-peer capabilities at stage 3; the engine stores them in `CapabilityInjector`
2. `buildOpen` merges configured capabilities with `pluginCapGetter` (injector), coalesces PATHS-LIMIT, calls `capability.PackOptionalParams`, returns `message.Open`; the chaos peer, the perf sender, the BMP Loc-RIB OPEN, `ze analyze` and the test peer's active-role OPEN call the same packer
3. `Open.WriteTo` chooses the envelope; `writeMessage` sends it
4. Reload: verify sections to plugins (in each capability plugin the SDK runs the plugin's own verify handler, then the registered extractor on the candidate, and refuses a changed declaration, Q1 = b), apply coarse plugin sections, then peer operations; a changed capability block the reload accepted restarts the peer, and the new connection repeats 2-3
5. Receive: `handleNotification` counts, reports, runs the FSM event, closes; the peer loop reconnects after backoff

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Plugin ↔ Engine | stage-3 `declare-capabilities`, unchanged; the refusal travels as the config-verify answer the plugin already returns | Yes, by reading `OnConfigVerify` and `SetCapabilities` |
| Engine ↔ Reactor | `pluginCapGetter` reads the injector per OPEN | Yes, by reading `buildOpen` |
| Ze ↔ peer | OPEN wire bytes | Yes, by reading `Open.WriteTo` |

### Integration Points
- `sdk.Plugin.DeclareCapabilitiesFrom` - the one capability registration: the SDK runs the plugin's extractor at stage 2 for the declaration and at config-verify for the comparison (Q1 = b)
- `configjson.CapabilitySelectors` - the candidate's peer selectors (Q1 = b)
- The six capability plugins (`softver`, `hostname`, `role`, `llnh`, `gr`, the exabgp bridge) - each registers its extractor; none writes a comparison
- `capability.PackOptionalParams` - the one Optional Parameters packer for six callers (Q4)

### Architectural Verification
| Check | Holds? | Evidence |
|-------|--------|----------|
| No bypassed layers | Yes (design) | capabilities still reach the OPEN only through the injector and `buildOpen` |
| No unintended coupling | Yes (design) | the reactor does not learn plugin capability semantics; plugins still own their declarations |
| No duplicated functionality | Yes (design) | one Optional Parameters packer (Q4; `encodeOpen` per Q5); one capability extractor per plugin, run by the SDK for both the stage-2 declaration and the verify comparison, so the two cannot be computed differently (Q1 = b) |
| Zero-copy preserved | Yes | the OPEN path allocates once per connection today; the packer keeps that one allocation, on a cold path |
| Registration over hardcoding, outbound | Yes (design) | each plugin registers its own extractor with the SDK; no per-plugin case in the engine or the SDK |
| Registration over hardcoding, inbound | Yes (design) | the SDK reads the plugin's own name, its registered extractor and the stage-3 set; the leaf names live in each plugin beside its code, no central list learns a plugin or a capability |
| Module tiers | Yes (design) | `internal/core/bgp/capability` gains no import; `internal/analyze` gains one `internal/core` import; `./le arch tier check` names no touched package (it exits 2 today on unrelated violations) |

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| Session start with a capability set of 254 octets | → | `buildOpen` → `capability.PackOptionalParams` → `Open.WriteTo` | `test/encode/open-optional-params-254.ci` (new: ze-peer `inspect-open-message` on the exact extended OPEN hex) |
| SIGHUP with a changed softver `encoding` | → | reload verify → SDK config-verify → softver's registered extractor → comparison with the stage-3 declaration → refusal | `test/reload/reload-softver-encoding-refused.ci` (new; model `test/reload/bgp-bfd-disable-refused-live.ci`: the reload is refused naming `bgp-softver`, the peer and `software-version`, and the session stays up on one TCP connection) |
| SIGHUP adding a peer with `software-version` | → | same path, new selector | `test/reload/reload-add-peer-softver-refused.ci` (new) |
| ze-peer answers Ze's OPEN with NOTIFICATION 2/0 | → | `handleNotification` WARN line | `test/plugin/open-refused-notification-warn.ci` (new: `expect=stderr` on the WARN) |

Q2 = deviation (answered 2026-10-13) leaves no config refusal for a peer-level `router-id`, so no wiring row is owed for it.

## Acceptance Criteria

AC-1 to AC-4 keep the skeleton's intent. AC-2's either/or is resolved by Q1
(owner, 2026-10-13: "b", refuse); AC-3 is narrowed from "WARN or INFO" to WARN
because the default level is WARN.

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1q | The reactor, the chaos peer, the perf sender, the BMP Loc-RIB OPEN, `ze analyze`'s OPEN and the test peer's active-role OPEN (Q4) | Each builds its Optional Parameters through `capability.PackOptionalParams`; reactor `buildOptionalParams` and `maxExtendedParamLen`, chaos `buildOptionalParams`, perf `packOptionalParams`, and the inline packing in `fabricateLocRIBOpen`, `bgpBuildOpen` and `buildActiveOpen` no longer exist; a tree grep finds no other type-2 parameter writer outside the packer, except `encodeOpen` pending Q5; the BMP Loc-RIB, `ze analyze` and active test peer OPEN bytes are unchanged; no `./le arch tier check` violation names `internal/core/bgp/capability` or a touched caller package |
| AC-1x | capabilities totalling 65533 octets or more | The packer returns an error naming the size and the limit, and `buildOpen` returns it; no OPEN with missing capabilities is sent |
| AC-1 | `buildOpen` with capabilities totalling 254 or 255 octets (and 256) | The OPEN carries Non-Ext OP Len 255, Non-Ext OP Type 255, Extended Opt. Parm. Length = 3 + capTotal, and one type-2 parameter whose two-octet Parameter Length = capTotal; a conforming decode (`message.UnpackOpen` then `capability.ParseFromOptionalParams`) returns every capability unchanged |
| AC-1n | capabilities totalling 253 octets (and 0, 1) | The OPEN uses the RFC 4271 form: Opt Parm Len = 2 + capTotal (255 at 253), no 255/255 marker, one-octet Parameter Length; bytes identical to today |
| AC-1b | chaos test peer with a capability set whose parameters exceed 255 octets | The chaos peer's OPEN round-trips through `message.UnpackOpen` and `capability.ParseFromOptionalParams` with every capability intact (its parameters use the RFC 9072 two-octet framing whenever the envelope is extended) |
| AC-2 | SIGHUP that changes `capability software-version encoding` on a running peer (legacy → draft) | The reload is refused with the text of "D2 refusal design": it names the peer, `software-version`, plugin `bgp-softver`, that the change cannot apply while ze runs because ze would keep sending the old value in its OPEN, and `systemctl restart ze` on its own line, followed by the unsupervised and appliance routes; it never names `ze signal restart`. Nothing is applied: the session is not restarted and keeps its startup OPEN |
| AC-2b | SIGHUP that adds a new peer with `software-version` (own or inherited from its group) | Refused the same way, naming the new peer |
| AC-2c | SIGHUP that removes `software-version` from a peer that stays configured | Refused the same way |
| AC-2d | The same changes for the `hostname`, `role`, `llnh` and `gr` capabilities, including a session family added on a `gr` peer whose `graceful-restart` container has no `family` list (the GR payload follows the session families) | Each is refused naming its plugin and its `capability` leaf |
| AC-2g | A session family added on a `gr` peer with an explicit `graceful-restart family` list that does not name it | Accepted: the GR payload is the explicit list and does not change; the peer restarts as today for the capability-block change |
| AC-2f | SIGHUP that changes the exabgp bridge's `route-refresh` or `add-path` | Refused naming `every peer`, plugin `exabgp-bridge` and the leaf |
| AC-2e | A reload that leaves every plugin-declared capability unchanged, one that only removes a peer, one that adds a peer with no plugin capability, and one that adds a peer while the exabgp bridge declares an unchanged all-peer set | Each is accepted; no refusal line appears |
| AC-3 | A NOTIFICATION received in OpenSent or OpenConfirm (Ze's OPEN refused) | Exactly one WARN line per received NOTIFICATION naming the peer, the FSM state, the code and subcode as names and numbers, and the RFC 8203 message when present; visible with no `ze.log` override |
| AC-3n | A NOTIFICATION received in Established | The same one WARN line (one line per NOTIFICATION, whatever the state); the existing INFO "peer shutdown communication" line is folded into it, not duplicated (no-layering) |
| AC-4 | The `weak` verdicts of RFC9072-2-1 and RFC6286-2.1-1, after this spec's producer fix | RFC9072-2-1 reaches `enforced`: a tagged test proves the quoted sentence, and an agent that did not write that test re-judges it with `./le rfc audit-stamp ... mode rejudge`. RFC6286-2.1-1 carries the deviation record (`{partial}`, reason, not counted conformant), and its verdict is re-judged against that record. Moved here from "Blocked by" in `plan/pre-release/spec-rfc-verdict-fix-bgp.md` (parent P-3, 2026-09-28) |
| AC-5 | `rfc/short/rfc6286.md` RFC6286-2.1-1 annotation and the `docs/features/rfc-status.md` RFC 6286 row | Neither claims the identifier is the same for every peer while the per-peer override exists; done FIRST, before any code, because `ai/rules/rfc-compliance.md` does not let a false claim wait |
| AC-6 | A config with a peer-level `router-id` (Q2 = deviation) | Accepted as today; the YANG description and `docs/guide/configuration.md` say the override departs from RFC 6286 Section 2.1 |
| AC-7 | `plan/journal/unwired-feature.md` (2026-09-27) and `plan/journal/silent-fall-through.md` (2026-09-27) rows | Marked fixed with the commit, in the closing commit |

## End-to-End User Stories

| # | User does | Path through system | Test proving it works |
|---|-----------|--------------------|-----------------------|
| 1 | Configures enough families and capabilities that the set is 254 octets, peers with FRR | config → plugins stage 3 → `buildOpen` → `capability.PackOptionalParams` → `Open.WriteTo` → FRR | `bgp-open-extended-params-boundary-frr` |
| 2 | Changes `software-version encoding` and sends SIGHUP | transaction verify → SDK config-verify → softver's registered extractor → comparison → refusal naming the restart; the session stays up | `reload-softver-encoding-refused.ci` |
| 3 | Peers with a speaker that refuses Ze's OPEN, reads the log | wire NOTIFICATION → `handleNotification` → WARN | `open-refused-notification-warn.ci`, `frr-software-version` (log assertion added) |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestRFC9072OptionalParamsBoundary` (table: capTotal 0, 1, 253, 254, 255, 256, 300) | `internal/component/bgp/reactor/rfc9072_optional_params_boundary_test.go` (new) | AC-1, AC-1n: builds the OPEN through `buildOpen` (which reaches `capability.PackOptionalParams`) plus `Open.WriteTo`, reads the envelope octets and walks the parameters with the framing the envelope announces; tagged `RFC9072-2-1 positive` (254, 255, 256 extended with two-octet Parameter Length) and `RFC9072-2-1 negative` (253 and below carry no marker and one-octet length; the tag quotes the Section 2 SHOULD sentence, see the audit table below) | |
| `TestPackOptionalParams` (table: empty, one capability, several, capTotal 253, 254, 65532, 65533) | `internal/core/bgp/capability/optional_params_test.go` (new) | the packer's own framing and flag, round-tripped through `ParseFromOptionalParams`, and the AC-1x error; replaces `TestBuildOptionalParams_Empty`, `_SingleCap`, `_MultipleCaps` (`reactor/session_negotiate_test.go`, untagged), which are deleted with the function they call | |
| `TestChaosPeerOpenRoundTripsAbove255` | `internal/chaos/peer/session_test.go` | AC-1b, and that the chaos peer reaches the one packer | |
| `TestPerfOpenUsesOnePacker` | `internal/perf/session_test.go` | AC-1q for perf: the OPEN round-trips through `message.UnpackOpen` and `capability.ParseFromOptionalParams`; `TestBuildOpenIPv6` reads the one-parameter form | |
| `TestAnalyzeOpenUsesOnePacker` | `internal/analyze/bgpconn_test.go` (new) | AC-1q for `ze analyze`: `bgpBuildOpen`'s bytes are unchanged and round-trip through `capability.ParseFromOptionalParams` | |
| `TestActiveOpenUsesOnePacker` | `internal/test/peer/inject_test.go` | AC-1q for the active test peer: `buildActiveOpen`'s bytes are unchanged and round-trip | |
| existing bmp Loc-RIB OPEN tests (the tagged ones are listed under "RFC-tagged tests that change") | `internal/component/bgp/plugins/bmp/` | AC-1q for BMP: every direct caller of `fabricateLocRIBOpen` handles its new error return, and the OPEN bytes stay identical | |
| `TestDeclareCapabilitiesFrom` (table over the six rows of the D2 verdict table) | `pkg/plugin/sdk/` | AC-2 to AC-2g at the comparison: changed payload, removed code on a live peer, new code, new peer refused; unchanged and peer-removed accepted; an all-peer declaration changed is refused naming `every peer`, unchanged with a peer added is accepted; the stage-2 declaration is the extractor's output; the plugin's own verify handler answers first; a plugin that registers an extractor and calls `SetCapabilities` is refused at `Run`; the text names plugin, peer, leaf and `systemctl restart ze` and never `ze signal restart` | |
| `TestCapabilitySelectors` | `internal/component/bgp/configjson/` | standalone, grouped and dynamic-group-template selectors, the same strings the extractors emit | |
| `TestSoftverCapabilityExtractorRegistered` and the same for `hostname`, `role`, `llnh`, `gr` and the exabgp bridge | each plugin's `_test.go` | each plugin registers its extractor, the SDK refuses a change of its own capability and accepts an unchanged one (AC-2, AC-2d, AC-2e, AC-2f, AC-2g); for `llnh` and `gr` the existing refusal still answers first | |
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
| `open-refused-notification-warn` | `test/plugin/` | ze-peer answers the OPEN with 2/0; `expect=stderr:contains=` the WARN with peer, state, code, subcode; no `ze.log` override | |

### Interop Tests (Scope: protocol)
| Scenario | Directory | Peer Daemon | What It Proves | Status |
|----------|-----------|-------------|----------------|--------|
| `bgp-open-extended-params-boundary-frr` (new) | `test/interop/scenarios/` | FRR | Ze's OPEN at capTotal 254 (sized with `software-version` padding plus families) reaches Established with FRR and FRR reports Ze's capabilities (D1). Red phase: revert the threshold, rebuild the Ze image, FRR refuses the OPEN | |
| `frr-software-version` (extended) | `test/interop/scenarios/frr-software-version/` | FRR | `opWaitLogContains` on Ze's WARN naming FRR's 2/0 for peer draft (D3) | |

## Files to Modify
- `internal/component/bgp/reactor/session_negotiate.go` - `buildOptionalParams` and `maxExtendedParamLen` deleted; `buildOpen` calls `capability.PackOptionalParams` and returns its error (Q4)
- `internal/chaos/peer/session.go` - chaos `buildOptionalParams` deleted; `BuildOpen` calls the packer, sets `ExtendedParams`, returns the error (Q4)
- `internal/perf/session.go` - `packOptionalParams` deleted; `BuildOpen` calls the packer and returns the error (Q4)
- `internal/component/bgp/plugins/bmp/bmp_locrib.go` - inline packing and `optParamCapabilities` deleted; `fabricateLocRIBOpen` calls the packer and returns the error (Q4)
- `internal/analyze/bgpconn.go` - `bgpBuildOpen`'s inline packing deleted; it calls the packer and returns the error (Q4)
- `internal/test/peer/inject.go` - `buildActiveOpen`'s inline packing deleted; it calls the packer and returns the error (Q4)
- the callers of chaos `BuildOpen`, perf `BuildOpen`, `fabricateLocRIBOpen`, `bgpBuildOpen` and `buildActiveOpen` - handle the new error (table in "D1 consolidation design")
- `internal/component/bgp/message/open.go` - the `writeToExtended` comment "optLen validated ≤ maxOptLen (65535)" names a validation no code performs today; after Q4 the packer performs it for every caller of the packer, so the comment names `capability.PackOptionalParams` (stale-comments rule)
- the reactor and bmp tests listed under "RFC-tagged tests that change" - calls move to the packer, or handle the new error
- `pkg/plugin/sdk/sdk_callbacks.go` (and `sdk.go` for the stage-2 run and the `Run` refusal) - `Plugin.DeclareCapabilitiesFrom` (Q1 = b)
- `internal/component/bgp/configjson/traverse.go` - `CapabilitySelectors` (Q1 = b)
- `internal/component/bgp/plugins/softver/softver.go`, `hostname/hostname.go`, `role/role.go`, `llnh/llnh.go`, `gr/gr.go` (and `gr_capability.go`), `internal/plugins/exabgp/bridgeplugin/internal.go` - `SetCapabilities` call deleted, extractor registered, leaf names beside the codes (Q1 = b); the bridge's `applyConfig` comment ("family/capability changes need a plugin restart") is replaced by the refusal it now gets
- `internal/component/bgp/reactor/peer_settings_negotiation.go` - the `negotiatedCapabilitySettings` comment that records the stale plugin store: it now says the plugins refuse that reload (stale-comments rule)
- `internal/component/bgp/reactor/session_handlers.go` - D3 WARN
- `internal/component/bgp/yang/ze-bgp-conf.yang` - the `peer-fields` `router-id` description says the override departs from RFC 6286 Section 2.1 (Q2 = deviation, AC-6)
- `docs/guide/configuration.md` - the peer-level `router-id` text says the same (AC-6)
- `rfc/short/rfc6286.md`, `rfc/short/rfc9072.md`, `rfc/audit/rfc6286.json`, `rfc/audit/rfc9072.json`
- `rfc/discrimination/rfc9072.json`, `rfc9069.json`, `rfc2918.json`, `rfc7705.json`, `rfc8050.json`, `rfc4659.json`, `draft-ietf-idr-bgp-bfd-strict-mode.json` - the re-records and first records of "Discrimination records Q4 stales"
- `plan/journal/unwired-feature.md`, `plan/journal/silent-fall-through.md`

## Files to Create
- `internal/core/bgp/capability/optional_params.go` - `PackOptionalParams`, with the RFC 9072 Section 2 and RFC 5492 Section 4 quotes (Q4)
- the unit tests, `.ci` tests and interop scenarios named above, each with its typed checker in `internal/le/interoplab/bgp`

### Integration Checklist
| Integration Point | Applies? | File / reason |
|-------------------|----------|---------------|
| YANG schema (new RPCs/config) | No | no leaf added or removed; the `peer-fields` `router-id` description changes only (Q2 = deviation) |
| YANG validation constraints | No | no new leaf |
| YANG custom validators | No | no new leaf |
| CLI commands/flags | No | none added |
| CLI grammar (keyword before value) | N-A | no command |
| Editor autocomplete | No | the schema keeps every leaf |
| Functional test for new RPC/API | No new RPC | the refusal travels in the existing config-verify answer; the `test/reload/` `.ci` files exercise it |
| Pipe completeness | N-A | no command output |
| Env var registration | No | none |
| Doctor check for runtime dependencies | N-A | no file, socket, port, module or binary added |
| Prometheus counters/metrics | No | the NOTIFICATION counters exist; D3 adds a log line only |
| BGP family surface (new SAFI / capability / attribute) | N-A | no new capability code; existing capabilities change only in when they are declared |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature, or scope changed? | No | defects |
| 2 | Config syntax changed? | No syntax change; the peer-level `router-id` text changes (Q2 = deviation) | `docs/guide/configuration.md` and the `ze-bgp-conf.yang` `peer-fields` `router-id` description: the override departs from RFC 6286 Section 2.1 (AC-6) |
| 3 | CLI command added/changed? | No | - |
| 4 | API/RPC added/changed? | No | the engine RPC is plugin protocol, row 8 |
| 5 | Plugin added/changed? | Yes | `docs/guide/plugins.md`: a capability plugin refuses a reload that changes its capability, the two new restrictions (a family edit on a `gr` peer with no `graceful-restart family` list, a peer added with a plugin capability), and how to restart: `systemctl restart ze`; without systemd `ze signal stop` then `ze start <config>`; on the appliance `reboot`; and the Graceful-Restart-keeping sequence (`ze signal restart`, wait for the exit, `systemctl start ze`). `docs/plugin-development/` page for the SDK: a plugin whose capabilities come from config registers its extractor and never calls `SetCapabilities` |
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
| 16 | Changed sources referenced by doc anchors? | Derived at implementation | `./le spec citation anchors spec plan/immediate/spec-bgp-open-session-rfc-defects.md`. Declared by `// Design:` headers of files this spec changes: `docs/architecture/chaos-web-dashboard.md` (`internal/chaos/peer/session.go`): unaffected, it describes the chaos dashboard, not the chaos peer's OPEN framing; `docs/architecture/core-design.md` (`internal/component/bgp/reactor/config.go`): unaffected by D1-D3 (it states no capability-declaration or router-id rule); `rfc/short/rfc9069.md` (`internal/component/bgp/plugins/bmp/bmp_locrib.go`): unaffected, the Loc-RIB OPEN keeps its capabilities and bytes; `docs/architecture/wire/capabilities.md` (`internal/core/bgp/capability/capability.go`): gains the one packer and its 254-octet boundary (row 7); `docs/architecture/api/process-protocol.md` (`pkg/plugin/sdk/sdk_callbacks.go`): gains the refusal sentence (row 8); `docs/architecture/config/syntax.md` (`internal/component/bgp/configjson/traverse.go`): re-read for the selector helper, expected unaffected; `internal/perf/session.go` declares none; `docs/architecture/wire/messages.md` (`internal/component/bgp/message/open.go`): its RFC 9072 paragraph ("When the Optional Parameters exceed 255 octets ... Ze writes the RFC 9072 extended form") stays true and gains that the one packer chooses the form at 2 + capTotal above 255 and refuses above 65535; `docs/architecture/mrt.md` (`internal/analyze/bgpconn.go`): unaffected, the `ze analyze` OPEN bytes do not change; `docs/architecture/testing/ci-format.md` (`internal/test/peer/inject.go`, `internal/test/peer/open.go`): unaffected, the active test peer's OPEN bytes do not change and `encodeOpen` waits on Q5; `docs/architecture/exabgp-bridge.md` (`internal/plugins/exabgp/bridgeplugin/internal.go`): its "Stage ordering constrains the family declaration" section gains that a reload changing the bridge's capabilities is refused |
| 17 | Existing docs show examples for this area? | No | the peer-level `router-id` stays (Q2 = deviation), so no example changes |

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis (file/doc/user statement) | If wrong | Validated by | Status |
|----|-----------|--------------------------------|----------|--------------|--------|
| A-1 | Every capability plugin receives config-verify for its root (`bgp`, or `exabgp` for the bridge) on a reload, before any apply, whether or not it registered its own verify handler | `OnConfigVerify` doc ("full candidate config sections"); `llnh` and `gr` already refuse there | the SDK's comparison never runs for a plugin without a verify handler; the SDK registers the config-verify callback itself when an extractor is registered, and the engine's verify dispatch must reach every plugin that declared capabilities | `reload-softver-encoding-refused.ci`, `reload-exabgp-bridge-capability-refused.ci` | unvalidated |
| A-2 | The live-stages reload fixture can assert a refusal's text, as `bgp-bfd-disable-refused-live.ci` does | that test's header ("the reload is refused, naming the peer") | fixture extension, `docs/functional-tests.md` row | first D2 `.ci` | unvalidated |
| A-3 | `Plugin.capabilities` still holds the stage-3 set when config-verify runs | today `SetCapabilities` is its only writer (`sdk_callbacks.go`); after this spec the stage-2 extractor run is its second writer, and nothing writes it after stage 3 | the comparison runs against a wrong baseline | `TestDeclareCapabilitiesFrom` | unvalidated |
| A-4 | ze-peer can answer Ze's OPEN with a NOTIFICATION before or instead of a KEEPALIVE (`action=notification` at conn 1 seq 1) | `expect.go` action list | a ze-peer `open` option to refuse | first D3 `.ci` | unvalidated |
| A-5 | FRR accepts an RFC 9072 OPEN of 254+ octets and reaches Established | FRR implements RFC 9072 (`bgp_open.c` extended opt params) | interop scenario uses another daemon (GoBGP, BIRD) | `bgp-open-extended-params-boundary-frr` | unvalidated |
| A-6 | `softver`, `hostname`, `role`, `llnh`, `gr` and the exabgp bridge derive their declaration only from the config the reload delivers (the review of 2026-10-13 found no `os.Hostname`, clock or random read in the five bgp extractors; `capabilityDecls` reads only `bridgeConfig`), so the same config yields the same declaration | each plugin's `OnConfigure` | a declaration that reads host state (a hostname default, say) could differ with no config change and refuse a reload the operator did not change; that plugin needs its own design | Phase 4 per plugin | unvalidated |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | The refusal fires on a reload the operator thinks harmless and that is accepted today (a family edit on a `gr` peer with no `graceful-restart family` list; a peer added that carries a plugin capability) | operator report | a new restriction, correct under Q1 = b: the restarted OPEN would carry the stale plugin store. The text says why and names the restart; `docs/guide/plugins.md` and Known Limitations state it; an explicit `graceful-restart family` list avoids the first case |
| R-2 | The candidate selectors and the declared selectors disagree in form, so an unchanged peer reads as removed and added | `TestCapabilitySelectors`, AC-2e red | both come from `configjson.CapabilitySelector`; no second spelling |
| R-4 | An OPEN above RFC 8654's 4096 octets is still sent | code reading | the one packer now refuses above 65532 octets (AC-1x), which closes the `maxExtendedParamLen` half; the 4096-octet half gets one row in `plan/journal/silent-fall-through.md` at implementation start, not fixed here (rule-precedence: closing first) |
| R-5 | The D3 WARN floods the log when a peer refuses every retry | ConnectRetry cadence in `frr-software-version` (nine refusals) | one line per received NOTIFICATION is bounded by the reconnect backoff; FRR and BIRD log every received NOTIFICATION at their default level too |
| R-7 | The chaos or perf OPEN's move to one parameter for all capabilities changes a byte expectation somewhere | a chaos or perf test red | every RFC 5492 receiver accepts it; correct the expectation, which asserted framing no RFC requires |

## Blast Radius

| Question | Answer |
|----------|--------|
| What breaks if this is wrong? | D1: OPENs of 254-255 octets (today broken) and, if the threshold were moved wrongly, every OPEN; the encode/decode `.ci` corpus pins the common sizes. D2: capability plugins on reload, and every capability plugin's stage-2 declaration, which now runs through the SDK registration. D3: log only. D4: documentation and the deviation record only |
| How is it reverted? | Single commit per row |
| Who else touches this path? | `plan/spec-bgp-fsm-delayopen-and-automatic-start.md` (OPEN and session handling), `spec-bgp-live-receive-binding-and-bfd-keep-session` (reload swap plan in `peer_settings_apply.go`, closed 2026-10-13), `plan/pre-release/spec-rfc-verdict-fix-bgp.md` (verdict rows), `plan/pre-release/spec-config-yang-when-unique-choice.md` (router-id mandatory ruling) |

## Implementation Steps

1. **Phase: Wiring and the false claim (FIRST)** - AC-5 edit of `rfc/short/rfc6286.md` and the rfc-status row; write the failing wiring tests of the Wiring Test table
   - Verify: each new test fails for the stated reason
2. **Phase: D1 (Q4 = Consolidate)** - `TestPackOptionalParams` and `TestRFC9072OptionalParamsBoundary` red at 254/255; delete reactor `buildOptionalParams` and `maxExtendedParamLen`, then write `capability.PackOptionalParams` with the RFC 9072 Section 2 quote above the threshold; move `buildOpen`, then delete chaos `buildOptionalParams`, perf `packOptionalParams`, and the inline packing in `fabricateLocRIBOpen`, `bgpBuildOpen` and `buildActiveOpen`, and move their callers, with the new error handled; `encodeOpen` per Q5; correct the `writeToExtended` comment; move the tests of "RFC-tagged tests that change" (owner runs the seven approval commands); green; `./le arch tier check` names no touched package; `./le rfc discriminate-record` for each new RFC9072-2-1 / RFC9072-2-7 tag and for every row of "Discrimination records Q4 stales" plus the two first records; `open-optional-params-254.ci`
3. **Phase: D2 registration (Q1 = b)** - `TestDeclareCapabilitiesFrom` and `TestCapabilitySelectors` red; `Plugin.DeclareCapabilitiesFrom` (stage-2 run, config-verify comparison after the plugin's own handler, `Run` refusal of a plugin that also calls `SetCapabilities`), `configjson.CapabilitySelectors`; green
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
| Rule: no-layering | the INFO RFC 8203 line is replaced, not kept beside the WARN; no Optional Parameters packing survives outside `capability.PackOptionalParams`: a tree-wide grep over `internal`, `pkg` and `cmd` for a type-2 parameter write finds only the packer (and `encodeOpen` while Q5 is open); no capability plugin whose set comes from config still calls `SetCapabilities` |
| Correctness | the refusal compares per selector and per code, treats an empty `Peers` as one "every peer" selector, accepts a removed peer, and names plugin, peer, leaf and `systemctl restart ze`, never `ze signal restart` |

### Deliverables Checklist
| Deliverable | Verification method |
|-------------|---------------------|
| D1 fixed | `TestRFC9072OptionalParamsBoundary` and `open-optional-params-254.ci` pass; revert red recorded |
| D1 consolidated | one packer, six callers; no `./le arch tier check` violation names a touched package; every row of "Discrimination records Q4 stales" re-recorded and the two first records written |
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
| A-1 broken (a capability plugin gets no config-verify) | DESIGN: route verify to every plugin that declared capabilities |
| Functional test fails | Check the AC: wrong AC → DESIGN, correct AC → IMPLEMENT |
| 3 fix attempts failed | STOP. Report all 3 approaches. Ask the user |

## Key Design Decisions
| Decision | Alternatives Considered | Rationale |
|----------|------------------------|-----------|
| D1: one packer, `capability.PackOptionalParams`, comparing 2 + capTotal with 255 (owner, 2026-10-13: "Consolidate") | home in `internal/component/bgp/message` | the capability package is a core leaf every caller already imports and holds the decoder; `message` would gain an import for no reader |
| D2: refuse a reload that changes a plugin-declared capability (owner, 2026-10-13: "b"), through one SDK registration of the plugin's extractor | the comparison in the engine; one SDK comparison method plus a verify handler in each plugin | only the plugin can compute its candidate declaration, so the engine cannot compare; a per-plugin verify handler is opt-in and already missed the exabgp bridge (review of 2026-10-13, ISSUE-6), while a registration that the SDK runs at stage 2 and at verify cannot be skipped and cannot compute the two declarations differently |
| D2: compare per selector and accept a removed peer | compare the whole lists | a whole-list compare would refuse every peer removal, which leaves no stale OPEN |
| D3: one WARN per received NOTIFICATION in `handleNotification` | WARN only before Established; raise the reconnect-loop DEBUG to WARN | the producer is the one place that knows code, subcode and state; one rule for all states matches FRR and BIRD defaults; the reconnect loop sees only an error string |

## Known Limitations
- R-4: the 4096-octet OPEN limit is recorded as a journal row, not fixed here; the two-octet limit is refused by the one packer (AC-1x)
- Out of scope by Q3 (owner, 2026-10-13: "Out of scope"): the live global `router-id` reload stays as it ships. RFC 6286 Section 2.1 says the identifier "is determined on startup"; Ze applies a changed global `router-id` on SIGHUP (`reactorAPIAdapter.applyGlobalsJournaled`, `internal/component/bgp/reactor/reload_globals.go`) and the peer diff restarts every peer whose effective Identifier changed, so every restarted peer sees the new identifier in a new OPEN (`test/reload/reload-global-router-id.ci`). Not refused, not counted as a deviation here
- A plugin-declared capability change (softver, hostname, role, llnh, gr, the exabgp bridge) needs a daemon restart (Q1 = b); the reload refuses it and says so
- New restriction against today (Q1 = b): two reloads that are accepted today are refused and need a daemon restart: a session family added or removed on a `gr` peer whose `graceful-restart` container has no `family` list (`grAdvertisedFamilies` derives the GR payload from the session families), and a peer added that carries a plugin-declared capability. An explicit `graceful-restart family` list avoids the first
- The restart the refusal names (`systemctl restart ze`, `ze signal stop` then `ze start <config>`, or `reboot` on the appliance) ends sessions with a normal teardown; the Graceful-Restart-keeping sequence is in `docs/guide/plugins.md`, not in the refusal
- RFC 4271 Section 4.2 carries the same "same for every local interface and BGP peer" sentence and has no row; RFC scope is frozen, so no row is added

## RFC Documentation (Scope: protocol)

Add `// RFC 9072 Section 2: "However, if the length of the Optional Parameters in the BGP OPEN message does exceed 255, the OPEN message MUST be encoded according to the procedure below."` above the threshold in `capability.PackOptionalParams`, and the RFC 9072 Section 2 SHOULD sentence above the base-form branch. Each caller of the packer carries `// RFC 9072 Section 2` at the call (`ai/rules/rfc-compliance.md`, functions added from 2026-09-24).

## Design Review Log

| Date | Round | Result |
|------|-------|--------|
| 2026-10-13 | owner answers applied 2026-10-13; review round owed (Q1 refusal, Q4 consolidation) | Q1 "b", Q3 "Out of scope", Q4 "Consolidate" recorded; D2 refusal design and D1 consolidation design written; review items named in both designs (the restart command; error propagation from chaos, perf and bmp OPEN builders) |
| 2026-10-13 | review of the owner-answer edits applied (`tmp/session/2026-10-12-0c3648c8-b9d7-45d8-b3f0-2473bceefb25/scratch/review-open-session-owner-answers.md`: 0 BLOCKER, 8 ISSUE, 6 NIT) | ISSUE-1: eleven stale records and two first records listed, seven approval commands. ISSUE-2: six packers (`bgpBuildOpen`, `buildActiveOpen` added), all deleted onto the one packer, grep widened to the tree; a seventh writer, `encodeOpen`, opened as Q5. ISSUE-3: AC-1q names no tier violation in a touched package. ISSUE-4: refusal names `systemctl restart ze`, the unsupervised and appliance routes, never `ze signal restart` (exits 0, `Restart=on-failure`). ISSUE-5: new restriction stated in Behavior to change, Known Limitations and R-1; GR claim scoped to no `graceful-restart family` list; AC-2d scoped, AC-2g accepted polarity. ISSUE-6: one SDK registration (`DeclareCapabilitiesFrom`) covering the five bgp plugins and the exabgp bridge, no per-plugin verify handler; empty-`Peers` verdict row; AC-2f. ISSUE-7: "ze would keep sending". ISSUE-8: every Q2-remove branch deleted; deviation names the YANG description and `docs/guide/configuration.md` throughout. NITs: N1 left; N2 sentence; N3 line; N4 resolved table; N5 `writeToExtended` comment in Files to Modify; N6 nothing. **Next round owed, narrow: the SDK registration (its shape, the kept `SetCapabilities`, A-1 dispatch to plugins without a verify handler) and the six-packer consolidation (plus Q5)** |

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
