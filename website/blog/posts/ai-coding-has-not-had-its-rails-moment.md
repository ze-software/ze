---
title: AI coding has not had its Rails moment
date: 2026-08-10
author: Thomas Mangin
description: AI coding needs shared conventions for connecting requirements and design decisions to their implementation, and for keeping those relationships current.

deck: Shared conventions could give an agent a reliable route from a code change to the design it must follow, including how to keep both consistent.

image: assets/blog/ai-coding-has-not-had-its-rails-moment.svg
image-dark: assets/blog/ai-coding-has-not-had-its-rails-moment-dark.svg
image-alt: Comparison of a repository with predictable locations for designs, rules and tests, and one without a common organisation.

---

Claude knew Go when I began building Ze, a network operating system, but I still had to teach it how to find the project's design decisions. I would like the next project to inherit a familiar way of connecting those decisions to the code.

*This article was co-authored with Claude and revised with OpenAI Codex. The argument and the conclusions are mine. Claude helped organise the material and draft the original text.*

## Why I think of Rails

At Exa in the early 2000s, we used the same layout across projects: configuration in `etc/`, working data in `data/`, reusable code in `lib/` and the rest in `src/`. During development, the repository acted as the installed root. Changing `$ETC` and `$DATA` pointed the program at the installed locations, so we could use the same layout during development and after installation.

