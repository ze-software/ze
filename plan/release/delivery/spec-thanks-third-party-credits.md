# Spec: thanks-third-party-credits

| Field | Value |
|-------|-------|
| Status | in-progress |
| Scope | cli |
| Priority | P2 |
| Release reason | A released `ze` binary redistributes third-party code whose licences require their copyright notice and conditions to travel with the binary. Today the binary carries none of them, and `NOTICE` and the website legal page claim a record that does not exist for the web assets |
| RFC gaps | - |
| Depends | - |
| Phase | 1/7 |
| Handoff | - |
| Updated | 2026-11-20 |

Recovery after compaction: `.claude/rules/post-compaction.md`.

## Task

Ze is built on other people's work, and nothing a user can run says so. The
goal is a thank-you that is also Ze's licence record: `ze thanks` lists every
third-party work compiled into the binary that answers, each with its version,
licence, copyright holder and the reason Ze uses it, and `ze thanks licence`
prints the licence texts, which the binary carries. The website's Dependencies
page, which already lists what Ze uses and why, gains the licence information
and stays the one curated list: the binary carries a generated copy of it.

Thomas's words: "create the thanks command line as well, same thing for the web
interface. Where we list all our dependencies, effectively thank you: here are
the things we use and the copyright licence thereunder. It is to make sure that
we are listing all the dependencies we have and the licence." The command is
`ze thanks`. On the web: "We already have a dependency webpage which lists what
we use, we should add licence information to it."

His decisions on 2026-11-20:

| Choice | Decision |
|--------|----------|
| Which dependencies | What ships: Go's own library, the third-party Go modules linked into a shipped `ze`, the web assets served to browsers, and code copied or ported from another project. Development tooling and lab software are thanked on the website page only, never by the binary |
| How much each credit carries | Name, version, licence, copyright holder and the why sentence in the list; the full licence text of each shipped work compiled into the binary |
| The appliance image | A spec of its own, `plan/release/delivery/spec-appliance-image-third-party-notices.md`: BusyBox, the boot code, the firmware and the gokrazy binaries the image carries beside `ze` |
| The router's web interface | No dedicated page. It reaches the daemon form of the command through the pages that already run any command |
| The website Dependencies page | Gains licence and copyright for each curated module, and a generated section for every other work the shipped binary carries |
| Where the curated list lives | It stays `website/data/dependencies.json`. The binary carries a generated copy |
| Spelling | `ze thanks` and `ze thanks licence` |

The list is complete by construction: a work that ships without a credit, or a
credit without its licence text, is refused by a check, never found by a
reader.

## Required Reading

### Architecture Docs
- [ ] `ai/doors/system.md` - the design every subsystem shares
  → Constraint: a command is declared once and reaches every interface; a local-data command (`command.MustRegisterLocalData`) is answered in the caller's own process and served by the daemon dispatcher under its YANG path
  → Constraint: code reaches a gated feature only through a registry the feature fills from its own `init()`; always-on code never imports a gated package
- [ ] `ai/doors/commands-cli.md` - command model and its registries
  → Constraint: a handler answers data, never text; a text answer is a `command.Blob` with shape `command.ShapeBlob`
  → Constraint: an offline root that is not a verb has no YANG node at its own path; its registration names its daemon form in `registry.Meta.DaemonPath` (`ze doctor` names `show doctor`)
- [ ] `ai/doors/core.md` - the leaf tier
  → Constraint: a core-leaf registry copies the shape of `internal/core/ipsecinventory`: standard library imports only, `Register` from `init()`, value-type records, and an explicit error, never an empty list, when it cannot answer
- [ ] `ai/doors/system-plugins.md` - edge plugins
  → Decision: a command-only plugin under `internal/plugins/` owns its YANG and handlers and delegates the work to a core package; `internal/plugins/env` over `internal/core/env` is the pattern
- [ ] `ai/doors/le-tooling.md` - development tooling
  → Constraint: a shipped `ze` never links an `internal/le` package; an `le` tool sits at the path its command name predicts (`le deps credits` is `internal/le/deps/credits`)
- [ ] `ai/doors/api-surfaces.md` - web, looking glass, REST
  → Constraint: the looking glass (`ze_lg`) and the REST documentation page (`ze_rest`) serve third-party assets on their own listeners and exist in a build without `ze_web`
- [ ] `ai/rules/cli.md` - grammar
  → Constraint: "A command whose action enumerates the members of a set MUST spell that action `list`", so the daemon form of the list is `show thanks list`
- [ ] `docs/architecture/api/commands.md`, "Design decisions" - the pipe ruling
  → Constraint: "The pipe wins: a command path names the subject and stops; scope, presentation and format are pipe operations after it, selector words included", so one work is selected with a pipe operation named after the field it selects on
- [ ] `website/AI.md` - the website's structure and owner rulings
  → Constraint: `website/data/dependencies.json` is the curated declaration; a site build runs no check and reads committed data only
- [ ] `ai/META-DECISIONS.md`, "Website design" and "Code and dependencies"
  → Constraint: website work is tested once, at closure, after Thomas approved the pages; while he reviews, the page is rendered alone into `../gh-pages` and nothing is committed
  → Constraint: a new third-party module lands in a commit of its own (`go.mod`, `go.sum`, `vendor/`), before the commit that first uses it; the credit is owed by the commit that first links it

**Key insights:** (minimal context to resume after compaction)
- The binary's build information lists every linked module with its version, so the modules a binary credits are derived at run time, never listed by hand in Go
- The Linux distro build links 97 third-party modules and the appliance build 84 (`go list -deps` per flavor, 2026-11-20); the macOS development binary links 72. The count differs per flavor, so the generated data holds the union and the answer filters by what the binary links
- The website's curated list holds direct dependencies only, development tooling included; the shipped set adds about 30 indirect modules that have no why sentence
- No licence text for htmx, uPlot or swagger-ui is in the repository, and the swagger-ui bundle's own companion licence file was never vendored
- The Go licence text is not in the repository
- The MPL-2.0 files of `github.com/cyphar/filepath-securejoin` are linked into the appliance build

## Current Behavior (MANDATORY)

