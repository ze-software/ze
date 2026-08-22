# Reference from the system

*2026-08-22 by Thomas Mangin*

A large project has to document itself for two readers: users and the agents that change it.

The first objective is public reference. Users need current facts when they decide whether Ze fits their network and when they operate it. If the site lags behind the code, the project moves risk to the operator.

The second objective is context for AI work. An agent needs to find the rule, source owner, evidence and prior decision while it edits. If that context comes from the same repository facts each time, the next session starts from the same map instead of inventing a new one.

Those objectives are equal. User documentation keeps the project honest in public. Agent context keeps decisions consistent over time.

The trick we used in Ze is simple. Keep each fact where the system already enforces it. Generate the reference page from that place. Check the links back to evidence.

This is useful beyond Ze. If your project already has structured knowledge in the repository, do not copy it into a page by hand.

Generate the table from the source that owns the fact. Then write the explanation around it.

*This article was drafted with OpenAI Codex. The design, decisions and conclusions are mine.*

## The useful split

A human writes context, judgement and tradeoffs. The system writes lists, counts and relationships.

That split keeps reference pages from becoming a second database. Each fact stays with its owner in the repository, and the website reads those owners to publish a view.

The scale is why this matters.

| Public reference | Source that owns the fact | Scale covered | Why generation helps |
|---|---|---:|---|
| CLI reference | Live command registry | 395 commands across 48 groups | The page cannot forget a command the binary exposes. |
| Configuration reference | YANG schema from `ze yang tree` | 36 top-level sections, 27 from plugins | The page follows the schema when a plugin adds a leaf. |
| Plugin catalogue | Plugin registry and metadata | 90 runtime plugins, 6 fixtures | Names, purposes, config roots and source paths come from registration. |
| Dependencies | `go.mod` plus written reasons | 42 direct dependencies | Versions come from Go, while the reason stays human-written. |
| RFC status | RFC requirement ledger | 4,703 requirements across 177 summaries | Public support claims stay tied to tests, gaps and annotations. |

The page can still be readable. It can group commands, add search, explain why a dependency exists, or warn that an RFC is partial. The fact itself still comes from the place that changes when the product changes.

Some pages are not regenerated every time the data changes. For those, the HTML carries `data-ze-stat` markers. JavaScript fetches `data/site-facts.json` and updates those values in the browser. The page stays static, while the visible count still comes from the latest generated data.


## Links turn a page into a route

A generated page is still weak if the reader cannot move from a claim to its source.

Ze uses links in both directions. Source files carry headers such as `// Design:` near the top. They point to the rule or design document that explains the file. Documents point back with `<!-- source: ... -->` anchors.

Two generated indexes invert those links:

```text
code file       -> documents that cite it
design document -> source files that name it
```

That is not small either.

| Link type | Current scale | What the link answers |
|---|---:|---|
| `// Design:` headers | 3125 | Which design or rule governs this file? |
| `// Related:` headers | 2076 | Which nearby file owns the related detail? |
| `// RFC:` headers | 773 | Which standards text is relevant here? |
| `<!-- source: ... -->` anchors | 6228 | Which source path supports this document claim? |
| Generated code-doc indexes | 2 files, about 600 KB together | Which side mentions the other side? |

A link is not proof. It is a route to proof. The useful property is that a stale route fails before a reader follows it.

This helps a developer with limited time. They can start from a public claim, follow the requirement or source link, and get to the code path. They do not have to search the whole tree.

The same path has to be part of the AI role. Ze's instructions tell the agent which header or generated index to read before it studies a subsystem. Without that rule, the tag is just text. The model can miss it, so the reference does not shape the work.

That is where the consistency comes from. The tag exists in the file, the generated index makes it searchable, and the role tells the agent when to use it.

## RFCs show the whole system

RFC compliance is the best example because it joins external text, local knowledge, tests and public claims.

