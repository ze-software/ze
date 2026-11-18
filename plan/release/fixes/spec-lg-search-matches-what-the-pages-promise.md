# Spec: lg-search-matches-what-the-pages-promise

| Field | Value |
|-------|-------|
| Status | design |
| Scope | plugin |
| Priority | P2 |
| Release reason | The looking glass ships, and two of its searches do not do what its own help, guide and API contract say: a host address finds no covering route, and a multi-AS path search is refused by the engine. Incorrect shipped behavior on a public surface |
| RFC gaps | - |
| Depends | - |
| Phase | - |
| Handoff | - |
| Updated | 2026-11-18 |

Recovery after compaction: `.claude/rules/post-compaction.md`.

The design below is a proposal. It was written without an approval gate and
waits for the owner's review before the status moves to `ready`. The companion
spec for new capabilities is
`plan/post-release/features/spec-lg-peer-history-origin-search-and-named-nodes.md`.

## Task

A looking glass on a route collector answers three questions: which sessions
are up, who announces a prefix or the prefix covering an address, and which
routes cross a given AS. Ze's looking glass answers the first, and it answers
the second only for an exact prefix. Two searches are broken against what the
product itself documents.

| # | Symptom | What the pages say | What happens |
|---|---------|--------------------|--------------|
| 1 | A host address typed in the prefix field finds no covering route | The help page says a host address matches "the most-specific covering prefix". The feature page lists "Prefix and IP containment search". The API table calls `routes/prefix` an "exact or containing prefix lookup" | The RIB compares text. A route matches when its prefix string starts with what was typed, so `10.1.2.3` finds `10.1.2.3/32` and never `10.0.0.0/8` |
| 2 | An AS path search with more than one AS is refused | The help page says "space-separated ASNs matched anywhere in the path". The page validator accepts spaces, `^`, `$`, `\|`, brackets and repetition characters | The engine reads one token as the pattern: comma-separated AS numbers with an optional leading `^`. The second AS arrives as a separate token and the engine answers `unknown keyword`. Every character beyond digits, dots, commas and a leading `^` is refused as an invalid AS number |

The goal: both searches return what the pages promise, through the web page and
through the birdwatcher API, and each page states exactly what the search does.

## Required Reading

### Architecture Docs
- [ ] `ai/doors/system.md` - the design every change respects
  → Constraint: a command is declared once by its owner and reaches every interface through the dispatcher. The looking glass adds no command
  → Constraint: a new filter keyword belongs to the plugin that owns the pipeline, `bgp-rib`, and must work when that plugin runs as a forked process
- [ ] `ai/doors/api-surfaces.md` - the looking glass is a thin adapter
  → Constraint: the looking glass never imports a RIB or peer package. It asks the dispatcher
  → Constraint: `docs/architecture/api/birdwatcher-compat.md` is normative for the API. A change to what `routes/prefix` answers is a change to that page, in the same work
  → Constraint: recorded trap: the looking glass reshapes engine data and gets it wrong, and its test fake answered shapes the engine never produces. A search change is proven against a real `bgp-rib` over HTTP, not only against the fake
- [ ] `ai/doors/bgp-plugins.md` - `bgp-rib` owns the query pipeline
  → Constraint: a plugin declares its commands and pipes twice, and `./le plugin declarations check` compares them. A new pipeline keyword is checked against both declarations
  → Constraint: no plain call from the looking glass into the plugin. The text command is the boundary
- [ ] `docs/architecture/api/birdwatcher-compat.md` - the API contract
  → Constraint: `routes/prefix` and `routes/search` both dispatch `show bgp rib received prefix <prefix>` (the endpoint table). Both change together
- [ ] `docs/architecture/plugin/rib-storage-design.md` - how routes are stored
  → Constraint: families keyed by an address prefix sit in a prefix trie per peer (`FamilyRIB.direct`, `FamilyRIB.multi`). Other families are keyed by wire bytes and have no notion of covering

