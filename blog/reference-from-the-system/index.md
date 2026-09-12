# Reference stays attached to code

*2026-08-22 by Thomas Mangin*

An operator reading the website and an agent arriving to change the code should be able to find the same facts, and the reasons behind them.

![A code-owned source publishes a reference view while an agent follows a lookup route back to the same source.](../../assets/blog/reference-from-the-system.svg)

An operator looking up a command needs the website to describe what the program accepts. Yet maintaining that list by hand gives us another copy to remember whenever an argument changes or a help description is corrected. The binary can reject advice which was perfectly good when somebody wrote the page, and we have created the disagreement ourselves.

The program already needs a declaration of each command in order to offer it. Using that declaration for the reference removes the repeated edit and leaves the writing for an explanation of why the command is useful, or why it has a limitation the operator needs to understand.

Building Ze with AI gives those explanations another reader. An agent arriving to change a command needs to recover the reasoning which led to it, and its next edit should leave that reasoning available to the operator too. Keeping the reference attached to the source gives both readers somewhere to continue when a list of accepted inputs is no longer enough.

*This article was drafted and revised with OpenAI Codex. The design, decisions and conclusions are mine.*

## The program already has the list

During a site build, Ze runs `ze help command --json` from the checkout's sources, with the project's feature tags. The [catalogue producer](https://github.com/ze-software/ze/blob/main/internal/le/docvalid/command_render.go) supplies the result, and the [site builder](https://github.com/ze-software/ze/blob/main/internal/le/site/build.go) saves it as [`data/cli-commands.json`](../../data/cli-commands.json). The [CLI reference renderer](https://github.com/ze-software/ze/blob/main/internal/le/site/commands.go) uses that catalogue for descriptions and invocation forms, and the Markdown mirror reads the same input.

A help correction can therefore serve the binary and both reference views without someone copying it between them. The page can still group commands for the reader and put an explanation beside a limitation. Its presentation has a different job from the command declaration, so there is no need for it to own another command list.

Configuration benefits from the same arrangement, particularly because plugins contribute to the schema. The [configuration producer](https://github.com/ze-software/ze/blob/main/internal/le/site/yang.go) builds and runs Ze's `show yang tree --config | json` command for the checkout. The reference uses the tree the program exposes, including the plugin contributions in that build, rather than a second list of configuration leaves.

The [plugin inventory](https://github.com/ze-software/ze/blob/main/internal/le/inventory/plugins.go) reads registrations and derives source locations for its catalogue. Plugin names and configuration roots come from the declarations used by the program, so a reader can follow an entry back to the package responsible for the feature.

This consistency is bounded by the build and publication. Editing a declaration leaves the existing public page unchanged until it is rebuilt and published, and a future release will need a reference built from that version's sources. It is also possible to be consistently wrong: a mistaken help description will be repeated in every view. Reading the generated result remains part of the job.

## Writing what the declarations leave out

Dependencies need both a declaration and an explanation, with a different source for each. A module's version is already in `go.mod`, while the reason I chose it has to be written. The [dependency page producer](https://github.com/ze-software/ze/blob/main/internal/le/site/dependencies.go) joins those versions to a curated list of reasons, and its drift check rejects a direct dependency missing from that list or a listed module which is no longer required directly.

The build can then remind us that a new dependency has no explanation. Deciding whether that dependency was sensible still requires reading the reason and considering the choice. Treating both inputs as though they were interchangeable would lose the distinction between what the program contains and why I put it there.

## Following the explanation back

An agent changing the implementation needs that distinction as much as an operator investigating a limitation. If it finds only the accepted input and the code which processes it, it can infer a different design and start removing a constraint I chose deliberately. The earlier reasoning needs to be reachable from the code it is about to change.

Ze's source files have `// Design:` headers pointing to their design documents. The generated [design-to-code index](https://github.com/ze-software/ze/blob/main/ai/DOCS-TO-CODE.md) collects those references, so somebody starting with a document can find the files which name it. In the other direction, documents cite producing source through `<!-- source: ... -->` anchors, and the [code-to-documents index](https://github.com/ze-software/ze/blob/main/ai/CODE-TO-DOCS.md) finds the documents which cite a file.

The two indexes answer different questions: which files implement a named design, and which documents describe a source file. In the command reference's own case, the renderer names the website authoring guide and the guide names the producer. A task which begins with a problem on the page can continue into the explanation of how it is assembled, then into the code responsible for it.

The [navigation guide](https://github.com/ze-software/ze/blob/main/docs/contributing/navigating-the-code.md) teaches the agent which route to use. A header it never reads would do little to preserve a decision, so consulting the design has to become part of how it approaches the task. [The repository is half the AI harness](../the-repository-is-the-ai-harness/) describes the instructions and feedback behind that habit.

When a change does alter the design, the same links identify the explanation which needs attention. A later session can recover the revised reasoning without needing the conversation in which we settled it, and the operator no longer has to read a description of the old choice beside a reference generated from the new one.

## Publishing what we support

RFC support tests this connection between declarations and explanations more severely. The original RFC remains the authority, but Ze has to interpret it into obligations which tests and known gaps can refer to. The checklist gives those obligations stable IDs, and extraction reviews record how they were derived from the text.

The [RFC ledger producer](https://github.com/ze-software/ze/blob/main/internal/le/site/rfcledger.go) gathers those records for the [public compliance pages](../../quality/rfc-compliance/). It uses the same requirement rows as the repository's per-RFC tables, and takes the public support wording from the summary metadata. A correction to that metadata can reach the website through its next build without a separately authored status table waiting for another edit.

An operator can follow a declaration of partial support to the missing requirement and to the tests for what is implemented. An agent can use the same requirement ID to find the tests affected by a change and return to the declaration. Their reasons for arriving differ, but the accounts of what Ze supports should agree.

Agreement alone is insufficient here because a test can carry the right tag while asserting the wrong behaviour, just as an extraction can misinterpret the RFC. The ledger keeps tagged tests distinct from stored discrimination evidence, and retains the gaps and runner classifications. [The proof is the expensive part](../the-proof-is-the-expensive-part/) follows the judgement behind those distinctions, including records which are still missing.

A polished page can give an unsupported claim an air of authority, particularly when precise counts and consistent formatting suggest that somebody has checked everything behind them. I care more about whether the reader can reach the missing record or disputed interpretation, even when following the link leaves them less impressed with Ze.

## Maintaining the route

Generating these views replaces repeated manual edits with producers and checks which need looking after themselves. A new kind of command data may require a renderer change, while moving a source file can break the link from its explanation. A false refusal also interrupts a correct edit until somebody establishes whether the edit or the check is wrong.

The dependency check catches an explanation we forgot to write, and a source-link check can expose a broken route into the code, but I still have to choose which sources belong together. A generator which joins the wrong ones will repeat the same mistake everywhere, however carefully each input is maintained. That choice deserves as much review as the prose on the page.

*Last updated: 11 September 2026. The history of this article is available in the [project's Git repository](https://github.com/ze-software/ze/commits/main/website/blog/posts/reference-from-the-system.md).*
