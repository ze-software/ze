# GitHub Pages and Presentations

Website sources live under `website/` on the main branch.
All `../gh-pages` content MUST be generated from this repository.
`./le site build` writes the publishable artifact to `../gh-pages` and removes
old source-only files there. It reuses matching demo artifacts. Run
`./le site terminal-demo render all` first to force new demo artifacts.
`./le site terminal-demo render name <demo-id>` re-records one demo while you work on
its tape, and publishes it beside the artifacts it did not record. The ids are in
`demos/terminal/manifest.json`.

A demo the manifest lists that has no recording does not stop the build. Its
page is published with "Recording not yet available." where the player would
be, the Markdown mirror says the same, and the build prints one warning line
naming the demo, its page and the `render name <demo-id>` command that records
it. `./le site check` judges that page like any other. Media that no longer
matches its artifact manifest is still refused.
<!-- source: internal/le/site/demo.go -- (*demoCatalog).expand, (*demoCatalog).warnUnrecorded, renderUnrecordedDemoHTML -->

A recording lives in the ARTIFACT tree, at `../gh-pages/assets/demos`. A render
writes it there, and a build reads it from there for the `?v=` digest it stamps
into every page that replays a demo. So one directory answers what the site
serves and what the pages name. A build that rewrites the artifact keeps the
media, because it lays the last published artifact down before any page is
rendered. `TERMINAL_DEMO_OUTPUT` moves a render to another root.
The last published artifact is the `gh-pages` checkout beside the MAIN
checkout, which Git names through `git rev-parse --git-common-dir`. A build run
from a linked worktree, such as the verify worktree under `tmp/verify-worktree`,
reads that tree to seed itself and never writes into it: its default output
stays beside the worktree.
<!-- source: internal/le/site/build.go -- lastPublished, mainCheckout -->
<!-- source: internal/le/site/demo.go -- (*demoCatalog).assetRoot -->
<!-- source: internal/le/site/terminaldemo/actions.go -- renderEngine -->

The render action and site build retire recordings whose IDs no longer exist
in the source manifest. They remove only media paths declared by those IDs in
the previous artifact manifest, then remove those entries. A retired path must
be the ID's own `.cast`, `.txt`, `.webm` or `.webp` file. Ownership checks
normalize current asset paths before comparison. Unsafe, missing or
symlink-aliased current assets stop retirement before any deletion, as does
shared ownership. Unlisted media and current recordings remain untouched.
The site build does this after seeding its output, before a page reads the media.
<!-- source: internal/le/site/terminaldemo/retire.go -- Engine.RetireDeleted, Engine.retireDeletedAssets -->
<!-- source: internal/le/site/terminaldemo/render.go -- Engine.renderSelected -->
<!-- source: internal/le/site/build.go -- Build -->

A recording runs in a container this repository builds and publishes to no
registry, so build it once per checkout with `./le site terminal-demo build image`.
It reads the image tag from `demos/terminal/manifest.json`, which is the tag the
recorder runs. The build goes through the lab's `interoplab.Docker.Build`, so
it carries the owner labels. This action opts into live build progress: Docker
runs without `-q`, and its stdout and stderr reach the action's output as they
arrive, including the native job log when the action runs under admission.
There is no synthetic heartbeat. A failure carries the last 4096 bytes of
Docker's stderr, or a bounded stdout excerpt if stderr is empty. After a
successful build, the wrapper reads the immutable image ID by inspecting its
unique run-owned tag, never the shared manifest tag. The run-owned tag is
released when the action ends, including a failed or canceled streamed build;
the manifest tag and Docker's build cache stay. Other `Docker.Build` callers
remain quiet unless they opt in. A render asks the daemon through
`Docker.ImagePresent` and refuses to
start when that image is absent, naming this action; an inspect the daemon
does not answer is reported as that failure, never as an unbuilt image.
Rebuild after a change to the renderer Dockerfile or either canonical peer pin.
<!-- source: internal/le/site/terminaldemo/render.go -- Engine.BuildImage, Engine.requireRendererImage -->

The renderer takes the FRR release from `interoplab.FRRImage` and installs that
release from FRRouting's signed `trixie` repository, not Debian's older package.
It reads the keepalived version and SHA-256 from
`test/interop/Dockerfile.keepalived`, then builds that verified source on Debian
with LVS disabled and VRRP VMAC enabled, using two compiler jobs. The resolved
peer build arguments participate in both Docker's cache key and recording
freshness. Browser and vendored font pins remain independent of these peers.

Before that, a render, like the renderer's `validate` mode, checks the Docker
daemon's kernel with the demo `ze`, and
refuses a kernel that lacks any feature Ze enrolls, naming each one: a demo
recorded there would show a host fact as product behavior. The check is the
one every Docker run that runs Ze makes, described in
`docs/architecture/testing/interop.md`, "The Docker host kernel check".
<!-- source: internal/le/site/terminaldemo/render.go -- Engine.preflightContainers, Engine.checkKernel -->

`./le site terminal-demo build binaries` cross-builds the programs the
container runs into `tmp/terminal-demos/bin`: `ze`, `ze-demo`, and a linux
`le`. The `le` build takes the recipe the perf sender container uses: the
`ze_le` tag plus every daemon feature tag, `GOOS=linux`, `CGO_ENABLED=0`, and
the renderer's architecture. The file MUST be named `le`, because `cmd/ze`
selects its personality from its file name. The container records a tape with
`le site terminal-demo pty --tape <file>`. That action hands its words to the
PTY recorder, so `./le site terminal-demo pty --help` prints the recorder's own
options.

Command-mode waits match freshly painted terminal rows, including cursor edits
to text painted by an earlier command. Unchanged success text left on screen
does not satisfy a later command's wait. Command and tape driving share the
terminal grid, which retains incomplete control sequences across PTY reads.
<!-- source: internal/le/site/terminaldemo/pty.go -- driveCommands -->
<!-- source: internal/le/site/terminaldemo/screen.go -- terminalScreen -->

The `ze-demo` runtime links the product composition root and uses the same
feature tags as the staged `ze`. Its configuration checks therefore load the
actual registered schemas, rather than parsing against only the foundation
modules linked by the fixture harness itself.
<!-- source: internal/le/linuxle/linuxle.go -- Argv -->
<!-- source: internal/le/site/terminaldemo/actions.go -- recorderBuildCommand -->
<!-- source: demos/terminal/cmd/ze-demo/main.go -- product schema composition -->
<!-- source: internal/le/site/terminaldemo/entrypoint.go -- recorderCommand -->
<!-- source: internal/le/site/terminaldemo/pty.go -- RunPTY -->

Each demo's lab and its output check are one scenario, registered under the
id that the manifest's `validate` field names. A tape starts, drives and stops
its lab with `ze-demo run <id> <action>`, and `ze-demo validate <id>` checks
the output with that scenario's validator. A new demo registers its scenario from
an `init()` in a `register_*.go` file in `internal/le/site/terminaldemo/` and
edits no central switch or map. The validator is required, so no recording
ships without a check of what it shows. A demo whose tape starts no lab
registers a validator and no runner. A tape that names an unregistered id, or a
manifest demo with no scenario, fails the package's unit test.
<!-- source: internal/le/site/terminaldemo/registry.go -- scenarioRegistry -->
<!-- source: internal/le/site/terminaldemo/register_scenarios.go -- init -->
<!-- source: internal/le/site/terminaldemo/registry_test.go -- TestEveryManifestDemoHasARegisteredScenario -->

Live topics use `registerLiveTopic` in that same registry. Each registration
names its configuration fragment, one live check and a read-only readiness
callback. The standalone validator owns prepare, start and stop. The continuous
validator reads chapter order from `showcase/demo.tape` and calls those same
checks against one daemon, checking its PID between chapters. Each check proves
absence before its commit, the failure diagnostic and recovery.
<!-- source: internal/le/site/terminaldemo/register_topics.go -- registerLiveTopic -->
<!-- source: internal/le/site/terminaldemo/validate_topics.go -- validateLiveTopic -->
<!-- source: internal/le/site/terminaldemo/validate_showcase_topics.go -- showcaseTopicOrder, validateShowcase -->

