# 1354 - Headless `gopls mcp` exhausts the system file table

**Date:** 2026-08-06
**Scope:** tooling, agent workflow, rules

## What Changed

`.mcp.json` registered `gopls mcp` as a stdio MCP server. It is removed. Symbol
navigation now goes through the LSP tool (`gopls-lsp@claude-plugins-official`)
or the `gopls` CLI, both already documented in `ai/rules/context-economy.md`.

## The Failure

One `gopls mcp` process held **245,764 open file descriptors** after 10 minutes.
That count is `kern.maxfilesperproc` exactly. It was 96.5% of `kern.num_files`
(254,573 of a 491,520 table). Every other process on the machine then failed to
open anything: `omp` died on SIGTRAP, three language servers failed to start,
and the shell reported `ENFILE: file table overflow`.

The count is one descriptor per FILE, not per directory: 228,862 REG against
16,893 DIR.

## The Mechanism

Two facts combine, and neither is a misconfiguration.

**Headless MCP mode watches the tree itself.** An LSP client registers
`workspace/didChangeWatchedFiles` and does its own watching. `gopls mcp` has no
client, so `internal/cmd/mcp.go` starts `filewatcher.New("fsnotify", nil, ...)`
and calls `WatchDir` on every root the MCP client advertises.

**fsnotify is kqueue on macOS, and kqueue needs an open descriptor per watched
path.** Watching a directory therefore opens a descriptor for each file in it.

## It Honors No Filter

`skipDir` (`golang.org/x/tools/gopls/internal/filewatcher/fsnotify_watcher.go`)
is the entire filter:

```go
// TODO(hxjiang): the file watcher should honor gopls directory
// filter or the new go.mod ignore directive, ...
return strings.HasPrefix(dirName, ".") || strings.HasPrefix(dirName, "_") || dirName == "testdata"
```

There is no `directoryFilters`, no config file, no environment variable, and no
flag. `gopls mcp -h` lists four flags and none of them scope the walk. The
placeholder `go.mod` in `gokrazy/modcache/` hides that tree from the go tool,
and the watcher walks it anyway.

`filepath.WalkDir` does not follow symlinks, which is why the `cache` symlink
(125,109 files under `~/.cache/ze/`) cost nothing.

## What Made It Fatal Here

`tmp/` held five git worktrees plus `tmp/golangci-lint-cache/`, so every file in
the repository was counted six times. `tmp/` alone was 196,052 of the 245,764.
The worktrees are gone. The shape that produced them is not.

## What To Do

| Situation | Do |
|-----------|-----|
| A tool needs to watch this repository | Ask what it costs per file first. 53,000 files is the floor, and a tree of worktrees multiplies it |
| Something fails with `ENFILE` | `sysctl kern.num_files kern.maxfiles`, then `lsof -w -n -P` aggregated by pid. One process at exactly `kern.maxfilesperproc` is the signature |
| You need symbols | LSP tool, or `gopls` from Bash (`ai/rules/context-economy.md`). The LSP server held 12 descriptors against the MCP server's 245,764 |

**A per-process limit does not protect the machine.** `kern.maxfilesperproc` is
50% of `kern.maxfiles` on this host, so one process reaching its own ceiling
starves every other process while never exceeding its own limit. The limit was
working as designed when the machine became unusable.
