# 26 Report the Go runtime SIGQUIT hang under -race to golang/go

A session dedicated to one job: report a Go runtime bug upstream, correctly.
An AI found the bug, diagnosed it and wrote the fix. A wrong or inflated
report would cost Thomas credibility with the Go team, so this session's
standard is that every sentence in the report is something we observed or
read in the Go source, and every inference is labelled as one. Nothing is
posted to GitHub until Thomas has read and approved the exact text.

## What is already done (do not redo)

- **The patch is in Ze:**
  `internal/le/go/stdpatch/patches/runtime-traceback-hexdump-off-stack.patch`
  (stamped `Ze-Reviewed-Against: go1.27.3`), documented in
  `docs/contributing/patching-the-standard-library.md`, section "The runtime
  traceback patch". Check `git log` for it before starting; if it is still
  uncommitted, ask Thomas rather than committing it from this session.
- **The diagnosis**, read in the go1.27.3 source at
  `/opt/homebrew/Cellar/go/1.27.3/libexec/src/runtime` on the Mac.
- **A standalone reproducer** with no Ze code (below).

## The bug, as verified

A SIGQUIT goroutine dump of a `-race` binary can stop after its first
goroutine and never finish. On darwin/arm64 the process hangs for ever.

1. `racecall<>` in `runtime/race_arm64.s` switches SP to g0's stack and calls
   into the race runtime without changing `g`. `msancall` and `asancall` (in
   `msan_amd64.s`, `asan_amd64.s`) do the same.
2. A SIGQUIT landing there traces the user goroutine from a PC inside C code.
   `(*unwinder).initAt` prints `runtime: g N: unknown pc` and calls
   `tracebackHexdump(gp.stack, &frame, 0)`.
3. `tracebackHexdump` clamps `lo` up to `stk.lo` and `hi` down to `stk.hi`.
   `frame.sp` is on g0's stack, outside `stk`, so `hi` ends below `lo`, and
   `hexdumpWords(lo, hi-lo, ...)` gets a wrapped length. Its `unsafe.Slice`
   throws `fatal error: unsafe.Slice: len out of range`, then `panic during
   panic`.
4. `fatalthrow` reaches `dopanic_m`, which traces the other goroutines, the
   same goroutine included, from its saved registers. Every main-thread
   sample of a hung child sits at `(*unwinder).resolveInternal+548`, which
   objdump maps to the load `frame.lr = *(*uintptr)(unsafe.Pointer(lrPtr))`
   in `resolveInternal` (11 of 11 hung children sampled).

Source locations as read in go1.27.3 (recheck before quoting them upstream):

```
runtime/race_arm64.s     racecall<>              461-485
runtime/msan_amd64.s     msancall                71-89
runtime/asan_amd64.s     asancall                95-113
runtime/traceback.go     initAt "unknown pc"     203-205
runtime/traceback.go     tracebackHexdump clamp  1456-1461, call at 1465
runtime/hexdump.go       hexdumpWords unsafe.Slice  34
runtime/panic.go         fatalthrow 1434, startpanic_m "panic during panic" 1558, dopanic_m 1580
runtime/traceback.go     tracebacksomeothers     1380-1384
runtime/traceback.go     resolveInternal lr load 374
```

**Regression.** CL 681375, "runtime: add hexdumper" (commit e912618bd2,
go1.26), introduced it. In go1.25, `hexdumpWords(p, end)` looped
`for i := 0; p+i < end; ...` and printed nothing when `end <= p`. golang/go
master at 0d37007fda (2026-10-24) has a byte-identical `tracebackHexdump`, so
master is affected.

**The fix in our patch** is one hunk: after the existing `stack:` print line,
`tracebackHexdump` returns when `hi <= lo`. That restores go1.25's outcome.

## What is NOT verified (say so in the report, or verify first)

- **Why the `resolveInternal` load never returns.** The belief is that it
  faults and is re-executed while the signal handler has SIGSEGV blocked. Not
  traced in the darwin kernel. Do not state it as fact.
- **Linux behaviour.** Not tested by us. golang/go#73470 shows the same chain
  ending in `hang 10m0s` on the linux msan/asan builders, which suggests Linux
  hangs too, while our own expectation was SIGSEGV. Run the reproducer on the
  Linux host (scoped run only, per `ai/META-DECISIONS.md`) before claiming
  anything about Linux.
