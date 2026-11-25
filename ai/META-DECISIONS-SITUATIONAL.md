# Owner Meta Decisions -- Situational

Rules for one kind of work, as Thomas has decided them. Neither main sessions
nor subagents load this file unconditionally: read a section only before the
work its `Read before:` paragraph names. Always-applicable and main-session
owner rulings live in `ai/META-DECISIONS.md`.

# Rules for one kind of work

Each section below binds only an agent doing the work its first paragraph
names. The start hook lists every section, with that paragraph and its line
range, to an agent that starts without the instruction files; every other
agent, like a main session, loads all of it.

## Release scope and spec planning

Read before: deciding first-release scope (fleet management, opt-in unreleased
features), claiming or closing specs, checking WIP caps, or running
child/dependency specs.

- Fleet management ships in the first release with its device registry,
  device health reports, metric history, the alarm list, alarm causes (an
  alarm hidden while the alarm that causes it is open) and configuration
  drift detection with repair by the hub. Their specs live in
  `plan/release/features/`. In the first release these features are reached
  through commands, on every command surface; their web pages, the hub's task
  model and panel, notifications and live page updates come after it.
- Opt-in unreleased capabilities: a capability that must be explicitly enabled
  is not part of Ze's default setup, is not released as a first-release
  feature, and is not covered by the first-release quality process. Its known
  issues, interop failures and open proof debt live in its post-release spec
  under `plan/post-release/features/`, not in `plan/release/fixes/`.
- Close in-progress specs before opening new ones. The WIP cap is lowered as
  specs close, never raised.
- Two limits apply to spec work, and they count different things. The open
  limit counts every spec that is started and not closed, a started spec set
  included, whether or not anyone is working on it: it is the reason to close
  before opening. The active limit counts the specs being worked on right now,
  each held under a live claim: it bounds how much runs at once. The WIP cap
  the other rules here name is the open limit.
- A spec whose work is committed may be claimed above the WIP cap only to
  close it: the closure agent sets `ZE_SPEC_WIP_CAP` one above the current
  in-progress count for that claim, closes the spec in the same run and
  releases the claim. That is not a raise of the cap, and it is never used to
  start work.
- A spec's main session may run that spec's child specs, or specs its own work
  depends on, through agents in the same session. The main session keeps its
  claim on the spec it supervises, and each agent claims the spec it runs
  under its own agent claim. Every such claim counts toward the WIP cap like
  any other: this permits no claim above the cap. The main thread still
  verifies each report and runs every test, one CPU-intensive job at a time.

## ExaBGP compatibility

Read before: working on ExaBGP compatibility (bridge, migration, tests), or
what Ze's BGP accepts or refuses (AS_SET, OPERATIONAL, multi-session).

- ExaBGP 6.0, 5.0 and 4.2 backward compatibility is agreed, so what it needs is
  done, not asked. It lives only in the bridge and the migration.
- The RFC outranks ExaBGP API compatibility, which outranks ExaBGP's behaviour.
  Where ze is stricter than ExaBGP without an RFC MUST behind it, ExaBGP's
  behaviour wins; where the RFC says MUST, ze keeps it and documents the
  difference.
- Compatibility means the same command produces a valid BGP message with the
  same effect on the remote router, not a byte-for-byte copy.
- AS_SET: a route from an ExaBGP script through the bridge may carry one, and a
  received one is accepted on a peer whose `session as-set accept` the operator
  set; `ze exabgp migrate` writes it for an ExaBGP `as-set accept` peer, and the
  leaf stays available to any operator. Ze's own API and plugins never
  originate one.
- Ze does not support the BGP OPERATIONAL message or BGP multi-session. An
  ExaBGP test that depends on either meets a documented difference, which is not
  owed work.

## Test systems and labs

Read before: VMs, test guests, labs, kernel features/modules, cross-machine
work or arch selection, `sudo` stress harness, host-setup commands in `le`, or
benchmarks and perf/alloc figures.

