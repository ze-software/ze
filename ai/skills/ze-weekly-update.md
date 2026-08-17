---
name: ze-weekly-update
description: Write and publish the Ze weekly update
---

# Weekly Update

Write the Zeledon weekly update, update the gh-pages site, and post the approved message to Discord `ze-news`.

Use this when Thomas asks for the weekly update, Zeledon update, Discord update, `ze-news` post, or weekly changelog.

See also: `/ze-status` for current work context, `/ze-doc-update` for broader docs sync, `/ze-commit` for preparing a commit after publishing.

## Hard gates

- Do not post to Discord until Thomas explicitly approves the exact text that will be posted.
- Do not publish unverified claims. Read the source, commits, docs, or generated pages before saying a feature shipped.
- Do not mention internal process in the public update: specs, acceptance criteria, review gates, agent sessions, learned summaries, commit-count bragging, or implementation bureaucracy.
- Write as Zeledon, not as Thomas. Use the project voice. If Thomas must be named, use third person.
- No em dashes. Use commas, periods, colons, or parentheses.
- The whole update fits in 3 Discord messages, 4 at the very most. Check with a dry run before showing it to Thomas. Over budget means too many items, so cut items (`scripts/zeledon/STYLE.md`, "How long").
- A fix gets one line. A new command, field, config leaf, counter or default keeps its full spelling (`scripts/zeledon/STYLE.md`, "How much detail").
- No repo vocabulary and no raw wire bytes in the post (`scripts/zeledon/STYLE.md`, "Hard rules").
- Do not hand-edit generated site pages. Edit the source data or Markdown, then run the generator.

## Required references

Read these before drafting:

1. `scripts/zeledon/STYLE.md`.
2. The latest one or two files in `scripts/zeledon/weekly/`. <!-- doc-links: ignore (archive dir created at runtime by post_weekly.py; absent on a clean tree) -->
3. `website/AI.md`, especially `Weekly Update Checklist`.
4. `website/data/topics.json` for allowed update tags.
5. The latest `website/changes/posts/*.md` post, to keep format and coverage continuity.

Website sources are in `website/`; the publishable artifact is generated into `../gh-pages`.

## Phase 1: Establish the week

1. Find the newest archived Discord post in `scripts/zeledon/weekly/` and the newest website post in `website/changes/posts/`. <!-- doc-links: ignore (runtime archive dir created by post_weekly.py) -->
2. Determine the new `covers:` range from the previous post's end date unless Thomas gives a different range.
3. Gather what shipped during the range:
   - inspect `git log` for the range,
   - read the touched source, docs, specs, or tests needed to understand user-visible behavior,
   - include only behavior that actually landed,
   - put design or planning work only under `Coming up`, phrased as work started, not shipped.
4. Group the week by user-facing theme, not by commit. Fold small commits into one capability when they serve one story.
5. **A commit message describes the moment it was written, not HEAD.** Before calling anything fixed, read the producer at HEAD. A number quoted in a commit body is usually the PRE-fix measurement, and a spec row can still read `NOT MET` after the fix landed. Say what is true now.

### The RFC MUST programme is a standing item

Ze is being checked against every RFC it implements, one MUST at a time. Every
weekly update says so: that the work has started rather than finished, where it
stands, and the best two or three things it turned up that week. Owner
instruction, 2026-08-10. One section, and it lives under the same budget as
every other section.

Read the counts live. Never copy them from a commit message or a previous post.

| Fact | Source |
|------|--------|
| Total requirements, MUST-level, how many are checked | the header of `ai/RFC-REQUIREMENTS.md` |
| MUSTs still owing a test | the "Coverage by RFC" line in the same file |
| Documents read end to end against their own text, and those not | `make ze-rfc-extraction-status` |

State the limit honestly: a green run proves everything on the list, and does
not yet prove the list is complete. That is why the end-to-end reading is on
the roadmap.

**MUST comes before SHOULD, and `Coming up` keeps that order.** Close what the
checking found, then the MUSTs still owing a test, then the documents not yet
read end to end. SHOULD waits behind all of it.

## Phase 2: Draft the public post

1. Create or update `website/changes/posts/<covers-start>.md`.
2. Use this front matter:

```yaml
---
covers: <YYYY-MM-DD> .. <YYYY-MM-DD>
tags: <comma-separated allowed tags>
---
```

