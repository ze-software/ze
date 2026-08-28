# CLI Root Namespace Grammar

> **Placement note.** This page describes a CLI-wide rule, not an IS-IS rule. It
> lives here because the IS-IS and OSPF offline decode CLIs are its two
> registered examples. The shape rule it extends is
> [`../cli/command-namespacing.md`](../cli/command-namespacing.md), "Token
> naming: hyphen for one name, space for a namespace".

The CLI grammar rules are enforced over the YANG command tree. The root
namespace, where handlers register directly rather than through YANG, was not
covered, so it drifted: four roots hyphenated two ideas that YANG already models
as two tokens, and one root was named after a single operator kind while
carrying a whole operator language.

The load-bearing deliverable was the **gate**, not the renames.

## Decision: a fourth feeder for the grammar gate

`grammar.CheckRootNamespace(roots, namespaces)` flags a root whose left
hyphen segment names a YANG verb or container. It is a **cross-surface** check,
not a reuse of the sibling-collision check.

The sibling check fires only when the colliding namespace is a sibling at the
same tree level. No `traffic` root existed to be `traffic-control`'s sibling,
which is exactly why the gate stayed green over four violations.

The gate now has four feeders: the static YANG check, plugin registration, the
runtime guard, and this root-namespace check. `./le cli-grammar` prints
the number of roots checked.

## Decision: the fixture test lives with the pure function

The gate itself is a `//go:build ignore package main` program, so a sibling test
can only run it as a subprocess and cannot unit-test its functions. A red-green
fixture test needs a pure function, so it lives in the `grammar` package.

Prove the gate **wiring** end to end separately, by temporarily reintroducing a
hyphenated root, watching it flag, and restoring.

## Decision: `pipe` names the operator language

`pipe` is the word the codebase already uses for the operator set, and it reads
correctly at a literal shell pipe. `format` is a **kind within** that set, so it
could never name the set: a command asking a "format" command for a filter is
incoherent.

## Decision: object roots with a closed-keyword sub-dispatch

`traffic`, `isis` and `ospf` are root handlers whose members (`control`,
`decode`) are dispatched by a closed keyword set inside the owning package. Two
alternatives were rejected: a YANG container, because these are offline tools
with no config surface; and promoting the operators to roots, because a bare
operator name would collide with a YANG verb.

<!-- source: internal/plugins/isis/cli/register.go -- the isis root namespace and the decode member -->
<!-- source: internal/plugins/ospf/cli/register.go -- the ospf root namespace and the decode member -->

Bare `ze isis` now enumerates its members. Completion, usage and the TUI needed
no change: they derive from the root listing.

## Decision: a server verb goes through the local-handler registry

A root named after a YANG verb is dead code, because the verb check returns
before root dispatch. Command execution consults local handlers before the YANG
tree, which is the mechanism `show version` already uses, so a verb-named command
registers there instead.

## Trap: a rename that frees a token can collide it in another binary

The test binary registers its own suite roots named `traffic`, `isis` and `ospf`
**and** imports the generated composition root. While the tools were
`traffic-control`, `isis-decode` and `ospf-decode` there was no clash. After the
rename the test binary panicked on a duplicate root command.

The fix marks the three tool CLIs `// codegen:skip`, so they reach the shipping
binaries only through their direct dispatch imports, consistent with the firewall,
iface and L2TP CLIs.

## Trap: a renamed token is not the same as a renamed identity

`isis-decode` and `ospf-decode` are unambiguously CLI command names and were
renamed everywhere, including about ten functional-test invocation lines that no
file list had recorded. `traffic-control` is **also** the config and component
identity, appearing in error messages, the YANG module and functional tests, and
that spelling stays. "No stragglers" therefore means no CLI-command references,
not zero matches.

## Trap: functional-test output matching is cumulative

Standard output and standard error assertions accumulate across all commands in
one file, so a negative assertion fails if any earlier step in the same file
emitted the string. Completion negatives live in their own isolated file.

## Coverage boundary

The feeder governs the **root** surface only. The local-handler surface is not
covered: local registration validates an empty path and a nil handler and never
checks command-name grammar. A hyphenated local command would not be flagged.
Extending the gate there is separate work.

Three further items are deliberately untouched. Root-level flags such as
`ze --plugins` are a different rule's question, and the feeder skips
leading-hyphen roots. The two `format` value vocabularies, the CLI output format
and the editor pipe operator, are both preserved and unreconciled. `ze pipe`
takes one shell-quoted pipe expression rather than repeated operators.
