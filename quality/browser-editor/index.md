# Browser `.wb` and Editor `.et` Tests

Ze has two UI functional formats. `.wb` drives the web interface through a real browser session. `.et` drives the interactive configuration editor through the CLI testing harness. Use them when the contract is what a person sees or types, not a helper function hidden under that interface.

There is no `.wt` parser in the current tree. Web tests use `.wb`; editor tests use `.et`.

## Browser tests

A `.wb` file is a short browser script. The runner starts Ze, opens an isolated `agent-browser` session, runs actions, waits for browser activity to settle, and evaluates expectations against the rendered page. The point is to test the behavior after templates, HTMX, handlers, and client-visible state have all interacted.

Use `.wb` when navigation, form behavior, HTMX replacement, visible copy, page title, URL, or accessibility-visible elements are the contract. Use a Go unit test for pure handler logic and use `.ci` for simple HTTP status checks.

```
bin/ze-test web --list
bin/ze-test web -p config -v
make ze-functional-web-test
```

### Browser syntax

| Line | Meaning | Example |
| --- | --- | --- |
| `action=open` | Navigate to a path | `action=open:path=/config` |
| `action=click` | Click by visible text or id | `action=click:text=Save` |
| `action=fill` | Fill an input selected by label, text, or id | `action=fill:text=Hostname:value=router1` |
| `action=press` | Press a key after optional focus | `action=press:text=Search:key=Enter` |
| `action=wait` | Wait for a small explicit condition when auto-waiting is not enough | `action=wait:ms=100` |
| `expect=element` | Assert visible text in the accessibility snapshot | `expect=element:text=Routes` |
| `expect=url` | Assert current browser URL | `expect=url:contains=/config` |
| `expect=html` | Assert markup when markup itself is the contract | `expect=html:contains=hx-get` |

Prefer `id=` for stable controls and `text=` when the visible label is the contract. A `.wb` test should assert after state-changing actions so the failure points at the first broken transition. Avoid fixed sleeps except for behavior that genuinely has no observable readiness signal.

## Editor tests

An `.et` file is a replay script for the interactive configuration editor. The runner creates a temporary config root, starts the editor model, sends key and text input, and checks the prompt, context, completions, dirty state, validation messages, and persisted files. It is headless, which makes it fast, but it still exercises the editor input model rather than a single parser function.

```
bin/ze-test editor --list
bin/ze-test editor -p completion -v
make ze-functional-editor-test
```

### Editor syntax

| Line | Meaning | Example |
| --- | --- | --- |
| `session=` | Create or reuse a named editor session | `session=main` |
| `input=type` | Type text into the editor | `input=type:text=set interfaces` |
| `input=key` | Send a named key | `input=key:name=tab` |
| `input=enter` | Submit the current line | `input=enter` |
| `expect=prompt` | Assert the prompt or context | `expect=prompt:contains=interfaces` |
| `expect=completion` | Assert completion candidates | `expect=completion:contains=neighbor` |
| `expect=dirty` | Assert whether pending changes exist | `expect=dirty:value=true` |
| `expect=file` | Assert saved config content | `expect=file:path=config.conf contains=router bgp` |
| `restart=` | Restart the editor and reopen persisted state | `restart=session=main` |

Use editor tests for completion, validation, path context, commit and discard behavior, session persistence, and lifecycle bugs. Use `.ci` when the behavior is already visible through a non-interactive command.

## Failure reading

Both runners emit per-step trace records. The human output shows action and expectation lines with source locations. The machine output emits `VERIFY STEP` JSON so `make ze-precommit-verify` can group failures without scraping prose.

```
bin/ze-test web config-menu -v
bin/ze-test editor completion-basic -v
ZE_TEST_KEEP_TMP=1 bin/ze-test editor completion-basic -v
```

For web failures, read the snapshot before changing selectors. For editor failures, read the prompt, context, and buffered text before changing parser code. Most flaky UI tests come from checking too early; prefer an observable wait such as URL, element text, validation state, or command completion.