- On Thomas's two development machines (`ai/owner/machines.md`), work that is
  tested on the other machine is developed in a worktree of its own on the
  machine the agent runs on, on a branch of its own, never in the shared
  checkout. When the work is ready, the agent commits on its branch, and each
  test runs once, on the machine chosen for it: in that worktree on the
  agent's machine, or on the other machine, where the commits are carried
  through git and tested in a worktree of their own, detached at those
  commits. The other machine's shared checkout is never pulled into or tested
  for that work. The branch reaches `main` only when Thomas lands it with
  `git rebase <branch>`. Everything the agent writes for that work, scratch
  files included, lives in that worktree, under its own `tmp/`, or on the
  machine the work lands on, never elsewhere on the disk of the machine the
  agent runs on.
- When development uses more than one machine, to spread load or to reach
  another architecture, the checkouts are kept in step with git commands and
  nothing else. A change is made in one checkout and committed, and its
  commits travel to the other machines by git itself: directly from one
  development machine to another (a fetch or pull over SSH, or a git bundle),
  or through origin. Moving commits between development machines publishes
  nothing and needs no order from Thomas; a push to origin or to any public
  remote still does. An agent never edits, copies or patches a file in another
  machine's checkout over SSH, tracked or untracked: that machine runs tests,
  labs and builds of what git delivered, and anything a run writes goes to
  scratch outside the tracked tree. An agent may update another machine's
  checkout with `git pull --rebase --no-autostash` when its tree is clean; when
  the tree is dirty the agent stops and reports what it found, and never
  stashes, discards or overwrites to get past it.
