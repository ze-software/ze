# Spec: config archive credentials and plain-HTTP upload

| Field | Value |
|-------|-------|
| Status | design |
| Scope | config |
| Priority | P0 |
| Release reason | Security: the archive destination's credentials are stored and displayed in clear, and the whole config, with its reversible `$9$` secrets, is uploaded over plain HTTP to any remote host the operator names. |
| RFC gaps | - |
| Depends | - |
| Phase | - |
| Handoff | - |
| Design tier | strongest tier |
| Implementation tier | development tier |
| Updated | 2026-11-27 |

Recovery after compaction: `.claude/rules/post-compaction.md`.

## Task

`system archive <name> location` is a plain string. The only way to give the
uploader credentials is userinfo in the URL (`https://user:pass@host/path`):
Go's HTTP client sends it as Basic auth. The leaf carries no `ze:sensitive`, so
the password is stored in the config file, printed by `show config`, carried
in every archived copy, and published on every surface that shows config.
Separately, `ValidateArchiveLocation` accepts `http://` to a remote host, while
`ValidateFetchURL` refuses plain HTTP to anything but loopback. An `http://`
archive sends the whole config, its `$9$` secrets (reversible by
`ze secret decode`) and the Basic auth header, in clear, on every trigger.

Goal: archive credentials live in their own leaves (`username` and `password`
under `list archive`), with `password` marked `ze:sensitive`; a `location`
carrying userinfo is refused at verify (with the password redacted and the
credential leaves named as the fix); an `http://` location to a non-loopback
host is refused at verify by the same loopback rule `ValidateFetchURL` enforces
for reads; and `system/archive[*]` registers as a start-time service block so
a legacy invalid archive entry is isolated at `ze start` under the 2026-11-26
"Ze always starts" ruling rather than blocking boot.

VyOS fixed the same class in T6304 (`575f265da` in
`~/Code/github.com/vyos/vyos-1x`): a structured archive location with separate
credentials.

## Required Reading

### Architecture Docs
- [ ] `ai/doors/system.md` - single YANG schema authority, registration over enumeration, user-facing errors with problem/why/fix
  → Decision: one YANG schema and validator pipeline serves `ze config validate`, editor commit, reload, and `ze config archive`.
  → Constraint: every refusal states the problem, why it is refused, and the exact fix.
- [ ] `ai/doors/config-yang.md` - `ze:validate`, `validatedSections`, `ze:sensitive`, and offline-vs-boot parity
  → Decision: `system` is already in `validatedSections`, so `ze:validate "archive-location"` runs on `ze config validate`, editor commit, reload, and daemon load.
  → Constraint: never rename `location`; add new optional `username` and `password` leaves; keep native YANG `length` constraints alongside `ze:validate`.
- [ ] `ai/doors/appliance.md` - covers `internal/component/config/system` for update backends and appliance images
  → Constraint: no appliance build or initrd path is altered; `ze-system-conf.yang` stays embedded by its existing `embed.go`.
- [ ] `ai/doors/core.md` - leaf tier `internal/core/redact`
  → Constraint: reuse `redact.URL` and `redact.URLError` (which remove the whole userinfo, including bare tokens, and fail closed on unparseable URLs).
- [ ] `ai/doors/le-tooling.md` - functional `.ci` format and Go fixtures under `internal/test/fixture/`
  → Constraint: compiled Go fixtures return an error on assertion failure; `.ci` tests work on both Linux and macOS without production TLS bypasses.
- [ ] `docs/guide/config-archive.md` - archive fields, location schemes, redirect refusal, validation rules, and doctor probes
  → Decision: `archiveClient` follows no redirects (same host included), so setting Basic auth on requests sent through `archiveClient` ensures credentials never leave the configured URL.
  → Constraint: the guide currently documents remote `http://host/path` and URL-embedded credentials; this spec departs from those two documented rules and updates the guide in the same change.
- [ ] `docs/architecture/config/yang-config-design.md` - `ze:sensitive`, `$9$` encoding on persistence/dump/gNMI, `/* SECRET-DATA */` masking on CLI/web display, `ValidateCustomSections`
  → Decision: `decodeSecretLeaf` decodes `$9$` before `ValidateLeafValue` and stores plaintext in `Tree`; `EncodeSecrets` writes `$9$` for `Serialize`/`SerializeSet`/`gNMI Get`; `MaskSecrets` writes `/* SECRET-DATA */` for `ze config show`, SSH `show`, and web config views.
  → Constraint: never mark `location` sensitive (`SensitiveKeys` collects sensitive leaves by name); `password` is already a sensitive leaf name across Ze (`ze-bgp-conf.yang`, `ze-iface-conf.yang`, `ze-container-conf.yang`, `ze-ipsec-conf.yang`), and `redact.Command` already redacts the token following `password`.
- [ ] `docs/architecture/core-design.md` - `## Design decisions` (2026-11-26 "Ze always starts")
  → Decision: `ze start` never exits over one misconfigured service block; `LoadConfigForStart` excludes registered service blocks and reports them via log and `ze doctor`, while commit, reload, and `ze config validate` refuse strictly.
  → Constraint: register `system/archive[*]` via `config.RegisterServiceBlock` in `internal/component/config/archive/register.go` so an invalid archive block is isolated individually at boot without disabling other archive blocks or networking.

**Key insights:**
- `SensitiveKeys` collects sensitive leaves by name, so marking `location` itself sensitive would mask every leaf named `location` in the schema; separate `username` and `password` leaves avoid that and reuse the existing `password` key already marked `ze:sensitive` and redacted by `redact.Command`.
- `ValidateArchiveLocation` is the single check behind commit-time `archive-location` validation, `ze config validate`, reload, manual `ze config archive <name>`, and (after this change) `ExtractConfigs` / `archiveToLocation` / `ToHTTP`.
- Display surfaces split cleanly into two producer contracts: `ze config show`, SSH `show`, and web config views mask `ze:sensitive` leaves to `/* SECRET-DATA */` (`config.MaskSecrets`), while `Serialize`/`SerializeSet`/`ze config fmt`, `ze config dump`, and `gNMI Get` write `$9$` (`config.EncodeSecrets` / `DisplayEncode`). Neither prints plaintext.
- `archiveClient` already sets `CheckRedirect: refuseArchiveRedirect`, so Basic auth attached via `req.SetBasicAuth` is never forwarded on any 3xx response.

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `internal/component/config/system/yang/ze-system-conf.yang` - `list archive` (lines 939-996) defines `name`, `location` (`type string`, `ze:validate "archive-location"`, no `ze:sensitive`), `filename`, `timeout` (`ze:validate "archive-timeout"`), `trigger`, and `on-change`; no `username` or `password` leaf exists.
- [ ] `internal/component/config/validators_archive.go` - `ValidateArchiveLocation` accepts any `file`, `http`, or `https` URL with no host check, no loopback check, and no userinfo check; its refusals redact the URL with `redact.URL` and `redact.URLError`.
- [ ] `internal/component/config/validators.go` - `loopbackFetchHosts` (`127.0.0.1`, `::1`, `localhost`) and `ValidateFetchURL` accept `https` with a non-empty host, accept `http` only when `parsed.Hostname()` is in `loopbackFetchHosts`, and refuse every other form with `redact.URL`.
- [ ] `internal/component/config/validate_sections.go` - `validatedSections` includes `system`, so `ValidateCustomSections` runs `archive-location` and `archive-timeout` for `LoadConfig`, `LoadConfigForStart`, `ze config validate`, and editor commit.
- [ ] `internal/component/config/service_blocks.go` - `RegisterServiceBlock` and `excludeInvalidServiceBlocks` let `LoadConfigForStart` exclude a misconfigured `prefix[*]` list entry individually at daemon start; `system/archive[*]` is not yet registered.
- [ ] `internal/component/config/archive/archive.go` - `ArchiveConfig` has no credential fields; `ExtractConfigs` validates `timeout` via `config.ParseArchiveTimeout` but does not validate `location`; `ToHTTP` POSTs the config through `archiveClient` (which sets `CheckRedirect: refuseArchiveRedirect` so no redirect is ever followed, same host included) and relies on Go's URL userinfo for Basic auth; `archiveToLocation` dispatches `http` and `https` without calling `ValidateArchiveLocation`.
- [ ] `internal/component/config/archive/scheduler.go` - `fireAll` and `fireByTrigger` pass `ac.Name, ac.Location, filename, ac.Timeout` to `archiveToLocation`.
- [ ] `internal/component/config/archive/register.go` - registers `archiveDoctorCheck`, doctor codes, and `CodeDoctorArchiveScheduledFailed`, but does not yet register `system/archive[*]` as a start-time service block.
- [ ] `internal/component/config/archive/doctor.go` - `checkArchiveDestinations` calls `ExtractConfigs(tree)` (reporting `doctor-archive-invalid` on error) and probes HTTP(S) destinations with HEAD through `archiveClient` without following redirects (`doctor-archive-redirected` / `doctor-archive-unreachable`).
- [ ] `internal/component/config/archive/cmd/archive.go` - `handleArchiveTrigger` parses the config, calls `archive.ExtractConfigs`, calls `iconfig.ValidateArchiveLocation(ac.Location)`, and runs `archive.NewNotifier`.
- [ ] `internal/component/config/schema.go`, `secret_text.go`, `mask.go`, `parser.go` - `SensitiveKeys` keys by leaf name; `decodeSecretLeaf` decodes `$9$` into `Tree` before `ValidateLeafValue`; `EncodeSecrets` encodes `ze:sensitive` leaves as `$9$` for `Serialize`/`SerializeSet`/`gNMI Get`; `MaskSecrets` replaces `ze:sensitive` leaves with `/* SECRET-DATA */` for CLI/web display; `RejectMaskedSecretLeaves` refuses `/* SECRET-DATA */` on write.
- [ ] `internal/core/redact/redact.go` - `redact.URL` replaces the whole userinfo (including bare tokens) with `<redacted>` and fails closed on parse errors; `redact.Command` already treats `password` as a credential keyword.
- [ ] `docs/guide/config-archive.md` - documents `http://host/path` as a valid scheme without a loopback restriction (line 47), documents `location` validation as scheme-only (line 79), and mentions URLs with embedded credentials (lines 82-83).

**Behavior to preserve:**
- `file://` locations and `https://` locations without userinfo keep working identically (including `commit-revisions` pruning on `file://` and redirect refusal on `https://`).
- `http://` to a loopback host (`127.0.0.1`, `::1`, `localhost`, with or without port) keeps working, matching `ValidateFetchURL`.
- Unauthenticated HTTP(S) uploads (both `username` and `password` absent) send no `Authorization` header.
- Refusals and upload errors keep redacting credentials, and `archiveClient` keeps refusing all redirects (same host included).
- `ze doctor` HEAD probe keeps reporting `doctor-archive-unreachable`, `doctor-archive-redirected`, and `doctor-archive-invalid` without sending credentials on passive HEAD probes.

