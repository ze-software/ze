---
title: AI coding has not had its Rails moment
date: 2026-08-10
author: Thomas Mangin
description: AI coding needs conventions that connect documentation and design decisions to the code they govern. Each project still has to build that structure for itself.

deck: Keeping a design document beside the code does not ensure an agent follows it. We need a structure that connects the two and keeps them consistent as the software changes.

image: assets/blog/ai-coding-has-not-had-its-rails-moment.svg
image-dark: assets/blog/ai-coding-has-not-had-its-rails-moment-dark.svg
image-alt: Comparison of a repository with predictable locations for designs, rules and tests, and one without a common organisation.

---

When I began building Ze, a network operating system, Claude could write Go, but a feature request did not give it all the design decisions I had made for the project. It could produce code that worked on its own but contradicted those decisions. Training in the language could not supply the reasons I had chosen one approach over another.

Writing those reasons down gave me something to refer to, but a document only helps a coding session if the agent finds it before choosing an implementation. The document must also identify which code the decision applies to, so the agent can check whether its changes follow that decision. Keeping a design somewhere in the repository leaves too much of this relationship to chance.

Requirements and designs are metadata about the software: they describe what we intend to build and the constraints on how we build it. In Ze, I have had to create conventions for connecting that metadata to the code, and instructions for using those connections during development. Other projects need the same relationship, yet we still lack a shared starting point comparable to what Rails gave web developers.

*This article was co-authored with Claude and revised with OpenAI Codex. The argument and the conclusions are mine. Claude helped organise the material and draft the original text.*

## Connect the decision to the implementation

In Ze, each plugin supplies its definition through a registry that the core reads, so a plugin can be added without changing the core. An agent asked to add a feature could instead put its name into a central switch. The feature might then produce the expected result, but every later plugin would require another core change. Judging only the feature's output would miss the reason for choosing registration in the first place.

