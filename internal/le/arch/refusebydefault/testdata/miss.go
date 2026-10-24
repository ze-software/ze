// Fixture for RuleMissAnswered (AC-2).
package fixture

import (
	"errors"
	"os"
	"strconv"
	"strings"
)

var resolver func(name string) (map[uint32]uint32, error)

var errNoResolver = errors.New("no resolver registered")

func locate(data []byte) (int, bool, bool) {
	return 0, len(data) > 4, len(data) > 0
}

// verify mirrors the RADIUS defect: an absent authenticator answers true. The
// first branch answers false, which refuses, so it is no finding.
func verify(data []byte) bool {
	_, present, ok := locate(data)
	if !ok {
		return false
	}
	if !present { // want: miss-answered-as-value
		return true
	}
	return len(data) > 8
}

// resolve mirrors cos.Resolve: no resolver answers nil maps and a nil error.
func resolve(name string) (map[uint32]uint32, error) {
	fn := resolver
	if fn == nil { // want: miss-answered-as-value
		return nil, nil
	}
	return fn(name)
}

// resolveDirect tests the registered package variable itself.
func resolveDirect(name string) (map[uint32]uint32, error) {
	if resolver == nil { // want: miss-answered-as-value
		return nil, nil
	}
	return resolver(name)
}

// resolveRefuses is the correct shape: no finding.
func resolveRefuses(name string) (map[uint32]uint32, error) {
	if resolver == nil {
		return nil, errNoResolver
	}
	return resolver(name)
}

// exists mirrors interfaceExists: any read error reads as absent.
func exists(path string) bool {
	_, err := os.Stat(path)
	return err == nil // want: miss-answered-as-value
}

// absentOnError answers "absent" for a read error.
func absentOnError(path string) bool {
	info, err := os.Stat(path)
	if err != nil { // want: miss-answered-as-value
		return false
	}
	return info.IsDir()
}

// size answers zero for a read error.
func size(path string) int64 {
	info, err := os.Stat(path)
	if err != nil { // want: miss-answered-as-value
		return 0
	}
	return info.Size()
}

// asserted answers a nil error for a value of the wrong type.
func asserted(value any) (string, error) {
	text, ok := value.(string)
	if !ok { // want: miss-answered-as-value
		return "", nil
	}
	return text, nil
}

// lookup hands the miss to its caller in the comma-ok form: no finding.
func lookup(table map[string]int, key string) (int, bool) {
	value, ok := table[key]
	if !ok {
		return 0, false
	}
	return value, true
}

// has answers the map's own "no" for a miss: no finding.
func has(table map[string]int, key string) bool {
	_, ok := table[key]
	if !ok {
		return false
	}
	return true
}

// redirect hands the miss on through its flag: no finding.
func redirect(token string) (string, bool, error) {
	value, ok := strings.CutPrefix(token, "redirect:")
	if !ok {
		return "", false, nil
	}
	return value, true, nil
}

type collector struct {
	refusals []string
	drops    int
}

// collect records the miss before it answers: no finding.
func (c *collector) collect(table map[string]int, key string) error {
	_, ok := table[key]
	if !ok {
		c.refusals = append(c.refusals, key)
		return nil
	}
	return nil
}

// drop counts the discard before it answers: no finding.
func (c *collector) drop(table map[string]int, key string) error {
	_, ok := table[key]
	if !ok {
		c.drops++
		return nil
	}
	return nil
}

// parsed answers the parse's own "no", which is not a read: no finding.
func parsed(text string) bool {
	_, err := strconv.Atoi(text)
	return err == nil
}

// count answers zero for a parse failure, the parse's own "none": no finding.
func count(text string) int {
	value, err := strconv.Atoi(text)
	if err != nil {
		return 0
	}
	return value
}

// carried returns the error inside its answer: no finding.
func carried(path string) (string, bool) {
	_, err := os.Stat(path)
	if err != nil {
		return "could not read " + path + ": " + err.Error(), true
	}
	return "", true
}

// captured hands the error to the enclosing function: no finding.
func captured(paths []string) error {
	var refusal error
	walk(paths, func(path string) bool {
		_, err := os.Stat(path)
		if err != nil {
			refusal = err
			return false
		}
		return true
	})
	return refusal
}

func walk(paths []string, visit func(string) bool) {
	for _, path := range paths {
		if !visit(path) {
			return
		}
	}
}

// visit steers a walk: its true means "keep going", not "found": no finding.
func visit(table map[string]int, keys []string) int {
	total := 0
	each(keys, func(key string) bool {
		value, ok := table[key]
		if !ok {
			return true
		}
		total += value
		return true
	})
	return total
}

func each(keys []string, yield func(string) bool) {
	for _, key := range keys {
		if !yield(key) {
			return
		}
	}
}