`loadShowcaseSections` loads every section into one actual SSH candidate and
commits it once. It never imports an offline configuration into a running
store. The tapes use the same section files and `configure.tape`/`show.tape`
fragments for standalone and continuous recordings. Hidden `await` actions
observe real state before visible proof commands; they neither configure a
feature nor replace a failed proof.
<!-- source: internal/le/site/terminaldemo/validate_topics.go -- loadShowcaseSections -->
<!-- source: internal/le/site/terminaldemo/register_topics.go -- registerLiveTopic -->
<!-- source: demos/terminal/showcase/demo.tape -- shared topic fragments -->


The topic recordings share one lab, the `showcase-lab` scenario. It starts the
shown Ze in the container's root network namespace with
`demos/terminal/showcase-lab/ze.conf`. That base config contains interfaces and
BGP peers and a loopback-only Prometheus exporter for diagnostic queries, but no
topic configuration or topic plugin bindings. Baseline BGP
plumbing attaches `bgp-adj-rib-in` to each peer with an UPDATE send grant for its
initial replay. Transit also attaches `bgp-rib`, receiving update, state and
refresh events and sending updates for Route Refresh catch-up.
The base guard admits this plumbing but rejects topic bindings.
FRR runs bgpd, bfdd and ospfd under one zebra. An RTR cache, IRR server and transit peer serve
the routing chapters. `announce` starts or restarts the customer peer so it
announces its routes after the IRR filter commit.
Its OPEN declares a distinct BGP identifier from transit's: these fixtures
represent separate speakers in the same AS, not parallel sessions to one speaker.
<!-- source: demos/terminal/showcase-lab/customer.msg -- explicit customer identifier -->
<!-- source: internal/le/site/terminaldemo/scenarios_showcase.go -- showcaseLabFixtures, showcaseCustomer -->
<!-- source: demos/terminal/showcase-lab/ze.conf -- base interfaces and BGP peers -->
<!-- source: internal/le/site/terminaldemo/validate_showcase.go -- requireShowcaseBase, requireShowcaseBaseTree -->

The hidden VPN endpoint is a second Ze in `showcase-vpn`, with its own config
store and real kernel XFRM backend. Each protected host has a separate
namespace. The shown Ze has no VPN, XFRM interface or tunnel route until the
topic's live commit. Both VPN configurations must explicitly select
`unmatched bypass`, so unrelated chapters and SSH stay reachable.
<!-- source: internal/le/site/terminaldemo/scenarios_showcase_network.go -- prepareShowcaseVPN, showcaseNetworkCommands -->
<!-- source: demos/terminal/showcase-lab/far-end.conf -- hidden responder and unmatched policy -->

| Link or service | Shown side | Fixture side |
|-----------------|------------|--------------|
| FRR | `frr0`, `172.29.0.1/24` | `showcase-frr`, `172.29.0.2/24`, loopback `10.255.1.2/32` |
| VPN underlay | `vpn0`, `172.30.0.1/30` | `showcase-vpn`, `172.30.0.2/30` |
| Protected networks | `vpn-lan`, `10.78.1.1/24`; host `10.78.1.2` in `showcase-host-left` | `lan0`, `10.78.2.1/24`; host `10.78.2.2` in `showcase-host-right` |
| VRRP LAN | `zeptrk0`, `203.0.113.251/24` | `showcase-vrrp`, `203.0.113.252/24`; keepalived v3, VRID 12, priority 100 |
| Tracked uplink | `zetrk1`, independently controlled dummy link | No peer process |
| Traffic | `traffic0`, `10.77.0.1/24`, HTTP port 8080 | `traffic-peer`, `10.77.0.2/24` |
<!-- source: internal/le/site/terminaldemo/scenarios_showcase_network.go -- showcaseNetworkCommands, showcaseTrafficFixture -->
<!-- source: demos/terminal/showcase-lab/keepalived.conf -- VRRP companion -->

Use `ze-demo run showcase-lab control <fixture> <action>` for scripted failures
and recovery. Controls belong to their fixture, not a central feature switch.
They never stop the shown Ze.

| Fixture | Actions | Effect |
|---------|---------|--------|
| `rtr-cache` | `stop`, `start` | Disconnect or restore the RTR service |
| `customer` | `reannounce` | Restart its owned BGP speaker and resend routes |
| `frr` | `down`, `up` | Change the `frr0` link state |
| `keepalived` | `stop`, `start` | Stop or restore the companion's advertisements |
| `tracked-uplink` | `down`, `up` | Change the independent `zetrk1` link state |
| `traffic` | `burst`, `stop` | Start bounded ICMP/HTTP senders or stop those senders, leaving the server running |
| `vpn` | `ping` | Send four real probes between the protected hosts |
<!-- source: internal/le/site/terminaldemo/scenarios_showcase_control.go -- controlShowcaseLab -->
<!-- source: internal/le/site/terminaldemo/scenarios_showcase_network.go -- showcaseVPNFixture, showcaseKeepalivedFixture, showcaseTrackedFixture, showcaseTrafficFixture -->

Each fixture declares a port or command-output readiness observation.
Startup refuses a fixture with no observations. The readiness path refuses an
empty target or a command observation without a command; neither can stand in
for a service that was actually observed ready.
`startFixtures` records its processes before waiting. Selective stop waits for
those processes to exit before restart. The shown daemon has its own `ze.pid`;
topic checks use `showcasePID` and `requireShowcasePID`, not the first aggregate
PID. `ze-demo validate showcase-lab` checks the topic-free base, fixture
controls, named BGP peers and unchanged shown PID inside the renderer container.
It does not replace each topic's live-commit and traffic proof.
<!-- source: internal/le/site/terminaldemo/scenarios_showcase.go -- startFixtures, awaitReadiness -->
<!-- source: internal/le/site/terminaldemo/scenarios_showcase_control.go -- stopShowcaseFixture, showcasePID, requireShowcasePID -->
<!-- source: internal/le/site/terminaldemo/validate_showcase.go -- validateShowcaseLab, requireShowcaseBase -->

A demo's container holds `NET_ADMIN`, `NET_RAW` and `SYS_ADMIN` under Docker's
default seccomp and AppArmor profiles. A demo that needs more sets
`"privileged": true` and MUST state why in `"privileged-reason"`: what it does
that those grants refuse, such as `ip netns add`, whose bind mount the default
AppArmor profile denies. Every mode refuses a privileged demo with no reason,
and a reason on a demo that is not privileged, before any command starts.
<!-- source: internal/le/site/terminaldemo/render.go -- containerCommand -->
<!-- source: internal/le/site/terminaldemo/manifest.go -- privilegeJustified -->

Every demo container runs through the lab Docker wrapper, so it carries the
lab owner labels and a `ze-oneshot-` name. An interrupt (SIGINT or SIGTERM) to
a validate or render action cancels the run, and the wrapper removes the
running container by that name rather than leaving it to record to the end. A
container that runs past one hour is stopped and removed the same way. The
same interrupt to `build image` cancels the `docker build` it runs.
<!-- source: internal/le/site/terminaldemo/render.go -- runDemoContainer, demoContainerTimeout -->
<!-- source: internal/le/site/terminaldemo/actions.go -- signalRooted -->
<!-- source: internal/le/interoplab/docker.go -- Docker.RunOneShot -->

A demo whose only extra need is writing its own network namespace's sysctls
under `/proc/sys/net`, such as `rib-fib`, where `fib-kernel` has the `sysctl`
plugin turn on forwarding, sets `"net-sysctl": true` instead. Its container runs with
`--security-opt systempaths=unconfined` and, on a daemon that applies AppArmor,
under the `ze-lab-net-sysctl` profile the interop labs use, which admits
`/proc/sys` writes under `net/` only. A host that applies AppArmor without that
profile loaded refuses the demo and names the command that loads it. When the
renderer cannot read the host's profile list (it runs as a user without
policy-view privilege), the container starts, and if the daemon then cannot
apply the profile the demo is refused with the same command rather than an
exit code. A renderer on a client that is not Linux, such as a Mac, is never
told to run that command, which loads a profile only on the Linux machine it
runs on: the refusal names the QEMU docker-lab route instead. On a daemon with no AppArmor nothing confines the writes to `net/`,
which is still narrower than `--privileged`. A demo that states both grants is
refused.
<!-- source: internal/le/site/terminaldemo/render.go -- demoGrants, runDemoContainer -->
<!-- source: internal/le/interoplab/apparmor_netsysctl.go -- Docker.NetSysctlSecurityArguments, Docker.NetSysctlUnapplied -->

