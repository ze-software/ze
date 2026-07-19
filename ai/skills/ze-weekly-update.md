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
- Do not hand-edit generated site pages. Edit the source data or Markdown, then run the generator.

## Required references

Read these before drafting:

1. `scripts/zeledon/STYLE.md`.
2. The latest one or two files in `scripts/zeledon/weekly/`. <!-- doc-links: ignore (archive dir created at runtime by post_weekly.py; absent on a clean tree) -->
3. `../gh-pages/AI.md`, especially `Weekly Update Checklist`.
4. `../gh-pages/data/topics.json` for allowed update tags.
5. The latest `../gh-pages/changes/posts/*.md` post, to keep format and coverage continuity.

If your working directory is `../gh-pages`, then the main repo is `../main`. If your working directory is `../main`, then the website worktree is `../gh-pages`.

## Phase 1: Establish the week

1. Find the newest archived Discord post in `scripts/zeledon/weekly/` and the newest website post in `../gh-pages/changes/posts/`. <!-- doc-links: ignore (runtime archive dir + gh-pages sibling worktree) -->
2. Determine the new `covers:` range from the previous post's end date unless Thomas gives a different range.
3. Gather what shipped during the range from `../main`:
   - inspect `git log` for the range,
   - read the touched source, docs, specs, or tests needed to understand user-visible behavior,
   - include only behavior that actually landed,
   - put design or planning work only under `Coming up`, phrased as work started, not shipped.
4. Group the week by user-facing theme, not by commit. Fold small commits into one capability when they serve one story.

## Phase 2: Draft the public post

1. Create or update `../gh-pages/changes/posts/<covers-start>.md`.
2. Use this front matter:

```yaml
---
covers: <YYYY-MM-DD> .. <YYYY-MM-DD>
tags: <comma-separated allowed tags>
---
```

3. Choose tags from `../gh-pages/data/topics.json`. If the week needs a genuinely new topic, add it to `data/topics.json` with the right category. Do not force a near miss.
4. Write the body in Zeledon style:
   - `**📅 Ze Weekly Update**` header,
   - one short framing sentence,
   - 3 to 6 themed sections with bold emoji headers,
   - bullets for multiple items,
   - `**🔭 Coming up**` only for planned or design work.
5. Run a self-review against the hard gates:
   - no em dashes,
   - no first person,
   - no internal process language,
   - no unverified shipped claims,
   - no hype.
6. Show Thomas the exact draft and wait for approval before posting.

If Thomas asks only for a draft, stop after the draft. Do not post, archive, or regenerate the site unless asked.

## Phase 3: Publish after approval

After Thomas approves the exact text:

1. Run a dry run first:

```sh
python3 scripts/zeledon/post_weekly.py ../gh-pages/changes/posts/<covers-start>.md
```

2. Check the chunk count and text. The tool splits at section boundaries for Discord's message limit.
3. If Thomas wants a Discord preview, post to test only:

```sh
python3 scripts/zeledon/post_weekly.py ../gh-pages/changes/posts/<covers-start>.md --channel ze-test --yes
```

4. Post to `ze-news` only after approval:

```sh
python3 scripts/zeledon/post_weekly.py ../gh-pages/changes/posts/<covers-start>.md --yes
```

The posting tool refuses incomplete weeks unless `--force` is used. Do not use `--force` unless Thomas explicitly asks for an in-progress week to be posted. The tool archives the exact posted text to `scripts/zeledon/weekly/<covers-start>-weekly.md` after a successful post.

## Phase 4: Update the website

In `../gh-pages`, apply the checklist from `AI.md`:

1. Confirm the post has valid `tags:` front matter.
2. Check `data/features.json` and `data/milestones.json` for drift.
3. Check `../main/docs/comparison.md` and `compare/comparison.md` for comparison drift.
4. Check whether a new lab page or `data/nav.json` Labs entry is needed.
5. Check whether `performance/index.html` needs fresh headline benchmark stats.
6. Run:

```sh
./update-website.sh
```

7. Verify these outputs exist and reference the new week:
   - `changes/<covers-start>/index.html`,
   - `changes/index.html`,
   - `changes/feed.xml`,
   - `index.html` homepage `Latest updates` cards, when the new week is within the rendered latest set.
8. Do not assume the homepage card count. Read `tools/render-index.py` and check `sitelib.latest_blog_posts(N)`. <!-- doc-links: ignore (tools/render-index.py lives in the ../gh-pages sibling repo, not this tree) --> If Thomas expects four cards and the renderer still uses a different number, update the renderer before claiming the homepage is correct.
9. Link-check the changed site. Reuse an existing local checker if available, otherwise use a temporary script that walks published `*.html` and `*.md` files, excludes `presentations/`, and resolves local `href` and `src` targets.

## Phase 5: Report

Report only grounded facts:

- the `covers:` range,
- the weekly source file path,
- the Discord archive path,
- whether `ze-news` was posted,
- which site files or data files changed,
- the `./update-website.sh` result,
- the link-check result,
- any intentionally skipped drift item, with the reason.

Do not say the update is done unless the Discord post, archive, generated site, homepage card, feed, and link check are all accounted for.
