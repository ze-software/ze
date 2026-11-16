# Spec: container-5-shell

| Field | Value |
|-------|-------|
| Status | ready |
| Scope | plugin |
| Priority | P3 |
| Release reason | A post-release feature. The owner ruled on 2026-11-16 (umbrella O-8) that a shell inside a running container is in scope now, as a child spec of the container work. No shipped behavior is wrong without it. |
| RFC gaps | - |
| Depends | spec-container-1-runtime |
| Phase | - |
| Handoff | - |
| Design tier | strongest tier |
| Implementation tier | strongest tier |
| Updated | 2026-11-16 |

Recovery after compaction: `.claude/rules/post-compaction.md`.

## Task

An operator who runs a container on Ze cannot look inside it. The container work (`spec-container-0-umbrella.md`) starts, confines and supervises a container, and gives no way to run a program in it after it started. The owner ruled that this is in scope now.

Two owner rules bind the design (`ai/META-DECISIONS.md`, "Goal and scope"):

- Every operator command runs through the daemon on every command surface (MCP, REST, gRPC, SSH, web) with the same output as the local command.
- A command whose result lives on the caller's side gets a daemon form designed for it, never an exclusion.

A shell breaks two things Ze has today, and this spec designs both:

| Problem | What Ze has today | What is missing |
|---------|-------------------|-----------------|
| A. Entering a running container | The upstream runtime library starts a second process in a container through C code that joins the namespaces before the Go runtime starts. Ze builds with cgo off, so that code is absent | A pure-Go way to put a new process inside the container's namespaces, cgroup and confinement |
| B. A command with an input stream | A command answers once, or streams output through a writer. No command takes input after it started, on any surface, and the plugin process protocol carries no byte stream in either direction | One mechanism for a command that holds a session with input, registered by any plugin, with a form on every surface |

The goal: an operator opens a shell, or runs one program, inside a running container from every surface. A surface that carries a terminal gives a terminal. A surface that carries only a request and an answer runs one program and answers its output and its exit status, inside a time bound and a size bound.

The owner approved this design on 2026-11-16, with the recommended answer in every row of the table below. The alternatives set aside are kept in Key Design Decisions.

Three rulings of the umbrella bound the work further:

- Containers run on the appliance and under a daemon that runs as root, at first. The runtime child refuses a container config anywhere else, so a shell is never asked for there.
- When Ze restarts, including for its own update, containers are stopped and started again. Every session ends then.
- The list of containers in the config is named `instance`. A command selects one container with the `name` keyword, placed last, as `spec-container-4-operator-surface.md` orders every container command. The design of that child is settled before this child's wiring phase, and this child waits for none of its code, so the Depends row names the runtime child alone. This child owns its three commands, their leaves and their answers, and lands them. They are rows of that child's table C-1, and the three session limit leaves are rows of its schema tables.

### Open decisions (owner)

None. Every row of "Owner decisions" below was decided on 2026-11-16, and the one owner action this child needed, OA-1, was authorised on the same day:

### Owner authorisation for the shell's program (given 2026-11-16)

