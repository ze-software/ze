package humanize

import "fmt"

func BPS(bps uint64) string {
	switch {
	case bps > (1024 * 1024):
		return fmt.Sprintf("%.f MiB/s", float64(bps)/1024/1024)
	case bps > 1024:
		return fmt.Sprintf("%.f KiB/s", float64(bps)/1024)
	default:
		return fmt.Sprintf("%d B/s", bps)
	}
}

func Bytes(bytes uint64) string {
	switch {
	case bytes > (1024 * 1024):
		return fmt.Sprintf("%.f MiB", float64(bytes)/1024/1024)
	case bytes > 1024:
		return fmt.Sprintf("%.f KiB", float64(bytes)/1024)
	default:
		return fmt.Sprintf("%d B", bytes)
	}
}
