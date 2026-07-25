// Design: docs/architecture/config/environment.md — port flag helpers for env-aware CLI defaults
// Related: registry.go — env var registration and entry listing

package env

import (
	"strconv"

	"github.com/ze-software/ze/internal/core/textbuf"
)

// PortDefault resolves an integer port flag's default value and builds a
// description string that shows the env var name, the hardcoded default,
// and (when the env var is set) the configured override.
//
// Returns the resolved value (env override or fallback) and the formatted
// description for use with flag.IntVar.
func PortDefault(key string, fallback int, desc string) (int, string) {
	val := fallback
	envVal := Get(key)
	if envVal != "" {
		if v, err := strconv.Atoi(envVal); err == nil {
			val = v
		}
	}

	if fallback == 0 && envVal == "" {
		var b0 textbuf.Buffer
		return val, b0.Str(desc).Str(" (disabled, env: ").Str(key).Byte(')').String()
	}
	if envVal != "" && envVal != strconv.Itoa(fallback) {
		var b1 textbuf.Buffer
		return val, b1.Str(desc).Str(" (default: ").Int(int64(fallback)).Str(", configured: ").Str(envVal).Str(" via ").Str(key).Byte(')').String()
	}
	var b2 textbuf.Buffer
	return val, b2.Str(desc).Str(" (default: ").Int(int64(fallback)).Str(", env: ").Str(key).Byte(')').String()
}

// AddrPortDefault resolves a string addr:port flag's default value and builds
// a description string that shows the env var name, the hardcoded default,
// and (when the env var is set) the configured override.
//
// Returns the resolved value (env override or fallback) and the formatted
// description for use with flag.StringVar.
func AddrPortDefault(key, fallback, desc string) (string, string) {
	val := fallback
	envVal := Get(key)
	if envVal != "" {
		val = envVal
	}

	if fallback == "" && envVal == "" {
		var b0 textbuf.Buffer
		return val, b0.Str(desc).Str(" (disabled, env: ").Str(key).Byte(')').String()
	}
	if envVal != "" && envVal != fallback {
		var b1 textbuf.Buffer
		return val, b1.Str(desc).Str(" (default: ").Str(fallback).Str(", configured: ").Str(envVal).Str(" via ").Str(key).Byte(')').String()
	}
	var b2 textbuf.Buffer
	return val, b2.Str(desc).Str(" (default: ").Str(fallback).Str(", env: ").Str(key).Byte(')').String()
}
