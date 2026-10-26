// Design: docs/contributing/running-commands.md -- "When the disk is full", the full empty and who it waits for
// Overview: cacheclean.go -- the scratch clean action whose `all` keyword reads this
//
// This file answers which running processes may be using the build caches, so
// `le scratch clean all` can refuse to empty them underneath a build.
// Emptying a cache a compile, a link, a vet or a lint is reading fails that
// run with "cache entry not found" or "no such file", which names no cause and
// reads as a code defect in another session (the staticcheck matrix,
// 2026-10-21).
//
// A process counts when its program is one that reads a Go build cache, or
// when it holds a slot in this checkout's job registry, which is where le
// records a `le job run` and a verify run (job.Holders). Its environment and
// working directory are not read: procscan carries neither, and macOS exposes
// another process's environment to no unprivileged API this package uses. The
// answer is not narrower for it, because the full empty includes the ambient
// cache, the machine default that every `go` command without a GOCACHE of its
// own writes, so any toolchain process this user runs may be reading one of
// the caches the empty would remove.
package scratch

import (
	"slices"
	"strings"

	"github.com/ze-software/ze/internal/core/textbuf"
	"github.com/ze-software/ze/internal/le/job"
	"github.com/ze-software/ze/internal/le/procscan"
)

// CacheUser is one running process a full empty refuses on: its pid, its
// command line as the process table shows it, and the job registry label it
// holds a slot under, when it holds one.
type CacheUser struct {
	PID     int    `json:"pid"`
	Command string `json:"command"`
	Job     string `json:"job,omitempty"`
}

// cacheReaders are the programs that read a Go build cache: the go command,
// the tools it runs (compile, link, vet, asm, cgo, cover), and the analysers
// that load packages through it. gopls is not one: it keeps its own cache, and
// the `go list` it starts is a go process of its own, seen here while it runs.
var cacheReaders = []string{"go", "compile", "link", "vet", "asm", "cgo", "cover", "staticcheck", "golangci-lint"}

// testBinarySuffix ends the name of a compiled test binary, which reads the
// cache when a test builds or runs a go command of its own.
const testBinarySuffix = ".test"

// commandShownMax bounds the command line a refusal prints for one process. A
// compile's argument list runs to kilobytes, and the program and its first
// arguments are what identify the run.
const commandShownMax = 160

// oldEntriesInstead names the default mode as the command to run instead of a
// refused full empty, with the floor it keeps.
func oldEntriesInstead(why string) string {
	var text textbuf.Buffer
	return text.Str("run `./le scratch clean`, which removes only cache entries older than the ").
		Str(cacheEntryAgeFloor.String()).Str(" floor and ").Str(why).Byte('.').String()
}

// refuseFullEmpty answers why a full empty must not run now, and the processes
// it refuses on, or "" when nothing that may use the caches is running. A
// process table or a registry that cannot be read refuses too: with no answer
// to "is a build running", the empty could fail one.
func (m *Manager) refuseFullEmpty(pass cleanPass) (string, []CacheUser) {
	processes, err := pass.scan()
	if err != nil {
		return "the full empty cannot tell whether a build is using the caches, because the process table could not be read: " +
			err.Error() + ". Instead, " + oldEntriesInstead("needs no process table"), nil
	}
	holders, err := pass.holders(m.Root)
	if err != nil {
		return "the full empty cannot tell whether a le job or verify run is using the caches, because the job registry could not be read: " +
			err.Error() + ". Instead, " + oldEntriesInstead("needs no registry"), nil
	}
	users := cacheUsers(processes, holders, pass.self)
	if len(users) == 0 {
		return "", nil
	}
	var text textbuf.Buffer
	text.Str("the full empty would remove cache entries that ").Int(int64(len(users)))
	if len(users) == 1 {
		text.Str(" running process may be reading")
	} else {
		text.Str(" running processes may be reading")
	}
	text.Str(", and a build that loses one fails with `cache entry not found`.").
		Str(" Wait for them to finish and run `./le scratch clean all` again, or ").Str(oldEntriesInstead("is safe while builds run"))
	return text.String(), users
}

// cacheUsers answers every process that may be using the caches, in the order
// the scan listed them, leaving out the pids that are this run itself. A
// registry entry counts only while its pid is in the scan: an entry outlives a
// killed job until the next admission reaps it.
func cacheUsers(processes []procscan.Process, holders []job.Holder, self []int) []CacheUser {
	labels := make(map[int]string, len(holders))
	for _, holder := range holders {
		labels[holder.PID] = holder.Label
	}
	var users []CacheUser
	for _, process := range processes {
		if slices.Contains(self, process.PID) {
			continue
		}
		label, holds := labels[process.PID]
		if !holds && !readsCache(process.Argv) {
			continue
		}
		users = append(users, CacheUser{PID: process.PID, Command: shownCommand(process.Argv), Job: label})
	}
	return users
}

// readsCache answers whether a command line runs a program that reads a Go
// build cache. The program is the last path component of argv[0], so a tool
// started by its full path under a GOROOT counts the same as one found on PATH.
func readsCache(argv []string) bool {
	if len(argv) == 0 {
		return false
	}
	program := argv[0]
	if index := strings.LastIndexByte(program, '/'); index >= 0 {
		program = program[index+1:]
	}
	if strings.HasSuffix(program, testBinarySuffix) {
		return true
	}
	return slices.Contains(cacheReaders, program)
}

// shownCommand renders a command line for a refusal, cut at commandShownMax.
func shownCommand(argv []string) string {
	command := strings.Join(argv, " ")
	if len(command) <= commandShownMax {
		return command
	}
	return command[:commandShownMax] + "..."
}

// line renders one refused-on process for the text report.
func (user CacheUser) line() string {
	var text textbuf.Buffer
	text.Str("pid ").Int(int64(user.PID)).Str(": ").Str(user.Command)
	if user.Job != "" {
		text.Str(" (le job ").Str(user.Job).Byte(')')
	}
	return text.String()
}
