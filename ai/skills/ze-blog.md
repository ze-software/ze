---
name: ze-blog
description: Write and revise Thomas's blog articles as connected accounts of Ze development. Review each article's message, its sections and its sentences for reader understanding. Read ze-author first.
---

# Blog articles

This skill covers what is specific to a blog article. Read `ze-author` first for the meaning-first review and Thomas's voice.

Posts live in `website/blog/posts/*.md` with YAML front matter (title, date, author, description). The pages under `website/blog/<slug>/` and `website/data/search-index.json` are generated, so edit the source post and regenerate with `./le site build`. The website source moved from the `gh-pages` branch into `main` under `website/` on 2026-08-17 (`b0430c2a9`), and `gh-pages` is now the published artifact branch alone.

## The articles tell a related story

The Ze blog records the development of Ze: the problems Thomas encounters, his decisions, their results, and what he learns from them. Each article develops part of that story. Do not rewrite the collection into unrelated essays about AI or software engineering.

Before editing an article, read it in full and read the related posts it refers to. Identify its own message and how it extends the account in those posts. For a collection-wide revision, read every article and check the sequence of ideas across the collection as well as within each post. Preserve differences in purpose; a later article can examine a limitation or consequence without retelling the earlier article.

When an article has been rewritten repeatedly, use Git history to read its earliest version and substantive author corrections before another revision. Follow its old path if the source moved. Compare arguments and examples to find what the rewrites lost; the latest draft alone is insufficient. Thomas's current brief takes precedence, and an old technical claim still needs current evidence or a historical qualification.

Review every section for how it advances the article's message, then every sentence for how it explains that section to the reader. Reorder or rewrite when the reasoning is hard to follow. Do not preserve a confusing structure merely because each sentence is grammatical. Include enough context for someone arriving directly at a post, and use cross-links where another article develops a relevant point. Explain that connection in the surrounding prose.

Apply `ze-author`'s "Explain the relationships between the facts" guidance throughout the article. In a development account, explain what caused the difficulty and how the design decision addresses it. The reader must be able to follow why one led to the other. An opening that lists a user problem, a Ze feature and a benefit still needs that explanation; accurate facts and an agreed central message do not supply it automatically.

Titles, descriptions and decks must express the same point as the article. A series-wide pass also checks repeated explanations, inconsistent terminology and claims that disagree between posts. Keep the date and qualifications attached to historical observations so later results do not silently change the earlier account.

## Author-confirmed messages for the current series

Thomas clarified these messages on 21 September 2026. Use them when reviewing the current articles. Do not replace an article's purpose with an argument inferred from its technical examples. A new direction requires a new brief from Thomas.

| Article | Message to develop |
|---|---|
| How Ze reuses memory for BGP UPDATEs | Efficiency, including memory efficiency, is a deliberate pattern of Ze development even in a garbage-collected language. |
| The proof is the expensive part | Unit tests and CI do not establish correct application behaviour. Ze's RFC testing framework aims to establish that behaviour against protocol requirements. |
| The repository is half the AI harness | AI development needs metaprogramming: explicit objectives and validation of both the objective and all required qualities. Ze's rules help agents recognise work below that standard. |
| AI coding has not had its Rails moment | AI development lacks shared conventions for relating metadata, including documentation and designs, to code so that the implementation follows it. |
| Keeping documentation in step with the code | Generate user-facing reference data from the code so it stays consistent with the implementation it describes. |

Thomas retired **One BGP UPDATE, many peers** on the same date. Remove links to it; do not recreate it or move its whole argument into the memory article.

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

## Voice references

Thomas approved `website/blog/posts/ai-coding-has-not-had-its-rails-moment.md` as a voice reference on 11 September 2026. His later request to revise all articles for clarity includes that post. Use it for perspective alongside `ze-author`; do not treat its wording or structure as exempt from review, or imitate the current draft as proof of quality.

Preserve the central argument Thomas gives you. Develop it through his problem, decisions and their consequences. Anecdotes and technical details support that argument; they must not replace it with a familiar AI story or a sequence of disconnected personal statements.

Voice approval is not permission to commit or publish a draft. Keep requested prose revisions uncommitted until Thomas explicitly asks for a commit.

## Historical source material

Use original TiddlyWiki posts for perspective, opinions and humor. Do not imitate their older grammar or clipped passages. Do not use the AI-written Aikido pages as voice sources. The observations below describe the historical posts, not the current prose style to reproduce.

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
- Calls his own posts "rants" or "ramblings".
- Technical authority delivered conversationally.

The old posts contain rhetorical questions ("But how can 2737504257 be in the AS-PATH ! ? !"). Do not copy that. The `ze-author` rule against rhetorical questions applies to blog articles too. Questions belong in a post only when they are genuine and answered, as in the practical checklists in "The proof is the expensive part".

NOTE: "AI objections, a story I have heard before", "What Neurodivergence Really Means", and "The lack of AI control plane" were authored by Claude from Thomas's input. Do NOT use these as style references. They are examples of what to improve.
