# Authoring a Terminal Demo

This page says what a terminal demo or video shows and how it sounds. The
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
loaded. Then the demo runs `show | compare` and `commit`.

### 2. Show the lab a user would build

> "the video should be what a user who is going to use the feature would have
> built in a lab to test it and see how it works and how it will be able to
> debug problems later"

Each feature recording has five beats, in this order:

| Beat | What the viewer sees |
|------|----------------------|
| 1. The lab | The topology, briefly: which peers and daemons exist, and why |
| 2. The load | The feature's configuration loaded section by section, each section explained, then `show \| compare` and `commit` |
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
  of the site. The README embeds it when GitHub can render it, and otherwise
  shows a thumbnail that links to the site player.
- When a feature does not enable on a live commit, that is a Ze defect to fix.
  Never work around it in the tape.
