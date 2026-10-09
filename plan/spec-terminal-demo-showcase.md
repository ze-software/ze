# Spec: terminal-demo-showcase

| Field | Value |
|-------|-------|
| Status | design |
| Scope | tooling \| docs |
| Depends | `plan/immediate/spec-session-editor-file-mode-parity.md` (session-mode load merge and commit confirmed, former AC-13/AC-14); `plan/immediate/spec-appliance-kernel-vpn-modules.md` (owner order: that spec first. The 2026-10-09 probe shows the render host already carries esp4 and xfrm_interface, so the dependency is the product's, not the recording's: see Probe Results) |
| Phase | - |
| Handoff | - |
| Updated | 2026-10-09 |

Recovery after compaction: `.claude/rules/post-compaction.md`.

## Task

Owner request (2026-10-09): "we need a new video for the readme and front site
which should [show] the unique feature of ze, like irr download, vpn, etc. all in
one big session ... identify the key things rpki, etc. and then have us configure
it and demonstrate each feature one after the other. It can be a longer video."

Owner amendment (2026-10-09): "we need a recording per topic and a
super-recording."

Two kinds of recording, each made on its own:

- **Topic recordings.** One per storyboard topic, chapters 2 to 9: RPKI, IRR,
  BFD, OSPF, IPsec, VRRP tracking, eBPF traffic, commit-confirmed. Each stands
  alone from a base lab: the topic is typed live in the SSH editor, committed and
  demonstrated. Each is placed on that feature's doc page and in the demo gallery
  (`docs/guide/terminal-demonstrations.md`).
- **Super-recording.** A separate continuous long session: one Ze daemon whose
  config builds up chapter by chapter (chapters 0 to 10 below), each chapter
  committed and then demonstrated. It is recorded on its own, never stitched
  from the topic casts and never sliced into them. Its home is the site
  (asciinema player, front-page hero); the GitHub README embeds it inline when
  GitHub can render it, else links to the site player.

Owner answer on placement (2026-10-09): "the content should be on the site but
if github can have it embedded that would be ideal".

### Owner decisions (2026-10-09, answers to the research questions)

| Decision | Answer |
|----------|--------|
| VPN chapter | Native IKEv2/IPsec in Go, far end a hidden second Ze daemon in its own netns. The kernel modules it needs must also be in the appliance kernel: `plan/immediate/spec-appliance-kernel-vpn-modules.md` |
| WireGuard | Needs runtime testing AND a demo: `plan/spec-wireguard-runtime-proof.md`. Not a chapter of this showcase until that spec lands |
| PPP / L2TP | Skip |
| Chapters | As listed below, in that order |
| Recordings (amendment) | One recording per topic (chapters 2 to 9) plus one super-recording. The super is a separate continuous session, recorded on its own, not stitched from nor sliced into the topic casts |
| Topic placement (amendment) | Each topic recording on its feature's doc page AND in the demo gallery `docs/guide/terminal-demonstrations.md` |
| Super placement (amendment) | Home is the site (asciinema player). The GitHub README embeds the super inline if GitHub can render it (animated SVG, or a video GitHub plays in markdown) within GitHub's size limits; otherwise the README shows a thumbnail linking to the site player. The carrier is a measured outcome of a Work Plan probe, not an owner question (R-4) |
| Length | Super about 7 minutes. The earlier "README gets a separate 60 to 90 s cut as an animated SVG" is superseded: the README carries the super itself, inline or as a linked thumbnail |
| Front page | Switch the hero from cli-dashboard to the super-recording (the earlier decision stands) |
| Configuration style | Superseded at design review by `docs/contributing/terminal-demos.md`, decision 1, and its "Recordings" section (a live commit that fails to enable a feature is a Ze defect) |

### Owner decisions (2026-10-09, at design review)

The owner's design-review decisions on what every recording shows and how it
sounds (sectioned config loads instead of typing, the five beats, the flawless run,
the two audiences, "it is a demo", and the sales-engineer tone) are recorded
verbatim, with their operational reading, in `docs/contributing/terminal-demos.md`.
That page is their one statement; this spec applies them through AC-15 and AC-17.

| Decision | Answer |
|----------|--------|
| Topic recording shape | The five beats of `docs/contributing/terminal-demos.md`, decision 2; the sectioned load of decision 1 goes into the RUNNING daemon's candidate through the SSH editor |
| Q1, replace vs keep | Resolved: the topic recordings REPLACE the old demos and keep everything the old ones showed. VRRP shows failover AND tracking; OSPF shows neighbour, database and routes |
| Q2, typed live vs prepared | Resolved by `docs/contributing/terminal-demos.md`, decision 1 |
| Missing diagnostics | Per `docs/contributing/terminal-demos.md`, decision 2; this spec lists them under Diagnosis Findings below |

### Storyboard (from research, to be validated chapter by chapter)

| # | Chapter | Shown | Validator proof |
|---|---------|-------|-----------------|
| 0 | Intro card, ExaBGP migrate | `ze exabgp migrate <file>` | output carries `bgp {` and the peer |
| 1 | Base BGP, live dashboard | `monitor bgp`, sort key, quit | peers Established |
| 2 | RPKI | set rpki cache-server, commit, `show bgp rpki status`, `show bgp adj-rib-in` | VRPs synced; Invalid route held `ineligible` with state 3 |
| 3 | IRR | set plugin/irr server/as-set/import filter, commit, `show bgp irr`, `show bgp irr check` | in-set accepted, out-of-set refused and absent from Adj-RIB-In. Card says the IRR server is a local test server |
| 4 | BFD | set bfd + peer bfd, commit, `show bfd sessions`, link down, `show bgp peer list` | BFD Up, then BGP leaves Established well inside the hold time |
| 5 | OSPF | set ospf area/interface, commit, `show ospf neighbor`, `show ospf route` | Full; FRR loopback route present |
| 6 | IPsec | set vpn ipsec ike/esp groups and peer, commit, `show vpn ipsec sa`, `show vpn ipsec dataplane sa`, ping through tunnel | SA established; kernel SPI listed; packets-in > 0 |
| 7 | VRRP tracking | set vrrp group with `track`, commit, `show vrrp`, tracked link down, `show vrrp` | master, then backup with reduced effective priority. Do NOT reuse runVRRP failover (it kills Ze) |
| 8 | eBPF traffic | set traffic usage, commit, burst, `show traffic usage name traffic0` | source and port counted |
| 9 | Safety net | risky change, `commit confirmed 8`, wait, `show \| compare` | "automatically rolled back" |
| 10 | Recap card | | |

Optional extras the owner did not add: FlowSpec into nftables, config graph, MCP.

### Topic recordings by the five beats

Diagnosis commands are the ones whose handler is declared today (`ze:command` in the
YANG of the owning component, read 2026-10-10). Command paths follow the handler names;
each is re-checked against `help` output when its tape is written. Cross-topic tools
every recording may use: `show log recent level <level>` and `show log levels`
(`ze-log`), `monitor event` (`ze-meta:event-monitor`), the `debug` module and profile
commands (`ze-debug`), `show capture` (`ze-diag`), `show config history` (editor).