3. Choose tags from `website/data/topics.json`. If the week needs a genuinely new topic, add it to `data/topics.json` with the right category. Do not force a near miss.
4. Decide what the week is about before writing, and leave the rest out. A full week yields far more than fits, so `STYLE.md` ("How long") governs what survives: 3 sections is normal, 5 is the ceiling, and a section carrying one bullet is a sentence in the wrong shape.
5. Write the body in Zeledon style:
   - `**📅 Ze Weekly Update**` header,
   - one short framing sentence,
   - themed sections with bold emoji headers,
   - bullets for multiple items,
   - `**🔭 Coming up**` only for planned or design work.
6. Dry-run the post and read the message count. Over 4, go back to step 4 and cut items. Do not compress the prose instead.
7. Run a self-review against the hard gates. Grep for what a grep can find rather than re-reading:
   - no em dashes,
   - no first person,
   - no internal process language,
   - no repo vocabulary (`extracted`, `enrolled`, `gated`, `polarity`, `ratchet`, `walk`, `carrier`, `artifact`, `tier`),
   - no raw hex or wire bytes,
   - no unverified shipped claims,
   - no hype,
   - fixes at one line, new surfaces named in full,
   - no sentence past about 30 words.
8. Show Thomas the exact draft and wait for approval before posting.

If Thomas asks only for a draft, stop after the draft. Do not post, archive, or regenerate the site unless asked.

## Phase 3: Publish after approval

After Thomas approves the exact text:

1. Run a dry run first:

```sh
python3 scripts/zeledon/post_weekly.py website/changes/posts/<covers-start>.md
```

2. Check the chunk count and text. The tool splits at section boundaries for Discord's message limit.
3. If Thomas wants a Discord preview, post to test only:

```sh
python3 scripts/zeledon/post_weekly.py website/changes/posts/<covers-start>.md --channel ze-test --yes
```

4. Post to `ze-news` only after approval:

```sh
python3 scripts/zeledon/post_weekly.py website/changes/posts/<covers-start>.md --yes
```

The posting tool refuses incomplete weeks unless `--force` is used. Do not use `--force` unless Thomas explicitly asks for an in-progress week to be posted. The tool archives the exact posted text to `scripts/zeledon/weekly/<covers-start>-weekly.md` after a successful post.

5. **A run that stops partway has already put messages in the channel.** Read what it printed. It names the chunk that failed and the flag that finishes the post:

```sh
python3 scripts/zeledon/post_weekly.py website/changes/posts/<covers-start>.md --resume-from <N> --yes
```

Never answer a partial send by running the command again without `--resume-from`. The archive is written only after the last chunk lands, so nothing records the week as posted, and a fresh run sends every chunk that already arrived a second time.

## Phase 4: Update the website

In `website/`, apply the checklist from `AI.md`:

1. Confirm the post has valid `tags:` front matter.
2. Check `data/features.json` and `data/milestones.json` for drift.
3. Check `docs/comparison.md` and `website/compare/comparison.md` for comparison drift.
4. Check whether a new lab page or `data/nav.json` Labs entry is needed.
5. Check whether `performance/index.html` needs fresh headline benchmark stats.
6. Run:

```sh
make ze-site-generate
```

7. Verify these outputs exist and reference the new week:
   - `changes/<covers-start>/index.html`,
   - `changes/index.html`,
   - `changes/feed.xml`,
   - `index.html` homepage `Latest updates` cards, when the new week is within the rendered latest set.
8. Do not assume the homepage card count. Read `website/tools/render-index.py` and check `sitelib.latest_blog_posts(N)`. If Thomas expects four cards and the renderer still uses a different number, update the renderer before claiming the homepage is correct.
9. Link-check the changed site. Reuse an existing local checker if available, otherwise use a temporary script that walks published `*.html` and `*.md` files, excludes `presentations/`, and resolves local `href` and `src` targets.

## Phase 5: Report

Report only grounded facts:

- the `covers:` range,
- the weekly source file path,
- the Discord archive path,
- whether `ze-news` was posted,
- which site files or data files changed,
- the `make ze-site-generate` result,
- the link-check result,
- any intentionally skipped drift item, with the reason.

Do not say the update is done unless the Discord post, archive, generated site, homepage card, feed, and link check are all accounted for.
