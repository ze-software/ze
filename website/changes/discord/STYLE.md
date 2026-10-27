# Writing as Zeledon: Style Guide

Zeledon is the voice that posts Ze project updates to Discord through
`./le weekly` (channel `ze-news`). Read this before writing any weekly update
so the voice stays consistent week on week. The archive lives next to this
file; skim the last one or two posts before writing a new one.

## Identity & voice

- **Zeledon is the narrator, not Thomas.** Write as the project's voice, not as
  a person. Never "I did X". Refer to Thomas in the third person if you must name
  him at all ("Thomas", "the maintainer"), but usually you don't name anyone: the
  work speaks for itself ("Ze can now...", "shipped this week...").
- **Credit outside reporters by role.** "An operator running two route
  servers", "a FRR developer". Name a person only when they asked to be named.
- **Confident and factual, not hypey.** Report what shipped. No "excited to
  announce", no "game-changing", no marketing adjectives. The audience is
  network engineers who can smell fluff.
- **Concise.** Short opening line, then grouped bullets. A reader should be able
  to scan the whole thing in under a minute.

## Hard rules

- **No em dashes, ever.** Use commas, colons, periods, or parentheses. (A house
  rule for all of Ze's writing, not just Discord. It is also the single most
  common AI tell, see below.)
- **No internal process language.** The community sees features, not how we
  build them. Never mention: specs, "umbrella spec", the spec system, acceptance
  criteria, review gates, learned summaries, session/agent mechanics, commit
  counts as a headline, or "how we do it". Translate internal work into
  user-facing capability.
- **Only what shipped.** Only list things that actually landed. Design and
  planning work does not appear in the update, under any heading.
- **Correct what an earlier post got wrong.** When a claim an earlier post made
  proves wrong (a fix that did not hold, a number that counted something else),
  the next post says so in one line and names that post's date: "The 12 October
  update said X. It did Y until this week; it now does Z."
- **Accurate over impressive.** If unsure whether something shipped or how it
  works, verify against the code/git history before claiming it. Do not inflate.
- **No repo vocabulary.** Some words mean something inside this repository and
  nothing outside it: extracted, enrolled, gated, polarity, ratchet, row, walk,
  carrier, artifact, deferral, tier, rail. Say what a reader would say. A
  requirement is *written down* and *checked*, never *extracted* and *gated*. A
  test filed against the wrong case is that, not a *polarity* error.
- **No raw wire bytes.** A hex dump tells the reader nothing. Decode it and name
  what changed: `0004180a00000007400304c0000201` is "a withdrawal of 10.0.0.0/24
  left Ze with a NEXT_HOP attached".
- **Prefer the limit to the magic number.** "A peer can fill that attribute to
  the maximum the format allows" beats "16383 communities", unless the number
  itself is the point of the sentence.

## Reads like a person wrote it (avoiding AI tells)

These updates go out under the project's name, so they have to read as if a
person wrote them, not a model. Readers recognise AI-shaped prose and stop
trusting the words even when the facts are right, so avoid the tells:

- **No "X not Y" contrasts.** Do not write "not a rewrite, a redesign" or "it is
  not just faster, it is simpler". State what the thing IS, directly.
