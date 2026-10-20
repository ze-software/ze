# Spec: the ExaBGP bridge speaks ExaBGP 6.0's API, and keeps 4.2 and 5.0 helpers working

| Field | Value |
|-------|-------|
| Status | in-progress |
| Scope | protocol |
| Depends | - |
| Phase | 1/8 |
| Handoff | - |
| Updated | 2026-10-20 |

Recovery after compaction: `.claude/rules/post-compaction.md`.

## Task

Owner order (Thomas, ExaBGP's author, 2026-10-17): "Do the ExaBGP 6.0 work including
backward compatibility to 5.0 and 4.2 which was added to main recently."

ExaBGP main (6.0.0, unreleased, checkout `/Users/thomas/Code/github.com/exa-networks/exabgp/main`
at `685f772e4`, 2026-10-14) changed what a helper script may write and what it is told, and
in the same month added machinery that holds 6.0 to what 4.2 and 5.0 helpers relied on. Ze's
bridge (`internal/exabgp/bridge/`, `internal/plugins/exabgp/`) was written against 5.0.13 and
the v6 selector keyword only. This spec makes the bridge answer every helper the way ExaBGP
main answers it, proven against ExaBGP main itself, and adopts the 4.2 and 5.0 corpora ExaBGP
now checks itself against.

Seven work items, all agreed in the order:

| # | Item | Phase |
|---|------|-------|
| 6 | A second compatibility pin on ExaBGP main beside 5.0.13; the stage clears `PYTHONPATH` and names its Python; port the new `qa/api` cases and the `qa/old` suites | 1 (first) |
| 1 | Detect the API version per helper (`auto`, 4, 6), `api version` command, `exabgp.api.version`, encoders per version | 2 |
| 2 | The error answer: v4 `error: <why>` then `error`; v6 `{"error":"<why>"}` then `error` | 3 |
| 3 | The v6 vocabulary, `group start/end`, inline `group a ; b`, `on-exit keep\|withdraw`, routes of an exiting helper withdrawn | 4 |
| 4 | `announce ipv4 rtc`, `announce ipv4 mpls-vpn` (SAFI 128), SR Policy with attributes (absorbs `plan/immediate/spec-api-sr-policy-attributes.md`) | 5 |
| 5 | ExaBGP's new refusals and keep-first rules | 6 |
| BC | 4.2 and 5.0 compatibility: the command and response corpora, the 5.x answer shapes ExaBGP restored | 7 |
| J | Journaled bridge defects: the stage's `PYTHONPATH`/Python (`plan/journal/gate-verdict-depends-on-the-machine.md`, rows 2026-10-17 and 2026-09-13); the `Net` comment in `bridge_batch.go` | 1 |

The SR Policy skeleton `plan/immediate/spec-api-sr-policy-attributes.md` is SUPERSEDED by
this spec: its design question is decided here (Key Design Decisions, row "SR Policy
attributes through the API"), its evidence table is carried into Current Behavior, and its
file is removed through `./le commit create ... remove` in the commit that lands this spec.

## Required Reading

### Architecture Docs
- [ ] `docs/architecture/exabgp-bridge.md` - the bridge's two directions, batching, ack, encoder, respawn
  → Decision: `TranslateLine` answers a `Translation` carrying `Commands`, `Selector`, `Route`, `Local`, `Unmatched`; every new form keeps that contract and states its `Command.Key` at the builder
  → Constraint: the fan-out reads each event ONCE and renders each format ONCE (`bridgerun.Fleet.Broadcast`); per-version encoders add formats (v4 text, v4 JSON, v6 JSON), never per-script renders
  → Constraint: answers run in write order per LINE (`AnswerBatch`), so a group's held lines and the error detail line must keep that order
  → Constraint: page section "`encoder` says which format a script reads" already records the divergence from e2142a435; that paragraph is rewritten by Phase 2, not appended to
  → Constraint: page section "The destination keyword" says the v6 control vocabulary is refused by name; Phase 4 rewrites it
- [ ] `docs/exabgp/exabgp-differences.md` - intentional differences; every deliberate deviation this spec keeps is recorded there
  → Decision: the unknown-command detail line (Key Design Decisions) and the Operational refusal are recorded here
- [ ] `docs/exabgp/exabgp-migration.md` - `ze exabgp migrate` operator guide
  → Constraint: `on-exit`, `api.version` and the 4.2/5.0 extended-next-hop rule reach the migration and its page in the same phase
- [ ] `docs/exabgp/exabgp-code-map.md`, `docs/exabgp/exabgp-comparison-report.md` - map of ExaBGP modules to Ze
  → Constraint: section 2.3 (`reactor/api/`) gains `dispatch/version.py`, `dispatch/v6.py`, `command/group.py` rows naming the Ze functions this spec adds
- [ ] `ai/patterns/bgp-family.md` - BGP family gate (RTC and VPN through the bridge, SR Policy attributes through the API)
  → Constraint: section 5b (ExaBGP migration sync) applies: `rtc` reaches `bridgeSAFI`; the 12-section checklist is answered in Integration Checklist below
- [ ] `docs/architecture/testing/interop.md` - interop suites, vacuity traps
  → Constraint: each scenario is named (no numeric prefix) and red-proven by reverting the change and rebuilding the image

### RFC Summaries (Scope: protocol)
- [ ] `rfc/short/rfc4684.md` - RTC NLRI the bridge must now originate
  → Constraint: RFC 4684 Section 4: "The NLRI field in the MP_REACH_NLRI and MP_UNREACH_NLRI is a prefix of 0 to 96 bits, encoded as defined in Section 4 of [5]." and "Except for the default route target, which is encoded as a zero- length prefix, the minimum prefix length is 32 bits." So valid lengths are 0 and 32..96; 1..31 and 97+ are refused
- [ ] `rfc/short/rfc4364.md` - VPN-IPv4 NLRI (SAFI 128) for `announce ipv4 mpls-vpn`
  → Constraint: the route travels in MP_REACH_NLRI with label and RD; ExaBGP 5.0 packed it in the unicast NLRI field, and 6.0 corrected that (CHANGELOG "Incompatible: announce ipv4 and ipv6 mpls-vpn ... send the route as SAFI 128")
- [ ] `rfc/short/rfc9830.md`, `rfc/short/rfc9012.md` - SR Policy NLRI and the Tunnel Encapsulation attribute the API must originate
  → Constraint: RFC 9830 Section 4.2.1 "Validation of an SR Policy NLRI": "When a BGP speaker receives an SR Policy NLRI from a neighbor, it MUST first perform validation based on the following rules in addition to the validation described in Section 5"; ExaBGP 6.0 refuses to SEND an SR Policy route that a receiver would treat as withdraw (no NO_ADVERTISE or IPv4-address Route Target, or no Tunnel Encapsulation); Ze's API path inherits whatever `parseConfigRoute` enforces, no second rule
- [ ] `rfc/short/rfc7911.md` - path-information
  → Constraint: RFC 7911 Section 3: the Path Identifier is four octets; `path-information` with no value is a missing field, refused (ExaBGP CHANGELOG 6.0.0 "path-information with no value is refused")
- [ ] `rfc/short/rfc8955.md` - flow route with no component
  → Constraint: a flow route with no match component matches every packet; ExaBGP 9c65367d0 refuses it on the one-line forms; Ze already refuses (`errFlowRouteNoMatch`), this spec adds the 6.0-pinned proof
- [ ] `rfc/short/rfc4760.md`, `rfc/short/rfc4271.md` - MP_REACH vs legacy NLRI sections, NEXT_HOP once per UPDATE
  → Constraint: RFC 4271 Section 5: each path attribute appears at most once in an UPDATE, so a repeated `next-hop` keeps one value; ExaBGP keeps the FIRST (bc37ddcf3)

**Key insights:** (minimal context to resume after compaction)
- ExaBGP main detects each helper's API from its first deciding command: first word in {daemon, session, system, rib, peer} is v6; `neighbor` or any command `translate_v4_to_v6` translates is v4; anything else decides nothing (`command_api_version`, `dispatch/version.py`). `group` always goes to the v6 dispatcher (031ab8fd4)
- Encoder: v4 helper gets v4 text or v4 JSON (envelope `4.0.1`) per its `encoder`; v6 helper gets v6 JSON whatever `encoder` says (warned); undecided gets v4 text when `encoder` is text (the default) and v6 JSON when it is json (`Processes._select_encoder`, `detect_api_version`)
- Error detail: `_error_detail` writes `{"error": "<why>"}` for a v6 helper (or ack-json), else `error: <why>`, then the `error` marker; an unknown command or a selector matching no peer answers the marker alone; ack disabled drops detail and marker alike (`_answer` without force)
- `on-exit` unset: v4 helper keeps its routes, v6 or undecided has them withdrawn (`Processes._on_exit`, 747da3108)
- The API keeps 5.0's leniency where a config now refuses: a repeated single-value attribute keeps the first and warns, a `split` shorter than the prefix sends the prefix and warns (0f4da74a7)
- Ze's API path for SR Policy hands the section to `EncodeNLRIHex` (NLRI only) since `bfee50f01d` removed `update_text_srpolicy.go`; attributes are refused there; `parseConfigRoute` is the one parser that builds them
- An untracked `test/exabgp-compat/api/api-sr-policy.ci` (+ `.conf`, `.run`, 2026-10-17 20:33) from another session already exists and asserts SR Policy announce/withdraw with segment lists

## ExaBGP Behaviour Mirrored (source of truth per item)

Every row was read at the producer in ExaBGP main `685f772e4`. "Quote" is the docstring or
CHANGELOG sentence of that producer.

| ID | Behaviour | ExaBGP function (file) | Commit | Quote |
|----|-----------|------------------------|--------|-------|
| X-1 | API version of one command | `command_api_version` (`src/exabgp/reactor/api/dispatch/version.py`) | e2142a435 | "A v4 command is one the v4 dispatcher knows: `neighbor ...`, or one with a v6 translation. A command neither dispatcher knows decides nothing" |
| X-2 | Version held per helper, once | `Processes.detect_api_version` (`reactor/api/processes.py`) | e2142a435 | "The first command whose form belongs to one version holds the helper to it for as long as it runs. Undecided commands leave it undecided." |
| X-3 | Cross-version command refused | `dispatch_for` (`dispatch/version.py`) | e2142a435 | "a v4 helper writing a v6 command gets the same UnknownCommand as a v6 helper writing a v4 one. Before the version is known, the v4 dispatcher is used, as it accepts both forms." |
| X-4 | `group` from any version | `dispatch_for` | 031ab8fd4 | "`group` is the v6 dispatcher's whatever the version" |
| X-5 | Encoder per version | `Processes._select_encoder`, `_adopt_v4`, `_adopt_v6` | e2142a435 | "An undecided helper is written to as its `encoder` asks, v4 text or v6 JSON, until its version is known"; "API v6 is JSON only, encoder text is ignored"; default `configuration.get('encoder', 'text')` |
| X-6 | Envelope versions | `exabgp/version.py` `json_v4 = '4.0.1'`, `text_v4 = '4.0.1'`, `json = _base` (6.0.0) | e2142a435 | "Legacy API v4 JSON version string (frozen)" |
| X-7 | `api version [auto\|4\|6]` | `api_version_cmd`, `_show_api_version` (`command/reactor.py`); `api_version` (`environment/parsing.py`) | e2142a435 | "api version auto - Detect each helper's version from its commands (default) ... Note: a change applies to helpers started afterwards." Show answers `API version: <setting> (this helper: <current>)` text, `{"api_version":..,"helper_api_version":..}` JSON; set answers `API version set to <v> (effective for helpers started afterwards)`; invalid answers `error: API version must be auto, 4 or 6, got <v>` |
| X-8 | `exabgp.api.version` default auto | `environment/config.py`, `parsing.api_version` | e2142a435 | "`auto` (0), each helper's version detected from its commands, or 4 or 6 for all of them." |
| X-9 | v4 JSON open capabilities by code | `response/json.py`, `response/v4/json.py` | 0e8dc3bd2 | "the capabilities of an OPEN filed under their code ("1", "65"), without "code" inside" |
| X-10 | v6 JSON open capabilities by name | `response/json.py` | 923e83956 | "capabilities": { "asn4": { "code": 65, "name": "asn4", "asn4": 65001 } } |
| X-11 | v4 shapes restored from 5.x | `response/v4/text.py`, `nlri/label.py`, `nlri/qualifier/labels.py` | 0e8dc3bd2 | "a label as its value ("label": [[100]]) ... the "string" of a flow route ending with its next-hop ... a cluster list of one cluster in brackets" |
| X-12 | v4 NOTIFICATION data | `Notification.v4_data` (`bgp/message/notification.py`) | dc1d55da1, 0e8dc3bd2 | "what 5.x hexed, the Shutdown Communication decoded, else the wire" |
| X-13 | Error detail per version | `Processes._error_detail`, `answer_error_sync`, `answer_error` | 1c8ce1903 | "The reason now goes to an API 6 helper as {"error": "<why>"}, from every command, before the error line; an API 4 or undecided helper keeps the `error: <why>` 5.x wrote." |
| X-14 | Unknown / no-peer answer | `API.process` (`reactor/api/__init__.py`) | e2142a435, 3d2f35533 | `except UnknownCommand: ... answer_error_sync(service)`; same for `NoMatchingPeers` (marker alone) |
| X-15 | v6 control vocabulary | `_build_v6_tree` (`dispatch/v6.py`); `V4_SIMPLE_TRANSLATIONS`, `translate_v4_to_v6` (`dispatch/v4.py`) | e2142a435 | table in Current Behavior, item 3 |
| X-16 | `group start` / `group end` | `group_start`, `group_end`, `_process_group`, `_answer_group` (`command/group.py`) | 3452111e8, 3f2101b9d | "A group answers what it applied, and its errors, then done ... The group now ends with error when one of its commands could not be applied." Text answers `group started`, `group ended (0 commands)`, `group processed: <n> announced, <m> withdrawn[, <k> errors]`; nested start answers `error: already in group block (nested groups not allowed)`; stray end answers `error: not in group block` |
| X-17 | v6 announce inside a group waits | `API._held_by_group` (`reactor/api/__init__.py`) | 3f2101b9d | "A command dispatched to the API 6 announce or withdraw is now held in the open group, with the peers its selector names ... An EOR or a route-refresh is still sent as it comes." |
| X-18 | Inline group, selector optional | `dispatch_v6` (`dispatch/v6.py`), `group_inline` | eeb2559e4 | "With no selector an inline group is now for every peer of the helper, as `announce ...` is: the dispatcher reads it as `peer * group ...`." |
| X-19 | Inline group split rule | `lexer.split_commands` (`configuration/grammar/lexer.py`) | 899cb1170 | "ends a command where the lexer ends a statement: outside quotes and braces, escapes resolved" |
| X-20 | Group `attributes` shares | `_process_group` | d540785a9 | "First command starting with 'attributes' sets shared attributes"; "the next-hop of an attributes line with no nlri is given to the routes which give none of their own" |
| X-21 | Routes withdrawn on exit | `Processes._queue_exit`, `Reactor` exit marker (`reactor/loop.py`), `withdraw_owner` (`rib/outgoing.py`) | f64e25ed9 | "The last announcer of a prefix owns it, and a withdraw from anywhere clears it. withdraw_owner() withdraws a process's routes, and gives back a configured route one of them had replaced ... A respawned process starts with nothing announced." |
| X-22 | `on-exit` default by version | `Processes._on_exit` | 747da3108 | "Unset, a helper using API 4 keeps its routes, as 4.x and 5.x did, and any other has them withdrawn (issue #304)." Each exit logs `process.exited process=<p> routes=<keep\|withdraw> by=<on-exit\|API 4\|API 6\|API undecided>` |
| X-23 | Family no selected peer carries | `carrying`, `unsendable` (`command/announce.py`) | 3b71dda20 | "An EOR or a route-refresh goes to the established peers which carry its family, and is answered error when there are none. With `peer *`, a family one selected peer carries is still sent to it and answered done." |
| X-24 | Selector matching no peer | `dispatch` (`dispatch/common.py`) | 3d2f35533 | "The dispatcher now refuses a selector which matches no peer, as the API 4 dispatcher already did for `neighbor <ip> ...`" |
| X-25 | Flow with no match | `grammar/tree/flow.py` | 9c65367d0 | "the one-line forms of the API and the configuration now give the same error" |
| X-26 | `next-hop` twice | `grammar/tree/announce.py`, `static.py` | bc37ddcf3 | "The first wins for both"; CHANGELOG "an API command keeps the first and logs which it dropped, as for any attribute" |
| X-27 | Repeated attribute, short split | `read_command` READING_COMMAND, `add_attribute`, `split` (`grammar/read.py`, `tree/static.py`) | 0f4da74a7 | "a file refuses, an API command keeps the old behaviour and logs a warning"; "split() sends the prefix itself and warns" |
| X-28 | path-information no value | grammar | 08e69a350 (CHANGELOG) | "\"path-information\" with no value is refused, in the configuration and on the API. 5.0 ignored it." |
| X-29 | Malformed prefix mask | grammar | CHANGELOG 6.0.0 | "a prefix whose mask is no number, `10.0.0.0/x`, is refused, in the configuration and on the API. It was taken as the host route 10.0.0.0/32." |
| X-30 | Operational kind | `announce_operational` (`command/announce.py`) | 376898cfc | "a kind it does not know ... is answered with an error naming the command" |
| X-31 | `announce ipv4 rtc` | `RTC_VALUES`, `AnnounceSafi(RTC_VALUES, RTC, prefix=False)` (`grammar/tree/announce.py`) | 6961c85e0 | forms `announce ipv4 rtc origin-as <asn> route-target <rt> next-hop <ip\|self>` and `announce ipv4 rtc default next-hop <ip\|self>`; "rtc keeps refusing" the static-route attribute keywords |
| X-32 | `announce ipv4 mpls-vpn` SAFI 128 | grammar `announce.py` | 08e69a350 (CHANGELOG) | "\"announce ipv4 mpls-vpn\" and \"announce ipv6 mpls-vpn\" from the API send the route as SAFI 128, as \"announce route ... rd ... label\" does." |
| X-33 | Family announce takes static keywords | `grammar/tree/announce.py` | 6961c85e0 | "They are read as on a static route now, and split stands for the more specifics as it does there" |
| X-34 | SR Policy send rule | grammar `sr-policy` | CHANGELOG 6.0.0 | "an SR Policy route is refused unless it carries the NO_ADVERTISE community or a Route Target in IPv4-address format, and a Tunnel Encapsulation (RFC 9830 4.2.1)"; "a word after the sub-TLVs of an sr-policy route is refused" |
| X-35 | 4.2/5.0 corpora | `qa/bin/test_old_commands`, `test_old_responses`, `test_old_scripts`, `qa/old/` | e6db92582, 12cac3c31, 7a4f0716d, d64da83f5 | "gives each recorded command to this tree's API 4 dispatcher and handler and compares the routes announced and withdrawn"; "A JSON line is compared parsed, without its time, host, process ids and counter ... A text line is compared as it is." |
| X-36 | Nightly zipapp | `.github/workflows/zipapp.yml` | 510cebfd4 | "every night main changed under src/ since the last nightly, it replaces the zipapp of the `nightly` pre-release, whose tag is moved to the commit built: https://github.com/Exa-Networks/exabgp/releases/download/nightly/exabgp.pyz" |
| X-37 | IPv4 route with IPv6 next hop in a 4.2/5.0 config | `_extended_next_hop` (`grammar/tree/neighbor.py`) | cc0b3b24a | "the configuration adds the entry and the Extended Next Hop Encoding capability itself, with a warning" |

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `internal/exabgp/bridge/bridge_command.go` - `Translator.Line` collapses spaces, `splitNeighborSelector`, then `convertControl` (enable/disable/silence-ack, shutdown, clear/flush adj-rib, watchdog), `convertNeighborControl`, `convertRoute`, `help` passthrough; anything else `ErrLineNotTranslated`. No version state. `convertAnnounceSRPolicy` appends leftover tokens to `mp <afi>/sr-policy add`
  → Constraint: `Translation.Unmatched` is answered `done` today ("ExaBGP acknowledges a command whose selector matched no session"), which X-24 reverses
- [ ] `internal/exabgp/bridge/bridge_family.go` - `bridgeSAFI` holds unicast, multicast, nlri-mpls, flow(spec), flow(spec)-vpn, mcast-vpn, mup; NOT `mpls-vpn` (the constant `bridgeVPNSAFI` exists but is not a key) and NOT `rtc`; `bridgeFamilyRE` is built from `bridgeSAFI`
  → Constraint: adding a key to `bridgeSAFI` is the one edit that admits a family to `announce <afi> <safi>`; `TestSAFIVocabularyRoundTrips` holds `exabgpSAFI` to it
- [ ] `internal/exabgp/bridge/bridge_selector.go` - `bridgeSelectorKeyword` reads `neighbor` or `peer`; `splitBracketSelector` reads the v6 bracket list
- [ ] `internal/exabgp/bridge/bridge_neighbor.go` - `convertNeighborControl`: teardown, `create neighbor`, `delete neighbor`, `receive` refused as an event
- [ ] `internal/exabgp/bridge/bridge_ack.go` - `AckMode.WriteError` writes ONE line `error <sanitized message>`; `WriteAck` writes `done`; both silent when ack disabled; `ackEnvKey = "exabgp.api.ack"`
  → Constraint: the one-line `error <msg>` matches neither X-13 form; it is replaced, not kept beside (`ai/rules/no-layering.md`)
- [ ] `internal/exabgp/bridge/bridge_encoder.go` - `Encoder` is `EncoderUnspecified|EncoderJSON|EncoderText`
- [ ] `internal/exabgp/bridge/bridge_event.go` - `Version = "6.0.0"` for every JSON event; `ExabgpJSON` has arms for state, update, notification (code, subcode, data as ze gave it), negotiated; an `open` event carries the header only, no capabilities object
- [ ] `internal/exabgp/bridge/bridge_event_text.go` - `AppendText` is ExaBGP's v4 text encoder, ported
- [ ] `internal/exabgp/bridge/bridge_attribute.go` - `parseRouteAttributes`: a repeated attribute is appended twice to the ze tokens; `split` with a length shorter than the prefix is refused by `splitPrefix` (`bridge_route_forms.go`, "/%d does not cut %s"); `pathIDNumber` takes a number or a dotted IPv4
- [ ] `internal/exabgp/bridge/bridge_route_forms.go` - `defaultPrefixLengths` completes a bare address to a host prefix and leaves anything unparsable as is (so `10.0.0.0/x` reaches ze's parser, whose refusal names no ExaBGP rule)
- [ ] `internal/exabgp/bridge/bridge_flow.go` - `errFlowRouteNoMatch` already refuses a flow route with no match component
- [ ] `internal/exabgp/bridge/bridge_batch.go` - `Net` comment line 194 says "`announce X` then `withdraw X` puts neither"; the code and the page table dispatch the withdrawal (journaled defect)
- [ ] `internal/plugins/exabgp/bridgerun/script.go`, `fleet.go` - one child per process, respawn limiter, per-child queue; no record of what a script announced; exit does nothing to its routes
- [ ] `internal/plugins/exabgp/bridgeplugin/yang/ze-exabgp-bridge-conf.yang` - process leaves `name`, `run`, `encoder` (default json), `feed`, `respawn` (default true); no `on-exit`
  → Constraint: changing `encoder`'s default to text is a YANG default change; `ze exabgp migrate` writes the leaf only when the ExaBGP config carried one, so a migrated config follows ExaBGP; a hand-written Ze config that omitted it changes format (R-3)
- [ ] `internal/exabgp/migration/env.go` - `api.ack` migrated; `api.version` is not handled; `internal/component/config/apply_env.go` maps `exabgp.api.ack` to the OS env the bridge reads
- [ ] `internal/component/bgp/plugins/cmd/update/update_text_nlri.go` - `parseRegistryNLRISection` hands a plugin family's `add|del` token run to `encodeViaRegistry`, whose `registry.EncodeNLRIByFamily` returns NLRI bytes only; SR Policy reaches `srpolicy.EncodeNLRIHex` (`parseSRPolicyNLRIArgs`), which knows distinguisher, color, endpoint only
- [ ] `internal/component/bgp/plugins/nlri/srpolicy/config.go` - `parseConfigRoute` (registered `InProcessConfigRouteParser`) parses distinguisher, color, endpoint, preference, priority, binding-sid (mpls, null, srv6), segment-list, policy-name, candidate-path-name and returns `registry.PluginRoute` with `NLRI` and `Attrs` (Tunnel Encapsulation, code 23)
- [ ] `internal/component/bgp/plugins/nlri/rtc/register.go` - RTC registers `InProcessDecoder` and `InProcessNLRIDecoder` only: Ze decodes RTC and cannot originate it from text
- [ ] `internal/le/test/functional/exabgp.go` - `exaBGPPythonPackage = "exabgp==5.0.13"`; `exaBGPCommands` runs `uv run --with paramiko --with exabgp==5.0.13 le test exabgp <suite>` for `encoding` and `api`, with `set.Environment(toolchain)`, which inherits the shell's `PYTHONPATH` and lets uv choose the interpreter
- [ ] `internal/test/cli/cmd_exabgp.go` - the suite runner: `startExaBGPClient` runs `ze start` on the config `migrateExaBGPConfig` produced with `ze exabgp migrate`; the peer is `test/exabgp-compat/bin/bgp`

**SR Policy skeleton evidence, carried over (re-read 2026-10-17):**

| Item | Value now |
|------|-----------|
| API parser | `update_text_srpolicy.go` is gone (`bfee50f01d`, 2026-10-16, "SR-Policy text encoding goes through the registry"); the section reaches `srpolicy.EncodeNLRIHex` through `parseRegistryNLRISection`, which refuses every token but the three NLRI fields |
| ExaBGP bridge | `convertAnnounceSRPolicy` still appends `preference ...`, `segment-list ...` to `mp ipv4/sr-policy add`, so every SR Policy announce with an attribute is refused by the API |
| Parser that carries the attributes | `srpolicy.parseConfigRoute`, reached from config through `registry.ConfigRouteParserByFamily` and from `ze bgp encode` through `EncodeRoute`; not from the API |
| Journal row | `plan/journal/parser-refuses-a-form-its-producer-emits.md`, 2026-10-14 row; repointed to this spec |
| Citing spec | `plan/immediate/spec-bgp-sr-policy-rfc-defects.md` Known Limitations and its 2026-10-14 log row; repointed to this spec |

**Per-item Ze state:**

| Item | Ze today (function) | Gap |
|------|---------------------|-----|
| 1 detection | none; `Translator` has no version; `bridge_selector.go` accepts `peer` and `neighbor` from any script | no hold, no cross-version refusal, no per-version encoder |
| 1 `api version` | refused (`ErrLineNotTranslated`) | command absent |
| 1 env | `exabgp.api.version` unmapped in `apply_env.go`, unmigrated in `migration/env.go` | absent |
| 1 encoder default | YANG `encoder` default json; JSON always `6.0.0` | undecided text helper gets JSON; v4 JSON helper gets `6.0.0` |
| 1 open JSON | `ExabgpJSON` writes no capabilities | X-9, X-10 absent |
| 2 errors | `AckMode.WriteError` one line `error <msg>` | wrong shape for both versions |
| 3 v6 vocabulary | refused by name (page "The destination keyword") | translation absent |
| 3 group | refused | absent |
| 3 on-exit | absent in YANG, migration and fleet | absent; routes of every exited script stay |
| 4 rtc | `rtc` not in `bridgeSAFI`; no RTC encoder in Ze | absent end to end |
| 4 mpls-vpn | `mpls-vpn` not in `bridgeSAFI` (only `announce route ... rd` reaches VPN) | `announce ipv4 mpls-vpn` refused |
| 4 SR Policy | attributes refused by the API | see skeleton evidence |
| 5 family not carried | ze's reactor decides per peer (`session_validation.go` `ErrFamilyNotNegotiated` path); what the bridge answers is unverified (A-4) | X-23 unproven |
| 5 selector no peer | `Unmatched` answered done | reversed by X-24 |
| 5 flow no match | refused (`errFlowRouteNoMatch`) | no 6.0-pinned case |
| 5 next-hop twice | both tokens forwarded; ze's parser decides (unverified, A-5) | keep-first + warning absent |
| 5 repeated attribute | both forwarded | keep-first + warning absent |
| 5 short split | refused | X-27: send the prefix and warn |
| 5 path-information no value | `takeAttrValue` refuses a missing value (unverified wording, A-6) | needs X-28 case |
| 5 malformed mask | passes through to ze's parser | X-29 refusal in ExaBGP's terms |
| 5 operational | every `announce operational` refused as untranslated (Ze does not originate OPERATIONAL) | unknown kind is refused already; known kinds are an absent feature (Open Owner Decision OD-1) |
| BC corpora | none | X-35 absent |
| J PYTHONPATH | inherited; uv picks CPython 3.8.20 on this workstation | journaled |
| J Net comment | says "puts neither" | stale comment |

**Behavior to preserve:**
- Every existing `test/exabgp-compat/api/*.ci` and `encoding/*.ci` case under the 5.0.13 pin stays green, unchanged except where a case asserted behaviour this spec reverses (selector-no-peer `done`, encoder default), each such edit named in the commit body
- The batching rules of `Net`, the answer order of `AnswerBatch`, the per-child queue, the respawn limiter
- `neighbor` and `peer` selectors, bracket lists, `Translator.Peers` scoping
- `help` passthrough

**Behavior to change:** items 1 to 5, BC and J, as the tables above state.

## Data Flow (MANDATORY - see `ai/rules/architecture.md`)

### Entry Point
- A helper script's stdout line (ExaBGP API text), read by `bridgerun.script` into a batch (`BatchReader`)
- Engine events (ze JSON) fanned out by `bridgerun.Fleet.Broadcast` to each script's stdin
- Config: `exabgp { bridge { process <name> { encoder; on-exit; ... } } }` and `environment { exabgp { api { ack; version } } }`

### Transformation Path
1. `script` reads a batch; for each line, the script's version state (auto, 4, 6) is consulted and updated by the detection rule (X-1, X-2) BEFORE translation
2. `Translator.Line` translates with the version: a cross-version line is refused (X-3); `group` lines are handled by the script's group buffer (X-16 to X-20); v6 control words map to the same ze commands their v4 spelling maps to
3. `Net` nets the batch; `dispatchBatch` sends ze commands; per-selector flush; `AnswerBatch` answers per line in the version's error form (X-13, X-14)
4. Route commands that carry a route record `(script, selector, RouteKey) -> withdraw command` in the fleet's owner ledger; a later announce of the same key by another script transfers ownership, any withdraw clears it (X-21)
5. On child exit, the `on-exit` policy (X-22) decides; withdraw dispatches the ledger's withdrawals through the same dispatch path, before the respawned child's first batch
6. Up: `ReadEvent` once; `Broadcast` renders once per format in use among {v4 text, v4 JSON, v6 JSON}; each script receives the format its current version and `encoder` select (X-5)
7. SR Policy through the API: `parseRegistryNLRISection` hands the section to the family's registered config-route parser when one is registered, receiving NLRI bytes plus `PluginRouteAttr` attributes that join the UPDATE's attribute set

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Script ↔ bridge | ExaBGP API text down, ExaBGP JSON or text up, `done`/`error` answers | No |
| Bridge ↔ engine | `send bgp <selector> update text ...`, `request ...`, `create/delete bgp peer` over the plugin RPC | No |
| Update-text parser ↔ plugin registry | `registry.ConfigRouteParserByFamily` (new caller), `EncodeNLRIByFamily` | No |
| RTC plugin ↔ registry | new `InProcessNLRIEncoder` | No |

### Integration Points
- `bridgerun.script` - owns the version state and the group buffer (both per child, reset on respawn: X-2 "until it restarts", X-21 "A respawned process starts with nothing announced")
- `bridgerun.Fleet` - owns the owner ledger (cross-script ownership) and the per-format render set
- `bridge.AckMode` - gains the version-aware error writer
- `registry.ConfigRouteParserByFamily` - reached from the API for families whose section carries attributes
- `config.ApplyEnvConfig` table (`apply_env.go`) - one row for `exabgp.api.version`, as `exabgp.api.ack`

### Architectural Verification
| Check | Holds? | Evidence |
|-------|--------|----------|
| No bypassed layers (data flows through the intended path) | Yes | exit withdrawals and group batches go through `dispatchBatch` and the per-selector flush like any batch |
| No unintended coupling (components stay isolated) | Yes | the engine learns nothing ExaBGP-specific; version, group and ownership live in `internal/exabgp` and `internal/plugins/exabgp` |
| No duplicated functionality (extends existing, does not recreate) | Yes | SR Policy attributes reuse `parseConfigRoute`; the v6 words map onto the existing v4 translations; RTC reuses the rtc plugin's types |
| Zero-copy preserved where applicable (refs, not copies) | Yes | the render-once fan-out is kept; formats grow from 2 to at most 3 |
| Registration over hardcoding, outbound | Yes | RTC's text encoder registers in `nlri/rtc/register.go`; SR Policy's section parser is its already-registered `InProcessConfigRouteParser` |
| Registration over hardcoding, inbound | Yes | searched: `bridgeSAFI` (bridge family vocabulary, its own declaration, `rtc` and `mpls-vpn` keys added there); `nativeTextFamily` (not touched: RTC goes through the registry); `isSupportedFamily` (derives from the registry); `endsRegistrySection` (unchanged); the v6 tree is a translation of ExaBGP's vocabulary, the external contract, not a ze registry |

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis (file/doc/user statement) | If wrong | Validated by | Status |
|----|-----------|--------------------------------|----------|--------------|--------|
| A-1 | The nightly zipapp of ExaBGP main runs as a daemon on the stage's Python (3.12) and exposes `exabgp.application.healthcheck` when placed on `PYTHONPATH` | `zipapp.yml` builds and tests it on 3.12 and 3.14; `doc/user/zipapp.md` | the main pin needs a wheel built from the pinned commit instead | Phase 1 `TestExaBGPMainPinCommands` plus one manual run of `api-healthcheck-module` under the main pin | broken (2026-10-18): the `nightly` release's asset is replaced and its tag moved each night main changes (`gh release view nightly -R Exa-Networks/exabgp`: built from b6bf80875, not the checkout's 685f772e4), so no zipapp digest stays fetchable. The main pin is `exabgp @ git+https://github.com/Exa-Networks/exabgp@685f772e4ede5c818d2ff5c84f4ae040afd37be6`, built and cached by uv; it resolves `6.0.0-20261018+uncontrolled` and imports `exabgp.application.healthcheck` on 3.12, and `api-healthcheck-module` is green against ze under it |
| A-2 | ExaBGP main's `qa/sbin/bgp` and Ze's `test/exabgp-compat/bin/bgp` read the same `.ci` grammar, so a case can be run against the ExaBGP daemon to prove its expectation | both come from ExaBGP's `qa/` lineage (`docs/architecture/exabgp-bridge.md`) | the proof run needs ExaBGP's own peer for the main pin | Phase 1: run three existing cases against the zipapp daemon | confirmed (2026-10-18): `le test exabgp api --dut exabgp` passes api-fast, api-announce, api-check against ExaBGP main, and 37 of 42 existing cases against 5.0.13 |
| A-3 | The suite's `.conf` files are ExaBGP format that ExaBGP 6.0's stricter configuration grammar accepts | `cmd_exabgp.go` migrates them; ExaBGP 6.0 CHANGELOG lists configuration refusals | a case's config needs editing to load in 6.0; each edit is named | Phase 1 proof run over the whole `api` population | broken in part (2026-10-18), proof mode over the 42 cases before the port: against 5.0.13, red api-health, api-sr-policy, api-vpnv4, api-peer-lifecycle (timeout), api-reload (refused: a reload); against main, red api-attributes-vpn, api-flow, api-flow-merge, api-ipv4-nexthop-ipv6, api-mvpn, api-peer-lifecycle, api-reload, api-sr-policy, api-teardown, api-vpnv4. Since proof mode runs reloads (2026-10-18, every config to `exabgp server`, SIGUSR1 to its pid): api-reload passes under main (PYTHONPATH cleared); under 5.0.13 the reload happens, then 5.0.13 respawns the helper five times ("Too many death for announce-routes (5)") and the mock never gets its sequence 3 withdrawal. Each is a case whose expectation is not that ExaBGP's; the phase owning its item rewrites it from that ExaBGP's output |
| A-4 | Ze's reactor returns an error to the bridge when no selected peer carries the family, rather than dropping silently | `ErrFamilyNotNegotiated` in `session_validation.go` | the bridge must compute "carrying" itself from the peers' negotiated families | `TestFamilyNoSelectedPeerCarriesAnswersError` against the real engine | WRONG, fixed 2026-10-20: the engine answers status `warning`, which the record wire drops to `done`; the bridge reads the answer document (`RefuseUnsentRoute`); `api-refusals-v4` red on ze with that refusal overlaid away |
| A-5 | ze's update-text parser takes the LAST of a repeated `nhop`, so forwarding both tokens sends the second | not read | keep-first must still be done in the bridge either way; only the red test's expected failure changes | Phase 6 red run | validated 2026-10-20: with keep-first overlaid away, `api-keep-first-v4` on ze put NEXT_HOP 2.2.2.2 on the wire |
| A-6 | `takeAttrValue` already refuses `path-information` with no value | `errMissingAttrValue` in `bridge_attribute.go` | Phase 6 adds the refusal | `TestPathInformationWithoutValueRefused` red run | validated 2026-10-20: the refusal predates Phase 6; the test is red only with the reason's wording overlaid away |
| A-7 | `parseConfigRoute` can be called from the API with the section's run as `Content` and the text's next hop, and the returned `Attrs` merge into the UPDATE the text parser builds | `registry.ConfigRouteRequest`, `PluginRoute.Attrs` | the API needs a section-level parser entry in the registry instead (Key Design Decisions alternative) | `TestAPISRPolicyAttributesReachParseConfigRoute` | unvalidated |
| A-8 | A route a script announces and the same prefix in Ze's static config are distinct entries, so withdrawing the script's route cannot remove the configured one | ExaBGP "gives back a configured route one of them had replaced" | the exit withdrawal must re-announce the configured route | `TestExitWithdrawRestoresConfiguredRoute` | broken (2026-10-20): an API withdrawal took the configured route off the wire (`api-on-exit-configured` red on ze, green on the pin); fixed by `restore-configured` (Phase 4 third package for prefix routes, fourth package for routes with an RD or a label stack) |
| A-9 | The untracked `api-sr-policy` case belongs to a session that has stopped writing it | files dated 2026-10-17 20:33, no session state names it | Phase 5 coordinates instead of adopting | `ls -dt tmp/session/*/` at Phase 5 start | unvalidated |
| A-10 | The 4.2/5.0 corpora (`commands-*.json`, `responses-*.json.gz`) may be vendored into Ze (licence) | ExaBGP is 3-clause BSD; the files are generated data | they are fetched at test time instead | read `COPYRIGHT` in the ExaBGP checkout before vendoring and quote the clause in the commit | unvalidated |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | Detection holds a 5.0 script to v6 because its first line is `peer ...` from a shared helper library | a 5.0.13-pinned case goes red with `error` on its second command | correct by ExaBGP's rule (X-1); the 5.0.13 pin is the regression net |
| R-2 | A script's first line is undecided (`help`, a typo), so it reads text while ze already rendered JSON | encoder flips mid-stream | encoder selection follows X-5 exactly: undecided reads per `encoder`, switch happens at detection, as ExaBGP does |
| R-3 | Changing the YANG `encoder` default from json to text changes every Ze-native config that omitted it | a `test/plugin` or exabgp-compat case expecting JSON goes red | grep every `exabgp { bridge` config in `test/`; add `encoder json` where the case means JSON; record in `docs/exabgp/exabgp-differences.md` that the difference is gone |
| R-4 | The owner ledger grows unbounded with a script that announces many routes | memory in a long run | the ledger holds one entry per (selector, RouteKey), cleared by withdraw; bounded by the routes the script holds, as ExaBGP's RIB is |
| R-5 | Exit withdrawals race the respawned child's first batch | a respawned child's announce withdrawn | ExaBGP queues an exit marker in the command queue (X-21); the fleet dispatches the withdrawals before reading the new child |
| R-6 | Three formats triple render cost on every UPDATE | `./le perf` fan-out benchmark | render only formats some script currently uses (already the rule) |
| R-7 | The main pin moves under the nightly tag | a green stage goes red with no ze change | pin a git commit of ExaBGP main (the zipapp's asset and tag move nightly, A-1); bump deliberately |
| R-8 | `group` held lines interact with `Net` and per-line answers | a group's answers out of order | the group end line carries the group's answer; held lines answer `done` when held (ExaBGP answers each held line? verify in `group_add_command`: it answers done) |
| R-9 | RTC origination emits a length ExaBGP and peers refuse | interop scenario red | boundary tests at 0, 32, 96 and refusals at 1, 31, 97 per RFC 4684 Section 4 |
| R-10 | Routing SR Policy through `parseConfigRoute` changes what config and API accept together | `spec-bgp-sr-policy-rfc-defects` tests | that spec owns `parseConfigRoute`'s rules; this spec adds a caller and no rule |
| R-11 | Vendored corpora bloat the repository (about 4.7 MB) | repo size | vendor gzipped, as ExaBGP stores `responses-*.json.gz`; or fetch from the pinned commit (A-10) |

## Blast Radius

| Question | Answer |
|----------|--------|
| What breaks if this is wrong? | ExaBGP helper scripts migrated to Ze: wrong answer shape stalls a script; a wrong exit policy leaves or drops routes; a wrong RTC/VPN/SR encoding reaches peers |
| How is it reverted? | per phase, one commit each; no config migration (the new leaves are optional) |
| Who else touches this path? | `spec-bgp-sr-policy-rfc-defects` (`parseConfigRoute`); the route-injection grammar work (`3f6bb1965e`, `textparse/section.go`); the session that wrote the untracked `api-sr-policy` case |

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| `le` exabgp stage, main pin | → | `exaBGPCommands` main-pin commands, cleared `PYTHONPATH`, named Python | `TestExaBGPMainPinCommands`, `TestExaBGPStageClearsPythonPath` |
| Script writes `announce route ...` first | → | script version state → v4, encoder v4 | `test/exabgp-compat/api/api-version-detect-v4.ci` |
| Script writes `peer * announce route ...` first | → | version v6, encoder v6 JSON | `test/exabgp-compat/api/api-version-detect-v6.ci` |
| Script writes `api version` / `system api version 4` | → | bridge-local answer | `test/exabgp-compat/api/api-version-command.ci` |
| `environment { exabgp { api { version 6 } } }` | → | forced version | `test/exabgp-compat/api/api-version-env.ci` |
| A refused line from a v4 / v6 script | → | version-aware `WriteError` | `test/exabgp-compat/api/api-error-v4.ci`, `api-error-v6.ci` |
| `daemon shutdown`, `session ack disable`, `rib flush out`, `peer create`, `peer delete` | → | v6 translations | `test/exabgp-compat/api/api-v6-vocabulary.ci` |
| `group start` ... `group end`, `group a ; b` | → | script group buffer | `test/exabgp-compat/api/api-group.ci`, `api-group-inline.ci` |
| Script exits | → | owner ledger + `on-exit` | `test/exabgp-compat/api/api-on-exit.ci` (ported), `api-on-exit-v4-keep.ci` |
| `announce ipv4 rtc ...` | → | `bridgeSAFI` rtc → `mp ipv4/rtc add` → rtc `EncodeNLRIHex` | `test/exabgp-compat/api/api-rtc.ci` |
| `announce ipv4 mpls-vpn ...` | → | `bridgeSAFI` mpls-vpn → `mp ipv4/mpls-vpn add` | `test/exabgp-compat/api/api-mpls-vpn.ci` |
| `announce ipv4 sr-policy ... preference ... segment-list ...` | → | `parseRegistryNLRISection` → `parseConfigRoute` | `test/exabgp-compat/api/api-sr-policy.ci`, `test/plugin/update-text-sr-policy-attributes.ci` |
| Refusal lines (item 5) | → | bridge refusals | `test/exabgp-compat/api/api-refusals-6.ci` |
| 4.2 / 5.0 recorded commands | → | `Translator.Line` + update text + encoder | `TestOldCommands50`, `TestOldCommands42` |
| 4.2 / 5.0 recorded events | → | v4 text / v4 JSON encoders | `TestOldResponses50`, `TestOldResponses42` |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | `./le` exabgp stage | runs the `encoding` and `api` populations under the 5.0.13 pin AND the ExaBGP-main pin (a recorded ExaBGP main git commit, installed by uv; A-1 change accepted by the main thread 2026-10-18); each `uv run` child has `PYTHONPATH` set empty and `--python` naming the interpreter; the report names both pins and the resolved ExaBGP version |
| AC-2 | the main-pin proof mode | each case under `test/exabgp-compat/api/` that this spec adds or changes passes with the ExaBGP-main pin as the daemon, so its expectation is 6.0's, before it is run against ze |
| AC-3 | `qa/api/api-on-exit.ci`, `qa/api/api-disable.ci` | ported to `test/exabgp-compat/api/` with their `.conf` and `.run` |
| AC-4 | `qa/old/4.2` and `qa/old/5.0` encoding tests | ported as populations `old-4.2` and `old-5.0` run by `le test exabgp`, with ExaBGP's ALLOWED reasons carried and Ze's own differences each named with a reason; green |
| AC-5 | first deciding line `announce route 10.0.0.0/24 next-hop 1.2.3.4` | the script is held to v4; a later `peer * announce ...` answers `error`; events reach it as v4 text (encoder text) or v4 JSON with `"exabgp": "4.0.1"` (encoder json) |
| AC-6 | first deciding line `peer * announce route ...` | held to v6; a later `announce route ...` answers `error`; events are v6 JSON (`"exabgp": "6.0.0"`) even with `encoder text`, and one warning names the process |
| AC-7 | a script whose first lines decide nothing (`help`, `# comment`, a typo) | stays undecided; reads v4 text when its `encoder` is text or absent, v6 JSON when json; its commands are accepted in both forms |
| AC-8 | `group start` from a v4 script | accepted (does not decide v6) |
| AC-9 | `api version` | answers `API version: auto (this helper: 4)` (text) or the JSON object of X-7, then `done`; `api version 6` answers `API version set to 6 (effective for helpers started afterwards)`; `api version 5` answers `error: API version must be auto, 4 or 6, got 5` then `error` |
| AC-10 | `environment { exabgp { api { version 4 } } }` | every script starts held to v4; `ze exabgp migrate` carries `exabgp.api.version` from an ExaBGP env file |
| AC-11 | OPEN event to a v4 JSON script | `capabilities` keyed by code (`"1"`, `"65"`) with no `code` member |
| AC-12 | OPEN event to a v6 JSON script | `capabilities` keyed by name, each with `"code"` inside |
| AC-13 | NOTIFICATION with an RFC 9003 Shutdown Communication to a v4 script | `data` is the hex of the readable text, `hexstring(message.v4_data)`: for a Shutdown Communication the decoded message without its length octet, for any other data the wire octets, each hexed once, and `message` carries the readable `Notification.text`. Corrected 2026-10-20 from "`data` is the readable message": the ExaBGP producer at the pin hexes the readable form, and Failure Routing makes the producer the source of truth (`TestV4NotificationDataReadable`) |
| AC-14 | a refused line from a v4 or undecided script with ack enabled | answers `error: <why>` then `error` |
| AC-15 | a refused line from a v6 script | answers `{"error": "<why>"}` then `error` |
| AC-16 | the same with ack disabled | neither line is written |
| AC-17 | v6 control words `daemon shutdown`, `session ack enable\|disable\|silence`, `rib flush out`, `rib clear in\|out`, `peer create <ip> ...`, `peer delete <selector>` | do what their v4 spelling does in Ze today; v6 words whose v4 spelling Ze does not translate are refused by name in both spellings, with the same reason |
| AC-18 | `group start`, two `announce route` lines, `group end` | the routes leave in ONE UPDATE; answers `group started`/`done`, then `group processed: 2 announced, 0 withdrawn` and `done`; a v6 `peer * announce` inside the block is held too |
| AC-19 | a group with one unparsable line | the others are applied; the end answers `..., 1 errors` and `error` |
| AC-20 | `group start` twice; `group end` with no start | `error: already in group block (nested groups not allowed)` then `error`; `error: not in group block` then `error` |
| AC-21 | `group announce route A next-hop N ; announce route B next-hop N` with no selector | both announced to every peer of the script; a `;` inside quotes or braces does not split |
| AC-22 | a v6 script announces a route and exits, `on-exit` unset | the route is withdrawn on the wire before the respawned child's first command; the log names `routes=withdraw by=API 6` |
| AC-23 | a v4 script announces and exits, `on-exit` unset | the route stays; log `routes=keep by=API 4` |
| AC-24 | `on-exit keep` / `on-exit withdraw` on any version | the leaf wins; `ze exabgp migrate` carries `on-exit` |
| AC-25 | script A announces X, script B announces X, A exits (withdraw) | X stays (B owns it); a withdraw of X from anywhere clears ownership |
| AC-26 | `announce ipv4 rtc origin-as 65000 route-target 65000:1 next-hop 192.0.2.1` and `announce ipv4 rtc default next-hop self` | MP_REACH_NLRI AFI 1 SAFI 132 with a 96-bit and a 0-bit prefix; the withdraw forms send MP_UNREACH_NLRI |
| AC-27 | `announce ipv4 mpls-vpn 10.0.0.0/24 next-hop 1.2.3.4 rd 100:100 label 10` | MP_REACH_NLRI AFI 1 SAFI 128, same bytes as `announce route 10.0.0.0/24 next-hop 1.2.3.4 rd 100:100 label 10` |
| AC-28 | `announce ipv4 sr-policy distinguisher 0 color 300 endpoint 10.0.0.3 next-hop 192.0.2.1 preference 300 segment-list ...` through the bridge, and the same through `send bgp ... update text mp ipv4/sr-policy add ...` | UPDATE carries the Tunnel Encapsulation attribute byte-identical to the config route's; a word after the sub-TLVs is refused |
| AC-29 | `peer * announce route 2001:db8::/32 ...` where no selected peer carries ipv6 unicast; same for EOR | `error` with the reason; with one carrying peer, sent to it and `done` |
| AC-30 | `peer 192.0.2.99 announce route ...` naming no session; `neighbor 192.0.2.99 ...` likewise | `error` (marker), nothing sent |
| AC-31 | `announce flow route discard`, `announce ipv4 flow discard` | refused with the reason |
| AC-32 | `announce route 10.0.0.0/24 next-hop 1.1.1.1 next-hop 2.2.2.2` | NEXT_HOP 1.1.1.1 on the wire; one warning names the dropped value; any repeated single-value attribute likewise |
| AC-33 | `announce route 10.0.0.0/24 next-hop 1.1.1.1 split /23` | the /24 is sent whole; one warning |
| AC-34 | `announce route 10.0.0.0/24 next-hop 1.1.1.1 path-information` | refused, reason names the missing identifier |
| AC-35 | `announce route 10.0.0.0/x next-hop 1.1.1.1` | refused, reason names the mask |
| AC-36 | `announce operational foo ...`, `announce operational` alone | refused, reason names the command; a known kind is refused naming that Ze does not send OPERATIONAL (OD-1) |
| AC-37 | every command in `qa/old/commands-5.0.json` and `commands-4.2.json` that the release accepted | through the bridge, ze announces and withdraws the routes the release did, decoded; each exception is an ALLOWED row with its reason |
| AC-38 | every event in `qa/old/responses-5.0.json.gz` and `responses-4.2.json.gz` | the v4 text line equals the recorded line; the v4 JSON line equals it parsed, ignoring time, host, pids, counter, and the version field reported once; each exception an ALLOWED row |
| AC-39 | an ExaBGP 4.2/5.0 config with an IPv4 route and an IPv6 next hop and no `nexthop` block | `ze exabgp migrate` adds the extended next hop entry and capability, with a warning naming the route |
| AC-40 | `bridge_batch.go` `Net` comment | states that `announce X` then `withdraw X` dispatches the withdrawal alone, matching the page table |
| AC-41 | `test/exabgp-compat/api/api-disable.conf` (neighbor `shutdown true`) and its script's `neighbor <ip> enable` / `disable "<msg>"` lines | `ze exabgp migrate` accepts `shutdown true` (today it refuses: "unknown field in neighbor: shutdown") and carries it to a peer that starts administratively down; the enable and disable lines bring the session up and send the Cease with the message; `api-disable.ci` green against ze |

## End-to-End User Stories

| # | User does | Path through system | Test proving it works |
|---|-----------|--------------------|-----------------------|
| 1 | runs a 5.0 helper written with `announce route` and `encoder text` | script → v4 detection → translation → wire; events as v4 text | `api-version-detect-v4.ci`, the whole 5.0.13 population, `old-5.0` |
| 2 | runs a 6.0 helper written with `peer *` | v6 detection → v6 JSON events, JSON error detail | `api-version-detect-v6.ci`, `api-error-v6.ci` |
| 3 | a DDoS detector crashes | exit → ledger → withdrawals on the wire | `api-on-exit.ci` |
| 4 | announces RTC / VPN / SR Policy from a helper | bridge family → update text → registry encoder → MP_REACH | `api-rtc.ci`, `api-mpls-vpn.ci`, `api-sr-policy.ci`, interop scenarios |
| 5 | an operator's maintainer bumps the ExaBGP pin | stage runs both pins | `TestExaBGPMainPinCommands` |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestExaBGPMainPinCommands` | `internal/le/test/functional/exabgp_test.go` | both pins appear, each with its Python and PYTHONPATH | |
| `TestExaBGPStageClearsPythonPath` | same | a shell `PYTHONPATH` never reaches the child | |
| `TestCommandAPIVersion` | `internal/exabgp/bridge/bridge_version_test.go` | X-1 table: every v6 root, `neighbor`, each v4 translation, undecided words | |
| `TestScriptHeldToDetectedVersion` | `internal/plugins/exabgp/bridgerun/script_test.go` | X-2, X-3, reset on respawn | |
| `TestGroupDispatchedWhateverTheVersion` | `bridge_version_test.go` | X-4 | |
| `TestEncoderForVersion` | `internal/exabgp/bridge/bridge_encoder_test.go` | X-5 matrix (auto/4/6 × text/json/absent) | |
| `TestV4JSONEnvelopeAndCapabilitiesByCode` | `internal/exabgp/bridge/bridge_event_test.go` | X-6, X-9 | |
| `TestV6JSONCapabilitiesByName` | same | X-10 | |
| `TestV4NotificationDataReadable` | same | X-12 | |
| `TestAPIVersionCommand` | `bridge_version_test.go` | X-7 answers | |
| `TestErrorAnswerV4`, `TestErrorAnswerV6`, `TestErrorAnswerAckDisabledSilent` | `internal/exabgp/bridge/bridge_ack_test.go` | X-13, X-14 | |
| `TestTranslateV6Vocabulary` | `internal/exabgp/bridge/bridge_control_test.go` | X-15 table row by row | |
| `TestGroupBlockHoldsUntilEnd`, `TestGroupAnswers`, `TestGroupInlineSplitsOutsideQuotes` | `internal/exabgp/bridge/bridge_group_test.go` (the group is shared by every runner, `ScriptLines.Batch`) | X-16 to X-20 | |
| `TestOnExitDefaultByVersion`, `TestOwnershipTransfers`, `TestExitWithdrawalRestoresConfigured`, `TestStandaloneExitWithdrawsOwnedRoutes` | `internal/exabgp/bridge/bridge_owner_test.go` (the ledger is shared by every runner) | X-21, X-22, A-8 | |
| `TestExitWithdrawsOwnedRoutes` | `internal/plugins/exabgp/bridgerun/owner_test.go` | X-21, X-22 | |
| `TestExitWithdrawRestoresConfiguredRoute`, `TestExitWithdrawRestoresConfiguredVPNAndLabeledRoutes`, `TestRestoreConfiguredWithoutConfigWithdrawsEverything` | `internal/component/bgp/reactor/peer_static_restore_test.go` | A-8 | |
| `TestRestoreConfiguredBeforeSectionMarksEveryGroup`, `TestRestoreConfiguredAfterSectionRefused` | `internal/component/bgp/plugins/cmd/update/update_text_restore_configured_test.go` | A-8 | |
| `TestBridgeFamilyRTC`, `TestBridgeFamilyMPLSVPN` | `internal/exabgp/bridge/bridge_test.go` | `bridgeSAFI` rows, translated text | |
| `TestRTCEncodeNLRIHex` | `internal/component/bgp/plugins/nlri/rtc/encode_test.go` | RFC 4684 Section 4 encoding and lengths | |
| `TestAPISRPolicyAttributesReachParseConfigRoute` | `internal/component/bgp/plugins/cmd/update/update_text_exabgp_test.go` | A-7, byte equality with config | |
| `TestFamilyNoSelectedPeerCarriesAnswersError` | `internal/exabgp/bridge/bridge_integration_test.go` | X-23, A-4 | |
| `TestSelectorMatchingNoPeerRefused` | `bridge_batch_test.go` | X-24 (replaces the `Unmatched` → done expectation) | |
| `TestRepeatedAttributeKeepsFirstAndWarns`, `TestSplitShorterThanPrefixSendsPrefix`, `TestPathInformationWithoutValueRefused`, `TestPrefixMaskNotNumberRefused`, `TestOperationalRefusedByKind` | `internal/exabgp/bridge/bridge_attribute_test.go` | X-26 to X-30 | |
| `TestOldCommands50`, `TestOldCommands42` | `internal/exabgp/bridge/old_commands_test.go` | X-35 commands | |
| `TestOldResponses50`, `TestOldResponses42` | `internal/exabgp/bridge/old_responses_test.go` | X-35 responses | |
| `TestMigrateOnExit`, `TestMigrateIPv4RouteIPv6NextHopAddsExtendedNextHop` | `internal/exabgp/migration/migrate_test.go` | AC-24, AC-39 | |
| `TestEnvSurvivingKey`, `TestParseExaBGPCIEnvOption` | `internal/exabgp/migration/env_test.go`, `internal/test/cli/cmd_exabgp_test.go` | AC-10 | No `TestMigrateOnExitAndAPIVersion`: AC-10's migration half is `TestEnvSurvivingKey` (`api.version` carried, 0 written auto) and `test/parse/cli-exabgp-migrate-env-api-version.ci`; `TestParseExaBGPCIEnvOption` covers the `option=env:` directive the functional case needs |

### Boundary Tests (numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| RTC prefix length | 0, 32-96 | 0, 32, 96 | 31 (and 1) | 97 |
| `api version` | auto, 4, 6 | 4, 6 | 3 | 5, 7 |
| path-information | 0-4294967295 | 4294967295 | N/A | 4294967296 |
| `split` length vs prefix /24 | 24-32 (IPv4) | 24, 32 | 23 (sent whole, warned) | 33 (refused) |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `api-version-detect-v4`, `api-version-detect-v6`, `api-version-command`, `api-version-env` | `test/exabgp-compat/api/` | item 1 | |
| `api-error-v4`, `api-error-v6` | same | item 2 | |
| `api-v6-vocabulary`, `api-group`, `api-group-inline`, `api-on-exit`, `api-on-exit-v4-keep`, `api-disable` | same | item 3 | |
| `api-rtc`, `api-mpls-vpn`, `api-sr-policy` | same | item 4 | |
| `api-refusals-6` | same | item 5 | |
| `old-4.2/*`, `old-5.0/*` | `test/exabgp-compat/old-4.2/`, `old-5.0/` | BC | |
| `update-text-sr-policy-attributes` | `test/plugin/` | SR Policy attributes through ze's own API | |

Each `test/exabgp-compat` case is proven three ways in its phase: green against the ExaBGP-main
daemon (AC-2), red against ze before the fix, green after. The red output is pasted in the spec's
closure section.

### Interop Tests (Scope: protocol)
| Scenario | Directory | Peer Daemon | What It Proves | Status |
|----------|-----------|-------------|----------------|--------|
| `bgp-exabgp-bridge-rtc-gobgp` | `test/interop/scenarios/` | GoBGP | an RTC route a helper announces is installed by GoBGP with its origin AS and route target, and the default RT | |
| `bgp-exabgp-bridge-vpnv4-frr` | `test/interop/scenarios/` | FRR | `announce ipv4 mpls-vpn` reaches FRR's VPN table with RD and label | |
| `bgp-exabgp-bridge-sr-policy-gobgp` | `test/interop/scenarios/` | GoBGP | an SR Policy with preference and segment lists from a helper is accepted by GoBGP (extends the shape of `bgp-sr-policy-gobgp`) | |

## Files to Modify
- `internal/le/test/functional/exabgp.go` - second pin, cleared `PYTHONPATH`, `--python`, populations `old-4.2`/`old-5.0`, proof mode
- `internal/test/cli/cmd_exabgp.go` - DUT selection (ze or the pinned ExaBGP daemon), new populations
- `internal/exabgp/bridge/bridge_command.go` - version-aware `Translator`, v6 control words, SR Policy text, `Unmatched` refused
- `internal/exabgp/bridge/bridge_family.go` - `rtc`, `mpls-vpn` keys in `bridgeSAFI`
- `internal/exabgp/bridge/bridge_ack.go` - version-aware error writer (replaces `WriteError`)
- `internal/exabgp/bridge/bridge_encoder.go`, `bridge_event.go`, `bridge_event_text.go` - three formats, envelope per version, OPEN capabilities, v4 NOTIFICATION data, v4 shapes of X-11
- `internal/exabgp/bridge/bridge_attribute.go`, `bridge_route_forms.go`, `bridge_flow.go` - item 5 rules
- `internal/exabgp/bridge/bridge_batch.go` - `Net` comment; per-line answer form
- `internal/exabgp/bridge/bridge_neighbor.go` - `peer create`, `peer delete`
- `internal/plugins/exabgp/bridgerun/script.go`, `fleet.go` - version state, group buffer, owner ledger, exit policy, render set
- `internal/plugins/exabgp/bridgeplugin/config.go`, `yang/ze-exabgp-bridge-conf.yang` - `on-exit` leaf, `encoder` default text
- `internal/component/config/apply_env.go` - `exabgp.api.version`
- `internal/exabgp/migration/env.go`, `migrate.go` - `api.version`, `on-exit`, extended next hop for IPv4 routes with IPv6 next hops
- `internal/component/bgp/plugins/cmd/update/update_text_nlri.go` - registry config-route parser for SR Policy sections
- `internal/component/bgp/plugins/nlri/rtc/register.go` - `InProcessNLRIEncoder`
- `docs/architecture/exabgp-bridge.md`, `docs/exabgp/exabgp-differences.md`, `docs/exabgp/exabgp-migration.md`, `docs/exabgp/exabgp-code-map.md`, `docs/architecture/config/environment.md`, `docs/functional-tests.md`, `rfc/short/rfc4684.md`
- `plan/journal/gate-verdict-depends-on-the-machine.md` - status cells of the 2026-10-17 and 2026-09-13 rows set to fixed with the commit

## Files to Create
- `internal/exabgp/bridge/bridge_version.go` - `commandAPIVersion`, the v6 roots, the v4 translation set
- `internal/plugins/exabgp/bridgerun/group.go`, `owner.go` - group buffer, owner ledger
- `internal/component/bgp/plugins/nlri/rtc/encode.go` - RTC text encoder
- `internal/exabgp/bridge/old_commands_test.go`, `old_responses_test.go` and `internal/exabgp/bridge/testdata/old/` (vendored corpora, A-10)
- `test/exabgp-compat/api/*.ci` named in Functional Tests, with `etc/*.conf` and `etc/run/*.run`
- `test/exabgp-compat/old-4.2/`, `test/exabgp-compat/old-5.0/`
- `test/plugin/update-text-sr-policy-attributes.ci`
- the three interop scenario directories

### Integration Checklist
| Integration Point | Applies? | File / reason |
|-------------------|----------|---------------|
| YANG schema (new RPCs/config) | Yes | `ze-exabgp-bridge-conf.yang`: `on-exit` (enumeration keep, withdraw; no default, absence meaning "by version"); `encoder` default text; environment `exabgp api version` in the environment YANG that holds `ack` |
| YANG validation constraints | Yes | `on-exit` enumeration; `version` enumeration auto, 4, 6 |
| YANG custom validators | N-A | enumerations suffice |
| CLI commands/flags | N-A | no ze CLI command; `api version` is ExaBGP API vocabulary answered by the bridge |
| CLI grammar (keyword before value) | N-A | no ze command added |
| Editor autocomplete | Yes | automatic for the two enumerations |
| Functional test for new RPC/API | Yes | `test/exabgp-compat/api/*` and `test/plugin/update-text-sr-policy-attributes.ci` |
| Pipe completeness | N-A | no ze CLI output |
| Env var registration | Yes | `exabgp.api.version` beside `exabgp.api.ack` in `apply_env.go`; registered where `exabgp.api.ack` is registered |
| Doctor check for runtime dependencies | N-A | the zipapp is a test-stage dependency of `le`, not a runtime dependency of ze; the stage reports it missing in its own failure text |
| Prometheus counters/metrics | N-A | no new observable state beyond logs |
| BGP family surface (new SAFI / capability / attribute) | Yes | `ai/patterns/bgp-family.md`: RTC gains text origination (encoder registration, bridge vocabulary, migration sync section 5b, decode already present); VPN and SR Policy are existing families gaining a bridge path; no new capability or attribute code |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature, or a feature's scope, evidence or level changed? | Yes | the ExaBGP bridge feature file under `features/` (`./le feature report` names its id and level) |
| 2 | Config syntax changed? | Yes | `docs/architecture/config/environment.md` (api version), `docs/exabgp/exabgp-migration.md` (on-exit) |
| 3 | CLI command added/changed? | N-A | no ze CLI command |
| 4 | API/RPC added/changed? | Yes | `docs/architecture/exabgp-bridge.md` (the ExaBGP API the bridge answers) |
| 5 | Plugin added/changed? | Yes | `docs/architecture/exabgp-bridge.md`; `docs/guide/plugins.md` if it lists bridge leaves |
| 6 | Has a user guide page? | Yes | `docs/exabgp/exabgp-migration.md` |
| 7 | Wire format changed? | N-A | no new wire format; RTC uses RFC 4684 as decoded today |
| 8 | Plugin SDK/protocol changed? | N-A | no SDK change |
| 9 | RFC behavior implemented, changed, or newly proven? | Yes | `rfc/short/rfc4684.md` (origination from text), its `docs/features/rfc-status.md` row |
| 10 | Test infrastructure changed? | Yes | `docs/functional-tests.md` (two pins, proof mode, old populations) |
| 11 | Affects daemon comparison? | Yes | `docs/exabgp/exabgp-comparison-report.md` API section |
| 12 | Internal architecture changed? | Yes | `docs/architecture/exabgp-bridge.md` (version state, group, owner ledger, three formats) |
| 13 | Route metadata keys added/changed? | N-A | none |
| 14 | Prometheus counters added/changed? | N-A | none |
| 15 | Registered plugin, event type, send type, command, capability, or inventory changed? | N-A | no registry inventory change beyond RTC's encoder, covered by row 9 |
| 16 | Any changed source file referenced by existing doc source anchors? | Yes | run `./le spec citation anchors spec plan/immediate/spec-exabgp-6-api-compatibility.md` at each phase; `docs/architecture/exabgp-bridge.md` and `docs/exabgp/exabgp-differences.md` are declared by `bridge.go`, `bridge_batch.go`, `bridge_ack.go` (`ai/CODE-TO-DOCS.md`). Also declared by files this spec changes, each re-read in the phase that edits the declaring file: `docs/architecture/api/commands.md` (update text sections: AFFECTED in Phase 5, the SR Policy `mp ipv4/sr-policy add` section now takes the attributes `parseConfigRoute` reads), `docs/architecture/config/syntax.md` (AFFECTED in Phase 2/4 if it lists the `exabgp bridge process` leaves or `environment exabgp api`), `docs/architecture/core-design.md` (expected unaffected: the engine gains no ExaBGP awareness; confirm in Phase 5), `docs/architecture/testing/ci-format.md` (AFFECTED in Phase 1 if the proof mode or the `old-*` populations add a `.ci` directive; unaffected otherwise, stated in the commit) |
| 17 | Existing docs show config/CLI/API examples for this area? | Yes | the `exabgp { bridge { ... } }` example in `docs/architecture/exabgp-bridge.md` (encoder default), the error example lines |

## Implementation Steps

Each phase is one commit, TDD (red pasted, then green), its page edits in the same commit.

1. **Phase 1: Wiring and the ExaBGP-main pin (FIRST)**
   - Tests: `TestExaBGPMainPinCommands`, `TestExaBGPStageClearsPythonPath`; proof-mode run of three existing cases (A-2, A-3)
   - Files: `exabgp.go`, `cmd_exabgp.go`, `bridge_batch.go` (comment), journal status cells; port `api-on-exit`, `api-disable`, `qa/old/4.2`, `qa/old/5.0`
   - Verify: both pins run; a shell `PYTHONPATH` is ignored; the ported cases are named in the report with their verdicts (the ones items 2 to 6 turn green are listed as the known reds those phases own, `ai/rules/pre-release.md`)
   - AC-4 state (2026-10-18, second package): ported and loading, NOT green. Runner: `exabgpConfigDir` (per-population `etc/`) and `option=allowed:<reason>` with `option=allowed-error:<text>` (the failure must carry the text, so a timeout or crash is a FAIL, never a SKIP; read only when the DUT is ze) judged by `judgeExaBGPAllowed` (`internal/test/cli/cmd_exabgp_old.go`). Proof mode now runs reload cases: `exabgp server` gets every config and the signal step goes to the ExaBGP pid (`deliverExaBGPDaemonSignals`). Cases: `test/exabgp-compat/old-5.0/` (72) and `old-4.2/` (34), each the branch's `.ci` and `.msg` joined, `etc/` copied unchanged from ExaBGP 685f772e4e `qa/old/<branch>/etc/exabgp`. Run against ze with `-t 60s`: old-5.0 51 pass, 1 allowed (conf-split, ExaBGP's reason verified against ze's migrate refusal), 12 fail, 8 timeout; old-4.2 9 pass, 20 fail, 5 timeout. Not yet in the `exabgp-test` stage (`exaBGPSuites` in `internal/le/test/functional/exabgp.go`): add them, once per run rather than once per pin (they import no ExaBGP), when green.
   - AC-4 state (2026-10-20, third package): (a) done, the mock reads 4.2 route steps (`matchExaBGPRoutes`, `exabgpLegacyRoutes` in `internal/le/interoplab/bgp/exabgp_old.go`, the 4.2 mock's decoded-route matching); (b) done, `option=notification:send-notification` (`notifyExaBGPUntilClosed`) and `option=open:send-unknown-message`, old-5.0 api-notification states `option=frames:none` as the api one does; the mock also reads `allowed:`/`allowed-error:` (conf-split was refused by it, `TestExaBGPCaseReadsEveryFixture` red); (c) allowed, ExaBGP 6.0 refuses the config too (proof run: "this '}' closes no section"), SKIP observed; ze's message journaled (`plan/journal/error-names-no-fix-command.md`). old-4.2 now 22/34 (`./le --name exa6 test exabgp old-4.2 --all`, the shared `bin/le` is stale for the mock, which is `le` itself). (d) partly: conf-hostname and conf-cap-software-version carry allowances (RFC 5492 Section 4 single Capabilities parameter; Software Version names ze) with ze's observed OPEN bytes, not yet observed as SKIP. Open: unknown-message allowance written uncommitted, unobserved (expects ze's `...0016030103FF` per RFC 4271 Section 6.1); old-4.2 reds api-check, api-eor, api-nexthop-self, api-reload, api-teardown (connection 2 gets ze's Cease 6/4 "Administrative Reset" communication, ExaBGP ALLOWED 4.2 api-teardown), api-vpls, api-vpnv4, conf-flow, conf-flow-redirect, conf-ipself6, conf-new-v4, conf-vpn; old-5.0 (d)/(e) list unchanged. NEW blocker for (e): parallel runs hit ze startup failing "stage stalled ... no plugin progress for 5s while waiting for stage 3" (bgp-rib, bgp-adj-rib-in, bgp-bmp at declare-capabilities), which turned 4 of 4 cases red under load on 2026-10-20; diagnose under `plan/journal/plugin-startup-barrier-deadlock.md` before reading any timeout as a wire difference. Stage: `exaBGPOldSuites` (old populations once, under the main pin) and `TestExaBGPStageRunsTheOldPopulationsOnce` are written in the tree, UNCOMMITTED, held until both populations are green
   - AC-4 remaining, each to diagnose to its producer and then fix ze or carry an `option=allowed:` with the verified reason: (a) the Go mock (`internal/le/interoplab/bgp/exabgp_case.go`) refuses the 4.2 peer's `N:announce:<prefix>` / `N:withdraw:<prefix>` steps ("unknown step kind"), 12 old-4.2 cases; (b) it refuses `option=open:send-unknown-message` (old-5.0 unknown-message) and old-5.0 api-notification's options; (c) `ze exabgp migrate` refuses old-5.0 conf-extended-attributes.conf ("line 36: expected keyword, got RBRACE"); (d) wire differences reported as unexpected messages: old-5.0 api-check, api-flow, api-multiple-api, api-rr, api-rr-rib, api-vpnv4, conf-cap-software-version, conf-flow, conf-hostname; old-4.2 api-check, api-teardown, api-vpnv4, conf-flow, conf-flow-redirect, conf-new-v4, conf-vpn (ExaBGP carries ALLOWED rows for api-flow, conf-flow, conf-flow-redirect, api-teardown; each still to be checked against ze's bytes, and ExaBGP's runner compares decoded routes where ze's mock compares bytes); (e) timeouts at 60s: old-5.0 api-eor, api-flow-merge, api-nexthop-self, api-rib, api-vpls, conf-ipself4, conf-ipself6, extended-nexthop; old-4.2 api-eor, api-nexthop-self, api-reload, api-vpls, conf-ipself6 (rerun at the 180s default first; extended-nexthop and conf-ipself6 log "unicast next hop has an unusable address family or role", which is AC-39's migration)
   - AC-4 state (2026-10-20, fourth package): committed 5cf57ad1aa (mock compares COMMUNITIES/EXT COMMUNITIES as sets, RFC 1997, RFC 4360 Section 2: 4.2 conf-new-v4, conf-vpn PASS) and ae49f963d0 (allowances, each SKIP observed: api-vpnv4 4.2+5.0 RFC 8277 Section 2.4 withdrawal label 0x800000; 4.2 conf-flow-redirect ExaBGP row, type 0x01; api-check 4.2+5.0 RFC 4271 Section 6.3 own-address NEXT_HOP; 4.2 api-reload ExaBGP row). 5.0 unknown-message allowance observed SKIP, still held uncommitted with the stage change. old-4.2 remaining: api-teardown (connection 2 gets the Cease before the third B1 frame: reconnect timing after `teardown 4`, NOT the ExaBGP row, ze's Cease bytes equal the expectation), conf-flow (ze defect: `redirect 1.2.3.4` migrates to next-hop 1.2.3.4 + redirect-to-nexthop-draft, but ze's flow MP_REACH carries next-hop length 0; expected 04 01020304; traffic-class 101 is encoded as recorded, so the ExaBGP row does not apply), timeouts api-eor, api-nexthop-self, api-vpls, conf-ipself6. old-5.0 fresh (old50-r3): 52/67, skip 5; remaining api-multiple-api (ze Cease 6/2 "Administrative Shutdown" early), api-rr (helper failure branch, 255.255.255.255), api-rr-rib, api-flow (ze sends an UPDATE with ORIGIN/AS_PATH/LOCAL_PREF and no NLRI where the MP_UNREACH flow withdrawal is expected: ze defect), conf-flow (same redirect next-hop defect), api-flow-merge, conf-ipself4, conf-ipself6, api-nexthop-self, api-vpls, api-eor, extended-nexthop, api-rib (timeouts).
   - AC-4 state (2026-10-20, fifth package): committed 4358372547: conf-flow defect FIXED (FlowSpec 0x0800 redirect/mirror-to-IP-next-hop now carries the MP_REACH next hop on all three origination rails, `message.MPReachCarriesNextHop`, draft-simpson-idr-flowspec-redirect-02 Section 3; `TestFlowSpecRedirectToNextHopCarriesTheNextHop`, 16 subtests red without the fix). old-5.0 conf-flow PASS; old-4.2 conf-flow SKIP observed on an allowance (4.2 sent the whole IPv6 address for `source ::1/128/120`; ze sends length-minus-offset bits, RFC 8956 Section 3.1, as 5.0 recorded). Two journal rows in `plan/journal/conflicting-feature-contracts.md` (forwarding strips the redirect next hop; traffic-class above 63 accepted). 5.0 api-flow re-diagnosed, NOT a plain ze defect: the first script line withdraws a flow never announced; ze withholds it (RFC 4271 Section 4.3) and `buildWithheldWithdrawUpdate` (reactor_api_batch.go) sends the attributes-only UPDATE on purpose, its comment citing ExaBGP 6.0 `test/exabgp-compat/api/api-flow.ci` frame 2; 5.0 recorded the MP_UNREACH instead. Open question for the next package: in the 6.0 api-flow.ci the 0025 attribute-only frame follows a `cmd:attributes origin igp local-preference 100` line, so verify the comment's claim against upstream `collection.py` before choosing between an allowance (5.0 vs 6.0 difference) and a fix. Remaining reds otherwise unchanged from the fourth package.
   - AC-4 state (2026-10-20, sixth package): committed febfd66217: api-flow RESOLVED as a ze fix. The comment's claim was false at the pin: upstream 685f772e4 `qa/api/api-flow.ci` has no attributes-only frame (frame 2 is the MP_UNREACH withdrawal) and `UpdateCollection._mp_family_messages` yields nothing for a withdrawal while `include_withdraw` is False; ze's vendored 6.0 copy (8d98caf8fe, 2026-09-05) predated the pin. `buildWithheldWithdrawUpdate` deleted: a withheld withdrawal writes nothing for every family; 6.0 api-flow.ci synced to the pin (3 lines). old-5.0 api-flow PASS; 6.0 api-flow PASS; old-4.2 api-flow SKIP on an observed allowance (4.2's attributes-only frame). The mock's unexpected-message error now ends `(first: <hex>)` like its never-arrived error, so a missing-frame allowance holds either way. New lead for the api-flow-merge timeouts (6.0 red too, and 5.0): the bridge refuses `interface-set` extended communities ("unknown extended community type: interface-set", draft-ietf-idr-flowspec-interfaceset): an absent feature, owner scope question before implementing. Item 2 reds not yet diagnosed this package: 4.2 api-teardown, timeouts (api-eor, api-nexthop-self, api-vpls, conf-ipself4/6, extended-nexthop, api-rib), 5.0 api-multiple-api, api-rr, api-rr-rib. Stage change and 5.0 `unknown-message.ci` allowance still held uncommitted.
   - AC-4 state (2026-10-20, seventh package, second agent): committed 40faec5c95 (migrate maps a bare IPv6 host route's IPv4 next hop; `extended-nexthop.conf` synced to the pin; 6.0 encoding extended-nexthop PASS), 2c46dd325c (allowances observed SKIP at -t 180s: old-5.0 extended-nexthop, conf-ipself4, conf-ipself6; old-4.2 conf-ipself6), 7fc1f19f28 (bridge JSON in ExaBGP's spacing, `MarshalExabgpJSON`: the upstream api-rr script matches `'"nlri": "..."'`), e3876abb19 (bridge translates `announce route-refresh <afi> <safi>` to `request peer <sel> refresh <family>`; it was read as `announce route` "-refresh"). api-rr PASS under 6.0 and old-5.0. api-rr-rib (6.0 and 5.0) and api-rib DIAGNOSED, not fixed: the bridge maps ExaBGP `clear adj-rib out` (upstream `neighbor_rib_out_withdraw`: withdraw every Adj-RIB-Out route) to ze's `clear bgp rib out`, a replay, and `flush adj-rib out` (upstream `neighbor_rib_resend`, BoRR/EoRR under RFC 7313) to `request peer flush`, a forward-pool drain; ze has no withdraw-all-Adj-RIB-Out command (journal row in `plan/journal/delegation-hands-the-callee-a-different-argv.md`). Not reached this package: 4.2 api-eor, 4.2 api-nexthop-self, 5.0 api-multiple-api; full old-4.2/old-5.0 runs; stage change and old-5.0 `unknown-message.ci` still held uncommitted.
   - AC-4 state (2026-10-20, eighth package): f4df6c5710 (bridge translates `announce`/`withdraw vpls`; api-vpls PASS in 6.0, old-5.0, old-4.2), 79060914b7 (update text accepts a list closed on its last value), allowances observed SKIP for old-4.2/old-5.0 api-nexthop-self and old-5.0 api-multiple-api (RFC 4271 Section 5.1.3: self is the loopback peer's own address), 63c789a62b (stage change: old populations run once under the main pin; old-5.0 unknown-message allowance). Full runs: old-4.2 25 pass / 8 skip, red api-eor; old-5.0 57 pass / 12 skip, red api-eor, api-rib, api-rr-rib; 6.0 encoding 40/42, red conf-flow (pin 685f772e4 encodes `redirect 1.2.3.4` as ext-community 0x010C with next-hop length 0; 5.0 sends 0x0800 plus next hop, which ze sends since 4358372547: contracts conflict, owner decision) and conf-watchdog (seq 2 frame never arrives, not diagnosed); 6.0 api 35/44, red api-eor, api-rib, api-rr-rib, api-disable (AC-41), api-on-exit (AC-22..24), api-sr-policy (AC-28, sr-policy encoder refuses `preference`), api-health (bridge refuses `ping`), api-peer-lifecycle (runtime-created peer has no exabgp-bridge attach), api-healthcheck-module (script imports exabgp.application.healthcheck, missing outside the stage's uv env). api-eor diagnosed: `claimInitialSyncEOR` in AnnounceEOR drops every scripted EOR after the initial-sync one with no log; its "RFC 4724 Section 2: one End-of-RIB per family per session" citation is not in Section 2; ExaBGP sends every `announce eor` (owner decision pending). Journal 0da4e240c1: bare `announce eor` means ipv4/unicast in ExaBGP but `convertEOR` expands over declared families. Owner decisions pending: clear/flush adj-rib-out (api-rib, api-rr-rib), api-eor, 6.0 conf-flow contract.
   - AC-4 state (2026-10-20, ninth package): 8dfa2fcfd5 (`create neighbor ... api <p>` attaches `exabgp-bridge`, `bridge.PluginName`; api-peer-lifecycle `.ci` JSON line corrected from mock defaults; PASS in 6.0). Owed on it: a script with a feed list does not gain the created peer in its scope (ExaBGP adds it to `api['processes']` with no grant). api-disable (AC-41) stopped: ze has no config leaf holding a peer administratively down and no peer enable/disable command, so the migration alone cannot meet the AC; owner question on the ze surface (handover 03, ninth package). api-on-exit (AC-22..24) not started: needs X-1/X-2 version detection first; design in handover 03, ninth package.
   - AC-4 state (2026-10-20, tenth package, exa6c): 1177ffc136 (bridge answers `ping`/`session ping` [text] with ExaBGP 6.0's bytes, reactor.py ping; 6.0 api-health passes ping and sequence 2, red on `status`, which the bridge refuses: its answer lists every peer with its FSM state and the bridge reads none from ze). af18a709d8 (AC-28: `update text` hands an sr-policy `add` section to the registered config-route parser via `NLRISectionCarriesAttributes`/`registry.SectionRouteParserFor`; attributes merged in type-code order; `TestAPISRPolicyAttributesReachParseConfigRoute` byte-equal to the fixture and the config parser). 6.0 api-sr-policy: announce and EOR frames match; red on the withdrawal, ze sends `800F10...` MP_UNREACH followed by ORIGIN, AS_PATH, LOCAL_PREF (`40010100 400200 40050400000064`) where ExaBGP sends MP_UNREACH alone; the bridge sends `update text mp ipv4/sr-policy del ...` with no attribute, so the attributes are added below the API (not diagnosed). conf-watchdog not started. AC-28's `test/plugin/update-text-sr-policy-attributes.ci` and the GoBGP interop scenario not written.
   - AC-4 state (2026-10-20, eleventh package): 76b51c8946 (api-sr-policy fixture corrected: the ExaBGP 6.0 pin 685f772e4 itself sends the sr-policy withdrawal as MP_UNREACH plus ORIGIN/AS_PATH/LOCAL_PREF, `collection.py` `_attribute_sets` sends it bare only when every MP withdrawal is unicast/multicast; ze's producer was right; 6.0 api-sr-policy PASS; under `--dut exabgp` the pin never sent the announce frame, so that frame is unverified against ExaBGP). 9dea34f1b9 (`test/plugin/update-text-sr-policy-attributes.ci`, PASS, red with `NLRISectionCarriesAttributes=false`). GoBGP interop scenario `bgp-exabgp-bridge-sr-policy-gobgp` written, uncommitted, first run not read. conf-watchdog and api-health `status` not started.
   - AC-4 state (2026-10-20, twelfth package): nothing green to commit; three uncommitted changes in flight (sr-policy GoBGP scenario red at assertion 2, watchdog helper loop, api-sr-policy script pause), api-health `status` designed, not built. Details and next steps: handover 03, "AC-4 twelfth package".
   - AC-4 state (2026-10-20, fifteenth package): `status` / `daemon status` answered with the pinned reactor.py bytes (text, or JSON under API 6 or a trailing `json`), peers read from `show bgp peer * detail` (new `fsm-state` field, 01fc0f642b), keyed by ExaBGP `Neighbor.name()`, ordered by address (ExaBGP uses configuration order, which ze's map answer does not carry). `status` is plugin-runner-only: the exabgp-bridge plugin (bridgerun) answers it; the standalone runner `ze exabgp plugin` reads no dispatch payload, so it refuses `status` by name and answers `error` (`errStatusStandalone`). 6.0 api-health PASS under the stage's uv line; red (five "status missing UUID" respawns, FAIL) with the bridgerun status answer disabled. `TestFleetAnswersStatusFromZesPeerList` covers the runner path.
2. **Phase 2: API version detection and encoders** - X-1 to X-12; AC-5 to AC-13
   - Phase 2 state (2026-10-20, first package): X-1 to X-8 done, in the commit that carries this line. `bridge.CommandAPIVersion` (`internal/exabgp/bridge/bridge_version.go`) is `command_api_version` row by row (`TestCommandAPIVersion`: every V6_ROOTS word, every V4_SIMPLE_TRANSLATIONS key, every `translate_v4_to_v6` branch, the undeciding words); `APIVersion.Admits` is `dispatch_for` (held v4 refuses a v6 line, held v6 a v4 one, `group` decides nothing); the refusal wraps `bridge.ErrUnknownCommand`, which `AnswerBatch` answers with the current one-line `error <why>` (Phase 3 reshapes it). Version state per script in `bridgerun` (`version.go`: `detectAPIVersion` before translation, `resetAPIVersion` at each fork, `adopt` logs ExaBGP's two warnings), fleet setting `exabgp.api.version` read at `Fleet.Start` (refused value stops the bridge), `api version [<v>]` / `system api version` answered by the runner byte for byte (`APIVersionRequest.Answer`; a trailing `json` is the parsed value, as ExaBGP passes it unchanged). `FormatFor` is the X-5 matrix; the fan-out renders per `bridge.Format` (three slots); v4 JSON envelope `4.0.1`. YANG `encoder` default is now text (`internal_test.go` quiet-encoder row changed with it); env leaf `exabgp api version` (ze-hub-conf.yang), `apply_env.go` row, env registration, migration of `api.version` (0 written auto). Cases `api-version-detect-v4`, `api-version-detect-v6`, `api-version-command` green against ExaBGP main (proof mode) and ze; red against ze with detection disabled (two FAIL, one timeout). NOT done: X-9 to X-12 (AC-11 to AC-13: v4 JSON open capabilities by code, v6 by name, X-11 5.x shapes, v4 NOTIFICATION data), so a v4 JSON line differs from v6 by its envelope only; `api-version-env.ci` (AC-10 functional) not written; the standalone runner `Bridge` in `bridge.go` (line ~718, `Translator{...}.Line`) has no version state and stays 6.0 JSON.
   - Phase 2 state (2026-10-20, second package): X-9, X-10, X-12 done in 6bc5593f8f, X-11 in fbec2a2061. OPEN: ze's event already carries code, name and value per capability, so `ReadEvent` carries nothing new; `Event.openJSON` (`bridge_event_open.go`) rebuilds ExaBGP's body per code (`capability/*.py` `json()`), keyed by code without `code` for v4, by `CapabilityCode.name` with `code` for v6; `asn` is the two-octet field (AS_TRANS above 65535). Expected bodies are the pinned ExaBGP's own `exabgp decode -o` output for a 13-capability OPEN (`TestV6JSONCapabilitiesByName`, `TestV4JSONEnvelopeAndCapabilitiesByCode`). NOTIFICATION (`bridge_event_notification.go`): `data` hexstring of the wire (v6) or of `v4_data` (v4 JSON and text), `message` = `Notification.text`; seven Data fields checked against `Notification.make_notification` at the pin (`TestV4NotificationDataReadable`). AC-13's wording ("`data` is the readable message") differs from the producer: ExaBGP hexes the readable form (`hexstring(message.v4_data)`); the code follows the producer, the AC text is owed a correction by the owner. X-11: v4 JSON label `[[100]]`, flow `string` + ` next-hop`, v4 text labeled/VPN route `prefix label N next-hop X rd RD` (`labeledText`), one-cluster list already bracketed. Also fixed: text OPEN line read `hold-time` flat (ze nests it under `timer`), `json-format.md` example corrected. No `.ci` case exercises an OPEN or NOTIFICATION reaching a script, so no proof-mode run applies to X-9/X-10/X-12. NOT done: `api-version-env.ci` (AC-10 functional), AC-6 "one warning names the process" proof, standalone runner `Bridge` (decision below in handover 03).
   - Phase 2 state (2026-10-20, third package): AC-13 wording corrected to the producer (960c6dd0ca). Standalone runner held to the detected version in 07941f7900: `bridgerun/version.go` deleted, its state is `bridge.ScriptVersion` (`bridge_script_version.go`), read from `exabgp.api.version` at Start, JSON encoder (`TestStandaloneRunnerHeldToDetectedVersion`). AC-6 proof `TestV6TextScriptWarnedOnceByName` (red with adopt's v6 condition inverted). api-healthcheck-module red is environmental (the run script imports `exabgp`; no exabgp on the shell's python3), green under the stage's uv line; journal row in `gate-verdict-depends-on-the-machine.md`. NOT done: AC-10 `api-version-env.ci` (needs an `option=env:<key>=<value>` directive in the exabgp `.ci` parser, fed through `exaBGPClientEnv` to both DUTs); the bridge reads only the dotted env names, while ExaBGP also accepts `exabgp_api_version`; population runs (encoding, old-4.2, old-5.0, api) under the uv line.
   - Phase 2 state (2026-10-20, fourth package): the bridge reads `exabgp.api.version` and `exabgp.api.ack` through `env.Get`, so the underscore spelling reaches it, and a dotted spelling wins over an undotted one for every key (3511f9ee1b). AC-10 functional done in the commit that carries this line: `option=env:<key>=<value>` in `parseExaBGPCI`, appended last by `exaBGPClientEnv` for both DUTs (the mock ignores it); case `api-version-env` green against the pinned ExaBGP (`--dut exabgp`) and ze, red against ze with `APIVersionFromEnv` overlaid to ignore the setting (the v6 first line answered `done`, 2.2.2.2 reached the peer); parse case `cli-exabgp-migrate-env-api-version.ci` green, red with the `api.version` arm of `migration/env.go` overlaid away (stdout lacked `version 4;`). Population results: handover 03, "Phase 2 fifth package".
3. **Phase 3: Error answers** - X-13, X-14; AC-14 to AC-16
   - Phase 3 state (2026-10-20): X-13, X-14, AC-14 to AC-16 done in the commit that carries this line. `AckMode.WriteError(w, version, reason)` (`bridge_ack.go`) is `answer_error_sync` + `_error_detail`: `{"error": <json.dumps string>}` for API 6, `error: <why>` for 4 or undecided, then `error`; both silent with ack disabled; the one-line `error <why>` is gone. `BatchLine.Version` (set by both runners at `Detect`) picks the form per line, as ExaBGP answers a line before reading the next. `jsonString` now quotes as `json.dumps` (ensure_ascii lowercase `\u`, surrogate pairs, `<>&` literal), which the X-7 JSON answers share. A translator-refused line, answered nothing before, now gets the reason and `error`, as the pin's `announce_route` does ("Could not parse route"). Unknown command and no-peer get Ze's reason too (Key Design Decisions row; `docs/exabgp/exabgp-differences.md`). Tests `TestErrorAnswerV4`, `TestErrorAnswerV6`, `TestErrorAnswerAckDisabledSilent`, `TestAnswerBatchRefusalInTheLineVersion` (red with `WriteError` overlaid to the old one-line form, except the silent test, which the old code also met). Cases `api-error-answer-v4` (AC-14, AC-16 via `disable-ack`) and `api-error-answer-v6` (AC-15) green against the pin (`--dut exabgp`) and ze, red against ze with the overlay (deadline exceeded: the script waits for the marker). api population against ze: 45/50, reds api-disable, api-eor, api-on-exit, api-rib, api-rr-rib (the five known reds of the Phase 2 fifth package). Not run: encoding, old-4.2, old-5.0 populations; lint.
4. **Phase 4: v6 vocabulary, group, on-exit, exit withdrawal** - X-15 to X-22; AC-17 to AC-25
   - Phase 4 state (2026-10-20, first package): X-15 (AC-17) in e9ac5ea736: `v6ControlSpellings`/`v4ControlSpelling` (`bridge_v6_control.go`) read a v6 control line in its v4 spelling in `Translator.Line`; `TestTranslateV6Vocabulary`; case `api-v6-vocabulary` green on the pin and ze, red with the overlay. X-21/X-22 (AC-22, AC-24 config and migration, AC-25 ledger) in the commit that carries this line: `Command.Withdrawal` written by each route builder; `ownerLedger` and `exitPolicy` (`bridgerun/owner.go`); `script.exited` runs between reap and respawn; YANG `on-exit` (keep, withdraw, no default); `parseOnExit`; migration `exabgpOnExit`. Tests `TestOnExitDefaultByVersion`, `TestOwnershipTransfers`, `TestExitWithdrawsOwnedRoutes` (red with `exited` overlaid to return), `TestParseScriptsReadsOnExit`, `TestMigrateOnExit`; `api-on-exit` green on the pin and ze, red with the overlay. NOT done: X-16 to X-20 group (AC-18 to AC-21), `api-on-exit-v4-keep` case (AC-23 functional), A-8 (`TestExitWithdrawRestoresConfiguredRoute`, engine-level), the standalone runner `Bridge` has no exit policy, the ledger keys on the selector text where ExaBGP's owner is per peer.
   - Phase 4 state (2026-10-20, second package): X-16 to X-20 (AC-18 to AC-21) in the commit that carries this line: `bridge.Group` (`bridge_group.go`: `Group.read` holds a v4 `announce`/`withdraw` and an admitted v6 `peer <sel> announce|withdraw <kind>` while open, and answers start, end, nested, stray, empty and over-limit as `command/group.py`; `SplitCommands` ports `lexer.split_commands`; `Translator.group` applies `attributes` sharing through `mergeShared`; `groupApply.answer`). `bridge.ScriptLines.Batch` (`bridge_lines.go`) is now the ONE batch path both runners call (`script.batch` through `scriptRunner`, `Bridge.dispatchBatch` through `muxRunner`). A closed group's route commands that differ only in their one-word NLRI are dispatched as one command (`mergeHead`), so they leave in one UPDATE. Tests `TestGroupBlockHoldsUntilEnd`, `TestGroupAnswers`, `TestGroupInlineSplitsOutsideQuotes`; cases `api-group`, `api-group-inline` green on the pin (`--dut exabgp`) and on ze; `api-group` red on ze with `Group.read` overlaid to pass every line (the script's `group start` is refused) and with the merge overlaid off (1.1.1.1/32 leaves alone). Deviation: shared attributes go before the `nlri` of the attributes form and after the route otherwise, so a flow route given shared attributes gets what the flow parser makes of a trailing attribute (not exercised at the pin). AC-23 functional: case `api-on-exit-v4-keep` (an API 4 one-shot announces 1.1.1.1/32 and exits; a sibling announces 2.2.2.2/32 three seconds later) green on the pin and on ze, red on ze with `exitPolicy` overlaid to withdraw when `on-exit` is unset (the withdrawal of 1.1.1.1/32 arrives before the marker). NOT done: A-8 (`TestExitWithdrawRestoresConfiguredRoute`), the standalone runner's exit policy (`muxRunner.Dispatched` records nothing; the ledger and `exitPolicy` live in `bridgerun` and would move to `bridge` to be shared), the api population rerun.
   - Phase 4 state (2026-10-20, third package): A-8 was broken and is fixed for prefix routes in the commit that carries this line. Case `api-on-exit-configured` (neighbor static 1.1.1.1/32 next-hop 10.10.10.10; an API 6 script announces 1.1.1.1/32 next-hop 11.11.11.11 and exits) is green on the pin (`--dut exabgp`: the configured route is announced again, no withdrawal) and was red on ze before the fix (the mock received the withdrawal `...001C02000520010101010000`). Fix: update-text keyword `restore-configured` (`kwRestoreConfigured`, carried as `NLRIGroup`/`NLRIBatch.RestoreConfigured`); `WithdrawNLRIBatch` asks `Peer.restoreConfigured` (`reactor/peer_static_restore.go`), which takes each withdrawn NLRI the peer's static config holds out of the withdrawal and re-sends the configured route through `sendStaticRoutes`; the ledger stores each exit withdrawal with the keyword (`restoringWithdrawal`). Tests `TestExitWithdrawRestoresConfiguredRoute`, `TestRestoreConfiguredWithoutConfigWithdrawsEverything` (reactor), `TestExitWithdrawalRestoresConfigured` (bridgerun); `api-on-exit`, `api-on-exit-v4-keep`, `api-on-exit-configured` green on ze after it. NOT done: a static route with an RD or a label stack is not matched (`staticRouteNamesNLRI`), so its prefix is still withdrawn on exit; the standalone runner's exit policy; the api population rerun.
   - Phase 4 state (2026-10-20, fourth package): d54ccff4ad: `TestRestoreConfiguredBeforeSectionMarksEveryGroup`, `TestRestoreConfiguredAfterSectionRefused` (red with the refusal overlaid away); the refusal text named the refused `nlri <family>` form and now names `nlri del <prefix>` and `mp <family> del <prefix>`. 159d3765a2: A-8 for routes with an RD or a label stack: `staticRouteNamesNLRI` compares the withdrawn NLRI's route key (`nlrisplit.GetPrefixKey` read as a withdrawal, RFC 8277 Section 2.4 Compatibility field ignored, RD kept) with `staticRouteKey`; `TestExitWithdrawRestoresConfiguredVPNAndLabeledRoutes` red against the old matcher (4 left to withdraw, want 2); whole reactor package green. 8f3048d439: `OwnerLedger`, `exitPolicy`, `OnExit`, `restoringWithdrawal` moved into `internal/exabgp/bridge/bridge_owner.go` (bridgerun copy deleted); `OwnerLedger.Exited` is the one settlement both runners call; the standalone `Bridge` records ownership in `muxRunner.Dispatched` and settles its child's exit in `Run` after the last batch (no process block: the version decides); `TestStandaloneExitWithdrawsOwnedRoutes` red with `Dispatched` recording nothing. api population rerun once at the pin with `bin/le-exa10`: 51/55, reds api-disable (migration refuses `shutdown`, the AC-41 owner question), api-eor (scripted EOR never arrives at sequence 3, `claimInitialSyncEOR`), api-rib and api-rr-rib (`clear`/`flush adj-rib out` mapping): the four known reds, each first error matching its recorded cause; api-on-exit now green. Phase 4's rows X-15 to X-22 and AC-17 to AC-25 are met. Open beside them, recorded in the first package and not in any AC: the ledger keys a route on the selector text where ExaBGP's owner is per peer.
5. **Phase 5: Families** - rtc encoder and bridge row, mpls-vpn row, SR Policy through `parseConfigRoute`; adopt the untracked `api-sr-policy` case (A-9); interop scenarios; AC-26 to AC-28
   - Phase 5 state (2026-10-20, exa11 and exa12): AC-26 and AC-27 met. 4d52d72875: RTC encoder (`nlri/rtc/encode.go`, RFC 4684 Section 4), update-text rtc section, bridge `rtc` and `mpls-vpn` rows of `bridgeSAFI`, case `api-rtc` (green on the pin and ze). 7cec4fa82c: interop `bgp-exabgp-bridge-rtc-gobgp` (GoBGP holds `65001:65001:100`) and `bgp-exabgp-bridge-vpnv4-frr` (FRR holds RD 65001:100, remote label 100, session up despite ze's NEXT_HOP beside MP_REACH), each green and red at assertion 2 with its `bridgeSAFI` row overlaid away; GoBGP v3.31.0 panics on a 0-bit default RTC membership (nil Route Target in `BgpServer.propagateUpdate`), so that scenario sends none. AC-28 met (af18a709d8, 76b51c8946, 65651c27e0); "a word after the sub-TLVs is refused" re-verified 2026-10-20: `TestAPISRPolicyAttributesReachParseConfigRoute` green, red ("An error is expected but got nil") with the srpolicy `parseConfigRoute` default-arm refusal overlaid to skip the word; the bridge passes the words to that parser unchanged. `docs/architecture/wire/nlri.md` (declared by `nlri/rtc/encode.go` and the srpolicy files) corrected: its "SR-Policy in `update text`" section still sent `add` to `EncodeNLRIHex` and refused every word but the three NLRI fields, false since af18a709d8; an "RTC in `update text`" section added. Open: the uncommitted `api-mpls-vpn` case pins ze's NEXT_HOP beside MP_REACH where the pin sends none (RFC 4760 Section 3 "SHOULD NOT"), red under `--dut exabgp`; owner decision owed on which contract wins.
6. **Phase 6: Refusals and keep-first rules** - X-23 to X-30; AC-29 to AC-36
   - Phase 6 state (2026-10-20, Mac): X-23 to X-30 done for routes; AC-29's EOR half NOT checked (`announce eor` belongs to the concurrent adj-rib-out/eor package). Bridge code for X-24, X-26..X-30 in fa2effd108. Unit tests `TestSelectorMatchingNoPeerRefused`, `TestRepeatedAttributeKeepsFirstAndWarns`, `TestSplitShorterThanPrefixSendsPrefix`, `TestPathInformationWithoutValueRefused`, `TestPrefixMaskNotNumberRefused`, `TestOperationalRefusedByKind`, each red under a `go -overlay` breaking its producer. X-23: ze's update-text answers status `warning` with `{"message": ...}` when no selected peer NEGOTIATED the family (ExaBGP tests the CONFIGURED one, `carrying`); the record wire drops the status, so `RefuseUnsentRoute` (`bridge_dispatch_answer.go`) reads the document in both runners, and the standalone runner releases a dispatch at the terminator holding the first record (a terminator message is now a failure; it was answered done). `TestFamilyNoSelectedPeerCarriesAnswersError`. X-25 was already refused. Cases `api-refusals-v4` (selector no peer, family not carried, flow no match, path-information, mask, operational twice) and `api-keep-first-v4` green on the pin and on ze; red on ze with the X-23, keep-first and short-split overlays (`GOFLAGS=-overlay=`). OD-1 recorded in `docs/exabgp/exabgp-differences.md`.
7. **Phase 7: 4.2 and 5.0 corpora and migration** - X-35, X-37; AC-37 to AC-39, AC-41 (the `api-disable` red: `ze exabgp migrate` refuses `shutdown true`, a migration defect, main-thread decision 2026-10-18)
8. **Phase 8: Docs consolidation check** - every page named above re-read against the code; `docs/exabgp/exabgp-differences.md` lists only differences that remain

### Critical Review Checklist
| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | Every AC-N has an implementation at file:line and a test |
| Feature completeness | Every user story has a working path, no broken links |
| Correctness | each answer string byte-equal to the ExaBGP producer quoted in its X row; detection identical to `command_api_version` for every row of `V4_SIMPLE_TRANSLATIONS` and `translate_v4_to_v6` |
| Naming | YANG leaves `on-exit` and `version` spelled as ExaBGP spells them; JSON keys as ExaBGP writes them (external contract) |
| Data flow | version, group and ownership state live in `bridgerun`; the engine learns nothing ExaBGP-specific |
| Rule: no-layering | the one-line `error <msg>` writer is deleted, not kept beside the versioned one; `Unmatched` → done is deleted |
| Rule: rfc-compliance | RTC encoder quotes RFC 4684 Section 4; callers name it |

### Deliverables Checklist
| Deliverable | Verification method |
|-------------|---------------------|
| main pin in the stage | `go test ./internal/le/test/functional/ -run TestExaBGPMainPin` |
| every new case green on ExaBGP main and on ze | `./le test exabgp api <case>` under each pin, output pasted |
| corpora tests | `go test ./internal/exabgp/bridge/ -run 'TestOld'` |
| interop | `./le test integration <scenario>` for the three scenarios |

### Security Review Checklist
| Check | What to look for |
|-------|-----------------|
| Input validation | inline group split ends only outside quotes and braces (X-19), so a peer-chosen string cannot inject a command |
| Injection | the error detail is newline-stripped and length-bounded (`sanitizeErrorMessage`) in both forms; the JSON form is marshalled, never concatenated |
| Resource exhaustion | owner ledger bounded by routes held (R-4); group buffer bounded by the batch line limit |
| Fail open | a selector matching no peer is refused, never widened to every peer (X-24) |

### Failure Routing

| Failure | Route To |
|---------|----------|
| Compilation error | Fix in the phase that introduced it |
| Test fails for the wrong reason | Fix the test assertion or setup |
| Test fails on behavior mismatch | Re-read the ExaBGP producer in the X row. If misunderstood → RESEARCH |
| A case fails against the ExaBGP-main daemon | the expectation is wrong: rewrite it from 6.0's output, never weaken it |
| Lint failure | Fix inline. If architectural → DESIGN |
| Functional test fails | Check the AC: wrong AC → DESIGN, correct AC → IMPLEMENT |
| 3 fix attempts failed | STOP. Report all 3 approaches. Ask the user |

## Design Insights

- ExaBGP's own answer to "is this a 5.x script" is behavioural (the first deciding command), never configured, and Ze copies the rule rather than adding a Ze-side leaf per process.
- The 4.2/5.0 corpora are generated by the releases themselves, so they measure the old contract without anyone hand-writing expectations; Ze adopts them instead of writing its own.

## Key Design Decisions

| Decision | Alternatives Considered | Rationale |
|----------|------------------------|-----------|
| SR Policy attributes through the API: `parseRegistryNLRISection` calls the family's registered `InProcessConfigRouteParser` (`registry.ConfigRouteParserByFamily`) when the family declares one, and merges its `PluginRoute.Attrs`; for SR Policy only in this spec, the families that register both keep `EncodeNLRIHex` until their own sections need attributes | (a) extend `parseSRPolicyNLRIArgs` / the text parser with the tunnel-encap grammar; (b) a new registry hook for "section with attributes" | (a) is a second parser for one grammar, banned by `ai/rules/no-layering.md` and by the skeleton's own goal; (b) adds a registration field when one already exists. The parser that config and `ze bgp encode` share becomes the API's too, so `spec-bgp-sr-policy-rfc-defects`' label checks apply on every path. Which families take the config parser is read from their registration (SR Policy declares its NLRI carries attributes), not a family list |
| Version state per script in `bridgerun.script`, reset on respawn | per bridge; per Translator only | X-2 holds "until it restarts"; the script is what restarts |
| Three render formats, rendered once each | render per script | keeps the fan-out cost per format (`docs/architecture/exabgp-bridge.md`) |
| Unknown line: Ze writes the version's detail line and then `error`, where ExaBGP writes the marker alone | copy ExaBGP exactly | `ai/rules/user-facing-errors.md` requires the reason; every 5.x and 6.0 helper already reads a detail line before `error` (X-13), so the extra line breaks none; recorded in `docs/exabgp/exabgp-differences.md` |
| Main pin = a recorded ExaBGP main commit, installed by uv from `git+https://github.com/Exa-Networks/exabgp@<sha>` (changed 2026-10-18, A-1 broken) | the nightly zipapp with its sha256; follow the moving `nightly` tag | a pin that moves is no pin (journal 2026-09-13 row); the zipapp was the first choice, but its asset is replaced each night main changes, so its digest stops being fetchable |
| The stage clears `PYTHONPATH` (option (a) of the 2026-09-13 journal row) | report the resolved version (option (b)) | the main pin now covers the newest ExaBGP, which was the only reason to let the shell's checkout through |
| `encoder` YANG default becomes text | keep json | ExaBGP's default is text (`configuration.get('encoder', 'text')`); the order names it |
| The API keeps 5.0's leniency (keep first, warn; short split sends whole) | refuse like a config | ExaBGP's own choice for the API (X-27, owner Thomas in 0f4da74a7) |

## Open Owner Decisions

| ID | Question | Recommendation |
|----|----------|----------------|
| OD-1 | `announce operational <kind> ...` with a KNOWN kind: Ze does not originate the OPERATIONAL message (draft-ietf-idr-operational-message), so the bridge refuses it. Implement origination, or keep the refusal recorded as a gap? | Keep the refusal, naming that Ze does not send OPERATIONAL, recorded in `docs/exabgp/exabgp-differences.md`; origination is a new protocol feature outside this order |

## Known Limitations
- OPERATIONAL origination: see OD-1. Not in the agreed scope; becomes its own spec only if the owner answers OD-1 with "implement".

## RFC Documentation (Scope: protocol)

Add `// RFC NNNN Section X.Y: "<quoted requirement>"` above enforcing code. The RTC encoder
quotes RFC 4684 Section 4 (both sentences under RFC Summaries); the RTC bridge caller names
RFC 4684 Section 4; the repeated NEXT_HOP rule names RFC 4271 Section 5; the path-information
refusal names RFC 7911 Section 3.

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
- [ ] AC-1..AC-40 all demonstrated
- [ ] Every user story has a working path and a passing test
- [ ] Wiring Test table complete: every row a concrete test name, none deferred
- [ ] `./le verify worktree` passes
- [ ] Feature code integrated (`internal/*`, `cmd/*`), not library-only
- [ ] Integration and Documentation checklists answered Yes/No/N-A with evidence
- [ ] Architectural Verification table filled, including registration over hardcoding
- [ ] Critical Review passes, and `ai/rules/quality.md` is satisfied
- [ ] Every A-N confirmed or broken, none `unvalidated`
- [ ] Every item this spec did not do is a spec of its own, named here, in its own bucket

### TDD
- [ ] Tests written
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)
- [ ] Boundary tests for all numeric inputs
- [ ] Functional `.ci` tests for end-to-end behavior
- [ ] Interop tests for protocol features

### Closure
- [ ] Append `plan/TEMPLATE-CLOSURE.md` and complete every section in it
- [ ] `/ze-review` gate clean, recorded via `internal/le/spec/review.go`
- [ ] Any lesson routed to its governing surface under `ai/rules/planning.md`; no lesson artifact created merely for closure
- [ ] **Commit A:** code + tests + docs + edited spec + any journal rows owed by the work
- [ ] **Commit B:** `remove plan/immediate/spec-exabgp-6-api-compatibility.md` only, in the same `./le commit create` script
