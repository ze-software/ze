# Functional `.ci` Tests

`.ci` is Ze's process and protocol test format. Use it when correctness is visible through a real command, daemon, socket, HTTP request, syslog line, file artifact, process exit, or BGP wire message.

<!-- source: ../main/docs/architecture/testing/ci-format.md -- format reference -->
<!-- source: ../main/internal/test/runner/record_parse.go -- parser -->
<!-- source: ../main/internal/test/runner/runner_exec.go -- process orchestration -->
<!-- source: ../main/internal/test/runner/runner_validate.go -- expectations -->
<!-- source: ../main/internal/test/peer/peer.go -- BGP test peer -->
<!-- source: ../main/internal/test/cli/ci_runner.go -- shared suite runner -->
<!-- source: ../main/internal/test/cli/cmd_bgp.go -- BGP suite routing -->

A `.ci` file is an executable transcript. The runner reads key-value directives, creates a temporary workspace, writes embedded files, allocates ports, starts foreground and background commands, waits for readiness, and checks the observable result. That makes the test close to the debugging session a developer would run by hand, but repeatable enough for the verify gate.

## Where it belongs

<table>
<thead><tr><th>Behavior</th><th>Files</th><th>Narrow run</th><th>Native suite</th></tr></thead>
<tbody>
<tr><td>BGP encode and route output</td><td><code>test/encode/*.ci</code></td><td><code>bin/ze-test bgp encode NAME -v</code></td><td><code>./le functional encode-test</code></td></tr>
<tr><td>BGP plugin behavior</td><td><code>test/plugin/*.ci</code></td><td><code>bin/ze-test bgp plugin NAME -v</code></td><td><code>./le functional plugin-test</code></td></tr>
<tr><td>Config parsing</td><td><code>test/parse/*.ci</code></td><td><code>bin/ze-test bgp parse NAME -v</code></td><td><code>./le functional parse-test</code></td></tr>
<tr><td>Decode command output</td><td><code>test/decode/*.ci</code></td><td><code>bin/ze-test bgp decode NAME -v</code></td><td><code>./le functional decode-test</code></td></tr>
<tr><td>Reload behavior</td><td><code>test/reload/*.ci</code></td><td><code>bin/ze-test bgp reload NAME -v</code></td><td><code>./le functional reload-test</code></td></tr>
<tr><td>CLI output</td><td><code>test/ui/*.ci</code></td><td><code>bin/ze-test ui NAME -v</code></td><td><code>./le functional ui-test</code></td></tr>
<tr><td>L2TP, firewall, policy, LDP, RSVP-TE, IS-IS, OSPF, OSPFv3, static, traffic, VPP, and install flows</td><td><code>test/&lt;suite&gt;/*.ci</code></td><td><code>bin/ze-test &lt;suite&gt; NAME -v</code></td><td><code>./le functional &lt;suite&gt;-test</code></td></tr>
</tbody>
</table>

Run `bin/ze-test bgp plugin --list` or the equivalent suite command before picking an id. The list output gives the exact test name, id, status, and rerun shape.

## Execution model

The runner has five stages. It parses the file into records, materializes embedded files in a temporary directory, starts background processes, waits for readiness, and then executes foreground commands and expectations in order. Each test owns its temporary directory and ports, so parallel runs do not share mutable state.

Background commands are for daemons and peers. Foreground commands are for assertions that should complete, such as `ze cli`, `ze config`, `curl`, helper scripts, or packet checks. A foreground process can assert stdout, stderr, exit code, files, JSON, HTTP, and BGP messages without adding a second test harness.

## Minimal shape

```text
file=config.conf<<EOF
router bgp 65000
  neighbor 127.0.0.1 remote-as 65001
EOF

cmd=background:ze:bin/ze config.conf
cmd=background:peer:bin/ze-peer --as 65001 --listen 127.0.0.1:$PORT1
cmd=foreground:show:bin/ze cli -c "show bgp peer list"
expect=stdout:show:contains=Established
```

Use embedded files for configs, fixtures, plugin payloads, and helper scripts. Use variables such as `$TMP`, `$PORT1`, and `$ZE` rather than hard-coded paths and ports. That keeps the test parallel-safe and portable.

## Expectations

<table>
<thead><tr><th>Expectation</th><th>Use it for</th><th>Example</th></tr></thead>
<tbody>
<tr><td><code>expect=stdout</code> and <code>reject=stdout</code></td><td>CLI-visible text</td><td><code>contains=Established</code></td></tr>
<tr><td><code>expect=stderr</code></td><td>Warnings and validation errors</td><td><code>contains=invalid prefix</code></td></tr>
<tr><td><code>expect=exit</code></td><td>Process status</td><td><code>code=1</code></td></tr>
<tr><td><code>expect=file</code></td><td>Generated files and state dumps</td><td><code>path=$TMP/out.json contains=neighbor</code></td></tr>
<tr><td><code>expect=json</code></td><td>Decoded JSON with volatile fields normalized</td><td><code>json={...}</code></td></tr>
<tr><td><code>expect=http</code></td><td>HTTP readiness and body checks</td><td><code>url=http://127.0.0.1:$PORT1/health code=200</code></td></tr>
<tr><td><code>expect=bgp</code></td><td>Exact BGP messages</td><td><code>seq=1 hex=...</code></td></tr>
</tbody>
</table>

Prefer the narrowest expectation that expresses the contract. For BGP, assert the decoded protocol behavior or the exact wire message. For JSON, remove known volatile fields instead of asserting a loose substring. For CLI output, assert the user-visible line that would catch a regression.

## Linux behavior

A `.ci` file that needs netlink, nftables, eBPF, PPP, L2TP, namespaces, kernel routing, or Linux-only sockets should say so directly:

```text
option=needs-linux
```

On macOS that test reports SKIP. Inside QEMU the option is inert and the same file runs for real. Use `./le qemu netns-test` for the curated Linux kernel suites. For one command or an interactive investigation, use `./le qemu run command '...' keep-alive`.

## Failure reading

A good `.ci` failure tells you the file, step, line, assertion, process output, temporary directory, and rerun command. With `-v`, the runner keeps enough detail to see command stdout and stderr. BGP mismatches are decoded before display, which turns a wire failure into protocol fields.

```bash
bin/ze-test bgp plugin 42 -v
ZE_TEST_KEEP_TMP=1 bin/ze-test bgp plugin 42 -v
ze.log.bgp.reactor.peer=debug bin/ze-test bgp plugin 42 -v
```

If the failure only reproduces under Linux, rerun it through `./le qemu run` rather than adding sleeps or Darwin-only skips.