**Behavior to change:**
- Add `username` and `password` (`ze:sensitive`) leaves under `system archive <name>`.
- Refuse any `location` carrying userinfo (`parsed.User != nil`, including bare-token userinfo) in `ValidateArchiveLocation`, redacting the URL and naming `username` and `password` as the fix.
- Refuse any `https://` location with an empty host and any `http://` location whose parsed hostname is not in `loopbackFetchHosts` (`127.0.0.1`, `::1`, `localhost`) in `ValidateArchiveLocation`, naming `https://` and the loopback hosts.
- Validate non-empty `location` in `ExtractConfigs` and `archiveToLocation`/`ToHTTP` as defense-in-depth, and set `Authorization: Basic` on the `archiveClient` request in `ToHTTP` when `Username` or `Password` is non-empty.
- Register `system/archive[*]` with `config.RegisterServiceBlock` in `archive/register.go` so `LoadConfigForStart` isolates a legacy invalid archive entry individually at boot rather than refusing daemon startup.
- Update every repository test and doc that uses a remote `http://` archive location or asserts the old `file://, http:// or https://` wording.

## Existing Mechanisms (MANDATORY)

| Mechanism | Where (file + symbol, or page + section) | What it does today | This spec (extends, replaces, or does not apply, with the reason) |
|-----------|------------------------------------------|--------------------|-------------------------------------------------------------------|
| Fetch URL loopback policy | `internal/component/config/validators.go` `ValidateFetchURL`, `loopbackFetchHosts` | Accepts `https://` with a host and `http://` only when `parsed.Hostname()` is in `loopbackFetchHosts` (`127.0.0.1`, `::1`, `localhost`) | Extends: `ValidateArchiveLocation` reuses `loopbackFetchHosts` directly (same parsed-hostname check, no prefix match) so upload and fetch rules cannot drift |
| Archive location validator | `internal/component/config/validators_archive.go` `ValidateArchiveLocation`, `ArchiveLocationValidator` | Checks scheme (`file`, `http`, `https`) for commit, `ze config validate`, reload, and manual `ze config archive` | Extends: adds userinfo refusal, `https://` non-empty host check, and `http://` loopback-only host check |
| Sensitive leaf lifecycle | `ze:sensitive` (`docs/architecture/config/yang-config-design.md`, `internal/component/config/schema.go` `SensitiveKeys`, `secret_text.go` `DecodeSecretText`/`EncodeSecrets`, `mask.go` `MaskSecrets`/`RejectMaskedSecretLeaves`) | Decodes `$9$` on parse, writes `$9$` on `Serialize`/`SerializeSet`/`ze config dump`/`gNMI Get`, masks to `/* SECRET-DATA */` on `ze config show`/SSH/web, and refuses placeholder commits | Extends: the new `system archive <name> password` leaf carries `ze:sensitive` and inherits all surfaces automatically |
| URL and command redaction | `internal/core/redact/redact.go` `redact.URL`, `redact.URLError`, `redact.Command`; `internal/component/config/archive/archive.go` `RedactURL` | `redact.URL` strips whole userinfo (including bare tokens) in validator refusals; `redact.Command` redacts the value after `password`; `archive.RedactURL` redacts redirect target URLs | Extends: new userinfo and plain-HTTP refusals format the offending URL through `redact.URL`; does not replace separate credential leaves because redaction covers messages only, not config storage or display |
| Non-redirecting HTTP uploader | `internal/component/config/archive/archive.go` `ToHTTP`, `archiveClient`, `refuseArchiveRedirect` | POSTs config bytes to the configured location with no redirect following, same host included | Extends: accepts `*ArchiveConfig`, validates `Location`, and calls `req.SetBasicAuth(ac.Username, ac.Password)` on requests sent by `archiveClient` so credentials reach only the configured URL |
| Start-time service block isolation | `internal/component/config/service_blocks.go` `RegisterServiceBlock`, `excludeInvalidServiceBlocks` | Excludes a misconfigured registered service block individually during `LoadConfigForStart` and reports it via log and `ze doctor` while commit/reload stay strict | Extends: `internal/component/config/archive/register.go` registers `system/archive[*]` so a legacy remote-HTTP or userinfo archive block does not prevent `ze start` |
| Archive destination doctor check | `internal/component/config/archive/doctor.go` `checkArchiveDestinations`, `codeArchiveInvalid` | Calls `ExtractConfigs(tree)` and emits `doctor-archive-invalid` on invalid archive config before probing HTTP(S) destinations | Extends: validating non-empty `location` in `ExtractConfigs` ensures `ze doctor` emits `doctor-archive-invalid` for remote `http://` or userinfo locations without sending a HEAD probe |

## Design documents

