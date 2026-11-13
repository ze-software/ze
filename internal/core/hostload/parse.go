// Design: docs/architecture/testing/runner-architecture.md -- host load detection for contended-run classification
// Overview: readings.go -- the readings these parsers serve
//
// The parsers of what each platform prints, kept apart from the platform
// readers and built everywhere, so every one is tested on recorded text on
// every platform. Each answers a value or an error and never a zero that
// stands for text it could not read.

package hostload

import (
	"errors"
	"fmt"
	"math"
	"strconv"
	"strings"
)

// Averages is the system load average over the last one, five and fifteen
// minutes: the mean number of runnable tasks, which on Linux also counts the
// tasks blocked on disk.
type Averages struct {
	One     float64
	Five    float64
	Fifteen float64
}

// The two page-statistics lines vm_stat prints besides its header, and the
// words that open the page size in that header.
const (
	vmStatHeaderOpens = "page size of "
	vmStatFree        = "Pages free"
	vmStatInactive    = "Pages inactive"
	vmStatSpeculative = "Pages speculative"
)

// kibBytes is the size of the unit /proc/meminfo counts in. The file writes
// "kB" and means 1024 bytes.
const kibBytes = 1024

// parseLoadAverages reads the three load averages out of what either platform
// prints: "0.52 0.58 0.59 1/467 12345" from Linux's /proc/loadavg, and
// "{ 2.11 1.98 1.85 }" from Darwin's `sysctl -n vm.loadavg`.
func parseLoadAverages(text string) (Averages, error) {
	fields := strings.Fields(text)
	if len(fields) != 0 && fields[0] == "{" {
		fields = fields[1:]
	}
	if len(fields) < 3 {
		return Averages{}, fmt.Errorf("the load averages read %q, want three numbers", strings.TrimSpace(text))
	}

	var parsed [3]float64
	for index := range parsed {
		value, err := strconv.ParseFloat(fields[index], 64)
		if err != nil {
			return Averages{}, fmt.Errorf("the load averages read %q: %w", strings.TrimSpace(text), err)
		}
		if math.IsNaN(value) {
			return Averages{}, fmt.Errorf("the load averages read %q, and %q is not a number", strings.TrimSpace(text), fields[index])
		}
		if math.IsInf(value, 0) {
			return Averages{}, fmt.Errorf("the load averages read %q, and %q is not finite", strings.TrimSpace(text), fields[index])
		}
		if value < 0 {
			return Averages{}, fmt.Errorf("the load averages read %q, and a load is never negative", strings.TrimSpace(text))
		}
		parsed[index] = value
	}
	return Averages{One: parsed[0], Five: parsed[1], Fifteen: parsed[2]}, nil
}

// parseMeminfo reads one field of Linux's /proc/meminfo, such as MemTotal or
// MemAvailable, and answers it in bytes. A line reads "MemTotal:  16384 kB".
func parseMeminfo(text, name string) (uint64, error) {
	for line := range strings.SplitSeq(text, "\n") {
		rest, found := strings.CutPrefix(line, name+":")
		if !found {
			continue
		}
		fields := strings.Fields(rest)
		if len(fields) != 2 {
			return 0, fmt.Errorf("meminfo line %q, want a count and its unit", line)
		}
		if fields[1] != "kB" {
			return 0, fmt.Errorf("meminfo line %q, want the unit kB", line)
		}
		kib, err := strconv.ParseUint(fields[0], 10, 64)
		if err != nil {
			return 0, fmt.Errorf("meminfo line %q: %w", line, err)
		}
		if kib > math.MaxUint64/kibBytes {
			return 0, fmt.Errorf("meminfo line %q counts more bytes than fit", line)
		}
		return kib * kibBytes, nil
	}
	return 0, fmt.Errorf("meminfo holds no %s line", name)
}

// parseVMStatAvailable reads Darwin's `vm_stat` output and answers, in bytes,
// the sum of its "Pages free", "Pages speculative" and "Pages inactive" counts
// multiplied by the page size its header states.
//
// That is the memory the kernel holds free plus the pages it reclaims first.
// It is an estimate of what the system can hand out without swapping, and an
// upper one: an inactive page that is dirty is compressed or written out
// before it is reused. Purgeable pages are left out, because vm_stat does not
// say which of them the inactive count already holds.
//
// The header reads "Mach Virtual Memory Statistics: (page size of 16384
// bytes)" and a count line reads "Pages free:      3651.".
func parseVMStatAvailable(text string) (uint64, error) {
	pageBytes, err := vmStatPageBytes(text)
	if err != nil {
		return 0, err
	}

	var pages uint64
	for _, name := range [...]string{vmStatFree, vmStatSpeculative, vmStatInactive} {
		count, err := vmStatCount(text, name)
		if err != nil {
			return 0, err
		}
		if count > math.MaxUint64-pages {
			return 0, errors.New("vm_stat counts more pages than fit")
		}
		pages += count
	}
	if pageBytes != 0 && pages > math.MaxUint64/pageBytes {
		return 0, errors.New("vm_stat counts more bytes than fit")
	}
	return pages * pageBytes, nil
}

// vmStatPageBytes reads the page size out of vm_stat's header line.
func vmStatPageBytes(text string) (uint64, error) {
	header, _, _ := strings.Cut(text, "\n")
	_, rest, found := strings.Cut(header, vmStatHeaderOpens)
	if !found {
		return 0, fmt.Errorf("vm_stat header %q states no page size", header)
	}
	digits, _, found := strings.Cut(rest, " ")
	if !found {
		return 0, fmt.Errorf("vm_stat header %q states no page size", header)
	}
	pageBytes, err := strconv.ParseUint(digits, 10, 64)
	if err != nil {
		return 0, fmt.Errorf("vm_stat header %q: %w", header, err)
	}
	if pageBytes == 0 {
		return 0, fmt.Errorf("vm_stat header %q states a page size of zero", header)
	}
	return pageBytes, nil
}

// vmStatCount reads one named page count out of vm_stat's lines.
func vmStatCount(text, name string) (uint64, error) {
	for line := range strings.SplitSeq(text, "\n") {
		rest, found := strings.CutPrefix(line, name+":")
		if !found {
			continue
		}
		digits := strings.TrimSuffix(strings.TrimSpace(rest), ".")
		count, err := strconv.ParseUint(digits, 10, 64)
		if err != nil {
			return 0, fmt.Errorf("vm_stat line %q: %w", line, err)
		}
		return count, nil
	}
	return 0, fmt.Errorf("vm_stat printed no %q line", name)
}