**Source files read:** (must read BEFORE you write this spec)
- [ ] `internal/core/version/version.go` - `readInfo` reads `debug.ReadBuildInfo()` for the Go version and VCS settings only; it never reads `Deps`
- [ ] `internal/component/support/support.go` - `collectVersion` reads `Deps` and answers each module path and version in the support bundle; no licence
- [ ] `internal/le/site/dependencies.go` - `renderDependencies` reads `website/data/dependencies.json` (categories of module plus why), takes versions of direct requirements from `go.mod` (`directModuleVersions`), refuses drift in both directions (`checkDependencyDrift`), and writes the page and its Markdown mirror with columns Module, Version, Why we use it. `dependenciesLead` states the count of curated modules
- [ ] `internal/le/site/facts.go` - publishes the dependency figure as the count of `go.mod`'s direct requirements (`facts.Dependencies = len(versions)`); it does not read the data file
- [ ] `internal/le/site/llmsdata.go` - `loadLLMSInputs` reads the published copy of the data file into `dependencyData` for `llms.txt`
- [ ] `internal/le/site/derived.go` - `writeLLMSDependencies` writes "## Dependency rationale": one line per curated module with its why sentence
- [ ] `internal/component/api/rest/server.go` - the documentation routes serve the two swagger-ui files by name, each behind `s.withAuth`: "Documentation follows the same auth policy as the API it describes."
- [ ] `internal/plugins/help/register.go` - two local-data commands at the offline root `help`, each naming its daemon form in `DaemonPath`; one answers rows, one a `command.Blob`
- [ ] `internal/component/doctor/register.go` - `registry.RegisterRoot("doctor", ...)` beside `command.MustRegisterLocalData("doctor", ..., registry.Meta{DaemonPath: "show doctor"}, ...)`
- [ ] `internal/plugins/env/register.go` - an edge plugin listing a core registry: `MustRegisterLocalData`, `RegisterShape(..., command.ShapeTab)`, `RegisterColumns`
- [ ] `internal/le/web/vendor/sync.go` - the `assets` and `targetedAssets` tables decide which vendored web files reach which consumer; `consumers` is a written policy
- [ ] `internal/le/web/vendor/vendorweb.go` - `vendorPackages` takes every file of a directory under `third_party/web/` as part of that package and refuses one file name in two directories ("is vendored twice")
- [ ] `internal/le/web/vendor/check.go` - the drift comparison needs no network; `extractVersionFromManifest` is the only reader of `MANIFEST.md`
- [ ] `internal/le/commit/dependencies.go` - `dependencyRefusal` is the commit gate on third-party imports: it judges added imports against the ledger and reads `go.mod` and `vendor/modules.txt` as the commit leaves them
- [ ] `internal/test/localdatacoverage/localdatacoverage.go` - `Evidence` holds one invocation per local-data command, `doctor` and `help command` included
- [ ] `internal/component/web/render.go` - `//go:embed assets` embeds the whole asset directory, so a file synced there is served
- [ ] `internal/component/lg/embed.go` - `//go:embed assets`, the same
- [ ] `internal/component/api/rest/embed.go` - embeds two named swagger-ui files, the stylesheet and the bundle; a further file needs its own embed line and route. The bundle's first line reads "For license information please see swagger-ui-bundle.js.LICENSE.txt"
- [ ] `internal/plugins/container/seccomp/profile.go` - `DefaultProfile` carries the rules of Moby's default seccomp profile, "source, commit and digest in third_party/moby-seccomp-profile/README.md"
- [ ] `internal/appliance/cmd_iso.go` - imports `internal/thirdparty/fat`, a copy of gokrazy's FAT writer with its own `LICENSE`
- [ ] `internal/exabgp/bridge/bridge_event.go` - the ExaBGP event translation; its tables are transcribed from ExaBGP's source
- [ ] `internal/plugins/trafficusage/trafficusage.go` - "matching the upstream lan-bandwidth-exporter it is ported from"
- [ ] `internal/core/ipregistry/registry.go` - embeds "the canonical IANA XML shipped with Ze"
- [ ] `internal/core/portname/services_table.go` - "Derived from /etc/services"
- [ ] `NOTICE` - states "Third-party code under vendor/ and third_party/ keeps its own copyright and license, recorded beside it."
- [ ] `website/legal/legal.md` - states "Third-party code and fonts that Ze includes keep their own copyright and licences, which are recorded beside them in the repository."
- [ ] `third_party/web/MANIFEST.md` - asset, version, source and date per vendored web file; no licence
- [ ] `vendor/github.com/miekg/dns/LICENSE` - "Redistributions in binary form must reproduce the above copyright notice, this list of conditions and the following disclaimer in the documentation and/or other materials provided with the distribution."
- [ ] `vendor/github.com/cyphar/filepath-securejoin/LICENSE.MPL-2.0` - section 3.2(a): "You must inform recipients of the Executable Form how they can obtain a copy of such Source Code Form by reasonable means in a timely manner"
- [ ] `vendor/github.com/google/nftables/LICENSE` - Apache-2.0 section 4(d) requires "a readable copy of the attribution notices contained within such NOTICE file", in one of three places, the third being "within a display generated by the Derivative Works, if and wherever such third-party notices normally appear"

**Behavior to preserve:** (unless the user explicitly said to change it)
- The website Dependencies page keeps its groups, their order, the why sentences, its filter box and its route `/reference/dependencies/`
- `checkDependencyDrift` keeps refusing a direct requirement with no curated entry and a curated entry `go.mod` no longer requires
- The published dependency figure stays the count of `go.mod`'s direct requirements, and the page's lead sentence stays the count of curated direct modules
- `show version` and the support bundle answer as they do today
- `/etc/linux-gpl-notice` in the appliance image is untouched
- The dependency ledger and `./le deps approve` and `./le deps check` are untouched

**Behavior to change:** (only what the user asked for)
- `ze thanks` and `ze thanks licence` exist, with the daemon forms `show thanks list` and `show thanks licence`
- The Dependencies page shows licence and copyright per module, and a section for every other shipped work
- `website/data/dependencies.json` records licence and copyright per work, and the works the page did not list
- The repository holds the licence text of every shipped work, the web assets and Go included
- `NOTICE` and `website/legal/legal.md` say where the record is

## Existing Mechanisms (MANDATORY)

| Mechanism | Where (file + symbol, or page + section) | What it does today | This spec (extends, replaces, or does not apply, with the reason) |
|-----------|------------------------------------------|--------------------|-------------------------------------------------------------------|
| Build information | `internal/component/support/support.go` `collectVersion`; `internal/core/version/version.go` `readInfo` | Reads `debug.ReadBuildInfo()`; the support bundle lists module path and version | Extends: the credits answer reads the same `Deps` to decide which modules this binary credits and at which version |
| Curated dependency list | `website/data/dependencies.json`; `internal/le/site/dependencies.go` `renderDependencies`, `checkDependencyDrift` | Groups direct modules with a why sentence; refuses drift against `go.mod` | Extends: each entry gains licence and copyright; two lists are added for indirect shipped modules and for works that are not modules. It stays the one declaration |
| Core-leaf registry | `internal/core/ipsecinventory/registry.go` `Register`; `internal/core/env/registry.go` `MustRegister` | A gated package contributes from `init()`; a reader lists value records | Extends the pattern: a feature that carries a non-module work declares its presence from `init()`, so a build without the feature does not credit it |
| Local-data command at an offline root | `internal/plugins/help/register.go`; `internal/component/doctor/register.go` | Answers in the caller's process and through the daemon under `DaemonPath` | Extends the pattern for `thanks` |
| Text answer | `command.Blob`, `command.ShapeBlob` (`docs/architecture/api/commands.md`, "Text and bytes answers") | Carries a document on every surface | Extends: `thanks licence` answers a Blob |
| Owned pipe operation | `internal/component/bgp/plugins/cmd/rib/rib.go` `registerPipeFilters`, `command.RegisterPipeFilters` | A command declares a pipe operation of its own | Extends: `thanks licence` declares `name` |
| Vendored web asset sync and check | `internal/le/web/vendor/sync.go` `assets`, `targetedAssets`; `internal/le/web/vendor/vendorweb.go` `vendorPackages` | Copies vendored files to the consumers that embed them and refuses drift | Extends: the licence files of each asset directory are vendored and synced like its other files, and a directory without one is refused |
| Vendored module licence files | `go mod vendor` (`copyMetadata` in the Go toolchain) | Keeps each module's licence and notice files under `vendor/` | Extends: the generator copies them into the package the binary embeds; `vendor/` stays the source |
| Commit gate on third-party imports | `internal/le/commit/dependencies.go` `dependencyRefusal` | Refuses an added import the ledger does not approve | Extends: the same gate refuses a commit that links a module with no credit |
| Kernel GPL notice | `internal/appliance/instance/provenance.go` `Provenance.LinuxNotice` | Places a notice for the kernel in the image | Does not apply: the image's own contents are the sibling spec |
| Dependency ledger | `dependency-ledger.txt`, `internal/le/deps` | Records which Ze directory may import which package | Does not apply: it is keyed by package and importer and records approval, not licence. The new tool sits beside it under `le deps` |
| Website asset provenance table | `website/assets/vendor/README.md` | Lists the site's own assets with their licence files | Does not apply: those assets are served by the website, not shipped in `ze` |

## Design documents

