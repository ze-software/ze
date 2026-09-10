---
title: Reference stays attached to code
date: 2026-08-22
author: Thomas Mangin
description: Ze already knows which commands and configuration it accepts. I would rather use that information for the reference and spend the writing on explaining why the system works that way.

deck: An operator reading the website and an agent arriving to change the code should be able to find the same facts, and the reasons behind them.

image: assets/blog/reference-from-the-system.svg
image-dark: assets/blog/reference-from-the-system-dark.svg
image-alt: A code-owned source publishes a reference view while an agent follows a lookup route back to the same source.
---

Maintaining a list of commands for a website is a peculiar use of time when the program already has to know every command it accepts. We correct a help description in the code and have another copy to remember, then somebody changes an argument and the website starts giving advice the binary will refuse. We have created the disagreement ourselves.

Building Ze with AI adds another reader to this arrangement. The operator needs to find the command which does the job; an agent arriving to change it needs to discover why it exists in that form. Stale guidance can send the agent back to a design I already rejected, and the next documentation update may then describe the mistake as though we intended it.

I would rather make the correction where the information belongs and have both readers find it there. The website can use what the program already knows, while I spend the writing on what its declarations cannot explain.

*This article was drafted and revised with OpenAI Codex. The design, decisions and conclusions are mine.*

## The program already has the list

For commands, that starts with the declarations used by the binary. During a site build, Ze runs `ze help command --json` from the checkout's sources, with the project's feature tags. The [catalogue producer](https://github.com/ze-software/ze/blob/main/internal/le/docvalid/command_render.go) supplies the result, and the [site builder](https://github.com/ze-software/ze/blob/main/internal/le/site/build.go) saves it as [`data/cli-commands.json`](../../data/cli-commands.json) before rendering the pages.

The [CLI reference renderer](https://github.com/ze-software/ze/blob/main/internal/le/site/commands.go) uses that catalogue for descriptions and invocation forms. The Markdown mirror reads the same input. A help correction can therefore serve the binary and both reference views, without someone copying it between them.

There is still a build and publication to do. Changing a declaration does not update a page already on the public website, and somebody running an older binary may need that version's reference. The generated page describes the catalogue used for its build. If the help description was wrong, we have now repeated it consistently, so reading the result remains part of the job.

I still have to explain why a command is useful and which compromise led to its design. The generator cannot infer that from a list of arguments. It saves me maintaining the list, and the page can group commands for the reader and put an explanation beside a limitation without taking over ownership of the command itself.

## Configuration has more than one contributor

Configuration would be particularly easy to get wrong by hand because plugins contribute to the schema. The [configuration producer](https://github.com/ze-software/ze/blob/main/internal/le/site/yang.go) builds and runs Ze's `show yang tree --config | json` command for the checkout. The reference uses the tree the program exposes, including the plugin contributions in that build, rather than a second list of configuration leaves somebody has to keep up to date.

The [plugin inventory](https://github.com/ze-software/ze/blob/main/internal/le/inventory/plugins.go) reads registrations and derives source locations for its catalogue. Plugin names and configuration roots come from the declarations used by the program. An agent following an entry can therefore get back to the package responsible for the feature, instead of finding a description and having to begin the source search again.

Dependencies are a useful example of where generation should stop. A module's version is already in `go.mod`; the reason I chose it is not. The [dependency page producer](https://github.com/ze-software/ze/blob/main/internal/le/site/dependencies.go) joins the versions to a curated list of reasons, and its drift check rejects a direct dependency missing from that list or a listed module which is no longer required directly.

Someone still has to write the explanation. A build failure can remind us that the new dependency has none, but it cannot decide whether introducing the dependency was sensible. That seems a reasonable division of labour to me, and avoids pretending that enough generated tables will eventually explain the design.

## Finding out why

A current reference gets an operator to the accepted input. It does less for somebody investigating a limitation, or for an agent about to change the code which imposes it. For those readers, the route needs to continue to the earlier reasoning.

Ze's source files have `// Design:` headers pointing to their design documents. The generated [design-to-code index](https://github.com/ze-software/ze/blob/main/ai/DOCS-TO-CODE.md) collects those references, so somebody starting with a document can find the files which name it. In the other direction, documents cite producing source through `<!-- source: ... -->` anchors, and the [code-to-documents index](https://github.com/ze-software/ze/blob/main/ai/CODE-TO-DOCS.md) finds the documents which cite a file.

These answer different lookups, which is why there are two indexes. One finds implementations from the design they name, while the other finds what has been written about a source file. In the command reference's own case, the renderer names the website authoring guide and the guide names the producer. Somebody trying to change the page can get from the output to the explanation of how it is assembled.

The agent also needs to be told to use this. A header it never reads does little to preserve a decision, so the [navigation guide](https://github.com/ze-software/ze/blob/main/docs/contributing/navigating-the-code.md) explains which index answers which lookup. [The repository is half the AI harness](../the-repository-is-the-ai-harness/) describes how those routes become part of the task, before the model starts inventing an alternative from the code it has found.

This is the benefit I care about beyond saving a documentation edit. The operator can find why a limitation exists, and a later session can recover the same reasoning without needing the conversation in which we settled it. Having already spent time making the choice, I would prefer not to reconstruct it merely because a new contributor has arrived.

## Publishing what we support

RFC support makes this arrangement more demanding. The original RFC remains the authority, but Ze has to interpret it into obligations which tests and known gaps can refer to. The checklist gives them stable IDs, and extraction reviews record how those obligations were derived from the text.

The [RFC ledger producer](https://github.com/ze-software/ze/blob/main/internal/le/site/rfcledger.go) gathers those records for the [public compliance pages](../../quality/rfc-compliance/). It uses the same requirement rows as the repository's per-RFC tables, and takes the public support wording from the summary metadata. There is no separately authored website status table waiting for someone to remember that the summary changed.

An operator can follow a declaration of partial support to the missing requirement and to the tests for what is implemented. An agent can use that same requirement ID to find the tests affected by a change and return to the declaration. They arrive for different reasons, but they should not end up reading different accounts of what Ze supports.

Generation cannot settle an RFC interpretation, and a test can carry the right tag while asserting the wrong behaviour. The ledger therefore keeps tagged tests distinct from stored discrimination evidence, and retains the gaps and runner classifications. [The proof is the expensive part](../the-proof-is-the-expensive-part/) follows the judgement behind those distinctions, including records which are still missing.

A polished page can make this worse if its presentation gives an unsupported claim an air of authority. Precise counts and consistent formatting are easy to produce. I am much more interested in whether the reader can reach the missing record or disputed interpretation, even when following the link leaves them less impressed with Ze.

## There is still maintenance

This arrangement replaces repeated manual edits with producers and checks, which need looking after too. A new kind of command data may require a renderer change, a moved file can break a reference, and a false refusal can interrupt a correct edit while somebody works out whether the edit or the check is wrong.

I accept that cost because the correction can serve the program and every generated view which reads it. The dependency check can catch an explanation we forgot to write, and a source-link check can expose a broken route into the code. Neither can tell me whether the explanation is useful. A generator joining the wrong sources would even make the same mistake everywhere, so the choice of source needs review as much as the prose does.

When publishing a version of Ze, its reference should be built from that version, with the reasons and limitations beside the facts they qualify. The arriving agent can start from those same maintained sources. I have enough design decisions left to make without paying to rediscover the old ones in a conversation, then remembering to correct a second account on the website.

*Last updated: 11 September 2026. The history of this article is available in the [project's Git repository](https://github.com/ze-software/ze/commits/main/website/blog/posts/reference-from-the-system.md).*
