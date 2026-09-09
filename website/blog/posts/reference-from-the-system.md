---
title: Reference stays attached to code
date: 2026-08-22
author: Thomas Mangin
description: I want Ze's public reference and its agents' working context to come from the same repository facts, so a website update and a new session cannot invent different versions of the system.

deck: Operators need to know what Ze supports, and the next agent needs to recover the decision behind it. I want both readers to reach the same source.

image: assets/blog/reference-from-the-system.svg
image-dark: assets/blog/reference-from-the-system-dark.svg
image-alt: A code-owned source publishes a reference view while an agent follows a lookup route back to the same source.
---

Building Ze has left me with two readers who need the project to explain itself. An operator needs to know which commands and configuration the system accepts, and where support for a protocol stops. An AI agent changing that system needs to recover the earlier decision and find the source which implements it, without the conversation in which we made it.

I do not want to maintain a different account for each reader. A website which remembers an older command sends the operator towards something the binary no longer accepts. An agent working from stale guidance can reintroduce the decision I already rejected, and then the next round of documentation can describe that mistake as the new design.

The website therefore has to stay attached to the sources the system already uses. The same route back to those sources can give the next agent its context. I want a correction made at the owner of a fact to reach both readers, rather than depend on somebody remembering every place we copied it.

*This article was drafted and revised with OpenAI Codex. The design, decisions and conclusions are mine.*

## Giving each fact somewhere to belong

I still have to explain why a command exists, when it is useful and which tradeoff led me to choose it. That judgement needs prose. Maintaining a second list of the command's accepted arguments spends time on something the program already has to know, and gives us another place to disagree.

The distinction is between information the system can derive and decisions somebody has to make. A dependency version belongs in `go.mod`, while the reason we use that dependency needs an explanation. A registry can give a plugin's name and configuration roots, but it cannot decide whether the plugin is a good design. Generating the former leaves the latter visible as an authored decision.

That does not make a generated page impersonal. I can group the commands in a way an operator can use, explain a limitation beside a table and link to the reason for it. The generator owns the repetitive inventory, and the page still needs an author who understands its reader.

## A command correction has one source

The command reference makes the arrangement concrete. If a help description is wrong, its correction belongs beside the command declaration, where the binary can use it. A separately written website description would require another edit for the same correction.

