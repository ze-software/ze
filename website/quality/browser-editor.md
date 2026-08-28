# Browser `.wb` and Editor `.et` Tests

Ze has two UI functional formats. `.wb` drives the web interface through a real browser session. `.et` drives the interactive configuration editor through the CLI testing harness. Use them when the contract is what a person sees or types, not a helper function hidden under that interface.

<!-- source: ../main/docs/architecture/testing/runner-architecture.md -- web runner architecture -->
<!-- source: ../main/internal/component/web/testing/parser.go -- web parser -->
<!-- source: ../main/internal/component/web/testing/runner.go -- web runner -->
<!-- source: ../main/internal/component/web/testing/expect.go -- web expectations -->
<!-- source: ../main/internal/test/cli/cmd_web.go -- web test CLI -->
<!-- source: ../main/internal/component/cli/testing/parser.go -- editor parser -->
<!-- source: ../main/internal/component/cli/testing/expect.go -- editor expectations -->
<!-- source: ../main/internal/test/cli/cmd_editor.go -- editor test CLI -->

There is no `.wt` parser in the current tree. Web tests use `.wb`; editor tests use `.et`.

## Browser tests

A `.wb` file is a short browser script. The runner starts Ze, opens an isolated `agent-browser` session, runs actions, waits for browser activity to settle, and evaluates expectations against the rendered page. The point is to test the behavior after templates, HTMX, handlers, and client-visible state have all interacted.

Use `.wb` when navigation, form behavior, HTMX replacement, visible copy, page title, URL, or accessibility-visible elements are the contract. Use a Go unit test for pure handler logic and use `.ci` for simple HTTP status checks.

```bash
bin/ze-test web --list
bin/ze-test web -p config -v
./le functional web-test
```

### Browser syntax

<table>
<thead><tr><th>Line</th><th>Meaning</th><th>Example</th></tr></thead>
<tbody>
<tr><td><code>action=open</code></td><td>Navigate to a path</td><td><code>action=open:path=/config</code></td></tr>
<tr><td><code>action=click</code></td><td>Click by visible text or id</td><td><code>action=click:text=Save</code></td></tr>
<tr><td><code>action=fill</code></td><td>Fill an input selected by label, text, or id</td><td><code>action=fill:text=Hostname:value=router1</code></td></tr>
<tr><td><code>action=press</code></td><td>Press a key after optional focus</td><td><code>action=press:text=Search:key=Enter</code></td></tr>
<tr><td><code>action=wait</code></td><td>Wait for a small explicit condition when auto-waiting is not enough</td><td><code>action=wait:ms=100</code></td></tr>
<tr><td><code>expect=element</code></td><td>Assert visible text in the accessibility snapshot</td><td><code>expect=element:text=Routes</code></td></tr>
<tr><td><code>expect=url</code></td><td>Assert current browser URL</td><td><code>expect=url:contains=/config</code></td></tr>
<tr><td><code>expect=html</code></td><td>Assert markup when markup itself is the contract</td><td><code>expect=html:contains=hx-get</code></td></tr>
</tbody>
</table>

Prefer `id=` for stable controls and `text=` when the visible label is the contract. A `.wb` test should assert after state-changing actions so the failure points at the first broken transition. Avoid fixed sleeps except for behavior that genuinely has no observable readiness signal.

## Editor tests

An `.et` file is a replay script for the interactive configuration editor. The runner creates a temporary config root, starts the editor model, sends key and text input, and checks the prompt, context, completions, dirty state, validation messages, and persisted files. It is headless, which makes it fast, but it still exercises the editor input model rather than a single parser function.

```bash
bin/ze-test editor --list
bin/ze-test editor -p completion -v
./le functional editor-test
```

### Editor syntax

<table>
<thead><tr><th>Line</th><th>Meaning</th><th>Example</th></tr></thead>
<tbody>
<tr><td><code>session=</code></td><td>Create or reuse a named editor session</td><td><code>session=main</code></td></tr>
<tr><td><code>input=type</code></td><td>Type text into the editor</td><td><code>input=type:text=set interfaces</code></td></tr>
<tr><td><code>input=key</code></td><td>Send a named key</td><td><code>input=key:name=tab</code></td></tr>
<tr><td><code>input=enter</code></td><td>Submit the current line</td><td><code>input=enter</code></td></tr>
<tr><td><code>expect=prompt</code></td><td>Assert the prompt or context</td><td><code>expect=prompt:contains=interfaces</code></td></tr>
<tr><td><code>expect=completion</code></td><td>Assert completion candidates</td><td><code>expect=completion:contains=neighbor</code></td></tr>
<tr><td><code>expect=dirty</code></td><td>Assert whether pending changes exist</td><td><code>expect=dirty:value=true</code></td></tr>
<tr><td><code>expect=file</code></td><td>Assert saved config content</td><td><code>expect=file:path=config.conf contains=router bgp</code></td></tr>
<tr><td><code>restart=</code></td><td>Restart the editor and reopen persisted state</td><td><code>restart=session=main</code></td></tr>
</tbody>
</table>

Use editor tests for completion, validation, path context, commit and discard behavior, session persistence, and lifecycle bugs. Use `.ci` when the behavior is already visible through a non-interactive command.

## Failure reading

Both runners emit per-step trace records. The human output shows action and expectation lines with source locations. The machine output emits `VERIFY STEP` JSON so `./le verify current mode full` can group failures without scraping prose.

```bash
bin/ze-test web config-menu -v
bin/ze-test editor completion-basic -v
ZE_TEST_KEEP_TMP=1 bin/ze-test editor completion-basic -v
```

For web failures, read the snapshot before changing selectors. For editor failures, read the prompt, context, and buffered text before changing parser code. Most flaky UI tests come from checking too early; prefer an observable wait such as URL, element text, validation state, or command completion.