| Document | Section | Derived from | Binding statement | Verdict | How this design stands against it |
|----------|---------|--------------|-------------------|---------|-----------------------------------|
| `ai/doors/system.md` | One schema, one command registry, every interface | the door every change reads | "Config is YANG-modeled, and `ConfigProvider` is its single authority" | agrees | Credentials are modeled in `ze-system-conf.yang`, validated through `ValidateCustomSections` and `ValidateArchiveLocation`, and masked/encoded by the shared `ze:sensitive` machinery across every surface; every refusal names the problem, why, and the fix. |
| `ai/doors/config-yang.md` | Who depends on it from outside | the door covering `internal/component/config/validators_archive.go`, `archive/archive.go`, `archive/scheduler.go`, `archive/register.go`, `validators_archive_test.go`, `archive/archive_test.go`, `archive/doctor_test.go`, `archive_secret_test.go` | "A leaf rename breaks every saved config and every script that writes it." | agrees | `leaf location` keeps its name and `ze:validate "archive-location"`; credentials are added as optional sibling leaves (`username`, `password`) with native `length` constraints, and offline `ze config validate`, commit, reload, and `ze config archive` share `ValidateArchiveLocation`. |
| `ai/doors/appliance.md` | Where it sits | the door covering `internal/component/config/system/yang/ze-system-conf.yang` | | silent | `appliance.md` covers `internal/component/config/system` for update backends (`UpdateBackend`) and appliance images; it states no rule about `system/archive` credentials or upload transport. |
| `ai/doors/le-tooling.md` | Boundaries | the door covering `test/parse/cli-config-archive-invalid-refused.ci`, `test/ui/config-archive-credentials.ci`, `internal/test/fixture/config_archive_credentials_fixture.go`, `internal/test/fixture/register_config_archive_credentials.go` | "A compiled Go fixture returns an error when an assertion fails" | agrees | The new `ui/config-archive-credentials` fixture follows `config_archive_prune_fixture.go`, returns a non-nil error on every failed assertion, uses loopback `http://127.0.0.1` so it runs on both Linux and macOS without `option=needs-linux`, and cleans up its temporary directory. |
| `docs/architecture/core-design.md` | 20. System Update Backend | anchored to `internal/component/config/system/yang/ze-system-conf.yang` | | silent | `core-design.md` anchors to `ze-system-conf.yang` for system update backends and appliance device config; its prose makes no statement about `system/archive` credentials or HTTP transport. All 13 `Design decisions` rows are judged individually below. |
| `docs/architecture/resolve.md` | Where the files are read from | anchored to `internal/component/config/system/yang/ze-system-conf.yang` | "A source is HTTPS, or plain HTTP from the host itself. What the rule protects is the ANSWER: this table decides which registry Ze reports as holding an AS number, so a transport nobody authenticates must not carry it off the box. The host is compared after parsing rather than by prefix, so `http://127.0.0.1.example.com` is not loopback." | agrees | `ValidateFetchURL` and RIR delegation sources are unchanged, and `ValidateArchiveLocation` reuses the exact same `loopbackFetchHosts` slice with parsed-hostname comparison (`parsed.Hostname()`) so archive uploads enforce the same transport guard as fetches. |
| `docs/guide/config-archive.md` | Location Schemes | the `// Design:` page of `internal/component/config/validators_archive.go`, `archive/archive.go`, `archive/scheduler.go`, `archive/register.go`, `archive/doctor_test.go`; modified by this spec | "HTTP POST with `text/plain` content type. Filename sent in `X-Archive-Filename` header." | departs | Under `Location Schemes`, `http://host/path` is currently documented as accepting any host over plain HTTP; this spec restricts `http://` to loopback hosts (`127.0.0.1`, `::1`, `localhost`), requires `https://` for remote uploads, and updates `docs/guide/config-archive.md` in the same change. |
| `docs/guide/config-archive.md` | Validation | the `// Design:` page of `internal/component/config/validators_archive.go`, `archive/archive.go`; modified by this spec | "Its scheme is not `file`, `http` or `https`, or it has no scheme" | departs | Under `Validation`, `location` is currently documented as refused only on scheme errors, with URL-embedded credentials implied by "never echoes the password of a URL with credentials in it"; this spec also refuses userinfo in `location` (directing operators to `username` and `password`) and non-loopback `http://`, and updates `docs/guide/config-archive.md` in the same change. |
| `docs/guide/config-archive.md` | Location Schemes | the `// Design:` page of `internal/component/config/archive/archive.go`; modified by this spec | "An HTTP or HTTPS upload follows no redirect, to the same host included. The upload carries the whole config with its reversible `$9$` secrets, so it goes only to the location the config names, never to one a server answers with." | agrees | `ToHTTP` attaches Basic auth from `username`/`password` only to requests sent by `archiveClient` (`refuseArchiveRedirect`), so neither the config body nor the `Authorization` header is ever sent to a redirect target. |
| `docs/architecture/config/yang-config-design.md` | Extensions: `ze-extensions.yang` | added: governs `ze:sensitive` behavior | "Leaf holds sensitive data. Display obfuscates it with JunOS-compatible `$9$` encoding, which `ze secret decode` reverses offline" | agrees | `system archive <name> password` uses `ze:sensitive`, so every parser decodes `$9$`, persistence/dump/gNMI write `$9$`, and CLI/web display views mask it with `/* SECRET-DATA */`. |
| `docs/architecture/core-design.md` | Design decisions | decision of 2026-11-23 | "A commit fails when it configures a feature that programs only the Linux kernel, or that reads only what the Linux kernel sees of the traffic, while the interfaces run on VPP. Two host-side uses stay accepted, so that mixed setups remain possible once VRFs are added: a `fib kernel` block beside a `fib vpp` block, which programs the Linux route table for the router's own traffic, and `sysctl`, which sets the Linux kernel's own tunables. A `fib kernel` block with no `fib vpp` block beside it is refused, for now." | agrees | Config archive is host/management configuration with no data-plane programming; this spec introduces no kernel-only data-plane feature and makes no change to VPP/kernel validation. |
| `docs/architecture/core-design.md` | Design decisions | decision of 2026-11-23 | "A configuration that holds such a mismatch is refused at daemon start as well as at commit, reload and `ze config validate`, by one verdict that is the same at each." | agrees | This spec does not touch data-plane mismatch checks and keeps one `ValidateArchiveLocation` verdict across `ze config validate`, commit, reload, and manual `ze config archive` (with start-time service-block isolation per the 2026-11-26 ruling below). |
| `docs/architecture/core-design.md` | Design decisions | decision of 2026-11-23 | "The VPP firewall verifier, not a YANG annotation, decides whether `connection-state` and `mark-set` are accepted on VPP. The `ze:backend` annotation of `flow-offload` is deleted with theirs; `flow-offload` stays refused on VPP because VPP has no flowtable and the `flowtable` list keeps its `ze:backend "kernel"` annotation." | agrees | This spec touches no firewall or `ze:backend` annotations. |
| `docs/architecture/core-design.md` | Design decisions | decision of 2026-11-23 | "Every data-plane selector names the plane it programs, `kernel` or `vpp`: `interface backend`, `firewall backend` and `traffic control backend` take `kernel` where they took `netlink`, `nft` and `tc`, and each defaults to `kernel`; the `fib` blocks already name `kernel` and `vpp`. The plane a registration declares and the value a `ze:backend` annotation names are the same word. A second implementation on the same plane, such as eBPF, takes a value of its own when it exists. In Thomas's words (voice-transcribed): "should tc and netfilter be renamed to kernel? so we have vpp and kernel?", then "please do it"." | agrees | This spec introduces no data-plane selector or backend name. |
| `docs/architecture/core-design.md` | Design decisions | decision of 2026-11-23 | "BGP egress next-hop rewriting belongs exclusively in the export filter chain, not under session configuration (`session { next-hop }`). Ze automatically appends `next-hop auto` at the end of the export chain when no earlier export filter has already set a next-hop outcome (`self`, `unchanged`, or an explicit address), so ordinary eBGP rewrites to self while iBGP and route-server clients preserve the received next hop. In Thomas's words (voice-transcribed): "I would think that all the rewrite should be in the prefix export and not under configuration. I believe we should automatically add to the export prefix list the auto at the end of the list if not defined with something else. So if no one, for example, used a self or unchanged, we should automatically change it on eBGP. Because otherwise we have two ways to modify packets on the way out to the peer.", then "Unify into the export chain it is the way I want it."" | agrees | This spec touches only `system/archive` and makes no change to BGP session or export-chain configuration. |
| `docs/architecture/core-design.md` | Design decisions | decision of 2026-11-23 | "Egress export chains configure next-hop rewriting through one underlying `policychain.Sets` next-hop intent exposed in two authoring forms: `policy { modify <name> { set { next-hop self \| unchanged \| auto \| <ip>; } } }` for matched/conditional rules, and built-in chain references `next-hop:self`, `next-hop:unchanged`, and `next-hop:auto` for peer-wide policy without a wrapper `modify` block. Selected by Thomas on 2026-11-23 ("Option A: Extend policy modify (self\|unchanged\|auto\|<ip>) AND provide built-in next-hop:self / next-hop:unchanged / next-hop:auto chain refs")." | agrees | This spec touches only `system/archive` and makes no change to policy modify or chain references. |
| `docs/architecture/core-design.md` | Design decisions | decision of 2026-11-23 | "Built-in `next-hop:auto`, `next-hop:self`, and `next-hop:unchanged` are BGP engine steps compiled into `policychain.Chain` (parallel to `loop-detection`), never external plugin RPCs or a `pkg/plugin/rpc.FilterInsertion` tail extension. Config loading appends `next-hop:auto` at the tail of every export chain unless an unconditional `next-hop:self`, `next-hop:unchanged`, or `next-hop:auto` is already present, leaving `Policy.Configured()` false for unconfigured chains; operators override via `next-hop:unchanged`, and `inactive:next-hop:auto` is refused. Selected by Thomas on 2026-11-23 ("Option A: Built-in BGP engine step appended at config compile time (like loop-detection; zero RPC; override via next-hop:unchanged, no inactive:next-hop:auto)")." | agrees | This spec touches only `system/archive` and makes no change to BGP filter chains or `pkg/plugin/rpc`. |
| `docs/architecture/core-design.md` | Design decisions | decision of 2026-11-23 | "Within an export chain, explicit next-hop sets (`self`, `unchanged`, `auto`, or an IP address) use standard last-write-wins folding while the auto-appended trailing `next-hop:auto` acts only when next-hop is still unset. For a whole-UPDATE raw override (`wireOverride`), Ze compares each emitted next-hop field against the received UPDATE (matching `forward_med.go` provenance): trailing `auto` is skipped only when the raw override changed the next-hop bytes or an earlier chain member already selected `unchanged`/`self`/explicit. Selected by Thomas on 2026-11-23 ("Option A: Provenance check on raw overrides (like MED): skip trailing auto only when the raw override changed the next-hop bytes or an earlier member set unchanged/self")." | agrees | This spec touches only `system/archive` and makes no change to BGP export chain folding or raw overrides. |
| `docs/architecture/core-design.md` | Design decisions | decision of 2026-11-23 | "Explicit next-hop addresses in `policy { modify <name> { set { next-hop <ip>; } } }` and `policychain.Sets.NextHop` support both IPv4 (attr 3) and IPv6/MP_REACH (through `NextHopProfile` and `ValidatePeerGlobalNextHop`) on both export and import chains. Egress-only next-hop modes (`self`, `unchanged`, `auto`), whether written as built-in `next-hop:*` refs or inside `policy { modify }`, are refused on import chains at config load. Selected by Thomas on 2026-11-23 ("Option A: Support IPv4 + IPv6/MP_REACH for explicit next-hop on both export and import; refuse self/unchanged/auto on import at config load")." | agrees | This spec touches only `system/archive` and makes no change to BGP next-hop validation. |
| `docs/architecture/core-design.md` | Design decisions | decision of 2026-11-23 | "Auto-appended trailing `next-hop:auto` applies only to forwarded routes and preserves the originator-chosen next hop on locally originated routes (static config routes, ExaBGP `announce`, API announcements). Explicit export-chain next-hop actions (`next-hop:self`, `modify { set { next-hop ... } }`) apply uniformly to both forwarded and locally originated routes. Selected by Thomas on 2026-11-23 ("Option A: Trailing next-hop:auto applies to forwarded routes only (preserves static/ExaBGP/API explicit next-hop); explicit next-hop:self / modify next-hop applies to both forwarded and originated routes")." | agrees | This spec touches only `system/archive` and makes no change to BGP route origination or forwarding. |
| `docs/architecture/core-design.md` | Design decisions | decision of 2026-11-23 | "Every automatic filter-chain member Ze inserts — both built-in BGP engine steps (`next-hop:auto` at the export tail, `loop-detection` at the import head) and plugin-declared `FilterInsertions` — is published through one queryable implied-filter seam carrying direction, position, reference and `implied-by` reason so `\| implied` displays every extra chain member alongside the operator's written filter list. In Thomas's words (2026-11-23): "Can you make sure that \| implied show the extra chain members when it displays the filter ?"." | agrees | This spec touches only `system/archive` and adds no automatic filter-chain member. |
| `docs/architecture/core-design.md` | Design decisions | decision of 2026-11-26 | "Ze always starts. The configuration of one service never makes `ze start` exit or refuse to start: a service whose configuration cannot be served is not started, ze runs with its other services, and the service is reported with its fix by an error line in the log under a doctor code and by `ze doctor` under the same code. Examples are a web certificate name that does not resolve, a looking-glass `server` entry with no certificate to present, and a looking-glass address list that does not parse. Where a service has several endpoints, only the endpoint that cannot be served stays down and the others serve. A reload whose config cannot be served stays refused as a whole, and the running services keep what they serve. In Thomas's words (voice-transcribed): "we should never not start ze as if we can not bring networking we can not fix things like web cert and the like so the only good answer is to disable the service badly configured and not start it"; asked about a reload, "reload stay refused"; asked "Should only that endpoint stay down while the others serve", "yes". The reason is that a daemon that does not start brings up no networking, so the operator cannot reach the box to correct it." | agrees | `internal/component/config/archive/register.go` registers `system/archive[*]` with `config.RegisterServiceBlock`. An invalid archive entry (for example, a legacy remote `http://` or userinfo `location` already on disk before upgrade) is excluded individually by `LoadConfigForStart` so `ze start` boots with all other archive blocks and networking intact and reports the excluded block with its fix via log and `ze doctor`; commit, reload, `ze config validate`, and manual `ze config archive` continue to refuse it. |
| `docs/architecture/core-design.md` | Design decisions | decision of 2026-11-26 | "The rule of 2026-11-23 above, that a configuration which holds a feature that programs only the Linux kernel while the interfaces run on VPP "is refused at daemon start", is superseded for the start: ze starts, that feature stays off, and `ze doctor` reports the mismatch with its fix. A commit, a reload and `ze config validate` still refuse such a configuration, by the same verdict as before. Thomas was asked whether the rule that ze always starts replaces the refusal at start, "(a) yes, ze starts, that feature stays off and ze doctor reports the mismatch, while commit and reload still refuse it; (b) no", and answered "2a" (voice-transcribed). The reason is the one of the row above: a daemon that does not start brings up no networking, so the operator cannot reach the box to correct the mismatch." | agrees | This spec introduces no VPP/kernel mismatch and applies the same start-isolation / strict-commit-and-reload split to `system/archive[*]` through `config.RegisterServiceBlock`. |

## Data Flow (MANDATORY - see `ai/rules/architecture.md`)

### Entry Point
- Operator configuration: `system archive <name> location <url>`, `system archive <name> username <user>`, and `system archive <name> password <secret>` (accepted in plaintext or `$9$` form via hierarchical config, set format, CLI editor, web editor, REST/gRPC, or gNMI Set).
- Manual trigger: `ze config archive <name>` -> SSH -> `request config archive <name>` (`ze-config:archive-trigger`).
- Scheduled trigger: `archive.Scheduler` (`daily` / `hourly` and boot archive).
- Commit trigger: editor commit hook -> `archive.NewNotifier`.

### Transformation Path
1. **Parse and secret decode:** Config parsers (`parser.go`, `setparser.go`, `setparser_meta.go`, `parser_list.go`) decode `$9$` on `password` (`ze:sensitive`) via `decodeSecretLeaf` into plaintext in `Tree` (retaining stored `$9$` text for round-trip stability) and check YANG `length` (`1..255` for `username`, `1..4096` for `password`) via `ValidateLeafValue`.
2. **Validation:** `ValidateCustomSections` walks `system` and invokes `ArchiveLocationValidator` -> `ValidateArchiveLocation(rawURL)`:
   - Checks scheme (`file`, `https`, `http`); refuses empty/missing/unsupported schemes with `redact.URL`.
   - Checks `parsed.User != nil`; refuses any URL with userinfo (password or bare token) on any scheme, redacting the URL with `redact.URL` and naming `username` and `password` as the fix.
   - For `https`: refuses an empty host (`parsed.Host == ""`).
   - For `http`: checks `slices.Contains(loopbackFetchHosts, parsed.Hostname())`; refuses any non-loopback host (including lookalikes such as `127.0.0.1.example.com`) naming `https://` and `loopbackFetchHosts`.
   - At `ze start` (`LoadConfigForStart`), a failure under `system/archive[<name>]` matches the `system/archive[*]` service block registration, excluding only that entry and reporting it via log and `ze doctor`. On `ze config validate`, editor commit, reload (`LoadConfig`), and manual `ze config archive`, it refuses strictly.
3. **Display and persistence:**
   - Persistence / dump / gNMI (`Serialize`, `SerializeSet`, `SerializeSetWithMeta`, `ze config dump`, `gNMI Get`/`Subscribe`) pass through `EncodeSecrets` / `DisplayEncode` -> `password` is emitted in `$9$` form.
   - Display surfaces (`ze config show`, SSH CLI `show`, web config forms and terminal `show`, `ze config set` echo, `redact.Command` logs/transcripts) pass through `MaskSecrets` / `DisplayValueAtPath` / `redact.Command` -> `password` is masked as `/* SECRET-DATA */` (or `<redacted>` in logs).
