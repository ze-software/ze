// Design: docs/architecture/core-design.md -- le's registration adapter
// Overview: leroot.go -- runWithStartup, which runs every action's body through this point
// Related: heavy.go -- SlotOfLine, the reading that says which lines are admitted here

package leroot

import (
	"fmt"
	"io"
	"strings"
	"sync"

	"github.com/ze-software/ze/internal/component/command"
	"github.com/ze-software/ze/internal/core/textbuf"
	leaction "github.com/ze-software/ze/internal/le/le/action"
)

// Admitter takes the job slot of a centrally admitted line (SlotCentral) and
// runs its body inside it. commandName is the name the line resolved to, as
// HeavyCommand answers it: the job label is derived from it. line is the
// program and the words as typed, for the entry's command field. body runs
// the action and answers its exit code.
//
// It answers the exit code le returns: the body's when the body ran, and a
// refusal's nonzero code when no slot could be asked for or the wait was
// ended, in which case it MUST NOT have run the body. It MUST call body at
// most once.
type Admitter func(commandName string, line []string, body func() int) int

// admitter is the one injected admission function. internal/le/job imports
// this package for its own registration, so the dispatcher cannot name the
// job package: the job package fills this seam instead. Registration runs in
// init() and dispatch reads it on any goroutine, so it is guarded. Safe for
// concurrent use.
var admitter struct {
	sync.RWMutex
	admit Admitter
}

// RegisterAdmitter fills the admission seam: the function the dispatcher
// calls for every line SlotOfLine reads as SlotCentral. One package fills it,
// so a nil function and a second registration are Ze defects and panic. MUST
// be called from init.
func RegisterAdmitter(admit Admitter) {
	if admit == nil {
		panic("BUG: leroot.RegisterAdmitter: nil admitter; see the init frame above for the package")
	}
	admitter.Lock()
	defer admitter.Unlock()
	if admitter.admit != nil {
		panic("BUG: leroot.RegisterAdmitter: a second admitter registered, and one line has one job slot")
	}
	admitter.admit = admit
}

// SetAdmitter replaces the admitter and answers the function that restores
// the previous one. It exists for tests, which install a fake so a dispatched
// line claims no slot of the machine: a caller MUST call the restore function
// before the test ends. A nil admit leaves the seam unfilled. It is a Go seam
// only, so no keyword, flag or environment variable reaches it.
func SetAdmitter(admit Admitter) (restore func()) {
	admitter.Lock()
	defer admitter.Unlock()
	previous := admitter.admit
	admitter.admit = admit
	return func() {
		admitter.Lock()
		defer admitter.Unlock()
		admitter.admit = previous
	}
}

// registeredAdmitter answers the admitter, and nil while none is registered.
func registeredAdmitter() Admitter {
	admitter.RLock()
	defer admitter.RUnlock()
	return admitter.admit
}

// centralSlot answers the admitter and the command name for a line the
// dispatcher admits, and a nil admitter for every line it runs as it is. name
// is the resolved area, typed its own words as typed, and line the words the
// body receives, from the action's verb on when prepared says the area's
// preparation already parsed them. A nonzero code is a refusal already
// written to errOut, and the body MUST NOT run.
//
// The order is fixed. The slot reading comes first, and it reads memory only,
// so a light line costs no file, lock or process. A line the area's own
// parser refuses is not queued: it goes to the handler, which refuses it as
// it always did. The whole-tree gate is asked before any slot, so a release
// check is refused at once and never after a wait.
//
// While no admitter is registered every line runs unadmitted, as it did
// before admission was central: the seam is filled by internal/le/job.
func centralSlot(name string, typed, line []string, prepared bool, list leaction.List, declared bool, errOut io.Writer) (Admitter, string, int) {
	admit := registeredAdmitter()
	if admit == nil {
		return nil, "", 0
	}
	if name == "" {
		// The root names no command: it answers what le holds.
		return nil, "", 0
	}
	if asksForUsage(name, typed) {
		return nil, "", 0
	}
	slot := slotOf(typedLine{area: name, rest: typed, list: list, declared: declared})
	if slot.Kind != SlotCentral {
		return nil, "", 0
	}
	if !declared {
		return admit, slot.Command, 0
	}
	if list.PassesWords {
		return admit, slot.Command, 0
	}
	if !prepared {
		line = list.AtVerb(line)
		if !tableTakes(list, line) {
			return nil, "", 0
		}
	}
	if err := list.AdmitWholeTree(line); err != nil {
		var tb textbuf.Buffer
		fmt.Fprintln(errOut, tb.Str("error: ").Err(err).String()) //nolint:errcheck // CLI output
		return nil, "", 2
	}
	return admit, slot.Command, 0
}

// tableTakes reports whether the area's own parser takes line, the words from
// the verb on. A refused line is not an error here: its area reports the
// refusal when the handler runs it.
func tableTakes(list leaction.List, line []string) bool {
	return list.AcceptsLine(line) == nil
}

// runInSlot runs body, the action's work, and answers what it answered. With
// a nil admit the body runs as it is. Otherwise admit takes the slot, and its
// code is the line's: where it refused without running the body, the answer
// carries that code and no payload.
func runInSlot(admit Admitter, commandName, name string, typed []string, body func() command.LocalAnswer) command.LocalAnswer {
	if admit == nil {
		return body()
	}
	line := make([]string, 0, 4+len(typed))
	line = append(line, strings.Fields(leaction.Program())...)
	line = append(line, strings.Fields(name)...)
	line = append(line, typed...)

	var answer command.LocalAnswer
	ran := false
	code := admit(commandName, line, func() int {
		answer = body()
		ran = true
		return answer.Code
	})
	if ran {
		answer.Code = code
		return answer
	}
	if code == 0 {
		panic("BUG: leroot: the admitter answered 0 for " + commandName + " without running its body, so nothing ran and le would report success")
	}
	return command.LocalAnswer{Code: code}
}
