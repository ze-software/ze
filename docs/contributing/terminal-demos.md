# Authoring a Terminal Demo

This page says what a terminal demo shows and how it sounds. The
owner set these decisions on 2026-10-09 so that every later recording has the
same tone. Each decision is quoted verbatim, then followed by how a demo applies
it. How a demo is built, rendered and published is in `gh-pages.md`. The public
gallery is `../guide/terminal-demonstrations.md`.

## The owner's decisions

### 1. Load the configuration in sections, do not type it

> "present the configuration, slow typing is slow, you can use load merge like
> feature and show each section as it is loaded, explaining it"

The demo does not type configuration one character at a time. It loads the
configuration into the running router one section at a time with a
`load merge`-style verb. Each section is shown on screen and explained as it is
loaded. Then the demo runs `show | compare` and `commit now`.

The sections load inside the SSH editor, which holds the terminal, so the
explanation takes two kinds of card. The owner chose both on 2026-10-11:

> "both"

| Card | When it shows | How a tape writes it |
|------|---------------|----------------------|
| The overview | Once, before the editor opens. It names every section the recording will load, in order | `Type "ze-demo card topic-<topic> overview"` in the outer shell, as every other card is printed |
| The section card | Before each section loads, over the editor's screen. It says what the section does | `Card topic-<topic> <section>` on the line above the section's `load ... merge` |

The recorder writes a section card into the recording only. The editor never
sees it, so the candidate and the session stay as they were. The card stays on
screen for two seconds plus 200 milliseconds a word, at most ten seconds, then the recorder repaints
the editor's screen and the section loads. The card text lives in
`internal/le/site/terminaldemo/cards.json` under `topic-<topic>:<section>`, and
the card's first line is its title.

Every screen stays up long enough to read. The recorder holds a result a tape
waited for (a loaded section, a commit, a show command's output) on screen for
at least four seconds before the next action may change it, and a card printed
in the outer shell with `ze-demo card` for the same two seconds plus 200
milliseconds a word. A `Sleep` in the tape counts toward that time, so a tape
sleeps only where a screen needs longer. Nothing is held while the recording is
hidden.
<!-- source: internal/le/site/terminaldemo/tapehold.go -- resultHold, outerCardHold, holdResult, awaitReading -->

A typed command is on screen whole before Enter runs it. The ze editor paints
on its own frame tick, so a last letter typed less than a frame before Enter
was never drawn, and the transcript check refused the recording because no
viewer saw the command. After the last key of a `Type`, the recorder waits
until the program's output goes quiet, then sends the next action.
<!-- source: internal/le/site/terminaldemo/pty.go -- tapeSession.typeText, tapeSession.settle -->

Two checks hold a tape to this. The recorder refuses a tape that loads a
section with no `Card` since the previous section load, and a `Card` between
`Hide` and `Show`, where nobody would see it. A unit test refuses a topic whose
overview card does not carry the title of every section card its fragments
show.
<!-- source: internal/le/site/terminaldemo/tapecard.go -- requireSectionCards, cardHold, restoreScreen -->
<!-- source: internal/le/site/terminaldemo/tapecard_test.go -- TestTopicOverviewCardNamesEverySection -->

### 2. Show the lab a user would build

> "the video should be what a user who is going to use the feature would have
> built in a lab to test it and see how it works and how it will be able to
> debug problems later"

Each feature recording has five beats, in this order:

| Beat | What the viewer sees |
|------|----------------------|
| 1. The lab | The topology, briefly: which peers and daemons exist, and why |
| 2. The load | The feature's configuration loaded section by section, each section explained, then `show \| compare` and `commit now` |
| 3. The proof | The operational show commands an operator uses to see that the feature works |
| 4. The failure | At least one realistic failure or misconfiguration, scripted |
| 5. The diagnosis | Ze's own tools (show commands, counters, logs, monitor, debug) reveal the cause, then the fix |

When an operator would need a diagnosis tool that Ze does not have, record that
as a finding for the owner. Never invent the tool in the tape.

### 3. A perfect lab for two audiences

> "the perfect lab showing everything and where everything went fine (not how
> lab usual run) but it must be the impression given. Teach them the feature by
> demo if they knew about it from another vendor and if they did not know show
> them enough to understand what they saw"

The run on screen is flawless: no retry, no waiting, no stray output. A failure
on screen is a deliberate teaching step (break, diagnose, fix), never an
accident. Readiness waits happen hidden, before the screen shows.

The cards serve two audiences:

| Audience | What the cards give them |
|----------|--------------------------|
| An operator who knows the feature from another vendor | The equivalent concept and command in Junos, IOS, EOS, FRR or BIRD terms. Verify each one against that vendor's documentation. Never guess |
| A newcomer | A short card before each step that explains the concept just enough to read the screen: what a VRP is, what a BFD session does, what Full means in OSPF |

Cards stay short. The terminal is the subject.

### 4. It is a demo

> "it is a demo"

A demo is a staged, scripted presentation that looks like a smooth lab session.
It is not a test lab and not a test suite. Staging behind the scenes is
acceptable: a pre-built lab, hidden setup, pre-seeded peers. What the screen
shows MUST be real Ze behavior. Validators exist only so that a recording cannot
ship with wrong output on screen.