4. **Extraction and upload:**
   - `archive.ExtractConfigs(tree)` reads `location`, `username`, `password`, `filename`, `timeout`, `trigger`, and `on-change` into `ArchiveConfig`, validating non-empty `location` via `config.ValidateArchiveLocation` (in addition to `timeout` via `config.ParseArchiveTimeout`).
   - `NewNotifier`, `Scheduler.fireAll`, and `Scheduler.fireByTrigger` pass `&ac` to `archiveToLocation(content, &ac, filename)`.
   - `archiveToLocation` validates `ac.Location` via `config.ValidateArchiveLocation` and dispatches `file` -> `ToFile` or `http`/`https` -> `ToHTTP(content, &ac, filename)`.
   - `ToHTTP` validates `ac.Location` via `config.ValidateArchiveLocation`, builds the `POST` request with `Content-Type: text/plain` and `X-Archive-Filename`, calls `req.SetBasicAuth(ac.Username, ac.Password)` when `ac.Username != "" || ac.Password != ""`, and sends through `archiveClient` (which refuses all redirects via `refuseArchiveRedirect`, so the Basic auth header and config body reach only the configured URL).

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Config file / editor / gNMI ↔ `config.Tree` | YANG schema (`ze-system-conf.yang`), `decodeSecretLeaf`, `EncodeSecrets`, `MaskSecrets` | No |
| `config` validator ↔ `archive` uploader | Shared `config.ValidateArchiveLocation` and `loopbackFetchHosts` | No |
| Ze daemon / editor ↔ External HTTP(S) archive server | HTTPS (or loopback HTTP) `POST`, `Authorization: Basic` from `username`/`password`, `Content-Type: text/plain`, `X-Archive-Filename`, no redirect following (`archiveClient`) | No |
| `LoadConfigForStart` ↔ `cmd/ze/hub` startup reporting | `config.RegisterServiceBlock("system/archive[*]", "config archive")` -> `ExcludedBlock` -> log + `ze doctor` | No |

### Changed-Wire Census

**Search evidence:** Searched the full checkout with `git grep -n "ValidateArchiveLocation\|ExtractConfigs\|ToHTTP\|archiveToLocation\|ArchiveConfig\|archive-location\|archiveHTTPHead\|probeArchiveDestination" -- internal/ cmd/`, `git grep -n "location.*http\|location.*@" -- test/ docs/ etc/ internal/ cmd/`, `git grep -n "system archive\|archive {" -- docs/ test/ etc/ internal/ cmd/`, and `git grep -l "archive" -- test/ etc/ internal/test/fixture/`. Positive controls verified: `internal/component/config/validators_archive.go:35` (`ValidateArchiveLocation`), `internal/component/config/archive/archive.go:171` (`ToHTTP`) and `:299` (`ExtractConfigs`), `internal/component/config/archive/cmd/archive.go:112` (`handleArchiveTrigger`), `test/parse/cli-config-archive-invalid-refused.ci:30`, `test/ui/doctor-update-archive.ci:18`, and `docs/guide/config-archive.md:22,47,79`. Census found zero `.ci` configs in `test/` or `etc/` using remote `http://` or URL userinfo for `system archive`; it found one remote `http://` archive URL in `internal/component/config/validators_archive_test.go:17`, one remote `http://` call in `internal/component/config/archive/archive_test.go:177`, one URL-userinfo call in `internal/component/config/archive/archive_test.go:961`, one old scheme-list expectation in `test/parse/cli-config-archive-invalid-refused.ci:59`, and remote `http://` / URL-credential prose in `docs/guide/config-archive.md:47,79,82-83`.

| Wire | Role (writer / reader) | Carrier (file + producer/consumer symbol or script entry) | Source evidence | Migration or preservation disposition |
|------|------------------------|-----------------------------------------------------------|-----------------|----------------------------------------|
| `system archive` config schema | writer | `internal/component/config/serialize.go` `Serialize`, `serialize_set.go` `SerializeSet` / `SerializeSetWithMeta`, `cli/cmd_set.go` `cmdSetImpl`, `internal/component/cli/editor_commands.go` `SetValue`, `internal/component/gnmi/set.go` | All config writers walk the YANG schema and invoke `EncodeSecrets` / `DisplayValueAtPath` for `ze:sensitive` leaves (`secret_text.go`, `mask.go`). | Preserve unchanged: adding `username` and `password` (`ze:sensitive`) to `ze-system-conf.yang` automatically wires `$9$` serialization and command redaction on every writer. |
| `system archive` config schema | writer | Repository `.ci` and doc config samples: `test/parse/cli-config-archive-invalid-refused.ci`, `test/parse/cli-config-archive-named.ci`, `test/parse/cli-config-system-archive.ci`, `test/parse/cli-config-archive.ci`, `test/ui/doctor-update-archive.ci`, `test/ui/doctor-writable-extended.ci`, `internal/test/fixture/config_archive_prune_fixture.go`, `docs/guide/config-archive.md` | Checked every file from the repo grep: `cli-config-archive-named.ci` (`https://server/archive`), `doctor-update-archive.ci` (`https://198.51.100.99/archive`), and `file://` fixtures already obey the new rules; `cli-config-archive-invalid-refused.ci:59` asserts the old accepted-forms string and `docs/guide/config-archive.md:47,79,82-83` documents remote `http://` and URL credentials. | Change `cli-config-archive-invalid-refused.ci` and `docs/guide/config-archive.md` (and add `test/ui/config-archive-credentials.ci`); preserve all other already-compliant `.ci` files. |
| `system archive` config schema | reader | `internal/component/config/parser.go` `parseLeaf`, `setparser.go`, `setparser_meta.go`, `parser_list.go` | Calls `decodeSecretLeaf` then `ValidateLeafValue` for every leaf declared in YANG. | Preserve unchanged: automatically decodes `$9$` and enforces `length` on `username` and `password`. |
| `system archive` config schema | reader | `internal/component/config/validators_archive.go` `ValidateArchiveLocation` / `ArchiveLocationValidator`, reached by `validate_sections.go` `ValidateCustomSections`, `service_blocks.go` `excludeInvalidServiceBlocks`, and `archive/cmd/archive.go` `handleArchiveTrigger` | Single validator for `system/archive[*]/location` across validate, commit, reload, start, and manual trigger. | Change: add userinfo refusal, `https://` non-empty host check, and `http://` loopback-only check reusing `loopbackFetchHosts`; register `system/archive[*]` in `archive/register.go`. |
| `system archive` config schema | reader | `internal/component/config/archive/archive.go` `ExtractConfigs` (called by `cmd/ze/hub/main_system.go` `startArchiveScheduler`, `config/cli/cmd_edit.go`, `archive/cmd/archive.go` `handleArchiveTrigger`, `archive/doctor.go` `checkArchiveDestinations`) | Reads `location`, `filename`, `timeout`, `trigger`, `on-change` from `system.archive` list entries. | Change: read `username` and `password` into `ArchiveConfig` and validate non-empty `location` via `config.ValidateArchiveLocation`. |
| `system archive` config schema | reader | Display surfaces: `internal/component/cli/editor_mask.go` `DisplayContentAtPath`/`DisplayTreeAtPath`, `internal/component/config/cli/cmd_show.go`, `internal/component/web/secret.go` `maskSecretLeaf`/`config.MaskSecrets`, `internal/component/config/cli/cmd_dump.go` `SensitiveKeys`/`maskMapValues`, `internal/component/gnmi/get.go` & `subscribe.go` `EncodeSecrets` | All surfaces derive secret handling from `LeafNode.Sensitive` (`LeafHoldsSecret` / `SensitiveKeys` / `EncodeSecrets`). | Preserve unchanged (verified by unit and `.ci` tests): `password` with `ze:sensitive` masks to `/* SECRET-DATA */` in CLI/web display and prints `$9$` in `ze config dump`, `Serialize`, and `gNMI Get`. |
| HTTP archive upload POST | writer | `internal/component/config/archive/archive.go` `ToHTTP`, `archiveToLocation` (called by `NewNotifier` and `scheduler.go` `fireAll`/`fireByTrigger`) | Builds `http.MethodPost` request sent through `archiveClient` (`CheckRedirect: refuseArchiveRedirect`) with `Content-Type: text/plain` and `X-Archive-Filename`. | Change: take `*ArchiveConfig`, validate `ac.Location` via `config.ValidateArchiveLocation`, and call `req.SetBasicAuth(ac.Username, ac.Password)` when `ac.Username != "" \|\| ac.Password != ""`. |
| HTTP archive upload POST | reader | External HTTP/HTTPS archive receiver outside the repository (and in-repo test receivers in `internal/component/config/archive/archive_test.go` and `internal/test/fixture/config_archive_credentials_fixture.go`) | Receives `POST` body, `X-Archive-Filename`, and RFC 7617 `Authorization: Basic` header. | Preserve wire compatibility: standard HTTP Basic auth header is identical on the wire to what Go's client previously generated from URL userinfo, while keeping credentials out of URLs, config display, and redirect targets. |
| HTTP archive doctor HEAD probe | writer | `internal/component/config/archive/doctor.go` `probeArchiveDestination` (via `checkArchiveDestinations`) | Sends unauthenticated `HEAD` through `archiveClient` (`refuseArchiveRedirect`); any non-redirect HTTP status (including 401/404) counts as reachable. | Preserve probe unchanged: `ExtractConfigs` now rejects invalid locations as `doctor-archive-invalid` before any probe runs, and passive HEAD probes do not transmit credentials. |
| HTTP archive doctor HEAD probe | reader | External HTTP/HTTPS archive receiver outside the repository (and in-repo test servers in `internal/component/config/archive/doctor_test.go`) | Answers the unauthenticated `HEAD` request with a non-redirect status (including 401/404) or a 3xx redirect that `archiveClient` refuses. | Preserve wire compatibility unchanged: the passive `HEAD` probe still sends no `Authorization` header and interprets any non-redirect response as reachable. |

### Integration Points
- `internal/component/config/validators.go` (`loopbackFetchHosts`) — shared loopback host slice reused directly by `ValidateArchiveLocation`.
- `internal/component/config/validators_archive.go` (`ValidateArchiveLocation`) — single location check shared by YANG `archive-location`, manual `ze config archive`, `ExtractConfigs`, `archiveToLocation`, and `ToHTTP`.
- `internal/component/config/archive/archive.go` (`ArchiveConfig`, `ExtractConfigs`, `archiveToLocation`, `ToHTTP`) — carries decoded credentials and attaches Basic auth on `archiveClient` requests.
- `internal/component/config/archive/register.go` (`init`) — registers `system/archive[*]` with `config.RegisterServiceBlock`.

