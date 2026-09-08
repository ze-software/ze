---
title: Reference stays attached to code
date: 2026-08-22
author: Thomas Mangin
description: How Ze builds its command reference from the binary, and why generated pages still need human judgement.

deck: A command's help text can reach the website from the same declaration the binary uses. Keeping the explanation correct still takes a reader.

image: assets/blog/reference-from-the-system.svg
image-dark: assets/blog/reference-from-the-system-dark.svg
image-alt: A code-owned source publishes a reference view while an agent follows a lookup route back to the same source.
---

A command changes in Ze, and the website needs to describe the changed command. If somebody has to remember to update a separate list, there are now two places which can disagree. The operator discovers that disagreement when the documented command does something else, or does not exist in the binary they are running.

I want to spend my time explaining why a command exists and when to use it. Maintaining another copy of its name and arguments gives me very little in return. Ze therefore builds its command reference from the binary's own catalogue, and the interesting difficulty is deciding how much of a page that catalogue can honestly supply.

*This article was drafted and revised with OpenAI Codex. The design, decisions and conclusions are mine.*

## From a command to its reference page

Consider correcting a command's help text. The correction belongs beside the command declaration, where it can change the help the program gives. A website with its own hand-written description would need the same correction a second time.

During a site build, Ze compiles and runs the command catalogue entry point, `ze help command --json`, with the project's feature tags. The build saves its answer as [`data/cli-commands.json`](../../data/cli-commands.json). The [CLI reference](../../reference/cli/) reads that file to produce the command rows, including their descriptions and invocation forms. Its Markdown mirror is rendered from the same catalogue.

The [catalogue publisher](https://github.com/ze-software/ze/blob/main/internal/le/site/build.go) and [reference renderer](https://github.com/ze-software/ze/blob/main/internal/le/site/commands.go) are separate parts of that process. The publisher obtains the data from the program. The renderer decides how to group it and where to put it on the page. Changing the layout does not require another command list.

When the site is rebuilt after the help correction, the reference receives the changed description. There is still a build and a publication step between the source and the reader, so this does not make an already published website update itself. It ties the page to the checkout used to build it.

It also ties mistakes together. If the command description is wrong at its declaration, the generated reference repeats it faithfully. I still have to read the description and decide whether it explains the command.

## Following the page back to its design

A catalogue can state which arguments a command accepts. It cannot explain why the command belongs in a plugin, or why an apparently convenient shortcut would break that plugin's independence. Those decisions need prose, and somebody changing the command needs a way to find it.

Ze's source files carry `// Design:` headers pointing to the relevant document. The [generated design index](https://github.com/ze-software/ze/blob/main/ai/DOCS-TO-CODE.md) groups those references by document, so a reader starting from the design can find the files which name it. Documents also cite producing functions with source anchors. The [navigation guide](https://github.com/ze-software/ze/blob/main/docs/contributing/navigating-the-code.md) explains which direction each index answers.

For the command reference itself, the renderer points to the website's authoring guide, and the guide names the renderer. A developer can start with the public page, find how it is produced, then reach the decision about which data it is allowed to publish.

This is also useful to an agent arriving without the previous conversation. The links give it a route to the design instead of requiring it to reconstruct the decision from the implementation. [The repository is half the AI harness](../the-repository-is-the-ai-harness/) describes how we make that route part of a task.

## A support claim needs more than a catalogue

The [RFC compliance pages](../../quality/rfc-compliance/) use the same publishing approach with more difficult source material. Their input includes requirements, tagged tests and records of what those tests demonstrate. The [ledger producer](https://github.com/ze-software/ze/blob/main/internal/le/site/rfcledger.go) collects that material from the repository before the pages are rendered.

The public support wording comes from the RFC summary's metadata. The requirement rows come from the same renderer used for the repository's requirement tables. Generating the page keeps those records together, but the support wording is still a judgement somebody wrote. The number of linked tests cannot settle it.

A test can name the right requirement and assert the wrong behaviour. A reviewer can read a paragraph of the standard and miss what it requires. Those mistakes survive a generator which joins every file correctly. [The proof is the expensive part](../the-proof-is-the-expensive-part/) follows the evidence behind a requirement and the checks needed before trusting it.

This is where I am most wary of a polished reference page. Consistent formatting and precise counts make a claim easy to believe. The page needs to leave the evidence and its limits visible enough for a reader to disagree with that claim.

## What I still have to maintain

The generators require maintenance, and the links need attention when files move. A new kind of command data can require a renderer change. An automatic check can reject a correct edit, and somebody has to find out whether the check or the edit is wrong.

I accept that cost because the command list is useful to the program as well as the website. A correction there can reach both. The explanation still needs a person who understands the command, and that is where I would rather spend the time.

*Last updated: 8 September 2026. The history of this article is available in the [project's Git repository](https://github.com/ze-software/ze/commits/main/website/blog/posts/reference-from-the-system.md).*
