// Design: docs/architecture/config/syntax.md -- YANG environment block plumbing to env vars
// Related: environment_extract.go -- config tree -> section/option map extractor

package config

import (
	"codeberg.org/thomas-mangin/ze/internal/core/env"
	"codeberg.org/thomas-mangin/ze/internal/core/slogutil"
)

// applyEnvLogger is the subsystem logger for YANG->env plumbing diagnostics.
// Controlled by ze.log.config.
//
//nolint:gochecknoglobals // Package-level lazy logger.
var applyEnvLogger = slogutil.LazyLogger("config")

// envPlumbing maps YANG `environment/<section>/<option>` (or a top-level
// `environment/<option>`) to a Ze env-var key. Entries land in os.Environ
// via env.Set, which lets child processes (e.g. the exabgp bridge) inherit
// them and read via os.Getenv. OS-env is authoritative: a pre-existing
// value for the target key is NOT overwritten.
type envPlumbing struct {
	section string // "" for top-level leaves under `environment/<option>`
	option  string
	envKey  string
}

// envPlumbingTable is the authoritative list of YANG-to-env mappings that
// bypass slogutil.ApplyLogConfig (the log path owns everything under
// `environment/log/`).
//
//nolint:gochecknoglobals // Table-driven plumbing.
var envPlumbingTable = []envPlumbing{
	{section: "daemon", option: "pid", envKey: "ze.pid.file"},
	{section: "daemon", option: "user", envKey: "ze.user"},
	{section: "bgp", option: "openwait", envKey: "ze.bgp.openwait"},
	{section: "bgp", option: "announce-delay", envKey: "ze.bgp.announce.delay"},
	{section: "chaos", option: "seed", envKey: "ze.bgp.chaos.seed"},
	{section: "chaos", option: "rate", envKey: "ze.bgp.chaos.rate"},
	{section: "reactor", option: "speed", envKey: "ze.bgp.reactor.speed"},
	{section: "reactor", option: "cache-ttl", envKey: "ze.bgp.reactor.cache-ttl"},
	{section: "reactor", option: "cache-max", envKey: "ze.bgp.reactor.cache-max"},
	{section: "reactor", option: "update-groups", envKey: "ze.bgp.reactor.update-groups"},
	{section: "reactor", option: "coalesce", envKey: "ze.bgp.reactor.coalesce"},
	{section: "exabgp.api", option: "ack", envKey: "exabgp.api.ack"},
	{section: "", option: "pprof", envKey: "ze.pprof"},
}

// ApplyEnvConfig plumbs surviving `environment/` YANG leaves into Ze env vars.
// This is the sibling of slogutil.ApplyLogConfig: the log path owns
// `environment/log/*`, this function owns everything else listed in
// envPlumbingTable.
//
// Priority: OS env var > config file value. A key already present in the OS
// environment is NOT overwritten -- the operator's explicit setting wins.
//
// Call immediately after slogutil.ApplyLogConfig during config load.
func ApplyEnvConfig(configValues map[string]map[string]string) {
	if configValues == nil {
		return
	}

	for _, p := range envPlumbingTable {
		value, ok := lookupPlumbValue(configValues, p)
		if !ok {
			continue
		}
		if existing := env.Get(p.envKey); existing != "" {
			applyEnvLogger().Debug("apply-env: OS env wins over config",
				"key", p.envKey, "os", existing, "config", value)
			continue
		}
		_ = env.Set(p.envKey, value)
		applyEnvLogger().Debug("apply-env: set from config",
			"key", p.envKey, "value", value, "section", p.section, "option", p.option)
	}
}

// lookupPlumbValue returns the value for a plumbing entry. Top-level leaves
// (section == "") are stored in configValues[""]. Nested containers like
// `exabgp.api` are stored as a dot-joined section key.
func lookupPlumbValue(configValues map[string]map[string]string, p envPlumbing) (string, bool) {
	section, ok := configValues[p.section]
	if !ok {
		return "", false
	}
	value, ok := section[p.option]
	if !ok || value == "" {
		return "", false
	}
	return value, true
}