A tape can `Source` a fragment that lives outside its demo directory, such as a
topic's `demos/terminal/topics/<topic>/configure.tape`, which both the topic
recording and the showcase play. A recording's source and definition digests
cover every tape it sources, nested, and every file beside a fragment outside
its own directory, such as the config snippets the fragment loads. Editing a
fragment therefore marks every recording that plays it stale.
<!-- source: internal/le/site/terminaldemo/manifest.go -- addSourceClosure -->

`Card <demo> <phase>` shows a card from `cards.json` while a program such as
the SSH editor holds the terminal, where `ze-demo card` cannot print. The
recorder writes the card into the cast and nothing into the PTY. Output the
program writes meanwhile is held back. When the card's time is up, the
recorder clears the screen and replays the output written since the program's
last screen erase. The repaint is the same byte stream the screen was drawn
from, so it keeps the colors. When the card should appear, and the rules a tape
must follow, are in `terminal-demos.md`.
<!-- source: internal/le/site/terminaldemo/tapecard.go -- tapeSession.card, castWriter.remember -->

`./le site build output <directory>` builds into another artifact root, and
`./le site check output <directory>` judges that same root. Both default to
`../gh-pages`. A private scratch directory is optional for previews and
intermediate checks. To complete approved website changes, run `./le site build`
and `./le site check` against the canonical `../gh-pages` checkout beside main.
Use the native builder to preserve `.git` and generated media.
<!-- source: internal/le/site/build.go -- Build, snapshotArtifact, seedArtifact, cleanArtifact -->

`./le site check` also refuses a page that shows a value of
`data/site-facts.json` as plain text, outside its `data-ze-stat` or
`data-ze-rfc` element, and lists each one under `plain-figures` with the page,
the figure and the key. Fix it in the page's source: write the value as
`{{ze:<dotted.key>}}` in a hand-written page or a `data/nav.json` description,
or through `factLookup.marked` (`markedFact`) in a producer, reading the value
from the snapshot rather than counting it again. The matching rule is
in `docs/architecture/site-facts.md`, "One published copy of each value".
<!-- source: internal/le/site/plaintext.go -- checkPlainFigures -->

The RFC pages that still carry server-written figures are listed in
`website/data/plain-figure-ratchet.json`, which the check reads from the
sources and never deploys. A listed page is not refused for its figures. When
a listed page no longer shows one, the check lists it under `stale-ratchet`
and fails: delete its entry from the file. Only a page under
`quality/rfc-compliance/` may be listed, so the file is not a way to excuse
any other page.
<!-- source: internal/le/site/plaintext.go -- loadPlainFigureRatchet -->

The release data the site shows (the newest release, and the appliance lab's
ISO size) is read from the committed `website/data/github-releases.json`, and
the build never reaches the network for it. After publishing a release, refresh
the file and commit it with the work that follows:

```
./le site release update
```

The command reads the forge's releases list and rewrites the file. When the
forge cannot be reached it exits non-zero, names the endpoint and the cause,
and leaves the committed file as it was, so retry it later. While the file
states no release, the lab page keeps its fallback text "under 150 MB".
The file's checks and the snapshot keys are in
`docs/architecture/site-facts.md`, "Release data".
<!-- source: internal/le/site/release/actions.go -- update -->

Local regeneration does not authorize a push or deployment. Never push without
owner instruction.

Before it builds, `./le site build` warns on stderr, and lists under
`stale-supported` in its report, every feature held at Supported whose recorded
test or interop run is stale. The level stands and the page is unchanged; the
warning says the evidence wants re-recording. `./le site build refresh`
re-records those features through `./le feature run` first, which runs
real tests and can need Docker or a QEMU guest, so it is never the default and
never a prompt. The rule is `feature-maturity.md`, "Recorded runs".
<!-- source: internal/le/site/freshness.go -- freshness -->

See `website/AI.md` for the full reference: structure, tools, and how to add a talk.

## README thumbnail and terminal playback

The homepage and README select the continuous recording through `HeroDemo`.
The site replays its `.cast` terminal events with the vendored asciinema player.
The README uses `docs/demos/showcase.webp`, linked to the continuous showcase's
actual player anchor. It does not embed encoded video or an animated SVG.
<!-- source: internal/le/site/terminaldemo/types.go -- HeroDemo -->
<!-- source: internal/le/site/home.go -- homeHeroSlots -->
<!-- source: README.md -- terminal-demo-hero -->

The homepage terminal shows a genuine recorded frame before playback. Its
`data-cast-poster` selects the BGP dashboard at 43 seconds without changing the
full replay's start. Check that frame against the published cast and transcript
when changing or re-recording the selected demo. The shared player uses
`data-cast-start` for both the poster and playback start of a chapter; that
chapter setting takes precedence over a preview-only poster. Neither player
autoplays, and both retain controls and transcript links.
<!-- source: internal/le/site/homebody.go -- homeTemplate -->
<!-- source: website/assets/js/site.js -- initTerminalDemoPlayers -->

The opening preview enlarges the dashboard's upper-left area only while the
player shows its initial play overlay. Full-screen mode shows the whole
terminal. Playing, seeking or stepping a frame removes preview enlargement
for that player; pausing does not restore it. The recording itself is unchanged.
<!-- source: website/assets/js/site.js -- initTerminalDemoPlayers -->
<!-- source: website/assets/css/30-components.css -- data-cast-preview-active -->

The vendored player carries a small `clearPoster` repair: recorded-time posters
retain terminal state when navigation continues with incremental events. Text
posters still clear. Keep the source patch and generation procedure in
`website/assets/vendor/README.md` when updating the player.
<!-- source: website/tools/asciinema-player-poster.patch -- clearPoster -->

Keep a thumbnail's captured source frame and conversion evidence with the
recording's acceptance evidence. Converting an existing PNG frame to lossless
WebP changes its image encoding, not the captured session. It does not constitute
a new recording or new runtime proof.

Browser demonstrations publish a WebP screenshot and their authored transcript.
The browser driver still performs the configuration workflow and observes its
result, but it does not record video. Terminal demonstrations retain their
`.cast` and transcript files. Recordings remain an explicit manual action, not
a site-build step.
<!-- source: demos/terminal/web-config/demo.cjs -- captured committed configuration -->
<!-- source: internal/le/site/terminaldemo/manifest.go -- assetExtensions, assetNames -->
<!-- source: internal/le/site/terminaldemo/render.go -- Engine.renderDemo -->

Older tape `Output` directives can still name `.webm`. The native PTY recorder
always replaces that suffix with `.cast` and writes terminal events, not video.
Changing those directives only for their spelling would change recording
definitions without changing their output.
<!-- source: internal/le/site/terminaldemo/pty.go -- recordTape -->

A presentation change does not make an old recording current. Preserve its
recorded source, definition, binary and release identities. The site publisher
checks the bytes against the artifact manifest. The separate
`./le site terminal-demo check artifacts` action also checks current recorder
sources and refuses stale recordings. Do not replace those digests to make a
presentation-only change pass.
<!-- source: internal/le/site/demo.go -- demoCatalog.verifyAssets, demoCatalog.verifyAsset -->
<!-- source: internal/le/site/terminaldemo/render.go -- Engine.checkAll -->
<!-- source: internal/le/site/terminaldemo/manifest.go -- nativeRecorderSources, Engine.verifyAssets -->

Do not generate or publish MP4 or WebM assets for this site. The former README
carrier conversion and GitHub SVG/video selection workflow is retired. A
local preview or conversion authorizes no upload, push or deployment.
The site's publication filter excludes these extensions when it seeds previous
assets, removes staged source files and checks the final output.
<!-- source: internal/le/site/paths.go -- isSourceOnly -->
<!-- source: internal/le/site/build.go -- publishable, removeSourceOnly -->

