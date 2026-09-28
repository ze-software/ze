# Functional `.ci` Tests

`.ci` is Ze's process and protocol test format. Use it when correctness is visible through a real command, daemon, socket, HTTP request, syslog line, file artifact, process exit, or BGP wire message.

A `.ci` file is an executable transcript. The runner reads key-value directives, creates a temporary workspace, writes embedded files, allocates ports, starts foreground and background commands, waits for readiness, and checks the observable result. That makes the test close to the debugging session a developer would run by hand, but repeatable enough for the verify gate.

## Where it belongs

| Behavior | Files | Narrow run | Native suite |
| --- | --- | --- | --- |
| BGP encode and route output | `test/encode/*.ci` | `./le test bgp encode NAME -v` | `./le test functional encode-test` |
| BGP plugin behavior | `test/plugin/*.ci` | `./le test bgp plugin NAME -v` | `./le test functional plugin-test` |
| Config parsing | `test/parse/*.ci` | `./le test bgp parse NAME -v` | `./le test functional parse-test` |
| Decode command output | `test/decode/*.ci` | `./le test bgp decode NAME -v` | `./le test functional decode-test` |
| Reload behavior | `test/reload/*.ci` | `./le test bgp reload NAME -v` | `./le test functional reload-test` |
| CLI output | `test/ui/*.ci` | `./le test ui NAME -v` | `./le test functional ui-test` |
| L2TP, firewall, policy, LDP, RSVP-TE, IS-IS, OSPF, OSPFv3, static, traffic, VPP, and install flows | `test/<suite>/*.ci` | `./le test <suite> NAME -v` | `./le test functional <suite>-test` |

Run `./le test bgp plugin --list` or the equivalent suite command before picking an id. The list output gives the exact test name, id, status, and rerun shape.

## Execution model

The runner has five stages. It parses the file into records, materializes embedded files in a temporary directory, starts background processes, waits for readiness, and then executes foreground commands and expectations in order. Each test owns its temporary directory and ports, so parallel runs do not share mutable state.

Background commands are for daemons and peers. Foreground commands are for assertions that should complete, such as `ze cli`, `ze config`, `curl`, helper scripts, or packet checks. A foreground process can assert stdout, stderr, exit code, files, JSON, HTTP, and BGP messages without adding a second test harness.

## Minimal shape

```
file=config.conf<<EOF
router bgp 65000
  neighbor 127.0.0.1 remote-as 65001
EOF

cmd=background:ze:bin/ze config.conf
cmd=background:peer:le test peer --as 65001 --listen 127.0.0.1:$PORT1
cmd=foreground:show:bin/ze cli -c "show bgp peer list"
expect=stdout:show:contains=Established
```

Use embedded files for configs, fixtures, plugin payloads, and helper scripts. Use variables such as `$TMP`, `$PORT1`, and `$ZE` rather than hard-coded paths and ports. That keeps the test parallel-safe and portable.

## Expectations

| Expectation | Use it for | Example |
| --- | --- | --- |
| `expect=stdout` and `reject=stdout` | CLI-visible text | `contains=Established` |
| `expect=stderr` | Warnings and validation errors | `contains=invalid prefix` |
| `expect=exit` | Process status | `code=1` |
| `expect=file` | Generated files and state dumps | `path=$TMP/out.json contains=neighbor` |
| `expect=json` | Decoded JSON with volatile fields normalized | `json={...}` |
| `expect=http` | HTTP readiness and body checks | `url=http://127.0.0.1:$PORT1/health code=200` |
| `expect=bgp` | Exact BGP messages | `seq=1 hex=...` |

Prefer the narrowest expectation that expresses the contract. For BGP, assert the decoded protocol behavior or the exact wire message. For JSON, remove known volatile fields instead of asserting a loose substring. For CLI output, assert the user-visible line that would catch a regression.

## Linux behavior

A `.ci` file that needs netlink, nftables, eBPF, PPP, L2TP, namespaces, kernel routing, or Linux-only sockets should say so directly:

```
option=needs-linux
```

On macOS that test reports SKIP. Inside QEMU the option is inert and the same file runs for real. Use `./le test qemu netns-test` for the curated Linux kernel suites. For one command or an interactive investigation, use `./le test qemu run command '...' keep-alive`.

## Failure reading

A good `.ci` failure tells you the file, step, line, assertion, process output, temporary directory, and rerun command. With `-v`, the runner keeps enough detail to see command stdout and stderr. BGP mismatches are decoded before display, which turns a wire failure into protocol fields.

```
./le test bgp plugin 42 -v
ZE_TEST_KEEP_TMP=1 ./le test bgp plugin 42 -v
ze.log.bgp.reactor.peer=debug ./le test bgp plugin 42 -v
```

If the failure only reproduces under Linux, rerun it through `./le test qemu run` rather than adding sleeps or Darwin-only skips.