- **The C function at the PC.** `atos` names `debug_control_port_for_pid+0`
  in libsystem_kernel.dylib, which is only the nearest exported symbol.
- **Which race-runtime call the goroutine was in** when the signal landed, in
  the standalone reproducer. Its goroutines only block on a channel, so the
  race call is presumably in goroutine creation or channel code; not checked.

## Upstream state

- **golang/go#73470**, "os/exec: TestWaitInterrupt/SIGQUIT failures", open,
  `NeedsInvestigation`, opened 2025-04-22. Since 2025-11-21 its builder logs
  show this exact chain: `unknown pc`, `fatal error: unsafe.Slice: len out of
  range`, `panic during panic`, then a 10 minute hang, on linux msan and asan.
  Every comment is from gopherbot and none names a cause. No CL on Gerrit.
- **golang/go#64030** ("runtime/traceback: segmentation violation failures
  from unwinding crash") is related in area, not the same bug.
- Search again before posting: issues and Gerrit CLs mentioning
  `tracebackHexdump`, `hexdumpWords`, `unsafe.Slice: len out of range`, and
  CL 681375. Someone may have found it since 2026-10-25.

The diagnosing agent recommends posting the analysis as a comment on #73470
rather than opening a new issue, since it explains that issue's failures. A
new issue linking #73470 is the alternative if the Go team's conventions
prefer it. Thomas decides, and also decides whether a CL goes to Gerrit (a CL
needs the Go CLA and a test, see below).

## Evidence

### Standalone reproducer

Build with `go build -race`, run `./sigquitrace -runs 120 -par 8` with
`GOMAXPROCS=4`. Measured on Thomas's Mac (darwin/arm64, go1.27.3): unpatched
16 hangs in 120, every one with the `unsafe.Slice` fatal; patched 0 in 120.

`go.mod`: `module sigquitrace` / `go 1.27`.

```go
// Command sigquitrace reproduces a Go runtime hang: a SIGQUIT dump of a
// race-detector binary stops after its first goroutine and the process
// never exits.
//
// Build it with -race, then run: ./sigquitrace -runs 120 -par 8
//
// The parent starts children that each start many goroutines and send
// themselves SIGQUIT. A child still alive after -timeout is a hang.
package main

import (
	"bytes"
	"context"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"sync"
	"syscall"
	"time"
)

func child(n int) {
	block := make(chan struct{})
	for range n {
		go func() { <-block }()
	}
	if err := syscall.Kill(os.Getpid(), syscall.SIGQUIT); err != nil {
		os.Exit(9)
	}
	time.Sleep(time.Minute)
	os.Exit(8)
}

func main() {
	isChild := flag.Bool("child", false, "run as the child")
	gor := flag.Int("goroutines", 4000, "goroutines each child starts")
	runs := flag.Int("runs", 120, "children to run")
	par := flag.Int("par", 8, "children at a time")
	timeout := flag.Duration("timeout", 25*time.Second, "a child alive this long is a hang")
	flag.Parse()
	if *isChild {
		child(*gor)
	}

	var mu sync.Mutex
	hangs, unsafeSlice, done := 0, 0, 0
	sem := make(chan struct{}, *par)
	var wg sync.WaitGroup
	for i := range *runs {
		sem <- struct{}{}
		wg.Add(1)
		go func() {
			defer wg.Done()
			defer func() { <-sem }()
			ctx, cancel := context.WithTimeout(context.Background(), *timeout)
			defer cancel()
			cmd := exec.CommandContext(ctx, os.Args[0], "-child", fmt.Sprint("-goroutines=", *gor))
			var stderr bytes.Buffer
			cmd.Stderr = &stderr
			_ = cmd.Run()
			hung := ctx.Err() != nil
			out := stderr.String()
			mu.Lock()
			defer mu.Unlock()
			done++
			if strings.Contains(out, "unsafe.Slice: len out of range") {
				unsafeSlice++
			}
			if hung {
				hangs++
				fmt.Printf("run %d: HANG, %d bytes of dump\n", i, len(out))
				if hangs == 1 {
					_ = os.WriteFile(fmt.Sprintf("hang-%d.txt", os.Getpid()), stderr.Bytes(), 0o644)
				}
			}
		}()
	}
	wg.Wait()
	fmt.Printf("runs=%d hangs=%d unsafe.Slice-fatal=%d\n", done, hangs, unsafeSlice)
}
```

