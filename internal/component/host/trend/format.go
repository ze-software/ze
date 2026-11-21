// Design: docs/architecture/host/observability.md -- the host trend monitor
// Related: register_kernel_memory.go -- a warning message built from these

package trend

import (
	"math"
	"time"

	"github.com/ze-software/ze/internal/core/textbuf"
)

// byteUnits are the binary units humanBytes words a size in.
var byteUnits = [...]string{"B", "KiB", "MiB", "GiB", "TiB", "PiB"}

// humanBytes words a size for an operator: the largest binary unit that keeps
// the figure at 1 or more, one decimal under 10 of that unit ("1.5 GiB"), none
// above ("412 MiB"). A negative size reads as 0 B: no figure a rule words is
// negative.
func humanBytes(bytes float64) string {
	value := math.Max(bytes, 0)
	unit := 0
	for value >= 1024 && unit < len(byteUnits)-1 {
		value /= 1024
		unit++
	}
	var tb textbuf.Buffer
	if unit > 0 && value < 10 {
		tb.Float(value, 1)
	} else {
		tb.Uint(uint64(math.Round(value)))
	}
	return tb.Byte(' ').Str(byteUnits[unit]).String()
}

// humanDuration words a span for an operator in its largest whole unit: days
// from two days, hours from two hours, minutes below that.
func humanDuration(d time.Duration) string {
	var tb textbuf.Buffer
	switch {
	case d >= 48*time.Hour:
		tb.Uint(uint64(math.Round(d.Hours() / 24))).Str(" days")
	case d >= 24*time.Hour:
		tb.Str("1 day")
	case d >= 2*time.Hour:
		tb.Uint(uint64(math.Round(d.Hours()))).Str(" hours")
	case d >= time.Hour:
		tb.Str("1 hour")
	case d >= 2*time.Minute:
		tb.Uint(uint64(math.Round(d.Minutes()))).Str(" minutes")
	default:
		tb.Str("1 minute")
	}
	return tb.String()
}
