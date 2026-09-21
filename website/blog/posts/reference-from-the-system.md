---
title: Keeping documentation in step with the code
date: 2026-08-22
author: Thomas Mangin
description: Ze generates command, configuration and support reference data from the code and its records, so the website does not depend on a second list being updated by hand.

deck: Shared command metadata supplies both the program and its reference pages, so keeping them consistent becomes part of the documentation build.

image: assets/blog/reference-from-the-system.svg
image-dark: assets/blog/reference-from-the-system-dark.svg
image-alt: Command definitions supply the reference pages, with links from the documentation back to the source.
---

When a command reference is written separately from the code, keeping it accurate depends on someone remembering to update it each time the command changes. The code can be changed and tested while the website still describes the old syntax, leaving a reader to follow instructions that no longer work. The documentation was correct when written; it became misleading because maintaining it required a separate edit that was missed.

In Ze, we address this by using shared command metadata to generate the reference pages. That metadata describes the arguments and their help text for use in the program, so it also provides the information needed to document them. An argument change is then recorded in one place and appears in the reference on the next documentation build. Keeping the two consistent becomes part of how we build the software and its documentation, instead of something a developer has to remember afterwards.

*This article was drafted and revised with OpenAI Codex. The design, decisions and conclusions are mine.*

## From a command definition to its reference page

The command metadata is available through Ze's built-in help. As well as displaying it in a terminal, we can request it as structured data with `ze help command --json`. The [catalogue producer](https://github.com/ze-software/ze/blob/main/internal/le/docvalid/command_render.go) runs that command against Ze compiled from the checkout, giving the website build a list of commands from the code being documented.

The [reference generator](https://github.com/ze-software/ze/blob/main/internal/le/site/commands.go) uses this list to produce the syntax and descriptions for both the website and a Markdown reference. Because both versions use the same input, correcting a help description once updates the terminal help and, on the next build, both documents. We can change how the reference is presented without having to maintain its command list separately.

The build also saves the catalogue as [`data/cli-commands.json`](../../data/cli-commands.json). Someone who wants to use the command information in another tool can read that data directly, without extracting it from the HTML page.

## Include the configuration supplied by plugins

Configuration presents the same maintenance problem, with the added complication that plugins can contribute their own settings. A reference maintained separately would have to track those contributions as well as changes to the core. If a plugin adds a setting and its author forgets the website, the new option can remain undocumented even though it is available in the program.

Ze's configuration tree already combines those definitions for the running program. The [configuration reference generator](https://github.com/ze-software/ze/blob/main/internal/le/site/yang.go) obtains that tree with `show yang tree --config | json`, including the settings supplied by plugins in the build. Generating the reference from this tree includes their contributions without a second description of the available configuration.

We use the registrations in the code for the [plugin inventory](https://github.com/ze-software/ze/blob/main/internal/le/inventory/plugins.go) too. Each entry identifies the plugin and its configuration root, with a link to the package that implements it. The reference can therefore show where a setting belongs and give a reader investigating it a route into the source.

## Publish the support records we use during development

Command names and configuration options can be taken directly from definitions in the program. Protocol support requires more judgement: the presence of BGP code does not establish that every requirement of a particular RFC has been implemented. A manually written list of supported RFCs would also need revisiting whenever we implemented a missing requirement or discovered a gap.

For Ze, we record individual RFC requirements and connect them to tests and records of missing behaviour. These are the records we use to assess implementation progress, so we use them to generate the [public compliance pages](../../quality/rfc-compliance/) as well. The [RFC ledger generator](https://github.com/ze-software/ze/blob/main/internal/le/site/rfcledger.go) combines the requirement rows with our recorded descriptions of support. Updating a gap or a description during development then updates the public account on the next website build.

This also lets a reader examine what a support claim means. A declaration of partial support links to the missing requirements and the tests cited for implemented behaviour. The page preserves the distinction between a test carrying a requirement identifier and evidence that the test detects a particular incorrect implementation. [The proof is the expensive part](../the-proof-is-the-expensive-part/) explains why that distinction matters: a test can pass while leaving the required application behaviour unchecked.

Using the same records for development and publication also gives both readers the same way to investigate a claim. An operator can follow a support entry to the requirement and its evidence. An agent changing that behaviour can follow the same identifiers to the tests and source, then update the records from which the public page is built. We avoid maintaining a separate account for each audience.

Generating the page keeps it consistent with our records; the accuracy of those records still depends on the evidence behind them. Publishing the gaps alongside the claims gives someone considering Ze a way to inspect that evidence, including reasons to be less confident in the implementation.

## Keep the explanations alongside the generated facts

The reason for a design choice cannot usually be recovered from the code that implements it. A dependency list makes this limit easy to see: `go.mod` records which modules we use and their versions, but it does not record why I chose them. The [dependency page generator](https://github.com/ze-software/ze/blob/main/internal/le/site/dependencies.go) therefore combines those versions with explanations we maintain separately.

Those explanations can become outdated too, so a check compares them with the direct dependencies. It reports a new dependency with no explanation, or an explanation for a module that is no longer used directly. This identifies an omission for us to correct, while reviewing whether the explanation is useful remains a writing task.

The same applies to documents about Ze's design. We can generate a command's accepted arguments, but explaining why we chose that interface requires the reasoning behind the decision. [AI coding has not had its Rails moment](../ai-coding-has-not-had-its-rails-moment/) describes how we connect those explanations to the code, so the person or agent changing it can find the decisions they need to consider.

Even the generated reference must be read and checked. An incorrect help description will appear in the terminal and on the website, and the generator will reproduce it until we correct the source. The pages must also identify which build they describe, because documentation generated from today's code can give the wrong instructions to someone running an older build.

For Ze, keeping the public website in step with the published code is a requirement. Generating the reference removes the separate manual description, but the resulting pages still have to be rebuilt and published with the corresponding changes. Otherwise we would have correct documentation in the build directory and outdated instructions in front of the reader.

*Originally written: 22 August 2026. Last updated: 21 September 2026. The history of this article is available in the [project's Git repository](https://github.com/ze-software/ze/commits/main/website/blog/posts/reference-from-the-system.md).*
