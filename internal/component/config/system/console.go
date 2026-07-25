// Design: docs/architecture/config/syntax.md — console serial config extraction

package system

import (
	"fmt"
	"strings"

	"github.com/ze-software/ze/internal/component/config"
)

// ConsoleDeviceEntry holds config for one serial console device.
type ConsoleDeviceEntry struct {
	Name  string
	Speed int
}

var validSpeeds = map[int]bool{
	9600:   true,
	19200:  true,
	38400:  true,
	57600:  true,
	115200: true,
}

// ValidConsoleDeviceName checks that a device name is a bare name
// with only printable ASCII, no path separators or traversal.
func ValidConsoleDeviceName(name string) bool {
	if name == "" {
		return false
	}
	for i := range len(name) {
		c := name[i]
		if c < 0x20 || c > 0x7e || c == '/' {
			return false
		}
	}
	return !strings.Contains(name, "..")
}

// ExtractConsoleFromMap extracts console config from a map[string]any
// tree (used by the reload path which has no *config.Tree).
func ExtractConsoleFromMap(tree map[string]any) []ConsoleDeviceEntry {
	sys, _ := tree["system"].(map[string]any)
	if sys == nil {
		return nil
	}
	console, _ := sys["console"].(map[string]any)
	if console == nil {
		return nil
	}
	devices, _ := console["device"].(map[string]any)
	if len(devices) == 0 {
		return nil
	}

	var entries []ConsoleDeviceEntry
	for name, entry := range devices {
		if !ValidConsoleDeviceName(name) {
			continue
		}
		m, _ := entry.(map[string]any)
		if m == nil {
			continue
		}

		speed := 115200
		if v, _ := m["speed"].(string); v != "" {
			var n int
			if _, err := fmt.Sscanf(v, "%d", &n); err == nil && validSpeeds[n] {
				speed = n
			}
		}

		entries = append(entries, ConsoleDeviceEntry{
			Name:  name,
			Speed: speed,
		})
	}

	return entries
}

func extractConsole(sys *config.Tree) []ConsoleDeviceEntry {
	console := sys.GetContainer("console")
	if console == nil {
		return nil
	}

	devices := console.GetList("device")
	if len(devices) == 0 {
		return nil
	}

	var entries []ConsoleDeviceEntry
	for name, dev := range devices {
		if !ValidConsoleDeviceName(name) {
			continue
		}

		speed := 115200
		if v, ok := dev.Get("speed"); ok {
			var n int
			if _, err := fmt.Sscanf(v, "%d", &n); err == nil && validSpeeds[n] {
				speed = n
			}
		}

		entries = append(entries, ConsoleDeviceEntry{
			Name:  name,
			Speed: speed,
		})
	}

	return entries
}
