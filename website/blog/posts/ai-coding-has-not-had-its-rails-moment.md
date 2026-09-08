---
title: AI coding has not had its Rails moment
date: 2026-08-10
author: Thomas Mangin
description: Rails gave developers and tools a shared project layout. AI coding needs a comparable convention for finding the decisions and evidence that each project must supply for itself.

deck: An agent can read repository documentation, but each project still has to teach it where authoritative decisions live and how they relate to the code.

image: assets/blog/ai-coding-has-not-had-its-rails-moment.svg
image-dark: assets/blog/ai-coding-has-not-had-its-rails-moment-dark.svg
image-alt: A recognised generated repository tree is compared with an arbitrary tree whose design, rules and tests sit apart; a shared convention joins project meaning to predictable locations.

---

An agent entering a repository needs to learn which decisions govern the code it is about to change. A project may have excellent documentation and still require a maintainer to explain where to begin, which pages are authoritative and how they relate to the tests.

I have been building that explanation into Ze, the network operating system I am working on. The difficulty is familiar from organising repositories for people, and Rails gives me a useful comparison: a convention that developers and tools can recognise before they understand the application. AI coding has some agreed entry points, but I do not yet see an equivalent shared convention for the project knowledge behind them.

*This article was co-authored with Claude and revised with OpenAI Codex. The argument and the conclusions are mine. Claude helped organise the material and draft the original text.*

## A layout I could carry between projects

At Exa in the early 2000s, our convention put each project's configuration in `etc/`, its working data in `data/`, reusable code in `lib/` and the rest in `src/`. It was not a one-to-one match with the Filesystem Hierarchy Standard, but changing `$ETC` and `$DATA` was enough to point a project at the right place on an installed system. During development, the repository acted as the installed root, so I could stay in the code-test loop without an installation step.

The [first ExaBGP commit, from September 2009](https://github.com/Exa-Networks/exabgp/commit/5490f7baf5981279e2360d88c735570bc9f72532), had `daemon`, `etc`, `lib` and `test` directories. Its commit message records that the `supervisor.py` test announced a route to a Cisco 7204 and kept the connection alive. The tree held the configuration and the code needed to try it.

That arrangement was familiar inside Exa. Rails made a shared layout part of the framework itself, and its [official philosophy](https://guides.rubyonrails.org/getting_started.html#rails-philosophy) calls the approach Convention over Configuration. Running `rails new` creates recognised places for application code and configuration, along with database migrations, libraries and tests.

A developer who knows Rails can enter an unfamiliar Rails application with some useful knowledge already, and the framework relies on the same conventions. The application still has to supply its own behaviour, but neither the person nor the tool has to begin by inventing where a model belongs.

Younger programmers may never have worked with Rails, but they will recognise the same benefit in React projects today. Familiar component conventions give them a starting point in a codebase they have never seen before.

I want that familiarity for the decisions around an AI-assisted change. The agent should have a predictable route from the source file to the design which governs it, and from that design to the evidence required when it changes.

## A harness cannot supply project decisions

An agent can search a repository and read its files, but finding a function does not establish which pattern the project expects it to follow. Ze requires registration for features which another project might put in a central switch statement. A general-purpose harness has no basis for choosing between those designs unless the project supplies the reason.

Repository documentation already answers some of this. An `AGENTS.md` or `CLAUDE.md` can explain the project and point to further reading, and those filenames are a useful beginning. They leave the organisation behind the entry point to each maintainer, so an agent still has to learn a different route to architectural decisions and checks in each repository.

The convention I am looking for would make those routes familiar without requiring every project to share Ze's architecture. The project would remain free to choose its implementation, while the harness could recognise where to find the rules for changing it. Better search helps recover a decision that has been written down; it cannot recover one which remains in my head.

## Existing attempts give us something to copy

Cloudflare described its own response in [How Cloudflare enforces engineering standards using AI](https://blog.cloudflare.com/engineering-standards-enforcement/) in August 2026. Its guidance already existed in formal documentation and repository files, as well as chat threads and engineers' accumulated knowledge. Engineers had trouble establishing whether what they found was current, authoritative or applicable to their situation.

Cloudflare organised its standards as RFCs with `MUST` and `SHOULD` requirements, using the meanings in [RFC 2119](https://www.rfc-editor.org/rfc/rfc2119.html). Statements receive stable identifiers, and agents can load a compact set of statements before requesting the full documents. Cloudflare also separates an approved standard from an enforced one, so publication does not immediately make every violation a reason to block a merge.

Ze has a task-oriented `ai/INDEX.md`, which points to rules and architecture documents, and source annotations which link the implementation back to its design. For example, the index sends a developer learning the modular core to `ai/patterns/registration.md`; it gives a separate route for implementing an RFC. [The repository is half the AI harness](../the-repository-is-the-ai-harness/) describes that navigation and the feedback provided during development.

Cloudflare's governed standards and Ze's repository rules serve different organisations, but both give an agent a route to guidance which people have already decided applies. Neither establishes an industry convention by itself. They are examples concrete enough for other projects to compare and adapt.

## A starter command can only supply the structure

Andrej Karpathy's [LLM Wiki gist](https://gist.github.com/karpathy/442a6bf555914893e9891c11519de94f) describes another recognisable arrangement: raw sources kept separately from cross-linked Markdown pages, an `index.md` for discovery, and an `AGENTS.md` or `CLAUDE.md` explaining the conventions. It is a proposal for a knowledge base rather than a software repository, but I recognised the use of ordinary files and links from what we had been doing in Ze.

A command could generate the equivalent starting structure for a repository. It could create an entry document and places for decisions, with an index and a convention for links between source and design. That would save each maintainer from inventing the arrangement, and give harness authors something consistent to support.

It could not decide which dependencies the project permits or why one implementation was rejected. Maintainers would still have to supply those decisions and choose the checks that make them enforceable. An empty architecture directory has no more meaning than an empty Rails model, and generated guidance would need the same review as generated code.

I am inclined to think a convention will spread through a project people can copy, though I do not know which one. A useful example would show both the starter structure and the project-specific reasoning added to it, so copying the files did not get mistaken for completing the job.

ExaBGP could run from its project folder because the layout let the program find its environment. I want an agent to find Ze's development environment with as little prior knowledge, including the designs which explain the code and the checks a change has to face. Building that for one repository is what I have spent much of this year doing.

The machinery supporting Ze's development is in place, and I have finished the update for Opus 5. I am going back to writing the router rather than spending the year on the tools used to build it.

*Last updated: 8 September 2026. The history of this article is available in the [project's Git repository](https://github.com/ze-software/ze/commits/main/website/blog/posts/ai-coding-has-not-had-its-rails-moment.md).*