**Key insights:**
- The engine's `prefix` filter is a text comparison by design, and its partial form is documented to operators (`show bgp rib sent prefix 10.0 count` in the AI help). Changing its meaning breaks a documented use. Covering is a new question and gets its own keyword.
- The AS path grammar has one owner, the engine. The page must send that grammar and describe that grammar.
- Tests cover only a single-AS search, which is the one form where the page and the engine agree.

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `internal/component/bgp/plugins/rib/rib_pipeline.go` - the query pipeline. `prefixFilter.Next` keeps a row when its prefix string starts with the pattern. `matchASPath` reads comma-separated AS numbers, optional leading `^`, and matches a contiguous run anywhere or at the start. `parsePipelineArgs` takes one token as a filter value and answers `unknown keyword` for a token it does not know. `filterKeywords` lists `path`, `aspath`, `prefix`, `community`, `family`, `match`, `first`, `last`
- [ ] `internal/component/lg/handler_ui.go` - `handleUISearch` builds `show bgp rib received` plus `prefix`, `path`, `community`, `family` from the form, unchanged. `isValidASPathPattern` accepts digits, space, dot, `^`, `$`, `|`, brackets, `*`, `+`, `?`, `-`, `_`. `handleUIRouteDetail` asks for one exact prefix
- [ ] `internal/component/lg/handler_api.go` - `routes/prefix` and `routes/search` dispatch `show bgp rib received prefix <prefix>`. `isValidPrefix` accepts hex digits, dots, colons and a slash
- [ ] `internal/component/lg/handler_graph.go` - the graph asks `show bgp rib received prefix <prefix>` for the same input the search took
- [ ] `internal/component/lg/help.templ` - the help text quoted in the Task table
- [ ] `internal/component/lg/ribfake_test.go` - the test fake accepts the engine's filter keywords by name (`fakeRIBFilters`)
- [ ] `internal/component/lg/server_test.go` - the only AS path search tested is `aspath=65001`
- [ ] `internal/component/bgp/plugins/rib/storage/familyrib.go` - address-keyed families are held in a prefix trie
- [ ] `internal/plugins/help/help_ai.go` - documents the partial text form `show bgp rib sent prefix 10.0 count`

**Behavior to preserve:**
- `prefix <text>` keeps its text meaning on every scope, including the partial form
- An exact prefix with a mask returns the same rows as today, through the page, the API and the graph
- A single-AS path search returns the same rows as today
- `^` anchoring and the comma form keep working from the CLI and every other surface
- The search result cap, the validation errors for other fields, and the 502 and 503 answers when the engine cannot answer

**Behavior to change:**
- A host address in the prefix field, on the page, the graph and the API, returns every route whose prefix contains the address
- A multi-AS path search from the page reaches the engine in the engine's grammar
- The page accepts only what the engine can match, and says why when it refuses
- The help text, the guide, the feature page and the API contract describe the two searches as built

## Existing Mechanisms (MANDATORY)

| Mechanism | Where (file + symbol, or page + section) | What it does today | This spec (extends, replaces, or does not apply, with the reason) |
|-----------|------------------------------------------|--------------------|-------------------------------------------------------------------|
| The RIB query pipeline | `internal/component/bgp/plugins/rib/rib_pipeline.go`, `parsePipelineArgs`, `filterKeywords`, the stage constructors | Parses scope, selector, filters and one terminal from the command line, then chains iterators | Extends: one new filter keyword for covering lookup, declared where the others are |
| The text `prefix` filter | Same file, `prefixFilter` | Keeps rows whose prefix string starts with the pattern | Does not apply: its text meaning is documented and preserved. Covering is a different question |
| The per-peer prefix trie | `internal/component/bgp/plugins/rib/storage/familyrib.go`, `FamilyRIB.direct` and `FamilyRIB.multi` | Stores address-keyed routes for exact lookup and iteration | Extends: the covering lookup reads the entries on the path from the root to the address, so its cost follows the prefix length, not the table size |
| AS path matching | `rib_pipeline.go`, `matchASPath` and `validatePathPattern` | One grammar: comma-separated AS numbers, optional `^` | Extends nothing here. The page adopts this grammar. New anchors are the companion spec's work |
| AS number spellings | `asn.Parse`, used by both functions above | Reads the three RFC 5396 spellings | Does not apply: unchanged, and the page validator must accept what it reads |
| The looking glass command dispatch | `internal/component/lg/server.go`, `ask` | Sends a command line to the dispatcher and classifies a failure | Extends: the same route carries the new keyword. No second path |
| Input validation at the page | `internal/component/lg/handler_ui.go`, `isValidASPathPattern`; `handler_api.go`, `isValidPrefix` | Character allowlists | Replaces `isValidASPathPattern` with a check for the engine's grammar. `isValidPrefix` is kept and joined by a parse that tells an address from a prefix |
| The LG tests against a real `bgp-rib` | `test/plugin/lg-ui-pages.ci` and its siblings | Drive the daemon over HTTP | Extends: two new functional tests in the same suite |

## Data Flow (MANDATORY - see `ai/rules/architecture.md`)

### Entry Point
- A browser posts the search form to `/lg/search`, or asks `/lg/graph`, or a client calls `routes/prefix` or `routes/search` on the birdwatcher API
- Format at entry: form fields or a query string