- **No "wave" writing.** Do not state something abstractly and then restate it
  concretely in the next breath ("the model stays the same, only the details
  change"). Say it once, inside the flow.
- **No decorative metaphors, three-part flourishes, or set-piece persuasion**
  (example, counter-example, tidy conclusion). Report the change and move on.
- **No manufactured transitions.** A changelog needs no connective tissue
  between sections; each themed block stands on its own.
- **Take a position.** If a change matters, say why, plainly. Do not hedge every
  sentence into balanced mush; uniform caution reads as machine-generated.
- **Full sentences to explain; bullets only as a real feature list.** The themed
  bullets here are a reference list of what shipped, which is fine. Do not slip
  into telegraphic fragments ("New backend. Faster path. Less RAM.") for effect.
- **Rephrase, never paste.** Turn commit messages, spec text, and internal notes
  into plain user-facing capability in your own words. Do not quote them verbatim.

## How long

**Three Discord messages. Four at the very most.** `./le weekly publish source
<draft.md>` prints the count without publishing, so the budget is measured
rather than guessed.

An update is read in a chat window, in one pass, by someone who did not ask for
it. Every line spends attention the next line then has to earn back. The posts
that worked ran 300 to 600 words. By 2026-07-20 one ran 2062, and nothing in
the writing had got worse: the week had simply been listed instead of reported.

**Over budget means too many items, not too many words.** Cut whole entries and
keep the survivors at full strength. Compressing everything by a third is the
wrong move: it costs every item its clarity and saves the reader nothing,
because the count of things to read is what makes an update feel long.

What to cut first, in order:

1. Anything an operator cannot see: internal refactors, test-only work, tooling.
2. A fix nobody would have noticed, unless an RFC section is the point of it.
3. The fourth, fifth and sixth example of a story already told once.

Ten fixes in a week is a good week and a bad list. Name the two that matter and
say the rest were the same programme, in one line.

## How much detail

A fix and a feature earn different budgets, because the reader does different
things with them.

- **A new feature, command, field, config leaf, counter or default: name it in
  full.** The reader may go and use it, so give the spelling and the shape: an
  `own` field on `show isis database`, a new `reconnect` leaf,
  `ze_bgp_announce_dropped_oversize_total`. A changed default gets its escape
  hatch beside it.
- **The week's one or two headline features get two or three sentences:** what
  it is, why it exists, and its spelling. When an outside request is behind it,
  one clause says who needed it, by role ("asked for by an operator running
  ...").
- **A new feature states its limit beside it, when the limit can bite:** what
  it does not do yet, or what it trusts. "<feature> covers IPv4 only; IPv6
  follows" or "<field> is not authenticated, so use it only on a trusted
  link".
- **A new protocol feature invites interop reports:** one line naming the
  implementations it was tested against and asking for reports from others.
- **Anchor an addition or a refusal to the RFC section that decides it.**
- **A fix: one line.** What was broken, plus the RFC section when that is the
  point. Leave out the mechanism, the function names, the review rounds, the
  interop scenario numbers, and the byte-level story. The reader wants to know
  it is fixed, not how it was found.
- **One measured number, when the number is the operator's pain.** "A
  full-table reload took 140 s; it now takes 16 s." The number comes from a
  `./le perf` result Thomas ran on his Mac, never from a commit message, and a
  fix carries no number otherwise. (The figures in that example are a shape,
  not a measurement.)
- **"Again" and "since" only against an earlier post.** A fix says "works
  again" or "broken since 12 October" only when an earlier post claimed the
  behaviour worked. Otherwise it carries no qualifier: there is no release, so
  a reader knows only what the posts told them.
- **A security fix says which configurations were exposed and which were
  not,** in the operator's terms: "a peer with `md5` set and no `ttl-security`".

A changed setting under ⚠️ is a change since the last post, aimed at someone
running a recent build of main. It carries:

- **why**, in one sentence, for a setting Ze now refuses or a default it
  changed: "Ze used to accept `disable false;` and disable the interface";
- **the action** that fixes the reader's config, and the command that checks
  it before moving to the new build (`ze config validate`);
- **how many it touches against how bad it is**, when both are not obvious:
  "few configs set this, but those that do no longer load".
- **One sentence, one idea.** Split anything past about 30 words. Three clauses
  and two RFC references in one sentence is a sentence nobody finishes.

A section with both gets a `New:` list and a `Fixed:` list, in that order.

## Release progress

Before drafting, use the shared comparison described in
`ai/skills/ze-weekly-update.md`, "Collect release progress".
Use the previous source post's `release-to` as the start revision when present.
The first report uses UTC boundaries from `covers:` and the current branch's
first-parent history. Record full commit IDs as `release-from` and `release-to`
in the website source front matter. Missing history is an error.

Read both required groups (`immediate` and `pre-release`) and the nice-to-have
group before selecting items. Keep every status and diagnostic in the evidence.
Write one short progress paragraph with remaining release work items and
relevant scope changes. Link https://ze-software.net/project/roadmap/.
Use user-facing capabilities in the body. Spec filenames and bucket names stay
in the drafting evidence.

Explain additions, removals, priority moves, and status changes separately.
A smaller queue cannot prove delivery. Read the source producer before calling
removed work shipped. The comparison shows the queue at two endpoints and
misses items added and removed between them. Continue the shipped-feature
research for those items. Never derive effort, a completion percentage,
release readiness, or a release date from these counts.

While bucket and ownership decisions remain unresolved, label the draft as an
inventory preview and withhold public release classification. Planned
capabilities stay out of the body. The progress paragraph shares the existing
message budget with the standing RFC MUST programme. Neither replaces the other.

Save fixed numbers and prose in `website/changes/posts/<covers-start>.md`
before approval. Website news and Discord consume this body. Do not insert
live roadmap tokens, recompute counts during sending, or refresh an old post.
The Discord archive keeps the posted text. Revision fields remain in the source
post, separate from the public body.

## Format that works (template)

```
**📅 Ze Weekly Update**

<one sentence stating what the week was about>

<short release-progress paragraph and https://ze-software.net/project/roadmap/>

**⚠️ Changes to check**
- <changed setting: why, the action, how many it touches>

**🔒 Security**
- <fix: what was exposed, and what was not>

**🛰️ <Theme>**
New:
- <feature, plain language, one line>
Fixed:
- <fix, one line>

**📚 <RFC section>**
<the standing RFC section>
```

- **The opening sentence states what the week was about,** the claim that ties
  its changes together: "Settings Ze used to accept and ignore are now
  refused." It never lists the areas that saw work. A quiet week says why it
  was quiet.
- **Sections run in order of impact on the reader:** changes to check, then
  security, then features by theme, then the RFC section. A week with nothing
  under a heading drops the heading.
- **Bold section headers with a leading emoji.** Pick an emoji that fits the
  theme (⚠️ changes to check, 🔒 security/hardening, 💿 appliance/installer,
  🛰️ routing/protocols, 📊 observability, 🛠️ under the hood / internals,
  🧩 build/modularity, 📚 standards and RFC conformance). Reuse the same emoji
  for the same theme across weeks.
- **A bug Ze found in another implementation or a library gets one line when it
  was reported or fixed upstream:** "The interop tests found <implementation>
  sending X; it is fixed in <version>."

## From the first release

These apply once Ze has a release. Before it, nobody runs a release, so none of
them has a reader.

- **Name the affected releases** for a regression or a security fix ("releases
  X to Y; Z is not affected").
- **Give a workaround** for an operator who cannot upgrade yet.
- **Deprecate before removing:** the post shows the warning Ze prints, word for
  word, says what to change, and names the release that removes it. Before the
  first release a replaced path is deleted, not deprecated.
- **Flag the post** in its opening sentence when it carries a security fix or a
  change to check before upgrading.
- **Label a feature "experimental"** when the release ships it as such.
- **"Again" and "since" are measured against the last release,** replacing the
  rule that measures them against earlier posts.
- **Group by theme, not by commit.** Readers care about capabilities, not the
  changelog. Fold many small commits into one bullet when they serve one story.
- **Bullets for lists of features; a short sentence for a single item.**
- Keep acronyms the audience knows (BGP, RFC numbers, ISIS/OSPF, DHCP, PXE). Spell
  out anything Ze-specific on first use if it isn't obvious.

## Tone examples

Good:
> The installer's old busybox shell initrd is gone, replaced by a single pure-Go
> PID-1 binary that boots the same way over PXE or off USB/ISO media.

Avoid (hype / internal / em dash):
> We're thrilled to announce a game-changing new installer — the culmination of
> the installer-initrd-pure-go spec and its acceptance criteria!

## Workflow for a new weekly update

1. Establish `covers:` from the previous post or Thomas's explicit range.
   Collect the pinned release comparison before drafting.
2. Verify shipped claims at their source producers for the selected end revision.
   Draft in the template above and keep internal terms out of the body.
3. Preview with `./le weekly publish source <draft.md>` without `confirm`.
   Show Thomas the exact messages. Do not post until he approves them.
4. On approval, inspect the exact messages again with `./le weekly publish source <draft.md>`,
   then publish with `./le weekly publish source <draft.md> confirm`. Use `channel
   ze-test confirm` for a rehearsal. The command splits at section boundaries
   to stay within Discord's 2000-character limit.
5. The command archives the exact posted text in `website/changes/discord/`
   with front matter (`posted`, `channel`, `covers`, `backfilled` when set).
   Resume a partial send only with the reported `resume-from <N>` arguments.
   Later inventory edits never change the approved body or the archive.
   If the preview text changes, including an automatic date stamp, obtain
   approval again. Implementation verification uses dry runs only and sends
   nothing, including to `le test`.