| Topic | Lab (beat 1) | Config sections loaded (beat 2) | Proof (beat 3) | Failure shown (beat 4) | Diagnosis with existing commands (beat 5) |
|-------|--------------|---------------------------------|----------------|------------------------|-------------------------------------------|
| RPKI | Ze, one eBGP peer announcing Valid, NotFound and Invalid routes, local RTR cache | RTR cache server; validation policy (`invalid reject`) | `show bgp rpki status`, `show bgp rpki summary`, `show bgp adj-rib-in` (Invalid held ineligible) | an Invalid route (origin AS not in the ROA); then the RTR cache stopped | `show bgp rpki roa <prefix>` explains the Invalid; `show bgp rpki cache` and `show bgp rpki status` show the cache down; fix: restart cache, VRPs resync |
| IRR | Ze, one customer peer, local IRR server (card says it is a test server) | IRR server and refresh; peer as-set; peer import filter | `show bgp irr status`, `show bgp irr prefix <peer>`, `show bgp irr check <peer> <prefix>` accepted | wrong as-set: the customer's route is refused | `show bgp irr check` (accepted false), `show bgp irr prefix` (set lacks the prefix); fix: load the right as-set, `update bgp irr as-set`, check again (needs AC-1) |
| BFD | Ze and FRR (bgpd+bfdd) across a veth | BFD profile; peer `bfd` | `show bfd sessions`, `show bfd session <peer>`, `show bgp peer list` Established | link down | `show bfd sessions` (Down, diagnostic), `show bgp peer history <peer>`, `show log recent`; BGP left Established well inside hold time; fix: link up |
| OSPF | Ze and FRR (ospfd) across a veth, FRR loopback | area and interface | `show ospf neighbor`, `show ospf database router`, `show ospf route` (FRR loopback) | Hello interval mismatch on the interface: no adjacency | `show ospf interface detail` (timers), `show ospf neighbor detail`; whether Ze names the rejected-Hello reason is Finding D-3; fix: load matching timer |
| IPsec | Ze and a hidden far-end Ze in its own netns, a host behind each | IKE group; ESP group; peer with PSK and selectors; xfrm interface | `show vpn ipsec status`, `show vpn ipsec sa`, `show vpn ipsec dataplane sa`, ping through the tunnel, packets-in above zero | proposal mismatch (ESP group the far end does not offer) | `monitor vpn ipsec`, `show vpn ipsec peer`, `show vpn ipsec dataplane drift`, `show log recent`; whether the negotiated failure reason (NO_PROPOSAL_CHOSEN) is shown is Finding D-4; fix: load the matching group, `clear vpn ipsec sa` |
| VRRP | Ze and keepalived on one LAN, plus an uplink Ze tracks | VRRP group; `track` on the uplink | `show vrrp`, `show vrrp interface` (master) | (a) tracked uplink down: Ze drops to backup with reduced effective priority; (b) failover: the master stops sending, the backup takes over | `show vrrp`, `show vrrp statistics`, `monitor event`; fix: uplink up, preempt back |
| Traffic usage | Ze, a traffic namespace sending bursts on traffic0 | `traffic usage` on traffic0 | `show traffic usage name traffic0` (source and port counted) | an unexpected heavy talker | `show traffic usage`, `show traffic stat`, `monitor traffic stat`; fix: identify the source |
| Commit confirmed | the base lab | a risky change | `commit confirmed <n>`, then rollback seen | the change is not confirmed in time | `show config history`, `show \| compare`; needs `plan/immediate/spec-session-editor-file-mode-parity.md`, former AC-14 (blocked in session mode today) |

### Diagnosis Findings (for the owner)

| ID | Finding | Evidence | Effect on the recordings |
|----|---------|----------|--------------------------|
| D-1 | The SSH editor connected to a running daemon (session mode) refuses `load`: `cmdLoadNew` returns `errLoadNotSupportedInSessionMode` when the editor holds a session. The verb exists in file mode: `load <file\|terminal> <absolute\|relative> <merge\|replace> [path]` merges into the candidate (`model_load.go`); `docs/guide/config-editor.md` lists it as blocked because it replaces the tree without per-leaf change entries | `internal/component/cli/model_load.go` `cmdLoadNew`; `editor_commands.go` | the owner's load-merge style cannot run against a running daemon until `plan/immediate/spec-session-editor-file-mode-parity.md` lands (former AC-13) |
| D-2 | `commit confirmed` is blocked in session mode ("Needs session-aware rollback", `docs/guide/config-editor.md`; `errCommitConfirmedNotYetSupportedIn`) | `model_commands.go` | the commit-confirmed topic cannot run over SSH until `plan/immediate/spec-session-editor-file-mode-parity.md` lands (former AC-14); this resolves A-3 as broken |
| D-3 | Not yet verified: whether OSPF exposes why a neighbour's Hello was rejected (interval, area, mask mismatch) in a show command or counter | no handler named for it among the `ze-ospf` commands; to be read at the producer when the tape is written | if absent, beat 5 for OSPF shows only the timers side by side: a finding, not invented |
| D-4 | Not yet verified: whether IKE shows the failure reason of a refused negotiation (NO_PROPOSAL_CHOSEN) in `show vpn ipsec peer` or `monitor vpn ipsec` | handlers exist; the content is unread | same treatment as D-3 |
| D-5 | IRR live edits are not applied (AC-1) | `filter_irr.go` | the IRR beat-5 fix cannot be shown until AC-1 |