### Transformation Path
1. The looking glass validates each field
2. It decides, from the prefix field alone, whether the value is a prefix with a mask or a bare address
3. It builds one command line for `bgp-rib`: the existing `prefix` keyword for a prefix with a mask, the new covering keyword for a bare address. An AS path typed with spaces becomes the engine's comma form
4. The dispatcher delivers the line to `bgp-rib`, in process or over RPC
5. `bgp-rib` parses the line. For the covering keyword the received source reads, in each peer's trie for the address's family, the entries on the path to the address
6. Rows return as the existing JSON, most specific prefix first within each peer
7. The looking glass renders the rows as it does today

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Browser or API client to looking glass | HTTP form and query string | No |
| Looking glass to `bgp-rib` | One text command through the dispatcher | No |
| `bgp-rib` to its storage | In-plugin trie read under the plugin's existing locking | No |

### Changed-Wire Census

**Search evidence:** searched `internal`, `cmd`, `pkg` and `test`, Go, `.ci`, shell, Python and templ files, for command lines that send `prefix` or `path` to `show bgp rib`, and for readers of the filter keyword set (`filterKeywords`, `filterASPath`). Positive control: the search finds `internal/component/lg/handler_api.go` line 556 and `internal/component/lg/handler_graph.go` line 67, both known writers. No shell or Python writer exists. The wire is the text command line; the JSON rows are unchanged.

| Wire | Role (writer / reader) | Carrier (file + producer/consumer symbol or script entry) | Source evidence | Migration or preservation disposition |
|------|------------------------|-----------------------------------------------------------|-----------------|----------------------------------------|
| `show bgp rib received` filter line | writer | `internal/component/lg/handler_ui.go`, `handleUISearch` | Builds `prefix`, `path`, `community`, `family` from the form | Change: bare address sends the covering keyword, AS path sent in comma form |
| Same | writer | `internal/component/lg/handler_api.go`, the `routes/prefix` and `routes/search` handler | Sends `show bgp rib received prefix <prefix>` | Change: bare address sends the covering keyword |
| Same | writer | `internal/component/lg/handler_graph.go`, the graph handler | Sends `show bgp rib received prefix <prefix>` | Change: same rule, so the graph draws what the table lists |
| Same | writer | `internal/component/lg/handler_ui.go`, `handleUIRouteDetail` | Sends one exact prefix taken from a result row | Preserve: a result row always carries a mask |
| Same | writer | `internal/test/fixture/plugin_fixture_05.go` | Sends `show bgp rib received prefix 10.0.0.0/24 count` | Preserve: text `prefix` unchanged |
| Same | writer | `internal/plugins/help/help_ai.go` | Documents `show bgp rib sent prefix 10.0 count` | Preserve: this documented partial form is why `prefix` keeps its meaning. Gains a line for the covering keyword |
| Same | writer | An operator at the CLI, or any API client | The command grammar is public | Preserve every existing form. The new keyword is additive |
| Same | reader | `internal/component/bgp/plugins/rib/rib_pipeline.go`, `parsePipelineArgs` | Looks each token up in `filterKeywords` and the terminal set | Change: learns the covering keyword and validates its value as an address |
| Same | reader | `internal/component/bgp/plugins/rib/rib_pipeline_best.go`, the best-path parser | Reads `filterKeywords` at line 488 | Change or refuse: the covering keyword on the `best` scope either works or is refused with a message naming the scopes that take it. The implementation reads this parser before choosing |
| Same | reader | `internal/component/lg/ribfake_test.go`, `fakeRIBFilters` | The test fake's own keyword list | Change: follows the engine, per the fixture rule in `birdwatcher-compat.md` section 9 |

### Integration Points
- `parsePipelineArgs` and the stage builder in `rib_pipeline.go` - the keyword joins the existing table
- The received source in the same package - gains the trie read for a covering query
- `LGServer.ask` - unchanged carrier