During a site build, Ze runs the catalogue entry point `ze help command --json` from the checkout's sources, with the project's feature tags. [`LiveCommandCatalog`](https://github.com/ze-software/ze/blob/main/internal/le/docvalid/command_render.go) supplies the catalogue, and [`publishCommandCatalog`](https://github.com/ze-software/ze/blob/main/internal/le/site/build.go) saves it as [`data/cli-commands.json`](../../data/cli-commands.json) before the pages are rendered.

The [CLI reference renderer](https://github.com/ze-software/ze/blob/main/internal/le/site/commands.go), `renderCLIReference`, reads that catalogue to publish descriptions and invocation forms. Its Markdown mirror uses the same input. Changing the grouping or layout does not require another list of commands, and rebuilding after the help correction brings the changed description into the reference.

There is still a build and publication between the source and the reader. An already published page cannot update itself when a source file changes, and a reader running an older binary may need the reference for that version. The useful guarantee is that the generated reference describes the catalogue used for that build. If the declaration's description is wrong, the generated page repeats it faithfully, so I still have to read the explanation.

## The same decision reaches beyond commands

Configuration needs the same treatment because plugins contribute to the schema. The [configuration producer](https://github.com/ze-software/ze/blob/main/internal/le/site/yang.go), `runYANGConfigTree`, builds and runs Ze's `show yang tree --config | json` command for the checkout. The reference can therefore use the configuration tree the program exposes, including the plugin contributions in that build, instead of asking someone to maintain a parallel list of leaves.

The [plugin inventory](https://github.com/ze-software/ze/blob/main/internal/le/inventory/plugins.go), in `Plugins`, reads the registrations and derives the source locations for the catalogue. Names and configuration roots come from the same declarations used by the program. That is important to the agent as well as the operator: a catalogue entry can lead back to the package responsible for the feature, where a change belongs.

Dependencies demonstrate why I do not want to generate every sentence. The [dependency page producer](https://github.com/ze-software/ze/blob/main/internal/le/site/dependencies.go) reads versions from `go.mod` and joins them to a curated list with the reasons for using each module. Its `checkDependencyDrift` rejects a direct dependency missing from that list and a listed module no longer required directly. Someone still has to write the reason, but forgetting to account for the dependency becomes a build failure.

These are variations of the same ownership decision. The command catalogue and schema already describe accepted input; the registry already names the plugins; Go already records module versions. I want the website to use that information and ask me for what the source cannot supply. Otherwise the site becomes a second database whose accuracy depends on memory.

## A reference should lead back to the decision

A current list answers only part of what either reader needs. An operator investigating a limitation needs to find its explanation, and an agent changing the relevant code needs to know why it exists. A reference which ends at the table leaves both of them searching the repository from the beginning.

Ze's source files carry `// Design:` headers pointing to the relevant document. The generated [design-to-code index](https://github.com/ze-software/ze/blob/main/ai/DOCS-TO-CODE.md) collects those references, so a reader arriving through the document can find the files which name it. Documents cite producing source through `<!-- source: ... -->` anchors, and the [code-to-documents index](https://github.com/ze-software/ze/blob/main/ai/CODE-TO-DOCS.md) answers which documents cite a source file.

The two directions are deliberately different. One starts from a design and finds its implementations; the other starts from code and finds what has been written about it. For the command reference itself, the renderer names the website's authoring guide, and the guide names the producer. That gives the developer a route from a page to the decision about how it is assembled.

I need this route to be part of the agent's instructions. A header which the model never reads retains nothing useful for the next change. The [navigation guide](https://github.com/ze-software/ze/blob/main/docs/contributing/navigating-the-code.md) tells it which index answers which question, so it can recover the design before inferring an alternative from the implementation. [The repository is half the AI harness](../the-repository-is-the-ai-harness/) describes how that becomes part of a task.

This is the part I care about most. The website gives an operator a way into the current system, and the same maintained references give a later session a way into the earlier reasoning. I do not have to reconstruct that reasoning in another conversation merely because the contributor has changed.

## A public support claim uses the same route

RFC support is a harder version of the problem because the source includes an interpretation of an external standard. The RFC remains the authority, Ze's checklist gives its obligations stable names, and tests and declared gaps refer to those names. Extraction reviews record how the checklist was derived from the text.

The [RFC ledger producer](https://github.com/ze-software/ze/blob/main/internal/le/site/rfcledger.go), `collectRequirementLedger`, gathers those records for the [public compliance pages](../../quality/rfc-compliance/). Its requirement rows come from `RequirementRows`, the same producer used for the repository's per-RFC tables. The public support wording comes directly from the summary metadata, so a separate public status table does not need to remember the judgement again.

For an operator, the route runs from a claim of partial support to the missing requirement and the evidence for what is implemented. An agent can use the same requirement ID to find the affected tests and return to the declaration when the implementation changes. The page and the development process refer to one obligation even though their readers need different views of it.

Generation cannot settle the claim. A test can name the right requirement while asserting the wrong behaviour, and an extraction review can miss the meaning of a paragraph. The ledger distinguishes test tags from stored discrimination evidence and retains gaps and runner classifications; those distinctions need to survive publication. [The proof is the expensive part](../the-proof-is-the-expensive-part/) follows the judgement behind them.

I am most wary of a polished page at that point. Precise counts and consistent formatting can make a declaration look stronger than its evidence. I want a reader to be able to reach the missing record or disputed interpretation, even when doing so weakens the project's claim.

## The machinery has its own maintenance bill

Keeping this attached replaces repeated manual updates with producers and checks which we also have to maintain. A new kind of command data can need a renderer change. A moved file leaves references to repair, and a false refusal can block a correct edit until somebody establishes whether the check or the edit is wrong.

I accept those costs because a correction at the source can serve the program and every generated view which reads it. The checks can expose an omitted dependency explanation or a stale route into the code, while I spend my time deciding whether the explanation is useful and the public claim is fair. A generator which joins the wrong sources would make the mistake consistent across every page, so the source choice needs review too.

The target is a reference built from the version of Ze we are publishing, with the explanations and limitations kept beside the facts they qualify. I want the next session to start there as well. A website which accurately remembers what Ze did last month is of little use when today's operator and tomorrow's contributor are both trying to understand the system we now have.

*Last updated: 9 September 2026. The history of this article is available in the [project's Git repository](https://github.com/ze-software/ze/commits/main/website/blog/posts/reference-from-the-system.md).*
