// Design: docs/architecture/config/environment.md — port flag helpers for env-aware CLI defaults
// Related: registry.go — env var registration and entry listing

package env

import "strconv"

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
		return val, desc + " (disabled, env: " + key + ")"
	}
	if envVal != "" && envVal != strconv.Itoa(fallback) {
		return val, desc + " (default: " + strconv.Itoa(fallback) + ", configured: " + envVal + " via " + key + ")"
	}
	return val, desc + " (default: " + strconv.Itoa(fallback) + ", env: " + key + ")"
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
		return val, desc + " (disabled, env: " + key + ")"
	}
	if envVal != "" && envVal != fallback {
		return val, desc + " (default: " + fallback + ", configured: " + envVal + " via " + key + ")"
	}
	return val, desc + " (default: " + fallback + ", env: " + key + ")"
}