`ze config import` (the command catalogue's `load merge` equivalent) stores a file as a
new version in the store; no path from it to a running daemon's reload was read, and it
is not an editor verb, so it does not meet the owner's direction.

## Required Reading

- [ ] `docs/guide/terminal-demonstrations.md`, `docs/contributing/gh-pages.md` - demo commands, image, assets. Silent on scenario design, validators and the README SVG.
  → Constraint: the gallery is a list of `<!-- terminal-demo: <id> -->` markers under headings. A topic recording that replaces a demo under the same id keeps its marker; a new id needs a heading and a marker.
  → Constraint: the page is silent on how a scenario starts its lab, on validators and on the README SVG, so those were read from source (below).
- [ ] `internal/le/site/terminaldemo/` - `scenarios.go` (runScenario switch, constants block, `initText`, `demoInstance`, `scenarioConfigDir`), `scenarios_routing.go` (`runRPKI`, `runIRR`), `scenarios_network.go` (`runBFD`, `runOSPF`, `runTraffic`, `runVRRP`, `labCreatePair`, `startFRRPair`), `validate_runtime.go` (`demoValidators`), `render.go` (`containerCommand`, `--privileged` from the manifest), `cards.json`, `tape_wait_test.go`
  → Constraint: `runScenario` (`scenarios.go`) is a `switch` over 15 demo ids and `demoValidators` (`validate_runtime.go`) is a literal map over 18. Both are central enumerations that every new scenario must edit, the shape `ai/rules/principles.md` forbids. → Decision: one scenario registry, filled from `init()` in each scenario file with the runner and the validator under one id; the switch and the map are deleted first (`ai/rules/no-layering.md`).
  → Constraint: the tape `Source` directive (`tapeLines`, `pty.go`) resolves a name against the demo root, then the sourcing tape's directory, and nests. Its `seen` set is never cleared, so ANY file sourced twice in one tape is refused as "sources itself". A topic fragment MUST NOT source `common.tape`, and the showcase tape sources each fragment exactly once.
  → Constraint: `sourceDigest` (`manifest.go`) digests `common.tape`, the Dockerfile, the recorder Go sources and the files in the demo's OWN directory; `definitionDigest` digests `common.tape` and the demo's own tape. Neither follows `Source`, so an edit to a shared fragment outside the demo directory moves no digest and a stale recording is reported current. → Decision: both digests take their tape files from the Source closure (the same `tapeLines` walk), not from a directory listing.
  → Constraint: `Set` after the first action is refused (`parseTape`, `pty.go`), so TypingSpeed is one value per tape: every typed config line costs real seconds in the super (R-8).
  → Constraint: no existing demo commits a topic into a RUNNING daemon. `irr-filter` types `ze config set -` pipelines offline, then starts the daemon; `commit-confirmed` uses the file editor (`ze config edit -f ze.conf`) with no daemon; `rpki`, `bfd-failover`, `ospf-adjacency`, `vrrp-failover` and `traffic-anomaly` import a prepared `ze.conf` at start. `zefs-config` is the only tape that enters the SSH editor (`sshpass -e ssh ze-demo`), sets a leaf and commits against a running daemon: the proven pattern the topic tapes follow.
  → Constraint: `startFRRPair` (`scenarios_network.go`) starts zebra plus ONE protocol daemon under the hardcoded `/run/frr`; the shared lab needs bgpd, bfdd and ospfd under one zebra.
- [ ] `demos/terminal/manifest.json`, `demos/terminal/Dockerfile` (frr, keepalived, iproute2, nftables; no strongSwan, no wireguard-tools)
  → Constraint: a manifest `Demo` (`types.go`) carries id, title, description, page, anchor, platform, kind, engine, source, validate, duration, privileged, realtime and nothing else. `page` + `anchor` place a recording on a feature page; nothing in it can declare a topic's config or checks (R-5).
  → Constraint: the image carries no IKE daemon, so the IPsec far end is a second Ze in its own netns (owner decision), started by the lab, never shown.
- [ ] `docs/guide/ipsec.md`, `test/ipsec/ipsec-sa-installed.ci` (Ze-to-Ze IPsec config shape)
  → Constraint: `test/ipsec/ipsec-peer-reload-applies-selectors.ci` proves an EDITED peer is applied by reload. Nothing proves that a peer ADDED by reload to a daemon with no `vpn` root brings a tunnel up, which is exactly what the IPsec topic types (AC-3).
- [ ] `docs/guide/vrrp.md` "Tracking an interface"
  → Constraint: tracking is the `track` container of `internal/plugins/vrrp/yang/ze-vrrp-conf.yang`, documented under "Tracking an interface" in `docs/guide/vrrp.md`. The topic shows effective priority dropping when a tracked link goes down, never a daemon kill.
- [ ] `docs/guide/config-reload.md` (what a live commit restarts), `internal/component/plugin/server/startup_autoload.go` (`autoLoadForNewConfigPaths`)
  → Constraint: on reload, `autoLoadForNewConfigPaths` (called from the reload path in `reload.go`) starts a plugin only when one of its `ConfigRoots` is among the ADDED diff paths and it is not already running. A plugin rooted at `bgp` is never auto-loaded by adding a sub-tree under an existing `bgp`.
  → Constraint: `OnConfigure` is Stage 2 (boot) only. With no `OnConfigApply` handler the SDK answers config-apply OK and calls nothing (`pkg/plugin/sdk/sdk_callbacks.go`, doc comment of `OnConfigApply`), so a plugin holding `OnConfigure` alone silently ignores every live commit. `docs/guide/config-reload.md` ("Plugin config changed: Plugin reloaded") is false for such a plugin (Doc row 6).
- [ ] `internal/le/site/home.go` (`homeHeroDemo`) and `internal/le/site/homebody.go` (hardcodes `cli-dashboard.terminal`, caption, `#live-bgp-dashboard`: a second declaration of the hero)
  → Constraint: `homeHeroDemo` (`home.go`) feeds `heroMount`, while `homebody.go` hardcodes `cli-dashboard.terminal` and `#live-bgp-dashboard`, and `README.md` names `docs/demos/cli-dashboard.svg` and that anchor again. → Decision: the hero's file name, caption and anchor derive from `homeHeroDemo` plus its manifest entry, and the README asset is produced by a `./le` action from the same declaration.

**Key insights:**
- Every tape wait must match output only its command prints; `TestTapeWaitsAreNotSatisfiedByTheTypedCommand` enforces it.
- `ze config set <file>` offline edits a file path, never a stored name; the store is edited through the SSH editor or `config cat | config set - | config import --yes --name ze.conf -`.
- Demo daemons start with bare `ze start` under `ZE_CONFIG_DIR=<state>/config`, instance name `ze` (= container hostname).
- An RPKI-Invalid route under `invalid reject` stays in Adj-RIB-In marked ineligible; an IRR-refused route is dropped at ingress.
- A loopback NEXT_HOP is treated as withdraw; demo peers announce 192.0.2.2.
- A released BFD session stays AdminDown about 9 s before it retires (RFC 5880 Section 6.8.1).
- The `demoWalkthrough` branches of runBFD/runOSPF/runTraffic/runVRRP print canned `$ ze show ...` lines that are not command output; do not reuse them.

## Current Behavior

**Source files read:**
- [ ] `internal/le/site/terminaldemo/render.go` - `containerCommand` builds the docker run; `--privileged` comes from the manifest; `--hostname` is `demoInstance`
- [ ] `internal/le/site/terminaldemo/scenarios_network.go` - `startFRRPair` runs one FRR protocol daemon under a hardcoded `/run/frr`
- [ ] `internal/le/site/home.go` - `homeHeroDemo = "cli-dashboard"`
- [ ] `internal/le/site/homebody.go` - hardcodes the hero file name, caption and anchor a second time

- 18 single-feature demos exist and were all re-recorded on 2026-10-09 (gh-pages 3252810ae7).
- The README shows `docs/demos/cli-dashboard.svg`, converted by hand from the site cast with svg-term-cli 2.1.1 (`--window --no-cursor --width 138`), after stripping `\x1b\[[>=<?][0-9;]*[mu]`, holding intro/outro cards 7 s, and embedding a JetBrains Mono subset as base64 WOFF2 (font-family ZeMono). No producer in the repo does this.
- A 58 s cast became a 96 KB SVG. A 7-minute SVG would be several MB.

### Probe Results (2026-10-09, design phase)

**R-2, IPsec kernel support on the render host: RESOLVED, supported.** The Docker VM
(colima, Ubuntu 24.04.4, kernel `6.8.0-117-generic`) builds `CONFIG_INET_ESP=m`,
`CONFIG_XFRM_INTERFACE=m`, `CONFIG_XFRM_USER=m`. `esp4`, `esp6`, `xfrm_user` were already
loaded. A privileged container (`--network none`, iproute2 6.11) created an xfrm
interface and an ESP tunnel SA; the kernel auto-loaded `xfrm_interface` on the link
creation:

| Step in the container | Output |
|-----------------------|--------|
| `ip link add xp0 type xfrm if_id 7` | `xp0@NONE ... xfrm if_id 0x7` |
| `ip xfrm state add ... proto esp spi 0x100 mode tunnel aead rfc4106(gcm(aes)) ... if_id 7` | `proto esp spi 0x00000100 reqid 1 mode tunnel`, rc 0 |
| `lsmod` on the VM afterwards | `xfrm_interface 28672 0`, `esp4 28672 0` |

→ Decision: the IPsec topic recording does not wait on the render host. The
appliance spec governs whether the APPLIANCE can do what the recording shows, which is
a publication question (Owner question 3), not a recording blocker.

**R-1, does a live commit enable each topic: read at the producer, NOT yet run.** Each
row names the reload path a commit adding the topic's config to a running daemon takes.
The runtime proof is AC-2 (each topic validator runs against a daemon started without
the topic).

| Topic | Plugin, ConfigRoots | Loaded on the commit? | Applies the config? | Reading |
|-------|--------------------|-----------------------|---------------------|---------|
| RPKI | `bgp-rpki`, `bgp` | already running: `bgp` is in the base config, so it loads at boot | `OnConfigVerify` + `OnConfigApply` (`replaceConfig`) in `rpki.go` | expected to work, unverified |
| IRR | `bgp-filter-irr`, `bgp` | never auto-loaded by a sub-tree under an existing `bgp`. The existing tape declares it explicitly (`plugin internal bgp-filter-irr`); whether adding that line in a live commit starts a fresh process, which would then get Stage 2 config, is unread (A-1) | **No, once running.** `runFilterIRR` (`filter_irr.go`) registers `OnConfigure` only. The SDK answers config-apply OK and calls nothing | **DEFECT, read at producer:** any live IRR edit to a daemon whose `bgp-filter-irr` is running (new as-set, new server, new peer filter) is accepted and not applied. It fails open: the commit reports success and the filter keeps its boot config. Owed fix: AC-1. The first-add path is probed under A-1 |
| BFD | `bfd`, `bfd` | new top-level root, auto-loaded | `OnConfigVerify` + `OnConfigApply` in `bfd.go`; `test/reload/bgp-bfd-profile-reload-*.ci` cover edits | expected to work, unverified for a first-time add |
| OSPF | `ospf`, `ospf` | new top-level root, auto-loaded | `OnConfigApply` in `internal/plugins/ospf/register.go` | expected to work, unverified |
| IPsec | `ike`, `vpn` and `pki` | new top-level root, auto-loaded | `OnConfigApply` in `internal/component/ike/engine/register.go` reconciles the peer set | expected to work; only an edited peer is proven (`ipsec-peer-reload-applies-selectors.ci`) |
| VRRP tracking | `vrrp`, `interface` | `interface` is in the base config, so whether the plugin runs before any `vrrp` block exists decides it | `OnConfigApply` in `internal/plugins/vrrp/register.go` | unverified: A-2 |
| eBPF traffic usage | `trafficusage`, `traffic/usage` | added path, auto-loaded | `OnConfigVerify` + `OnConfigApply` in `internal/plugins/trafficusage/register.go` | expected to work, unverified |
| commit-confirmed | editor command, no plugin | n/a | proven in the file editor by the `commit-confirmed` demo; not yet in the SSH editor | unverified over SSH: A-3 |

### Open owner questions (asked at approval, 2026-10-09)

Q1 and Q2 are answered (Owner decisions at design review, above): replace and keep
everything; load-merge sections with explanation, no typing. Q3 stays open. The rows
below are kept as asked.

| # | Question | Recommendation | Why |
|---|----------|----------------|-----|
| 1 | Replace each existing single-feature demo with its topic recording, or keep both | Replace, keeping the existing id where the subject is the same (`rpki`, `irr-filter`, `bfd-failover`, `ospf-adjacency`, `traffic-anomaly`, `commit-confirmed`) and renaming `vrrp-failover` to `vrrp-tracking`; add `ipsec` | `ai/rules/no-layering.md`; two recordings of one feature drift, and the old ones stage config the owner decided must be typed live. Keeping the id keeps every page marker |
| 2 | Typed-live config in topic recordings vs the prepared-config pattern the existing demos use | Typed live for every topic, committed in the SSH editor against the running daemon; the BASE (peers, interfaces, chapter 1) stays imported at lab start | owner decision 5 already says typed live; this only confirms it overrides the existing pattern and that the base is not a topic. It makes each recording a live-reload test, which found the IRR fail-open |
| 3 | Order relative to `plan/immediate/spec-appliance-kernel-vpn-modules.md` | Everything, IPsec included, can be built and recorded now: the render host has esp4 and xfrm_interface (Probe Results). Publish the IPsec topic and the super (which contains it) after the appliance spec lands, if the owner wants the appliance able to do what the hero shows | the dependency is the product's, not the recording's |

## Work Plan (ordered)

1. Probes, before any tape:
   - live commit enables RPKI, IRR, BFD, OSPF, IPsec, VRRP, traffic usage on a running daemon (each a defect to fix if not);
   - esp4 and xfrm_interface load inside the privileged demo container on the render host.
2. Lab: one root-netns Ze managing veths to lab netns; FRR running bgpd+bfdd+ospfd together (`startFRRPair` hardcodes one daemon and `/run/frr`; generalize it); RTR cache, IRR server, HTTP source, keepalived peer, hidden second Ze for IKE.
3. Scenarios, each with a runner case, a validator in `demoValidators`, a manifest entry (`platform linux`, `privileged true` where the lab needs it), cards, tape and transcript:
   - one topic scenario per chapter 2 to 9 (RPKI, IRR, BFD, OSPF, IPsec, VRRP tracking, eBPF traffic, commit-confirmed), each starting from the base lab and typing its topic live. Seven topics already have a single-feature demo (`rpki`, `irr-filter`, `bfd-failover`, `ospf-adjacency`, `vrrp-failover`, `traffic-anomaly`, `commit-confirmed`) that loads a prepared `demos/terminal/<id>/ze.conf` instead of typing it, and `vrrp-failover` shows failover, not tracking. Whether each topic recording replaces its existing demo or sits beside it is settled at design (`ai/rules/no-layering.md` favours replacing). IPsec has no demo today;
   - the `showcase` super scenario, all chapters in one session, driven from the same per-topic declarations (R-5).
4. Hero: derive the front-page demo, caption and anchor from one declaration (`homeHeroDemo` plus manifest), then point it at the super.
5. README carrier probe, once the super is recorded: render the super as an animated SVG and measure its size; test whether GitHub renders it inline in the README, and likewise a video GitHub plays in markdown; check each against GitHub's size limits. Record the measurements in this spec.
6. README: embed the super inline with the carrier the probe proved; when neither renders within limits, show a thumbnail linking to the site player. Its producer is a native `./le` action (`internal/le/site/...`), not a hand recipe. Reword README line 10.
7. Gallery: `docs/guide/terminal-demonstrations.md` (the manifest's `gallery-page`) lists each topic recording by name, and the super.
8. Feature pages: embed each topic recording in its feature doc page through the manifest's per-demo `page` and `anchor` (today `guide/rpki.md`, `guide/irr-filtering.md`, `guide/bfd.md`, `guide/ospf.md`, `guide/vrrp.md`, `guide/traffic-usage.md`, `guide/config-editor.md`; the new IPsec recording goes on `guide/ipsec.md`).

## Risks

| ID | Risk | Mitigation |
|----|------|-----------|
| R-1 | A feature does not enable on a live commit | Fix in Ze (owner decision 5) |
| R-2 | Render host kernel lacks esp4/xfrm_interface | Load in container; else record where available; appliance spec covers the product side |
| R-3 | Long session flakes (plugin stage stall under load seen 2026-10-08) | Per-chapter waits on output; journal and fix the stall if it recurs |
| R-4 | A 7 minute animated SVG is likely too large for GitHub to render (a 58 s cast gave 96 KB; 7 minutes is several MB) | Not an owner question: a measured outcome. Work Plan step 5 measures the SVG and tests inline rendering (SVG, then a video GitHub plays in markdown); if neither renders within GitHub's limits, the fallback is a README thumbnail linking to the site player |
| R-5 | Each topic is recorded twice, as its own cast and as a super chapter, and the two drift (config lines, commands, expected output) | Declare each topic's typed config and demonstration once and drive both recordings from it. No such declaration exists today: a manifest `Demo` (`internal/le/site/terminaldemo/types.go`) carries only id, title, description, page, anchor, platform, kind, engine, source tape, validate id, duration and flags; the config is a per-demo `demos/terminal/<id>/ze.conf` loaded at prepare, the steps are that demo's `demo.tape`, and each validator in `demoValidators` is a Go function keyed by demo id that starts its own runner. The tape format has an include (every tape opens with `Source common.tape`), so a per-topic tape fragment sourced by both the topic tape and the showcase tape is one candidate; each topic's validator checks must likewise be one function that both validators call |

## Work Not Done (owned elsewhere)

| Item | Spec |
|------|------|
| Appliance kernel carries esp4, xfrm_interface, wireguard | `plan/immediate/spec-appliance-kernel-vpn-modules.md` |
| WireGuard runtime test, interop and demo | `plan/spec-wireguard-runtime-proof.md` |

## Data Flow (MANDATORY - see `ai/rules/architecture.md`)

### Entry Point
- Recording: `./le site terminal-demo render name <demo-id>` (one demo) or the render-all action, reading `demos/terminal/manifest.json`. Format: a tape file (`demo.tape`) of directives, executed by the native recorder in the renderer container.
- Proof: the same action's validate mode, which calls the scenario's validator.
- Product path under demonstration: keystrokes typed into `sshpass -e ssh ze-demo`, the SSH configuration editor, ending in `commit`.

### Transformation Path
1. `parseTape` / `tapeLines` (`pty.go`) expand `Source` includes: `common.tape`, then the base-lab fragment, then the topic fragments, into one action list.
2. The recorder drives `ze-demo shell` in a pty; `ze-demo run <scenario> <action>` dispatches to the scenario's runner (today `runScenario`, after this spec the scenario registry), which builds the lab (netns, veths, FRR, RTR cache, IRR server, keepalived, hidden second Ze) and starts the Ze daemon with the base config.
3. Typed `set` lines land in the SSH editor's candidate; `commit` runs the config transaction: parse, validate, diff, `autoLoadForNewConfigPaths` for added roots, then per-plugin verify and apply (`reload.go`).
4. The plugin applies the config (RTR sessions, IRR fetch, BFD sessions, OSPF adjacency, IKE SA and XFRM state, VRRP group, eBPF usage maps).
5. Typed `show` commands print the result into the cast; `Wait+Screen` holds the tape until output only that command prints appears.
6. The validator re-runs the topic's checks (one Go check per topic, shared by the topic scenario and the showcase) against the scenario state, outside the recording.
7. The artifact manifest records source and definition digests; the site build mounts each cast on its gallery heading and its feature page (`page` + `anchor`), and the hero on the home page.

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Recorder ↔ demo shell | pty keystrokes from the tape; screen model reads output | Yes: `TestScreenModelReconstructsTheRenderedTerminal` |
| Demo shell ↔ lab runner | `ze-demo run <id> <action>` argv | Yes for existing ids: `TestActionsCarryTheirContracts` |
| Operator ↔ daemon | SSH editor session, `commit` | Yes for one leaf: the `zefs-config` demo; not for any topic (AC-2) |
| Engine ↔ plugin | config-verify / config-apply RPC with JSON sections | Read: IRR has no apply handler (AC-1) |
| Ze ↔ kernel | netlink XFRM, eBPF maps, VRRP raw sockets inside the privileged container | Probe: XFRM yes (Probe Results) |

### Integration Points
- `homeHeroDemo` and `heroMount` (`internal/le/site/home.go`): the hero is pointed at the `showcase` id; `homebody.go` stops hardcoding it.
- `nativeRecorderSources`, `sourceDigest`, `definitionDigest` (`manifest.go`): digests follow the Source closure.
- `tapeLines` (`pty.go`): unchanged; fragments are plain tapes.
- `startFRRPair` (`scenarios_network.go`): generalised to a list of FRR daemons.
- `bgp-filter-irr` (`internal/component/bgp/plugins/filter_irr/filter_irr.go`): gains the verify/apply reload pair.

### Architectural Verification
| Check | Holds? | Evidence |
|-------|--------|----------|
| No bypassed layers (data flows through the intended path) | Yes, by design | topics are configured through the SSH editor and the config transaction, never by importing a prepared file mid-recording |
| No unintended coupling (components stay isolated) | Yes, by design | the recorder package only drives `ze` commands; the IRR fix stays inside the plugin |
| No duplicated functionality (extends existing, does not recreate) | Yes, by design | topic recordings replace the seven single-feature demos (Owner question 1); the shared lab replaces the per-demo labs it subsumes |
| Zero-copy preserved where applicable (refs, not copies) | N-A | tooling, no wire path; the IRR fix reuses `handleConfigure` |
| Registration over hardcoding, outbound: new commands, views, families, and handlers register, and the core discovers them. No per-feature field, switch case, or factory is added to a core/shared package (`ai/rules/plugins.md`) | Yes, by design | scenarios register runner + validator from `init()`; no `case` is added |
| Registration over hardcoding, inbound: no existing switch, seed map, validator, parser, runner, help string, or completion table has to learn this feature's name. Evidence names every list that was searched for the names this feature introduces, and the registry each one now derives from (`ai/rules/principles.md`) | Yes, once AC-6 lands | lists searched: `runScenario` switch and `demoValidators` map (both replaced by the scenario registry); `cards.json` is keyed data per demo, not a list of code paths; `homebody.go` hero literals (derived from `homeHeroDemo` + manifest, AC-8) |

## Risks & Assumptions

<!-- LIVE: written during RESEARCH/DESIGN, statuses updated during implementation.
     Gate answers from /ze-spec (assumption challenge, Failure Mode Analysis)
     land HERE, not only in conversation. -->

### Assumptions
<!-- Every row needs a validation method. `unvalidated` is not a valid final
     status: closure re-checks each one. A broken assumption also gets a
     Mistake Log row and a Deviations entry. -->
| ID | Assumption | Basis (file/doc/user statement) | If wrong | Validated by | Status |
|----|-----------|--------------------------------|----------|--------------|--------|
| A-1 | Adding `plugin internal bgp-filter-irr` plus the IRR policy in ONE live commit starts the filter with its config (Stage 2 delivery to a new process) | the existing `irr-filter` tape declares the plugin explicitly; reload path for a newly declared plugin not read | the first-add IRR topic also fails, and AC-1's fix must cover the start path as well as edits | Work Plan step 1 probe; then AC-1's `.ci` | unvalidated |
| A-2 | The `vrrp` plugin is running when the base config holds `interface` but no `vrrp` block, so a live VRRP commit reaches its `OnConfigApply` | `ConfigRoots` is `interface` (`internal/plugins/vrrp/groups.go`), which the base config carries | the VRRP topic commit is accepted and does nothing: a defect fixed under AC-2 | Work Plan step 1 probe | unvalidated |
| A-3 | `commit confirmed <n>` and its automatic rollback behave in the SSH editor as in the file editor | the `commit-confirmed` demo proves the file editor only | the safety-net topic needs a fix or a different editor path; the owner decision rules out the latter | Work Plan step 1 probe | unvalidated |
| A-4 | Two Ze daemons can run in one container (the shown one, and the hidden IKE far end in its own netns) with separate `ZE_CONFIG_DIR`, SSH port and instance name | `demoInstance` is the container hostname `ze`; the scenario code runs one daemon per demo | the IPsec topic needs a second container or a different instance naming | Work Plan step 2, lab bring-up | unvalidated |
| A-5 | A 7-minute super fits the time budget with every topic typed live at 125 ms per character | storyboard length; typing speed is one value per tape | the super runs long (R-8) | measure the super's typed characters at step 3 | unvalidated |
| A-6 | eBPF traffic usage attaches inside the privileged renderer container on the colima 6.8 kernel | the `traffic-anomaly` demo records today with `privileged` | the traffic topic cannot run in the shared lab | the existing demo's validator passes; re-check in the shared lab | unvalidated |

### Risks
The R-1 to R-5 rows of the earlier `## Risks` table stand; R-2 is resolved by the probe (Probe Results). New failure modes:

| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-6 | A topic fragment edited without its recordings re-rendering, because the digests do not follow `Source` | `./le site terminal-demo` check reports current after a fragment edit | AC-5: digests over the Source closure, with a test that edits a sourced fragment and expects drift |
| R-7 | The shared lab is heavier than any current lab (FRR with three daemons, RTR, IRR, keepalived, second Ze) and flakes under host load, which sibling sessions raise (load 70 seen during this design) | validator timeouts in lab bring-up | per-component readiness waits on output, never sleeps; record only when `docker ps` shows no interop containers |
| R-8 | Typed IPsec config is long; the super exceeds its length or drags | measured duration in the artifact manifest | keep each topic's typed lines to the minimum that enables it (groups pre-named, one peer); the base lab may carry PKI material files the config only references |
| R-9 | A topic validator passes against the boot config rather than the live commit (vacuous) | validator never started a daemon without the topic | AC-2 requires each topic scenario's daemon to start with the base config only, and its validator to assert the topic was ABSENT before the commit |
| R-10 | Replacing the seven demos drops what a replaced one showed (VRRP failover by daemon kill; IRR offline staging) | gallery diff review | the topic recording covers the feature's operator story; failover-by-kill is not kept (no-layering) |
| R-11 | The README carrier works on github.com today and breaks later (size or sanitiser change) | the README image stops rendering | the fallback thumbnail-and-link is always produced by the same `./le` action, so switching is one README line |
| R-12 | Hidden far-end Ze leaks into the recording (its logs or prompts) | screenshot review | the far end runs detached with logs to the state dir, started by the lab runner, never by the tape |

## Blast Radius

<!-- What a wrong landing costs, and how to get out. A reviewer reads this first. -->
| Question | Answer |
|----------|--------|
| What breaks if this is wrong? | The public site and README show a recording whose claim the product does not meet (a false public claim, corrected at once under `ai/rules/rfc-compliance.md`), or a demo render fails. The IRR fix touches the reload path of one BGP filter plugin: a wrong fix could drop or wrongly accept routes after a commit |
| How is it reverted? | Recordings and site: one commit revert plus a gh-pages re-publish. IRR fix: one commit revert, no config migration |
| Who else touches this path? | `internal/le/site/terminaldemo/` (any demo work), `internal/le/site/home*.go`, `README.md`; `filter_irr` is shared with `test/plugin/*irr*.ci`. Sibling specs: `plan/immediate/spec-appliance-kernel-vpn-modules.md`, `plan/spec-wireguard-runtime-proof.md` |

## Wiring Test (MANDATORY -- NOT deferrable)

<!-- BLOCKING: proves the feature is reachable from its intended entry point.
     Without it the feature exists in isolation: unit tests pass, nothing calls it.
     Every row needs a concrete test name. "Deferred"/"TODO"/empty is rejected
     by `internal/le/hookruntime/lifecycle.go`, which is the point: an unedited row fails. -->
| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| `./le site terminal-demo` validate mode, any manifest id | → | scenario registry lookup replacing `runScenario` and `demoValidators` | `TestEveryManifestDemoHasARegisteredScenario` |
| `ze-demo run showcase-lab start` from a topic tape | → | shared lab runner (FRR multi-daemon, RTR, IRR, keepalived, far-end Ze) | `TestShowcaseLabStartsEveryFixture` (runtime, Linux, under the renderer container) |
| A topic fragment sourced by its topic tape and by the showcase tape | → | `parseTape` / `tapeLines` + `sourceDigest` / `definitionDigest` | `TestTopicFragmentIsSourcedByItsTopicAndTheShowcase`, `TestSourceDigestFollowsSourcedTapes` |
| SSH editor `commit` adding IRR config to a running daemon | → | `bgp-filter-irr` verify/apply handlers | `test/reload/bgp-filter-irr-added-live.ci` |
| SSH editor `commit` adding each topic's config to a daemon started without it | → | each topic's plugin apply path | each topic validator: `validateTopicRPKI`, `validateTopicIRR`, `validateTopicBFD`, `validateTopicOSPF`, `validateTopicIPsec`, `validateTopicVRRPTracking`, `validateTopicTrafficUsage`, `validateTopicCommitConfirmed` |
| Site build home page | → | `heroMount` with `homeHeroDemo` = `showcase`, caption and anchor from the manifest | `TestHomeHeroDerivesFromTheManifest` |
| README asset action | → | the new `./le` README-carrier action | `TestReadmeCarrierIsProducedFromTheHeroDeclaration` |

## Acceptance Criteria

<!-- Define BEFORE implementation. Each row is a testable assertion, stated as
     observable behavior, never as the mechanism used to reach it. -->
| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | A running daemon with `bgp-filter-irr` loaded; the operator commits a changed IRR as-set or peer import filter (and, under A-1, a first-time IRR block) | The commit applies: a prefix outside the new set is refused at ingress and absent from Adj-RIB-In, an in-set prefix is accepted. A config the filter cannot apply fails the commit with the reason; the commit never reports success while the filter keeps its old config |
| AC-2 | Each topic scenario (RPKI, IRR, BFD, OSPF, IPsec, VRRP tracking, traffic usage, commit-confirmed) starts its daemon with the base config only; the tape load-merges the topic's sections in the SSH editor (Depends: `plan/immediate/spec-session-editor-file-mode-parity.md`) and commits | The topic's validator first asserts the feature was absent before the commit (no VRPs, no IRR filter, no BFD session, no OSPF neighbour, no SA, no VRRP group, no usage counters, the original value), then asserts the storyboard's proof column after it. Any topic whose live commit does not enable it is a Ze defect fixed in this spec, with its own `.ci` |
| AC-3 | IPsec topic | The hidden far-end Ze and the shown Ze establish an IKE SA after the live commit; `show vpn ipsec sa` lists it, `show vpn ipsec dataplane sa` lists a kernel SPI, and a ping through the tunnel raises the SA's packets-in above zero |
| AC-4 | VRRP tracking topic | After the commit the shown Ze is master; taking the tracked link down makes it backup with the effective priority reduced by the configured amount; the Ze process id is the same before and after |
| AC-5 | A topic fragment outside the demo directory is edited | Both the topic recording's and the showcase recording's source digests change, and the check mode reports both stale |
| AC-6 | Any id in `demos/terminal/manifest.json` | It resolves to exactly one registered scenario carrying a runner and a validator; `runScenario`'s switch and the `demoValidators` literal map no longer exist; an id with no registration is refused by name |
| AC-7 | Each topic's typed config and demonstration | Exist once, as fragments that both the topic tape and the showcase tape source; each topic's checks exist once, as a Go function both validators call |
| AC-8 | Home page build | The hero is the `showcase` recording; its file name, caption and transcript anchor come from `homeHeroDemo` plus the manifest entry, and `homebody.go` names no demo id, file or anchor literally |
| AC-9 | README carrier action, run on the recorded super | Produces the carrier the probe proved (inline SVG or a GitHub-playable video, within GitHub's size limits) or, failing both, a thumbnail image; README line 8 embeds it, linking to the site player's anchor; the measured sizes and the result of the inline-render check are recorded in this spec |
| AC-10 | Gallery and feature pages | `docs/guide/terminal-demonstrations.md` carries one heading and marker per topic recording and one for the super; each topic recording is embedded on its feature page through `page` + `anchor`, the IPsec one on `docs/guide/ipsec.md` |
| AC-11 | Showcase validator | Runs chapters 0 to 9's checks in order against ONE daemon whose process id never changes, each chapter's absence check before its commit and its proof after |
| AC-12 | The replaced single-feature demos (per Owner question 1) | Their tapes, prepared `ze.conf` files, runners, validators and cards are deleted; no page marker names a deleted id; the published asset of a deleted id is removed from gh-pages by the publish action |
| AC-13 | Moved out (owner, 2026-10-10) | Session-mode `load ... merge\|replace` is owned by `plan/immediate/spec-session-editor-file-mode-parity.md` (its AC-1 to AC-7). This spec consumes it through Depends |
| AC-14 | Moved out (owner, 2026-10-10) | Session-mode `commit confirmed` (unit: seconds, 1 to 3600, not minutes) is owned by `plan/immediate/spec-session-editor-file-mode-parity.md` (its AC-13 to AC-21). This spec consumes it through Depends |
| AC-15 | Each topic recording | Shows the five beats in order (lab, sectioned load with explanation, proof, a realistic failure, its diagnosis with existing Ze commands then the fix); the validator asserts the failure's diagnostic output and the recovery, not only the healthy state |
| AC-17 | Every topic recording | Its cards carry the vendor mapping (the equivalent concept and command an operator knows from another vendor, verified, not guessed) and a short concept explanation before each step, and the run on screen is flawless: no visible retry or stray output, every failure a scripted teaching step. Reviewed by watching the recording; no extra test machinery |

Tone and staging: `docs/contributing/terminal-demos.md`, decisions 4 ("it is a demo")
and 5 (the sales-engineer tone). Card wording and pacing in every topic and in the
super follow them (AC-17). Read AC-2's absence check and AC-15's failure assertions in
the light of decision 4: they guard against wrong output on screen, nothing more.
| AC-16 | VRRP and OSPF recordings | VRRP shows tracked-link demotion AND master failover with Ze staying up; OSPF shows neighbour, database and routes. Nothing an old demo showed is lost |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestEveryManifestDemoHasARegisteredScenario` | `internal/le/site/terminaldemo/scenarios_test.go` | AC-6: every manifest id resolves to one registration; an unregistered id is refused by name | |
| `TestScenarioRegistrationRefusesADuplicateId` | `internal/le/site/terminaldemo/scenarios_test.go` | AC-6: two registrations under one id panic at init | |
| `TestSourceDigestFollowsSourcedTapes` | `internal/le/site/terminaldemo/terminaldemo_test.go` | AC-5: editing a fragment outside the demo dir changes source and definition digests | |
| `TestTopicFragmentIsSourcedByItsTopicAndTheShowcase` | `internal/le/site/terminaldemo/tape_wait_test.go` | AC-7: each topic fragment is in its topic tape's and the showcase tape's Source closure, exactly once | |
| `TestTapeWaitsAreNotSatisfiedByTheTypedCommand` (existing) | `internal/le/site/terminaldemo/tape_wait_test.go` | extended over the expanded Source closure so fragment waits are checked | |
| `TestHomeHeroDerivesFromTheManifest` | `internal/le/site/home_test.go` | AC-8: changing `homeHeroDemo` changes file name, caption and anchor; no literal remains | |
| `TestReadmeCarrierIsProducedFromTheHeroDeclaration` | `internal/le/site/readme_carrier_test.go` | AC-9: the action names the hero's asset and anchor; it reports the carrier's size | |
| `TestFilterIRRAppliesAReloadedAsSet` | `internal/component/bgp/plugins/filter_irr/filter_irr_test.go` | AC-1: apply after verify swaps the per-peer set; a verify failure leaves the old one | |
| `TestFilterIRRRefusesApplyWithoutVerify` | `internal/component/bgp/plugins/filter_irr/filter_irr_test.go` | AC-1: no silent no-op apply | |

### Boundary Tests (numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| N-A | no new numeric input: topics reuse existing YANG leaves and their existing boundary tests | | | |

### Functional Tests
<!-- REQUIRED: a unit test proves the algorithm, a .ci proves the user can reach
     the feature. New RPCs/APIs are never covered by unit tests alone.
     Structure: ai/patterns/functional-test.md -->
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `bgp-filter-irr-added-live` | `test/reload/bgp-filter-irr-added-live.ci` | AC-1: operator changes the IRR as-set of a running peer and commits; an out-of-set route sent after the commit is refused and absent from Adj-RIB-In. Must go red with the apply handler removed | |
| one `.ci` per further topic found not to enable on a live commit | `test/reload/<topic>-added-live.ci` | AC-2: named when the Work Plan step 1 probe finds the defect; none exists yet because none is proven | |
| topic and showcase validators | `./le site terminal-demo` validate mode, Linux renderer container | AC-2, AC-3, AC-4, AC-11: the recording's claims hold against the live lab | |

### Interop Tests (Scope: protocol)
<!-- REQUIRED when wire-visible behavior changes. See
     ai/rules/interop-and-goal-validation.md, including the vacuity traps: prove
     the test FAILS when the behavior under test is reverted. -->
| Scenario | Directory | Peer Daemon | What It Proves | Status |
|----------|-----------|-------------|----------------|--------|
| N-A | | | No wire-visible change. The IRR fix changes when config is applied, not what is sent; the demos run against FRR and keepalived already, and the existing interop suites cover the protocols | |

## Files to Modify
- `internal/le/site/terminaldemo/scenarios.go` - delete the `runScenario` switch; dispatch through the scenario registry
- `internal/le/site/terminaldemo/validate_runtime.go` - delete the `demoValidators` map; existing validators register beside their runners; replaced demos' validators deleted
- `internal/le/site/terminaldemo/scenarios_routing.go`, `scenarios_network.go` - register existing scenarios; delete the replaced runners; generalise `startFRRPair` to several FRR daemons
- `internal/le/site/terminaldemo/manifest.go` - `sourceDigest` and `definitionDigest` over the Source closure
- `internal/le/site/terminaldemo/cards.json` - intro and recap cards for each topic and the super's chapter cards; replaced demos' cards removed
- `internal/le/site/home.go`, `internal/le/site/homebody.go` - hero from one declaration, pointed at `showcase`
- `internal/component/bgp/plugins/filter_irr/filter_irr.go` - `OnConfigVerify` + `OnConfigApply` (+ rollback) for the `bgp` section
- `docs/architecture/config/yang-config-design.md` - declared by the changed editor code; updated where it describes session-mode write-through, otherwise named unaffected with the reason at implementation
- `demos/terminal/manifest.json` - topic entries, `showcase` entry, replaced entries removed
- `demos/terminal/Dockerfile` - only if the lab needs a package it lacks (none known; the far end is Ze)
- `docs/guide/terminal-demonstrations.md`, `docs/contributing/gh-pages.md`, `README.md`, `docs/guide/config-reload.md`, `docs/architecture/bgp/filter-irr.md`, `docs/guide/irr-filtering.md` (live IRR edits now apply), `website/AI.md` (declared by `home.go`: the hero it describes moves to `showcase`), and each feature page in Work Plan step 8

## Files to Create
- `internal/le/site/terminaldemo/registry.go` - the scenario registry (id to runner and validator)
- `internal/le/site/terminaldemo/scenarios_showcase.go` - shared lab runner (`showcase-lab`) and the `showcase` scenario
- `internal/le/site/terminaldemo/validate_topics.go` - one check function per topic, called by the topic validators and the showcase validator
- `internal/le/site/readme_carrier.go` (+ test) - native `./le` action producing the README carrier from the hero declaration
- `demos/terminal/topics/<topic>/configure.tape` and `show.tape` - per topic, sourced by the topic tape and the showcase tape
- `demos/terminal/<topic-id>/demo.tape` + `transcript.txt` - per topic (replacing the seven, plus `ipsec`)
- `demos/terminal/showcase/demo.tape` + `transcript.txt` - the super
- `demos/terminal/showcase-lab/` - base config and lab fixtures (routes, RTR data, IRR data, far-end Ze config)
- `test/reload/bgp-filter-irr-added-live.ci`

### Integration Checklist
<!-- Answer every row Yes / No / N-A. Never leave a bare marker: an unanswered
     row is indistinguishable from a forgotten one. N-A needs a reason. -->
| Integration Point | Applies? | File / reason |
|-------------------|----------|---------------|
| YANG schema (new RPCs/config) | N-A | topics type existing leaves only |
| YANG validation constraints | N-A | no new leaf |
| YANG custom validators | N-A | no new leaf |
| CLI commands/flags | Yes | `le` only: the README-carrier action registered under `site` (`internal/le/site/`); `ze` gains nothing. A ze command found missing while typing a topic is a defect under AC-2 |
| CLI grammar (keyword before value) | Yes | the new `le` action takes `name <demo-id>` like `render` |
| Editor autocomplete | N-A | no new leaf or value |
| Functional test for new RPC/API | Yes | `test/reload/bgp-filter-irr-added-live.ci` for the IRR reload path; no new RPC |
| Pipe completeness | N-A | no new show output |
| Env var registration | N-A | none added; the far-end Ze gets its own `ZE_CONFIG_DIR`, an existing variable |
| Doctor check for runtime dependencies | N-A | the lab's netns, XFRM and eBPF use is inside the renderer container, not a product dependency; the appliance's IPsec modules are owned by `plan/immediate/spec-appliance-kernel-vpn-modules.md` |
| Prometheus counters/metrics | N-A | none added |
| BGP family surface (new SAFI / capability / attribute) | N-A | no family, capability or attribute |

### Documentation Update Checklist (BLOCKING)
<!-- Answer every row Yes / No / N-A. A No must be backed by a source-aware
     check, not a guess: at minimum grep docs/ for source anchors pointing at the
     files you changed. Any factual doc change carries a source anchor. -->
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature, or a feature's scope, evidence or level changed? | Yes | `features/irr-filtering` entry (or whichever `features/<id>.md` carries IRR): live IRR edits now apply; each topic's feature entry gains the recording as evidence where the entry lists demos |
| 2 | Config syntax changed? | N-A | no syntax change |
| 3 | CLI command added/changed? | N-A for `ze`; the `le` action is documented in `docs/contributing/gh-pages.md` |
| 4 | API/RPC added/changed? | N-A | none |
| 5 | Plugin added/changed? | Yes | `docs/guide/plugins.md` only if it states reload behaviour per plugin; otherwise `docs/architecture/bgp/filter-irr.md` |
| 6 | Has a user guide page? | Yes | `docs/guide/irr-filtering.md` (live edits apply), `docs/guide/config-reload.md` (a plugin without an apply handler: state the rule), and each topic's page gains its recording: `rpki.md`, `irr-filtering.md`, `bfd.md`, `ospf.md`, `ipsec.md`, `vrrp.md`, `traffic-usage.md`, `config-editor.md` |
| 7 | Wire format changed? | N-A | none |
| 8 | Plugin SDK/protocol changed? | N-A | the SDK is used, not changed |
| 9 | RFC behavior implemented, changed, or newly proven? | N-A | no RFC behaviour; the recordings are not RFC evidence |
| 10 | Test infrastructure changed? | Yes | `docs/contributing/gh-pages.md` and `docs/guide/terminal-demonstrations.md` (scenario registry, topic fragments, shared lab, digests over the Source closure) |
| 11 | Affects daemon comparison? | N-A | no capability change |
| 12 | Internal architecture changed? | Check | `docs/architecture/core-design.md` is declared by `manifest.go` and mentioned by `filter_irr.go`: re-read its paragraphs on those files when the digest closure and the IRR apply path change, and edit any sentence they make false. Recorder internals otherwise live in `docs/contributing/gh-pages.md` (row 10). `docs/guide/configuration.md` is also mentioned by the changed code: same check |
| 13 | Route metadata keys added/changed? | N-A | none |
| 14 | Prometheus counters added/changed? | N-A | none |
| 15 | Registered plugin, event type, send type, command, capability, or inventory changed? | N-A | no registration change in `ze` |
| 16 | Any changed source file referenced by existing doc source anchors? | Yes | from `./le spec citation anchors`: `website/AI.md` (declared by `home.go`, updated: hero is `showcase`); `docs/guide/irr-filtering.md` (mentions `filter_irr.go`, updated under row 6) |
| 17 | Existing docs show config/CLI/API examples for this area? | Yes | the typed config of each topic fragment is checked against the guide page's example; a guide example that the topic cannot type verbatim is corrected on that page |

## Implementation Steps

1. **Phase: Wiring** -- scenario registry and digest closure
   - Tests: `TestEveryManifestDemoHasARegisteredScenario`, `TestScenarioRegistrationRefusesADuplicateId`, `TestSourceDigestFollowsSourcedTapes`
   - Files: `registry.go`, `scenarios.go`, `validate_runtime.go`, `scenarios_routing.go`, `scenarios_network.go`, `manifest.go`
   - Verify: delete the switch and the map first, then register every existing scenario; all existing terminaldemo tests stay green
2. **Phase: Live-commit probes** -- Work Plan step 1 inside the renderer container, one topic at a time against a daemon started with the base config
   - Tests: the result per topic is written into Probe Results; each failing topic gets its `.ci` and its fix
   - Files: `filter_irr.go` (AC-1), plus whatever the probes name
   - Verify: `test/reload/bgp-filter-irr-added-live.ci` red without the apply handler, green with it
3. **Phase: Shared lab** -- `showcase-lab` runner, FRR multi-daemon, far-end Ze, fixtures under `demos/terminal/showcase-lab/`
   - Tests: `TestShowcaseLabStartsEveryFixture`
   - Verify: each fixture reports ready on output, never on a sleep
4. **Phase: Topics** -- per topic: `configure.tape`, `show.tape`, topic tape, check function, validator, manifest entry, cards; delete the replaced demo in the same change (AC-12)
   - Tests: `TestTopicFragmentIsSourcedByItsTopicAndTheShowcase`, `TestTapeWaitsAreNotSatisfiedByTheTypedCommand`, the topic validator
   - Verify: absence before commit, proof after (AC-2); render and validate the topic
5. **Phase: Super** -- `showcase` tape sourcing chapter 0, the base, every topic in order, the recap; `validateShowcase`
   - Tests: AC-11 validator
   - Verify: one daemon pid across chapters; measured duration recorded
6. **Phase: Hero and README** -- hero from one declaration; README carrier action; carrier probe and measurements recorded here
   - Tests: `TestHomeHeroDerivesFromTheManifest`, `TestReadmeCarrierIsProducedFromTheHeroDeclaration`
7. **Phase: Pages** -- gallery and feature pages (AC-10), docs in the Documentation checklist, each in the phase whose code made it wrong

### Critical Review Checklist

| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | Every AC-N has an implementation at file:line |
| Live, not staged | No topic tape imports or pipes config into the store; every topic section is load-merged in the SSH editor against the running daemon, displayed with its explanation, followed by `show \| compare` and `commit` |
| Diagnosis is real | every beat-5 command exists at source; no tape shows a diagnosis Ze cannot produce; gaps are Diagnosis Findings |
| Non-vacuous validators | Each topic validator asserts absence before the commit, so it cannot pass against a boot config |
| Single declaration | No topic `set` line or demonstration command appears in two tapes; `grep` of the topic tapes and the showcase tape shows only `Source` lines for topics |
| Fail closed | `bgp-filter-irr` apply without a verified candidate returns an error; a failed apply undoes what it applied (SDK `OnConfigApply` contract) |
| Rule: no-layering | the seven replaced demos and their runners, validators, cards and prepared configs are gone in the same change as their replacements |
| Rule: documentation | each page edit lands with the code that made it wrong |

### Deliverables Checklist

| Deliverable | Verification method |
|-------------|---------------------|
| Eight topic recordings and the super, validated | `./le site terminal-demo` validate mode for each id, output pasted |
| Scenario registry, no switch, no map | `gopls symbols` of `scenarios.go` and `validate_runtime.go` show neither `runScenario`'s switch nor `demoValidators` |
| IRR live edit applies | `test/reload/bgp-filter-irr-added-live.ci` green, and red with the handler removed (pasted) |
| Hero and README | site build shows `showcase` as hero; README embeds the carrier; carrier measurements recorded in this spec |
| Gallery and pages | grep for each topic id's marker in `docs/guide/terminal-demonstrations.md` and on its feature page |

### Security Review Checklist

| Check | What to look for |
|-------|-----------------|
| Secrets in recordings | the IKE pre-shared key and the demo password typed on screen are demo-only values, never reused from any real config; no host path or user name leaks into a cast |
| Fail-open filter | the IRR apply path never leaves a peer unfiltered between verify and apply; a failed fetch keeps the previous set, as at boot |
| Privileged container | the shared lab runs `--privileged` like the existing network demos; nothing it starts listens outside the container |

### Failure Routing

| Failure | Route To |
|---------|----------|
| Compilation error | Fix in the phase that introduced it |
| Test fails for the wrong reason | Fix the test assertion or setup |
| Test fails on behavior mismatch | Re-read the source in Current Behavior. If misunderstood → RESEARCH |
| Lint failure | Fix inline. If architectural → DESIGN |
| Functional test fails | Check the AC: wrong AC → DESIGN, correct AC → IMPLEMENT |
| Audit finds a missing AC | Back to the relevant phase and implement |
| 3 fix attempts failed | STOP. Report all 3 approaches. Ask the user |

## Design Insights
<!-- LIVE: write immediately when you learn something. Route each lesson to its
     governing surface under ai/rules/planning.md. A problem-class journal row
     is appropriate only when no surface governs the lesson yet; closure alone
     requires no lesson artifact. -->

- The owner's "typed live" decision turns every recording into a reload test the
  product never had: no existing demo commits a topic into a running daemon. The
  first producer read under it found a fail-open (IRR, AC-1). Expect the probes to
  find more; each is fixed here under owner decision 5.

## Key Design Decisions
| Decision | Alternatives Considered | Rationale |
|----------|------------------------|-----------|
| Topic config as section snippet files (`demos/terminal/topics/<topic>/<n>-<section>.conf`) with one explanation card per section, loaded by `load file relative merge`; the beats as tape fragments (`configure.tape` loads and explains the sections, `show.tape` proves, `failure.tape` breaks and diagnoses) sourced by both the topic tape and the showcase tape; topic checks as one Go function both validators call. The snippet files are also what `sourceDigest` must cover (AC-5) | (b) a declarative per-topic data file (config lines, commands, expected output) from which a generator writes the tapes; (c) hand-written tapes per recording | (a) uses the include the recorder already has and adds no format. (b) adds a generator and a second tape dialect for the same facts. (c) is the drift R-5 names |
| One shared lab (`showcase-lab`) that every topic and the super start | per-topic labs, as today | one code path, and the super needs it anyway; a topic recording shows the same world the super does. Costs a heavier start (R-7) |
| Scenario registry replacing `runScenario` and `demoValidators` | add ten cases to the switch and ten entries to the map | `ai/rules/principles.md`: a new scenario registers; both lists are central enumerations |
| Digests over the Source closure | keep fragments inside each demo directory (duplicated) | fragments must live outside any one demo to be shared; a digest that misses them reports stale recordings current |
| Base lab config (peers, interfaces) imported at start; topics load-merged section by section in the SSH editor (owner, 2026-10-09) | type each config line live (rejected by the owner: slow) ; `ze config import` of the whole topic (not an editor verb, shows nothing section by section, and no reload path from it was read) | chapter 1 is the base and is not a topic. Load merge needs `plan/immediate/spec-session-editor-file-mode-parity.md` because session mode refuses `load` today |
| Session-mode `load merge` and `commit confirmed` (AC-13, AC-14) moved to `plan/immediate/spec-session-editor-file-mode-parity.md` (owner, 2026-10-10) | built in this spec | the owner expected both to exist already, a defect against the running router rather than recording work; the new spec also covers copy, activate, deactivate and `commit force` in session mode |
| IRR fixed in `bgp-filter-irr` | work around in the tape (restart, or stage offline) | owner decision 5 forbids the workaround, and the defect is a fail-open filter |

## Known Limitations
- WireGuard is not a chapter: `plan/spec-wireguard-runtime-proof.md` owns it (owner decision).
- PPP and L2TP are skipped (owner decision).
- The appliance kernel's IPsec modules are owned by `plan/immediate/spec-appliance-kernel-vpn-modules.md`; the recording runs on the renderer container, not the appliance.

## RFC Documentation (Scope: protocol)

N-A: no protocol code is added. The IRR fix changes when the filter's config is applied, not any RFC-governed behaviour.

## Checklist

### Pre-Spec Verification (before the design is presented)
- [ ] Metadata table present, with a valid Status, Depends, Phase and Updated
- [ ] `ai/INDEX.md` keyword table checked
- [ ] An `rfc/short/` summary exists for every RFC referenced
- [ ] Template format followed: the 🧪 emoji, tables rather than prose, `[ ]` never `[x]`
- [ ] No code snippets
- [ ] Files to Modify names feature code, not only tests
- [ ] Current Behavior and Data Flow sections completed
- [ ] AC-N rows carry testable assertions
- [ ] Every assumption has a Basis and a validation method; every failure mode is a risk row
- [ ] Required Reading carries `→ Decision:` / `→ Constraint:` checkpoints
- [ ] Integration Checklist marks "CLI grammar" when a command is added, "Doctor check" when a runtime dependency is

### Goal Gates (MUST pass)
- [ ] AC-1..AC-N all demonstrated
- [ ] Every user story has a working path and a passing test (N-A when Scope is tooling or docs, which delete that section)
- [ ] Wiring Test table complete: every row a concrete test name, none deferred
- [ ] `./le verify worktree` passes. It runs every stage against a COMMIT in a throwaway worktree, which is the pre-commit gate (`ai/rules/git-safety.md`). An in-place `./le verify current` is void the moment the tree moves under it
- [ ] Feature code integrated (`internal/*`, `cmd/*`), not library-only
- [ ] Integration and Documentation checklists answered Yes/No/N-A with evidence
- [ ] Architectural Verification table filled, including registration over hardcoding
- [ ] Critical Review passes, and `ai/rules/quality.md` is satisfied: lint fixed rather than disabled, the focused check for the changed behavior run once with its OUTPUT PASTED, and any red named with the one-line reason it is scaffolding
- [ ] Every A-N confirmed or broken, none `unvalidated`
- [ ] Every item this spec did not do is a spec of its own, named here, in its own bucket

### TDD
- [ ] Tests written
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)
- [ ] Boundary tests for all numeric inputs (or N-A when the feature takes none)
- [ ] Functional `.ci` tests for end-to-end behavior
- [ ] Interop tests for protocol features (or N-A with a reason)

### Closure
- [ ] Append `plan/TEMPLATE-CLOSURE.md` and complete every section in it
- [ ] `/ze-review` gate clean, recorded via `internal/le/spec/review.go`
- [ ] Any lesson routed to its governing surface under `ai/rules/planning.md`; no lesson artifact created merely for closure
- [ ] **Commit A:** code + tests + docs + edited spec + any journal rows owed by the work
- [ ] **Commit B:** `remove <the spec's path in its bucket>` only, in the same `./le commit create` script (commit A preserves the spec in history)