| Document | Section | Derived from | Binding statement | Verdict | How this design stands against it |
|----------|---------|--------------|-------------------|---------|-----------------------------------|
| `ai/doors/system.md` | One schema, one command registry, every interface | the door every change reads | "A command is declared once and reaches every interface." | agrees | The two commands are declared once, as local-data commands with a daemon path; no interface package is edited |
| `website/AI.md` | Structure | the `// Design:` page of `internal/le/site/dependencies.go` and its test | "curated JSON inputs only" | agrees | The declaration stays a curated JSON input under `website/data/`; the page's line describing the file is extended to say it also records licences and the shipped works |
| `ai/doors/le-tooling.md` | Boundaries | the door covering the `internal/le` files and the `.ci` tests | "Never import `internal/le` from product code" | agrees | The generator and check live under `internal/le/deps/credits`; the binary reads only the generated data in `internal/core/credits` |
| `docs/architecture/core-design.md` | Command engine and composition roots | the `// Design:` page of `internal/le/web/vendor/sync.go` and `internal/exabgp/bridge/bridge_event.go` | "there is no second command engine for repository tooling" | agrees | `le deps credits` registers as an `le` tool on the shared command engine, and `thanks` as a product command on the same one |
| `docs/architecture/web-interface.md` | URL Scheme | the `// Design:` page of `internal/component/web/render.go` | "Static files (CSS, JS)" | agrees | The asset licence files are static files served under the existing asset path of the web interface and of the looking glass; no route is added to either. The REST documentation page gains three routes of its own, judged in the row for its page |
| `docs/architecture/web-components.md` | Per-page asset imports | anchored to `internal/component/web/render.go` | - | silent | The page describes the interface's components; this spec adds no component and no page, only a presence declaration and static licence files |
| `ai/doors/api-surfaces.md` | Boundaries | the door covering `internal/component/web/render.go` | "Never add a command in an adapter." | agrees | The command is registered by its owner plugin; web, REST, gRPC and MCP reach it through the dispatcher |
| `docs/architecture/api/architecture.md` | REST and gRPC authentication flow | the `// Design:` page of `internal/component/api/rest/embed.go` and `internal/component/api/rest/server.go` | - | silent | The page describes how REST and gRPC authenticate and says nothing of the documentation routes or the swagger-ui files; the three added routes use the wrapper the existing documentation routes use, so no flow it describes changes |
| `docs/architecture/api/commands.md` | Served by the daemon too | anchored to `internal/component/api/rest/server.go` | "cannot have a node at its own path: the root is not a verb" | agrees | `thanks` is an offline root that is not a verb, so its registrations name the daemon paths `show thanks list` and `show thanks licence`; the page gains the two paths |
| `docs/architecture/api/commands.md` | Design decisions | decision of 2026-08-21 | "The pipe wins: a command path names the subject and stops; scope, presentation and format are pipe operations after it, selector words included" | agrees | One work is selected with the declared pipe operation `name`, after the path. `thanks licence` is a second subject, the licence texts, with a different answer shape from the list; it is neither scope nor presentation of the list |
| `docs/architecture/container/runtime.md` | The system-call filter | the `// Design:` page of `internal/plugins/container/seccomp/profile.go` | - | silent | The page describes the container runtime; the profile's rules are unchanged, and the spec adds only the declaration that the Moby-derived profile is present |
| `ai/doors/system-plugins.md` | Boundaries | the door covering the plugin files | "A plugin owns its whole surface." | agrees | `internal/plugins/thanks` owns its commands, its YANG module and its tests, and delegates the data to a core package |
| `docs/architecture/appliance/iso-installer.md` | Decisions | the `// Design:` page of `internal/appliance/cmd_iso.go` | - | silent | The page describes the ISO installer; the spec changes nothing in it and adds only the declaration that the copied FAT writer is present |
| `docs/architecture/appliance/build-artifacts.md` | Decisions | anchored to `internal/appliance/cmd_iso.go` | - | silent | The page lists build artifacts; this spec produces none, and the image's own contents are the sibling spec |
| `ai/doors/appliance.md` | Where it sits | the door covering `internal/appliance/cmd_iso.go` | "`internal/appliance` is build-host tooling, not a component." | agrees | The only edit there is a presence declaration read by the host binary's own `thanks`; nothing reaches the image or the device |
| `docs/architecture/api/json-format.md` | Overview | anchored to `internal/exabgp/bridge/bridge_event.go` | - | silent | The page defines the event JSON the bridge translates; the translation is untouched |
| `docs/architecture/exabgp-bridge.md` | The text encoder is ExaBGP's own, ported | anchored to `internal/exabgp/bridge/bridge_event.go` | - | silent | The page describes the bridge's runners and translation; the spec adds only the declaration that the ExaBGP-derived tables are present |
| `docs/architecture/traffic/traffic-usage.md` | Decisions | the `// Design:` page of `internal/plugins/trafficusage/trafficusage.go` | - | silent | The page describes the accounting plugin; the spec adds only the declaration that the ported work is present |
| `docs/architecture/wire/attributes.md` | TL;DR (Read This First) | the `// Design:` page of `internal/core/ipregistry/registry.go` | - | silent | The page describes attribute wire formats and the pinned registry data; the data and its use are untouched |
| `ai/doors/core.md` | Boundaries | the door covering `internal/core/credits` and `internal/core/ipregistry/registry.go` | "Never import upward." | agrees | `internal/core/credits` imports the standard library only; features declare their presence into it from their own `init()` |
| `docs/contributing/dependency-approval.md` | The commit gate | the `// Design:` page of `internal/le/commit/dependencies.go` | "It judges only what the commit adds." | agrees | The added refusal judges the modules the commit newly links; a module already linked at HEAD without a credit is refused by the tree check, not by the commit gate. The page gains the refusal in its list |

## Data Flow (MANDATORY - see `ai/rules/architecture.md`)

### Entry Point
- Build time: `./le deps credits update` reads `website/data/dependencies.json`, the licence files under `vendor/` and `third_party/`, and writes the generated data the binary embeds
- Run time: the operator types `ze thanks` or `ze thanks licence`, or a daemon surface dispatches `show thanks list` or `show thanks licence`

### Transformation Path
1. Declaration: `website/data/dependencies.json` names each work with its licence identifier, copyright holder, source location and, for a curated entry, its group and why sentence
2. Generation: the tool resolves each shipped work's licence and notice files, copies them, and writes one record per work into `internal/core/credits/data/`
3. Presence: at start-up, each package that carries a non-module work declares it present, except Go itself, which every binary lists with no declaration at the build information's Go version; the modules present are read from the build information
4. Answer: the credits package joins the generated records with what is present and answers rows, or the licence document
5. Rendering: the command layer applies pipes and renders the rows or the Blob on the surface that asked
6. Website: the site build reads the same declaration and renders the page and its mirror

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Website data to product binary | A generated copy under `internal/core/credits/data/`, refused when it differs from what the generator would write | No |
| Core registry to gated features | Presence declared from each feature's `init()` | No |
| Caller process to daemon | The local-data bridge serves the same handlers under the YANG paths | No |

### Changed-Wire Census

**Search evidence:** searched `internal`, `cmd`, `website` and `.github` for `dependencies.json`, `dependenciesDataFile` and `dependencyData` across Go, JavaScript, Python, shell and workflow files on 2026-11-20. Positive control: `internal/le/site/dependencies.go` `renderDependencies` is found. A second reader is `internal/le/site/llmsdata.go` `loadLLMSInputs`, which reads the published copy into `dependencyData` for `llms.txt`. Two other hits are not readers: `internal/le/site/facts/sitefacts.go` names the file in a comment only, and `internal/le/site/facts.go` counts `go.mod`'s direct requirements through `directModuleVersions`. No JavaScript, Python, shell or workflow file in the repository reads the file. The file is also published: the site build stages it into the Pages artifact, so it has readers outside the repository.

| Wire | Role (writer / reader) | Carrier (file + producer/consumer symbol or script entry) | Source evidence | Migration or preservation disposition |
|------|------------------------|-----------------------------------------------------------|-----------------|----------------------------------------|
| `website/data/dependencies.json` | writer | hand-edited data file | `internal/le/site/dependencies.go` `dependencyData`: "the curated grouping" | Change: each module entry gains `licence`, `copyright` and `source`; two top-level lists are added, `linked` and `works` |
| `website/data/dependencies.json` | reader | `internal/le/site/dependencies.go` `renderDependencies` | Reads it through `readSourceJSON` into `dependencyData` | Change: reads the new fields and renders them; `total()` keeps counting the curated modules only |
| `website/data/dependencies.json` | reader | `internal/le/site/llmsdata.go` `loadLLMSInputs`, rendered by `internal/le/site/derived.go` `writeLLMSDependencies` | Reads the published copy with `readArtifactJSON` into the same `dependencyData`; the renderer walks `Categories` only and writes "## Dependency rationale" | Change: each module line of that section gains its licence identifier, so the machine-readable answer states what the page states; the `linked` and `works` lists are not rendered there, because the section's subject is why Ze takes each direct module |
| `website/data/dependencies.json` | reader | Anyone outside the repository: `./le site build` stages the file unchanged into the Pages artifact, where it is published as `data/dependencies.json` | `internal/le/site/paths.go` `isSourceOnly` does not list it in `sourceOnlyFiles`, and `../gh-pages/data/dependencies.json` exists | Change, additive: `categories`, `name`, `modules`, `module` and `why` keep their names and meaning; each module gains `licence`, `copyright` and `source`, and the top level gains `linked` and `works`. A reader of the existing keys reads what it read before |
| `website/data/dependencies.json` | reader | `internal/le/deps/credits` (new) | New | New reader |
| `internal/core/credits/data/credits.json` and the text files beside it | writer | `internal/le/deps/credits` `update` (new) | New | New wire |
| `internal/core/credits/data/credits.json` and the text files beside it | reader | `internal/core/credits` (new), through `go:embed` | New | New wire |
| Answer of `show thanks list` | writer | `internal/plugins/thanks` (new) | New | New wire |
| Answer of `show thanks list` | reader | API clients, MCP, the web command pages, operators' scripts | Outside the repository | New wire; the keys are the columns the Acceptance Criteria name |