The external authority is the RFC. Ze does not get to redefine it. The local knowledge lives under `rfc/short/`, where each RFC summary gives stable ids to the requirements. A requirement id such as `RFC7606-7.1-1` lets a test, an audit result, a gap and a public status row all name the same obligation.

A summary can still miss a sentence. That is the failure mode that makes compliance checklists dangerous. The checklist can be green because nobody wrote down the missing rule.

Ze uses `rfc/extraction/<stem>.json` to bound that risk. The file starts as a generated skeleton from the RFC source text. A reviewer classifies each normative site by hand. Each site maps to a requirement id, or gets excluded with a reason from a closed set.

Tests then carry the same ids:

```go
// RFC requirement: RFC7606-7.1-1 negative - ORIGIN length 2 selects treat-as-withdraw.
```

`make ze-rfc-index-update` joins the summary with the test tags. It writes one generated file per RFC under `rfc/requirements/`, and an index over all of them in `ai/RFC-REQUIREMENTS.md`.

The chain is deliberately boring.

| Layer | File or page | Job | Failure it catches |
|---|---|---|---|
| External text | `rfc/full/<stem>.txt` | Keep the RFC text local and stable. | A claim based on memory. |
| Local summary | `rfc/short/<stem>.md` | Give each obligation a stable id. | A test or gap with no named requirement. |
| Extraction sign-off | `rfc/extraction/<stem>.json` | Map source text to requirement ids. | A summary that missed a normative sentence. |
| Test tag | `RFC requirement:` comment | Tie evidence to one requirement id. | A passing test that proves no public claim. |
| Per-RFC ledger | `rfc/requirements/<stem>.md` | Join requirements, tests, gaps and evidence type. | Evidence hidden in the tree. |
| Global ledger | `ai/RFC-REQUIREMENTS.md` | Show coverage across all summaries. | Backlog hidden across many files. |
| Public page | `reference/rfcs/` | Publish support and gaps. | Private gaps missing from the public claim. |

The public RFC page is a support view over that system. A supported row has to stay tied to current source, tests or documentation. If a private `{gap}` annotation exists in `rfc/short/`, the public page has to disclose it. A newly enrolled RFC has to get a public row.

That is the system I wanted. A public claim can be followed to a requirement. The requirement can be followed to evidence. The evidence can be followed to code. When the code changes, the route has to move with it.

## Why this serves Ze

This machinery pays for itself when the project changes.

The public site gives users the current support view and does not turn reference pages into separate ledgers.

The same structure gives agents a route through the repository. A change can start from the page, reach the requirement, reach the source, and then return to the page if the public claim changed.

That is the part I care about most. The website is useful for users, and the same links make AI work less forgetful. A later session can find the decision the earlier session used.

I hope the article is useful for other projects for that reason. The exact generators are Ze-specific. The shape is more general: public reference and working context can come from the same checked facts.

## The cost is worth naming

This is not free.

Generators become code to maintain. Build logs get longer. A false positive can block correct work. A source link that felt helpful when it was written can become debt when files move.

The system can also be believed too much. A generated page can be generated from the wrong source. A test tag can name the right requirement and assert the wrong behaviour. An extraction sign-off can record a walk over RFC text and still miss the meaning of a paragraph.

The fix is honesty. Each generated file has to say what it knows. The RFC ledger separates unit evidence from functional evidence, verification evidence from nightly evidence, mapped requirements from annotations, and extraction sign-offs from perfect understanding.

A page that publishes its uncertainty is safer than a page that says "supported" and stops.

## The shape I want

The reference comes from the system wherever the system already owns the fact.

A human writes the explanation, the judgement and the tradeoff. The program provides the lists, relationships and counts. The gate checks the links between them.

The result has to serve both readers. The public page tells users what the system supports now. The same links give an agent the context it needs to make the next decision consistent with the last one.

I do not want a website that remembers what Ze did last month. I want a website that is rebuilt from what Ze is now.
