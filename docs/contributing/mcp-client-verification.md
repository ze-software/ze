# MCP Real-Client Verification

This lane records which real agent clients connect to Ze's MCP server, call a
tool, and what context they report the tool catalog costs. The clients are
Claude Code and Codex. The practice comes from REA, an outside project: its
`docs/development/client-verification.md` keeps one lane per client, and its
`docs/mcp-catalog-evaluation.md` reads catalog cost from a real client's
reported usage. Those paths are in REA's repository, not in this one.

## Who runs it

Only Thomas starts this lane. An agent never starts it, and CI never runs it.
Each run spends model credits under his account. An agent that needs a result
reads the table at the end of this page, or asks Thomas for a run.

## What the bytes side already measures

The catalog's size in bytes, and the structural limits strict clients enforce,
are measured without a model. The `le test mcp` script verb `tools-size` reads
the `tools/list` answer of a running daemon, and <!-- doc-links: ignore (MCP method name, not a path) -->
`test/plugin/mcp-tools-list-structural-limits.ci` asserts the limits. Bytes do
not establish tokens: each client tokenizes and frames the catalog its own
way, so only the client's own report gives the context cost.

## Protocol revision risk

Ze accepts only MCP revision `2026-07-28`, the stateless Streamable HTTP
profile. A client that speaks an earlier revision first POSTs `initialize`.
Ze answers it with error `-32020`, which names the revision Ze speaks, and the
client cannot connect. That is a valid result: record the row with
"connected: no" and the error text the client shows. A client that cannot
connect also opens a fix spec in `plan/release/fixes/`, because an MCP
surface that no current client reaches is a shipped defect.

<!-- source: internal/component/mcp/streamable.go -- ProtocolVersion, Endpoint -->
<!-- source: internal/component/mcp/headers.go -- initializeEraError -->

## Steps

### 1. Start a loopback daemon with a bearer token

Make a new token for each lane run, and discard it after the run. Pass it to
the daemon through the environment, not on the command line, so it is not in
the process arguments:

The daemon needs its own config store, so run it from an empty directory that
`ze init` sets up, with `ze` built with the `ze_mcp` tag:

```bash
printf 'admin\nlanepass\n127.0.0.1\n12222\n' | ZE_CONFIG_DIR=. XDG_DATA_HOME=. ./ze init
printf '# ze mcp lane\n' > lane.conf
token=$(openssl rand -hex 32)
ZE_CONFIG_DIR=. XDG_DATA_HOME=. ze_mcp_token="$token" ./ze start --mcp 9718 lane.conf &
```

`--mcp <port>` is a `ze start` option and binds `127.0.0.1:<port>`. The MCP endpoint is
`http://127.0.0.1:9718/mcp`. A token with no `auth-mode` set selects the
`bearer` mode, so every request must carry `Authorization: Bearer <token>`.
The `--mcp-token <token>` flag and the config leaves `environment mcp
auth-mode bearer` and `token` do the same, but the flag puts the token in the
process arguments. Use the same `<config>` for every run in one round, and
record it in the notes.

<!-- source: cmd/ze/ze_core_dispatch.go -- flagStartMCP, flagStartMCPToken -->
<!-- source: cmd/ze/hub/main.go -- ze.mcp.token read in runYANGConfig -->
<!-- source: internal/component/config/environment.go -- ze.mcp.token registration -->
<!-- source: internal/component/mcp/streamable.go -- auth-mode inference from Token -->
<!-- source: internal/component/mcp/yang/ze-mcp-conf.yang -- auth-mode, token -->

Before you register a client, confirm the daemon answers. Put the line
`Authorization: Bearer <token>` in a mode-0600 file, then send the request
from [the MCP guide](../guide/mcp/overview.md), "Protocol", with
`--header @<file>`. The answer must be HTTP 200 with a `tools` result.

### 2. Measure each client without Ze's server

Run the headless prompt of step 4 once before Ze's server is registered with
the client. Record the token usage the client reports. This is the baseline.

### 3. Register Ze's server with the client

**Claude Code.** Add the server over HTTP with the Authorization header:

```bash
claude mcp add --transport http ze http://127.0.0.1:9718/mcp \
  --header "Authorization: Bearer $token"
```

Claude Code stores the header in its own configuration file. Remove the entry
after the run with `claude mcp remove ze`.

**Codex.** Add Ze's endpoint and the bearer token in the client's HTTP MCP
server configuration. This page does not give the exact syntax: it was not
checked against a reliable source when the page was written. Use the Codex
documentation of the version under test, and record that version in the row.

### 4. Run a headless prompt that calls `ze_reference`

Ze's server instructions tell a client to call `ze_reference` first, and the
tool takes no parameters. Use one fixed prompt for every client and every
run, for example: "Call the ze_reference tool of the ze MCP server once, then
reply with the number of address families it lists."

<!-- source: internal/component/mcp/discover.go -- instructions -->
<!-- source: internal/component/mcp/tools.go -- handcraftedTools, ze_reference handler -->

- **Claude Code:** run the prompt in headless mode (`claude -p "<prompt>"`)
  with JSON output (`--output-format json`). The result object carries the
  usage figures. Stream JSON output shows each tool call, so it also answers
  whether `ze_reference` was called.
- **Codex:** run the prompt in its non-interactive mode with JSON events
  enabled. The events carry the usage figures and the tool calls.

Run the prompt once with Ze's server registered. Compare the reported input
tokens with the baseline of step 2. The difference is the context the catalog
costs that client. If a client reports no usage, record the reason.

### 5. Clean up

Remove Ze's server from each client, stop the daemon, and discard the token.

## What each run records

| Field | Content |
|-------|---------|
| Client and version | The client name and the version it reports |
| Connected | yes or no; for no, the error text the client shows |
| `ze_reference` called | yes or no, read from the client's tool-call output |
| Tokens without Ze | The usage the client reports for the baseline run, or the reason it reports none |
| Tokens with Ze | The usage the client reports with Ze's server registered, or the reason it reports none |
| Ze commit | The commit the `ze` binary was built from |
| Date | The date of the run |

The bearer token never appears in a recorded result: not in a row, not in a
pasted log, not in a pasted client configuration. Remove it from any output
before you paste it here.

## Results

The table is empty until the first run.

| Client and version | Connected | `ze_reference` called | Tokens without Ze | Tokens with Ze | Ze commit | Date |
|--------------------|-----------|-----------------------|-------------------|----------------|-----------|------|