## Rendering tests

Editorial sources own the text and publication dates. An older Pages artifact
does not override an intentional source edit. Blog, weekly-update and homepage
tests render controlled authored inputs instead of freezing the public site's
wording, newest date or article count. Their assertions cover escaped metadata,
external-link isolation, Markdown content and mirrors, shells and media,
date ordering, draft and undated feed exclusions, and page ownership and cleanup.
Do not refresh a prose snapshot to make a source edit pass.

Check the painted page as well as its DOM text. The shared reveal animation
starts on any viewport intersection: a long command reference cannot expose a
fixed fraction of its total height. DOM text alone does not prove the section
is visible.
<!-- source: website/assets/js/site.js -->

The fill of `data-ze-stat` figures from `data/site-facts.json` is proven in a
real browser, because the shared header is inserted after the body is filled
and only a loaded page shows that ordering. The test serves a fixture site with
the real `site.js`, drives headless Chrome through chromedp, swaps the snapshot
and reloads, and counts one snapshot fetch per load. It needs Google Chrome or
Chromium and fails when neither is installed; it lives in its own package so
the other site tests run without a browser (`website/AI.md`, "The browser
fill").
<!-- source: internal/le/site/browsertest/fill_test.go -- TestTheBrowserFillsEveryMarkedValueFromTheSnapshot -->

The corpus tests materialize all registered derived artifacts once per process.
This setup requires the checkout's real Git history, including a resolvable
`HEAD` for the roadmap; a source-only mount is not a substitute. A failed
materialization is reported to every consumer, not replaced by a fake revision.

The RFC site tests still read the complete checkout ledger. Read-only corpus
assertions share process-local source collection, proof input and published
ledger snapshots, rather than rescanning the tree for every rendering assertion.
The helpers refuse a different checkout root. Tests that change a ledger or its source tree
use independent fixtures; production builds do not cache the collection. The
implementer disclosure is checked within each implementation-kind section of
the complete generated HTML page and its Markdown mirror.
<!-- source: internal/le/site/rfcdetail_test.go -- publishedLedgerOfThisCheckout -->
<!-- source: internal/le/site/rfccompliance_test.go -- TestThePublishedPageSaysWhoImplementsEachDocument -->

The two full-checkout facts assertions likewise share one result from the real
facts producer, including its live test-health input. They assert both answered
counts and source provenance; controlled mutation fixtures remain independent.

## Release roadmap

`/project/roadmap/` is generated from committed `HEAD` on every full or partial
build. The `roadmap` producer calls the shared spec collector once and renders
its Markdown through the docs renderer. The page, `index.md`, and
`data/release-roadmap.json` describe that same snapshot. The JSON records the
schema version, full revision, and digest of the plan inputs.
<!-- source: internal/le/site/roadmap.go -- renderRoadmap -->

The output is a classified planning inventory. It separates release fixes,
validation and delivery from post-release features, engineering and proposals,
including unknown metadata and parked states. Priority and release rationale
describe planning intent, not proof of correctness. Counts do not prove readiness
or delivery. Pending edits appear after commit and regeneration.
The feature page links this inventory rather than maintaining pending cards.
<!-- source: internal/le/spec/roadmap/roadmap.go -- Collect -->
<!-- source: internal/le/site/datapages.go -- featuresBody, featuresMirror -->

The feature page's cards come from `website/data/features.json`, but no card
states its own maturity. Each card names the `features/*.md` declarations it
describes, and the build derives its section and badge from them. A card that
no declaration backs stops the build. `docs/features.md`, which the docs
producer publishes at `/reference/feature-status/`, is rendered from the same
declarations. The rules are in `docs/contributing/feature-maturity.md`, "The
published surfaces".
<!-- source: internal/le/site/datapages.go -- loadFeatureData -->

Card colours identify subjects across the website. Use the existing categories
`operate`, `routing`, `services`, `automate`, `observe`, `secure`, and `platform`;
use `meta` for mixed subjects, general editorial or project work. The
[style guide](../../website/style-guide/index.html#sg-category) defines the
palette and examples. When a card names a feature already in
`website/data/features.json`, keep that feature's category. A category does not
express maturity, health, test success or RFC compliance; those remain separate
labels and status treatments.

All producer inputs use the shared category vocabulary in `docpage.go`.
`cardCategory` makes an omitted optional category neutral and rejects unknown
values. Feature and milestone inventories use `validateSubjectCategory` to
retain their seven-subject filters and reject neutral entries.
Blog front matter can state `category:` for a clear subject. Weekly cards use a
subject colour only when all their topics agree, while every topic chip retains
its own category. Reordering a list never changes a card's colour. Plugin cards
use their generated area's subject independently of their navigation filter
bucket. Do not add presentation tones, position cycles, hashes, or inline hues.
<!-- source: internal/le/site/docpage.go -- cardCategory, validateCategory, validateSubjectCategory -->
<!-- source: internal/le/site/changes.go -- cardCategory -->
<!-- source: internal/le/site/plugins.go -- pluginGroupCategory, pluginBucketOf -->

The feature catalog uses the shared category palette. Its filter rail combines
text, category and maturity, with one reset action and a live result count.
Every card states its maturity in words as well as its border treatment. Those
labels and filter values come from the same derived state that places the card
in its section; the page never freezes a mockup's counts. The sections, cards,
bullets, chips and links stay in source order, and the full catalog and category
counts remain readable without JavaScript. The Markdown mirror carries the same
maturity labels and complete card content.
<!-- source: internal/le/site/datapages.go -- featuresBody, featureCardHTML, featuresMirror -->
<!-- source: website/assets/js/site.js -- initFeatureFilters -->

The build does not read a local `plan/release/roadmap.md`. Source links name the selected
commit, and an unreadable or missing `plan/` stops the build. Verify privately
with `./le site build output <session-scratch-directory>`, then inspect the
roadmap with JavaScript disabled and a narrow viewport.
<!-- source: internal/le/site/roadmap.go -- renderRoadmap -->


## Plugin catalog

The website plugin catalog at `../gh-pages/reference/plugins/` is
generated, not hand-authored. Its data source is each plugin's
`registry.Registration`: name, description, config roots, dependencies,
optional dependencies, startup ordering, and the YANG schema it registers.
<!-- source: internal/component/plugin/registry/registry.go -- Registration metadata -->

Two facts the catalog shows are DERIVED rather than declared, so do not look for
a field to fill in. The source directory is the package the plugin's engine
function was compiled in. The YANG file list is every `.yang` file in the
directory that holds the module the registration carries, and beside the
package when it carries none.
<!-- source: internal/le/repo/inventory/plugins.go -- pluginPackageDir, pluginYANGFiles -->

A build reads the plugin registrations, writes
`../gh-pages/data/plugin-registry.json`, and renders the catalog plus one local
detail page per plugin.

The registrations published are always the Linux build's, because Ze ships only
on Linux and some plugins register from a file built only there (memlock,
iface-dhcp and iface-ra, for example). On Linux the build reads its own
registry. On any other host, such as a Mac, its own registry is smaller with no
error, so the build never reads it: it cross-compiles a linux `le` into the
checkout's `tmp/` with the lab recipe (`interoplab.StageBinaries`, `linuxle.Base`
and every feature gate, for the Docker daemon's architecture), runs
`le repo inventory | json` in a one-shot container over the checkout mounted
read-only, and publishes the plugins it printed. This happens before the daemon
compile. When Docker does not answer, or the container fails, the build stops
with the reason and the fix (start Docker, check `docker info`, then
`./le site build` again); it never falls back to the host's registry.
<!-- source: internal/le/site/pluginregistry.go -- pluginRegistryFor, readLinuxPluginRegistry, linuxRegistryRefusal -->

The command catalog and the configuration tree below need no container: a
command whose handler is Linux-only registers a non-Linux stub under the same
name, so a darwin daemon answers both byte for byte as a linux one does
(compared on 2026-10-29).

The build reads the daemon's own configuration schema, writes
`../gh-pages/data/yang-config-tree.json`, and renders the configuration
reference from that tree and the same registrations. Do not add a parallel
hand-written plugin list.
<!-- source: internal/le/site/build.go -- refreshNativeSurfaces -->
<!-- source: internal/le/site/plugins.go -- renderPluginCatalog -->
<!-- source: internal/le/site/config.go -- renderConfiguration -->

Startup ordering is published as `start_after` in the site registry JSON.
The catalog and detail pages label it **Start after**, and the target's detail
page labels the reverse relationship **Starts before**. These are order-only
edges when both plugins are selected, not required or optional dependencies;
they never auto-load the target. The Markdown mirrors and generated `llms.txt`
retain this distinction.

A plugin the catalog no longer carries loses its page: a build removes every
detail directory whose plugin is not in the registry it just read.

A config root that names no node of the configuration schema STOPS the build.
The owning plugin's section would otherwise publish as core, with its owner and
its YANG source silently absent.

When adding or changing a plugin, update the registration metadata in the
plugin's `register.go`. If the website needs another fact, add a structured
field to `registry.Registration` first, then render from that data. Regenerate
the site with `./le site build`.

## Command surfaces

Every published command page is generated from one file. A build compiles
`cmd/ze` once, with `ze_core` and every gate `feature-gates.txt` declares, into a
temporary directory it removes afterwards. `publishCommandCatalog` asks
`internal/le/doc/yangcontract.CommandCatalog` for the answer that binary's
`ze help command --json` gives, and writes it to
`../gh-pages/data/cli-commands.json`. The configuration tree above is read from
the same binary. The producers below read that file and nothing else.
`yangcontract.BuildDaemon` performs the compile, under a 5-minute bound of its
own, and the documentation drift check (`./le doc check all`) compiles its
live catalog the same way. Each run of the compiled binary has a 2-minute bound
that covers the command alone, so a slow compile on a loaded host does not kill
the catalog.
<!-- source: internal/le/site/build.go -- refreshNativeSurfaces, publishCommandCatalog, liveCommandCatalog -->
<!-- source: internal/le/site/yang.go -- liveDaemon, runYANGConfigTree -->
<!-- source: internal/le/doc/yangcontract/command_surfaces.go -- BuildDaemon, loadLiveCommandCatalog, runCommandCatalog -->
<!-- source: internal/le/site/catalog.go -- catalogFile, loadCommandCatalog -->

| Published route | Producer | What it renders |
|-----------------|----------|-----------------|
| `reference/cli/` | `internal/le/site/commands.go` -- `renderCLIReference` | One row per command, plus the pipe-operator table `renderOperatorGuide` writes into the same page |
| `reference/command-equivalents/` | `internal/le/site/equivalents.go` -- `renderCommandEquivalents` | The vendor map index, joined with `website/data/command-equivalents.json` |
| `reference/command-equivalents/<slug>/` | `internal/le/site/equivalentdetail.go` -- `renderEquivalentDetail` | One detail page per mapped command |
| `llms.txt` | `internal/le/site/derived.go` -- `renderLLMS` | One line per command, for a machine reader |

A command carries two help texts, and each surface reads the one it has room
for. `short-help` is the one-line summary, and all four producers print it
whole. `description` is the explanation, and only `renderEquivalentDetail` prints
it, as the detail page body. No producer derives one text from the other, and
none cuts either one. `docs/architecture/api/commands.md` holds the same table
for every other surface.

Registry help is literal text, not authored Markdown or HTML. The Markdown
mirrors and `llms.txt` escape Markdown punctuation rather than stripping it, so a
placeholder such as `<destination>` remains visible and a leading `-` stays text
rather than starting a list. Usage stays in a code
span, where prose escapes would change the displayed command. The drift reader
compares visible text: neutral emphasis, links and HTML wrappers are allowed,
but an unescaped tag that hides a placeholder is lost contract text.
<!-- source: internal/le/site/commands.go -- markdownProse, commandMirrorDescription -->
<!-- source: internal/le/site/derived.go -- writeLLMSCommands -->
<!-- source: internal/le/doc/yangcontract/command_surfaces.go -- markdownInlineVisibleText, validatePrimaryMarkdownContract, validateLLMSCommandContract -->

The command directory presents the complete catalog beside a group navigation
rail. Search still offers command suggestions and jumps to a stable `cmd-`
anchor; a group selector, result count, reset action and expand/collapse actions
help narrow a long reference. Search reads the complete command row, including
arguments and pipe details. Arguments and output details use native disclosures,
and the four table columns retain the shared column controls. Every group is
open in the generated HTML, so the complete catalog and its disclosures work
without JavaScript. A direct command link opens the containing group when the
interactive directory is active. The global pipe guide and JSON download remain
available beside the generated commands.
<!-- source: internal/le/site/commands.go -- cliReferenceBody, writeCommandGroup, writeCommandFacts -->
<!-- source: website/assets/js/site.js -- initCliCatalogFilter -->

`internal/le/doc/yangcontract` publishes no page. Its unexported `renderCommandSurfaces`
writes a contract fixture into a temporary tree, and the documentation drift
gate reads that fixture and each published page with one reader, so a fixture
the reader rejects is a reader defect and a page it rejects is a page defect.
The fixture therefore carries the shape the site writes: the usage line after
the summary, the command facts after the usage line, the detail page's pipe
labels on both detail surfaces, the command's own pipes and aliases as the
detail mirror lists them, and the `usage` segment on the `llms.txt` line. The
reader reads the mirrors' stated absences (`not declared`, `none`, the bare
`Pipes` term) as absences, and counts a command's index row once, under the
full catalog heading, though the index lists a mapped command twice. The symbol
was exported until 2026-08-29, and a build that called it overwrote 396 pages
with the fixture.
<!-- source: internal/le/doc/yangcontract/command_render.go -- renderCommandSurfaces -->
<!-- source: internal/le/doc/yangcontract/command_surfaces.go -- validateGeneratedCommandSurfaces, htmlDescriptionAndUsage, equivalentMarkdownCommandIdentities -->

## Quality pages

`../gh-pages/quality/health/` and `../gh-pages/quality/rfc-compliance/` are
rendered from the tree being built, through the two packages that own those
numbers. Neither page computes a figure of its own.

The testing-health page reads `internal/le/test/health.Render`, which answers the
metric record and the Markdown mirror in one pass. The mirror it publishes is
`docs/features/test-health.md`'s own bytes, so the site is never a second author
of that document. Every figure the page itself prints is read back from the
`health` and `health_trends` sections of `data/site-facts.json`, which
`healthFacts` derives from the same record, and printed as a marked element
(`docs/architecture/site-facts.md`, "One published copy of each value").

`../gh-pages/reference/rfcs/` is rendered from the same tree, and it is the one
page of the docs producer that is. Every other published Markdown source is
authored and is read from the checkout; `docs/features/rfc-status.md` is
generated from each summary's `## Meta` table and is not tracked, so the site
asks `rfc.StatusPage` for it instead. The renderer, the link rewriting and the
Markdown mirror are the docs producer's own, unchanged: only where the bytes
come from differs, and `liveDocSources` is the one table that says so.
<!-- source: internal/le/site/docs.go -- liveDocSources -->
<!-- source: internal/le/rfc/render_ledger.go -- StatusPage -->

The RFC compliance report reads `internal/le/rfc`: `Collect` for the
requirements and the test tags, `NewRenderInput` for the public ledger and the
recorded audit verdicts, and `Check` for the verdict and the open issues. It
writes `../gh-pages/data/rfc-compliance.json`, which is the same answer in
machine-readable form and is linked from the page.

`../gh-pages/quality/rfc-compliance/profiles/` is the priority tree in public:
one page per file under `rfc/profiles/`, each use-case profile and each
technology aggregate, and an index listing them with the release profiles
first, then the other use cases, then the aggregates. Each index row carries
both figures, the RFC and not-walked counts, and whether the mandatory
register is complete, so a 100% bar beside an unwalked RFC cannot read as
done. A row's RFC count links to the compliance index with the profile's
counted RFCs named in the link (`?profile=<name>&rfc=<stem>,<stem>#rfc-list`).
There a script shows only those rows of the enrolled and not-enrolled tables,
names any listed RFC with no summary, and offers a text filter by RFC number
or title; without scripts every row stays visible. Such a link also moves both
RFC tables to the top of the page, under its title, so the reader does not
scroll past the gate cards to reach them. Above the rows, an overview
sums the shown RFCs' mandatory and optional figures the way a profile rollup
does (walked RFCs only, the rest counted apart), so a profile's link reads that
profile's percentages. Every row of both tables carries the RFC's mandatory and
optional proof bars, followed by attainable ceilings and per-kind exclusion
counts and shares. These come from the corpus graph, including RFCs no profile
lists. The enrolled table uses short column names; its legend states the
computed ceiling over the RFCs shown. Linux limits, lower-layer behavior,
not-applicable requirements and declined optional features stay in the proof
denominator but are excluded from the attainable count. The statement names
the lowest nonempty mandatory and optional RFC ceilings. A page carries both coverage figures of its rollup and the not-walked
count, then its RFCs grouped by DAG level, roots first, each with a bar for
the mandatory and the optional figure. For a use case those are its required
RFCs, the only tier its figures count. Its optional RFCs follow in a separate
"Optional (not counted)" table with their own bars and their own rollup. An
RFC that is not walked keeps an empty dashed bar and says so in words. The
figure cards name the keywords each figure counts from `rfc.MandatoryKeywords`
and `rfc.OptionalKeywords`. A use case's rows also name the summary whose
`Use cases` or `Optional use cases` row declares each RFC. The `rfc-profiles`
producer reads the `profiles` of the published requirement ledger, which
`rfc.PriorityOf` fills, so a page counts what `./le rfc tree profile <name>`
counts. It refuses a ledger with no profile, and it retires a page of a profile
that no longer exists by the marker its pages carry, never by name. The
compliance index links to it under "Coverage by profile", the first section
after the gate verdict, which makes the compliance index the all-RFCs view.

A use-case page under `website/use-cases/` declares which profile measures it
by linking to that profile's page. The producer reads those links and gives
each profile page a link back to every use-case page that names it, so the
pairing is written once and the two directions cannot disagree. A link to a
profile no file under `rfc/profiles/` declares refuses the build. The homepage
regression-prevention block and the Examples menu both link to the profile
index.
<!-- source: internal/le/site/rfcprofiles.go -- renderRFCProfiles, rfcProfileGroups, rfcProfileBarHTML, removeRetiredRFCProfilePages, rfcProfileUseCases, rfcProfileUseCasesKnown -->
<!-- source: internal/le/site/rfcfilter.go -- rfcProfileListHref, rfcFilterHTML, rfcFilterScript, rfcStemRows, rfcIndexLegend, rfcCeilingStatement -->

Ceilings describe current classifications, not an immutable Linux maximum.
Not-applicable and feature-declined classifications can be revisited. The
browser reads `data-ma`/`data-oa` attainable counts and `data-me`/`data-oe`
per-kind exclusion counts from each RFC row, plus registered reason labels.
It sums only walked rows, as `rfc.CeilingOf` does, and compares exact count
ratios to identify the lowest ceiling. Ties put numbered RFCs first in numeric
order, then other stems by name, matching the Go model. Profile links and text
filters update the same sums. Empty figures say `no rows`, never a percentage;
an unwalked RFC has no published ceiling. The JSON ledger retains its proof fields and
adds attainable and excluded counts to the graph, profiles and each stem's
`figures` member. Profile cards, profile tables, detail pages and their
Markdown mirrors read those same model counts.
<!-- source: internal/le/site/rfcfilter.go -- rfcFigureRowCells, rfcCeilingStatement, rfcFilterScript -->
<!-- source: internal/le/site/rfcledger.go -- rfcLedgerStem, requirementLedgerFrom -->
<!-- source: internal/le/site/rfcprofiles.go -- rfcProfileCards, rfcProfileBody, rfcProfileMirror -->
<!-- source: internal/le/site/rfcdetail.go -- rfcDetailCeilingMirror -->

One page per RFC sits under that route, one for every summary in `rfc/short/`.
The compliance page itself is the index over them: two link tables, one for the
enrolled summaries and one for the summaries that are not enrolled, so no page
of the family is reachable only through search. 39 stems carry no row in
`docs/features/rfc-status.md`, which is why the index lives here rather than on
the mirror of that page.

Each detail page opens with the card grid the index carries, over that RFC's own
numbers. **Scale leads, then standing**, on both pages: `Gated MUSTs` and `Out
of scope`, then the shares that partition the binding population, then the proof
ratio. Every card carries its value, the arithmetic under it, and a sentence
saying what the measure means. `rfcCardsHTML` renders the grid for both pages
and `rfcCardsMirror` states it in both mirrors.

**The ratio cards partition their denominator.** `rfcStanding` groups every
binding bucket into exactly one card, so the shares add to 100% and a reader who
adds them lands on the whole. Publishing two parts of a four-part split left
3.3% of RFC 4271 unexplained. `rfcLedgerCoverage.Bucket` is the one translation
between the index's bucket keys and a stem's own counters, so the two pages
cannot publish different partitions of one idea. The proof ratio is over TAGGED
UNITS, a different population, and the sentence under the grid says so.

The `linux_limit` bucket is labeled **Linux cannot do this** and has a neutral
tone. Its obligation still binds Ze and remains unmet. It is neither proven
nor removed from any denominator. The stem coverage JSON counts it under
`linux-cannot`, separately from ordinary gaps and scope classifications; the
row's annotation names the Linux Limitations Register entry.
<!-- source: internal/le/site/rfccompliance.go -- rfcSatisfaction, rfcStanding, rfcLedgerCoverage.Bucket -->
<!-- source: internal/le/site/rfcledger.go -- rfcLedgerCoverage, rfcLedgerCoverageOf -->

The one bucket in no card is `rollup`, marked `Derived` in `rfcSatisfaction`. A
`{rollup}` row carries no obligation of its own, and `rfc.CoverageRows` never
counts it. So the bucket is outside the gated population, and it enters no
card, no tape segment and no row of the bucket table. The gate derives each
such row's state from the rows it names. `rollupDeriver.fill` writes `Requirement.Derived`
for the gate and for `rfc.NewRenderInput`, so every renderer reads the state
the gate reported.

The stem page lists those rows under `Derived from other rows`, apart from the
parts and marked as outside the population. Each row's marks carry a `derived`
mark beside the `{rollup}` reason that names its targets. The mark reads `met`,
or `gap` or `unproven` with the cause naming the target that decided it.

**A color names what the measure MEANS, never how well Ze scores on it.** Green
is a good outcome at any value, red a bad one above zero, and neither a
population nor a scope count is an outcome, so both take the neutral tone. The
number under the label already carries the performance; a color that graded as
well as labeled put an amber card on the measure that is the good news.

**The grid reads in four movements**: Overall, Positive, Neutral, Negative. A
card's movement is DERIVED from its tone, `rfcCardsIn`, so a heading and a color
cannot disagree: a card under "Negative" is red because that is the only way it
lands there. **The grid holds measures only.** How the gate is enforced -- the
pre-commit stage count, the reproduce command, the inputs it reads, the
artifacts it publishes -- is one section, "How this is checked". `Pre-commit
gate / ON` was a card until 2026-09-01; it answers how, not where Ze stands.

**The index is written for a reader who is not a contributor.** Under the
title it carries "How to read this page", two paragraphs and a glossary that
explains each project term once (gated, tagged unit, recorded break, escaped,
lapsed, walked, each annotation kind). Every section opens with one plain
paragraph saying what it shows and why it matters to someone choosing a router.
The words live in `rfcreader.go`, and each glossary entry restates what a card,
legend or section already says, so it adds no claim of its own.
<!-- source: internal/le/site/rfcreader.go -- rfcGlossary, rfcReaderIntroHTML, rfcLeadHTML -->

**Check results leads with one plain line for each group of checks**: its
name, Passed or the number of problems, what the group verifies, and for a
failed group what that means for the figures. The groups are declared once in
`rfc.CheckGroups`, and every check `rfc.Check` runs files its findings under
one through `inGroup`, which `rfc.Finding.Check` carries, so nothing on the
page parses a message to learn which check raised it. A finding that names no
group is counted on an "Other checks" line rather than under a guess. Under the
list, a collapsed block for contributors carries the table, one row per finding:
the RFC, the requirement id linked to its own page and row, the level, what is
wrong, and the requirement's own text. Those columns come from `rfc.Finding`,
which carries the parts each check had BEFORE it formatted its line.
`CheckReport.Findings` is the one list and `Violations` is rendered from it. A
finding about a file, a ratchet or a ledger row carries no requirement and
states itself in the column it fills.
<!-- source: internal/le/rfc/checkgroups.go -- CheckGroups, inGroup -->
<!-- source: internal/le/site/rfcreader.go -- rfcCheckLines, rfcCheckLinesHTML -->

**Two mechanisms take an obligation off the gated ledger, and the index
publishes both.** `{not-applicable}` annotates a requirement that EXISTS, and
the Out of scope card carries it. An excluded site never becomes a requirement:
a reviewer walked the RFC's text sentence by sentence and declined to map that
one. The Exclusion disclosure section counts those by kind, reads the vocabulary
and each kind's meaning from `rfc.ExclusionKinds` and `rfc.ExclusionKindMeaning`,
and links every summary that used a kind to its own page where the reason is.
It states its own coverage on its face, because sign-offs exist for a minority
of summaries, and it states that `ai/rules/rfc-compliance.md` treats
`binds-another-role` as PRESUMED WRONG until justified -- publishing the largest
kind without that context would be the flattery failure one layer down.

**The kinds do not all mean the same thing, and the section splits on that.**
Five say the obligation never bound Ze. `relocated-to-spec` says the opposite:
it is real, Ze owes it, it is unbuilt, and a named spec owns it, which
`./le rfc check` verifies still reserves the requirement id. Those sit under
their own heading with the reserved id, the quoted sentence and the owning spec,
and no count sums them into "declined". The split is `ExclusionKindGroup`, so a
seventh kind lands in one group or reddens a test rather than defaulting to
scope.

**The page names where a fact is AUTHORED.** `rfc/enrolled.txt`,
`rfc/not-enrolled.txt` and `docs/features/rfc-status.md` are generated by
`./le rfc update` from each summary's `## Meta` table, so the published
prose names that table and the ledger reads `RenderInput.Metas` rather than the
generated copies. The enrolment and disposition renderers carry no fallback
branch: `ParseMeta` refuses a summary with no `| Enrolment |` row and refuses a
kind with no reason, and `loadRequirementLedger` refuses the state by name at
the artifact boundary rather than printing a placeholder for it.

**Every ratio is taken over the GATED population**, and no annotation takes an
obligation out of that denominator. `{not-applicable}` is scope rather than
coverage, so it is a named slice with a neutral card of its own rather than a
subtraction: removing it flattered every share above it, and the owner ruled
against that on 2026-09-02. `{lower-layer}` is a second neutral slice, added on
2026-09-03: the obligation binds Ze and a layer under Ze meets it, on state Ze
installs, so it is neither proven by Ze nor work Ze owes.
`{feature-declined}` is a third, added the same day: the obligation is
conditional on a feature the RFC makes optional and Ze does not offer, so its
condition is false and nothing is owed. That one is counted with
`{not-applicable}` in the `Out of scope` card, whose note names both kinds and
what each one says (`docs/contributing/rfc-conformance-gates.md`). The gated count keeps its place below the ratios as
the accounting total it is, and the bucket table states it as the sum of the two
populations. The page led with that count until 2026-09-01, when the owner
called the arrangement deceptive: a count of obligations judged reads as a count
of obligations met.

The mapping from annotation kind to site bucket comes from
`rfc.AnnotationTraitsOf`, the same declaration that drives the RFC gate's
coverage and rollup rules. `rfcAnnotationBucket` reads that mapping rather than
enumerating kinds again. A kind the declaration does not know is counted as
unmapped, never silently dropped from the accounting.
<!-- source: internal/le/site/rfccompliance.go -- rfcAnnotationBucket -->
<!-- source: internal/le/site/rfcledger.go -- rfcLedgerCoverageOf -->

**The proof ratio sits immediately after the shares it corrects**, because a
test pair is not a proof. A break is observed ONCE and never re-run: what
`verifyOneDiscrimination` (`internal/le/rfc/discriminate.go`) re-checks on every
run is that nothing the red rested on has moved, so the unit still carries the
tag and the unit's behavior, the tag's claim and the producer's behavior still
hash to what the record stored. The card says that in its own words. An escape
claiming no break exists is not a proof, and neither is a LAPSED record whose
ground has moved: both are counted apart, and each appears under its own
requirement id in the proof-state section.

Under it the page carries the requirement table `rfc/requirements/<stem>.md`
carries, cell for cell, plus the requirement text, the per-RFC coverage
counters, every declared gap (naming the test that demonstrates it, where one
does) and every gated MUST with no test, the recorded
audit verdict and its freshness, what stands behind each tagged unit, the
extraction sign-off, and where a superseded obligation now lives.

Five shapes are load-bearing on that page:

- **A requirement id is never bare.** Its own row carries the anchor
  `id="<lowercased rid>"`, and every other mention -- a coverage bucket, a gap
  row, a proof-state heading, a superseded row -- links to it and carries the
  requirement's text where the mention is a row.
- **The proof state is one row per tagged unit**, with the polarity, the test
  file, the test function, the `kind/tier` carrier and the proof state each in
  its own cell. The sentence that explains `unproven` is a legend above the
  table, stated once.
- **Prose is not a table cell, and not one blob either.** The enrolment reason
  and the public ledger's cells sit under their own headings. A Coverage cell is
  a semicolon-chained list of claims and renders as one item per claim; a
  Remaining cell is a lead sentence and authored "Theme: body" groups and
  renders as those. `internal/le/site/rfcprose.go` does the split, with a depth
  counter so a semicolon inside `(...)` or inside a `{ med; }` code span never
  becomes a cut. The words are NEVER rewritten, paraphrased, summarized or
  truncated: the split is lossless, every item is balanced, and a cell carrying
  no such structure renders whole. Inside it every requirement id the RFC
  declares links to its row and every repository-rooted path links to its file;
  an id the RFC does not declare and a relative path citation are left as the
  author wrote them, because a link nobody can follow is worse than none.
- **Every table scrolls inside its own container**, `div.rfc-table-wrap`, the
  convention `.cmd-eq-table-wrap` already holds for the command family. A test
  path is one unbreakable token, so the page body would scroll sideways without
  it.
- **The requirement's sentence LEADS its row**, always visible, with no
  disclosure. Each requirement is two rows: the anchored id and its quoted
  sentence beside it spanning the rest of the row, then the level, the section
  and the tests. The
  sentence is what the row is ABOUT and everything else is metadata on it;
  hiding the subject while showing its attributes is inside out. It stays a
  TABLE rather than a block per requirement because the Proof state section
  already renders one block per requirement, and two lists of the same shape on
  one page help nobody. The colspan comes from `rfcRequirementColumns`, so a
  column added or removed cannot leave it wrong.
- **The tests read in carrier order**: kind first, tier within a kind, then
  polarity within a group, positive before negative. So every `unit/verify` row
  comes first, then `functional/verify`, then `interop/nightly`, and a reader
  sees at a glance whether an obligation is carried by unit tests or by a
  nightly interop run. The order is `rfc.CarrierRank`, declared in
  `internal/le/rfc/carriers.go` beside the kinds and tiers it orders, so a kind
  added to the carrier table cannot sort last on this page in silence;
  `TestEveryCarrierTheVocabularyDeclaresIsRanked` holds that over the real
  table. The full order is unit, functional, editor, interop, unknown by kind,
  and verify, nightly, unrun within each. A carrier the vocabulary does not rank
  sorts after every ranked one rather than at rank zero, and an absent-polarity
  row takes the rank of the requirement's best carrier so the gap shows inside
  the group the eye is already reading.
- **The requirement id sits beside its sentence**, in a narrow left cell, with
  the sentence in a cell spanning the rest of the row.
- **One table convention for the whole family, and no opt-out.**
  `.md-content td:first-child` is bold and sticky for every table on the site,
  which reads an identity into a first column. Eight of the eleven tables this
  family renders put a LABEL there instead, so the family drops the weight for
  all of them and keeps the stickiness that earns its place in a horizontally
  scrolling container. Every table goes through `rfcTableHTML`, which is what
  makes that true by construction rather than by discipline; 228 bold rows read
  as shouting, and two conventions on one page read as an accident.
- **Each stem page proves its own arithmetic.** Its coverage table carries a
  total row saying whether the partitioning buckets account for the gated
  population, and each bucket says whether the count the gate produced and the
  membership the page names agree. The counts come from `rfc.CoverageRows` and
  the membership from a second walk over the same requirements, so the two can
  disagree; the index carries the same shape of check against the gate's own
  gated total.
- **A page for a summary the gate does not hold never says it does.**
  `evaluate` (`internal/le/rfc/check_core.go`) skips every requirement of an
  un-enrolled RFC, so on those pages the population card names what the summary
  DECLARES and states that the gate holds none of it.
- **An un-enrolled kind is published with its meaning.** Six kinds exist:
  `enrolled` and the five dispositions `non-normative`, `backlog`, `blocked`,
  `source-restricted` and `out-of-scope`. The last says the extraction is DONE
  and the owner declined the feature, which is a scope decision rather than a
  conformance gap, and its public row may read only `Future` or `Unsupported`.
  The sentence for each comes from `rfc.DispositionKindMeaning`, so a seventh
  kind cannot print as a bare word.
- **Retiring a page is keyed on the marker this family writes**, never on a
  directory name absent from the live set. The output of a real build is the
  `../gh-pages` checkout, so a name test would delete any directory under
  `quality/rfc-compliance/` that another producer or an author put there.
- **The tests are a grid of divs**, one row per citation: the polarity, the kind
  and tier, then the test. The TIER LEADS, because a fixed-width token in front
  of a variable-width name wraps cleanly and lets the tiers align down the page.
  Divs rather than a nested table: a table inside a cell inherits the outer
  table's width pressure, which is what the restructure exists to remove. The
  block carries table roles so a screen reader meets data rather than a run of
  text, and it stacks per citation on a narrow viewport. A polarity with no test
  keeps its row and says so, because an absent polarity is a disclosed fact and
  a missing row reads as an oversight.
- **There is no Note column.** It carried four marks and each went where it
  explains something. The `{kind} reason` annotation and the nightly-only mark
  render beneath the tests they explain. The audit verdict and the superseded
  pointer are already rendered per requirement id under Proof state and
  Superseded, so they are not repeated. That was measured before the column
  went, not assumed: all 4 audit marks and all 310 superseded marks had those
  homes, while 374 of 375 `{single-polarity}` reasons and 14 of 851
  `{not-applicable}` reasons had none, which is why the annotation is rendered
  rather than dropped. `TestNothingTheNoteCarriedWasLost` holds it.
- **The Markdown mirror diverges here, and has to.** Markdown cannot nest a
  table inside a cell, so the mirror keeps labeled lines with the tier still
  leading each citation. Same population, same order, one shape it can express.

**A test is cited by NAME, never by the file it lives in.** The page printed
`internal/component/bgp/message/rfc4271_test.go` beside
`TestRFC4271MarkerAllOnesOnSend` until 2026-09-01; the path is machinery,
the link already resolves to the exact line, and the path cost width and gave
nothing. `internal/le/site/rfccitation.go` renders all three shapes: a Go test
and an interop checker by their function, a `.ci` or `.et` scenario by its own
file name, because the scenario IS the file. The whole path stays reachable as
the link's `title`, and as the link target in the mirror. Where TWO units on one
page share a test name -- three such collisions exist in this corpus, all three
with both units on one page -- the package directory is prefixed to both, and
only there, because rendering two different tests identically is the one thing
that change must not do.

The citations are built from the tagged units rather than from the shard's own
markdown, so the file, the line and the carrier are read as fields.
`TestEveryShardCitationIsATaggedUnit` holds that the two populations agree, over
10,768 cells.

Two links leave the page. A proof-state row links the TEST, not the path, to
`blob/main/<file>#L<line>` built from the line `rfc.Tag` records, so a reader
lands on the assertion. `repositoryBlobURL` and `repositoryLineURL`
(`internal/le/site/rfcmarkup.go`) answer every such URL, for this family and for
the documentation renderer, so the repository is spelled once.

The card colors are a vocabulary of four: `ok`, `neutral`, `warn`, `bad`. Every
card declares the RULE that chose its tone beside the tone, and
`rfcToneLegendHTML` publishes those rules under the grid. A number with no good
or bad direction -- a population, for instance -- takes `neutral` and gets no
color, because a color on a number is a verdict.

On the index page the requirement buckets carry a `Gated MUST-level requirements`
total for the implemented-RFC population and a sentence saying whether every
requirement in that population falls in exactly one bucket. The tape above it carries no text of its own:
a bucket at 4.5% of the width had a label wider than its segment, so the small
buckets were the unreadable ones. The tape is the proportion, the key beneath it
is the words, and every bucket the vocabulary declares has a color rule.

A section with nothing to show says so in one sentence. An omitted section and
an empty one read the same to a reader, and only one of them is a fact.

The gate verdict on the index page is a status block, `div.rfc-verdict`, tinted
by the same tone vocabulary as the cards. It was a `<pre>` block until
2026-09-01, which gave a one-line verdict terminal styling and the copy button
`website/assets/js/site.js` attaches to every `pre`; both told a reader to paste
a status into a shell. The invocation that reproduces the check is the only
thing on that block a reader runs.

The disclosure is FULL, by owner ruling of 2026-09-01. A gated MUST with no
test, an audit verdict of `weak`, `wrong` or `unimplemented`, a verdict that is
stale or shifted, a tagged unit with no discrimination record, and a `no-break`
record are each named on the page under the requirement id they belong to. A
count may accompany the list and never stands in for it.

For a partial requirement, the proof block states that the records cover only
scoped tag claims and give zero whole-requirement credit. The gate population
is the gated MUSTs of enrolled RFCs; published satisfaction shares use the
implemented RFCs within that population. Rollup rows belong to neither count.
These figures come from the native RFC collector, not hand-maintained totals.

The input is `../gh-pages/data/rfc-requirements.json`, derived once per build by
`publishRFCLedger` before any producer runs, in the way the plugin registry and
the command catalog already are. Beside the per-stem entries it carries the
dependency graph (`graph`: nodes with level, cycle and both figures, edges with
their kind) and one priority tree per profile (`profiles`), from
`rfc.PriorityOf` over the same render input. It is one reading of the checkout through
`rfc.Collect` and `rfc.NewRenderInput`. It never calls `rfc.Check`, which is a
second full pass that also type-checks every tagged package; the aggregate page
pays for that once and publishes the verdict it answers.

Two figures the retired renderer published are gone rather than ported. It
counted a per-check error total, which the Go gate does not answer: it returns
one list of open issues, and the page publishes that list. It also grepped a
marker string out of a hook file, a Makefile target and a status script, all
three of which were deleted with the interpreter, to claim an agent guard was
ON. The page states the live verification wiring instead, read from the declared
pre-commit stage population.
<!-- source: internal/le/site/health.go -- renderHealth -->
<!-- source: internal/le/site/rfccompliance.go -- collectRFCCompliance, rfcSatisfactionRows, rfcAccountedNote -->
<!-- source: internal/le/site/rfcledger.go -- collectRequirementLedger -->
<!-- source: internal/le/site/rfcdetail.go -- writeRFCDetailPage -->
<!-- source: internal/le/site/rfcevidence.go -- rfcProofHTML -->
<!-- source: internal/le/site/rfcmarkup.go -- rfcTableHTML -->
<!-- source: internal/le/verify/engine/stages.go -- fullStages -->