### Integration Points
- `command.MustRegisterLocalData`, `registry.RegisterRoot`, `command.RegisterShape`, `command.RegisterColumns`, `command.RegisterPipeFilters` - the command is declared with them, as `help` and `env` are
- `debug.ReadBuildInfo` - the modules present
- `internal/le/site` `registerProducer` - the page keeps its producer
- `internal/le/web/vendor` `assets`, `targetedAssets` - the licence files join the vendored sets
- `internal/le/commit` `dependencyRefusal` - the commit gate
- `./le repo update` - regenerates the composition root and the YANG embed files for the new plugin

### Architectural Verification
| Check | Holds? | Evidence |
|-------|--------|----------|
| No bypassed layers (data flows through the intended path) | No | |
| No unintended coupling (components stay isolated) | No | |
| No duplicated functionality (extends existing, does not recreate) | No | |
| Zero-copy preserved where applicable (refs, not copies) | No | |
| Registration over hardcoding, outbound: new commands, views, families, and handlers register, and the core discovers them. No per-feature field, switch case, or factory is added to a core/shared package (`ai/rules/plugins.md`) | No | |
| Registration over hardcoding, inbound: no existing switch, seed map, validator, parser, runner, help string, or completion table has to learn this feature's name. Evidence names every list that was searched for the names this feature introduces, and the registry each one now derives from (`ai/rules/principles.md`) | No | |

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis (file/doc/user statement) | If wrong | Validated by | Status |
|----|-----------|--------------------------------|----------|--------------|--------|
| A-1 | `debug.ReadBuildInfo` lists every linked third-party module with its version in a vendored build of every shipped flavor | `go version -m bin/ze` prints 72 `dep` lines with versions | The answer cannot know what the binary links; the design fails | A unit test in `internal/core/credits` that the build information is available and non-empty, run under the feature tags | unvalidated |
| A-2 | The set of modules a shipped flavor links can be computed without compiling it, from `go list -deps` under that flavor's tags | The inventory research computed 97, 84 and 12 that way; `internal/le/repo/compiles/matrix.go` holds the flavors | The check cannot see a flavor's set before a build | Compare the tool's set for the development flavor with `go version -m bin/ze` | unvalidated |
| A-3 | Every linked module keeps a licence file at its vendored root | Found true for the 97 modules on 2026-11-20 | A module needs its text fetched from upstream and committed under `third_party/` | The check refuses a shipped module with no licence file | unvalidated |
| A-4 | A declared pipe operation reaches a local-data handler on the daemon route as it does on the local route | `ValidatedArgs.PipeOperations()` is read at `local_data.go` and at `plugin/server/command.go`; not traced end to end | `show thanks licence \| name <work>` answers every text through the daemon | A `.ci` test through the daemon: the draft `thanks-daemon-licence-name` passed, run by the main thread on 2026-11-20 | confirmed |
| A-5 | The Go licence text Ze must carry is the `LICENSE` file of the Go release `go.mod` pins | The Go distribution ships it one level above GOROOT on this machine | A wrong or missing text for the largest single dependency | Read the file from the Go source release of the pinned version and compare: `third_party/go/README.md` records `LICENSE` and `PATENTS` of tag `go1.27.4`, the version `go.mod` pins, compared with `cmp` against the files fetched from that tag on 2026-11-20 and found identical | confirmed |
| A-6 | The licence of each work that states none in the repository can be read upstream | Not checked | The work has no licence to record, and whether Ze may ship it is Thomas's decision | Read each upstream; the rows are in "Choices for Thomas". IANA: its licensing statement is quoted in `third_party/iana/README.md`. The lan-bandwidth-exporter port and the root certificate data inside `golang.org/x/crypto/x509roots/fallback`: no licence was read for either (no `third_party/lan-bandwidth-exporter/` exists, and the module is declared under its Go licence only), so both wait on Thomas (C-2) | confirmed for IANA; broken for the exporter port and the root bundle data |
| A-7 | `ze cli -c "show thanks list"` answering from the client binary is acceptable | `show plugin list` does the same; client and daemon are one build in every shipped arrangement | A mismatched client reports its own credits; the daemon's are reached by an interactive session or any other surface | Thomas's answer to C-1 | unvalidated |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | A licence identifier or copyright holder is declared wrongly: about 100 entries are filled once by hand | A declared holder that is not in the work's own files | The check requires the declared copyright string to appear verbatim in the work's licence or notice file when that file states one, and the declared identifier's title or defining sentence to appear in the text |
| R-2 | A dependency bump changes a licence text and the binary keeps the old copy | The generated copy differs from `vendor/` | `./le deps credits check` compares bytes over the whole tree and refuses; it runs on demand and in CI, never as a commit gate. The commit gate judges only what the commit adds: a commit that carries a changed licence file of a credited work, or a newly linked module, without the matching generated data is refused (AC-18) |
| R-3 | A new feature embeds a third-party file and declares no presence | A work in the declaration that no package declares, or a vendored web directory with no licence file | The check refuses a declared non-module work that no package declares present other than the toolchain work, and a vendored web directory with no licence file. Every binary lists the toolchain work (Go) with no declaration. The check refuses a declaration that marks no work toolchain, a toolchain work with a version or a `carried-by` list, a package that declares it present, and a second toolchain work. The binary refuses, with `ErrNoData`, a record that marks no work toolchain, and panics with BUG on a `DeclarePresent` of it. |
| R-4 | The licence document is large for a remote answer | A size near the Blob cap | It is about 300 KB against `command.BlobRemoteOctetsMax` of 8 MiB; a unit test asserts the document stays under the cap |
| R-5 | A binary built outside the repository's build path carries no build information | `ReadBuildInfo` answers not ok | The command refuses with the cause and never prints an empty list |
| R-6 | Identical licence texts repeated per work bloat the document | Document size | Works that share a byte-identical text are listed under one copy of it |
| R-7 | The swagger-ui bundle's companion file lists libraries the declaration misses | An entry in the companion file with no declared work | The check parses the companion file's package banners and refuses a name with no declared work |
| R-8 | The website page is edited while Thomas reviews it and tests chase a moving target | Test churn during review | Website rule: render the one page into `../gh-pages` during review; fixtures and tests at closure |
| R-9 | A licence file added to a vendored web directory is refused by the existing sync check until every consumer holds it, and two directories cannot share the name `LICENSE` | `./le web vendor check` reports MISSING or "is vendored twice" | Each licence file carries its asset's name (`htmx.LICENSE`), and the sync tables gain a row for it in the same change |

## Blast Radius

