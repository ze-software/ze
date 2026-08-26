// Design: docs/architecture/testing/qemu-integration.md -- what a QEMU proof asks for
//
// sizes.go reads the two byte quantities for the hugepage proof. One quantity
// is the appliance memory. The other is the amount reserved as hugepages. Both
// arrive as operator strings, such as "128mb" and "1gb". The appliance
// configuration uses the same words.
//
// The parse REFUSES what it cannot read. The Python original called int() with
// the text before the unit. A negative size produced a negative page count. A
// fractional size ended the run in a traceback. Neither is a quantity of
// memory. If accepted, a reservation derived from either would reach a kernel
// command line.

package qemu

import (
	"errors"

	"github.com/ze-software/ze/internal/core/textbuf"
)

// sizeUnit defines one valid size suffix and its multiplier.
type sizeUnit struct {
	suffix string
	scale  uint64
}

// sizeUnits is the table, longest suffix first. The order is load-bearing: "b"
// matches the tail of every other unit, so it MUST be tried last.
var sizeUnits = []sizeUnit{
	{"tb", 1 << 40},
	{"gb", 1 << 30},
	{"mb", 1 << 20},
	{"kb", 1 << 10},
	{"b", 1},
}

// sizeDigitsMax limits the number of digits in a size. Sixteen decimal digits
// can represent ten petabytes at the smallest unit. That is more memory than
// any appliance has. The limit also keeps accumulation below the point where a
// uint64 wraps.
const sizeDigitsMax = 16

// ErrBadSize is the answer to a string that is not a byte quantity.
var ErrBadSize = errors.New("a size must be a whole number of b, kb, mb, gb or tb")

// ErrNoWholePage is the answer to a reservation smaller than one page.
var ErrNoWholePage = errors.New("the hugepage reservation is smaller than one page, so it would reserve nothing")

// ParseSize answers how many bytes a size names.
//
// The comparison is over the lower-cased string, because an operator writes
// "1GB" as readily as "1gb" and the appliance configuration accepts both.
func ParseSize(size string) (uint64, error) {
	lowered := lower(size)
	for _, unit := range sizeUnits {
		digits, ok := cutSuffix(lowered, unit.suffix)
		if !ok {
			continue
		}
		value, err := wholeNumber(digits)
		if err != nil {
			return 0, err
		}
		return value * unit.scale, nil
	}
	return 0, ErrBadSize
}

// PageCount answers the number of pageSize pages that fit in total. It rounds
// DOWN to whole pages. A reservation contains whole pages. A kernel command
// line cannot request a fraction of a page.
//
// PageCount refuses a page size of zero rather than divide by it. It also
// refuses a total smaller than one page. Such a total would reserve nothing,
// but the run would assert that the kernel had reserved something.
func PageCount(total, pageSize string) (uint64, error) {
	totalBytes, err := ParseSize(total)
	if err != nil {
		return 0, err
	}
	pageBytes, err := ParseSize(pageSize)
	if err != nil {
		return 0, err
	}
	if pageBytes == 0 || totalBytes < pageBytes {
		return 0, ErrNoWholePage
	}
	return totalBytes / pageBytes, nil
}

// wholeNumber reads an unsigned decimal, refusing everything else. A sign, a
// decimal point, a space and an empty string are each a size this tool cannot
// turn into a page count.
func wholeNumber(digits string) (uint64, error) {
	if digits == "" || len(digits) > sizeDigitsMax {
		return 0, ErrBadSize
	}
	var value uint64
	for i := range len(digits) {
		char := digits[i]
		if char < '0' || char > '9' {
			return 0, ErrBadSize
		}
		value = value*10 + uint64(char-'0')
	}
	return value, nil
}

// cutSuffix answers what precedes suffix, and whether the string ended in it. A
// string that is ONLY the suffix answers false, because a unit with no number
// in front of it names no quantity.
func cutSuffix(text, suffix string) (string, bool) {
	if len(text) <= len(suffix) {
		return "", false
	}
	if text[len(text)-len(suffix):] != suffix {
		return "", false
	}
	return text[:len(text)-len(suffix)], true
}

// lower answers text with each ASCII capital converted to lowercase. It is
// written out instead of taken from strings. The input is one short operator
// setting with a known alphabet. A Unicode fold would accept letters that no
// unit uses.
func lower(text string) string {
	var tb textbuf.Buffer
	for i := range len(text) {
		char := text[i]
		if char >= 'A' && char <= 'Z' {
			char += 'a' - 'A'
		}
		tb.Byte(char)
	}
	return tb.String()
}