For the report, trim it: the Go team will want the smallest program that
shows the hang. Try fewer goroutines and lower parallelism, and record the
hit rate of whatever version is posted. Rerun it before posting; never quote
these numbers for a program you changed.

### Start of a hung child's dump (unpatched)

```
SIGQUIT: quit
PC=0x19283cc74 m=0 sigcode=0

goroutine 2951 gp=0xc00095af00 m=0 mp=0x10310fa40 [running]:
runtime: g 2951 gp=0xc00095af00: unknown pc 0x19283cc74
stack: frame={sp:0x16cf49370, fp:0x0} stack=[0xc000be1000,0xc000be1800)
fatal error: unsafe.Slice: len out of range
panic during panic

goroutine 0 gp=0xc000004000 m=0 mp=0x10310fa40 [idle]:
runtime.throw(...)
	runtime/panic.go:1243
runtime.panicCheck1(...)
	runtime/panic.go:59
runtime.panicunsafeslicelen1(...)
	runtime/unsafe.go:101
runtime.panicunsafeslicelen()
	runtime/unsafe.go:96
runtime.hexdumpWords(0xc000be1000, 0xffffff416c368470, 0xc00000d590)
	runtime/hexdump.go:34
runtime.tracebackHexdump(...)
	runtime/traceback.go:1465
runtime.(*unwinder).initAt(...)
	runtime/traceback.go:205
runtime.traceback1.func1(0x0)
	runtime/traceback.go:962
runtime.traceback1(...)
	runtime/traceback.go:986
runtime.tracebacktrap(...)
	runtime/traceback.go:880
runtime.sighandler(0x3, ...)
	runtime/signal_unix.go:773
```

`sp:0x16cf49370` lies outside `stack=[0xc000be1000,0xc000be1800)`, and the
length passed to `hexdumpWords` is that wrapped difference. Paths are trimmed
here; take the full text from a fresh run for the report.

### Ze-side measurements

Ze's crash-log test (`TestSIGQUITDumpDrainsAfterInit` in
`internal/core/crashlog`) is where this was found, from one 30 second timeout
in a full verify. Side by side at the same load (GOMAXPROCS=4, 8 children at
a time, 60 s limit): unpatched 8 of 60 children hung, and every child that hit
`unknown pc` hung; patched 0 of 60 hung, and the 5 that hit `unknown pc`
dumped in full. These numbers are about Ze's test; the report should lead
with the reproducer.

The session scratch directory
`tmp/session/2026-10-25-85f8903f-5b25-4c3d-ae01-0486bd9c008f/scratch/` held
the raw evidence (`repro/`, `upstream/` copies of the go1.25, go1.26 and
master sources, `st1/` sample files, `objdump.txt`). It is untracked and may
be gone; regenerate rather than rely on it.

## How to run this session

1. Re-read the go1.27.3 source and golang/go master for every step above and
   confirm each location still holds. Correct this file if not.
2. Search upstream again (issues, Gerrit) for an existing report or fix.
3. Rebuild and run the reproducer, unpatched and patched, on the Mac, then on
   the Linux host, scoped. Record the numbers you observe yourself.
4. If a CL is wanted, write the test the Go team would accept: probably a
   runtime test that drives `tracebackHexdump` with a frame whose sp lies
   outside the given stack and asserts it returns without throwing. Read how
   runtime tests reach unexported functions (`export_test.go`). Send nothing.
5. Draft the comment or issue in Thomas's voice (read
   `~/.claude/commands/write.md`): first person, UK spelling, plain, no em
   dashes. Order: what happens, the reproducer and its hit rate, the
   mechanism with source locations, the regression CL, the one-line fix.
   Mark the unverified points as such. Say plainly that the analysis was done
   with an AI assistant and that every claim was checked against the source
   and by running the reproducer.
6. Show Thomas the draft. Post only the approved text, through `gh`.
7. After posting, update the patch header's "Upstream status" line and the
   doc page with the issue or CL link, through `./le commit create`.