| Question | Answer |
|----------|--------|
| What breaks if this is wrong? | Nothing in routing. A wrong credit misstates a licence in public; a wrong check blocks commits that touch `vendor/` |
| How is it reverted? | A single commit revert per phase; no config or state is involved |
| Who else touches this path? | Sessions bumping dependencies (`vendor/`, `go.mod`), website sessions editing `website/data/dependencies.json`, and web sessions touching `internal/le/web/vendor` |

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| `ze thanks` | → | `internal/plugins/thanks` list handler over `internal/core/credits` | `test/ui/thanks-list.ci` |
| `ze thanks licence` | → | `internal/plugins/thanks` licence handler | `test/ui/thanks-licence.ci` |
| `show thanks list` through the daemon | → | the same list handler through the local-data bridge | `test/plugin/thanks-daemon-list.ci` |
| `show thanks licence \| name <work>` through the daemon | → | the same licence handler with its pipe operation | `test/plugin/thanks-daemon-licence-name.ci` |
| `./le deps credits check` | → | `internal/le/deps/credits` check | `TestCheckRefusesALinkedModuleWithNoCredit` |
| `./le site build` Dependencies page | → | `internal/le/site/dependencies.go` `renderDependencies` | `TestDependenciesPageShowsLicenceAndShippedSection` |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior | Permitted Survivors and Why |
|-------|-------------------|-------------------|-----------------------------|
| AC-1 | `ze thanks` on a shipped build | Prints one row per third-party work this binary carries, under the group headings of the curated list, with columns name, version, licence, copyright and why. `github.com/miekg/dns` appears with the version the build information states | A work the binary does not carry is absent, because the list describes this binary |
| AC-2 | `ze thanks` | Go's own library is a row, with the Go version the binary was built with and the Go licence | None: every binary links it |
| AC-3 | `ze thanks` on a build that carries the web interface | htmx, uPlot and each library the swagger-ui bundle contains are rows | A build without `ze_web`, `ze_lg` and `ze_rest` lists none of them, because it serves none of them |
| AC-4 | `ze thanks` | Each work copied or ported into Ze's own tree and compiled in is a row: the gokrazy FAT writer, the Moby seccomp profile, the ExaBGP encoders, the bandwidth exporter port, the IANA tables | A work whose feature is compiled out is absent |
| AC-5 | `ze thanks` when a linked module has no record | The module is still listed, with its version, and its licence cell reads `not recorded`; the command exits non-zero and names the module | None: a missing record is never an omitted row |
| AC-6 | `ze thanks` in a binary with no build information | Refuses, saying the binary carries no build information and so cannot list what it links, and how it must be built | None |
| AC-7 | `ze thanks licence` | Prints one document: for each distinct licence text, the works it covers with their copyright lines and source location, then the text, byte for byte as the work ships it, notice files included | Works sharing an identical text share one copy, because repeating it adds nothing |
| AC-8 | `show thanks licence \| name github.com/miekg/dns` | Prints that work's copyright, source location and licence text only | A name that matches no carried work is refused, naming `show thanks list` as the way to see the names |
| AC-9 | `show thanks list` on SSH, REST, gRPC, MCP and the web command page | Answers the same rows as `ze thanks`; `\| json` answers kebab-case keys `name`, `version`, `licence`, `copyright`, `why`, `group`, `source` | The local root `ze thanks` is not offered on the daemon surfaces, which offer its daemon path instead, as for `ze doctor` |
| AC-10 | `./le deps credits check` with a module linked by any shipped flavor and absent from `website/data/dependencies.json` | Refuses, naming the module, the flavors that link it and the entry to add | A module only development tooling links needs no licence copy in the binary, because the binary does not carry it |
| AC-11 | `./le deps credits check` after a vendored licence file changed | Refuses, naming the work and `./le deps credits update` | None |
| AC-12 | `./le deps credits check` with a declared copyright string that the work's licence and notice files do not contain | Refuses, quoting the declared string and naming the files it read | A work whose files state no copyright line (an Apache-2.0 text with no notice file) passes on its declared holder, because there is nothing to compare; the declaration marks it so |
| AC-13 | A test inside the binary, under the feature tags | Every module the build information lists has a record and a text, and every presence declared names a declared work | None |
| AC-14 | `./le web vendor check` with a vendored web directory that holds no licence file | Refuses, naming the directory | `third_party/web/ze/` holds Ze's own logo and is exempt by declaration |
| AC-15 | A browser on the looking glass or the REST documentation page | The licence file of each third-party asset is served beside it, and the swagger-ui bundle's reference to `swagger-ui-bundle.js.LICENSE.txt` resolves | None |
| AC-16 | The website Dependencies page | Each curated module shows its licence and copyright; a section "Also in the shipped binary" lists every other shipped work with version, licence and copyright; the Markdown mirror carries the same | The lead sentence and the published figure keep counting direct dependencies, because the new section is not a direct dependency |
| AC-17 | `NOTICE` and the website legal page | Each states that the record of third-party works and their licences is `ze thanks` and the Dependencies page | None |
| AC-18 | A commit that first links a new third-party module, or that carries a changed licence file of a credited work | `./le commit create` refuses it until the declaration and the generated data the commit carries match, naming the module and `./le deps credits update` | The commit that only adds a module to `go.mod`, `go.sum` and `vendor/` passes, because nothing links it yet. A module already linked at HEAD with no credit does not refuse an unrelated commit, because the gate judges only what the commit adds; the tree check reports it |
| AC-19 | `llms.txt` after a site build | Each module line under "Dependency rationale" states the module's licence identifier after its why sentence, so the line still opens with the module and its why | The indirect modules and the works that are not modules are absent from that section, because it answers why Ze takes each direct module |
| AC-20 | A browser on the REST documentation page requesting the swagger-ui licence, notice or companion file | Each is served under the documentation path, behind the same authentication as the page | None |

## End-to-End User Stories

