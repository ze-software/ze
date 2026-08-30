# CLI and Output

**When:** adding or changing any CLI command, flag, exit code, output format, error message, JSON envelope, or agent-facing contract
**Severity:** blocking
**Related:** evidence, performance, protocol, repo-maintenance, git-safety

## Directives

- **A command's response payload MUST be structured data satisfying `ResponseData` (`internal/component/plugin/types.go`), and MUST NOT be text a renderer already formatted.** `| json`, `| yaml` and `| table` are three renderings of ONE payload, so a handler that answers with finished text has picked the reader's format for them, and a row's state belongs in a field rather than a character glued to an identifier. Every command that produces output MUST route it through `ApplyPipes` and answer every GLOBAL operator on every surface the catalog (`internal/component/command/pipe_catalog.go`) makes it available on, deriving availability from that catalog rather than a hand-copied list; an operator the answer shape cannot support MUST be refused BY NAME, and the `always`, `with-rows`, `when-streaming` and `local-only` qualifiers MUST NOT be flattened into unconditional support.

- **Every JSON key MUST be lowercase kebab-case matching its YANG leaf or config tree key, and MUST NOT be camelCase or snake_case; every exported struct field that reaches JSON output MUST carry a `json:"kebab-name"` tag.** JSON MUST be built with `encoding/json` rather than string concatenation. The envelopes are `{"status":"ok","data":{...}}`, `{"status":"error","error":"msg"}` and `{"error":"description","parsed":false}`; raw hex is uppercase with no `0x`. The key set, the address families and the one exemption are `docs/architecture/api/json-format.md`.

## CLI Grammar: Keywords Before Values

**The first token after the noun MUST be a keyword from a closed set known at compile time, and a free-form value MUST NOT sit in an untyped positional slot.** A command addressing one member of a set MUST type the selector with `name`, `id`, `index`, `address`, `type`, `key`, or another schema-defined closed keyword, and `selector` MUST NOT be exposed as an operator keyword. Peer commands are the one exception: they address a peer positionally. The two token orders this produces are on `docs/architecture/cli/root-namespace-grammar.md`.

**The verb MUST be chosen by the command's effect on live state, never by how diagnostic it feels: does running it change what the router does, emits, or forwards?** No, it only reports: `show` for one snapshot or `monitor` for the same read streamed, however deep the introspection goes. Yes, as a normal operational action: the existing action verbs `request`, `clear`, `create`, `set`, `delete`, `update`, `cache`. Yes, as a deliberate diagnostic PERTURBATION (inject, force, corrupt, drop, toggle a fault mode): `debug`, double-gated by authz and a fail-closed runtime enablement. An operational `add`, `del` or `remove` MUST NOT be invented for an object that already lives in the config YANG tree: a change to a tree node stays in engine path form under `set`/`delete` and MUST NOT be mirrored as an RPC to make the grammar look regular.

**The `--flag` form belongs to the offline `cmd/ze/` Go flag tooling that reaches no daemon, and it MUST NOT reach the YANG layer or travel from a client to the daemon.** A flag baked into a YANG description is documentation lying about structure: it is invisible to completion and dispatch, and it couples the shared model to one front-end. A filter (address family, row limit, VRF, table) is grammar, so it MUST be modelled as a keyword-value pair, and every offline flag MUST be declared through the flag registry. The vendor namespacing behind family-as-filter is on `docs/architecture/cli/command-namespacing.md`.

**The R1-R9 ruleset (verb-first, token form, no `--flag`, namespace discipline, keyword-before-value, action-before-identifier, config-tree mutation stays in `set`/`delete`, string identifiers even when numeric, compound-versus-namespace split) is implemented once in `internal/component/command/grammar` over the canonical verb registry, seven feeders enforce it, and a grammar change MUST be run through `./le cli-grammar`.** Ze is unreleased, so an unreleased grammar MUST be replaced outright rather than deprecated; command ownership MUST NOT be reshuffled while grammar is being fixed, and a rename of a programmatic command path breaks the wire, so every programmatic sender MUST be found first. What each feeder checks is `docs/architecture/cli/root-namespace-grammar.md`.

## CLI Patterns

- **A command MUST write errors to stderr and MUST return an exit code; `os.Exit()` MUST NOT be called in a handler.** `-` means stdin or stdout, and a user-supplied path MUST be read or written through `internal/core/cliio` rather than a raw `os` call, which `./le dash-stdio check` enforces. Every error MUST say what failed, why, and what to do next, naming what the operator configured rather than the library, and a check that cannot run MUST return an error rather than a zero result. Every validation error surfaced to an agent MUST carry a stable diagnostic code, registered with its explanation. Every user-facing command MUST have tab-completion; `Hidden: true` on a `CommandDecl` is the exception, for an internal or diagnostic command that still works when typed in full.

## Agent Tooling Contract

**When a skill covers the task (`/ze-rfc`, `/ze-review`, `/ze-implement`, and the rest), it MUST be used instead of spawning a raw agent or improvising the workflow.** A skill encodes the conventions, gates and ordering a raw agent misses. Skill content is embedded in the binary, so it matches the version in hand.