The browser configuration demo has one authored journey,
`test/web/recorded-config-save.wb`. Both its validator and recorder parse it with
the web test parser and execute it with the browser runner: submit the real login
form, edit the hostname, click Save, review the generated diff, confirm the
commit, re-fetch the clean pending diff and navigate to read back the value.
There is no separate Basic-auth HTTP validator. The authored millisecond waits
retain reading pace; assertions, not those waits, establish each transition.
The case deadline also bounds the real init and import children and daemon
readiness. Each journey has a private store; inherited case/separator spellings
of the config-root override are removed before assigning that private root.
It retains the daemon's sole-reaper owner, not a numeric PID file. Cleanup sends
the daemon TERM, escalates after the runner's existing ten-second grace, and
joins it before removing the store.
Cleanup failures remain errors alongside any earlier browser failure.
After a started daemon fails readiness, its final log accompanies the error
after the owned process is joined and before its private store is removed.
Setup and daemon children use that workspace as their working directory, so
relative audit-log output stays private instead of rewriting checkout files.
<!-- source: internal/le/site/terminaldemo/web_config_startup.go -- webConfigWorkspace.start, webConfigWorkspace.run, webConfigWorkspace.waitReady -->
<!-- source: internal/test/runner/server_process.go -- ServerProcess.Stop, ServerProcess.Wait -->

Only the recorder adds a capture, after fonts, finite animations, render frames
and finite requests settle within the case deadline. Capture requires installed
request observation with finite counters; missing observation cannot mean idle.
Its presentation wrapper keeps the introductory card and captures WebP through
the owned browser's CDP endpoint; it carries no second configuration workflow.
<!-- source: test/web/recorded-config-save.wb -- shared authored journey -->
<!-- source: internal/le/site/terminaldemo/web_config.go -- runWebConfigJourney, captureWebConfig -->
<!-- source: internal/component/web/testing/capture.go -- settleCapture, captureReadyExpr -->

On a Linux development host, run the actual three-capture poster proof with
`./le job run label web-config-poster command env CGO_ENABLED=0 go test -tags integration -run '^TestWebConfigPosterUsesAuthoredInputs$' ./internal/le/site/terminaldemo`.
Build the renderer image first with `./le site terminal-demo build image`.
The test builds its own `ze` from the tree under test, with the recipe of
`./le site terminal-demo build binaries`, into its private demo root, and the
child refuses to run when another `ze` comes first on its PATH: the shared
`tmp/terminal-demos/bin` copy is rebuilt only by hand, so driving it would judge
code older than the tree.
The test uses the existing renderer preflight, lock and owned container runner;
it does not start the network-configuring daemon on the host. Its compiled test
binary runs inside the pinned image with a private network and artifact mount.
The proof gives the existing entrypoint a private demo root inside that mount,
including its home, runtime and ownership-return directories. Checkout and
shared runtime inputs are read-only. It never rewrites shared SSH configuration
or walks shared scratch to change its ownership.
The QEMU integration runner excludes this package because that guest lacks the
browser dependencies and cannot substitute for this Docker execution route.
<!-- source: internal/le/site/terminaldemo/web_config_poster_integration_test.go -- TestWebConfigPosterUsesAuthoredInputs, runWebConfigPosterInRenderer, buildPosterZe -->
<!-- source: internal/le/site/terminaldemo/web_config_poster_binary_test.go -- posterZeIsOwned -->
<!-- source: internal/le/test/qemu/alltests.go -- excludedIntegrationPackages -->

### 5. The tone of a sales engineer

> "like you would join a VC to be presented a software by a technico-commercial"

Each recording feels like a sales engineer's live video-call demo. The pacing is
confident. The cards talk as the sales engineer would talk: "here is the lab, we
load the RPKI section, this line points at the cache, commit, and you can
see...". They show what matters to the buyer, skip nothing important, stay on
nothing trivial, and it works the first time.

## Recordings

The owner set these on 2026-10-09 with the decisions above.

- Each feature has its own recording. It goes on that feature's page and in the
  gallery.
- A separate super-recording is one continuous session that builds the
  configuration up feature by feature. It is recorded on its own, never
  stitched from the feature recordings and never cut into them. It is the hero
  of the site. Under the owner's 2026-10-27 presentation policy, the README uses
  a WebP thumbnail linked to the actual site player. Terminal events remain
  `.cast` files. Browser presentation uses WebP screenshots, not encoded video.
- When a feature does not enable on a live commit, that is a Ze defect to fix.
  Never work around it in the tape.

## Authored transcript checks

Each recording's `transcript.txt` is also a command oracle. Explanatory prose
does not count as an assertion: include nonempty command lines starting at the
left margin with `$ ` for the shell, `ze# ` for the configuration editor, or
`ze> ` for operational mode. The renderer requires those prompt-and-command
pairs in order in the captured terminal's painted rows. A prose-only transcript
is refused after capture because it proves nothing.

Quote the visible section loads, comparison and commit, operational proof,
failure diagnosis and recovery. Do not quote hidden setup or readiness commands,
or invent output. For a command that wraps, quote an unambiguous prefix from its
first painted row. The continuous recording needs its own ordered assertions
across the shared chapters; referring to the topic transcripts in prose does
not include their checks.
<!-- source: internal/le/site/terminaldemo/render.go -- transcriptCommands, checkTranscript, castVisibleText -->
