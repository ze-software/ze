// Design: docs/architecture/doctor-and-health-checks.md -- event codes the recent-events check declares
// Related: doctor_registry.go -- DoctorCheck.DeclaresEventCodes, which lists these codes
// Related: codes.go -- builtinCodes, the metadata a registration must find
//
// A transient problem is raised on the report bus and reported by the
// recent-events doctor check, which renders each event under the code its
// producer raised. That check is the one doctor check that answers for those
// codes, so it declares them; it cannot list them itself, because the codes
// belong to their owners. An owner therefore registers each code it raises
// here, from its init(), and the doctor registry adds them to the declaring
// check when it is read.

package diagnostic

import (
	"errors"
	"slices"
	"sync"

	"github.com/ze-software/ze/internal/core/textbuf"
)

// EventCode is a transient code an owner raises on the report bus. It is a
// struct, and owners write it as a composite literal, because the
// doctor-coverage gate (internal/le/arch/doctorcoverage) reads the declared
// codes from those literals in the source: Code MUST be a string literal or a
// constant, never a value computed at run time.
type EventCode struct {
	Code string
}

var eventCodeRegistry = struct {
	sync.Mutex
	codes map[string]struct{}
}{codes: make(map[string]struct{})}

// RegisterEventCode declares that the caller raises code on the report bus,
// so the recent-events check declares it and the doctor-coverage gate counts
// a line or raise carrying it covered. Owners call it from init().
//
// The code MUST be described: by an entry in builtinCodes (which registers
// after init(), so the table is read directly) or by an earlier Register.
// A code with no description is refused, because `ze show diagnostic` must
// answer for every code doctor declares. Two owners may register one code (a
// cross-cutting code such as doctor-internal-defect has several), and it is
// listed once.
//
// Safe for concurrent use.
func RegisterEventCode(event EventCode) error {
	if !isLowerKebabDiag(event.Code) {
		var tb textbuf.Buffer
		return errors.New(tb.Str("diagnostic event code: invalid code ").Str(event.Code).
			Str(": a code is lower kebab case, such as doctor-plugin-crashed").String())
	}
	if !describedCode(event.Code) {
		var tb textbuf.Buffer
		return errors.New(tb.Str("diagnostic event code: ").Str(event.Code).
			Str(" has no metadata, so ze show diagnostic cannot describe it. ").
			Str("Add its CodeMeta entry to builtinCodes in internal/core/diagnostic/codes.go").String())
	}
	eventCodeRegistry.Lock()
	defer eventCodeRegistry.Unlock()
	eventCodeRegistry.codes[event.Code] = struct{}{}
	return nil
}

// EventCodes answers every registered event code, sorted. With none
// registered it answers an empty list: an event code is a declaration, so no
// registration is the true answer of a binary that links no owner.
//
// Safe for concurrent use.
func EventCodes() []string {
	eventCodeRegistry.Lock()
	defer eventCodeRegistry.Unlock()
	codes := make([]string, 0, len(eventCodeRegistry.codes))
	for code := range eventCodeRegistry.codes {
		codes = append(codes, code)
	}
	slices.Sort(codes)
	return codes
}

// UnregisterEventCodeForTest removes one event code. Test use only: a test
// that registers a fixture code MUST call it from a cleanup, so the next test
// in the binary reads the codes the binary's init() functions registered.
func UnregisterEventCodeForTest(code string) {
	eventCodeRegistry.Lock()
	defer eventCodeRegistry.Unlock()
	delete(eventCodeRegistry.codes, code)
}

// describedCode answers whether `ze show diagnostic` describes code: the
// builtin table, read directly because it registers only after init(), or a
// code registered already.
func describedCode(code string) bool {
	for i := range builtinCodes {
		if builtinCodes[i].Code == code {
			return true
		}
	}
	return Lookup(code) != nil
}
