// Design: docs/features/ai-first.md — diagnostic code registry

package diagnostic

import (
	"errors"
	"slices"
	"sync"
)

// CodeMeta holds the explanation metadata for a registered diagnostic code.
type CodeMeta struct {
	Code         string
	Title        string
	Description  string
	Examples     []string
	RelatedCodes []string
	// HostVerdict is true for a code that judges the host running the
	// command (its kernel, its OS, its hardware), not the configuration: the
	// same config validates on a host that has what it needs. A tool that
	// judges configs written for another machine reads it to tell the two
	// apart, never the message text.
	HostVerdict bool
}

var (
	mu               sync.Mutex
	codes            = map[string]*CodeMeta{}
	errDuplicateCode = errors.New("diagnostic: duplicate code")
)

// Register adds a diagnostic code with its explanation metadata.
// Returns an error if the code is already registered.
func Register(m CodeMeta) error {
	mu.Lock()
	defer mu.Unlock()
	if _, exists := codes[m.Code]; exists {
		return errDuplicateCode
	}
	codes[m.Code] = &m
	return nil
}

// Lookup returns the metadata for a code, or nil if unknown.
func Lookup(code string) *CodeMeta {
	mu.Lock()
	defer mu.Unlock()
	return codes[code]
}

// AllCodes returns all registered codes sorted alphabetically.
func AllCodes() []string {
	mu.Lock()
	defer mu.Unlock()
	result := make([]string, 0, len(codes))
	for k := range codes {
		result = append(result, k)
	}
	slices.Sort(result)
	return result
}

// ResetForTest clears the registry. Test use only.
func ResetForTest() {
	mu.Lock()
	defer mu.Unlock()
	codes = map[string]*CodeMeta{}
}