| # | What the owner authorised | What was authorised before | Why it is a second use | Where the register row lands |
|---|---------------------------|----------------------------|------------------------|------------------------------|
| OA-1 | Ze running a program an operator names for a shell in a container, scoped to the shell. Authorised by the owner on 2026-11-16 ("Approve and authorise.", in answer to the main thread's ask for this second kind of external program) | The umbrella's O-4: the container init role executing a container's entrypoint, and nothing more | The entrypoint is named by the container's config and image and started once with the container. A shell program is named by an operator on a command line (`command`, or `/bin/sh` when none is named) and started in a container that already runs. `ai/rules/go-standards.md` allows shipped code to run no external program without the owner's row | The row of `ai/allowed-system-commands.md` lands in the commit that adds the code it names, citing this ruling: phase 6 of Implementation Steps, under A1 and under A3. The probes of the gate before phase 1 and phases 1 to 5 run no such program |

### Owner decisions (decided 2026-11-16)

| # | Question | Decided: the recommendation | What the operator sees under the decided answer, and under the answer set aside |
|---|----------|-----------------------------|---------------------------------------------------------------------------------|
| O-1 | How a new process enters a running container with cgo off (Key Design Decisions, rows A1 to A4) | A1: extend the build-time diff. The daemon joins the namespaces that allow it on a throwaway thread, starts the init role once, and the library's own second-process sequence applies the confinement. The probes for A-1 to A-8 are the first task of this child, run in the appliance guest before any other implementation. If the probe for A-8 fails, A3 becomes the design ("The fallback entry design" under Data Flow) | The command and its output are the same under every answer. A1 and A2: the shell starts after one (A1) or three (A2) starts of the `ze` binary, and the program runs under exactly the container's confinement as the library computes it. A3: the same, and Ze owns the confinement sequence, so an upstream security fix reaches Ze only when someone reads it. A4: every container carries a resident Ze helper as its first process, so a process listing of the container shows a `ze` process as process 1, the image's entrypoint is no longer process 1, and each container costs the memory of one more Go program |
| O-2 | The command words. The reference model uses a root word that is not a Ze verb, so the grammar refuses it | One command, `request container shell name <name>`, with the optional leaves `command`, `input` and `timeout`. The session listing is `show container session list`. The forced end is `delete container session id <id>`: the operator surface child lists this child's three commands in its table C-1 and applies its verb rules, and an open session that stops existing is the `delete` test (main thread, 2026-11-16). The token order (action, then the typed selector) is the one `spec-container-4-operator-surface` uses for `request container restart name` | Recommended: `request container shell name web` opens `/bin/sh`; `request container shell name web command "ip addr"` runs one program. Alternative, two commands: `request container shell name web` for the terminal and `request container run name web command "ip addr"` for one program, which gives the one-shot surfaces a path of their own and costs a second registration for one mechanism |
| O-3 | The program when the operator names none, and how `command` is read | The program is `/bin/sh`. `command` is split into words by Ze's quoted-value grammar and run directly, with no shell between | Recommended: `... shell command "ls /etc"` works in an image that has `ls` and no shell, and `... shell command "ls | wc"` passes `|` and `wc` to `ls` as words. Alternative, run `command` through `/bin/sh -c`: pipes and redirections work, and an image with no shell runs nothing |
| O-4 | A live terminal in the browser. The web terminal sends one line and renders one answer; Ze carries no terminal emulator for the browser | Not now. The web console and the web terminal run the bounded form. A live browser terminal is a second spec, because it adds a third-party browser asset, which the owner approves | Recommended: in the web terminal, `request container shell name web command "ip addr"` prints the output and `exit-status 0`; `request container shell name web` alone is refused with the line that says to add `command` or to use SSH. Alternative: the web terminal opens a live shell with keys, colors and resize, as SSH does |
| O-5 | A live session over REST. REST carries a request and an answer, and a server event stream | Not now. REST runs the bounded form. gRPC gets the live session, because it carries a stream in both directions natively | Recommended: `POST /api/v1/execute` with the command answers a record holding `exit-status`, `stdout` and `stderr`. Alternative: a session resource (open, an event stream for output, a post for input, close), which gives a REST client a live shell and adds four routes for one feature |
| O-6 | What the audit log holds for a session | The start and the end: who, from where, on which surface, the command line, the exit status, the duration and the byte counts. Not the bytes | Recommended: `show audit` lists `session-start` and `session-end` for each shell, and nothing typed inside it. Alternative: Ze records every byte in and out, so an auditor replays the session; passwords typed inside the container are then stored on the router, and the store needs a size bound |
| O-7 | A container whose config grants the capability to trace processes, or all capabilities. Such a container can take control of the entering process in the window before it drops its privileges (R-2) | Refuse the shell for that container, and say why. The refusal holds under the chosen entry design (A1) only. Under the fallback entry design (A3) no host process image is visible inside the container, so nothing can be taken over and the refusal does not exist there | Recommended, under A1: `request container shell name dbg` answers that the container holds the trace capability, that a process entering it can be taken over, and names the config leaf to remove. Under A3 the same command opens the shell. Alternative: allow it under A1, with a warning line before the prompt; a hostile program in that container can then reach the host |
| O-8 | The limits and their defaults | Idle 15 minutes, absolute 8 hours, 4 sessions for one container, 16 for the daemon. The bounded form: 30 seconds unless `timeout` says otherwise (1 to 600), 1 MiB for each output stream | Recommended: a shell left alone is closed after 15 minutes with a line naming the limit and the leaf that changes it. A fifth shell into one container is refused with the list command. Any other values change only those numbers |
| O-9 | How session bytes cross the plugin process protocol. This changes a published contract (`pkg/plugin/rpc`) | Framed messages on the existing connection, each acknowledged, so one session cannot starve the plugin's other calls | Nothing an operator sees differs. A third-party plugin author sees either seven new messages on the connection they already parse (recommended), or a second connection per session that carries raw bytes, which is faster for bulk data and needs a second acceptor path and a one-time token |
| O-10 | Authorization. A shell in a container is a privileged act | It follows command authorization as every `request` command does: the read-only profile denies it, and a profile grants it by path, down to one container name. No second mechanism | Recommended: an operator whose profile lacks `request container` reads the standard unauthorized line. Alternative: a dedicated grant that no existing profile holds until the owner of the router adds it, so an upgrade never widens what an existing operator can do |

## Required Reading

### Architecture Docs
- [ ] `plan/post-release/features/spec-container-0-umbrella.md` - the approved container design and the owner's rulings
  → Decision: the upstream runtime library is used whole behind a small build-time diff, with cgo off. This spec extends that diff or adds beside it; it does not replace it
  → Decision: the container init role is selected by the library's own environment key before command dispatch. The second-process role is selected the same way
  → Constraint: the library seals the `ze` binary before each start of the init role and marks every descriptor of the calling process close-on-exec. Both happen again at each shell
  → Constraint: user namespaces, namespace paths at create and time offsets are refused by the diff. The container's namespaces are pid, network, ipc, uts, mount and cgroup
- [ ] `plan/post-release/features/spec-container-1-runtime.md` (a draft; its task, diff table, seams, lifecycle states, data flow, wire census, risks and file lists were read) - what this child builds on
  → Constraint: its hunk H-3 makes `newSetnsProcess` refuse and names this child as the owner of entering a running container. This child rewrites that hunk
  → Constraint: the series is one patch file per hunk, under a review stamp and a named guard test, and the filter is Ze code behind a registration seam (H-6). This child's hunks follow both
  → Constraint: a container is killed through its cgroup, never by process group, and the daemon stop reaches containers only in the stop-drain call, whose budget is declared at Stage 1
  → Decision: the early role leaf selects the init role from the library's environment key. The second process is one more role behind the same key
  → Constraint: its functional tests sit in the `container` suite, `test/container/`, which it registers, and run in the own-system guest. This child's container tests sit beside them
- [ ] `ai/doors/system.md` - the design every subsystem shares
  → Constraint: every plugin also runs as a separate process, which talks only over TLS and RPC. A session mechanism that hands a plugin a Go reader and writer works only internal, and is the classic defect
  → Constraint: a command is declared once and reaches every interface. Adding a command to one interface by hand is the defect
  → Constraint: a change under `pkg/` is a change to a published contract and is made on purpose, with its page
- [ ] `ai/doors/commands-cli.md`, `docs/architecture/api/commands.md` ("Which surfaces offer a command", "One-shot answer of a streaming command", "Where a monitor command streams", "A command that starts work outliving its request") - where a command is served
  → Decision: which surfaces offer a command is one predicate, `command.AnswerSurface`. The interactive kind is one more declaration it reads; no surface keeps a list
  → Decision: a streaming command declares what it answers where no stream can be shown, and its one-shot route is the RPC under the same path. An interactive command declares the same thing, and the engine derives the bounded run from the session
  → Constraint: a handler answers data, never text. The bounded form answers a record, and `| json` renders it
  → Constraint: work that outlives its request binds to the plugin server with `Server.OnStop`, so the daemon's stop ends it
- [ ] `ai/doors/plugin-infra.md`, `ai/doors/core.md`, `docs/architecture/api/process-protocol.md` ("Wire Format", "Command Execution") - the process protocol
  → Constraint: a request is one newline-framed line, so a payload is compact JSON. Raw bytes cannot travel on the connection unframed
  → Constraint: one engine-op entry per RPC, and every feature works over both transports. `DirectBridge` is a shortcut of the same protocol
  → Constraint: before the first release the protocol owes no backward compatibility. It changes directly, with no version gate
- [ ] `ai/doors/api-surfaces.md`, `ai/digests/mcp.md` - REST, gRPC, web and MCP
  → Constraint: the surfaces are thin adapters and never own a command
  → Constraint: MCP is stateless and strictly client to server. A task is polled and raises no input request, so MCP carries a request and an answer and nothing else
- [ ] `ai/doors/access-security.md` - authorization and audit
  → Constraint: TACACS+ accounting starts for every command entered, whatever authorization answers. A session is one command whose accounting stops when the session ends
  → Constraint: `internal/core/audit` owns the audit log, and a recorder must not block the operation
- [ ] `ai/doors/system-plugins.md`, `ai/doors/hub-engine.md`, `ai/doors/le-tooling.md` - placement, daemon lifecycle, tests
  → Decision: the container plugin is an edge plugin and owns its commands. The session mechanism is infrastructure under `internal/component/plugin/server` and names no plugin
  → Constraint: one reload path. A reload ends a session only when it stops or restarts that session's container
  → Constraint: a `.ci` that needs a Linux kernel feature carries `option=needs-linux` and runs in the QEMU guest
- [ ] `ai/rules/cli.md`, `docs/architecture/cli/command-verbs.md` - grammar
  → Decision: starting a process changes the system and is none of create, delete or update, so the verb is `request`
  → Constraint: keywords before values, a typed selector (`name`, `id`), no `--flag`, and a set action never acts on all when the selector is omitted
- [ ] `ai/rules/go-standards.md`, `ai/allowed-system-commands.md` - external programs
  → Constraint: shipped code runs no external program without the owner's row. The umbrella's O-4 authorizes the init role executing a container's entrypoint. A program an operator names for a shell is a second use, which the owner authorised on 2026-11-16 (OA-1 under Task), and the row must say so (R-9)
- [ ] `ai/rules/goroutine-lifecycle.md` - workers
  → Constraint: a session's relay is a worker that lives exactly as long as the session and selects on its cancellation, the shape `sshclient.RunInteractive` already uses for resize

### Upstream source (reference, read at tag v1.5.3)
- [ ] `libcontainer/process_linux.go` `setnsProcess.start`, `startWithCgroupFD`, `prepareCgroupFD`, `execSetns` - the parent side of a second process
  → Constraint: the parent starts the init role directly inside the container's cgroup with the clone-into-cgroup flag, and retries without it when the start fails. It then sends the bootstrap data, waits for the first stage to exit, reads the final process id from the init socket, sets resource limits when the child reports ready, and releases it
- [ ] `libcontainer/nsenter/nsexec.c` `join_namespaces`, `nsexec` - the C join
  → Constraint: the join opens every namespace descriptor first, because paths can vanish after the mount or user join. It joins in the list order (ipc, uts, network, pid, mount, cgroup when no user namespace is used), sets the process non-dumpable when it joins by path, and forks once more because a pid namespace join applies only to children
- [ ] `libcontainer/setns_init_linux.go` `linuxSetnsInit.Init`, `libcontainer/init_linux.go` `finalizeNamespace`, `setupConsole`, `verifyCwd` - what Go applies after the join
  → Decision: this sequence is the checklist for any design that leaves the library (A3), and what A1 and A2 inherit unchanged
- [ ] `libcontainer/console_linux.go` `safeAllocPty` - the terminal
  → Constraint: the terminal is allocated inside the container from its own pts instance, through a handle opened without following a link and verified, and the peer is obtained from the descriptor, never by path
- [ ] The Go standard library, `syscall/exec_linux.go` `forkAndExecInChild1`, `runtime/proc.go` `LockOSThread` - process start from a locked thread
  → Decision: Go starts a child with the virtual-memory and vfork flags and without the filesystem-sharing flag, and supports clone-into-cgroup. A locked thread forces new runtime threads to come from a template thread

### RFC Summaries (Scope: protocol)
N-A: Scope is plugin. The SSH terminal, resize and signal requests are served by the vendored SSH library; Ze reads them and implements no part of the SSH protocol here.

**Key insights:** (minimal context to resume after compaction)
- The library's second-process path needs the C join for one reason only: a pid namespace join applies to children, and the Go runtime cannot fork without exec. Every other step of the path is Go and is inherited.
- `setns` moves one thread. The man page on this machine (man-pages 6.7) restricts two namespace types: a user namespace cannot be joined by a multithreaded process, and a mount namespace cannot be joined by a thread that shares filesystem attributes with another. It states no thread restriction for network, ipc, uts, pid, cgroup and time. A process descriptor joins several namespaces in one atomic call since Linux 5.8.
- Ze already enters a namespace on a locked thread that is never unlocked (`kernelcap.InThrowawayNetworkNamespace`). The same discipline joins a container's network, ipc, uts, cgroup and children's pid namespace in the daemon, on a thread that then starts one child and dies.
- The mount namespace is joined by the child itself, on its locked main thread, after it stops sharing filesystem attributes. Its other runtime threads stay in the host mount namespace until the final exec replaces the process. This is weaker than upstream and is R-2.
- A streaming handler gets a writer only, and the two streaming executors authorize as read-only with a comment that a write-capable stream needs registry metadata. An interactive command is that metadata.
- The SSH exec channel is already handed whole, as a reader and a writer, to the plugin protocol debug session, and the SSH client already opens such a session (`sshclient.OpenProtocolSession`). The vendored SSH library exposes the terminal request, a window-change channel and signals. Bubble Tea exposes release and restore of the terminal. gRPC has a server stream and no client stream. REST has an event stream. The web terminal posts one line. MCP is request and answer.
- The process protocol frames a request by its newline and answers a command with a head, records and a terminator. Nothing flows toward a plugin for a command that is already running.

## Current Behavior (MANDATORY)

**Source files read:** (must read BEFORE you write this spec)
- [ ] `internal/component/plugin/server/handler.go` - `StreamingHandler` takes a context, the server, a writer, the user name and validated arguments, and nothing to read from. `RegisterStreamingHandler` requires a one-shot declaration (`OneShotSample` or `OneShotStreamingOnly`). `RPCRegistration.AuditAction` lets an owner declare the audit action of a builtin
- [ ] `internal/component/command/surface.go` - `AnswerSurface` derives the offer from the registration alone: a streaming command is offered streaming on SSH, `ze cli` and the shell, and by its one-shot declaration elsewhere. The offers are must, must not, streaming and TUI-only
- [ ] `cmd/ze/hub/streaming_surface.go` - `streamingRegistration` builds the predicate's input from the streaming registry; the one-shot route of a streaming command is the builtin RPC under the same path
- [ ] `internal/component/ssh/ssh.go` (`execMiddleware`, `teaHandler`, `sessionCloseMiddleware` read; the rest mapped by symbol) - an exec channel runs one command: lifecycle words, the plugin protocol session (the channel handed as reader and writer), a streaming command through the streaming executor, or a dispatched command with its pipe chain. A channel with no command becomes a Bubble Tea model
- [ ] `cmd/ze/hub/service_ssh.go` (`sshWireImpl`, the two executor factories) - the streaming executor starts accounting, authorizes with the read-only flag set, validates the arguments against the model, then runs the handler on the session's writer
- [ ] `cmd/ze/hub/api.go` (`buildAPIEngine`, `apiStreamSource`) - REST and gRPC streams run the same streaming handlers into a channel of lines, with the same accounting and the same read-only authorization
- [ ] `internal/component/plugin/server/command.go` (`Dispatcher.Dispatch`) - the one-shot route: accounting, the pipe refusal, the match, argument validation, authorization by the command's read-only class, the streaming-only refusal, then the handler and its audit record
- [ ] `internal/component/plugin/server/startup.go` (`validateHelpDecls`, `validateDeclaredText`) - a Stage 1 declaration is judged before it is stored
- [ ] `internal/component/plugin/server/dispatch_registry.go` (`engineOp`, the head of `engineOps`) - one entry per plugin-to-engine operation drives the JSON path, the direct path and the typed bridge slot
- [ ] `internal/core/ipc/yang/ze-plugin-callback.yang` (`execute-command`), `internal/core/ipc/yang/ze-plugin-engine.yang` (module head) - every protocol method is a YANG RPC with a `ze:method` name
- [ ] `pkg/plugin/rpc/types.go` (`CommandDecl`, `ExecuteCommandInput`, `ExecuteCommandOutput`), `pkg/plugin/rpc/bridge.go` (`DirectBridge`, `BridgeCallback`) - a command declaration carries its name, help, arguments and answer shape; the bridge carries engine-to-plugin callbacks on a channel and typed slots
- [ ] `pkg/plugin/sdk/sdk_callbacks.go` (`OnExecuteCommand`), `pkg/plugin/sdk/sdk_types.go` - one `On*` method per callback; the SDK types alias the wire types
- [ ] `internal/component/api/engine.go` - `Execute` answers once; `Stream` returns a channel of lines and a cancel function. Both authorize from the command's read-only class
- [ ] `internal/component/api/grpc/server.go` (`Execute`, `Stream`) - `Stream` loops over the engine's channel and sends one response per line
- [ ] `api/proto/ze.proto` (the two services and the command messages) - `Execute` is unary and `Stream` is a server stream. No method carries a client stream
- [ ] `internal/component/cli/sshclient/terminal.go` - `RunInteractive` opens a blocking terminal, requests a terminal on the SSH session, forwards window changes from one worker that lives as long as the session, and transports bytes only. The daemon owns the model
- [ ] `internal/component/cli/sshclient/client.go` (`StreamCommand`, `OpenProtocolSession`) - the client reads a stream line by line, or holds a session with an input pipe and an output pipe
- [ ] `internal/component/cli/client/main.go` (`cliClient.execute`, `isMonitorCommand`) - `ze cli -c` serves a local command in its own process, else streams the daemon's answer; it asks the streaming registry whether a command streams
- [ ] `internal/component/cli/model_keys.go` (the end of `handleEnter`) - the model opens a registered view, then a streaming session for a streaming command, then dispatches
- [ ] `internal/component/kernelcap/namespace_linux.go` - `InThrowawayNetworkNamespace` unshares a network namespace on a goroutine that locks its thread and returns without unlocking, so the runtime destroys the thread
- [ ] `internal/core/audit/audit.go` (the constants and `Entry`) - an entry holds a timestamp, an actor, a remote address, a surface, an action, a detail and an outcome. The actions are a closed set of constants
- [ ] `docs/architecture/api/process-protocol.md` ("Wire Format", "Command Execution") - one line per message; a command answer is a head, records and a terminator

**Behavior to preserve:**
- Every existing command, streaming command and local command answers as it does today on every surface. `AnswerSurface` answers the same for every registration that is not interactive.
- The streaming executors keep the read-only authorization for streaming commands.
- The plugin process protocol keeps every existing message. The new messages are additions.
- The container runtime of `spec-container-1-runtime` starts, confines and supervises a container as that spec defines. This spec changes no step of the create path.
- An interactive `ze cli` session stays a shell on the daemon: the client transports terminal bytes and holds no model.

**Behavior to change:**
- None of the existing behavior. Two things are added: a registration kind for a command that holds a session with input, with its route on every surface, and the container plugin's use of it.

## Existing Mechanisms (MANDATORY)

| Mechanism | Where (file + symbol, or page + section) | What it does today | This spec (extends, replaces, or does not apply, with the reason) |
|-----------|------------------------------------------|--------------------|-------------------------------------------------------------------|
| Streaming command registry | `internal/component/plugin/server/handler.go` `RegisterStreamingHandler`, `RegisterPluginStreamingHandler`, `OneShot` | A command registers a handler that writes a stream, and declares its one-shot answer. Output only | Extends by a sibling: the interactive registry has the same shape (prefix, declared one-shot form, handler, optional plugin command) and adds the input half. It is not folded into the streaming registry, because a streaming command is a read and an interactive command changes the system, and the authorization differs |
| Surface predicate and its registry adapter | `internal/component/command/surface.go` `AnswerSurface`, `CommandRegistration`; `cmd/ze/hub/streaming_surface.go` `streamingRegistration` | One predicate decides what each surface owes each command, from the registration | Extends: one field (`Interactive`) and one offer. A terminal surface owes the live session; a one-shot surface owes the bounded form. The hub gains the matching adapter for the interactive registry |
| Surface parity test | `cmd/ze/hub/surface_parity_test.go` `TestEveryCommandOnEverySurface`, `TestStreamingOneShotAnswersCarryData` | Fails a command a surface omits or offers against the predicate, and a sample that carries no data | Extends: an interactive command's bounded answer must carry its record on every one-shot surface |
| One-shot dispatch route | `internal/component/plugin/server/command.go` `Dispatcher.Dispatch` | Accounts, authorizes by the read-only class, validates, refuses a streaming-only command, runs the handler, records its audit action | Extends: the bounded run is the interactive command's handler on this route, derived by the engine from the session handler. No step of the route changes |
| SSH exec channel as a raw duplex | `internal/component/ssh/ssh.go` `execMiddleware`, the `plugin protocol` branch; `internal/component/cli/sshclient/client.go` `OpenProtocolSession` | The server hands the session as a reader and a writer to `HandleAdHocPluginSession` after authorization; the client holds an input pipe and an output pipe | Extends: the interactive branch hands the same channel, with the terminal request and the window-change channel, to the session route. The branch is chosen from the interactive registry, never from a command name |
| SSH terminal session | `internal/component/cli/sshclient/terminal.go` `RunInteractive`; `internal/component/ssh/ssh.go` `teaHandler` | A persistent terminal carries the daemon's model; the client forwards bytes and window changes | Extends: the daemon's model releases the terminal to a session and restores it, so the client changes nothing. `ze cli -c` gains a terminal request for an interactive command |
| Streaming executors and accounting | `cmd/ze/hub/service_ssh.go` `SetStreamingExecutorFactory`; `cmd/ze/hub/api.go` `apiStreamSource`; `Dispatcher.BeginAccounting`, `Dispatcher.IsAuthorized` | Account for every command, authorize, validate the arguments against the model, run the handler | Extends: the session route does the same four steps with the read-only flag clear, and stops accounting when the session ends |
| Audit actions declared by the owner | `internal/component/plugin/server/handler.go` `RPCRegistration.AuditAction`; `internal/core/audit/audit.go` | A builtin declares the action a successful run records | Extends: two actions, session start and session end, recorded by the session route for every interactive command. The owner declares nothing more |
| Work that outlives a request | `internal/component/plugin/server/stop_hooks.go` `Server.OnStop` (read through `commands.md`, "A command that starts work outliving its request") | The daemon's stop runs each registered hook and waits for it | Extends: the session registry registers one hook, which ends every session and waits for the relays |
| Plugin process protocol, engine-op registry and SDK callbacks | `pkg/plugin/rpc/types.go`, `pkg/plugin/rpc/bridge.go` `DirectBridge`; `internal/component/plugin/server/dispatch_registry.go` `engineOps`; `pkg/plugin/sdk/sdk_callbacks.go`; the two protocol YANG modules | Line-framed requests both ways, one entry per plugin-to-engine operation, one `On*` method per callback, a typed bridge slot for a fast path | Extends: seven messages, two engine-op entries and one callback. No existing message can carry a session, because none flows toward the plugin for a running command |
| Stage 1 declaration checks | `internal/component/plugin/server/startup.go` `validateHelpDecls` | Judges declared text before it is stored | Extends: an interactive declaration without a one-shot form is refused at the same place |
| Locked-thread namespace entry | `internal/component/kernelcap/namespace_linux.go` `InThrowawayNetworkNamespace` | A goroutine locks its thread, changes namespace, works, and returns without unlocking | Extends: the same discipline joins a container's namespaces before one process start. The shared rule (never unlock) moves with it |
| Upstream second-process path, refused today | The runtime library `newSetnsProcess`, `setnsProcess.start`, `linuxSetnsInit.Init`; `spec-container-1-runtime.md`, "The diff against the upstream release", hunk H-3 | With cgo, starts a process in a container. The runtime child's H-3 makes `newSetnsProcess` refuse, with an error naming the missing join, and names this child as its owner | Replaces H-3: the refusal is removed and the path works (A1). Under A3 the refusal stays and Ze starts the process itself |
| The runtime child's patch series | `spec-container-1-runtime.md`: one series against runc v1.5.3, one patch file per hunk under the vendor patch directory, stamped with the reviewed version, with a named guard test | Six hunks over five upstream files, 60 to 90 changed lines. `libcontainer/container_linux.go` and `libcontainer/process_linux.go` are two of the five | Extends: this child's hunks add to those two files and to no other upstream file. They are patch files of the same series, under the same stamp and the same bump gate |
| The filter's registration seam | `spec-container-1-runtime.md`, hunk H-6 | The patch makes a library stub call a function a Go package registers, so the filter is Ze code in the plugin, where Ze's gates judge it | Extends by the same shape: the hunk in `setnsProcess.start` calls a registered function around the process start, and the join is Ze code in the plugin, never a file inside the patch |
| Early role dispatch | `spec-container-1-runtime.md`: an always-on leaf under `internal/core`, called first in `cmd/ze/dispatch.go` `dispatchMain`, selects the container init role from the library's environment key | One role: the library's init, for a container's first process | Extends: the same key is set for a second process, with the library's init type saying which. The plugin's role function does the child's first steps (non-dumpable, the mount namespace join) before it calls the library's init |
| Container supervision | `spec-container-1-runtime.md`: one supervisor worker per container owns the library's container handle and its lifecycle states. A container is killed through its cgroup. The daemon stop reaches containers in the stop-drain call. A leftover sweep runs at plugin start | Starts, stops and restarts a container | Extends: the session handler asks the container's worker for a second process while the state is `running`. One session process is ended by its own process descriptor, because a cgroup kill would end the container. A container stop kills the cgroup, which holds the session's process too, so no session outlives its container. The plugin's drain handler ends sessions before it stops containers |
| Doctor checks | `internal/core/diagnostic/doctor_registry.go` `RegisterDoctorCheck`; the plugin's `DoctorChecks` and `OnDoctorCheck` (`ai/rules/plugins.md`) | A plugin answers a runtime check over its own live state | Extends: one runtime check, answered by the container plugin, for a session process that outlived its session |
| Record streaming on one-shot surfaces | `plan/post-release/features/spec-streaming-answer-protocol.md` (skeleton) | Plans record streaming for REST, gRPC, web and MCP answers | Does not apply: it carries output records of one answer and no input. It touches the same adapters, so Blast Radius names it. Its scope cannot take a session, so this work is not an amendment of it |

## Data Flow (MANDATORY - see `ai/rules/architecture.md`)

### Entry Point
- A command line on any surface: `request container shell name <name>`, with the optional leaves `command`, `input` and `timeout` (O-2).
- On a terminal surface, bytes follow: keys from the operator, and terminal events (size, signals).
- On gRPC, a stream of frames: open, input, resize, signal, end of input.

### Transformation Path
1. The surface adapter asks the interactive registry whether the line is an interactive command. It asks a registry and never compares a name.
2. The session route starts accounting, authorizes the line as a state-changing command, and validates the words after the prefix against the model's leaves.
3. The route builds a session request: the validated arguments, the caller, the surface, and either a terminal (type, columns, rows) or none.
4. On a one-shot surface the line reaches `Dispatcher.Dispatch` as any command does, and the command's handler is the bounded run the engine derived: no terminal, the `input` leaf as the whole input, then end of input.
5. The route opens the session with the owner: a Go call for a daemon builtin, the bridge for a compiled-in plugin, and the open message for a plugin that runs as a separate process. The owner answers with the limits it holds (idle, absolute) or with a refusal.
6. The container plugin checks the container's state and its session count, then starts the process: it asks the runtime library for a second process in that container, with a console socket when a terminal was requested and with pipes when not.
7. Under A1 the daemon side of the library, changed by this child's hunks, calls the plugin's registered function, which runs the start on a throwaway locked thread that first joins the container's network, ipc, uts and cgroup namespaces and sets the pid namespace for its children, from a process descriptor of the container's init. The child is created directly inside the container's cgroup, which the runtime child's H-1 created.
8. The child is the `ze` binary. The early role dispatch of the runtime child sees the library's key, and the library's init type says this is a second process. Before anything else the role makes the process non-dumpable, stops sharing filesystem attributes on its locked main thread, joins the mount namespace from a descriptor the patched parent passed under its own environment key, closes that descriptor, and calls the library's init.
9. The library's second-process init allocates the terminal inside the container and sends its descriptor back, sets no-new-privileges, reports ready, installs the system-call filter, drops capabilities, sets the user and the working directory, verifies the working directory, closes every descriptor it does not pass on, and executes the program.
10. The plugin receives the terminal descriptor, verifies that it is a terminal master, and relays bytes between it and the session: output chunks toward the engine, input chunks from it, a resize as a terminal ioctl, a signal to the process.
11. The engine relays between the owner and the surface, counts bytes, and enforces the idle limit, the absolute limit and, for a bounded run, the time bound and the size bound.
12. The process exits, or a limit, a disconnect, a container stop or a daemon stop ends the session. The owner ends the process (hang-up through the terminal, then a kill by process descriptor after a grace), reports the exit status and the reason, and the route records the end, stops accounting and answers the surface. A daemon stop or restart reaches the plugin through the stop-drain call, where it ends every session and then stops every container. Ending the sessions is this child's step of the drain: a hang-up through the terminal, then a kill by process descriptor after a grace of 2 seconds. The runtime child states the arithmetic of the one drain budget, which holds those 2 seconds before the containers' stop (its Key Design Decisions).

### This child's hunks (A1), against what the runtime child specifies

| Hunk | Upstream function or file | Change | Relation to the runtime child's series |
|------|---------------------------|--------|-----------------------------------------|
| H-3 (rewritten) | `libcontainer/container_linux.go` `newSetnsProcess` | Builds no bootstrap data. Records the container's init process and passes one more descriptor, for the mount namespace, under an environment key of its own | Replaces the runtime child's H-3, which made this function refuse. The patch file of H-3 is rewritten, not added beside |
| H-7 | `libcontainer/process_linux.go` `setnsProcess.start`, `execSetns` | The process start goes through a function a Go package registers, and refuses when none registered. The write of the bootstrap data, the wait for the first stage and the read of the pid reply are removed, and the started process is the session's process | A seventh patch file of the series. The same file holds H-2 |

The series becomes seven patch files over the same five upstream files. The hunk ids follow the runtime child's numbering of patch files: H-3 (rewritten) is its third file with new content, and H-7 is the new seventh file. The estimate for H-3 (rewritten) and H-7 is 30 to 50 changed lines, not counted against a build. The join, the terminal handling and the child's first steps are Ze code in the plugin.

### The fallback entry design (A3), kept ready

A3 becomes the design when the probe for A-8 fails, or when A-1, A-3 or A-5 breaks in a way A1 cannot absorb. What differs from A1:

| Item | Under A3 |
|------|----------|
| The patch series | H-3 is not rewritten and H-7 is not written. The runtime child's H-3 stays as it is: the library refuses a second process |
| Who starts the process | The plugin starts a helper: the `ze` binary in a role of its own, selected by an environment key of Ze's through the same early role dispatch, from the library's sealed copy of the binary, created inside the container's cgroup |
| The helper's steps, in order | 1. Make itself non-dumpable. 2. Lock one thread and never unlock it. 3. Stop sharing filesystem attributes. 4. Join every namespace of the container's init by process descriptor, the mount namespace included, and set the pid namespace for children. 5. When a terminal was asked for, open the container's pts handle without following a link, verify it, take the peer from the descriptor, and send the master to the plugin. 6. Change to the working directory and verify it is inside the root. 7. Set no-new-privileges, drop the capability bounding set to the container's, and load the container's filter on that thread. 8. Start the program from that thread as a child of the daemon, with the container's user and groups, a new session, the terminal as its controlling terminal, and only the descriptors it is given. 9. Report the program's process id and exit |
| What the program inherits | The thread's namespaces, root, working directory, bounding set, no-new-privileges bit and filter. Its capability sets are what the kernel computes at exec from the bounding set and the user |
| What Ze owns that A1 inherits | The order of steps 5 to 8, which is the library's `linuxSetnsInit.Init` and `finalizeNamespace` rewritten. Each upstream fix to those two functions is read at every version bump, and the bump gate lists them |
| Known gaps to close at the switch | The filter must allow the calls the start itself makes after step 7. A container whose user is not root and whose config grants capabilities needs them as ambient capabilities at the start. Both are checked against the default profile and the fourteen default capabilities before code is written |
| What does not change | The session mechanism, the commands, the limits, every refusal but O-7's, every acceptance criterion and every test but two. `TestNoDaemonThreadStaysInContainerNamespaces` then runs against the helper. O-7's refusal is removed with its test, `TestShellRefusesTraceCapableContainer`, because no host process image is visible inside the container |
| Files | `shell_join_linux.go` holds the helper's role in place of the daemon-thread join. No patch file is added |

### Session messages (the process protocol addition)

| Message | Direction | Fields | Meaning |
|---------|-----------|--------|---------|
| `interactive-open` | Engine to plugin | session id, command, validated arguments, caller name, surface, terminal (type, columns, rows) or absent | Start a session. The answer carries the idle and absolute limits, or a refusal with its fix line |
| `interactive-input` | Engine to plugin | session id, bytes (base64), end-of-input flag | Input for the session, in order. Acknowledged, and the engine sends the next chunk after the acknowledgment |
| `interactive-resize` | Engine to plugin | session id, columns, rows | The terminal changed size |
| `interactive-signal` | Engine to plugin | session id, signal name from a closed set | Deliver a signal to the session's process |
| `interactive-close` | Engine to plugin | session id, reason from a closed set | End the session and its process |
| `interactive-output` | Plugin to engine | session id, stream (terminal, standard output, standard error), bytes (base64) | Output, in order for each stream. Acknowledged, and the plugin sends the next chunk after the acknowledgment |
| `interactive-exit` | Plugin to engine | session id, exit status or terminating signal, reason from a closed set | The session ended. No message for that id follows |

### Bounded answer (one-shot surfaces)

| Field | Type | Description |
|-------|------|-------------|
| `exit-status` | integer | The program's exit status. Absent when it did not exit |
| `signal` | string | The signal that ended the program. Absent when it exited |
| `stdout` | string | Standard output, up to the size bound |
| `stderr` | string | Standard error, up to the size bound |
| `truncated` | boolean | True when either stream passed the size bound |
| `timed-out` | boolean | True when the time bound ended the run |
| `duration` | string | How long the run took |

### What each surface carries, and its form

| Surface | It carries (read at source) | Form of an interactive command |
|---------|-----------------------------|--------------------------------|
| SSH exec with a terminal request | A terminal, window changes, signals, a duplex channel, an exit status | The live session with a terminal |
| SSH exec without a terminal | A duplex channel, end of input, an exit status | The live session without a terminal: input is piped, the two output streams go to the channel's output and error |
| SSH interactive session, and the `ze cli` shell it carries | A terminal owned by the daemon's model | The model releases the terminal to the session and restores it at the end |
| `ze cli -c` and `ze request ...` | The SSH exec channel | The client requests a terminal when its own input is a terminal and the command is interactive; otherwise the form without a terminal |
| gRPC | A unary call and a server stream today | A new method with a stream in both directions for the live session; `Execute` answers the bounded form |
| REST | A request and an answer; a server event stream | The bounded form on `POST /api/v1/execute` (O-5) |
| Web admin console and web terminal | One posted line and one answer; a server event stream | The bounded form (O-4) |
| MCP | A tool call and its result; a task that is polled | The bounded form as the tool's result, with `command`, `input` and `timeout` as typed parameters |

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Surface to engine | The interactive registry, then the session route with accounting and authorization | No |
| Engine to plugin | The seven session messages over the process protocol; the bridge internal | No |
| Plugin to runtime library | The library's second-process call with a console socket or pipes | No |
| Daemon to second-process role | The library's startup protocol over inherited descriptors, plus one descriptor for the mount namespace | No |
| Host to container | Namespace joins by process descriptor; the cgroup at creation; the terminal descriptor passed back over the console socket | No |
| gRPC client to daemon | A new bidirectional method in `api/proto/ze.proto` | No |

### Changed-Wire Census

**Search evidence:** searched `pkg/plugin/rpc`, `pkg/plugin/sdk`, `internal/core/ipc/yang` and `docs/architecture/api/process-protocol.md` for `stream`, `chunk` and `interactive`: the only streamed form is the command answer (head, records, terminator) and no message carries bytes toward a plugin for a running command. Searched `api/proto/ze.proto` for `stream`: one server stream (`Stream`), no client stream. Searched `go.mod` for a WebSocket module: none. Searched `internal/component/web/assets` for a terminal emulator: none. The positive control is `execute-command`, which the same search finds in the protocol page, in `ze-plugin-callback.yang` and in `pkg/plugin/rpc/types.go` (`ExecuteCommandInput`). No existing message or field changes: every row below is an addition. The REST, web and MCP wires carry the bounded answer through their existing envelopes and do not change.

| Wire | Role (writer / reader) | Carrier (file + producer/consumer symbol or script entry) | Source evidence | Migration or preservation disposition |
|------|------------------------|-----------------------------------------------------------|-----------------|----------------------------------------|
| Session messages, engine to plugin | writer | The engine's session route, new, in `internal/component/plugin/server`; declared in `internal/core/ipc/yang/ze-plugin-callback.yang` | No message of these names exists in the tree; `execute-command` is the nearest and carries no bytes after the request | New. Existing messages are preserved unchanged |
| Session messages, engine to plugin | reader | `pkg/plugin/sdk` (one new callback) for a Go plugin; any third-party plugin in another language, outside the repository | The SDK callbacks are one `On*` method each (`sdk_callbacks.go` `OnExecuteCommand`) | New. A plugin that declares no interactive command never receives one. A third-party plugin reads the protocol page |
| Session messages, plugin to engine | writer | `pkg/plugin/sdk`; any third-party plugin, outside the repository | No message of these names exists | New |
| Session messages, plugin to engine | reader | The engine's session route, through two new entries of `engineOps` (`dispatch_registry.go`); declared in `internal/core/ipc/yang/ze-plugin-engine.yang` | The registry holds one entry per operation, and both transports derive from it | New: two entries |
| Stage 1 command declaration, interactive field | writer | `pkg/plugin/rpc/types.go` `CommandDecl` through `pkg/plugin/sdk`; any third-party plugin | The declaration carries the name, help, arguments and answer shape today, each optional field with `omitempty` | New optional field. A declaration without it is unchanged |
| Stage 1 command declaration, interactive field | reader | The engine's Stage 1 validation, `internal/component/plugin/server/startup.go`, beside `validateHelpDecls` | Declared text is judged there before it is stored | New check: an interactive declaration without a one-shot form is refused |
| gRPC session method | writer | A gRPC client, outside the repository; `ze` holds no gRPC client of its own that needs it | `api/proto/ze.proto` has no client stream | New method. `Execute` and `Stream` are preserved |
| gRPC session method | reader | `internal/component/api/grpc/server.go`, a new method of `zeServiceImpl` | The service implements `Stream` as a loop over a channel today | New |
| SSH exec channel for an interactive command | writer | An SSH client: `internal/component/cli/sshclient` and any standard client | The exec channel already carries a raw duplex for the plugin protocol session | No format change. The bytes are the program's own; the terminal request and window changes are the SSH library's |
| SSH exec channel for an interactive command | reader | `internal/component/ssh/ssh.go` `execMiddleware` | It branches on the streaming registry today | New branch on the interactive registry |
| Runtime library startup protocol, second-process role | writer | The upstream library's parent side (`setnsProcess.start`), inside the `ze` binary, as changed by the build-time diff | Read at tag v1.5.3 | Both ends are the same build. Under A1 the bootstrap data and the pid reply are removed from this path, as the umbrella removed them from the create path |
| Runtime library startup protocol, second-process role | reader | The upstream library's init side (`linuxSetnsInit.Init`), inside the `ze` binary | Read at tag v1.5.3 | The same build as the writer, so no version skew exists |
| Role key and init type of the library (`_LIBCONTAINER_INITPIPE`, `_LIBCONTAINER_INITTYPE`), plus one new key for the mount namespace descriptor | writer | The vendored library's parent side, with the new key written by H-3 (rewritten) | Read at v1.5.3: `newSetnsProcess` sets the init type to the second-process value. The runtime child's census lists the role key | The two existing keys are preserved. The mount namespace key is new and lives only between the two ends of one build |
| Role key and init type of the library (`_LIBCONTAINER_INITPIPE`, `_LIBCONTAINER_INITTYPE`), plus one new key for the mount namespace descriptor | reader | The runtime child's early role leaf, then the plugin's role function, then the library's init | No reader of the init type or of the new key exists in the tree | New reader: the role function reads the init type and the new key before it calls the library's init |
| Audit entry actions | writer | The session route, through the dispatcher's recorder | `internal/core/audit/audit.go` holds a closed set of action constants | Two new constants. Existing entries are unchanged |
| Audit entry actions | reader | `show audit` and any reader of the audit file, outside the repository | The entry's fields are unchanged | New values of an existing field |

### Integration Points
- `command.AnswerSurface` - one new registration field and one new offer.
- `pluginserver` streaming route - the session route sits beside it and shares accounting, authorization and argument validation.
- `Server.OnStop` - one hook ends every session at daemon stop.
- `ssh.Server` - one new executor factory, as the streaming executor factory is set today.
- `api.APIEngine` - one new method beside `Execute` and `Stream`.
- The Bubble Tea model - one message that releases the terminal to a session and one that restores it.
- The container plugin - one interactive registration, the second-process start, the relay, the session table, one doctor check.
- The build-time diff of `spec-container-1-runtime` - new hunks in the second-process path (A1).

### Architectural Verification
| Check | Holds? | Evidence |
|-------|--------|----------|
| No bypassed layers (data flows through the intended path) | No | |
| No unintended coupling (components stay isolated) | No | |
| No duplicated functionality (extends existing, does not recreate) | No | |
| Zero-copy preserved where applicable (refs, not copies) | No | |
| Registration over hardcoding, outbound: new commands, views, families, and handlers register, and the core discovers them. No per-feature field, switch case, or factory is added to a core/shared package (`ai/rules/plugins.md`) | No | |
| Registration over hardcoding, inbound: no existing switch, seed map, validator, parser, runner, help string, or completion table has to learn this feature's name. Evidence names every list that was searched for the names this feature introduces, and the registry each one now derives from (`ai/rules/principles.md`) | No | |

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis (file/doc/user statement) | If wrong | Validated by | Status |
|----|-----------|--------------------------------|----------|--------------|--------|
| A-1 | A thread of a multithreaded process may join a network, ipc, uts and cgroup namespace and set the pid namespace of its children | Read: `setns(2)` on this machine restricts only the user and mount namespaces by thread, and says a pid join changes only the namespace of children created afterward | A1 fails; A2 has the same need and fails too; A3 and A4 remain | A root probe: a Go program whose locked thread joins those five namespaces of a child it created, starts a process, and that process reports the same namespace identifiers as the target and process id 2 or higher inside it | unvalidated |
| A-2 | One call joins several namespaces of a target at once through a process descriptor | Read: `setns(2)`, "fd is a PID file descriptor", since Linux 5.8, with the namespace types combined in one mask | The join opens each namespace link and joins one at a time, as the C code does. Slower and open to a recycled process id | The same probe, with the single call | unvalidated |
| A-3 | A child started by Go from a locked thread takes that thread's namespaces and filesystem attributes | Read: `syscall/exec_linux.go` builds the clone flags without the filesystem-sharing flag. Recalled, not read: the kernel copies namespaces from the calling thread | The child is born in the daemon's namespaces and A1 does not work as designed | The same probe: the child reports its namespace identifiers | unvalidated |
| A-4 | The Go process start runs on the calling goroutine's thread, and the runtime never creates a thread from a locked thread | Read: `runtime/proc.go` `LockOSThread` starts the template thread for that purpose. Not read: the call chain from `os/exec` to the clone | A runtime thread could inherit the container's namespaces | The same probe, run in a loop under load, listing the namespace identifiers of every thread of the daemon afterward: all but the dead thread are the daemon's | unvalidated |
| A-5 | A thread that stopped sharing filesystem attributes may join a mount namespace in a multithreaded process, and the join moves its root and working directory into that namespace | Read: `setns(2)` forbids the join only while filesystem attributes are shared. Recalled: the root and working directory are reset to the namespace's root. `mount_namespaces(7)` on this machine says nothing on either point | The child cannot enter the mount namespace; only A4 remains, or a design where the daemon's thread joins it and the child is found through the container's own process filesystem | A root probe: a Go program locks its main thread, unshares filesystem attributes, joins the mount namespace of a container-like child, and lists its root | unvalidated |
| A-6 | The final exec from the locked thread carries that thread's credentials, capabilities, no-new-privileges bit, filter, root and working directory, and ends every other thread | Basis: the library's standard init already relies on this in the approved create path | Confinement is not applied to the program | AC-8 through the real path | unvalidated |
| A-7 | Go's clone-into-cgroup works together with a start from a thread in the container's namespaces | Read: `syscall/exec_linux.go` uses the newer clone call when the cgroup descriptor is set, and `clone(2)` on this machine describes the flag (Linux 5.7, version 2 cgroups). The library retries without it on failure | The process starts in the daemon's cgroup and is moved after, as the library's fallback does. Its start-up memory is charged to the daemon | The probe of A-1 with a cgroup descriptor, reading the child's cgroup file | unvalidated |
| A-8 | A process whose permitted capabilities are not a subset of the tracer's cannot be traced by it, unless the tracer holds the trace capability. A plain exec makes the new image dumpable again | Both recalled; the passages were not found by the search run on this machine. Read: `prctl(2)` says a non-dumpable process cannot be attached, and lists when the attribute is reset | R-2's window is open to every container, and O-7's refusal is not enough: A3 becomes the recommendation | A root probe: a process with the container's default capability set tries to attach to a full-capability process in its pid namespace and is refused | unvalidated |
| A-9 | The vendored SSH library delivers the terminal request, window changes and signals for an exec channel, and Bubble Tea's release and restore let another reader own the session's terminal | Read: the session interface declares `Pty`, `Signals` and `Break`; the program type declares `ReleaseTerminal` and `RestoreTerminal`. Their behavior over an SSH session was not read | The interactive `ze cli` hand-off needs another route, such as ending the model and starting a new one | A unit test over an in-memory SSH session, then `container-shell-from-cli-session.ci` | unvalidated |
| A-10 | The container always has its own pts instance | Basis: the umbrella's default mounts list a pts mount with a new instance | A terminal cannot be allocated in a container whose config removes that mount | The plugin checks at open and refuses with the fix line; a session without a terminal still works. AC-17 | unvalidated |
| A-11 | The library's second-process init needs no path that resolves in the host mount namespace after the child joined the container's | Read: `linuxSetnsInit.Init` opens the pts handle, looks the program up and writes process attributes, all inside the container, as upstream does after its C join | A step fails after the join | AC-8 and AC-1 through the real path | unvalidated |
| A-12 | The runtime child keeps a handle to each running container from which a second process can be started, and its patch series can take more hunks | Read in `spec-container-1-runtime.md` (a draft, not approved): one supervisor worker per container owns the library's handle; the series is one patch file per hunk; H-3 refuses `newSetnsProcess` and names this child as the owner of the join | This spec's hunks and its seam to the worker move | Settled by that spec as drafted. Read again if its hunk table or its supervisor changes before this child starts | confirmed against the draft |
| A-13 | The library's sealed copy of the binary and its close-on-exec sweep run for a second process as they do for init | Read at v1.5.3: both sit in the shared `newParentProcess` and `Container.start` path the umbrella describes. Not traced for the second-process call | A shell would run the unsealed binary, and R-1's first defense is missing | `TestShellRunsFromASealedBinary`, the twin of the runtime child's init test, in the guest | unvalidated |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | A host process enters a container an attacker controls. Upstream's history for this path: the runtime binary overwritten through the process image of the entering process (CVE-2019-5736), a leaked descriptor used as the working directory of an entering process (CVE-2024-21626), console and process-attribute redirection (CVE-2025-52565, CVE-2025-52881), a `/dev` link (CVE-2026-41579) | A test container replaces `/dev/pts/ptmx` with a link, or sets the working directory to a descriptor path, and the shell starts | A1 and A2 inherit the library's defenses unchanged: the sealed binary, the close-on-exec sweep, the working directory check, the verified pts handle. One test per defense through the shell path |
| R-2 | Under A1 and A2 the entering process is a new `ze` image inside the container's pid namespace whose secondary threads are still in the host mount namespace, until its final exec. Upstream joins before the runtime starts, so all of its threads are inside. A container process that can trace it controls a host-root process | A container granted the trace capability traces the entering process in a test | The process makes itself non-dumpable first; the default capability set cannot trace it (A-8); O-7 refuses the shell for a container that holds the trace capability. Fallback: A3, where no host image is ever visible inside the container |
| R-3 | A daemon thread that joined a container's namespaces returns to the scheduler and runs other goroutines there | A daemon goroutine sees the container's network | The thread is never unlocked and dies with its goroutine, as `InThrowawayNetworkNamespace` does. A test lists the namespaces of every daemon thread after 100 shells |
| R-4 | The plugin trusts a descriptor a process inside the container sent as the terminal | A unit test sends a regular file or a socket | The plugin verifies the descriptor is a terminal master before the first read or ioctl, and refuses the session otherwise |
| R-5 | A session's process outlives its session: the operator disconnected, the kill failed, or the program moved to its own session | `show container session list` is empty while the process runs | Hang-up through the terminal, then a kill by process descriptor after a grace. A doctor check reports a process that survived, with the restart command. A program the operator detached on purpose stays the container's own process, under its cgroup limits |
| R-6 | One session floods the plugin's connection and starves its other calls, or a slow operator blocks the plugin | A command to the container plugin stalls during a large output | Each chunk is acknowledged before the next is sent, the chunk size is bounded, and the engine drops the session, not the connection, when the surface stops reading |
| R-7 | A bounded run returns megabytes or never ends | An MCP call hangs or returns a huge result | The time bound and the size bound are mandatory and enforced by the engine; the answer says which one fired |
| R-8 | An upstream release changes the second-process path the diff touches. That path carries most of the churn of its file | A shell fails after a version bump | The runtime child's review stamp refuses a bump until the patched functions are read again, and its bump gate gains `container-shell-ssh-terminal` and `container-shell-hostile-container`. H-3 (rewritten) and H-7 are small, and the join is Ze code behind a registered function, outside the patch |
| R-9 | The owner's authorization for running an external program covers a container's entrypoint, and a program an operator names for a shell is a second use | A review reads the register row | The owner authorised the second use on 2026-11-16: OA-1, in the table "Owner authorisation for the shell's program" under Task. The register row lands in the commit that adds the code of phase 6, citing that ruling |
| R-10 | Three surfaces offer a live session and the web and REST forms differ from them, so an operator expects a terminal where there is none | An operator types the bare shell command in the web terminal | The refusal on a one-shot surface names the leaf to add and the surfaces that carry a terminal |
| R-11 | `ze cli -c` cannot know that a command a plugin declared at run time is interactive, so it sends no terminal request and the operator gets a shell with no prompt | An external plugin's interactive command run with `ze cli -c` | The client reads the registry for commands the binary carries, as it reads the streaming registry today. For any other path the daemon answers an exec without a terminal by running the form without a terminal, which is correct for a script; the `ze cli` shell and `ssh -t` give the terminal. Recorded in Known Limitations |
| R-12 | Each shell seals the `ze` binary again and costs its size in memory when the zero-copy sealing is unavailable | Memory rises by the binary's size at each shell | The umbrella's R-11 settles whether the zero-copy sealing works on the appliance; the session caps bound the worst case |
| R-13 | A session is left open by a client that vanished without closing | Sessions accumulate to the cap | The idle limit, the absolute limit, and the surface's own disconnect all end it |

## Blast Radius

| Question | Answer |
|----------|--------|
| What breaks if this is wrong? | The worst case is a container escape through the entering process (R-1, R-2). Next: a daemon thread left in a container's namespaces (R-3), a plugin connection stalled by a session (R-6), or a process left running in a container (R-5). No routing behavior is on this path |
| How is it reverted? | The container plugin's registration is one commit: without it no interactive command exists and the session mechanism is idle. The protocol messages and the gRPC method are additions, and no existing caller depends on them |
| Who else touches this path? | `spec-container-1-runtime` owns the diff and the plugin's supervisor. `spec-container-4-operator-surface` owns the command schema and the config root. `spec-streaming-answer-protocol.md` plans changes in the same REST, gRPC, web and MCP adapters. `plan/release/fixes/spec-pipe-chain-honored-on-every-command-surface.md` changes how those surfaces split a pipe chain |

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| SSH exec with a terminal, `request container shell name web` | → | `execMiddleware` interactive branch, the session route, the container plugin's session handler, the library's second-process start | `test/container/container-shell-ssh-terminal.ci` |
| SSH exec without a terminal, input piped | → | The same route with no terminal | `test/container/container-shell-ssh-piped.ci` |
| A `ze cli` session, the same line typed at the prompt | → | The model's release and restore of the terminal | `test/container/container-shell-from-cli-session.ci` |
| MCP `tools/call` for the command with `command` | → | `Dispatcher.Dispatch`, the derived bounded run | `test/container/container-shell-bounded-mcp.ci` |
| REST `POST /api/v1/execute` | → | The same bounded run | `test/container/container-shell-bounded-rest.ci` |
| gRPC session method | → | `APIEngine` session method, the session route | `TestGRPCSessionCarriesInputAndExit` |
| A plugin running as a separate process declares an interactive command | → | The seven session messages, the SDK callback | `test/plugin/interactive-command-external-plugin.ci` |
| Every registered interactive command, every surface | → | `command.AnswerSurface` | `TestEveryCommandOnEverySurface` |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior | Permitted Survivors and Why |
|-------|-------------------|-------------------|-----------------------------|
| AC-1 | An operator runs the shell command over SSH with a terminal, into a running container | A prompt appears. A typed command prints its output. After the client's window changes size, `stty size` inside prints the new size. `exit 3` ends the SSH command with status 3 | A program that ignores the hang-up keeps running in the container when the operator detached it on purpose, because it is the container's own process (R-5) |
| AC-2 | The same line is typed at the prompt of a `ze cli` session | The shell takes the terminal. When it exits, the CLI prompt returns with its screen restored and a line stating the exit status | The CLI history holds the command line and nothing typed inside the shell, because those bytes never reach the model |
| AC-3 | The command runs over SSH without a terminal, with input piped | The program reads the piped input to its end, its standard output and standard error arrive on the channel's output and error, and the SSH command ends with the program's exit status | No prompt and no echo appear, because no terminal was requested |
| AC-4 | The command with `command "cat /etc/hostname"` is run through MCP, REST, gRPC `Execute`, the web console, and `ze cli -c ... | json` | Each answers the same record: `exit-status` 0, `stdout` holding the container's host name, an empty `stderr`, `truncated` false, `timed-out` false | The `duration` value differs between runs, because it is measured |
| AC-5 | A bounded run of a program that never exits, with `timeout 2` | The answer arrives after two seconds with `timed-out` true and the output so far, and the program is no longer in the container's process list | The container and its other processes keep running, because only the session's process is ended |
| AC-6 | A bounded run whose program writes 4 MiB | The answer holds the first 1 MiB of that stream and `truncated` true, and the program is ended | The exit status may be absent with a `signal`, because the engine ended the program |
| AC-7 | A gRPC client opens the session method, sends an open frame with a terminal, input frames, a resize frame and end of input | Output frames carry the program's bytes, the resize is visible inside, and the last frame carries the exit status | A client that sends no terminal gets the two output streams marked apart, because that is the form without a terminal |
| AC-8 | A shell is open in a container with the default confinement | Inside the shell: every namespace identifier equals the container init's, the cgroup is the container's, the capability sets equal the container's configured sets, the no-new-privileges bit is set, the filter mode is on, a system call outside the profile is refused, and the working directory is inside the container's root | A capability or namespace the container's config grants is present in the shell too, because the shell takes the container's confinement and adds none |
| AC-9 | An operator whose profile denies the command runs it; another whose profile allows it opens and closes a shell | The first reads the standard unauthorized answer, and accounting and the audit log each hold a record with outcome denied. The second produces a session-start and a session-end audit entry with the user, the remote address, the surface, the command line, the exit status, the duration and the byte counts | Nothing typed inside the shell is in the audit log, under the recommended answer to O-6 |
| AC-10 | A shell receives no input and produces no output for the idle limit | The session ends, the operator reads a line naming the limit and the config leaf that changes it, and the process is gone | A session that keeps producing output stays open until the absolute limit, because output is activity |
| AC-11 | A fifth session is opened into a container that holds four | The open is refused with a line stating the cap, and naming the session list command and the command that closes one | The four open sessions continue, because a refusal changes nothing |
| AC-12 | `show container session list` while two shells are open; then one is closed by its id | The list holds one row per session with its id, container, user, surface, remote address, start time and idle time. After the close, that session's operator reads that an administrator closed it, and the list holds one row | The other session continues |
| AC-13 | The container restarts or is stopped while a shell is open | The session ends, and the operator reads that the container stopped, with an exit status distinct from a program's | The container's restart proceeds on its own schedule, because a session never holds a container |
| AC-14 | A config reload that does not touch the container; then a daemon stop or restart | The session continues across the reload. At the stop or restart, the session ends in the stop-drain step with a line saying the daemon is stopping, before its container is stopped, and no process of it remains. After a restart the container runs again and holds no session | A reload that removes or restarts the container ends the session, as AC-13 states. A daemon that crashed gives no line: the next start's sweep removes the container and its session process together, as the runtime child specifies |
| AC-15 | The shell command into a container whose image holds no `/bin/sh`; then with `command` naming a program that is absent | Each is refused before any session exists, naming the program that is missing and the fix: name a program the image holds with `command` | A bounded run of a program the image does hold succeeds in that container, because no shell is involved (O-3) |
| AC-16 | The shell command into a container that is stopped or waiting for its image | The command is refused with the container's state and the command that starts it or fetches its image | A container that is running accepts the session |
| AC-17 | A terminal is requested for a container whose config removed its pts mount | The session is refused with a line naming the mount and stating that the form without a terminal works | The form without a terminal runs in that container |
| AC-18 | A test plugin that runs as a separate process declares an interactive command that echoes its input | Over SSH and over the bounded form, the command behaves as the same plugin does when it runs in the daemon: the echoed bytes and the exit status are equal | Timing differs, because the bytes cross a connection |
| AC-19 | A plugin declares an interactive command with no one-shot form | Its Stage 1 declaration is refused with a line naming the command and the missing declaration | A declaration with the form is accepted |
| AC-20 | A build without the container gate | The shell command is unknown on every surface, the session mechanism holds no registration, and the surface parity test passes | Every other command behaves as in a build with the gate |
| AC-21 | The process inside the container sends a descriptor that is not a terminal master as the terminal | The plugin refuses the session, ends the process, and logs a warning with its doctor code | A real terminal master is accepted |
| AC-22 | A session's process is still alive after the kill | `show doctor` reports it under the container plugin's runtime check, with the container name, the process id and the command that restarts the container | A process the session never started is not reported, because the check reads the plugin's own session table |

## End-to-End User Stories

| # | User does | Path through system | Test proving it works |
|---|-----------|--------------------|-----------------------|
| 1 | Opens a shell in a container from an SSH client and works in it | SSH exec with a terminal, interactive registry, session route, container plugin, library second-process start, terminal relay | `test/container/container-shell-ssh-terminal.ci` |
| 2 | Opens a shell from the `ze cli` prompt and returns to the prompt | The daemon's model releases and restores the terminal | `test/container/container-shell-from-cli-session.ci` |
| 3 | Runs one program in a container from a script | SSH exec without a terminal, piped input, exit status | `test/container/container-shell-ssh-piped.ci` |
| 4 | An AI agent debugs a container through MCP | `tools/call`, dispatcher, bounded run, record answer | `test/container/container-shell-bounded-mcp.ci` |
| 5 | An automation system runs one program through REST or gRPC | `POST /api/v1/execute` or `Execute`, the same bounded run | `test/container/container-shell-bounded-rest.ci` |
| 6 | An administrator lists who is inside which container and closes one session | `show container session list`, the close command, the plugin's session table | `test/container/container-shell-session-list-and-close.ci` |
| 7 | A third-party plugin author offers a command that takes input | Stage 1 declaration, the seven messages, the SDK callback | `test/plugin/interactive-command-external-plugin.ci` |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestAnswerSurfaceInteractive` | `internal/component/command/surface_test.go` | A terminal surface owes the live session and a one-shot surface owes the bounded form, for every surface | |
| `TestRegisterInteractiveRequiresOneShotForm` | `internal/component/plugin/server/interactive_test.go` | A registration with no declared one-shot form is refused | |
| `TestSessionRouteAuthorizesAsStateChanging` | `internal/component/plugin/server/interactive_test.go` | The route authorizes with the read-only flag clear and accounts for a refused command | |
| `TestBoundedRunTimeBound` | `internal/component/plugin/server/interactive_bounded_test.go` | The run ends at the bound and answers `timed-out` | |
| `TestBoundedRunSizeBound` | `internal/component/plugin/server/interactive_bounded_test.go` | Each stream stops at the bound and the answer says `truncated` | |
| `TestSessionIdleAndAbsoluteLimits` | `internal/component/plugin/server/interactive_limits_test.go` | Output and input both reset the idle clock; the absolute limit does not reset | |
| `TestSessionsEndAtServerStop` | `internal/component/plugin/server/interactive_stop_test.go` | The stop hook ends every session and waits for the relays | |
| `TestSessionMessagesRoundTrip` | `pkg/plugin/rpc/interactive_test.go` | Each of the seven messages encodes to one compact line and decodes to the same value | |
| `TestSessionChunksAreAcknowledgedInOrder` | `pkg/plugin/sdk/interactive_test.go` | A second chunk is not sent before the first is acknowledged, in both directions | |
| `TestStage1RefusesInteractiveWithoutOneShot` | `internal/component/plugin/server/startup_interactive_test.go` | AC-19 | |
| `TestExecMiddlewareRoutesInteractiveByRegistry` | `internal/component/ssh/interactive_test.go` | The branch is taken for a registered prefix and for no other line; the terminal request and window changes reach the session | |
| `TestModelReleasesAndRestoresTerminal` | `internal/component/cli/model_interactive_test.go` | The model yields the terminal for a session and repaints after it | |
| `TestGRPCSessionCarriesInputAndExit` | `internal/component/api/grpc/session_test.go` | AC-7 over an in-memory connection | |
| `TestTerminalDescriptorIsVerified` | `internal/plugins/container/shell_terminal_test.go` | AC-21: a regular file and a socket are refused, a terminal master is accepted | |
| `TestSessionTableCapsPerContainer` | `internal/plugins/container/shell_sessions_test.go` | AC-11 | |
| `TestShellRefusesTraceCapableContainer` | `internal/plugins/container/shell_refusal_test.go` | O-7, under A1 only: the refusal names the capability and the leaf | |
| `TestShellRunsFromASealedBinary` | `internal/plugins/container/shell_join_linux_test.go` | A-13: the entering process's executable link does not name the `ze` file on disk. Runs in the guest | |
| `TestNoDaemonThreadStaysInContainerNamespaces` | `internal/plugins/container/shell_join_linux_test.go` | R-3: after 100 joins every live thread of the test process is in its original namespaces. Needs root; skips with the reason otherwise | |

### Boundary Tests (numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| `timeout` (bounded run, seconds) | 1-600 | 600 | 0 | 601 |
| Idle limit (seconds) | 30-86400 | 86400 | 29 | 86401 |
| Absolute limit (seconds) | 60-86400 | 86400 | 59 | 86401 |
| Sessions for one container | 1-16 | 16 | 0 | 17 |
| Terminal columns | 1-1000 | 1000 | 0 | 1001 |
| Terminal rows | 1-1000 | 1000 | 0 | 1001 |
| Session chunk (bytes before encoding) | 1-16384 | 16384 | 0 | 16385 |
| Session id in the close command | 1-4294967295 | 4294967295 | 0 | 4294967296 |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `container-shell-ssh-terminal` | `test/container/container-shell-ssh-terminal.ci` | AC-1 and AC-8: a shell with a terminal, a resize, the confinement read from inside, the exit status. `option=needs-linux` | |
| `container-shell-ssh-piped` | `test/container/container-shell-ssh-piped.ci` | AC-3 | |
| `container-shell-from-cli-session` | `test/container/container-shell-from-cli-session.ci` | AC-2 | |
| `container-shell-bounded-mcp` | `test/container/container-shell-bounded-mcp.ci` | AC-4 on MCP, and AC-5 | |
| `container-shell-bounded-rest` | `test/container/container-shell-bounded-rest.ci` | AC-4 on REST and gRPC `Execute`, and AC-6 | |
| `container-shell-bounded-web` | `test/container/container-shell-bounded-web.ci` | AC-4 on the web console, and the refusal of the bare command there (R-10) | |
| `container-shell-denied` | `test/container/container-shell-denied.ci` | AC-9: the denial, the accounting record, the audit entries | |
| `container-shell-idle-limit` | `test/container/container-shell-idle-limit.ci` | AC-10, with the limit set to its minimum | |
| `container-shell-session-list-and-close` | `test/container/container-shell-session-list-and-close.ci` | AC-11 and AC-12 | |
| `container-shell-container-restart` | `test/container/container-shell-container-restart.ci` | AC-13 | |
| `container-shell-reload-and-stop` | `test/container/container-shell-reload-and-stop.ci` | AC-14 | |
| `container-shell-no-shell-binary` | `test/container/container-shell-no-shell-binary.ci` | AC-15, with an image that holds one static program | |
| `container-shell-not-running` | `test/container/container-shell-not-running.ci` | AC-16 | |
| `container-shell-no-pts` | `test/container/container-shell-no-pts.ci` | AC-17 | |
| `container-shell-hostile-container` | `test/container/container-shell-hostile-container.ci` | R-1: a container that replaced its pts handle with a link, and one whose program tries to reach a host descriptor, each fail closed | |
| `container-shell-orphan-doctor` | `test/container/container-shell-orphan-doctor.ci` | AC-22 | |
| `interactive-command-external-plugin` | `test/plugin/interactive-command-external-plugin.ci` | AC-18, through `./le mock plugin-external` | |
| `interactive-command-plugin-dies` | `test/plugin/interactive-command-plugin-dies.ci` | A plugin's process ends while it holds a session: the operator reads that the plugin stopped, and the plugin's failure policy applies as the protocol page declares | |
| `build-tag-container-absent-shell` | `cmd/ze/hub/build_tag_container_absent_test.go` (extended by `spec-container-1-runtime`'s pair) | AC-20 | |

### Lifecycle scenarios (`ai/rules/testing.md`, "Lifecycle Regressions")
| Scenario | Reached? | Proof or reason |
|----------|----------|-----------------|
| A plugin's process ends after it declared | Yes: its sessions end | `interactive-command-plugin-dies.ci`, in external mode; the internal mode has no process to end |
| A plugin exits during the stop drain or during teardown | Yes: the engine's stop hook ends every session, and the container plugin's drain handler ends its sessions before it stops containers | `container-shell-reload-and-stop.ci` and `TestSessionsEndAtServerStop` |
| A signal arrives during the stop | Yes: a session open during a second signal | `TestSessionsEndAtServerStop`, a controlled seam, because a `.ci` cannot place the signal between the hook and the join |
| A reload or commit arrives after shutdown began | No | The session route adds no reload trigger and no commit path |
| A plugin fails a startup stage, passes its restart bound, the `respawn` leaf, a crash during verify or apply, a reload's context ending during a handshake, a plugin dispatching at startup | No | A session exists only after startup ended, and the session messages are refused before the handshake ends, as state messages are |

### Interop Tests (Scope: protocol)
N-A: Scope is plugin. No protocol Ze implements changes. The SSH requests are the vendored library's, and `container-shell-ssh-terminal.ci` drives them through Ze's own client.

## Files to Modify
- `internal/component/command/surface.go` - the interactive registration field and its offer
- `internal/component/plugin/server/handler.go` - nothing moves; the interactive registry is a sibling file, and this file's comment on `StreamingHandler` names it
- `internal/component/plugin/server/startup.go` - the Stage 1 check for an interactive declaration
- `internal/component/plugin/server/dispatch_registry.go` - two engine-op entries for the plugin-to-engine messages
- `internal/component/plugin/server/command.go` - no step of `Dispatcher.Dispatch` changes; the file is listed because the bounded run is reached through it and its audit record is the session's
- `internal/core/ipc/yang/ze-plugin-callback.yang`, `internal/core/ipc/yang/ze-plugin-engine.yang` - the seven methods
- `pkg/plugin/rpc/types.go`, `pkg/plugin/rpc/bridge.go` - the declaration field and the bridge slot
- `pkg/plugin/sdk/sdk_callbacks.go`, `pkg/plugin/sdk/sdk_types.go` - one callback and the aliases
- `internal/component/ssh/ssh.go` - the interactive branch of `execMiddleware` and its executor factory
- `cmd/ze/hub/service_ssh.go`, `cmd/ze/hub/api.go`, `cmd/ze/hub/streaming_surface.go` - wiring of the session route for SSH and the API engine, and the predicate's adapter
- `cmd/ze/hub/surface_parity_test.go` - the interactive rows
- `internal/component/cli/model_keys.go`, `internal/component/cli/client/main.go`, `internal/component/cli/sshclient/client.go` - the model's hand-off, and the terminal request of `ze cli -c`
- `internal/component/api/engine.go`, `internal/component/api/grpc/server.go`, `api/proto/ze.proto` - the session method
- `internal/core/audit/audit.go` - two action constants
- The runtime child's patch file for H-3 (the refusal of a second process, in the vendor patch directory `spec-container-1-runtime.md` names) - rewritten in place, as H-3 (rewritten)
- The runtime child's named guard test for the patch series, and its bump gate - the rewritten hunk, the new hunk H-7 and the two shell tests. The series is then seven patch files over the same five upstream files
- The runtime child's `TestPatchedLibraryRefusesToEnterAContainer`, and the line of its Known Limitations about entering a container - under A1 the test is replaced by one that a second process starts, and the line is removed, because the refusal they describe is the hunk this child rewrites
- The runtime child's early role registration and its drain handler in the plugin - the second-process role, and ending sessions before containers stop
- The container plugin's `register.go` and command schema, created by `spec-container-1-runtime` and `spec-container-4-operator-surface` - the interactive registration and the three command nodes
- `ai/allowed-system-commands.md` - the row for a program an operator names for a shell, authorised by the owner on 2026-11-16 (OA-1, R-9), in the commit that adds the code it names
- `docs/architecture/api/commands.md`, `docs/architecture/api/process-protocol.md`, `docs/plugin-development/protocol.md`, `docs/plugin-development/handlers.md`, `docs/architecture/api/architecture.md`, `docs/guide/api.md`, `docs/guide/audit.md`, `docs/guide/command-reference.md`, `ai/doors/commands-cli.md`, `ai/doors/plugin-infra.md` - the pages this work makes incomplete
- `docs/architecture/api/ipc_protocol.md` (declared by `pkg/plugin/rpc/types.go`) - affected: "SSH exec channel" says every session gets the answer frame on its error stream. An interactive command's channel carries the program's bytes and no frame, and the section gains that row
- `docs/architecture/system-architecture.md` (declared by `internal/component/ssh/ssh.go`) - affected: "How CLI Works" describes one command and one response over SSH, and gains one sentence for a command that holds a session
- `docs/architecture/hub-architecture.md` (declared by `cmd/ze/hub/service_ssh.go`) - named as unaffected: its status header says the process model it draws is not the live code, and the part read holds no statement about the SSH executor factories. The implementation phase confirms it against the "Command Handling" section, which this spec did not read
- `docs/architecture/config/yang-config-design.md` (declared by `internal/component/cli/model_keys.go`) - named as unaffected: the page documents the config editor, and the hand-off this spec adds runs an operational command and changes no editor behavior. Only its headings were read, so the implementation phase confirms it

## Files to Create
- `internal/component/plugin/server/interactive.go` - the registry, the session route, the limits, the stop hook
- `internal/component/plugin/server/interactive_bounded.go` - the derived bounded run and its record
- `pkg/plugin/rpc/interactive.go` - the seven message types and their framing
- `pkg/plugin/sdk/interactive.go` - the plugin side of a session: ordered, acknowledged chunks
- `internal/component/ssh/interactive.go` - the session's use of the channel, the terminal request and window changes
- `internal/component/cli/model_interactive.go` - release and restore of the terminal
- `internal/component/api/session.go` - the engine method the gRPC transport calls
- `internal/plugins/container/shell.go` - the session handler, the refusals, the program and its arguments
- `internal/plugins/container/shell_sessions.go` - the session table, the caps, the list and close commands, the doctor check
- `internal/plugins/container/shell_terminal_linux.go` - receiving and verifying the terminal descriptor, resize, the relay
- `internal/plugins/container/shell_join_linux.go` - the throwaway-thread join and the child's mount namespace join (A1), registered into the seam of H-7
- `internal/le/vendorpatch/patches/runc-07-second-process-start.patch` - H-7, the seventh file of the runtime child's series
- `internal/test/containerprobe/shellprobe/main.go` - the root probe programs for A-1 to A-8, run in the appliance guest
- The unit test files named in the TDD plan
- `test/container/container-shell-*.ci` and `test/plugin/interactive-command-*.ci` - the functional tests named in the TDD plan. The `container-shell-*` tests are in the `container` suite beside the runtime child's, and run in the own-system guest. The `interactive-command-*` tests drive the general session mechanism with a test plugin and no container, and are in the plugin suite
- `internal/test/fixture` test plugin for an interactive echo command - the external-mode proof
- `docs/guide/container-shell.md` - the operator's page, or a section of the container guide page `spec-container-4-operator-surface` creates

### Integration Checklist
| Integration Point | Applies? | File / reason |
|-------------------|----------|---------------|
| YANG schema (new RPCs/config) | Yes | The three command nodes in the container plugin's command schema: `request container shell name`, `show container session list` and `delete container session id`. The three limit leaves in the container config root, `shell idle-timeout`, `shell absolute-timeout` and `shell session-limit`, which are rows of the operator surface child's tables and land with this child. The seven methods in the two plugin protocol modules |
| YANG validation constraints | Yes | `timeout` range 1 to 600; the limit ranges of the boundary table; `command` and `input` with a length bound; the session id as a ranged integer |
| YANG custom validators | Yes | The container name completes from the running containers, through the completer `spec-container-4-operator-surface` defines |
| CLI commands/flags | Yes | Three commands, registered by the container plugin. No offline flag |
| CLI grammar (keyword before value) | Yes | `name <container>`, `command <words>`, `input <text>`, `timeout <seconds>`, `id <session>`; `./le cli grammar` judges them |
| Editor autocomplete | Yes | Automatic for the leaves; the container name through the completer above |
| Functional test for new RPC/API | Yes | `test/container/container-shell-*.ci`, `test/plugin/interactive-command-*.ci` |
| Pipe completeness | Yes | The bounded record and the session list go through `ApplyPipes`. A live session carries the program's bytes and takes no pipe; a chain on it is refused by name |
| Env var registration | No | The limits are config leaves; no environment key is added |
| Doctor check for runtime dependencies | Yes | The container's pts instance (checked at open, and by the container plugin's runtime check); the session process that outlived its session |
| Doctor code per problem class | Yes | `doctor-container-session-process-survived` and `doctor-container-terminal-refused`, named as the runtime child names its codes, both in the runtime phase. A session ended by a limit is an expected outcome and gets no code |
| Prometheus counters/metrics | Yes | Sessions open (gauge, by owner plugin), sessions started and ended (counters, by end reason), bytes in and out (counters). Names follow `docs/plugin-development/metrics.md` and are fixed at implementation |
| BGP family surface (new SAFI / capability / attribute) | N-A | No BGP surface |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature, or a feature's scope, evidence or level changed? | Yes | `features/containers.md`, which `spec-container-1-runtime` creates. This child extends it: the shell as part of the feature's scope |
| 2 | Config syntax changed? | Yes | The limit leaves, in `docs/guide/configuration.md` |
| 3 | CLI command added/changed? | Yes | `docs/guide/command-reference.md` |
| 4 | API/RPC added/changed? | Yes | `docs/architecture/api/commands.md`: a section "A command that holds a session", the surface table, the bounded record |
| 5 | Plugin added/changed? | Yes | `docs/guide/plugins.md`, through the container plugin's entry |
| 6 | Has a user guide page? | Yes | `docs/guide/container-shell.md`, and `docs/guide/api.md` for the gRPC method |
| 7 | Wire format changed? | No | No protocol wire format under `docs/architecture/wire/` changes |
| 8 | Plugin SDK/protocol changed? | Yes | `docs/architecture/api/process-protocol.md`, `docs/plugin-development/protocol.md`, `docs/plugin-development/handlers.md`, `ai/rules/plugins.md` |
| 9 | RFC behavior implemented, changed, or newly proven? | No | No RFC behavior of Ze changes |
| 10 | Test infrastructure changed? | Yes | `docs/functional-tests.md`: the interactive test plugin and how a `.ci` drives a terminal session |
| 11 | Affects daemon comparison? | No | `docs/comparison.md` compares routing daemons, and none of its rows covers containers |
| 12 | Internal architecture changed? | Yes | `docs/architecture/core-design.md`, the command engine section: the interactive registration beside the streaming one |
| 13 | Route metadata keys added/changed? | No | No route metadata |
| 14 | Prometheus counters added/changed? | Yes | `docs/plugin-development/metrics.md` |
| 15 | Registered plugin, event type, send type, command, capability, or inventory changed? | Yes | `docs/plugin-overview.md`, `docs/features/plugins.md`, `docs/guide/status.md`, for the three commands |
| 16 | Any changed source file referenced by existing doc source anchors? | Yes | Derived at implementation with `./le spec citation anchors spec plan/post-release/features/spec-container-5-shell.md`. Known now: `commands.md` anchors `handler.go`, `surface.go`, `ssh.go` and `service_ssh.go` |
| 17 | Existing docs show config/CLI/API examples for this area? | Yes | `docs/architecture/api/commands.md` "Which surfaces offer a command" and "Where a monitor command streams" gain the interactive rows |

## Implementation Steps

**Gate before phase 1 (owner ruling, 2026-11-16): the kernel probes.** The probes for A-1 to A-8 are the first task of this child and run in the appliance guest, as root, before any other implementation. They are probe programs and their recorded output, and no product code. Their verdict fixes the entry design: A1 when all hold, A3 when A-8 fails. A broken A-1, A-3 or A-5 is reported with the probe's output before phase 1 starts. Phases 1 to 5 build the session mechanism, which is the same under A1 and A3.

1. **Phase: Wiring (MANDATORY FIRST)** -- the interactive registry, the surface predicate, and a test command
   - Tests: `TestAnswerSurfaceInteractive`, `TestRegisterInteractiveRequiresOneShotForm`, `TestEveryCommandOnEverySurface`
   - Files: `internal/component/plugin/server/interactive.go`, `internal/component/command/surface.go`, `cmd/ze/hub/streaming_surface.go`, `cmd/ze/hub/surface_parity_test.go`
   - Verify: a registered test command is listed on every surface with the right offer, and running it fails because the route is a stub
2. **Phase: the session route and the bounded run** -- accounting, authorization, limits, the stop hook, the derived one-shot answer
   - Tests: `TestSessionRouteAuthorizesAsStateChanging`, `TestBoundedRunTimeBound`, `TestBoundedRunSizeBound`, `TestSessionIdleAndAbsoluteLimits`, `TestSessionsEndAtServerStop`
   - Files: `interactive.go`, `interactive_bounded.go`, `internal/core/audit/audit.go`
   - Verify: tests fail → implement → tests pass; the test command answers its record on the dispatcher route
3. **Phase: the process protocol** -- the seven messages, the SDK callback, the bridge slot, the Stage 1 check
   - Tests: `TestSessionMessagesRoundTrip`, `TestSessionChunksAreAcknowledgedInOrder`, `TestStage1RefusesInteractiveWithoutOneShot`, `interactive-command-external-plugin.ci`, `interactive-command-plugin-dies.ci`
   - Files: `pkg/plugin/rpc`, `pkg/plugin/sdk`, the two protocol YANG modules, `startup.go`, `dispatch_registry.go`, the test plugin
   - Verify: the echo plugin answers the same bytes internal and external
4. **Phase: the terminal surfaces** -- SSH exec, the `ze cli` model, `ze cli -c`
   - Tests: `TestExecMiddlewareRoutesInteractiveByRegistry`, `TestModelReleasesAndRestoresTerminal`
   - Files: `internal/component/ssh`, `internal/component/cli`, `cmd/ze/hub/service_ssh.go`
   - Verify: the echo plugin holds a terminal session over SSH and from the CLI prompt
5. **Phase: gRPC** -- the session method
   - Tests: `TestGRPCSessionCarriesInputAndExit`
   - Files: `api/proto/ze.proto`, `internal/component/api`, `cmd/ze/hub/api.go`
   - Verify: the echo plugin holds a session over gRPC
6. **Phase: entering a container** -- under the design the probe gate fixed: for A1 the hunks H-3 (rewritten) and H-7, the registered join, the second-process role, the terminal descriptor; for A3 the helper role of "The fallback entry design"
   - Tests: `TestNoDaemonThreadStaysInContainerNamespaces`, `TestTerminalDescriptorIsVerified`, `TestShellRunsFromASealedBinary`, `container-shell-ssh-terminal.ci`, `container-shell-hostile-container.ci`, and the runtime child's patch guard test extended to H-3 (rewritten) and H-7
   - Files: the two patch files, `internal/plugins/container/shell_join_linux.go`, `shell_terminal_linux.go`, the plugin's early role registration
   - Verify: a shell opens in a container in the appliance guest, and the confinement of AC-8 reads the same inside it as in the container's own process
7. **Phase: the container plugin's commands** -- the handler, the refusals, the session table, the list and close commands, the doctor check, the limits
   - Tests: the remaining unit tests of `internal/plugins/container`, and every remaining `test/container/container-shell-*.ci`
   - Files: `internal/plugins/container/shell.go`, `shell_sessions.go`, the command schema, the config leaves
   - Verify: every AC has its test green through the real entry point
8. **Phase: pages** -- every page the Documentation checklist names, in the phase that made it wrong where the change is small, and the operator's page here

### Critical Review Checklist
| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | Every AC-N has an implementation at file:line |
| Feature completeness | Every user story has a working path, no broken links |
| Correctness | The session ends exactly once whatever ends it first (exit, limit, disconnect, container stop, daemon stop, plugin death), and the process is ended on every one of those paths |
| Naming | JSON keys kebab-case (`exit-status`, `timed-out`); the message names match the YANG method names; the audit actions match their constants |
| Data flow | No surface adapter compares a command name. Each asks the interactive registry. The engine names no plugin and no container |
| Process boundary | The container plugin holds a session only through the SDK callback. `./le plugin boundary check` finds no direct call |
| Thread discipline | No code path unlocks the thread that joined a container's namespaces, and no goroutine other than the joining one runs on it |
| Descriptor discipline | The terminal descriptor is verified before use, every descriptor of a session is closed when it ends, and none reaches a later process start |
| Authorization | The route passes the read-only flag clear. A denial is accounted and audited. The bounded form and the live form authorize the same line |
| Operator errors and doctor codes | Every refusal is an `opererr` error with its fix on the last line, every `Warn`/`Error` line and raise names the code the Integration Checklist's doctor row lists, and each doctor check carries `Fix` at the phase that holds its state (`ai/rules/user-facing-errors.md`) |
| Rule: `ai/rules/goroutine-lifecycle.md` | One relay worker for each session, started at open and joined at end; nothing is started for each chunk |
| Rule: `ai/rules/go-standards.md` | The register row covers a program an operator names, as the owner authorised on 2026-11-16 (OA-1), and lands in the commit that adds the code of phase 6 |

### Deliverables Checklist
| Deliverable | Verification method |
|-------------|---------------------|
| The interactive registry and its surface offer | `./le test unit pkg path ./internal/component/command/ run TestAnswerSurfaceInteractive` |
| The session route, the bounded run, the limits | `./le test unit pkg path ./internal/component/plugin/server/ run 'Interactive\|Session\|Bounded'` |
| The seven protocol messages and the SDK callback | `./le test unit pkg path ./pkg/plugin/rpc/` and `./le test functional plugin` with the two `interactive-command-*.ci` |
| The SSH and CLI forms | `./le test unit pkg path ./internal/component/ssh/ run Interactive` and `container-shell-from-cli-session.ci` |
| The gRPC method | `./le test unit pkg path ./internal/component/api/grpc/ run TestGRPCSession` |
| The shell in a container, with its confinement | `container-shell-ssh-terminal.ci` in the QEMU suite the container specs define |
| Surface parity | `TestEveryCommandOnEverySurface` in `cmd/ze/hub` |
| Grammar, ownership, declarations | `./le cli grammar`, `./le cli ownership`, `./le plugin declarations check`, `./le plugin boundary check` |
| The pages | `./le doc check all` |

### Security Review Checklist
| Check | What to look for |
|-------|-----------------|
| Input validation | `command` is split by Ze's grammar and passed as words, never through a host shell. The container name, the session id, the terminal size and the signal name are each validated against the model before the owner sees them |
| Entering the container | The entering process is non-dumpable before any join. The sealed binary is used. Every namespace is joined from a process descriptor of the container's init, checked against the container's recorded start time. The working directory check runs after the last change of directory |
| Descriptors | No descriptor of the daemon reaches the program: the close-on-exec sweep runs before the start and the real close before the exec. The mount namespace descriptor is closed after the join. The terminal descriptor is verified |
| Thread safety | No daemon thread remains in a container's namespaces (R-3) |
| Resource exhaustion | Session caps for one container and for the daemon; the idle and absolute limits; the bounded time and size; acknowledged chunks; the memory of each sealed binary (R-12) |
| Authorization that could fail open | A missing authorizer, an unknown surface or a registry miss refuses. The streaming executors' read-only default is not reused for an interactive command |
| Error leakage | A refusal names the container and the program, and never a host path, a host process id or a descriptor number |
| Audit | A start entry exists before the first byte flows, and an end entry on every ending path, including a daemon stop |
| Terminal injection | The bytes of a live session are the program's own and go only to the session's surface. The exit line, the limit line and every refusal are written by Ze after the terminal is restored, so a program cannot forge them in the CLI's own screen |
| The bounded answer | Output is data in a record. A control sequence in `stdout` is rendered as text by `| table` and never interpreted by the web console |

### Failure Routing

| Failure | Route To |
|---------|----------|
| Compilation error | Fix in the phase that introduced it |
| Test fails for the wrong reason | Fix the test assertion or setup |
| Test fails on behavior mismatch | Re-read the source in Current Behavior. If misunderstood → RESEARCH |
| Lint failure | Fix inline. If architectural → DESIGN |
| Functional test fails | Check the AC: wrong AC → DESIGN, correct AC → IMPLEMENT |
| Audit finds a missing AC | Back to the relevant phase and implement |
| A kernel probe breaks A-1, A-3 or A-5 | STOP. Report the probe's output. The owner chooses again under O-1 |
| 3 fix attempts failed | STOP. Report all 3 approaches. Ask the user |

## Design Insights

- The C join exists for one kernel rule: a pid namespace join applies to children. Everything else upstream does after the join is Go, and is the same sequence the create path already inherits.
- The daemon cannot let its own thread enter a container's mount namespace and then start the init role, because Go finds the program by path and the path would resolve inside the container. Joining the mount namespace in the child, after it started from the host's view, avoids that, and costs R-2.
- A one-shot form is owed by the registration, not by the surface. The engine derives it from the session (no terminal, the input given up front, a time bound and a size bound), so a plugin author writes one handler and every surface has a form.

## Owned journal rows
| Class file | Date | Surface (short) |
|------------|------|-----------------|

## Key Design Decisions
| Decision | Alternatives Considered | Rationale |
|----------|------------------------|-----------|
| A1 (approved 2026-11-16): the build-time diff gains hunks in the library's second-process path. The daemon side runs the start on a throwaway locked thread that joins the container's network, ipc, uts and cgroup namespaces and sets the children's pid namespace, through a process descriptor. The child, the `ze` binary in the second-process role, joins the mount namespace on its locked main thread and calls the library's init. One process start | A2, A3, A4 below | Written: two hunks, H-3 (rewritten) and H-7, in two upstream files the runtime child's series already patches (the first replaces the content of its H-3, the second is a new seventh patch file), and the join and the child's first steps as Ze code in the plugin, about 100 lines, behind a registered function as the filter is. Inherited: the cgroup placement at creation with its fallback, the whole second-process init (terminal, no-new-privileges, filter, capabilities, user, working directory and its check, the descriptor close), and upstream's future fixes to it. It matches the approved design for the create path. Cost: the hunks sit in the functions with the most upstream churn (R-8), and the entering process is weaker than upstream's against a container that can trace (R-2). Unproven without a run: A-1 to A-8 |
| A2 (set aside): no new hunk. Ze's init role speaks the library's startup protocol for a second process: a first stage reads the bootstrap data, joins the namespaces that allow it on a locked thread, starts a throwaway process and the second stage as children of the daemon, writes the pid reply and exits. The second stage joins the mount namespace and calls the library's init | A1 | Written: the trampoline, about 200 lines. Inherited: the same as A1, with the library unmodified on this path. Cost: three starts of the `ze` binary for each shell, each from a sealed copy; a dependence on undocumented protocol details (the reply's shape, the descriptor order, the parent relation) that change with no compile error; and the same R-2. The umbrella set the same trade aside for the create path, for the same reasons |
| A3 (set aside, the fallback if A-8 breaks): Ze leaves the library on this path. A helper process in the host's pid namespace, non-dumpable, joins every namespace on a locked thread, allocates the terminal, sets no-new-privileges, the filter and the capability bounds on that thread, and starts the program from it as a child of the daemon. The program is born inside with everything applied, and the helper exits | A1 | Written: the join and Ze's own copy of the second-process sequence, about 300 to 400 lines of security-critical code. Inherited: the capability package, Ze's filter builder, the console package. Gain: no host image is ever visible inside the container's pid namespace, which closes R-2 by construction. Cost: Ze owns the confinement sequence and follows upstream's fixes by reading, the ordering upstream uses between the user change and the capability change cannot be reproduced between fork and exec in Go, and the filter must allow the calls the start itself makes. The umbrella refused owning that sequence for the create path |
| A4 (set aside): no join at all. Ze's helper stays as the container's first process and starts the entrypoint as its child. The daemon holds a control descriptor to it and asks it to start a process, which is born inside | A1 | Written: a resident supervisor (signal forwarding, reaping, the request protocol) and a change to the library's standard init, which ends in an exec today. Inherited: less than the approved design, because the first process no longer is the entrypoint. Gain: no host process ever enters a running container. Cost: every container carries a resident Go program and its memory; an image that expects to be process 1 behaves differently; the helper holds more privilege than the entrypoint for the container's whole life; and the approved create path changes |
| The mount namespace is joined by the child, not by the daemon's thread (approved 2026-11-16, inside A1) | The daemon's thread joins every namespace, mount included, and the child is born fully inside | Go starts a program by path, and the sealed binary is reached through a descriptor path that would resolve in the container's own process filesystem, which the container controls. Joining in the child keeps the lookup on the host's side. The cost is R-2 |
| B1 (approved 2026-11-16): one mechanism, the interactive command. A registration declares a handler that holds a session (input, output, an optional terminal, resize, signals, an exit status) and declares its one-shot form. The engine derives the bounded run from the session. Any plugin registers one, compiled in or running as a separate process | B2, B3 below | It is the streaming registration's shape with the input half added, so every surface producer and the parity test extend by one field. The one-shot form is owed by the registration, so no surface is excluded, which is the owner's rule. The bounded run is the same handler with no terminal, so the two forms cannot drift apart |
| B2 (set aside): no new mechanism. A session is state the owner holds, driven by ordinary commands: one opens it and answers an id, one sends input, one reads the output so far, one closes it | B1 | Every surface works with no protocol change, and an AI agent can hold a long dialogue with a program. But a terminal surface would poll: no raw keys, no resize, no signals, and a visible delay, so the SSH operator gets a worse shell than the local one. It also makes each plugin invent its own four commands. It can be added later on top of B1's session table if an AI dialogue is wanted; it is not needed for the goal |
| B3 (set aside): the shell is an SSH feature. The SSH layer hands the channel to the container plugin, as it does for the plugin protocol session, and a separate ordinary command runs one program for the other surfaces | B1 | The cheapest to build. It is a container special case in a shared package, it does not cross the process boundary, it gives gRPC nothing, and it makes two commands that must be kept alike by hand |
| Session bytes travel as framed, acknowledged messages on the plugin's existing connection (approved 2026-11-16, O-9) | A second connection for each session carrying raw bytes | The connection frames a request by its newline, so bytes are encoded. Reusing it reuses authentication, the request ids, the stop and failure handling, and every third-party parser. The acknowledgment of each chunk is the flow control. A terminal session is small; the second connection pays for bulk transfer nobody asked for |
| An interactive command is authorized as a state-changing command, and its registry is separate from the streaming registry (approved 2026-11-16) | A flag on the streaming registration | Both streaming executors pass the read-only flag and say a write-capable stream needs registry metadata. A separate registry keeps a read from ever being routed as a session, and the reverse |
| The bounded answer is a record with both output streams, the exit status and two flags (approved 2026-11-16) | The program's output as the answer text | A handler answers data. The exit status and the reason a run ended are part of the answer, and an automation reads them as fields |
| The terminal is allocated inside the container by the library's own code (inherited under A1) | Allocating it on the host and passing the peer in | The library opens the container's own pts handle without following a link, verifies it, and takes the peer from the descriptor. A host terminal inside a container was upstream's earlier design and the source of its console defects |
| The limits are held by the owner and enforced by the engine (approved 2026-11-16) | Each plugin enforces its own | The owner knows its config and its subject (the container); the engine sees every byte and every surface. The open answer carries the limits, so the engine holds no plugin's config path |
| `command` is passed as words, with no shell between (approved 2026-11-16, O-3) | Running it through the container's shell | It works in an image with no shell, and nothing the operator types is interpreted twice |

## Known Limitations
- A container health check is not part of this spec. The owner ruled it a later child of the container work. It will run its command through the bounded run this spec provides (a program in a running container, with a time bound, a size bound and an exit status), and it is owed to `plan/post-release/features/spec-container-6-health-check.md`, a skeleton that depends on this spec. `spec-container-4-operator-surface.md` already fixes the config form it will take.
- A shell exists only where a container does: on the appliance and under a daemon that runs as root, at first. The runtime child refuses a container config elsewhere.
- A session does not survive a restart of Ze. Containers are stopped and started again then, and re-adopting a running container is not part of the container work.
- A live terminal in the browser and a live session over REST are not built (owner decisions O-4, O-5). Both surfaces run the bounded form. Either one, if wanted later, is a spec of its own.
- A program the operator detached on purpose inside the shell keeps running after the session ends. It is a process of the container, under the container's limits, and ends with the container.
- The shell runs as the container's configured user, with the container's confinement. No leaf selects another user or widens a capability for one session.
- `ze cli -c` requests a terminal only for an interactive command the binary itself carries. For a command an external plugin declared at run time it runs the form without a terminal; the `ze cli` shell and `ssh -t` give the terminal (R-11).
- Nothing typed inside a live session is authorized or accounted by Ze command by command. The session is one authorized command; what runs inside it is the container's business.

## RFC Documentation (Scope: protocol)

N-A: Scope is plugin. No RFC-enforcing code is added.

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
- [ ] Every user story has a working path and a passing test (N-A when Scope is tooling or docs, which delete that section)
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
