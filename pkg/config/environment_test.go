package config

import (
	"testing"
)

func TestLoadEnvironmentDefaults(t *testing.T) {
	env := LoadEnvironment()

	// Check daemon defaults
	if env.Daemon.User != "nobody" {
		t.Errorf("Daemon.User = %q, want %q", env.Daemon.User, "nobody")
	}
	if !env.Daemon.Drop {
		t.Error("Daemon.Drop should be true by default")
	}
	if env.Daemon.Umask != 0o137 {
		t.Errorf("Daemon.Umask = %o, want %o", env.Daemon.Umask, 0o137)
	}

	// Check log defaults
	if !env.Log.Enable {
		t.Error("Log.Enable should be true by default")
	}
	if env.Log.Level != "INFO" {
		t.Errorf("Log.Level = %q, want %q", env.Log.Level, "INFO")
	}
	if env.Log.Destination != "stdout" {
		t.Errorf("Log.Destination = %q, want %q", env.Log.Destination, "stdout")
	}
	if !env.Log.Short {
		t.Error("Log.Short should be true by default")
	}

	// Check TCP defaults
	if env.TCP.Port != 179 {
		t.Errorf("TCP.Port = %d, want %d", env.TCP.Port, 179)
	}

	// Check BGP defaults
	if env.BGP.OpenWait != 60 {
		t.Errorf("BGP.OpenWait = %d, want %d", env.BGP.OpenWait, 60)
	}

	// Check API defaults
	if !env.API.ACK {
		t.Error("API.ACK should be true by default")
	}
	if env.API.Encoder != "json" {
		t.Errorf("API.Encoder = %q, want %q", env.API.Encoder, "json")
	}
	const defaultSocketName = "zebgp"
	if env.API.SocketName != defaultSocketName {
		t.Errorf("API.SocketName = %q, want %q", env.API.SocketName, defaultSocketName)
	}
}

func TestLoadEnvironmentFromEnv(t *testing.T) {
	// Use t.Setenv for test-scoped env vars
	t.Setenv("zebgp.log.level", "DEBUG")
	t.Setenv("zebgp.tcp.port", "1179")
	t.Setenv("zebgp.bgp.passive", "true")
	t.Setenv("zebgp.api.socketname", "test-socket")

	env := LoadEnvironment()

	if env.Log.Level != "DEBUG" {
		t.Errorf("Log.Level = %q, want %q", env.Log.Level, "DEBUG")
	}
	if env.TCP.Port != 1179 {
		t.Errorf("TCP.Port = %d, want %d", env.TCP.Port, 1179)
	}
	if !env.BGP.Passive {
		t.Error("BGP.Passive should be true")
	}
	if env.API.SocketName != "test-socket" {
		t.Errorf("API.SocketName = %q, want %q", env.API.SocketName, "test-socket")
	}
}

func TestLoadEnvironmentUnderscoreNotation(t *testing.T) {
	// Use t.Setenv for test-scoped env vars
	t.Setenv("zebgp_log_level", "WARNING")
	t.Setenv("zebgp_tcp_port", "2179")

	env := LoadEnvironment()

	if env.Log.Level != "WARNING" {
		t.Errorf("Log.Level = %q, want %q", env.Log.Level, "WARNING")
	}
	if env.TCP.Port != 2179 {
		t.Errorf("TCP.Port = %d, want %d", env.TCP.Port, 2179)
	}
}

func TestLoadEnvironmentDotPriority(t *testing.T) {
	// Set both notations - dot should take priority
	t.Setenv("zebgp.log.level", "DEBUG")
	t.Setenv("zebgp_log_level", "WARNING")

	env := LoadEnvironment()

	if env.Log.Level != "DEBUG" {
		t.Errorf("Log.Level = %q, want %q (dot notation should take priority)", env.Log.Level, "DEBUG")
	}
}

func TestLoadEnvironmentBooleanValues(t *testing.T) {
	tests := []struct {
		value string
		want  bool
	}{
		{"1", true},
		{"true", true},
		{"True", true},
		{"TRUE", true},
		{"yes", true},
		{"Yes", true},
		{"on", true},
		{"enable", true},
		{"0", false},
		{"false", false},
		{"no", false},
		{"off", false},
		{"disable", false},
		{"random", false},
	}

	for _, tt := range tests {
		t.Run(tt.value, func(t *testing.T) {
			t.Setenv("zebgp.bgp.passive", tt.value)
			env := LoadEnvironment()

			if env.BGP.Passive != tt.want {
				t.Errorf("BGP.Passive with %q = %v, want %v", tt.value, env.BGP.Passive, tt.want)
			}
		})
	}
}

func TestLoadEnvironmentTCPOnceBackwardCompat(t *testing.T) {
	// tcp.once should set tcp.attempts to 1
	t.Setenv("zebgp.tcp.once", "true")

	env := LoadEnvironment()

	if env.TCP.Attempts != 1 {
		t.Errorf("TCP.Attempts = %d, want 1 (from tcp.once backward compat)", env.TCP.Attempts)
	}
}

func TestOpenWaitDuration(t *testing.T) {
	env := LoadEnvironment()

	dur := env.OpenWaitDuration()
	want := 60 * 1000 * 1000 * 1000 // 60 seconds in nanoseconds

	if int64(dur) != int64(want) {
		t.Errorf("OpenWaitDuration() = %v, want 60s", dur)
	}
}

func TestSocketPath(t *testing.T) {
	t.Run("default", func(t *testing.T) {
		env := LoadEnvironment()
		if env.SocketPath() != "/var/run/zebgp.sock" {
			t.Errorf("SocketPath() = %q, want %q", env.SocketPath(), "/var/run/zebgp.sock")
		}
	})

	t.Run("custom", func(t *testing.T) {
		t.Setenv("zebgp.api.socketname", "custom")
		env := LoadEnvironment()
		if env.SocketPath() != "/var/run/custom.sock" {
			t.Errorf("SocketPath() = %q, want %q", env.SocketPath(), "/var/run/custom.sock")
		}
	})
}