Because the configuration was already beside the code, another developer could try the program without first recreating its environment. The [first ExaBGP commit, from September 2009](https://github.com/Exa-Networks/exabgp/commit/5490f7baf5981279e2360d88c735570bc9f72532), contains `daemon`, `etc`, `lib` and `test` directories, and its commit message records a test announcing a route to a Cisco 7204. A familiar layout removed some of the work of getting started.

Rails brought this benefit to a much larger community through [Convention over Configuration](https://guides.rubyonrails.org/getting_started.html#rails-philosophy). A project created with `rails new` has recognised places for application code, configuration, database migrations, libraries and tests. Rails uses those conventions to connect the parts of an application, so they tell a developer more than where to put a file. Understanding one Rails application helps when entering the next.

Readers who have never used Rails may recognise the benefit in the component conventions shared by their team's React projects. React leaves more of the [surrounding project structure](https://react.dev/learn/creating-a-react-app) to frameworks and developers, but those local conventions still give a newcomer somewhere familiar to start.

Adding `AGENTS.md` or `CLAUDE.md` gives a project a place for instructions, but leaves its author to invent how designs relate to source files and how an agent should use that relationship. What I am waiting for is an arrangement recognisable from one project to the next, so that understanding carries across too.

A source file would lead an agent to its governing design, and the design would identify the code which implements it. Instructions for making a change would include checking the result against that decision and updating the explanation when the decision changes. Each author would still choose the architecture, within a familiar structure for recording and applying those choices. Useful pieces of this already exist; the missing step is making the whole arrangement familiar across projects.

## Connect the decision to the implementation

In Ze, Claude could produce code that worked on its own but contradicted decisions I had already made. Knowing Go could not tell it why I had chosen one design over another. A feature request needed a route to those reasons before the agent chose an implementation.

Requirements and designs are metadata about the software: they describe what we intend to build and the constraints on how we build it. Writing those reasons down gives us an explanation to consult, but the agent also has to find it and establish which code the decision governs. Otherwise it can read a perfectly good document and still miss its relevance to the change it is making.

For example, the [registration pattern](https://github.com/ze-software/ze/blob/main/ai/patterns/registration.md) explains why plugins must remain independent of the core and shows the code we expect. Ze's task index, [`ai/INDEX.md`](https://github.com/ze-software/ze/blob/main/ai/INDEX.md), gives an agent adding a plugin a route to that explanation. The index connects the requested task to the design that governs its implementation.

A bug investigation can begin at the other end, with a function rather than a request for a new feature. The relevant design still applies, but the agent first has to discover which document describes it. Source files therefore identify their design documents, and documents cite the code they describe. Generated [code-to-documents](https://github.com/ze-software/ze/blob/main/ai/CODE-TO-DOCS.md) and [design-to-code](https://github.com/ze-software/ze/blob/main/ai/DOCS-TO-CODE.md) indexes make those connections available from either starting point.

With these links, an agent investigating a function can find the constraints its fix must preserve. If I decide to change a design, the lookup in the other direction identifies source files which refer to it. That gives us a place to begin checking the effect of the decision, though the links cannot tell us whether every affected file has been identified.

## Keep the explanation and the code together

Finding the registration document does not establish that a new plugin follows it. The agent has to use the design when choosing its implementation, and the reviewer has to compare the result with the stated constraints. The connections become useful because the development process requires the author and reviewer to follow them.

In Ze, the [root agent instructions](https://github.com/ze-software/ze/blob/main/AGENTS.md) direct a session to the relevant rules and design documents. [`ai/INSTRUCTIONS.md`](https://github.com/ze-software/ze/blob/main/ai/INSTRUCTIONS.md) is the source for the copies supplied to different AI tools, so changing tools does not require maintaining another account of the project. The rules require documentation to change alongside code when a change would make its explanation wrong.

That requirement matters when I revise a design. Leaving its old explanation in place would tell the next agent to follow a decision we had abandoned, while adding another account without resolving the old one would leave conflicting instructions. Links make the relationship discoverable; keeping the explanation current makes it useful to the next session.

As the documentation became more useful, the generated code followed the intended design more closely. A correction that once had to be repeated in conversation could improve the explanation the next session was directed to read. I still have to judge whether the design is right and whether an implementation follows it, but each session can build on the reasoning already recorded. Shared conventions could give other projects a starting point for establishing the same relationship.

[The repository is half the AI harness](../the-repository-is-the-ai-harness/) examines how rules and validation help an agent recognise work below the required standard. The organisational problem here comes before that check: the agent and reviewer must be able to find the decision that sets the standard.

## Better tools still need a prepared repository

The tools around a model affect the result even before we consider project knowledge. Stencil's [February 2026 editing benchmark](https://stencil.so/blog/the-harness-problem) compared different edit formats across sixteen models and reported an average improvement of fifteen percentage points over its patch format, without changing the models. Stencil promotes its own format, so the figures deserve the usual caution, but the result illustrates how much can depend on the tools used to apply an edit.

Perfect editing would still leave the problem of choosing the right change. A general coding tool can search a repository and run its tests, but neither operation supplies an unrecorded reason for using registration. Model and harness developers can improve how an agent uses its tools. Project authors still have to provide their decisions, although a shared structure could save each of us from inventing how to make them available.

## Other projects are connecting the same pieces

Parts of this structure are already being explored elsewhere. Cloudflare's [August 2026 account of its engineering standards system](https://blog.cloudflare.com/engineering-standards-enforcement/) describes guidance spread across documents, repositories and conversations. Giving standards stable identifiers lets reviews refer back to the same requirement even when the discussion and the implementation are in different places.

Cloudflare records requirements using `MUST` and `SHOULD`, then supplies the relevant statements to agents before loading whole documents when more context is required. Ze had a different starting point: protocol RFCs already described many of the behaviours its code must implement. We gave those obligations identifiers and linked them to tests, so a reviewer can move from a requirement to the code that checks it. [The proof is the expensive part](../the-proof-is-the-expensive-part/) explains why that link still needs evidence behind it.

Ze's task indexes and Cloudflare's structured standards address a similar difficulty at very different sizes. Both make an earlier decision available during implementation or review instead of depending on someone to repeat it. The decisions belong to each project, but there is enough common work in recording and finding them to make shared conventions useful.

## A convention needs something people can copy

A shared convention is more likely to spread through a useful project than through agreement on the perfect arrangement. I expect a command which prepares a repository for an agent to play the part that `rails new` played for web development. Its value would come from providing the relationships and the instructions for using them, as well as creating directories.

Andrej Karpathy's [LLM Wiki gist](https://gist.github.com/karpathy/442a6bf555914893e9891c11519de94f) shows how an arrangement can be made available to copy. It describes raw sources kept separately from cross-linked Markdown pages, with an index and instructions for using the collection. Ze was already using ordinary files, links and generated indexes for its rules and designs. Karpathy supplied a clear explanation of how the parts of a knowledge base belong together, giving readers a starting point they could adapt.

A recognisable reference project can make that arrangement familiar to a large audience, including people who would never have assembled it themselves. Its choices need to be useful and easy to copy to spread; they do not have to be the best possible choices. People copy the awkward parts along with the good ones, and familiarity can make both harder to replace.

I have seen useful technologies become less awkward before. CVS made [renaming a directory while preserving old versions](https://ftp.gnu.org/old-gnu/Manuals/cvs-1.9/html_chapter/cvs_15.html) difficult, and some engineers only reconsidered version control after Git. At home between 1994 and 1996, I used the Internet over 64K after experiencing a faster university connection. Faster access and services such as BitTorrent and YouTube later made it useful to far more people.

AI coding gives me a similar combination of usefulness and frustration: I can build more with it, while still assembling the conventions that make it usable. In 1999 and 2000, working for an ISP felt like having a superpower; perhaps working for a frontier AI lab feels similar now, with enough resources to explore what everyone else is still trying to make usable. I would like useful repository arrangements to become familiar enough that others can adopt them without repeating all that preparation.

For software development, I would extend the knowledge-base arrangement to the implementation. A template could provide the design links and instructions described earlier before an author had written the first feature, leaving the project's architecture for them to decide.

Ze has a local version built through months of development. I would have liked to start with a familiar place for its first design decision, with the connection to its future implementation already defined. That would have left more time to write the router.

*Originally written: 10 August 2026. Last updated: 13 November 2026. The history of this article is available in the [project's Git repository](https://github.com/ze-software/ze/commits/main/website/blog/posts/ai-coding-has-not-had-its-rails-moment.md).*