| # | User does | Path through system | Test proving it works |
|---|-----------|--------------------|-----------------------|
| 1 | Runs `ze thanks` on a box with no daemon | root `thanks` -> local-data handler -> credits records joined with build information -> table | `test/ui/thanks-list.ci` |
| 2 | Reads one licence over SSH | `show thanks licence \| name <work>` -> dispatcher -> local-data bridge -> Blob | `test/plugin/thanks-daemon-licence-name.ci` |
| 3 | A program reads the credits over REST | `show thanks list \| json` -> API engine -> dispatcher -> rows | `test/plugin/thanks-daemon-list.ci` |
| 4 | A developer adds a dependency | commit gate -> credits check -> refusal naming the entry to add | `TestCheckRefusesALinkedModuleWithNoCredit` |
| 5 | A visitor reads the Dependencies page | `./le site build` -> `renderDependencies` -> page with licence columns | `TestDependenciesPageShowsLicenceAndShippedSection` |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestEveryLinkedModuleHasACredit` | `internal/core/credits/credits_test.go` | Every module in this binary's build information has a record and a text | |
| `TestPresenceOfAnUndeclaredWorkPanics` | `internal/core/credits/credits_test.go` | Declaring a work the generated data does not hold is refused at start-up | |
| `TestPresenceOfTheToolchainWorkPanics` | `internal/core/credits/credits_test.go` | Declaring the toolchain work present panics with a BUG naming the toolchain, since every binary lists Go without a declaration | |
| `TestRowsListGoWithNothingDeclared` | `internal/core/credits/credits_test.go` | In a binary where nothing is declared present and no module is linked, `Works()` answers no error and a row for Go at `runtime.Version()`, never an empty list (AC-2) | |
| `TestLicenceListsGoWithNothingDeclared` | `internal/core/credits/credits_test.go` | In the same binary, `LicenceDocument()` answers no error and a document holding each Go licence text byte for byte under Go's heading, and `LicenceOf("github.com/miekg/dns")` answers `ErrUnknownWork` with no bytes | |
| `TestRowsListTheToolchainUndeclared` | `internal/core/credits/credits_test.go` | The join over a go1.99 build that links no module, with nothing declared, answers the Go row alone at go1.99 in its group, and no other work | |
| `TestRecordWithNoToolchainWorkIsRefused` | `internal/core/credits/credits_test.go` | A record that marks no work as the Go toolchain is refused on load with `ErrNoData` naming the cause | |
| `TestMissingRecordIsRefused` | `internal/core/credits/credits_test.go` | A data directory with no record is refused with `ErrNoData`, never read as a record with no work | |
| `TestRecordWithAnUnknownKindIsRefused` | `internal/core/credits/credits_test.go` | A work whose kind is neither module nor work is refused with `ErrNoData` naming the work and the kind | |
| `TestRecordWithAWorkWithNoFileIsRefused` | `internal/core/credits/credits_test.go` | A work listing no licence file is refused with `ErrNoData` naming the work | |
| `TestRowsMarkAModuleWithNoRecord` | `internal/core/credits/credits_test.go` | A linked module without a record is listed as `not recorded` | |
| `TestRowsRefuseWithoutBuildInformation` | `internal/core/credits/credits_test.go` | No build information is an error, never an empty list | |
| `TestLicenceDocumentGroupsIdenticalTexts` | `internal/core/credits/credits_test.go` | One copy of a text shared by several works, each work named above it | |
| `TestLicenceDocumentStaysUnderTheRemoteCap` | `internal/core/credits/credits_test.go` | The document is smaller than `command.BlobRemoteOctetsMax` | |
| `TestLicenceNameSelectsOneWork` | `internal/plugins/thanks/thanks_test.go` | The `name` pipe operation answers one work and refuses an unknown name | |
| `TestThanksListColumns` | `internal/plugins/thanks/thanks_test.go` | The declared column order and kebab-case keys | |
| `TestCheckRefusesALinkedModuleWithNoCredit` | `internal/le/deps/credits/check_test.go` | A linked module absent from the declaration is refused by name | |
| `TestCheckRefusesAStaleLicenceCopy` | `internal/le/deps/credits/check_test.go` | A generated copy that differs from `vendor/` is refused | |
| `TestCheckRefusesACopyrightNotInTheText` | `internal/le/deps/credits/check_test.go` | A declared copyright string absent from the work's files is refused | |
| `TestCheckRefusesADeclaredWorkNobodyCarries` | `internal/le/deps/credits/check_test.go` | A non-module work other than the toolchain work, that no package declares present, is refused | |
| `TestCheckRefusesABundledLibraryWithNoWork` | `internal/le/deps/credits/check_test.go` | A package banner in the swagger-ui companion file with no declared work is refused | |
| `TestUpdateWritesWhatCheckAccepts` | `internal/le/deps/credits/update_test.go` | The generator's output passes the check, and a second run changes nothing | |
| `TestCheckAcceptsAToolchainWorkWithNoCarrier` | `internal/le/deps/credits/update_test.go` | The toolchain work passes the check with no `carried-by` and no package declaring it | |
| `TestCheckRefusesAToolchainWorkWithACarrier` | `internal/le/deps/credits/update_test.go` | A toolchain work that lists `carried-by` packages is refused, naming the package | |
| `TestCheckRefusesADeclarationOfTheToolchainWork` | `internal/le/deps/credits/update_test.go` | A package that declares the toolchain work present is refused at the call, with `./le deps credits update` as the fix | |
| `TestCheckRefusesADeclarationWithNoToolchainWork` | `internal/le/deps/credits/update_test.go` | A declaration that marks no work as the Go toolchain is refused, naming `website/data/dependencies.json` | |
| `TestCheckRefusesAVendorDirectoryWithoutALicence` | `internal/le/web/vendor/check_test.go` | A vendored web directory with no licence file is refused | |
| `TestDependenciesPageShowsLicenceAndShippedSection` | `internal/le/site/dependencies_test.go` | Licence and copyright columns, the shipped section, the mirror | |
| `TestDependenciesLeadCountsDirectModulesOnly` | `internal/le/site/dependencies_test.go` | The lead sentence's count is unchanged by the new lists | |
| `TestLLMSDependenciesStateEachLicence` | `internal/le/site/derived_test.go` | Each module line of "Dependency rationale" carries its licence identifier, and no indirect module appears | |
| `TestDocsServeTheSwaggerLicenceFiles` | `internal/component/api/rest/server_test.go` | The licence, notice and companion files answer under the documentation path, and are refused without authentication | |

### Boundary Tests (numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| none | The commands take no numeric input | N/A | N/A | N/A |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `thanks-list` | `test/ui/thanks-list.ci` | `ze thanks` lists `github.com/miekg/dns` with a version and its licence, and Go itself | |
| `thanks-licence` | `test/ui/thanks-licence.ci` | `ze thanks licence` prints the BSD text of `github.com/miekg/dns` with its copyright line | |
| `thanks-daemon-list` | `test/plugin/thanks-daemon-list.ci` | `show thanks list \| json` through the daemon answers the same rows | |
| `thanks-daemon-licence-name` | `test/plugin/thanks-daemon-licence-name.ci` | `show thanks licence \| name <work>` answers one text, and an unknown name is refused | |
| `pipe-local-command` | `test/ui/pipe-local-command.ci` | The local-data coverage marker counts the two new commands | |

## Files to Modify
- `website/data/dependencies.json` - licence, copyright and source per entry; the `linked` and `works` lists
- `internal/le/site/dependencies.go` - read and render the new fields and the shipped section
- `internal/le/site/dependencies_test.go`, `internal/le/site/testdata/published-dependencies.json`, `internal/le/site/testdata/published-dependencies.md`, `internal/le/site/testdata/published-dependencies-body.html` - at closure, after the page is approved
- `internal/le/site/derived.go` - `writeLLMSDependencies` states each module's licence
- `internal/le/site/derived_test.go` - the licence in the "Dependency rationale" lines; its existing assertion on the opening of the bubbletea line stays true
- `internal/component/api/rest/server_test.go` - the three licence routes of the documentation page
- `internal/le/web/vendor/sync.go` - licence files in the vendored sets
- `internal/le/web/vendor/check.go` - refusal of a vendored directory without a licence file
- `internal/le/web/vendor/vendorweb.go` - the licence file of a vendored package
- `third_party/web/MANIFEST.md` - a licence column and the licence files
- `internal/component/web/render.go` - declares the web assets it embeds as present; the synced licence files are embedded with the asset directory
- `internal/component/lg/embed.go` - the same for the looking glass
- `internal/component/api/rest/embed.go` - declares swagger-ui and the libraries in its bundle present, and embeds the licence, notice and companion files
- `internal/component/api/rest/server.go` - serves those three files under the documentation path, behind `s.withAuth`
- `internal/plugins/container/seccomp/profile.go` - declares the Moby profile present
- `internal/appliance/cmd_iso.go` - declares the FAT writer present
- `internal/exabgp/bridge/bridge_event.go` - declares the ExaBGP ports present
- `internal/plugins/trafficusage/trafficusage.go` - declares the exporter port present
- `internal/core/ipregistry/registry.go` - declares the IANA special-purpose registries present
- `internal/core/portname/services_table.go` - declares the IANA service names present
- `internal/le/commit/dependencies.go` - the commit gate refuses a commit that links a module with no credit
- `internal/test/localdatacoverage/localdatacoverage.go` - evidence rows for the two commands
- `test/ui/pipe-local-command.ci` - the coverage marker
- `NOTICE`, `website/legal/legal.md` - the corrected statement
- `docs/guide/command-reference.md`, `docs/contributing/dependency-approval.md`, `docs/architecture/api/commands.md`, `website/AI.md`, `ai/INDEX.md` - the pages the Documentation checklist names

## Files to Create
- `internal/core/credits/credits.go`, `internal/core/credits/credits_test.go` - records, presence, rows, licence document
- `internal/core/credits/data/credits.json` and the text files under `internal/core/credits/data/texts/` - generated; never edited by hand
- `internal/plugins/thanks/register.go`, `internal/plugins/thanks/thanks.go`, `internal/plugins/thanks/thanks_test.go` - the two commands
- `internal/plugins/thanks/yang/ze-thanks-cmd.yang`, `internal/plugins/thanks/yang/self_containment_test.go` - the daemon paths; the embed, register and doc files beside them are generated
- `internal/le/deps/credits/credits.go`, `check.go`, `update.go`, `register.go`, `check_test.go`, `update_test.go` in that directory - `le deps credits check` and `update`
- `internal/le/web/vendor/check_test.go` - the refusal of a vendored directory without a licence file
- `third_party/go/LICENSE`, `third_party/go/README.md` - Go's licence at the pinned version, and where it was read
- `third_party/web/htmx/htmx.LICENSE`, `third_party/web/uplot/uPlot.LICENSE`, `third_party/web/swagger-ui/swagger-ui.LICENSE`, `third_party/web/swagger-ui/swagger-ui.NOTICE`, `third_party/web/swagger-ui/swagger-ui-bundle.js.LICENSE.txt` - fetched from each upstream at the vendored version
- `third_party/exabgp/LICENSE`, `third_party/lan-bandwidth-exporter/LICENSE`, each with a `README.md` naming what Ze derived - the texts of the ported code
- `test/ui/thanks-list.ci`, `test/ui/thanks-licence.ci`, `test/plugin/thanks-daemon-list.ci`, `test/plugin/thanks-daemon-licence-name.ci`
- `docs/architecture/third-party-credits.md` - the design page: the declaration, the generated copy, presence, the checks
- `features/third-party-credits.md` - the feature record

### Integration Checklist
| Integration Point | Applies? | File / reason |
|-------------------|----------|---------------|
| YANG schema (new RPCs/config) | Yes | `internal/plugins/thanks/yang/ze-thanks-cmd.yang`: `show thanks list`, `show thanks licence` |
| YANG validation constraints | N-A | The commands take no leaf; `name` is a pipe operation, typed by its declaration |
| YANG custom validators | N-A | No leaf |
| CLI commands/flags | Yes | `internal/plugins/thanks/register.go`: root `thanks`, local-data `thanks` and `thanks licence` |
| CLI grammar (keyword before value) | Yes | `./le cli grammar`; the list action is `list` on the daemon path |
| Editor autocomplete | Yes | Derived from the root registry and the YANG nodes; the `name` operation completes from the carried works |
| Functional test for new RPC/API | Yes | `test/plugin/thanks-daemon-list.ci`, `test/plugin/thanks-daemon-licence-name.ci` |
| Pipe completeness | Yes | Rows declare `command.ShapeTab` and columns; the document declares `command.ShapeBlob`; both render through the pipe layer |
| Env var registration | N-A | No setting |
| Doctor check for runtime dependencies | N-A | No file, socket, service or kernel dependency at run time: the data is compiled in |
| Doctor code per problem class | N-A | The only run-time refusals are a missing record and missing build information, both defects of the build that the checks and the in-binary test refuse before release; neither is a state of the box an operator can repair |
| Prometheus counters/metrics | N-A | No observable state |
| BGP family surface (new SAFI / capability / attribute) | N-A | Not BGP |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature, or a feature's scope, evidence or level changed? | Yes | `features/third-party-credits.md` |
| 2 | Config syntax changed? | No | No config |
| 3 | CLI command added/changed? | Yes | `docs/guide/command-reference.md` |
| 4 | API/RPC added/changed? | Yes | `docs/architecture/api/commands.md`: the two daemon paths among the offline roots served by the daemon |
| 5 | Plugin added/changed? | Yes | `docs/guide/plugins.md` |
| 6 | Has a user guide page? | No | The command reference entry is the guide |
| 7 | Wire format changed? | No | No protocol wire |
| 8 | Plugin SDK/protocol changed? | No | Unchanged |
| 9 | RFC behavior implemented, changed, or newly proven? | No | No RFC |
| 10 | Test infrastructure changed? | No | Existing runners |
| 11 | Affects daemon comparison? | No | Not a compared capability |
| 12 | Internal architecture changed? | Yes | `docs/architecture/third-party-credits.md` (new), and `docs/contributing/dependency-approval.md`: what a new dependency owes |
| 13 | Route metadata keys added/changed? | No | None |
| 14 | Prometheus counters added/changed? | No | None |
| 15 | Registered plugin, event type, send type, command, capability, or inventory changed? | Yes | `docs/plugin-overview.md`, `docs/features/plugins.md`; `ai/INDEX.md` gains the keyword row |
| 16 | Any changed source file referenced by existing doc source anchors? | Yes | Run `./le spec citation anchors spec plan/release/delivery/spec-thanks-third-party-credits.md` at the start of implementation and name each page here |
| 17 | Existing docs show config/CLI/API examples for this area? | Yes | `website/AI.md`: the description of the dependency data file and of the Dependencies page; `third_party/web/MANIFEST.md` |

## Design presentation

### What changes

`ze thanks` thanks the projects Ze is built on and is Ze's licence record. It
lists every third-party work compiled into the binary that answers, grouped as
the website groups them, with version, licence, copyright holder and the
sentence saying why Ze uses it. `ze thanks licence` prints the licence texts,
which the binary carries, as one document, and narrows to one work with
`| name <work>`. The daemon serves both as `show thanks list` and
`show thanks licence`, so SSH, the web command pages, REST, gRPC and MCP answer
them too.

The website's curated list, `website/data/dependencies.json`, stays the one
declaration. It gains a licence, a copyright holder and a source location per
entry, a list of the indirect modules the shipped binary links, and a list of
the works that are not Go modules: Go itself, the web assets, and code copied
or ported into Ze. The Dependencies page shows the licences and a new section
for everything else that ships. A tool generates the copy the binary embeds,
with each work's licence files, and a check refuses a shipped module with no
entry, a licence copy that differs from the vendored one, and a declared
copyright holder the work's own files do not name.

What a binary lists is what it carries. Modules come from the binary's build
information. A work that is not a module is declared present by the feature
that carries it, so a build without the looking glass does not credit a chart
library it does not serve.

The licence texts missing from the repository are added: Go's, htmx's, uPlot's,
swagger-ui's with the notice and the companion file listing the libraries in
its bundle, ExaBGP's and the bandwidth exporter's. The web asset licences are
served beside the assets, so the looking glass and the REST documentation
page, which have listeners of their own, carry them without a page.

### Before and after

1. The command. Before: `ze thanks` answers `unknown command`. After:

   | group | name | version | licence | copyright | why |
   |-------|------|---------|---------|-----------|-----|
   | Go | Go standard library and runtime | go1.27.4 | BSD-3-Clause | The Go Authors | The language Ze is written in, its runtime and its library |
   | DNS | github.com/miekg/dns | v1.1.73 | BSD-3-Clause | The Go Authors, Miek Gieben | (the sentence the website holds today) |
   | Web assets | htmx | 4.0.0 | 0BSD | Big Sky Software | Drives the web interface and the looking glass without a JavaScript framework |

2. One licence. Before: nothing. After, `show thanks licence | name github.com/miekg/dns` prints the work's name and version, its copyright line, `https://github.com/miekg/dns` as its source, then the BSD 3-Clause text exactly as the module ships it.

