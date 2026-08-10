---
name: ze-blog
description: Structure and register for Thomas's blog articles, including everything under blog/posts/. Use when writing or editing a blog post. Read the ze-author skill first for voice and AI anti-patterns.
---

# Blog articles

This skill covers what is specific to a blog article. The voice, and the AI tells to avoid, are in the `ze-author` skill and apply here too. Read that first.

Posts live in `blog/posts/*.md` with YAML front matter (title, date, author, description). The pages under `blog/<slug>/` and `data/search-index.json` are generated, so edit the source post and regenerate with `./update-website.sh`.

## How an intro is built

An article opens by setting the scene, then throws the hook, then discloses the use of AI. In that order.

1. **Set the scene.** Whatever context the reader needs to understand why the subject matters. This can be several paragraphs.
2. **The hook, as the last point of the intro.** One or two sentences that make the reader want to keep going: the promise of the article, the question it will answer, or the place the argument starts from. It is the last thing before the disclaimer, so it carries the reader into the first section.
3. **The AI disclaimer.** When Claude helped write the article, say so honestly, in italics, immediately after the hook. Existing wording to reuse: *This article was co-authored with Claude. The argument and the conclusions are mine. Claude helped organise the material and draft the text.*

The hook must not be a list of what the article covers ("What follows is X, why Y, and what Z"). That three-part preview reads as AI and gives the reader the summary instead of a reason to continue.

## Structure of the body

- Headers and sections are welcome. Structure is a feature of the blog register, not an AI artifact.
- Tables are fine for a genuine lookup. They carry no interactive decoration on an article: the blog template sets `data-table-columns="off"`, which keeps the "Show columns" selector off blog pages because it breaks the flow of reading.
- Short paragraphs, rarely more than three or four sentences.
- Links go inline to their source, without ceremony.
- Examples earn their length. Cut an illustration down to the one detail that carries the point.

## Blog register (thomas.mangin.com, his own writing)

Short technical commentary (2007 to 2008 era: Cogent, Comcast, Phorm series):
- Short, direct posts. Gets to the point fast.
- States opinions flat out: "it look like Cogent is the bad guy".
- Speculates openly with honest disclaimers: "This is pure speculation".
- Ends abruptly. No neat conclusions.
- Humour and informal asides: "some crazy russians :D".

"Claude Code Best Practices" (December 2025):
- Direct, practical, experience-based.
- "Every rule exists because something went wrong".
- Scar-tissue tone.

"Can you not see we are competing?" and "The real cost of P2P":
- Opinionated, self-described "rants".
- Industry insider perspective stated frankly.

Across all his blog posts:
- Personal voice: "I can only conclude", "I am inclined to think".
- Calls his own posts "rants" or "ramblings".
- Technical authority delivered conversationally.
- Older posts have non-native speaker grammar; current English is much stronger.

The old posts contain rhetorical questions ("But how can 2737504257 be in the AS-PATH ! ? !"). Do not copy that. The `ze-author` rule against rhetorical questions applies to blog articles too. Questions belong in a post only when they are genuine and answered, as in the practical checklists in "The proof is the expensive part".

NOTE: "AI objections, a story I have heard before", "What Neurodivergence Really Means", and "The lack of AI control plane" were authored by Claude from Thomas's input. Do NOT use these as style references. They are examples of what to improve.