### Architectural Verification
| Check | Holds? | Evidence |
|-------|--------|----------|
| No bypassed layers (data flows through the intended path) | Yes | The looking glass still reaches the RIB only through the dispatcher |
| No unintended coupling (components stay isolated) | Yes | No new import in `internal/component/lg`. The address test uses the standard library |
| No duplicated functionality (extends existing, does not recreate) | Yes | One new pipeline keyword beside the existing ones. The AS path grammar keeps one owner |
| Zero-copy preserved where applicable (refs, not copies) | Yes | The covering read returns the same row items the iteration returns today |
| Registration over hardcoding, outbound: new commands, views, families, and handlers register, and the core discovers them. No per-feature field, switch case, or factory is added to a core/shared package (`ai/rules/plugins.md`) | Yes | The keyword lives in `bgp-rib`'s own pipeline table. Nothing outside the plugin names it except its callers |
| Registration over hardcoding, inbound: no existing switch, seed map, validator, parser, runner, help string, or completion table has to learn this feature's name. Evidence names every list that was searched for the names this feature introduces, and the registry each one now derives from (`ai/rules/principles.md`) | No | Three lists inside the plugin transcribe the keyword set today (`rib_pipeline.go` says so at the keyword constants), and the plugin's two command declarations may list pipes. The implementation adds the keyword to each and proves it with `./le plugin declarations check`. Collapsing the three lists is outside this spec |

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis (file/doc/user statement) | If wrong | Validated by | Status |
|----|-----------|--------------------------------|----------|--------------|--------|
| A-1 | The trie type behind `FamilyRIB.direct` can enumerate the stored prefixes that contain an address | `familyrib.go` names it a BART; such a trie supports that walk | The walk is added to the store type first, in this spec | Reading the store type's methods before phase 2 | unvalidated |
| A-2 | A value with a mask and a bare address can be told apart by parsing alone | `isValidPrefix` already limits the characters | An ambiguous form needs a rule of its own | Unit test over IPv4 and IPv6 inputs, with and without a mask | unvalidated |
| A-3 | The dispatcher passes the command line to `bgp-rib` split on spaces, with no quoting the page could use to send a spaced AS path as one value | `parsePipelineArgs` takes `args[i]` as the whole value | The page could send the spaced form unchanged and the engine would learn it | Reading the dispatcher's tokenizer | unvalidated |
| A-4 | No caller relies on the page accepting `$`, `\|`, brackets or repetition characters | The engine refuses every one of them today | A user who typed them got an error before and gets a clearer one now | The search in the census found no such writer | unvalidated |
| A-5 | The companion spec, not this one, adds the end anchor | The split of fix from feature in `plan/README.md` | If the owner wants origin search in the release, that row moves here | Owner review of both specs | unvalidated |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | The covering lookup scans every route of every peer, on a public endpoint | A lookup on a full table takes seconds | AC-5 requires the trie read. The existing result cap stays |
| R-2 | The page normalizes the AS path differently from the engine's reader | A search that the CLI answers returns nothing from the page | One functional test drives a real `bgp-rib` with a two-AS search. The page-side check accepts exactly what `validatePathPattern` accepts |
| R-3 | A family not keyed by address is asked a covering question | An empty answer read as "no route" | The answer covers address-keyed families only and the page says so in its help |
| R-4 | The graph and the table disagree for a bare address | The graph is empty under a full table | The graph handler follows the same rule, AC-4 |
| R-5 | The test fake accepts the new keyword without the engine's meaning | Unit tests green, the daemon wrong | The functional tests are the proof. The fake follows the engine |
| R-6 | A covering lookup under ADD-PATH misses all but one path | Fewer rows than paths | A unit test stores two paths for one covering prefix and expects both |

## Blast Radius