3. The website page. Before: columns Module, Version, Why we use it, for direct dependencies. After: columns Module, Version, Licence, Copyright, Why we use it, and a section "Also in the shipped binary" with Name, Version, Licence, Copyright.

4. A new dependency. Before: a commit that links a new module passes with no licence record. After: `./le commit create` refuses it, naming the module and the entry to add to `website/data/dependencies.json`, then `./le deps credits update`.

5. `NOTICE`. Before: "Third-party code under vendor/ and third_party/ keeps its own copyright and license, recorded beside it." After: the same sentence, followed by one naming `ze thanks`, `ze thanks licence` and the Dependencies page as the list of those works and their licences.

### Alternatives set aside

| Alternative | Why set aside |
|-------------|---------------|
| Move the curated list out of the website folder into the product | Thomas chose to keep the website list where it is |
| A separate licence table for the binary beside the website list | Two lists of the same modules, and `ze thanks` loses the why sentences |
| A notices file shipped beside the binary | Thomas chose texts in the binary; a copied binary would lose the file |
| A licence classifier tool | A new dependency that guesses a licence family; a declared identifier checked against the work's own text is stricter |
| A dedicated page in the router's web interface | Thomas chose none; the command pages already answer `show thanks list` |
| One generated list with no presence declaration | A build without a feature would credit works it does not carry, and nothing would tie a new embedded asset to a credit |
| `show thanks list \| licence <work>` as the only form | No single document carries every text, which is what a redistribution clause asks for |

### Documents departed from

none

### Choices for Thomas

| # | Choice | Options | Decision |
|---|--------|---------|----------|
| C-1 | Whose binary `ze cli -c "show thanks list"` describes | (a) the caller's, as `show plugin list` does; (b) the daemon's, as `show version` does | (a), assumed: Thomas approved the design with "implement" on 2026-11-20 after being told (a) would be taken unless he said otherwise |
| C-2 | A shipped work whose upstream states no licence (to be read for the bandwidth exporter port, the IANA tables and the root certificate bundle in `golang.org/x/crypto/x509roots/fallback`) | (a) come back to you per work with what upstream says; (b) list it with the licence cell `none stated` | (a), assumed on the same basis |
| C-3 | Whether the ExaBGP row names every copyright holder of ExaBGP's licence file or the project | (a) every holder as the file lists them; (b) "the ExaBGP authors" | (a), assumed on the same basis |

## Implementation Steps

1. **Phase: Wiring (MANDATORY FIRST)** -- the two commands exist and answer from a stub registry
   - Tests: `test/ui/thanks-list.ci`, `test/ui/thanks-licence.ci`, `test/plugin/thanks-daemon-list.ci`, `test/plugin/thanks-daemon-licence-name.ci`
   - Files: `internal/core/credits/credits.go`, `internal/plugins/thanks/register.go`, the YANG module, `./le repo update` for the composition root
   - Verify: the commands are reachable on both routes; the tests fail because the registry is empty
2. **Phase: Licence texts into the repository** -- fetch each missing text from its upstream at the vendored version, record where it was read
   - Tests: `TestCheckRefusesAVendorDirectoryWithoutALicence`
   - Files: `third_party/go/LICENSE`, the licence files under `third_party/web/`, `third_party/exabgp/LICENSE`, `third_party/lan-bandwidth-exporter/LICENSE`, `third_party/web/MANIFEST.md`, `internal/le/web/vendor/sync.go`
   - Verify: every text is the upstream file byte for byte; a work with no upstream licence goes to C-2
3. **Phase: The declaration** -- licence, copyright and source for every curated entry; the `linked` and `works` lists
   - Tests: none yet; the check in phase 4 judges it
   - Files: `website/data/dependencies.json`
   - Verify: each identifier and holder is read from the work's own file, quoted in the phase report, never from memory
4. **Phase: Generator and check** -- `le deps credits update` and `check`, the commit gate
   - Tests: the six tests of `internal/le/deps/credits/check_test.go` and `update_test.go`
   - Files: `internal/le/deps/credits/credits.go`, `internal/le/commit/dependencies.go`, `internal/core/credits/data/credits.json`
   - Verify: the check is green on the tree and red on each seeded defect