### Architectural Verification
| Check | Holds? | Evidence |
|-------|--------|----------|
| No bypassed layers (data flows through the intended path) | Yes | Credentials enter via YANG `ze-system-conf.yang`, decode in `config` parser (`decodeSecretLeaf`), validate in `ValidateCustomSections` / `ValidateArchiveLocation`, extract in `ExtractConfigs`, and attach in `ToHTTP` on `archiveClient`. |
| No unintended coupling (components stay isolated) | Yes | `archive` already imports `config`; `config` never imports `archive`. `ValidateArchiveLocation` stays in `config` and shares `loopbackFetchHosts` in the same package. |
| No duplicated functionality (extends existing, does not recreate) | Yes | Reuses `loopbackFetchHosts`, `redact.URL`, `ze:sensitive` (`EncodeSecrets`/`MaskSecrets`/`SensitiveKeys`), `archiveClient` (`refuseArchiveRedirect`), and `config.RegisterServiceBlock`. |
| Zero-copy preserved where applicable (refs, not copies) | Yes | Control-plane config path only; `archiveToLocation` and `ToHTTP` take `*ArchiveConfig` (avoiding copying a >80-byte struct) and stream `bytes.NewReader(content)`. |
| Registration over hardcoding, outbound: new commands, views, families, and handlers register, and the core discovers them. No per-feature field, switch case, or factory is added to a core/shared package (`ai/rules/plugins.md`) | Yes | No new command, view, family, or handler is added; start-time isolation uses the existing `config.RegisterServiceBlock` registry from `archive/register.go`. |
| Registration over hardcoding, inbound: no existing switch, seed map, validator, parser, runner, help string, or completion table has to learn this feature's name. Evidence names every list that was searched for the names this feature introduces, and the registry each one now derives from (`ai/rules/principles.md`) | Yes | Searched `schema.go` (`SensitiveKeys`), `mask.go` (`SecretKeys`), `redact/redact.go` (`isSecretConfigKey`), `validators_register.go` (`RegisterValidators`), `validate_sections.go` (`validatedSections`), and `service_blocks.go` (`RegisterServiceBlock`): `username` and `password` derive automatically from `ze-system-conf.yang` (completion, web forms, CLI help, `$9$` encoding, secret masking), `password` is already in `redact.isSecretConfigKey`, `archive-location` is already registered in `RegisterValidators` under `system`, and `system/archive[*]` registers into `service_blocks.go` from `archive/register.go`. |

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis (file/doc/user statement) | If wrong | Validated by | Status |
|----|-----------|--------------------------------|----------|--------------|--------|
| A-1 | No archive config in `test/`, `etc/`, or `docs/` requires plain `http://` to a remote host or URL userinfo | Full repo grep of `test/`, `etc/`, `docs/`, and `internal/` performed during design | Any existing test or doc depending on remote `http://` or userinfo would break until converted | Grep census above + running `parse` and `ui` functional suites and `internal/component/config/...` unit tests | validated (census found 0 `.ci` configs using remote `http://` or userinfo; 2 Go unit test literals and 1 `.ci` error-string assertion + `docs/guide/config-archive.md` are in `Files to Modify`) |
| A-2 | Marking `system archive <name> password` with `ze:sensitive` introduces no new `SensitiveKeys` name collision | `git grep "leaf password" internal/` shows `password` is already `ze:sensitive` in `ze-bgp-conf.yang`, `ze-iface-conf.yang`, `ze-container-conf.yang`, `ze-ipsec-conf.yang`, `ze-l2tp-auth-local-conf.yang`, and `ze:bcrypt` in `ze-authz-conf.yang`/`ze-telemetry-conf.yang` (`cmd_dump.go` masks bcrypt first to avoid `$9$` collision) | Another leaf named `password` could be masked unexpectedly | `SensitiveKeys` and `cmd_dump.go:309-319` source read + config unit test suite | validated |
| A-3 | Existing routers with legacy remote `http://` or userinfo archive URLs on disk must still boot after upgrading Ze | `docs/architecture/core-design.md` 2026-11-26 decision ("Ze always starts") | `ze start` would fail on upgrade and cut off network access to fix the config | Unit test of `LoadConfigForStart` with `system/archive[*]` registered showing invalid archive entry excluded and valid sibling kept | unvalidated (proven by new unit test in Phase 2) |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | Calling `ValidateArchiveLocation` inside `ToHTTP` breaks an existing unit test that passes a remote `http://` or userinfo URL directly to `ToHTTP` | `TestToHTTP_Unreachable` or `TestToHTTP_RedactsCredentials` fails in `internal/component/config/archive` | Update `TestToHTTP_Unreachable` to use `https://192.0.2.1:9999/archive` and `TestToHTTP_RedactsCredentials` to assert immediate redacted refusal without opening a socket, keeping `TestRedactURL_WithCredentials` unchanged for redirect redaction |
| R-2 | Functional `.ci` test using a self-signed HTTPS server fails on macOS where Go's `crypto/x509` ignores `SSL_CERT_FILE` | `test/ui/config-archive-credentials.ci` passes on Linux but fails TLS handshake on macOS | Drive the `.ci` fixture against a loopback `http://127.0.0.1:<port>/upload` receiver (which works identically on Linux and macOS and proves the loopback exception + full daemon upload + Basic auth + `$9$` body + `show config` masking), and prove TLS upload (`https://`) with `httptest.NewTLSServer` in `archive_test.go` |
| R-3 | Changing `archiveLocationForms` breaks `.ci` or unit tests asserting exact refusal text | `test/parse/cli-config-archive-invalid-refused.ci` or `validators_archive_test.go` fails | Update both in the same change and verify `TestValidateArchiveLocation_Invalid` checks the new accepted-forms wording |
| R-4 | A `system archive` password set to `/* SECRET-DATA */` (pasted from `show config`) could be uploaded as a literal password | Operator pastes masked `show config` output and commits | `ze:sensitive` automatically puts `password` under `config.LeafHoldsSecret` -> `config.RejectMaskedSecretLeaves` refuses `/* SECRET-DATA */` at validate, commit, and load (verified by unit test) |

## Blast Radius