- When Thomas names the machine or checkout where work is to land ("on my
  laptop", "on the server"), the agent finds out which machine it is on before
  its first edit, from `uname -sm` and `hostname`, and its first line says so
  when it is not the one he named. It never does the work where it happens to
  be and mentions the difference afterwards. The work still travels by git
  only, a commit here and a pull there, and it is not delivered, and nothing is
  reported as done, until the checkout he named holds the commit. A pull that
  refuses there is reported at once, with what blocks it.
- `sudo` is allowed only for the stress harness (`./le test integration
  stress`): one root run at a time, alone on the host, removing what it
  created, and leaving anything in a shared location usable by the owner's
  user.
- VMs, test guests and the lab Docker host run Ze's own kernel and system, never
  Alpine or a stock distribution kernel. A problem that exists only because a
  guest runs another system is out of scope: the fix is moving that guest to
  Ze's system. Colima may manage the VM on macOS, booting Ze's image only.
- A test that needs a kernel feature runs on Ze's kernel, which carries it.
- Ze running on a standard distribution kernel can load the kernel modules it
  needs with `modprobe`, as an optional feature of `ze` itself. `le` is
  development tooling and never carries a product feature. It acts only when
  the running kernel is not Ze's, and is proven on booted Debian, Ubuntu,
  Rocky and Fedora VMs: the one exception to Ze's kernel in test guests, and
  only for behaviour that exists off Ze's kernel.
- A performance figure (a number that is reported, compared or recorded)
  comes only from a machine whose performance is known: real hardware, or a
  VM running on a host whose performance is known, so the VM's resources are
  guaranteed. Thomas's Mac is such a host, and so is a real Linux host. A VM
  on a host nobody here knows, such as the Linux development VM, gives no
  figure: that host may overprovision its resources. On such a VM an agent
  may run a perf harness or benchmark to check that it works (it builds,
  runs and produces output), and never reports or compares the figures it
  prints. A Linux-only harness is measured on the Mac inside a Linux QEMU
  guest, whose resources the Mac guarantees.
- Memory allocation figures (bytes and allocations per operation) do not
  depend on the host's performance, so an agent measures, reports and
  compares them on any host, the Linux development VM included. Only time
  and throughput figures need a known host. Two allocation figures are
  compared only when the same Go toolchain and architecture produced them.
- Running on an unknown host limits only performance figures. Every test runs on whatever
  host the agent is on: on Linux, QEMU guests, netns runs, `caps=` and
  `needs-linux` tests included. Only a test that runs only on another OS
  waits for a host of that OS.
- A test runs once, on one machine. Go is multi-architecture, so a test of
  Ze's Go code that passes on one architecture is not repeated on the other:
  no second run is owed, asked for or started, and a spec never lists one as
  remaining work. Running the same test on both Intel and ARM spends CPU to
  learn nothing. When more than one development machine is available, the
  agent picks one for the run, normally the one with the least CPU in use at
  that moment.
- Kernel work uses the architecture of the machine the agent is on. Never
  cross-compile or emulate the other architecture's kernel locally; its proof
  comes from a host that runs it natively.
- A kernel is never rebuilt when a build of the same inputs exists, and never
  built twice for one set of inputs. A test, a record run or a lab never builds
  a kernel or a reusable image (the docker-host image): it boots the cached
  one, and on a cold cache it refuses and names the one command that builds
  it, which runs on its own, outside any test deadline. An appliance image that
  carries one run's own settings is assembled from the cached kernel, never
  with a kernel build. A finished build reaches the cache the moment it completes, so
  no timeout, kill or later failure discards it, and two runs that need the
  same build share one.
- A kernel bump does not re-run the test suite. The kernel keeps its
  interface to programs stable, and a regression in it is fixed upstream, so
  a feature proven on one kernel is not re-proven on the next. The pinned test
  kernel is still kept current. A bump owes only what that promise does not
  cover: the kernel build, the required-config and kernel capability checks
  (configuration options and Ze's own patches are not part of the promise),
  the re-read of the Linux limitations register, and the tests whose proof is
  behaviour the kernel produces or performs on Ze's behalf, because upstream
  may change that behaviour as a bug fix and only a test notices it for Ze.
- Third-party software in the interop and integration labs runs its latest
  stable upstream release; BIRD runs the BIRD 3 line. Software with no stable
  release (tac_plus-ng, freeRtr) is built from source, first updated to
  upstream's latest when the source in use is more than a month old.

## Developer tooling

Read before: adding or changing a command, keyword, value or option of `le`,
or a word the `./le` launcher consumes.

- `le help` covers all of le: every command, keyword and value, and every word
  or option the `./le` launcher script consumes before the binary runs (such as
  `--name` and `rollback`). Completion and `le help ai` cover the same set. A
  word le accepts that its help does not show is a defect.

## Website design

Read before: editing or publishing the website, its generator or CI, a blog
post, weekly update/ze-news, README, published docs, or Ze demos, screenshots,
or videos.

- The weekly update is published about every two weeks, not on a calendar
  week. Each one covers the commits from the previous update's `release-to`
  to the branch tip when it is drafted, so its span varies. The website calls
  each one an "update report"; the Discord post title, the command, the skill
  and the archive files keep the "weekly" names. It keeps the same length
  budget, three Discord messages and four at most, whatever the span.
- The update report is written in Thomas's tone, under the project's name: it
  reads as he would write it, never as a model's prose
  (`website/changes/discord/STYLE.md`, "Identity & voice").
- The update report's RFC section gives its figures as percentages, in plain
  words, and gives two of them, each saying what it counts: the share of all
  the MUSTs of the RFCs Ze implements whose test was seen to fail with the
  code broken on purpose when it was written (the check is repeated only when
  the test itself changes, never on every run), and the same share over only
  the MUSTs of what Ze builds.
  It says that a MUST for a part of an RFC Ze does not build needs no test
  until that part is built, so the second figure shows how well the shipped
  features are tested.
- An update report compares a figure with an earlier report only when both
  were counted the same way. When the counting changed, it explains why the
  number moved only when the reason is known; when it is not known, it gives
  today's figure alone and says nothing about the earlier one or about the
  counting, because an unexplained "the counting changed" reads as cooking
  the books.
- The RFC compliance page explains how compliance is calculated in plain
  language written for an engineer, not in internal tooling jargon: what each
  headline figure counts, how fault-checked tests work, why default behavior
  is judged separately from custom operator overrides, and how OS-provided
  behavior, superseded RFCs, and non-applicable roles are handled.
- Every change to how RFC compliance figures are calculated is recorded with
  its date, before and after counts, and plain-English reason in
  `plan/records/rfc-counting-history.md`, in a news fragment under
  `website/changes/fragments/`, and in the public Counting history note, so
  subsequent update reports can explain why figures moved.
- In blog articles, everything before the AI authorship disclaimer is a brief
  teaser. The developed argument, substantial background and technical
  explanations follow the disclaimer. The deck also leaves the explanation
  for the body.
- Website design follows the approved principles in
  [the website style guide](../website/style-guide/index.html#sg-design-principles),
  including its information-preservation map and owner approval for removals.
  This decision applies to the website only.
- What Thomas has ruled about the website's look or wording stays until he
  changes it. Each ruling is recorded in `website/AI.md`, "Owner rulings",
  applied at the scope he stated in one rule that later elements inherit, and
  pinned by a test that goes red when it is undone. An agent reads that
  section before its first website edit and never changes what a ruling fixes
  while doing other work. When a pinned test goes red after its edit, it
  repairs the edit, never the test. A ruling he has to repeat was applied too
  narrowly or left unpinned: the agent finds how it was undone and closes
  that route in the same work.
- Completing approved website changes includes regenerating and checking the
  canonical `../gh-pages` checkout with `./le site build` and `./le site check`.
  Local regeneration does not authorize a push or deployment.
- A request to rebuild the website for publication includes refreshing all
  recorded statistics before the full build. Run `./le site facts update`
  and `./le site history update`, refresh the other recorded statistics the
  site uses, then build and check the canonical artifact. Confirm that the
  generated pages and their JSON inputs use the refreshed data, including
  Project activity through the latest committed work. A successful page build
  alone does not complete this request. Review and testing builds reuse the
  recorded statistics unless Thomas asks to refresh them. Determine the
  purpose from his request and the session context; he does not need to name
  the recorder commands. This preparation does not itself authorize a push
  or deployment.
- The website is always generated into the canonical `../gh-pages` checkout,
  while Thomas reviews as well as at completion. He serves that checkout
  through a local web interface and reviews the pages there. An agent never
  renders a review copy into a private or scratch directory, and never starts
  a server of its own to show one.
- Website work is tested once, at the closure of the work, after Thomas has
  said he is happy with the pages. While he is still reviewing, an agent
  changes the page, renders that page alone from the working tree and shows
  it. It neither commits nor runs a test: the output is most often not right
  the first time, and a test run during review checks a moving target. The
  commit, the tests and their fixtures come when the pages are approved.
- When the website talks about how Ze is developed with AI, it presents AI as
  a means to quality: the project built a framework that constrains and guides
  how agents develop (specs agreed first, tests first, independent review,
  commit gates). It also says plainly that the process is iterative, as human
  development is: a first version is often not fully correct until every
  check exists (for an RFC, every requirement's test), which takes time and
  resources, so published code and the website itself can be imperfect, and
  readers are invited to report what is wrong. It says why the website has
  more mistakes than the code: whilst an RFC defines behaviour clearly and
  strictly enough for tests to check it, the same can not be said of the
  website's free text. It also says that most of the site's information is
  generated from the code, so the data reflects the software's real state.
- Building the website renders data; it never runs a check. A proof over
  the code (the comparison of the built `ze`'s plugin registry with the
  declared list, a coverage or conformance gate) is a check like a unit test: it runs in CI and at
  release, never as a step of `./le site build`. A fact the site publishes
  from the code is read from its declaration or from a committed data file,
  so a page edit publishes in minutes.
- What the website shows about RFC conformance (the gate's verdict, its open
  issues and the figures the RFC pages need) comes from a recorded data file,
  and the pages are generated from that file. The site build does not run
  the RFC check to render a page. It regenerates the file only when the file
  is more than a week old; a command regenerates it on demand, so Thomas
  can update it when he needs to. Preparing the site for publication,
  including producing the weekly update, always regenerates the file,
  whatever its age, and regenerates the site from it:
  by timing the file can be slightly under a week old then, and the update
  must not publish RFC data that old. The week is measured from the time the
  file was saved. The file lives in a git-ignored folder of the website, so
  it is never pushed.
- CI does not generate the website. The site is always generated on request.
- Website screenshots use WebP, alongside lightweight terminal-event playback
  for application demonstrations. Encoded video assets are not published on the
  website or stored in the repository, and CI/CD does not generate videos.
- The README shows a short animated SVG derived from the website's terminal
  recording, so the demonstration plays inside GitHub. Website regeneration
  refreshes that SVG from the same recording. The README does not need the
  complete continuous demonstration or an encoded-video carrier.
- Every website value that changes over time (counts, percentages, release
  versions, download artifacts and their sizes) lives in exactly one JSON file,
  and pages render it with JavaScript. Site-wide values live in
  `data/site-facts.json`; RFC completion data lives in one file per RFC and one
  per profile under `data/rfc/`. Updating that file updates every page that
  shows the value; a page keeps only the last build's value as fallback text
  inside the marked element, never a copy of its own.

## Protocol and RFC scope

Read before: changing or testing any protocol (BGP, IS-IS, OSPF, TFTP, YANG,
DNS, IPsec, L2TP, etc.), `rfc/short/` summary, RFC test, discrimination record,
or release-profile RFC list.

- A question the RFC text answers is not asked. Where the RFC is silent or
  unclear, the goal is interoperability: Ze must work with BIRD and FRR, so read
  their source and do what lets Ze interoperate with both. freeRtr's source
  may always be read as a further reference. Before a protocol
  design is approved, read FRR's and BIRD's source and quote the deciding
  function. Before an interop test expects a peer to accept, print or send
  something, read that peer's source for it.
- An RFC sentence quoted in a comment stays byte-exact, its full stop inside the
  quote included, and is never edited to satisfy a linter.
- An RFC defines the required baseline behavior, not a ceiling on explicit
  operator control. RFC compliance is assessed on the default configuration
  path. Explicit configuration options that provide more specific behavior or
  extensions remain supported; their existence does not make the default path
  fail compliance. An operator's ability to configure custom policy, raw
  octets, or explicit overrides in a way that violates an RFC is not a failure
  of Ze and never turns a compliant default path into a gap or partial
  requirement. RFC work does not remove existing features to improve a
  conformance figure. Specs name the default configuration being assessed, the
  options to preserve, and tests for both. Claims and evidence identify the
  configuration they cover; an explicitly selected extension is not presented
  as RFC behavior when it differs from the RFC. BGP router-ID overrides for
  peers and groups, and router-ID changes on reload, are features to preserve.
- A requirement of an obsoleted RFC whose `{superseded}` marker points to a
  proven requirement in its successor (`restated`), or that an implemented
  successor dropped (`dropped`), counts as validated and complete rather than
  as an open gap, including when Ze implements the newer protocol version in
  place of the obsoleted one (such as RFC 8210 RTR v1 superseding RFC 6810
  RTR v0). When its successor row is excluded as not applicable or declined
  with checked evidence, it leaves Ze's owed count the same way.
- A requirement that binds only a human operator's configuration choices,
  operational procedures, or network topology (which software cannot observe
  or verify), or that binds IANA registries, IETF allocation processes, or the
  author of a future RFC or specification, does not bind Ze and is excluded as
  `{not-applicable}` rather than recorded as a `{gap}`.
- When an RFC section heading or scoping sentence scopes a section to a role
  Ze does not fill, rows under that section may be excluded as
  `{not-applicable}` by naming that scoping heading or sentence and the bound
  role, even when an individual obligation sentence uses passive voice, a
  comma, or an adjectival keyword.
- BGP's RFC state machine describes expected behaviour; it does not prescribe
  the implementation's structure. Implement and assess the required externally
  visible behaviour using RFC 4271 together with applicable updates and errata,
  rather than copying its original state machine literally. The original
  description is not a complete model of modern BGP. Absence of a separate FSM
  object or a one-to-one correspondence with its states, events, timers and
  transitions is not evidence of a defect or conformance gap. A gap must name
  unmet behaviour required by the applicable specification. This leaves all
  applicable protocol requirements in force.
- A release profile owes exactly the RFCs declared for it, the ones its role
  implements, the way an established implementation of that role (BIRD for a
  route server) does. A normative citation is never an obligation: the RFC
  dependency graph only suggests candidates. An RFC for an optional transport
  or feature is listed for the profile as optional, and it counts toward the
  profile's percentage only once Thomas brings it into scope. Every RFC a
  profile lists, required or optional, has a valid summary. A technology
  profile (OSPF, DNS, ...) lists an RFC as optional the same way, when Thomas
  calls it not a priority for the first release. Its rows stay honest gaps and
  count only in that profile's optional figure, never as declined. A release
  profile needs every mandatory requirement (MUST, MUST NOT, SHALL, SHALL NOT,
  REQUIRED) of every declared RFC covered before release, except a requirement
  the operating system blocks because Linux cannot provide its primitive, and
  work moves from root RFCs to leaves, except where a leaf updates a root's
  requirements. A requirement counts as covered only when a recorded break
  proved its test goes red. Optional requirements (SHOULD, SHOULD NOT,
  RECOMMENDED, NOT RECOMMENDED, MAY, OPTIONAL) are reported as a second figure
  and improved after mandatory coverage reaches 100%. RFCs no release profile
  declares wait until after the first release.
- RFC validation scope does not grow: no summary is written or enrolled for an
  RFC that no profile lists, because too many enrolled RFCs are still not fully
  implemented. Adding an RFC text under `rfc/full/` is fine.
- ISO/IEC 10589 (IS-IS) is not counted in any conformance figure. Ze holds no
  authoritative text for it, only a reconstruction the project wrote, and a
  requirement list drawn from that reconstruction proves nothing about the
  standard.
- Internet-Drafts (`draft-*`) are not RFCs and are not RFC requirements, so
  they do not count in RFC conformance figures, RFC targets, or
  release-profile obligations. Ze may still implement, summarize, and test a
  draft feature, and track draft conformance separately, without mixing
  `draft-*` documents into RFC conformance counts until they are published as
  RFCs.
- Ze's TFTP server accepting uploads (write requests) is in scope, so the
  RFC 1350, 2347, 2348, 2349 and 7440 requirements that bind a receiver are
  owed work, never declined.
- Full YANG (RFC 7950) coverage runs after the first release. Before it, YANG
  work is limited to what blocks something Ze needs.
- RFC 9463 (encrypted DNS resolver discovery, DNR) may be added later and is
  not in the first release. Work done before it shapes learned DNS resolvers so
  DNR can extend them without undoing anything.
- DNS zone transfer (AXFR, IXFR) is not required. The resolver Ze runs for
  customers is not an authoritative server, and the plugins that answer
  authoritatively (GeoDNS, AS112) answer queries only: none of them serves a
  zone to a secondary. A zone-transfer requirement therefore binds the primary
  or secondary server role no part of Ze fills, and its annotation names that
  role.
- Ze's DNS resolver serves a home connection. A DNS requirement that binds only
  a role neither that resolver nor the answering plugins fill is out of scope,
  and its annotation names that role.
- Ze may build its own DNS library on top of miekg/dns that covers more of a
  DNS RFC than the resolver and the answering plugins use today. That work is
  in scope before anything calls it; a requirement the library meets counts as
  met only once a shipped path uses it.
- Every MUST of an implemented RFC is a requirement unless Thomas has named its
  role or feature as unwanted. An annotation that proves less (declined, not
  applicable) is written only for a role or feature he named, a lower-layer
  requirement still owes a test of what Ze installs, and no annotation class is
  put to him row by row.
- An RFC published before RFC 2119 defined the BCP 14 keywords states its
  obligations in lowercase. In such an RFC, "must" is read as MUST and "must
  not" as MUST NOT, everywhere a keyword is looked for: requirement rows, their
  levels, and the quotes an annotation carries.
- A feature an earlier judge or agent declined, with no decision of Thomas's
  behind it, is a gap, not a declined feature.
- A feature leaves the count as declined only when Thomas said Ze will never
  implement it. A feature he left out of the current work, or out of the first
  release, is wanted later: its rows stay gaps owed after the release, and no
  page calls them out of scope.
- A public page says what Ze does not build and what is not built today, and
  says nothing about whether or when it will be: no page publishes an intent
  for an unmet requirement (never, not yet, undecided), because that answer
  changes too easily. A requirement that has tests but no recorded proof is
  published apart from one that is not built, never under one heading.
- A declined feature leaves the count only on the RFC's own words. Either a
  sentence makes the feature optional, or a sentence makes the obligation
  conditional on using the feature (as in "If the client uses a delegated
  prefix ... MUST"). The second case also needs evidence that Ze never meets
  the condition. A section heading that names the declined feature scopes every
  obligation under it the same way (RFC 8907 "CHAP", RFC 9915 "DUID-LLT"),
  with the same evidence that Ze never uses the feature.
- A requirement the operating system meets on Ze's behalf still owes a test
  that checks the OS behaviour on the state Ze installs. Until that test
  exists, the requirement is recorded as a gap in testing, never as a gap in
  correctness. Once that test proves it, the requirement counts as done in
  every published figure, the same as one Ze's own code meets, unless a rule
  here names a reason to keep it out.
- A requirement the operating system blocks, because Linux cannot provide the
  primitive it needs and a verified limit entry shows it, is reported in a
  section of its own as a limit of the kernel. It is not missing coverage in
  Ze's code, so it never counts against Ze's mandatory coverage or holds a
  release.
- A requirement the Linux kernel produces from the packet alone, where no state
  Ze installs decides the outcome, takes the `kernel-producer` escape and is
  published as escaped, never as proven. Its record notes that a patched-kernel
  proof could prove it later, and that route is planned in a kernel spec of its
  own.
- The tests of an RFC MUST prove that the code obeys it and that the test would
  notice if it stopped. The positive test proves Ze does what the clause
  requires, in the case the clause describes, observed where the behaviour
  shows: the bytes on the wire, the state installed, the message refused. The
  negative test proves the behaviour is tied to the clause's condition and is
  not unconditional: it takes the case just outside the clause, where the
  correct answer is the opposite one. Code that refuses everything, or sends
  something always, passes a positive test alone.
- What the negative is depends on which side of the wire the clause binds. A
  clause that obliges Ze to generate something is proven positively on what Ze
  sends, and its negative is on reception: how Ze handles a peer's message that
  lacks it. That handling is what the RFC prescribes (ignore, refuse, tear
  down), or, where the protocol cannot run without it, the right way to deal
  with its absence. A clause that obliges Ze to accept something is proven
  positively by acceptance, and its negative is how Ze handles it absent or
  malformed. For a MUST NOT, the test that matters is the violating input being
  refused: a positive alone proves nothing.
- A tag claims no more than its test body asserts. A claim wider than the
  assertion publishes an unproven MUST as proven.
- An RFC proof record shows that its tagged test fails when the behaviour it
  checks is broken. That proof belongs to the test: it goes stale only when the
  tagged test or its claim changes. A change to the code the test checks, the
  function the record broke included, leaves it standing, because the unchanged
  test still catches what it caught. A commit that changes a tagged test or its
  claim carries the record `./le rfc record` re-took, or removes the tag with
  the judge's approval, and the commit gate refuses it otherwise. This is a
  refusal of what the commit itself does, never a red, so it holds before
  release.

## Third-party dependencies and licensing

Read before: adding, updating, patching, or importing a third-party Go module
or external code, editing `go.mod`, `vendor/`, or `third_party/`, or touching
Cloudflare `circl`.

- A change Ze needs in a third-party Go module is kept here as a diff against
  the upstream release, applied at build time. A fork exists only to open an
  upstream pull request and is never a dependency.
- A new third-party Go module lands in a commit of its own (`go.mod`, `go.sum`
  and its `vendor/` tree, nothing else), before the commit that first uses it.
- A change Ze makes to third-party code it carries (a patch to a vendored
  module, a patch to source copied in from another project) is released under
  that code's own licence, never under Ze's. A patch to Apache-2.0 code is
  Apache-2.0, so upstream can take it. Code Ze writes that only uses the
  third-party code (a client, a backend, a plugin) stays under Ze's licence.
- A third-party module that is experimental and implements cryptography (the
  Cloudflare `circl` module) is approved for the use Thomas approved, and
  nothing more. Its presence in `go.mod` or `vendor/` is never permission to
  use another of its packages or features, or to import it from another Ze
  package: each new use is put to Thomas with what it is for, before the code
  is written. Every other module already in `go.mod`, `vishvananda/netlink`
  included, may be used in full.

## CLI, config, and daemon surfaces

Read before: changing a daemon command surface (CLI, MCP, REST, gRPC, SSH,
web), `ze doctor`, peer reload, filter-list insertion, implied config, or CLI
pipe operators.

- Every operator command runs through the daemon on every command surface (MCP,
  REST, gRPC, SSH, web) with the same output as the local `ze` command,
  including the ones that answer without a daemon (`ze doctor`, `show config`,
  `show yang` and the rest), so a remote operator or AI can debug the box. The
  local route stays for when the daemon is unreachable. A command whose result
  lives on the caller's side gets a daemon form designed for it, never an
  exclusion.
- Ze's MCP server speaks only the current MCP revision. A client that still
  speaks an earlier revision cannot connect until it updates; that is recorded
  as the client's state, never treated as a Ze defect, and Ze adds no
  compatibility path for the earlier revision.
- `ze doctor` reports every problem an operator can hit, so it can be used to
  find any problem on the box: each one with its cause and the command that
  fixes it. A problem class found in code, a lab or a test that doctor does not
  report is a gap owed a doctor check, never left to the logs.
- An operator adds a peer, with any of its settings, to the running
  configuration without restarting Ze, as every other routing implementation
  allows. A reload that refuses a new peer because of what it configures is a
  defect to fix, never a documented limit and never a question.
- Every route-processing step is a filter in the peer's filter list. A
  procedure a feature's config implies (a role's RFC 9234 checks, the
  transit-leak check a role brings) is auto-inserted into that list by the
  feature's own plugin when the config needs it, visible like any member; a
  required one refuses an operator edit that removes, moves or deactivates it.
  The plugin declares the insertion; the engine and config code never name a
  feature, a plugin or a config path to make it happen.
- Nothing Ze does on an operator's behalf is hidden. Where the configuration
  leaves the operator a choice, the operator writes it. What a plugin
  registers is not such a choice: a configured plugin receives and sends what
  it registered, shown as implied, and the operator writes an allow-list only
  to narrow it. Where it leaves none
  (the plugin that serves a block the operator wrote, the flow a rule the
  operator wrote needs), Ze does it without asking or correcting at commit, and
  one display option on the configuration shows every such implied item beside
  what the operator wrote, marked as implied, with what implies it. A design
  that implies something it cannot show that way is wrong. A capability that is
  merely on by default implies nothing. Where the operator wrote two things
  that disagree, a feature and a grant narrower than the feature needs, the
  commit is refused with the line that completes it: Ze never widens what was
  written, and never loads it to do nothing.
- A command path names the subject and stops; scope, presentation and format
  are pipe operations after it. A command may declare optional pipe operations
  of its own, such as `| count`, and the engine handles a declared operation
  for every caller. The command line is never rewritten to carry one as an
  argument. A common operation such as `| count` is generic and applies to
  every command's answer. A command overrides its behaviour by declaring the
  same name with its own description, keeping the generic's meaning and answer
  shape; an operation that means something else takes a name of its own.