| Question | Answer |
|----------|--------|
| What breaks if this is wrong? | Looking glass searches return wrong or empty results. No session, route selection or forwarding is touched |
| How is it reverted? | A commit revert. No config or stored state changes |
| Who else touches this path? | `spec-show-bgp-rib-best-row-top-level-fields.md` and `spec-bgp-per-peer-received-counter.md` in `plan/release/fixes/` work the same pipeline and peer summary. The companion spec changes the same page handlers |

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| `POST /lg/search` with a bare address | → | covering read in `bgp-rib` | `test/plugin/lg-covering-lookup.ci` |
| `GET /api/looking-glass/routes/prefix` with a bare address | → | covering read in `bgp-rib` | `test/plugin/lg-covering-lookup.ci` |
| `POST /lg/search` with two AS numbers | → | `matchASPath` in `bgp-rib` | `test/plugin/lg-aspath-search.ci` |
| `show bgp rib received` with the covering keyword, from the CLI | → | `parsePipelineArgs` | `TestPipelineCoveringKeyword` |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior | Permitted Survivors and Why |
|-------|-------------------|-------------------|-----------------------------|
| AC-1 | Two peers hold `10.0.0.0/8`, one also holds `10.1.0.0/16` and `0.0.0.0/0`. The page is asked for `10.1.2.3` | The result lists all four routes with their peers, `10.1.0.0/16` first and `0.0.0.0/0` last | `192.0.2.0/24` held by the same peers stays out of the result, because it does not contain the address |
| AC-2 | Same state. The page is asked for `10.0.0.0/8` | The result lists the two `10.0.0.0/8` routes, as today | `10.1.0.0/16` stays out: a prefix with a mask is an exact question |
| AC-3 | Same state. `routes/prefix?prefix=10.1.2.3` and `routes/search?prefix=10.1.2.3` | Each answers the four routes in the birdwatcher shape, `primary` set as today | Routes that do not contain the address stay out |
| AC-4 | Same state. The graph is asked for `10.1.2.3` | It draws the AS paths of the same four routes the table lists | None: the graph and the table read one answer |
| AC-5 | A peer holds a full table. A covering lookup runs | The plugin reads only the trie entries on the path to the address, proven by a test that counts entries visited | Reading at most one entry per prefix length is permitted, because that is the path |
| AC-6 | An IPv6 address is asked | The result lists the covering IPv6 routes, most specific first | IPv4 routes stay out, because the families differ |
| AC-7 | A peer negotiated ADD-PATH and holds two paths for a covering prefix | Both paths are listed | None |
| AC-8 | The page is asked for AS path `65001 65002` | The result lists the routes whose path holds 65001 directly followed by 65002 | A route holding 65002 then 65001 stays out, because the run is ordered |
| AC-9 | The page is asked for AS path `^65001` | The result lists routes whose path starts with 65001 | A route with 65001 later in its path stays out, because the pattern is anchored |
| AC-10 | The page is asked for AS path `65001$` or `(65001\|65002)` | The page answers an error that names the accepted form: AS numbers separated by spaces, with an optional leading `^` | A plain AS number search still succeeds, because only unsupported characters are refused |
| AC-11 | The CLI sends `show bgp rib sent prefix 10.0 count` | The count is the same as before this change | None: the text filter is preserved whole |
| AC-12 | The covering keyword is sent with a value that is not an address | The engine refuses with a message naming the value and showing a valid form | A valid address still answers, because only the value is at fault |
| AC-13 | The help page, `docs/guide/looking-glass.md`, `docs/features/looking-glass.md` and `docs/architecture/api/birdwatcher-compat.md` | Each states that a bare address returns every covering route, most specific first, and that an AS path is a run of AS numbers with an optional leading `^` | Text on other subjects in those pages is unchanged |

## End-to-End User Stories