The [registration pattern](https://github.com/ze-software/ze/blob/main/ai/patterns/registration.md) records that reason beside an example of the code we expect. Before an agent chooses how to add a plugin, the task index, [`ai/INDEX.md`](https://github.com/ze-software/ze/blob/main/ai/INDEX.md), directs it to this explanation. The example shows how to implement the feature, while the reason explains why a familiar alternative would be wrong for Ze.

A bug investigation can begin at the other end, with a function rather than a request for a new feature. The relevant design still applies, but the agent first has to discover which document describes it. Source files therefore identify their design documents, and documents cite the code they describe. Generated [code-to-documents](https://github.com/ze-software/ze/blob/main/ai/CODE-TO-DOCS.md) and [design-to-code](https://github.com/ze-software/ze/blob/main/ai/DOCS-TO-CODE.md) indexes make those connections available from either starting point.

With these links, an agent investigating a function can find the constraints its fix must preserve. If I decide to change a design, the reverse lookup identifies the implementation that must change with it. A document becomes useful during the task because we have defined how that task leads to it.

## Use the documents when authoring the code

Finding the registration document does not establish that a new plugin follows it. The agent has to use the design when choosing its implementation, and review has to compare the result with the stated constraints. Otherwise we have made the explanation easier to find without changing how the code is authored.

In Ze, `CLAUDE.md` directs the agent to rules which make reading and checking the design part of the task. Those rules also say where to record a new decision and require an existing explanation to change when the design changes. Leaving the old explanation in place would tell the next agent to follow a decision we had abandoned, while adding another document without replacing it would leave two conflicting accounts.

This gave me a way to make a correction useful beyond the session in which I made it. I could improve the explanation that later agents would be directed to read, then check their changes against it. I still have to judge whether the design is right, but I no longer have to include another account of it in every feature request.

[The repository is half the AI harness](../the-repository-is-the-ai-harness/) describes the rules and validation behind this process. They give the agent an objective and ways to detect when its implementation falls short. The links between documentation and code make those requirements available where the implementation is being written or reviewed.

## Why I think of Rails

Other projects have their own decisions to preserve, but much of the structure connecting those decisions to the code could be shared. Adding `CLAUDE.md` or `AGENTS.md` gives an author a place to put instructions. It leaves them to invent how documents relate to source files and how agents should use that relationship, even when another project has already solved the same organisational problem.

At Exa in the early 2000s, we used the same layout across projects: configuration in `etc/`, working data in `data/`, reusable code in `lib/` and the rest in `src/`. During development, the repository acted as the installed root. Changing `$ETC` and `$DATA` pointed the program at the installed locations, so we could use the same layout during development and after installation.

Because the configuration was already beside the code, another developer could try the program without first recreating its environment. The [first ExaBGP commit, from September 2009](https://github.com/Exa-Networks/exabgp/commit/5490f7baf5981279e2360d88c735570bc9f72532), contains `daemon`, `etc`, `lib` and `test` directories, and its commit message records a test announcing a route to a Cisco 7204. We could begin with a usable project whose organisation we understood, rather than explaining a new arrangement each time.

Rails brought this benefit to a much larger community through [Convention over Configuration](https://guides.rubyonrails.org/getting_started.html#rails-philosophy). A project created with `rails new` has recognised places for application code, configuration, database migrations, libraries and tests. Because Rails uses those conventions, they describe how the parts of an application fit together as well as where its files belong. A developer can carry that understanding from one Rails application to another.

An equivalent starting point for AI development would make the relationship between a design and its implementation just as familiar. The conventions would say how to find the relevant decision for a task and how to check that the resulting code follows it. They would also identify what must be updated when that decision changes. The architecture would remain the author's choice, but each project would no longer need its own invention for recording and applying that choice.

## Better tools still need a prepared repository

The tools around a model affect the result even before we consider project knowledge. Stencil's [February 2026 editing benchmark](https://stencil.so/blog/the-harness-problem) compared different edit formats across sixteen models and reported an average improvement of fifteen percentage points over its patch format, without changing the models. The authors were promoting their own format, so the figures deserve the usual caution. They nevertheless demonstrate how a model can complete more tasks when the tools make its proposed edits easier to apply.

Perfect editing would still leave the problem of choosing the right change. A general coding tool can search a repository and run its tests, but neither operation supplies an unrecorded reason for using registration. Model and harness developers can improve how an agent uses its tools; the project author still has to provide the decisions those tools will help implement. Improving one part does not remove the need to build the other.

This distinction gets lost when a frustrating experience with current tools becomes a verdict on AI development as a whole. Version control went through something similar. CVS made changing a project's structure awkward: [renaming a directory while preserving access to old versions](https://ftp.gnu.org/old-gnu/Manuals/cvs-1.9/html_chapter/cvs_15.html) required moving its files individually. In the early 2000s, engineers frustrated by such restrictions could conclude that version control itself got in the way, and some only reconsidered after Git and its community offered a different experience.

I also remember using the Internet at home over 64K between 1994 and 1996, after using a faster university connection. The home connection was painful, but I could already see why I wanted to use it. Faster access and services such as BitTorrent and YouTube later made the Internet useful to far more people. Judging its possibilities by my home connection would have confused an early limitation with a permanent one.

AI coding gives me that same combination of usefulness and frustration. I can build more with it, and I still spend time assembling conventions and repairing the development process that makes it usable. Better tools can remove some of that effort, while shared repository conventions can remove the repeated work of connecting a project's decisions to its code. In 1999 and 2000, working for an ISP felt like having a superpower; perhaps working for a frontier AI lab feels similar now, with enough resources to explore what everyone else is still trying to make usable.

## Other projects are connecting the same pieces

Parts of this structure are already being explored elsewhere. Cloudflare's [engineering standards system](https://blog.cloudflare.com/engineering-standards-enforcement/) addresses guidance spread across documents, repositories and conversations by giving standards stable identifiers that agents can use during reviews. A stable identifier lets a review refer back to the same requirement even when the discussion and the implementation are in different places.

Cloudflare records requirements using `MUST` and `SHOULD`, then supplies the relevant statements to agents before loading whole documents when more context is required. Ze had a different starting point: the protocol RFCs already contained many of its obligations. Giving those obligations identifiers and linking them to tests made it possible to inspect whether the implementation had been checked against them. [The proof is the expensive part](../the-proof-is-the-expensive-part/) explains why the link alone is insufficient and what evidence still has to be established.

Ze's task indexes and Cloudflare's structured standards address a similar difficulty at very different sizes. Both make an earlier decision available during implementation or review instead of depending on someone to repeat it. That is a useful indication that the structure could be shared, even though the decisions recorded inside it would belong to each project.

## A convention needs something people can copy

A shared convention is more likely to spread through a useful project than through agreement on the perfect arrangement. I expect a command which prepares a repository for an agent to play the part that `rails new` played for web development. An author could start with connected places for designs and code, then add the project's own requirements, without first inventing how an agent should find and apply them.

Andrej Karpathy's [LLM Wiki gist](https://gist.github.com/karpathy/442a6bf555914893e9891c11519de94f) shows how an arrangement can be made available to copy. It describes raw sources kept separately from cross-linked Markdown pages, with an index and instructions for using the collection. Ze was already using ordinary files, links and generated indexes for its rules and designs. Karpathy supplied a clear explanation of how the parts of a knowledge base belong together, giving readers a starting point they could adapt.

A recognisable reference project can make that arrangement familiar to a large audience, including people who would never have assembled it themselves. Its choices need to be useful and easy to copy to spread; they do not have to be the best possible choices. People copy the awkward parts along with the good ones, and familiarity can make both harder to replace.

For software development, I would extend that arrangement to the implementation. A design would identify the code it governs, and a task starting in that code would lead back to the design. Instructions for making a change would include checking the result against the recorded decision and keeping the explanation current. A project template could provide these conventions before its author had written the first feature.

Ze has a local version built through months of development. I would like to begin the next project by writing its first design decision and having an established place for it, with the connection to its future implementation already defined. That would leave more time to write the router.

*Last updated: 21 September 2026. The history of this article is available in the [project's Git repository](https://github.com/ze-software/ze/commits/main/website/blog/posts/ai-coding-has-not-had-its-rails-moment.md).*