5. **Phase: The answer** -- records joined with build information and presence; the licence document; the `name` operation
   - Tests: the tests of `internal/core/credits/credits_test.go` and `internal/plugins/thanks/thanks_test.go`; the four `.ci` tests turn green
   - Files: `internal/core/credits/credits.go`, `internal/plugins/thanks/thanks.go`, the presence declarations in each carrying package
   - Verify: the list on the development binary matches `go version -m` module for module
6. **Phase: Website page** -- licence columns and the shipped section, under the website rules
   - Tests: at closure, `TestDependenciesPageShowsLicenceAndShippedSection`, `TestDependenciesLeadCountsDirectModulesOnly`, refreshed fixtures
   - Files: `internal/le/site/dependencies.go`, `website/AI.md`
   - Verify: the page is rendered into `../gh-pages` for Thomas's review; tests and commit follow his approval
7. **Phase: Corrections and pages** -- `NOTICE`, the legal page, the command reference, the design page, the dependency approval page
   - Tests: `./le doc check all`
   - Files: the documentation files listed above
   - Verify: no page still claims a record that does not exist

### Critical Review Checklist

| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | Every AC-N has an implementation at file:line |
| Feature completeness | Every user story has a working path, no broken links |
| Correctness | Every declared licence identifier and copyright holder was read from the work's own file; no entry was written from memory |
| Naming | JSON keys kebab-case; the command spells `licence`; the daemon list path ends in `list` |
| Data flow | The binary reads only the generated copy and its own build information; nothing under `internal/le` is linked into `ze` |
| Operator errors and doctor codes | Each refusal (`ze thanks` without build information, an unknown `name`, each check refusal) says what is wrong, why, and the exact command that fixes it |
| Rule: one declaration | No Go file lists module names or licence identifiers; the declaration is the only list |
| Rule: silent zero | No path answers an empty list where it could not answer |

### Deliverables Checklist

| Deliverable | Verification method |
|-------------|---------------------|
| `ze thanks` lists what the binary links | Compare its module rows with `go version -m bin/ze` |
| Every licence text is in the binary | `ze thanks licence` contains the first line of each vendored licence file of a linked module |
| The check refuses each defect class | The six check tests |
| The page shows licences | `./le site build`, then read the generated Dependencies page in `../gh-pages` |
| The false statement is gone | `grep -n "recorded beside" NOTICE website/legal/legal.md` shows the corrected text |

### Security Review Checklist

| Check | What to look for |
|-------|-----------------|
| Input validation | The `name` value selects by exact match against carried works; it never reaches a file path |
| Information disclosure | The list states module versions. It is reachable only where `show version` already is: a shell on the box or an authenticated daemon surface. No unauthenticated route is added |
| Served files | The licence files served under the asset paths are static text; no directory listing is introduced |
| Resource use | The document is built once per process and bounded by the embedded data |

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

## Progress

### 2026-11-20

| Item | State |
|------|-------|
| Ran green (main thread) | Unit tests of `internal/core/credits`, `internal/le/deps/credits`, `internal/le/deps`, `internal/plugins/thanks`, `internal/component/api/rest`, `internal/component/lg`, `internal/thirdparty/fat`, `internal/exabgp/bridge`, `internal/core/ipregistry`, `internal/core/portname`, `internal/test/localdatacoverage`; scoped lint of `internal/core/credits`, `internal/le/deps`, `internal/plugins/thanks`; credits `update` and `check` through the temporary driver, both exit 0 (119 works); the four drafts `thanks-list`, `thanks-licence`, `thanks-daemon-list`, `thanks-daemon-licence-name` |
| Known reds, none from this work | `internal/component/web` `TestWebHandlerGoldenOutput/get-l2tp-session-json` (another session's uncommitted L2TP edits); `internal/le/web/vendor` `TestEveryAssetDirectoryIsDecided` (fleet interface assets); `internal/le/site` `TestADetailPageStatesWhatAnUndeclaredCommandTakes` and `TestTheHomepageRFCCardSaysWhatTheOldFigureCounted`; `./le arch tier check` (the `internal/core/ipc/yang` test import, the PPPoE VPP dataplane build tags) |
| Remaining | The `pipe-local-command` draft run (blocked while `internal/le/interoplab/l2tp` does not compile, another session's edit); load proof and promotion of the five drafts; the commits; `./le deps credits update` and `./le web vendor update` once the launcher rebuilds; removal of the temporary driver; Phase 6 (website page); Phase 7 (`NOTICE`, legal page) |

## Design Insights

- A command-owned pipe operation is recorded in the JSON answer's `pipe` metadata by the engine (`collectPipeMeta` in `internal/component/command/pipe.go` records an operation the partition hands the handler as a step, its first word the op and the rest the argument), so `thanks licence | name <work> | json` answers `blob` and `pipe`. The local-data coverage walk (`pipedBlobContent` in `internal/test/localdatacoverage/localdatacoverage.go`) asserts exactly those two keys and the one step `name` with its argument.
- An upstream licence file's name is not always `LICENSE`: ExaBGP ships `LICENCE.txt`, which Ze keeps byte for byte as `third_party/exabgp/LICENSE`, the name the other licence copies use, and `third_party/exabgp/README.md` records the upstream name and the comparison. A licence can also sit in a banner inside an asset rather than in a file beside it: `swagger-ui.css` embeds normalize.css v7.0.0, whose banner reads only "normalize.css v7.0.0 \| MIT License \| github.com/necolas/normalize.css", with no holder. The bundle-banner check reads only the bundle companion files (`*.js.LICENSE.txt`), so it never sees that banner; normalize.css is declared as a work of its own, with its licence and holder read from `third_party/swagger-ui-bundled/normalize.css.LICENSE`, and declared present by `internal/component/api/rest/embed.go`.
- WIP cap: Thomas authorised one claim above the cap for this spec only, on 2026-11-20, when the claim was refused at 21 in progress against a cap of 12. It is a permission for this spec, not a change to the cap.
- The website's Dependencies page was missed by a research pass that searched for thanks, credits and licence. A search for the subject under the name the project already uses for it (dependencies) would have found it first.

## Owned journal rows
| Class file | Date | Surface (short) |
|------------|------|-----------------|

## Key Design Decisions
| Decision | Alternatives Considered | Rationale |
|----------|------------------------|-----------|
| The website list stays the declaration and the binary carries a generated copy | Moving the list into the product | Thomas's choice on 2026-11-20 |
| Modules present are read from build information | A generated list per flavor | The binary states what it links; a per-flavor list is a second declaration |
| Non-module works are declared present by the package that carries them | One list filtered by feature tags in the credits package | The credits package would have to name features; presence is a registration |
| The licence document is one Blob with a `name` operation | One text per command invocation only | A redistribution clause asks for the notices to accompany the binary; one document is that |
| Web asset licences are synced and served beside the assets | A separate directory of texts | The bundle's own header points at a file beside it, and the looking glass and REST page then carry their licences without a page |

## Known Limitations
- The appliance image's own contents (BusyBox as `ze-recovery-shell`, the MBR and EFI boot code, the firmware blob, the gokrazy binaries, the installer's kernel) are `plan/release/delivery/spec-appliance-image-third-party-notices.md`
- `ze-installer` and `ze-serial-shell` are separate binaries with no command surface; their credits belong to the image and are in that spec

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
- [ ] The targeted tests for what changed pass, run in the main checkout by the main thread: the scoped package tests, the component groups and functional suites `./le repo changed scope print both` reaches. No agent runs any `./le verify` command: GitHub CI verifies the pushed commit, and the full verification is Thomas's release check. A red CI reports gets a fix commit
- [ ] The named runs for this spec pass: `./le test unit pkg path ./internal/core/credits/`, `./le test unit pkg path ./internal/plugins/thanks/`, `./le test unit pkg path ./internal/le/deps/credits/`, `./le test unit pkg path ./internal/le/web/vendor/`, `./le test unit pkg path ./internal/le/site/`, `./le test functional ui`, `./le test functional plugin`
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
- [ ] Append `plan/templates/TEMPLATE-CLOSURE.md` and complete every section in it
- [ ] `/ze-review` gate clean, recorded via `internal/le/spec/review.go`
- [ ] Any lesson routed to its governing surface under `ai/rules/planning.md`; no lesson artifact created merely for closure
- [ ] **Commit A:** code + tests + docs + edited spec + any journal rows owed by the work
- [ ] **Commit B:** `remove plan/release/delivery/spec-thanks-third-party-credits.md` only, in the same `./le commit create` script (commit A preserves the spec in history)