| # | User does | Path through system | Test proving it works |
|---|-----------|--------------------|-----------------------|
| 1 | Types an address in the looking glass and reads who announces the networks containing it | form, `handleUISearch`, dispatcher, `bgp-rib` covering read, result table | `test/plugin/lg-covering-lookup.ci` |
| 2 | Asks the birdwatcher API for an address from a script | `routes/prefix`, dispatcher, `bgp-rib`, birdwatcher transform | `test/plugin/lg-covering-lookup.ci` |
| 3 | Types two AS numbers and reads the routes crossing that adjacency | form, `handleUISearch`, dispatcher, `matchASPath`, result table | `test/plugin/lg-aspath-search.ci` |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestPipelineCoveringKeyword` | `internal/component/bgp/plugins/rib/rib_pipeline_test.go` | The keyword parses, and a bad value is refused with its message | |
| `TestCoveringReadsOnlyThePath` | `internal/component/bgp/plugins/rib/rib_pipeline_test.go` | Entries visited follow the prefix length, not the table size | |
| `TestCoveringOrdersMostSpecificFirst` | `internal/component/bgp/plugins/rib/rib_pipeline_test.go` | Row order within a peer | |
| `TestCoveringListsEveryAddPathPath` | `internal/component/bgp/plugins/rib/rib_pipeline_test.go` | Both paths of an ADD-PATH prefix | |
| `TestPrefixTextFilterUnchanged` | `internal/component/bgp/plugins/rib/rib_pipeline_test.go` | The partial text form still matches | |
| `TestSearchSendsCoveringForBareAddress` | `internal/component/lg/handler_ui_test.go` | The command line built for an address and for a prefix with a mask | |
| `TestSearchSendsEnginePathGrammar` | `internal/component/lg/handler_ui_test.go` | Spaces become the comma form, `^` is kept | |
| `TestSearchRefusesUnsupportedPathCharacters` | `internal/component/lg/handler_ui_test.go` | The error text names the accepted form | |
| `TestAPIPrefixSendsCoveringForBareAddress` | `internal/component/lg/handler_api_test.go` | Both API endpoints follow the rule | |
| `TestGraphSendsCoveringForBareAddress` | `internal/component/lg/graph_test.go` | The graph follows the rule | |

### Boundary Tests (numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| IPv4 mask in the prefix field | 0-32 | 32 | N/A | 33 |
| IPv6 mask in the prefix field | 0-128 | 128 | N/A | 129 |
| AS number in a path search | 0-4294967295 | 4294967295 | N/A | 4294967296 |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `lg-covering-lookup` | `test/plugin/lg-covering-lookup.ci` | Two peers announce nested prefixes. The page and the API, asked for an address, list every covering route, most specific first | |
| `lg-aspath-search` | `test/plugin/lg-aspath-search.ci` | The page, asked for two AS numbers, lists the routes that cross that adjacency and leaves out the reverse order | |

## Files to Modify
- `internal/component/bgp/plugins/rib/rib_pipeline.go` - the covering keyword, its validation, and the received source's trie read
- `internal/component/bgp/plugins/rib/rib_pipeline_best.go` - the covering keyword on the `best` scope, or its refusal
- `internal/component/bgp/plugins/rib/storage/familyrib.go` - the read of covering entries, if the store does not offer it
- `internal/component/bgp/plugins/rib/register.go` - the keyword in the declared pipes, if the declarations list them
- `internal/component/lg/handler_ui.go` - address or prefix decision, AS path grammar, the validator
- `internal/component/lg/handler_api.go` - the same decision for `routes/prefix` and `routes/search`
- `internal/component/lg/handler_graph.go` - the same decision for the graph
- `internal/component/lg/help.templ` - the search help text
- `internal/component/lg/ribfake_test.go` - the fake follows the engine
- `internal/plugins/help/help_ai.go` - one example line for the covering keyword
- `docs/guide/looking-glass.md`, `docs/features/looking-glass.md`, `docs/guide/looking-glass-howto.md` - what the searches do
- `docs/architecture/api/birdwatcher-compat.md` - the endpoint table and the dispatch table
- `docs/guide/cli.md` - the pipeline keywords, where the others are documented

## Files to Create
- `test/plugin/lg-covering-lookup.ci` - covering lookup through the page and the API
- `test/plugin/lg-aspath-search.ci` - multi-AS search through the page

### Integration Checklist
| Integration Point | Applies? | File / reason |
|-------------------|----------|---------------|
| YANG schema (new RPCs/config) | No | No new command and no config. A keyword of an existing command |
| YANG validation constraints | N-A | No leaf added |
| YANG custom validators | N-A | No leaf added |
| CLI commands/flags | Yes | `internal/component/bgp/plugins/rib/rib_pipeline.go`: one filter keyword on `show bgp rib` |
| CLI grammar (keyword before value) | Yes | The keyword precedes its address, as every filter does |
| Editor autocomplete | Yes | Wherever the pipeline keywords are offered for completion, found by searching for the existing keyword names during phase 1 |
| Functional test for new RPC/API | Yes | `test/plugin/lg-covering-lookup.ci`, `test/plugin/lg-aspath-search.ci` |
| Pipe completeness | Yes | The keyword is a pipeline filter and composes with `count`, `first`, `last` and `json` as the others do |
| Env var registration | N-A | No environment leaf |
| Doctor check for runtime dependencies | N-A | No new file, socket, port or kernel dependency |
| Doctor code per problem class | N-A | The only new refusals are answers to a query the user just typed. No log line and no persistent state |
| Prometheus counters/metrics | No | No new observable state |
| BGP family surface (new SAFI / capability / attribute) | N-A | No family, capability or attribute |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature, or a feature's scope, evidence or level changed? | Yes | `features/looking-glass.md`, which renders the looking glass row of `docs/features.md` |
| 2 | Config syntax changed? | No | No config |
| 3 | CLI command added/changed? | Yes | `docs/guide/cli.md` and `docs/guide/command-reference.md` where `show bgp rib` filters are listed |
| 4 | API/RPC added/changed? | Yes | `docs/architecture/api/birdwatcher-compat.md` |
| 5 | Plugin added/changed? | No | `bgp-rib` gains a keyword, not a registration |
| 6 | Has a user guide page? | Yes | `docs/guide/looking-glass.md`, `docs/guide/looking-glass-howto.md` |
| 7 | Wire format changed? | No | No BGP bytes |
| 8 | Plugin SDK/protocol changed? | No | The command line is carried unchanged |
| 9 | RFC behavior implemented, changed, or newly proven? | No | No RFC row |
| 10 | Test infrastructure changed? | No | Two tests in an existing suite |
| 11 | Affects daemon comparison? | No | `docs/comparison.md` makes no claim about lookup forms. Checked at closure |
| 12 | Internal architecture changed? | Yes | `docs/architecture/plugin/rib-storage-design.md`, the covering read |
| 13 | Route metadata keys added/changed? | No | None |
| 14 | Prometheus counters added/changed? | No | None |
| 15 | Registered plugin, event type, send type, command, capability, or inventory changed? | No | No registration changes |
| 16 | Any changed source file referenced by existing doc source anchors? | Yes | Derived at implementation with `./le spec citation anchors spec plan/release/fixes/spec-lg-search-matches-what-the-pages-promise.md`. `docs/architecture/web-interface.md` is declared by the looking glass handlers and is checked there. `docs/guide/mcp/overview.md` is declared by `internal/plugins/help/help_ai.go` and is unaffected: this spec adds one example line to the AI help and changes nothing that page describes |
| 17 | Existing docs show config/CLI/API examples for this area? | Yes | The `curl` examples in `docs/guide/looking-glass-howto.md` gain one with a bare address |

## Implementation Steps

1. **Phase: Wiring (MANDATORY FIRST)** -- the keyword parses and the page sends it
   - Tests: `TestPipelineCoveringKeyword`, `TestSearchSendsCoveringForBareAddress`, both functional tests written and failing
   - Files: `rib_pipeline.go`, `handler_ui.go`
   - Verify: the engine accepts the keyword and answers nothing useful yet. The functional tests fail on the result, not on a refusal
2. **Phase: Covering read** -- the received source answers from the trie
   - Tests: `TestCoveringReadsOnlyThePath`, `TestCoveringOrdersMostSpecificFirst`, `TestCoveringListsEveryAddPathPath`, `TestPrefixTextFilterUnchanged`
   - Files: `rib_pipeline.go`, `rib_pipeline_best.go`, `storage/familyrib.go`
   - Verify: A-1 is settled first. `lg-covering-lookup` passes for the page
3. **Phase: API and graph** -- the other two writers follow the rule
   - Tests: `TestAPIPrefixSendsCoveringForBareAddress`, `TestGraphSendsCoveringForBareAddress`
   - Files: `handler_api.go`, `handler_graph.go`, `ribfake_test.go`
   - Verify: `lg-covering-lookup` passes for the API. The contract page is edited in this phase
4. **Phase: AS path grammar** -- the page speaks the engine's grammar
   - Tests: `TestSearchSendsEnginePathGrammar`, `TestSearchRefusesUnsupportedPathCharacters`
   - Files: `handler_ui.go`, `help.templ`
   - Verify: A-3 is settled first. `lg-aspath-search` passes. The help and guide pages are edited in this phase

### Critical Review Checklist
| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | Every AC-N has an implementation at file:line |
| Feature completeness | The page, the API and the graph give one answer for one input |
| Correctness | Row order is most specific first. ADD-PATH paths are all listed. IPv4 and IPv6 never mix |
| Naming | The keyword follows the existing filter names: one lowercase word, value after it |
| Data flow | The looking glass builds a command line and nothing else. No RIB import appears in `internal/component/lg` |
| Operator errors and doctor codes | The two refusals name the value, the reason and a valid form on the last line |
| Rule: fixture follows the engine | The test fake accepts the keyword only with the meaning the engine gives it, and both functional tests run a real `bgp-rib` |
| Rule: one grammar, one owner | The page-side AS path check accepts exactly the inputs `validatePathPattern` accepts after normalization. A test feeds both the same table of inputs |

### Deliverables Checklist
| Deliverable | Verification method |
|-------------|---------------------|
| Covering lookup through the page and the API | `./le test functional plugin` selecting `lg-covering-lookup` |
| Multi-AS path search through the page | `./le test functional plugin` selecting `lg-aspath-search` |
| Text `prefix` filter unchanged | `TestPrefixTextFilterUnchanged` |
| Pages state the built behavior | `./le doc check all`, and a read of the four pages against AC-13 |
| Declarations agree | `./le plugin declarations check` |

### Security Review Checklist
| Check | What to look for |
|-------|-----------------|
| Input validation | The prefix field is parsed as an address or a prefix before any command is built. The AS path field holds only AS numbers, separators and one leading `^` |
| Command injection | No user text reaches the command line except a parsed address, a parsed prefix, or validated AS numbers. A space inside a value cannot add a keyword |
| Resource exhaustion | The covering read is bounded by the prefix length per peer. The result cap stays in force |
| Error leakage | A refusal repeats the user's own value and nothing about the daemon |

### Failure Routing

| Failure | Route To |
|---------|----------|
| Compilation error | Fix in the phase that introduced it |
| Test fails for the wrong reason | Fix the test assertion or setup |
| Test fails on behavior mismatch | Re-read the source in Current Behavior. If misunderstood → RESEARCH |
| Lint failure | Fix inline. If architectural → DESIGN |
| Functional test fails | Check the AC: wrong AC → DESIGN, correct AC → IMPLEMENT |
| Audit finds a missing AC | Back to the relevant phase and implement |
| 3 fix attempts failed | STOP. Report all 3 approaches. Ask the user |

## Design Insights

- The page promised containment in three places while the engine compared text. Each page was reviewed for its links and anchors, never against the function that answers the search.

## Owned journal rows

| Class file | Date | Surface (short) |
|------------|------|-----------------|
| `reference-checked-claim-unchecked.md` | 2026-11-18 | looking glass pages promise an address lookup the RIB does not perform |
| `parser-refuses-a-form-its-producer-emits.md` | 2026-11-18 | looking glass sends a spaced AS path the RIB pipeline refuses |

## Key Design Decisions

| Decision | Alternatives Considered | Rationale |
|----------|------------------------|-----------|
| A new covering keyword in the `bgp-rib` pipeline | Change `prefix` to mean containment when given a bare address | `prefix` is a documented text filter with a documented partial form. Changing it alters an operator contract to fix a page. The new keyword is additive, and the CLI, REST and MCP get covering lookup too |
| The covering answer lists every covering route | Return only the most specific one, as the help text says today | A collector's user asks who announces the networks around an address, and the less specific routes are part of that answer. Most specific first keeps the old promise visible in the first row |
| The page converts a spaced AS path to the engine's comma form | Teach the engine to take several tokens as one path. Tell users to type commas | Routers print AS paths with spaces, so that is what users paste. A multi-token value would change the pipeline's one rule, that a filter takes one value. The conversion is one function with one test table shared with the engine's validator |
| The page refuses characters the engine cannot match | Keep accepting them and let the engine answer | The engine's answer is "invalid ASN", which names the wrong problem. The page knows the user tried a pattern and can say which forms exist |
| No option to hide default routes | A checkbox that drops `0.0.0.0/0` and `::/0` from a covering answer | Most specific first puts a default route last, where it informs and does not mislead. An option can be added when someone needs it |

## Known Limitations

- Covering lookup answers address-keyed families only. A VPN or EVPN route has no single covering order across route distinguishers, so those families are outside the question.
- The end anchor and exact-path search are new capabilities, specified in `plan/post-release/features/spec-lg-peer-history-origin-search-and-named-nodes.md`.

## Checklist

### Pre-Spec Verification (before the design is presented)
- [ ] Metadata table present, with a valid Status, Depends, Phase and Updated
- [ ] `ai/INDEX.md` keyword table checked
- [ ] An `rfc/short/` summary exists for every RFC referenced
- [ ] Template format followed: the 🧪 emoji, tables rather than prose, `[ ]` never `[x]`
- [ ] No code snippets
- [ ] Files to Modify names feature code, not only tests
- [ ] Current Behavior and Data Flow sections completed
- [ ] AC-N rows carry testable assertions
- [ ] Every assumption has a Basis and a validation method; every failure mode is a risk row
- [ ] Required Reading carries `→ Decision:` / `→ Constraint:` checkpoints
- [ ] Integration Checklist marks "CLI grammar" when a command is added, "Doctor check" when a runtime dependency is

### Goal Gates (MUST pass)
- [ ] AC-1..AC-N all demonstrated
- [ ] Every user story has a working path and a passing test
- [ ] Wiring Test table complete: every row a concrete test name, none deferred
- [ ] The targeted tests for what changed pass, run in the main checkout by the main thread: the scoped package tests, the component groups and functional suites `./le repo changed scope print both` reaches. No agent runs any `./le verify` command: GitHub CI verifies the pushed commit, and the full verification is Thomas's release check. A red CI reports gets a fix commit
- [ ] Feature code integrated (`internal/*`, `cmd/*`), not library-only
- [ ] Integration and Documentation checklists answered Yes/No/N-A with evidence
- [ ] Architectural Verification table filled, including registration over hardcoding
- [ ] Critical Review passes, and `ai/rules/quality.md` is satisfied: lint fixed rather than disabled, the focused check for the changed behavior run once with its OUTPUT PASTED, and any red named with the one-line reason it is scaffolding
- [ ] Every A-N confirmed or broken, none `unvalidated`
- [ ] Every item this spec did not do is a spec of its own, named here, in its own bucket

### TDD
- [ ] Tests written
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)
- [ ] Boundary tests for all numeric inputs (or N-A when the feature takes none)
- [ ] Functional `.ci` tests for end-to-end behavior
- [ ] Interop tests for protocol features (or N-A with a reason)

### Closure
- [ ] Append `plan/templates/TEMPLATE-CLOSURE.md` and complete every section in it
- [ ] `/ze-review` gate clean, recorded via `internal/le/spec/review.go`
- [ ] Any lesson routed to its governing surface under `ai/rules/planning.md`; no lesson artifact created merely for closure
- [ ] **Commit A:** code + tests + docs + edited spec + any journal rows owed by the work
- [ ] **Commit B:** `remove <the spec's path in its bucket>` only, in the same `./le commit create` script (commit A preserves the spec in history)