| Question | Answer |
|----------|--------|
| What breaks if this is wrong? | Config archive uploads could fail authentication (wrong Basic auth header), leak archive passwords in config display/dump/errors, wrongly reject valid `https://` or loopback `http://` archive blocks, or block `ze start` on a router with a legacy invalid archive entry. Live BGP/routing sessions are not affected. |
| How is it reverted? | Single commit revert (`ze-system-conf.yang`, `validators_archive.go`, `archive/`, tests, and `docs/guide/config-archive.md`); configs written with `username`/`password` would only need those leaves removed if reverted. |
| Who else touches this path? | `cmd/ze/hub/main_system.go` (`startArchiveScheduler`), `internal/component/config/cli/cmd_edit.go` (editor commit notifier), `internal/component/config/archive/cmd/archive.go` (`request config archive`), `internal/component/config/archive/doctor.go` (`ze doctor`), and `internal/component/config/validate_sections.go` (`ValidateCustomSections`). |

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| `system archive` with `username` and `password` + `ze config archive` / commit trigger | → | `ExtractConfigs` → `archiveToLocation` → `ToHTTP` Basic auth on `archiveClient` | `test/ui/config-archive-credentials.ci` (`internal/test/fixture/config_archive_credentials_fixture.go`) |
| `system archive` with `password` → `ze config show`, `ze config dump`, `Serialize`, `gNMI Get` | → | `ze:sensitive` (`MaskSecrets`, `EncodeSecrets`, `SensitiveKeys`) | `test/ui/config-archive-credentials.ci` and `TestArchivePasswordSensitiveSurfaces` (`internal/component/config/archive_secret_test.go`) |
| `system archive <name> location` with userinfo | → | `ValidateCustomSections` → `ArchiveLocationValidator` → `ValidateArchiveLocation` | `test/parse/cli-config-archive-invalid-refused.ci` and `TestValidateArchiveLocation_RefusesUserinfo` (`internal/component/config/validators_archive_test.go`) |
| `system archive <name> location` with remote `http://` | → | `ValidateCustomSections` → `ArchiveLocationValidator` → `ValidateArchiveLocation` (and `handleArchiveTrigger`) | `test/parse/cli-config-archive-invalid-refused.ci`, `test/ui/config-archive-credentials.ci`, and `TestValidateArchiveLocation_RefusesRemoteHTTP` (`internal/component/config/validators_archive_test.go`) |
| `ze start` (`LoadConfigForStart`) with an invalid `system archive` entry | → | `excludeInvalidServiceBlocks` via `system/archive[*]` service block registration | `TestLoadConfigForStartExcludesInvalidArchiveBlock` (`internal/component/config/archive/archive_test.go`) |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior | Permitted Survivors and Why |
|-------|-------------------|-------------------|-----------------------------|
| AC-1 | An archive entry with an `https://` or loopback `http://` location and the new `username` and `password` leaves (plaintext or `$9$` on disk), triggered by commit, manual `ze config archive <name>`, or scheduler | The receiver gets the `POST` with `Authorization: Basic` matching the decoded `username` and `password`, `Content-Type: text/plain`, `X-Archive-Filename`, and the config body holding the `$9$`-encoded password (never plaintext); when the receiver answers 3xx, `archiveClient` follows no redirect and never sends the `Authorization` header to the redirect target. Proven by `TestToHTTP_BasicAuth`, `TestToHTTP_TLSWithBasicAuth`, `TestToHTTP_RedirectDoesNotForwardAuth` (`archive_test.go`), and `test/ui/config-archive-credentials.ci`. | When both `username` and `password` are absent, no `Authorization` header is sent so unauthenticated destinations keep working; `username` alone or `password` alone still sends standard RFC 7617 Basic auth. |
| AC-2 | Inspecting or serializing a config with `system archive <name> password` through `ze config show` (and SSH/web display via `MaskSecrets`), `Serialize`/`SerializeSet`/`ze config fmt`, `ze config dump`, and `gNMI Get` (`EncodeSecrets`) | `ze config show` and `MaskSecrets` display `/* SECRET-DATA */`; `Serialize`, `SerializeSet`, `ze config dump`, and `EncodeSecrets` (used by `gNMI Get`) emit the `$9$`-encoded form; `location` carries no credential; and `RejectMaskedSecretLeaves` refuses `/* SECRET-DATA */` on write. Proven by `TestArchivePasswordSensitiveSurfaces` (`archive_secret_test.go`) and `test/ui/config-archive-credentials.ci`. | `$9$` is reversible offline by `ze secret decode` (Ze's standard `ze:sensitive` design in `yang-config-design.md`), because the uploader must present the credential to the remote HTTP server. |
| AC-3 | A `system archive <name> location` carrying userinfo (`https://u:p@host/path`, `https://token@host/path`, `http://u:p@127.0.0.1/path`, `file://u:p@/path`) | `ze config validate`, editor commit, reload, manual `ze config archive`, `ExtractConfigs`, and `ToHTTP` refuse it; the error redacts the entire userinfo (`<redacted>`, never echoing password or bare token) and names the `username` and `password` leaves as the fix. Proven by `TestValidateArchiveLocation_RefusesUserinfo`, `TestToHTTP_RedactsCredentials`, and `test/parse/cli-config-archive-invalid-refused.ci`. | None: userinfo in `location` is refused on every archive scheme. |
| AC-4 | A `system archive <name> location` using `http://` to a non-loopback host (`http://archive.example.com/configs`, `http://192.0.2.1/configs`, `http://127.0.0.1.example.com/configs`, `http:///path`) or `https://` with no host | `ze config validate`, editor commit, reload, manual `ze config archive`, `ExtractConfigs`, and `ToHTTP` refuse it; the non-loopback `http://` message names `https://` and the loopback hosts (`127.0.0.1`, `::1`, `localhost`). Proven by `TestValidateArchiveLocation_RefusesRemoteHTTP`, `TestDoctorArchiveDestinations_InvalidLocation`, `test/parse/cli-config-archive-invalid-refused.ci`, and `test/ui/config-archive-credentials.ci`. | `http://` to a parsed loopback host in `loopbackFetchHosts` (`127.0.0.1`, `::1`, `localhost`, with or without port) is accepted, matching `ValidateFetchURL`: the upload never leaves the host. |
| AC-5 | Daemon startup (`LoadConfigForStart`) when a config on disk holds one invalid `system archive` entry (remote `http://` or userinfo `location`) beside a valid `system archive` entry | `LoadConfigForStart` succeeds, excludes only the invalid `system/archive[<name>]` entry into `ExcludedBlocks` (with redacted cause), and preserves the valid sibling archive entry so `ze start` boots and reports the excluded entry with its fix via log and `ze doctor`. Proven by `TestLoadConfigForStartExcludesInvalidArchiveBlock` (`archive_test.go`). | None: commit, reload (`LoadConfig`), `ze config validate`, and manual `ze config archive` remain strictly refusing. |
| AC-6 | All repository configs, `.ci` suites, unit tests, and documentation referencing `system archive` | Zero configs, `.ci` bodies, or docs use URL userinfo or remote `http://` for a valid archive destination; `docs/guide/config-archive.md` documents `username`, `password`, the userinfo refusal, and the loopback-only `http://` rule; converted tests run green under `parse` and `ui` functional suites and `config` unit tests. | Refusal test inputs intentionally supply invalid remote `http://` and userinfo URLs to prove they are rejected. |

## End-to-End User Stories

| # | User does | Path through system | Test proving it works |
|---|-----------|--------------------|-----------------------|
| 1 | Configures `system archive offsite` with `location`, `username`, and `password`, then triggers an archive upload | `ze-system-conf.yang` -> `Parser` (`decodeSecretLeaf`) -> `ValidateCustomSections` -> `ExtractConfigs` -> `archiveToLocation` -> `ToHTTP` (`req.SetBasicAuth` on `archiveClient`) -> HTTP receiver | `test/ui/config-archive-credentials.ci` and `TestToHTTP_TLSWithBasicAuth` |
| 2 | Runs `ze config show`, `ze config dump`, and formats/serializes the config holding an archive `password` | `Tree` -> `MaskSecrets` (`/* SECRET-DATA */`) / `EncodeSecrets` & `SensitiveKeys` (`$9$`) -> CLI / dump / gNMI output | `test/ui/config-archive-credentials.ci` and `TestArchivePasswordSensitiveSurfaces` |
| 3 | Attempts to validate or commit `system archive offsite location "https://admin:s3cret@host/configs"` or `"http://archive.example.com/configs"` | `ze config validate` / commit / `ze config archive` -> `ValidateCustomSections` / `handleArchiveTrigger` -> `ValidateArchiveLocation` -> redacted refusal naming fix | `test/parse/cli-config-archive-invalid-refused.ci` and `test/ui/config-archive-credentials.ci` |
| 4 | Upgrades Ze on a router whose existing config still holds a legacy remote `http://` archive block and boots `ze` | `ze start` -> `LoadConfigForStart` -> `excludeInvalidServiceBlocks` (`system/archive[*]`) -> invalid entry dropped, valid sibling kept, `ExcludedBlock` logged + reported by `ze doctor` | `TestLoadConfigForStartExcludesInvalidArchiveBlock` |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestValidateArchiveLocation_Valid` | `internal/component/config/validators_archive_test.go` | `file://`, `https://` with host, and loopback `http://` (`127.0.0.1`, `127.0.0.1:8080`, `[::1]:8080`, `localhost`) pass (AC-4) | |
| `TestValidateArchiveLocation_RefusesUserinfo` | `internal/component/config/validators_archive_test.go` | `https://u:p@h/`, `https://token@h/`, `http://u:p@127.0.0.1/`, `file://u:p@/p` refused; neither password nor bare token appears in error; `username` and `password` named (AC-3) | |
| `TestValidateArchiveLocation_RefusesRemoteHTTP` | `internal/component/config/validators_archive_test.go` | `http://archive.example.com/c`, `http://192.0.2.1/c`, `http://127.0.0.1.example.com/c`, `http:///c`, and `https:///c` refused naming `https://` and loopback hosts (AC-4) | |
| `TestValidateArchiveLocation_Invalid` & `_RedactsCredentials` | `internal/component/config/validators_archive_test.go` | Unsupported/missing/unparseable schemes refused with updated accepted-forms text and password/token redacted (AC-3, AC-4) | |
| `TestArchivePasswordSensitiveSurfaces` | `internal/component/config/archive_secret_test.go` | `system archive <name> password` prints as `$9$` in `Serialize`, `SerializeSet`, and `EncodeSecrets`, masks to `/* SECRET-DATA */` in `MaskSecrets`, is refused as `/* SECRET-DATA */` by `RejectMaskedSecretLeaves`, and decodes back to plaintext on parse while `location` holds no credential (AC-2) | |
| `TestExtractConfigs_CredentialsAndLocationValidation` | `internal/component/config/archive/archive_test.go` | `ExtractConfigs` populates `Username` and decoded `Password` (from `$9$`-parsed tree) and refuses remote `http://` or userinfo `location` naming the entry (AC-1, AC-3, AC-4) | |
| `TestToHTTP_BasicAuth` | `internal/component/config/archive/archive_test.go` | `ToHTTP` sends matching `Authorization: Basic` for username+password and username-only, and omits `Authorization` when both are empty (AC-1) | |
| `TestToHTTP_TLSWithBasicAuth` | `internal/component/config/archive/archive_test.go` | `ToHTTP` over `httptest.NewTLSServer` (`https://`) delivers `Authorization: Basic`, `X-Archive-Filename`, and body (AC-1) | |
| `TestToHTTP_RedirectDoesNotForwardAuth` | `internal/component/config/archive/archive_test.go` | `ToHTTP` with credentials against a 308/307 redirect refuses the upload and sends zero requests (and zero `Authorization` header) to the redirect target (AC-1) | |
| `TestToHTTP_RefusesRemoteHTTPAndUserinfo` | `internal/component/config/archive/archive_test.go` | Direct `ToHTTP` calls with remote `http://` or userinfo fail immediately via `ValidateArchiveLocation` with credentials redacted (AC-3, AC-4) | |
| `TestDoctorArchiveDestinations_InvalidLocation` | `internal/component/config/archive/doctor_test.go` | `checkArchiveDestinations` emits `doctor-archive-invalid` and runs zero HTTP probes when config has remote `http://` or userinfo `location` (AC-3, AC-4) | |
| `TestLoadConfigForStartExcludesInvalidArchiveBlock` | `internal/component/config/archive/archive_test.go` | `LoadConfigForStart` isolates an invalid `system archive` entry (remote `http://` or userinfo) into `ExcludedBlocks` with redacted cause while keeping a valid sibling archive entry (AC-5) | |

### Boundary Tests (numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| `system archive <name> username` length | `1..255` characters | 1 char, 255 chars | 0 chars (empty string) | 256 chars |
| `system archive <name> password` length | `1..4096` characters | 1 char, 4096 chars | 0 chars (empty string) | 4097 chars |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `cli-config-archive-invalid-refused` | `test/parse/cli-config-archive-invalid-refused.ci` | `ze config validate -` refuses `ftp://`, URL userinfo (redacted, naming `username`/`password`), and remote `http://`, and accepts loopback `http://127.0.0.1:8080/configs` and `https://` with `username` and `password` (AC-3, AC-4, AC-6) | |
| `config-archive-credentials` | `test/ui/config-archive-credentials.ci` | Live daemon + loopback receiver proves `ze config archive` sends matching Basic auth and `$9$` body, `ze config show` masks `password` to `/* SECRET-DATA */`, `ze config dump` writes `$9$`, and `ze config archive` / validate refuse userinfo and remote `http://` (AC-1, AC-2, AC-3, AC-4) | |

### Interop Tests (Scope: protocol)
N-A: Scope is `config` (no BGP/IGP/IKE/L2TP wire protocol peer).

## Files to Modify
- `internal/component/config/system/yang/ze-system-conf.yang` - add `username` (`length "1..255"`) and `password` (`length "1..4096"`, `ze:sensitive`) leaves under `list archive`; update `location` description and `ze:help`.
- `internal/component/config/validators_archive.go` - refuse userinfo (redacting via `redact.URL`, naming `username` and `password`), refuse empty host on `https://`, and refuse `http://` outside `loopbackFetchHosts`; update `archiveLocationForms`.
- `internal/component/config/validators_archive_test.go` - update valid/invalid cases and add userinfo, bare-token, remote `http://`, loopback lookalike, and empty-host tests.
- `internal/component/config/archive/archive.go` - add `Username` and `Password` to `ArchiveConfig`, extract them and validate non-empty `Location` in `ExtractConfigs`, pass `*ArchiveConfig` into `archiveToLocation` and `ToHTTP`, validate `Location` in `archiveToLocation`/`ToHTTP`, and set `req.SetBasicAuth` when credentials are present.
- `internal/component/config/archive/scheduler.go` - update `fireAll` and `fireByTrigger` calls to pass `&ac` to `archiveToLocation`.
- `internal/component/config/archive/register.go` - register `system/archive[*]` with `config.RegisterServiceBlock` in `init()`.
- `internal/component/config/archive/archive_test.go` - update `ToHTTP` calls, replace remote `http://` in `TestToHTTP_Unreachable` with `https://`, update `TestToHTTP_RedactsCredentials`, and add Basic auth, TLS, redirect-no-auth, `ExtractConfigs`, and `LoadConfigForStart` exclusion tests.
- `internal/component/config/archive/doctor_test.go` - add `TestDoctorArchiveDestinations_InvalidLocation`.
- `test/parse/cli-config-archive-invalid-refused.ci` - update accepted-forms assertion and add userinfo, remote `http://`, loopback `http://`, and credential-leaf validation cases.
- `docs/guide/config-archive.md` - document `username` and `password` (`ze:sensitive`), update `Location Schemes` to loopback-only `http://`, update `Validation` for userinfo and remote `http://` refusals, and add an authenticated HTTPS example.

## Files to Create
- `internal/component/config/archive_secret_test.go` - unit tests proving `ze:sensitive` `$9$` encoding, `MaskSecrets` masking, `RejectMaskedSecretLeaves`, and length bounds on `system archive <name> username` and `password`.
- `internal/test/fixture/config_archive_credentials_fixture.go` - functional fixture driving a live `ze` daemon and local receiver for AC-1, AC-2, AC-3, and AC-4.
- `internal/test/fixture/register_config_archive_credentials.go` - registers the `ui/config-archive-credentials` fixture.
- `test/ui/config-archive-credentials.ci` - functional test invoking `le test fixture ui/config-archive-credentials $PORT`.

### Integration Checklist
| Integration Point | Applies? | File / reason |
|-------------------|----------|---------------|
| YANG schema (new RPCs/config) | Yes | `internal/component/config/system/yang/ze-system-conf.yang` (`username` and `password` leaves under `list archive`); verify `./le yang glue check`. |
| YANG validation constraints | Yes | `username` carries `length "1..255"`; `password` carries `length "1..4096"` and `ze:sensitive`. |
| YANG custom validators | Yes | Extends existing `ze:validate "archive-location"` (`internal/component/config/validators_archive.go`, already registered in `validators_register.go` under `system` in `validatedSections`). |
| CLI commands/flags | No | No new CLI command or flag; existing `ze config archive <name>` and `request config archive <name>` reuse `ValidateArchiveLocation` and `ExtractConfigs`. |
| CLI grammar (keyword before value) | N-A | No new CLI command added. |
| Editor autocomplete | Yes | Automatic from YANG (`username` and `password` under `system archive <name>`); no custom `CompleteFn` needed for free-form credentials. |
| Functional test for new RPC/API | N-A | No new RPC/API added; existing archive path is covered by `test/ui/config-archive-credentials.ci` and `test/parse/cli-config-archive-invalid-refused.ci`. |
| Pipe completeness | N-A | No new operational command or renderer output added. |
| Env var registration | N-A | No environment variables added (credentials are per-archive YANG leaves). |
| Doctor check for runtime dependencies | No | Existing `archive-destinations` doctor check (`internal/component/config/archive/doctor.go`) already covers archive locations and emits `doctor-archive-invalid` when `ExtractConfigs` fails; covered by `TestDoctorArchiveDestinations_InvalidLocation`. |
| Doctor code per problem class | Yes | Refusals in `ValidateArchiveLocation` state problem, why, and fix; invalid archive entries in `ze doctor` emit `doctor-archive-invalid` (existing code in `archive/doctor_codes.go`); start-time exclusions reuse `cmd/ze/hub/service_not_started.go`'s excluded-block doctor code. |
| Prometheus counters/metrics | N-A | No new metrics; archive upload state is not metric-instrumented by this fix. |
| BGP family surface (new SAFI / capability / attribute) | N-A | Scope is `config`, not BGP. |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature, or a feature's scope, evidence or level changed? | No | Security fix to existing config archive feature; no `features/*.md` maturity change. |
| 2 | Config syntax changed? | Yes | `docs/guide/config-archive.md` (new `username` and `password` leaves, loopback-only `http://`, userinfo refusal). Checked `docs/guide/configuration.md` (line 4265 mentions "archives" only as a consumer of `system host`, no archive syntax) and `docs/architecture/config/syntax.md` (no archive example) — both unaffected. |
| 3 | CLI command added/changed? | No | `docs/guide/command-reference.md` line 438 (`ze config archive <name>`) syntax is unchanged. |
| 4 | API/RPC added/changed? | No | `ze-config:archive-trigger` input/output unchanged. |
| 5 | Plugin added/changed? | No | `archive` is a config subpackage, not a new plugin. |
| 6 | Has a user guide page? | Yes | `docs/guide/config-archive.md` (updated for `username`, `password`, loopback-only `http://`, and userinfo refusal). |
| 7 | Wire format changed? | No | No BGP/IGP/IPC wire format changed; HTTP upload still uses standard RFC 7617 Basic auth. |
| 8 | Plugin SDK/protocol changed? | No | No `pkg/plugin/` changes. |
| 9 | RFC behavior implemented, changed, or newly proven? | N-A | Scope is `config`, not a tracked protocol RFC in `rfc/short/`. |
| 10 | Test infrastructure changed? | No | Uses existing `.ci` runner and `internal/test/fixture` registration pattern. |
| 11 | Affects daemon comparison? | No | No change to `docs/comparison.md`. |
| 12 | Internal architecture changed? | No | Reuses existing `ze:sensitive`, `loopbackFetchHosts`, `archiveClient`, and `config.RegisterServiceBlock`. |
| 13 | Route metadata keys added/changed? | N-A | No route metadata involved. |
| 14 | Prometheus counters added/changed? | N-A | No metrics changed. |
| 15 | Registered plugin, event type, send type, command, capability, or inventory changed? | No | No new plugin, event type, command, or capability. |
| 16 | Any changed source file referenced by existing doc source anchors? | Yes | Derived via `./le spec citation anchors`: `docs/guide/config-archive.md` (updated — declared `// Design:` page of `validators_archive.go`, `archive.go`, `scheduler.go`, `register.go`); the 7 pages anchored to `ze-system-conf.yang` are checked and unaffected: `docs/architecture/core-design.md` (anchors `ze-system-conf.yang` for update backends), `docs/architecture/resolve.md` (anchors `system/rir/delegation-source` and `system/dns`), `docs/config-reference.md` (anchors `system/update-check/url` only), `docs/features/dns-resolver.md` (anchors `system/dns` and `system/name-server`), `docs/guide/cli.md` (anchors system identity), `docs/guide/command-reference.md` (line 438 `ze config archive <name>` unchanged), `docs/guide/migrating-from-vyos.md` (line 275 `system archive <name> location <url>` mapping still accurate). |
| 17 | Existing docs show config/CLI/API examples for this area? | Yes | `docs/guide/config-archive.md` (only page with `system archive` config blocks — updated in this spec). |

## Design presentation

### What changes

HTTP Basic auth credentials for `system archive <name>` move out of the `location` URL into dedicated `username` and `password` leaves under `list archive`, with `password` marked `ze:sensitive` so config display surfaces (`ze config show`, SSH `show`, web UI) mask it as `/* SECRET-DATA */` and serialized/dumped/gNMI config outputs write it in `$9$` form. `ValidateArchiveLocation` now refuses any `location` containing URL userinfo (redacting the URL and pointing operators to `username` and `password`), refuses `https://` without a host, and refuses `http://` to any host outside the shared `loopbackFetchHosts` set (`127.0.0.1`, `::1`, `localhost`) so configs with reversible `$9$` secrets are never posted over unencrypted remote HTTP. `ToHTTP` sends Basic auth from those leaves exclusively through the non-redirecting `archiveClient`. To honor the 2026-11-26 "Ze always starts" ruling, `system/archive[*]` registers as a start-time service block so a router upgraded with a legacy remote-HTTP or userinfo archive block still boots while commit, reload, `ze config validate`, and `ze config archive` refuse it.

### Before and after

1. Configuring archive authentication:
   - Before: operator wrote `location "https://backup:s3cret@archive.example.com/configs";` — `s3cret` was stored in clear in the config file, printed verbatim by `ze config show` and the web UI, and uploaded in clear inside every archived config copy.
   - After: operator writes `location "https://archive.example.com/configs";`, `username "backup";`, and `password "s3cret";` — `ze config show` and the web UI print `password /* SECRET-DATA */;`, `ze config fmt`/`dump`/`gNMI Get` and the archived copy hold `password "$9$...";`, and `ToHTTP` sends the decoded credential in the `Authorization: Basic` header over HTTPS with no redirect following.
2. Refusing URL credentials in `location`:
   - Before: `ze config validate` accepted `location "https://backup:s3cret@archive.example.com/configs";` with exit code 0.
   - After: `ze config validate` exits 1 with `system/archive[offsite]/location: archive location "https://<redacted>@archive.example.com/configs" carries credentials in the URL, which stores and displays them in clear; move them to the archive username and password leaves (set system archive <name> username <user> and set system archive <name> password <password>)` (never echoing `s3cret`).
3. Refusing remote plain HTTP in `location`:
   - Before: `ze config validate` accepted `location "http://archive.example.com/configs";` with exit code 0, posting the full config and `$9$` secrets in cleartext.
   - After: `ze config validate` and `ze config archive` exit 1 with `system/archive[offsite]/location: archive location "http://archive.example.com/configs" uploads to archive.example.com over plain http, exposing the config and its reversible $9$ secrets; use https://, or http:// to a loopback host (127.0.0.1, ::1, localhost)`. Loopback `http://127.0.0.1:8080/configs` continues to pass.
4. Booting with a legacy invalid archive entry:
   - Before: an invalid archive entry in `ze.conf` failed `LoadConfigForStart` and prevented `ze start` from booting.
   - After: `LoadConfigForStart` excludes only the invalid `system/archive[<name>]` block, boots all other archive blocks and networking, and reports the excluded entry with its fix in the log and `ze doctor`.

### Alternatives set aside

| Alternative | Why set aside |
|-------------|---------------|
| Mark `leaf location` itself `ze:sensitive` | `SensitiveKeys` collects sensitive leaf names across the whole schema, so marking `location` sensitive would mask every leaf named `location` in every module, and would also hide the destination hostname/path from operators inspecting their config. |
| Strip userinfo silently in `ToHTTP` or upgrade `http://` to `https://` automatically | Violates `ai/rationale/exact-or-reject.md`: Ze never silently approximates or rewrites operator intent; invalid forms are refused at verify with the exact fix. |
| Duplicate a second loopback host list inside `validators_archive.go` | Risks drift between `ValidateFetchURL` and `ValidateArchiveLocation`, the exact problem class recorded in `plan/journal/guard-added-to-one-half-of-a-pair.md`. Reusing `loopbackFetchHosts` keeps one source of truth. |
| Allow a new `--insecure-http` flag or YANG escape hatch for remote plain HTTP | Uploads carry the router's entire configuration including reversible `$9$` secrets and Basic auth credentials; plain HTTP to remote hosts is a P0 security defect with no safe remote use case. |
| Require a `ze-system-conf.yang` `container authentication { mandatory username; mandatory password; }` wrapper | `list archive` keeps all its fields flat (`location`, `filename`, `timeout`, `trigger`, `on-change`), VyOS T6304 uses flat `username`/`password` leaves, and optional flat leaves allow Basic auth with a username-only or token-only credential (also offered as Choice C-1 for Thomas). |

### Documents departed from

1. `docs/guide/config-archive.md`, section `Location Schemes`: currently documents `http://host/path` as `"HTTP POST with \`text/plain\` content type. Filename sent in \`X-Archive-Filename\` header."` with no host restriction. After this work, the table states `http://127.0.0.1/path` (and `::1`, `localhost`) is allowed only for loopback hosts, and remote uploads require `https://host/path`.
2. `docs/guide/config-archive.md`, section `Validation`: currently documents `location` refusal as `"Its scheme is not \`file\`, \`http\` or \`https\`, or it has no scheme"` and implies URL-embedded credentials in `"A refused location never echoes the password of a URL with credentials in it."` After this work, `Validation` documents that `location` is also refused when it carries URL userinfo (directing operators to `username` and `password`) or uses `http://` to a non-loopback host, while continuing to redact any credentials present in a refused URL.

### Choices for Thomas

| # | Choice | Options | Decision |
|---|--------|---------|----------|
| C-1 | YANG placement of archive credential leaves | (a) Flat `username` and `password` leaves directly under `list archive` (recommended — matches `list archive`'s flat layout and VyOS T6304, and allows username-only or token-only Basic auth); (b) Nested `container authentication { presence; mandatory username; mandatory password; }` (matches `container registry` and `pppoe-client`) | |
| C-2 | Setting `username` or `password` on a `file://` archive entry | (a) Leave `username`/`password` scheme-independent and ignored on `file://` (recommended — matches `timeout`, which `file://` already ignores in `test/parse/cli-config-system-archive.ci`); (b) Refuse `username` or `password` when `location` is `file://` via an original-tree validator | |

## Implementation Steps

1. **Phase 1: Wiring (MANDATORY FIRST)** — add YANG schema leaves, service-block registration, and failing wiring/unit tests
   - Tests: `TestValidateArchiveLocation_RefusesUserinfo`, `TestValidateArchiveLocation_RefusesRemoteHTTP`, `TestArchivePasswordSensitiveSurfaces`, `TestLoadConfigForStartExcludesInvalidArchiveBlock`, and `test/ui/config-archive-credentials.ci`
   - Files: `internal/component/config/system/yang/ze-system-conf.yang`, `internal/component/config/archive/register.go`, `internal/component/config/validators_archive_test.go`, `internal/component/config/archive_secret_test.go`, `internal/test/fixture/config_archive_credentials_fixture.go`, `internal/test/fixture/register_config_archive_credentials.go`, `test/ui/config-archive-credentials.ci`
   - Verify: new schema parses (`./le yang glue check`), wiring tests compile and fail because `ValidateArchiveLocation` still accepts userinfo/remote `http://` and `ToHTTP` does not yet send `Authorization: Basic` from the new leaves.
2. **Phase 2: Validator hardening (`ValidateArchiveLocation`)** — refuse userinfo, empty `https://` host, and non-loopback `http://`
   - Tests: `TestValidateArchiveLocation_Valid`, `TestValidateArchiveLocation_RefusesUserinfo`, `TestValidateArchiveLocation_RefusesRemoteHTTP`, `TestValidateArchiveLocation_Invalid`, `TestValidateArchiveLocation_RedactsCredentials`, `TestArchivePasswordSensitiveSurfaces`, `test/parse/cli-config-archive-invalid-refused.ci`
   - Files: `internal/component/config/validators_archive.go`, `internal/component/config/validators_archive_test.go`, `test/parse/cli-config-archive-invalid-refused.ci`
   - Verify: validator unit tests and `cli-config-archive-invalid-refused.ci` turn green; `ValidateFetchURL` tests remain green.
3. **Phase 3: Uploader credentials, defense-in-depth, and doctor/start isolation** — wire `Username`/`Password` through `ArchiveConfig`, `ExtractConfigs`, `archiveToLocation`, `ToHTTP`, and `Scheduler`
   - Tests: `TestExtractConfigs_CredentialsAndLocationValidation`, `TestToHTTP_BasicAuth`, `TestToHTTP_TLSWithBasicAuth`, `TestToHTTP_RedirectDoesNotForwardAuth`, `TestToHTTP_RefusesRemoteHTTPAndUserinfo`, `TestDoctorArchiveDestinations_InvalidLocation`, `TestLoadConfigForStartExcludesInvalidArchiveBlock`, `test/ui/config-archive-credentials.ci`
   - Files: `internal/component/config/archive/archive.go`, `internal/component/config/archive/scheduler.go`, `internal/component/config/archive/archive_test.go`, `internal/component/config/archive/doctor_test.go`
   - Verify: all unit tests in `internal/component/config/...` pass with `-race`, and `test/ui/config-archive-credentials.ci` passes.
4. **Phase 4: Documentation and final checks** — update `docs/guide/config-archive.md` and run doc/yang/functional checks
   - Tests: `./le doc check all`, `./le yang glue check`, `test/parse/cli-config-archive-*.ci`, `test/ui/config-archive-*.ci`, `test/ui/doctor-update-archive.ci`
   - Files: `docs/guide/config-archive.md`
   - Verify: every anchor and doc check is green, zero remote `http://` or userinfo archive examples remain in docs or tests.

### Critical Review Checklist
| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | Every AC-1..AC-6 has an implementation at `file:line` and a non-vacuous test |
| Feature completeness | Every user story (1-4) runs end-to-end with no broken link between YANG, parser, validator, extractor, uploader, and display masks |
| Correctness | `ValidateArchiveLocation` uses `parsed.Hostname()` against `loopbackFetchHosts` (never `strings.HasPrefix`), refuses bare-token userinfo (`https://tok@host/`), refuses empty host on `https://`, and sets `req.SetBasicAuth` only when `Username != "" \|\| Password != ""` |
| Naming | YANG leaves are `username` and `password`; Go struct fields are `Username` and `Password` (with `//nolint:gosec` G117 comment explaining config field, masked via `ze:sensitive`, never logged) |
| Data flow | `$9$` decoding happens once in `config` parser (`decodeSecretLeaf`), never duplicated in `archive.ExtractConfigs`; `archiveClient` remains the sole HTTP client for uploads and HEAD probes |
| Operator errors and doctor codes | Every refusal in `ValidateArchiveLocation` states problem, why, and fix, redacts userinfo via `redact.URL`, and `checkArchiveDestinations` emits `doctor-archive-invalid` on invalid locations without probing |
| Rule: `ai/rules/config.md` & `docs/architecture/core-design.md` (2026-11-26) | `system/archive[*]` is registered via `config.RegisterServiceBlock` so `ze start` isolates a broken archive entry rather than failing boot, while commit/reload/validate refuse strictly |

### Deliverables Checklist
| Deliverable | Verification method |
|-------------|---------------------|
| `username` and `password` (`ze:sensitive`) leaves in `ze-system-conf.yang` | `./le yang glue check` and `TestArchivePasswordSensitiveSurfaces` |
| Userinfo and non-loopback `http://` refusals in `ValidateArchiveLocation` | `./le test unit pkg path ./internal/component/config/ run ValidateArchiveLocation` |
| Basic auth upload on `archiveClient` + `ExtractConfigs` + `system/archive[*]` start isolation | `./le test unit pkg path ./internal/component/config/archive/` |
| End-to-end validation and upload/masking `.ci` proofs | `./le test functional parse` (archive cases) and `./le test functional ui` (`config-archive-credentials`, `config-archive-prune`, `doctor-update-archive`) |
| Updated user guide with valid source anchors | `./le doc check all` |
| Zero remaining unmigrated remote `http://` or userinfo archive configs in repo | `git grep -n "location.*http\|location.*@" -- test/ docs/ etc/` |

### Security Review Checklist
| Check | What to look for |
|-------|-----------------|
| Credential leakage in refusals/logs | Both password (`u:p@h`) and bare-token (`tok@h`) userinfo are scrubbed by `redact.URL` and `redact.URLError` in every refusal; `ac.Password` is never logged or formatted into error messages |
| Credential leakage on display/export surfaces | `password` carries `ze:sensitive` so `MaskSecrets` renders `/* SECRET-DATA */` (CLI/web) and `EncodeSecrets`/`SensitiveKeys` render `$9$` (file/dump/gNMI); `RejectMaskedSecretLeaves` rejects `/* SECRET-DATA */` on write |
| Credential forwarding on redirects | `ToHTTP` sends only through `archiveClient` (`CheckRedirect: refuseArchiveRedirect`), proven by `TestToHTTP_RedirectDoesNotForwardAuth` |
| Loopback bypass / lookalike host | `ValidateArchiveLocation` checks `slices.Contains(loopbackFetchHosts, parsed.Hostname())`, never `strings.HasPrefix`, refusing `http://127.0.0.1.example.com/` and `http://localhost.evil.com/` |
| Defense in depth on direct callers | `ExtractConfigs`, `archiveToLocation`, and `ToHTTP` all invoke `config.ValidateArchiveLocation` so bypassing `ValidateCustomSections` still cannot upload over remote `http://` or send URL userinfo |

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

- Reading `docs/architecture/core-design.md`'s 2026-11-26 decision and `internal/component/config/service_blocks.go` revealed that `system/archive[*]` was not registered as a start-time service block; tightening `ValidateArchiveLocation` without registering `system/archive[*]` would have caused `ze start` to refuse booting on upgraded routers that still hold a legacy remote `http://` archive location.
- Reading `internal/component/cli/editor_mask.go`, `internal/component/web/secret.go`, and `internal/component/config/secret_text.go` clarified the exact split of `ze:sensitive` across surfaces (`/* SECRET-DATA */` on CLI/web display vs `$9$` on file/dump/gNMI) so AC-2 tests the real producer contract.

## Owned journal rows

| Class file | Date | Surface (short) |
|------------|------|-----------------|
| `secret-echoed-to-the-client.md` | 2026-11-08 | archive `location` userinfo stored and shown in clear |
| `guard-added-to-one-half-of-a-pair.md` | 2026-11-08 | `ValidateArchiveLocation` accepts remote `http://` that `ValidateFetchURL` refuses |

## Key Design Decisions

| Decision | Alternatives Considered | Rationale |
|----------|------------------------|-----------|
| Flat `username` and `password` (`ze:sensitive`) leaves directly under `list archive` | Mark `location` sensitive; nested `container authentication` | `SensitiveKeys` keys by leaf name so marking `location` sensitive breaks other `location` leaves and hides the URL; `password` is already sensitive across Ze and redacted by `redact.Command`; flat leaves match `list archive` and VyOS T6304 while allowing optional username-only/password-only Basic auth |
| Reuse `loopbackFetchHosts` from `validators.go` via `parsed.Hostname()` in `ValidateArchiveLocation` | Copy loopback list into `validators_archive.go`; prefix matching | One shared slice prevents fetch/upload policy drift (`guard-added-to-one-half-of-a-pair.md`), and `parsed.Hostname()` blocks lookalike domains such as `127.0.0.1.example.com` |
| Pass `*ArchiveConfig` to `archiveToLocation` and `ToHTTP` and call `ValidateArchiveLocation` in `ExtractConfigs`, `archiveToLocation`, and `ToHTTP` | Validate only in YANG validator; pass 7 positional arguments to `ToHTTP` | Avoids `hugeParam` lint warnings, makes forgetting credentials impossible, and gives defense-in-depth for `ze doctor` (`doctor-archive-invalid`) and direct Go callers |
| Register `system/archive[*]` via `config.RegisterServiceBlock` in `archive/register.go` | Leave `system/archive` unscoped at `LoadConfigForStart` | Satisfies the 2026-11-26 "Ze always starts" ruling so a router upgraded with a legacy remote `http://` or userinfo archive URL boots with networking and reports the excluded block via log and `ze doctor` |
| Keep `ze doctor` HEAD probe unauthenticated | Send `username`/`password` on `probeArchiveDestination` HEAD requests | Passive reachability probes count any non-redirect HTTP status (including 401/404) as reachable; omitting credentials reduces exposure while `ExtractConfigs` still flags invalid locations as `doctor-archive-invalid` |

## Known Limitations

- `ze:sensitive` uses JunOS-compatible `$9$` obfuscation (reversible offline via `ze secret decode`) rather than an external KMS/vault, because the HTTP archive uploader must recover the plaintext credential to build the `Authorization: Basic` header; filesystem permissions (`0600`) protect the config file at rest, as documented for every `ze:sensitive` leaf in Ze.
- Only HTTP Basic authentication is supported for HTTP(S) archive destinations; bearer token or mTLS client-certificate archive authentication is out of scope for this P0 security fix.
- `ze doctor` HEAD probes verify reachability and non-redirect behavior without validating whether the configured `username`/`password` are accepted by the remote server (which a manual `ze config archive <name>` verifies).

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
- [ ] AC-1..AC-6 all demonstrated
- [ ] Every user story has a working path and a passing test
- [ ] Wiring Test table complete: every row a concrete test name, none deferred
- [ ] The targeted tests for what changed pass, run in the main checkout by the main thread: the scoped package tests, the component groups and functional suites `./le repo changed scope print both` reaches. No agent runs any `./le verify` command: GitHub CI verifies the pushed commit, and the full verification is Thomas's release check. A red CI reports gets a fix commit
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
- [ ] Interop tests for protocol features (N-A: Scope is `config`, no protocol peer)

### Closure
- [ ] Append `plan/templates/TEMPLATE-CLOSURE.md` and complete every section in it
- [ ] `/ze-review` gate clean, recorded via `internal/le/spec/review.go`
- [ ] Any lesson routed to its governing surface under `ai/rules/planning.md`; no lesson artifact created merely for closure
- [ ] **Commit A:** code + tests + docs + edited spec + any journal rows owed by the work
- [ ] **Commit B:** `remove plan/release/fixes/spec-config-archive-location-credentials.md` only, in the same `./le commit create` script (commit A preserves the spec in history)
